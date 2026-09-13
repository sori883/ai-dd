package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

const MaxArchiveBytes int64 = 512 << 20
const MaxSourceBytes int64 = 32 << 20

func StrictJSON(raw []byte, value any) error {
	// Reject duplicate object keys as well as unknown fields and trailing values.
	d := json.NewDecoder(bytes.NewReader(raw))
	var scan func(int) error
	scan = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("JSON nesting too deep")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return e
					}
					k, ok := key.(string)
					if !ok || seen[k] {
						return fmt.Errorf("duplicate/invalid JSON key")
					}
					seen[k] = true
					if e := scan(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e := scan(depth + 1); e != nil {
						return e
					}
				}
			default:
				return fmt.Errorf("unexpected delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if err := scan(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(value)
}
func ValidHash(s string) bool { return regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s) }
func VerifySums(raw []byte, want map[string]string) error {
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(want) {
		return fmt.Errorf("checksum set differs")
	}
	previous := ""
	for _, line := range lines {
		hash, name, ok := strings.Cut(line, "  ")
		if !ok || !ValidHash(hash) || name <= previous || want[name] != hash {
			return fmt.Errorf("invalid checksum line")
		}
		previous = name
	}
	return nil
}
func ValidateManifest(raw, sums []byte, product, version, target string) (Manifest, Artifact, error) {
	var m Manifest
	var selected Artifact
	fail := func() (Manifest, Artifact, error) {
		return m, selected, fmt.Errorf("invalid binary manifest for %s", product)
	}
	if err := StrictJSON(raw, &m); err != nil {
		return m, selected, err
	}
	if m.SchemaVersion != 1 || m.Version != version || !ValidCommit(m.SourceCommit) || !regexp.MustCompile(`^go1\.[0-9]+\.[0-9]+$`).MatchString(m.GoVersion) || len(m.Artifacts) != len(Targets) {
		return fail()
	}
	manifestName, _ := MetadataNames(product)
	expected := map[string]string{manifestName: Hash(raw)}
	seen := map[string]bool{}
	for _, a := range m.Artifacts {
		binary, suffix := product, ".tar.gz"
		if strings.HasPrefix(a.Target, "windows/") {
			binary += ".exe"
			suffix = ".zip"
		}
		name := product + "_" + version + "_" + strings.ReplaceAll(a.Target, "/", "_") + suffix
		if !slices.Contains(Targets, a.Target) || seen[a.Target] || a.Binary != binary || a.Archive != name || !ValidHash(a.BinarySHA256) || !ValidHash(a.ArchiveSHA256) || a.BinarySize <= 0 || a.BinarySize > MaxArchiveBytes || a.ArchiveSize <= 0 || a.ArchiveSize > MaxArchiveBytes {
			return fail()
		}
		seen[a.Target] = true
		expected[a.Archive] = a.ArchiveSHA256
		if a.Target == target {
			selected = a
		}
	}
	if selected.Target == "" {
		return fail()
	}
	if err := VerifySums(sums, expected); err != nil {
		return m, selected, err
	}
	return m, selected, nil
}
func Unpack(raw []byte, windows bool, maxBytes int64) (map[string][]byte, error) {
	entries := map[string][]byte{}
	var total int64
	add := func(name string, size int64, mode fs.FileMode, r io.Reader) error {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || !mode.IsRegular() || size < 0 || size > maxBytes-total {
			return fmt.Errorf("unsafe or oversized archive member %s", name)
		}
		if _, ok := entries[name]; ok {
			return fmt.Errorf("duplicate archive member")
		}
		data, err := io.ReadAll(io.LimitReader(r, size+1))
		if err != nil {
			return err
		}
		if int64(len(data)) != size {
			return fmt.Errorf("archive member size mismatch")
		}
		total += size
		entries[name] = data
		return nil
	}
	if windows {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, err
		}
		for _, file := range z.File {
			if file.UncompressedSize64 > uint64(maxBytes) {
				return nil, fmt.Errorf("oversized zip entry")
			}
			r, err := file.Open()
			if err != nil {
				return nil, err
			}
			err = add(file.Name, int64(file.UncompressedSize64), file.Mode(), r)
			closeErr := r.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	} else {
		g, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer g.Close()
		t := tar.NewReader(g)
		for {
			h, err := t.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return nil, fmt.Errorf("nonregular tar entry")
			}
			if err := add(h.Name, h.Size, h.FileInfo().Mode(), t); err != nil {
				return nil, err
			}
		}
		if _, err := io.Copy(io.Discard, io.LimitReader(g, 1)); err != nil {
			return nil, err
		}
	}
	return entries, nil
}
func ValidateBinary(raw []byte, a Artifact) ([]byte, error) {
	if int64(len(raw)) != a.ArchiveSize || Hash(raw) != a.ArchiveSHA256 {
		return nil, fmt.Errorf("binary archive checksum/size mismatch")
	}
	entries, err := Unpack(raw, strings.HasPrefix(a.Target, "windows/"), MaxArchiveBytes)
	if err != nil {
		return nil, err
	}
	binary, ok := entries[a.Binary]
	if !ok || int64(len(binary)) != a.BinarySize || Hash(binary) != a.BinarySHA256 {
		return nil, fmt.Errorf("binary checksum/size mismatch")
	}
	for name := range entries {
		if name == a.Binary || name == "README.md" {
			continue
		}
		if !strings.HasPrefix(name, "LICENSES/") || (!strings.HasSuffix(name, ".txt") && !strings.HasSuffix(name, ".md") && path.Base(name) != "LICENSE") {
			return nil, fmt.Errorf("unknown binary archive member")
		}
	}
	for _, name := range []string{"LICENSES/PRODUCT.txt", "LICENSES/Go-LICENSE.txt", "LICENSES/Go-PATENTS.txt"} {
		if len(entries[name]) == 0 {
			return nil, fmt.Errorf("missing binary license")
		}
	}
	return binary, nil
}
func ValidateData(manifestRaw, sums, raw []byte, version, commit string) (map[string][]byte, error) {
	var m DataManifest
	if err := StrictJSON(manifestRaw, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion != 1 || m.Version != version || m.SourceCommit != commit || m.Archive != "aidlc-assets_"+version+".tar.gz" || m.ArchiveSize <= 0 || m.ArchiveSize > MaxSourceBytes || int64(len(raw)) != m.ArchiveSize || Hash(raw) != m.ArchiveSHA256 || len(m.Files) != len(sourcePaths) {
		return nil, fmt.Errorf("invalid source asset manifest/schema; update installer if schema differs")
	}
	if err := VerifySums(sums, map[string]string{"aidlc-assets-manifest.json": Hash(manifestRaw), m.Archive: m.ArchiveSHA256}); err != nil {
		return nil, err
	}
	entries, err := Unpack(raw, false, MaxSourceBytes)
	if err != nil {
		return nil, err
	}
	if len(entries) != len(m.Files) {
		return nil, fmt.Errorf("source asset count mismatch")
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		data, ok := entries[f.Path]
		if !ValidSourcePath(f.Path) || seen[f.Path] || !ok || int64(len(data)) != f.Size || Hash(data) != f.SHA256 {
			return nil, fmt.Errorf("source asset mismatch: %s", f.Path)
		}
		seen[f.Path] = true
	}
	return entries, nil
}
