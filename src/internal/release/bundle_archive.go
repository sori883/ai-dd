package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// BundlePaths is the closed schema 2 member set, excluding manifest.json.
func BundlePaths(target string) map[string]uint32 {
	paths := make(map[string]uint32)
	for p := range sourcePaths {
		paths[p] = 0644
	}
	paths["README.md"] = 0644
	for _, product := range Products {
		binary := product
		if strings.HasPrefix(target, "windows/") {
			binary += ".exe"
		}
		paths[binary] = 0755
		prefix := "LICENSES/" + product + "/"
		for _, n := range []string{"PRODUCT.txt", "Go-LICENSE.txt", "Go-PATENTS.txt"} {
			paths[prefix+n] = 0644
		}
		if product != "natural-japanese-go" {
			for _, n := range []string{"yaml-LICENSE.txt", "yaml-NOTICE.txt", "Apache-2.0.txt"} {
				paths[prefix+n] = 0644
			}
		}
		if product == "natural-japanese-go" {
			for p := range sourcePaths {
				if strings.HasPrefix(p, "core/skills/natural-japanese-go/licenses/") {
					paths[prefix+path.Base(p)] = 0644
				}
			}
		}
		if product == "aidlc" || product == "aidlc-install" || product == "aidlc-dist" {
			for p := range sourcePaths {
				if strings.HasPrefix(p, "core/skills/") && (path.Base(p) == "LICENSE" || strings.HasSuffix(p, "/references/source.md")) {
					paths[prefix+strings.TrimPrefix(p, "core/")] = 0644
				}
			}
		}
	}
	return paths
}

func ValidateBundleArchive(raw []byte, version, target, digest string) (BundleManifest, map[string][]byte, error) {
	var m BundleManifest
	if int64(len(raw)) > MaxArchiveBytes || !ValidHash(digest) || Hash(raw) != digest {
		return m, nil, fmt.Errorf("bundle checksum/size mismatch")
	}
	entries, modes, err := unpackBundle(raw, strings.HasPrefix(target, "windows/"))
	if err != nil {
		return m, nil, err
	}
	m, err = ParseBundleManifest(entries["manifest.json"], version, target)
	if err != nil {
		return m, nil, err
	}
	want := BundlePaths(target)
	if len(entries) != len(want)+1 || len(m.Files) != len(want) || modes["manifest.json"] != 0644 {
		return m, nil, fmt.Errorf("bundle member set mismatch")
	}
	var sourceSize int64
	for _, f := range m.Files {
		mode, ok := want[f.Path]
		data, exists := entries[f.Path]
		if !ok || !exists || mode != f.Mode || modes[f.Path] != f.Mode || int64(len(data)) != f.Size || Hash(data) != f.SHA256 || len(data) == 0 {
			return m, nil, fmt.Errorf("bundle member mismatch %s", f.Path)
		}
		if (f.Path == "aidlc-install" || f.Path == "aidlc-install.exe") && f.Size > MaxInstallerBytes {
			return m, nil, fmt.Errorf("installer size limit")
		}
		if ValidSourcePath(f.Path) {
			sourceSize += f.Size
		}
	}
	if sourceSize > MaxSourceBytes {
		return m, nil, fmt.Errorf("source limit exceeded")
	}
	return m, entries, nil
}

func unpackBundle(raw []byte, windows bool) (map[string][]byte, map[string]uint32, error) {
	entries := map[string][]byte{}
	modes := map[string]uint32{}
	var total int64
	add := func(name string, size int64, mode fs.FileMode, r io.Reader) error {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || !mode.IsRegular() || (mode.Perm() != 0644 && mode.Perm() != 0755) || mode & ^(fs.ModePerm) != 0 {
			return fmt.Errorf("invalid bundle member %q", name)
		}
		if _, ok := entries[name]; ok {
			return fmt.Errorf("duplicate member %q", name)
		}
		for old := range entries {
			if strings.HasPrefix(old, name+"/") || strings.HasPrefix(name, old+"/") {
				return fmt.Errorf("member parent collision")
			}
		}
		if size < 0 || size > MaxArchiveBytes-total {
			return fmt.Errorf("bundle expansion limit")
		}
		if name == "manifest.json" && size > MaxManifestBytes {
			return fmt.Errorf("manifest limit")
		}
		b, err := io.ReadAll(io.LimitReader(r, size+1))
		if err != nil {
			return err
		}
		if int64(len(b)) != size {
			return fmt.Errorf("member size mismatch")
		}
		total += size
		entries[name] = b
		modes[name] = uint32(mode.Perm())
		return nil
	}
	if windows {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, nil, err
		}
		for _, f := range z.File {
			if f.UncompressedSize64 > uint64(MaxArchiveBytes) {
				return nil, nil, fmt.Errorf("member size limit")
			}
			r, err := f.Open()
			if err != nil {
				return nil, nil, err
			}
			err = add(f.Name, int64(f.UncompressedSize64), f.Mode(), r)
			r.Close()
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		g, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, nil, err
		}
		defer g.Close()
		// Bound the entire decompressed stream, including tar headers and padding.
		limited := &io.LimitedReader{R: g, N: MaxArchiveBytes + 1}
		tr := tar.NewReader(limited)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, nil, err
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return nil, nil, fmt.Errorf("nonregular member")
			}
			if err := add(h.Name, h.Size, h.FileInfo().Mode(), tr); err != nil {
				return nil, nil, err
			}
		}
		padding := make([]byte, 32<<10)
		for {
			n, err := limited.Read(padding)
			for _, b := range padding[:n] {
				if b != 0 {
					return nil, nil, fmt.Errorf("unexpected data after tar end")
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, nil, err
			}
		}
		if limited.N <= 0 {
			return nil, nil, fmt.Errorf("bundle expansion limit")
		}
	}
	return entries, modes, nil
}
