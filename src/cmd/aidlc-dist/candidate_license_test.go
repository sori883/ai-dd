package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/release"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestCandidateLicense(t *testing.T) {
	for _, tc := range []struct{ product, path string }{
		{"okf", "LICENSES/yaml-NOTICE.txt"},
		{"natural-japanese-go", "LICENSES/UniDic-NOTICE.txt"},
		{"aidlc", "LICENSES/skills/okf-agent-memory/LICENSE"},
		{"aidlc", "LICENSES/skills/tdd/references/source.md"},
	} {
		for _, mutation := range []string{"missing", "modified", "extra"} {
			t.Run(tc.product+"/"+tc.path+"/"+mutation, func(t *testing.T) {
				o := releaseFixture(t)
				if err := packageArchives(o); err != nil {
					t.Fatal(err)
				}
				name := release.BundleName(o.Version, "linux/amd64")
				raw, err := os.ReadFile(filepath.Join(o.OutputDir, name))
				if err != nil {
					t.Fatal(err)
				}
				entries, err := release.Unpack(raw, false, release.MaxArchiveBytes)
				if err != nil {
					t.Fatal(err)
				}
				if err := validateCandidateLicenses(tc.product, raw, false); err != nil {
					t.Fatal("valid candidate", err)
				}
				switch mutation {
				case "missing":
					delete(entries, "LICENSES/"+tc.product+"/"+strings.TrimPrefix(tc.path, "LICENSES/"))
				case "modified":
					entries["LICENSES/"+tc.product+"/"+strings.TrimPrefix(tc.path, "LICENSES/")] = []byte("changed")
				case "extra":
					entries["LICENSES/"+tc.product+"/extra.txt"] = []byte("unexpected")
				}
				bad, err := release.ArchiveModes(entries, release.BundlePaths("linux/amd64"), false)
				if err != nil {
					t.Fatal(err)
				}
				if err := validateCandidateLicenses(tc.product, bad, false); err == nil {
					t.Fatal("incorrect candidate license accepted")
				}
			})
		}
	}
}

// Read source files independently of the packaging implementation. The release
// entry point already requires this checkout's commit and build Go version.
func validateCandidateLicenses(product string, raw []byte, windows bool) error {
	entries, err := release.Unpack(raw, windows, release.MaxArchiveBytes)
	if err != nil {
		return err
	}
	bundled := entries["manifest.json"] != nil
	if bundled {
		selected := map[string][]byte{}
		binary := product
		if windows {
			binary += ".exe"
		}
		selected[binary] = entries[binary]
		if product == "natural-japanese-go" {
			selected["README.md"] = entries["README.md"]
		}
		prefix := "LICENSES/" + product + "/"
		for p, b := range entries {
			if strings.HasPrefix(p, prefix) {
				selected["LICENSES/"+strings.TrimPrefix(p, prefix)] = b
			}
		}
		entries = selected
	}
	paths := map[string]string{"LICENSES/PRODUCT.txt": "../../../LICENSE", "LICENSES/Go-LICENSE.txt": filepath.Join(runtime.GOROOT(), "LICENSE"), "LICENSES/Go-PATENTS.txt": filepath.Join(runtime.GOROOT(), "PATENTS")}
	if product != "natural-japanese-go" {
		for _, name := range []string{"yaml-LICENSE.txt", "yaml-NOTICE.txt", "Apache-2.0.txt"} {
			paths["LICENSES/"+name] = "../../distribution/licenses/" + name
		}
	}
	if product == "natural-japanese-go" {
		for _, name := range []string{"natural-japanese.txt", "kagome.txt", "kagome-dict.txt", "uni.txt", "UniDic-NOTICE.txt"} {
			paths["LICENSES/"+name] = "../../core/skills/natural-japanese-go/licenses/" + name
		}
	}
	if product == "aidlc" || product == "aidlc-install" || product == "aidlc-dist" {
		for _, skill := range []string{"architecture", "code-review", "domain-modeling", "grill-with-docs", "grilling", "natural-japanese-go", "okf-agent-memory", "planning", "research", "systematic-debugging", "tdd", "to-spec", "verification-before-completion"} {
			for _, name := range []string{"LICENSE", "references/source.md"} {
				paths["LICENSES/skills/"+skill+"/"+name] = "../../core/skills/" + skill + "/" + name
			}
		}
	}
	binary := product
	if windows {
		binary += ".exe"
	}
	checkMode := func(name string, mode fs.FileMode) error {
		want := fs.FileMode(0644)
		if name == binary || (bundled && slices.Contains(release.Products, strings.TrimSuffix(name, ".exe"))) {
			want = 0755
		}
		if mode != want {
			return fmt.Errorf("archive mode differs: %s %v", name, mode)
		}
		return nil
	}
	if windows {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return err
		}
		for _, f := range z.File {
			if err := checkMode(f.Name, f.Mode()); err != nil {
				return err
			}
		}
	} else {
		g, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return err
		}
		defer g.Close()
		tr := tar.NewReader(g)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if err := checkMode(h.Name, h.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}
	expectedCount := len(paths) + 1
	if product == "natural-japanese-go" {
		expectedCount++
		want, err := os.ReadFile("../../core/skills/natural-japanese-go/references/cli.md")
		if err != nil {
			return err
		}
		if !bytes.Equal(entries["README.md"], []byte(strings.ReplaceAll(string(want), "@@NATURAL_BINARY@@", "natural-japanese-go"))) {
			return fmt.Errorf("README differs")
		}
	}
	if len(entries) != expectedCount || len(entries[binary]) == 0 {
		return fmt.Errorf("archive entry set differs")
	}
	for name, source := range paths {
		want, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if !bytes.Equal(entries[name], want) {
			return fmt.Errorf("license differs from source: %s", name)
		}
	}
	return nil
}

func TestCandidateLicenseExecutableMode(t *testing.T) {
	for _, windows := range []bool{false, true} {
		t.Run(fmt.Sprint(windows), func(t *testing.T) {
			o := releaseFixture(t)
			if err := packageArchives(o); err != nil {
				t.Fatal(err)
			}
			name := release.BundleName(o.Version, "linux/amd64")
			if windows {
				name = release.BundleName(o.Version, "windows/amd64")
			}
			raw, err := os.ReadFile(filepath.Join(o.OutputDir, name))
			if err != nil {
				t.Fatal(err)
			}
			entries, err := release.Unpack(raw, windows, release.MaxArchiveBytes)
			if err != nil {
				t.Fatal(err)
			}
			var buffer bytes.Buffer
			if windows {
				w := zip.NewWriter(&buffer)
				for name, data := range entries {
					h := zip.FileHeader{Name: name, Method: zip.Deflate}
					h.SetMode(0644)
					f, err := w.CreateHeader(&h)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = f.Write(data); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				g := gzip.NewWriter(&buffer)
				w := tar.NewWriter(g)
				for name, data := range entries {
					if err := w.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write(data); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				if err := g.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := validateCandidateLicenses("okf", buffer.Bytes(), windows); err == nil {
				t.Fatal("non-executable candidate accepted")
			}
		})
	}
}
