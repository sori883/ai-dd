package flow

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// VerificationDigest describes the bounded content set read from a project.
type VerificationDigest struct {
	Version int      `json:"version"`
	Paths   []string `json:"verification_paths"`
	SHA256  string   `json:"verification_sha256"`
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`
	Missing []string `json:"missing_paths"`
}
type verificationOptions struct {
	maxFiles int
	maxBytes int64
	between  func()
}

func verificationPaths(paths []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || strings.Contains(p, "\\") || filepath.IsAbs(p) || strings.Contains(p, ":") {
			return nil, invalid("verification paths must be project relative")
		}
		for _, part := range strings.Split(p, "/") {
			if part == ".." {
				return nil, invalid("verification path escapes root")
			}
		}
		p = path.Clean(p)
		if excludedVerificationPath(p) {
			return nil, invalid("verification path includes managed aidlc or .git")
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}
func excludedVerificationPath(p string) bool {
	if p == "aidlc" || strings.HasPrefix(p, "aidlc/") {
		return true
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

// ComputeVerification reads a content set twice and rejects inconsistent reads.
func ComputeVerification(root string, paths []string) (VerificationDigest, error) {
	return computeVerification(root, paths, verificationOptions{maxFiles: 10000, maxBytes: 256 * 1024 * 1024})
}
func computeVerification(root string, paths []string, options verificationOptions) (VerificationDigest, error) {
	normalized, err := verificationPaths(paths)
	if err != nil {
		return VerificationDigest{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return VerificationDigest{}, err
	}
	rooted, err := os.OpenRoot(root)
	if err != nil {
		return VerificationDigest{}, err
	}
	defer rooted.Close()
	first, err := verificationPass(rooted, normalized, options)
	if err != nil {
		return VerificationDigest{}, err
	}
	if options.between != nil {
		options.between()
	}
	second, err := verificationPass(rooted, normalized, options)
	if err != nil {
		return VerificationDigest{}, err
	}
	if !reflect.DeepEqual(first, second) {
		return VerificationDigest{}, invalid("verification content changed during reading; retry")
	}
	return second, nil
}
func digestField(h hash.Hash, b []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	h.Write(size[:])
	h.Write(b)
}
func verificationPass(root *os.Root, paths []string, options verificationOptions) (VerificationDigest, error) {
	d := VerificationDigest{Version: 1, Paths: paths, Missing: []string{}}
	h := sha256.New()
	digestField(h, []byte("aidlc-verification-v1"))
	for _, p := range paths {
		digestField(h, []byte(p))
	}
	digestField(h, nil)
	entries := map[string]fs.FileInfo{}
	for _, p := range paths {
		prefix := ""
		for _, part := range strings.Split(p, "/") {
			prefix = path.Join(prefix, part)
			info, err := root.Lstat(prefix)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return d, err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return d, invalid("verification target contains symlink")
			}
		}
		err := fs.WalkDir(root.FS(), p, func(name string, entry fs.DirEntry, err error) error {
			if excludedVerificationPath(name) {
				if entry != nil && entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if err != nil {
				if name == p && os.IsNotExist(err) {
					d.Missing = append(d.Missing, p)
					return nil
				}
				return err
			}
			info, err := root.Lstat(name)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return invalid("verification target is symlink or special file")
			}
			entries[name] = info
			return nil
		})
		if err != nil {
			return d, err
		}
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range d.Missing {
		digestField(h, []byte(name))
		digestField(h, []byte("missing"))
	}
	for _, name := range names {
		info := entries[name]
		digestField(h, []byte(name))
		if info.IsDir() {
			digestField(h, []byte("directory"))
			continue
		}
		d.Files++
		if d.Files > options.maxFiles {
			return d, fmt.Errorf("verification exceeds %d files (%d files, %d bytes); narrow verification_paths", options.maxFiles, d.Files, d.Bytes)
		}
		f, err := root.Open(name)
		if err != nil {
			return d, err
		}
		opened, err := f.Stat()
		if err != nil {
			f.Close()
			return d, err
		}
		if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
			f.Close()
			return d, invalid("verification file changed while opening")
		}
		digestField(h, []byte("file"))
		content := sha256.New()
		n, readErr := io.Copy(content, io.LimitReader(f, options.maxBytes-d.Bytes+1))
		after, statErr := f.Stat()
		closeErr := f.Close()
		if readErr != nil {
			return d, readErr
		}
		if statErr != nil {
			return d, statErr
		}
		if closeErr != nil {
			return d, closeErr
		}
		d.Bytes += n
		if d.Bytes > options.maxBytes {
			return d, fmt.Errorf("verification exceeds %d bytes (%d files, %d bytes); narrow verification_paths", options.maxBytes, d.Files, d.Bytes)
		}
		current, err := root.Lstat(name)
		if err != nil {
			return d, err
		}
		if !current.Mode().IsRegular() || !os.SameFile(opened, current) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
			return d, invalid("verification file changed during reading; retry")
		}
		digestField(h, []byte(fmt.Sprint(n)))
		digestField(h, content.Sum(nil))
	}
	d.SHA256 = hex.EncodeToString(h.Sum(nil))
	return d, nil
}

// VerificationView binds a digest to the selected execution and Unit.
type VerificationView struct {
	VerificationDigest
	IntentID string `json:"intent_id"`
	StepID   string `json:"step_id"`
	UnitID   string `json:"unit_id,omitempty"`
}

// Hash reads the current Intent or registered Unit content without saving state.
func (s Store) Hash(id, unitID, root string) (VerificationView, error) {
	st, err := s.Read(id)
	if err != nil {
		return VerificationView{}, err
	}
	paths := st.Config.VerificationPaths
	if unitID != "" {
		found := false
		for _, unit := range st.Config.Units {
			if unit.ID == unitID {
				found = true
				if len(unit.VerificationPaths) > 0 {
					paths = unit.VerificationPaths
				}
				break
			}
		}
		if !found {
			return VerificationView{}, invalid("unknown Unit")
		}
	}
	if root != "" {
		if unitID == "" {
			return VerificationView{}, invalid("root requires Unit")
		}
		runtime, err := s.assignment(id, unitID)
		if err != nil {
			return VerificationView{}, err
		}
		canonical, err := filepath.EvalSymlinks(root)
		if err != nil {
			return VerificationView{}, err
		}
		if canonical != runtime.Root {
			return VerificationView{}, invalid("root is not registered to Unit")
		}
		if err := s.managedUnit(st, runtime); err != nil {
			return VerificationView{}, err
		}
	} else {
		root = s.Root
	}
	digest, err := ComputeVerification(root, paths)
	if err != nil {
		return VerificationView{}, err
	}
	return VerificationView{VerificationDigest: digest, IntentID: id, StepID: st.CurrentStepID, UnitID: unitID}, nil
}
func validateVerificationConfig(config Config) error {
	paths, err := verificationPaths(config.VerificationPaths)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, unit := range config.Units {
		if unit.ID == "" || seen[unit.ID] {
			return invalid("invalid or duplicate Unit")
		}
		seen[unit.ID] = true
		if unit.ResultSHA256 != "" && !validHash(unit.ResultSHA256) {
			return invalid("invalid Unit result SHA")
		}
		unitPaths, err := verificationPaths(unit.VerificationPaths)
		if err != nil {
			return err
		}
		for _, p := range unitPaths {
			covered := false
			for _, parent := range paths {
				if parent == "." || p == parent || strings.HasPrefix(p, parent+"/") {
					covered = true
					break
				}
			}
			if !covered {
				return invalid("Unit verification paths must be included in Intent paths")
			}
		}
	}
	return nil
}
