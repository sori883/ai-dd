package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"
)

func Archive(entries map[string][]byte, binary string, windows bool) ([]byte, error) {
	return ArchiveModes(entries, map[string]uint32{binary: 0755}, windows)
}
func ArchiveModes(entries map[string][]byte, modes map[string]uint32, windows bool) ([]byte, error) {
	names := make([]string, 0, len(entries))
	for name := range entries {
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			return nil, fmt.Errorf("invalid archive path %s", name)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	var out bytes.Buffer
	if windows {
		z := zip.NewWriter(&out)
		for _, name := range names {
			h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
			mode := fs.FileMode(0644)
			if modes[name] == 0755 {
				mode = 0755
			}
			h.SetMode(mode)
			w, e := z.CreateHeader(h)
			if e != nil {
				return nil, e
			}
			if _, e = w.Write(entries[name]); e != nil {
				return nil, e
			}
		}
		if e := z.Close(); e != nil {
			return nil, e
		}
	} else {
		g := gzip.NewWriter(&out)
		t := tar.NewWriter(g)
		for _, name := range names {
			mode := int64(0644)
			if modes[name] == 0755 {
				mode = 0755
			}
			if e := t.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(entries[name])), Typeflag: tar.TypeReg}); e != nil {
				return nil, e
			}
			if _, e := t.Write(entries[name]); e != nil {
				return nil, e
			}
		}
		if e := errors.Join(t.Close(), g.Close()); e != nil {
			return nil, e
		}
	}
	return out.Bytes(), nil
}
func ValidSourcePath(name string) bool { return sourcePaths[name] }

func BuildData(version, commit string, common, host fs.FS, productLicense []byte) ([]byte, DataManifest, error) {
	m := DataManifest{SchemaVersion: 1, Version: version, SourceCommit: commit, Archive: "aidlc-assets_" + version + ".tar.gz"}
	if !ValidVersion(version) || !ValidCommit(commit) {
		return nil, m, fmt.Errorf("invalid release identity")
	}
	entries := map[string][]byte{"LICENSES/PRODUCT.txt": productLicense}
	if len(productLicense) == 0 {
		return nil, m, fmt.Errorf("product license required")
	}
	for _, source := range []struct {
		prefix string
		fs     fs.FS
	}{{"core", common}, {"codex", host}} {
		err := fs.WalkDir(source.fs, ".", func(name string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			p := source.prefix + "/" + name
			if !d.Type().IsRegular() || !ValidSourcePath(p) {
				return fmt.Errorf("unsupported source path %s", p)
			}
			raw, e := fs.ReadFile(source.fs, name)
			if e != nil {
				return e
			}
			entries[p] = raw
			return nil
		})
		if err != nil {
			return nil, m, err
		}
	}
	if len(entries) != len(sourcePaths) {
		return nil, m, fmt.Errorf("missing schema 1 source files")
	}
	raw, err := Archive(entries, "", false)
	if err != nil {
		return nil, m, err
	}
	m.ArchiveSHA256 = Hash(raw)
	m.ArchiveSize = int64(len(raw))
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		m.Files = append(m.Files, DataFile{name, Hash(entries[name]), int64(len(entries[name]))})
	}
	return raw, m, nil
}
