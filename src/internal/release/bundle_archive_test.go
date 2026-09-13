package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

// A manifest alone must never authorize an arbitrary executable or incomplete bundle.
func TestBundleArchiveRejectsIncomplete(t *testing.T) {
	entries := map[string][]byte{"aidlc": []byte("binary")}
	m := BundleManifest{2, "v0.1.2", strings.Repeat("a", 40), "go1.26.4", "linux/amd64", []BundleFile{{"aidlc", 6, Hash(entries["aidlc"]), 0755}}}
	entries["manifest.json"], _ = json.Marshal(m)
	raw, err := Archive(entries, "aidlc", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ValidateBundleArchive(raw, m.Version, m.Target, Hash(raw)); err == nil {
		t.Fatal("incomplete bundle accepted")
	}
}
func TestBundleArchiveRejectsUnsafe(t *testing.T) {
	for _, windows := range []bool{false, true} {
		for _, mode := range []string{"valid", "duplicate", "symlink", "hardlink", "parent collision", "reverse collision", "bad mode", "parent path", "absolute path", "backslash", "special"} {
			if windows && mode == "hardlink" {
				continue
			} // ZIP has no hard-link member type.
			t.Run(fmt.Sprintf("zip=%v/%s", windows, mode), func(t *testing.T) {
				names := []string{"a"}
				switch mode {
				case "duplicate":
					names = []string{"a", "a"}
				case "parent collision":
					names = []string{"a", "a/b"}
				case "reverse collision":
					names = []string{"a/b", "a"}
				case "parent path":
					names = []string{"../a"}
				case "absolute path":
					names = []string{"/a"}
				case "backslash":
					names = []string{`a\b`}
				}
				var b bytes.Buffer
				if windows {
					w := zip.NewWriter(&b)
					for _, name := range names {
						h := &zip.FileHeader{Name: name, Method: zip.Deflate}
						perm := fs.FileMode(0644)
						switch mode {
						case "bad mode":
							perm = 0777
						case "symlink":
							perm |= fs.ModeSymlink
						case "special":
							perm |= fs.ModeNamedPipe
						}
						h.SetMode(perm)
						f, err := w.CreateHeader(h)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := f.Write([]byte("payload")); err != nil {
							t.Fatal(err)
						}
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					g := gzip.NewWriter(&b)
					w := tar.NewWriter(g)
					for _, name := range names {
						h := &tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: 7}
						switch mode {
						case "bad mode":
							h.Mode = 0777
						case "symlink":
							h.Typeflag = tar.TypeSymlink
							h.Linkname = "outside"
							h.Size = 0
						case "hardlink":
							h.Typeflag = tar.TypeLink
							h.Linkname = "outside"
							h.Size = 0
						case "special":
							h.Typeflag = tar.TypeFifo
							h.Size = 0
						}
						if err := w.WriteHeader(h); err != nil {
							t.Fatal(err)
						}
						if h.Size > 0 {
							if _, err := w.Write([]byte("payload")); err != nil {
								t.Fatal(err)
							}
						}
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
					if err := g.Close(); err != nil {
						t.Fatal(err)
					}
				}
				// Observe only the unpacker, without a manifest parser masking acceptance.
				entries, modes, err := unpackBundle(b.Bytes(), windows)
				if mode == "valid" {
					if err != nil || len(entries) != 1 || string(entries["a"]) != "payload" || modes["a"] != 0644 {
						t.Fatal("normal archive must unpack", entries, modes, err)
					}
				} else if err == nil {
					t.Fatal("unsafe member accepted", mode)
				}
			})
		}
	}
}

func TestBundleArchiveComplete(t *testing.T) {
	for _, change := range []string{"valid", "missing", "unknown", "size", "hash", "mode", "source limit", "installer limit", "trailing", "transport hash"} {
		t.Run(change, func(t *testing.T) {
			target := "linux/amd64"
			entries := map[string][]byte{}
			modes := BundlePaths(target)
			for p := range modes {
				entries[p] = []byte("fixture")
			}
			if change == "missing" {
				delete(entries, "okf")
			}
			if change == "unknown" {
				entries["unexpected"] = []byte("x")
				modes["unexpected"] = 0644
			}
			if change == "installer limit" {
				entries["aidlc-install"] = bytes.Repeat([]byte("x"), MaxInstallerBytes+1)
			}
			if change == "source limit" {
				entries["core/adr-template.md"] = bytes.Repeat([]byte("x"), int(MaxSourceBytes)+1)
			}
			names := make([]string, 0, len(entries))
			for p := range entries {
				names = append(names, p)
			}
			slices.Sort(names)
			m := BundleManifest{2, "v0.1.2", strings.Repeat("a", 40), "go1.26.4", target, nil}
			for _, p := range names {
				m.Files = append(m.Files, BundleFile{p, int64(len(entries[p])), Hash(entries[p]), modes[p]})
			}
			switch change {
			case "size":
				m.Files[0].Size++
			case "hash":
				m.Files[0].SHA256 = strings.Repeat("0", 64)
			case "mode":
				modes["aidlc"] = 0644
			}
			entries["manifest.json"], _ = json.Marshal(m)
			names = append(names, "manifest.json")
			modes["manifest.json"] = 0644
			var b bytes.Buffer
			g := gzip.NewWriter(&b)
			tw := tar.NewWriter(g)
			for _, p := range names {
				if err := tw.WriteHeader(&tar.Header{Name: p, Size: int64(len(entries[p])), Mode: int64(modes[p]), Typeflag: tar.TypeReg}); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write(entries[p]); err != nil {
					t.Fatal(err)
				}
			}
			tw.Close()
			if change == "trailing" {
				g.Write([]byte("unexpected trailing member"))
			}
			g.Close()
			digest := Hash(b.Bytes())
			if change == "transport hash" {
				digest = strings.Repeat("0", 64)
			}
			got, files, err := ValidateBundleArchive(b.Bytes(), m.Version, target, digest)
			if change == "valid" {
				if err != nil || got.Version != m.Version || len(files) != len(entries) {
					t.Fatal(got.Version, len(files), err)
				}
			} else if err == nil {
				t.Fatal("invalid bundle accepted", change)
			}
		})
	}
}
