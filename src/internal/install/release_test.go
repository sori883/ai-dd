package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestReleaseDownload(t *testing.T) {
	called := false
	_, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: "v0.1.1", Target: "linux/amd64", fetch: func(context.Context, string) ([]byte, error) { called = true; return nil, errors.New("network down") }})
	if err == nil || !called {
		t.Fatalf("fetch=%v err=%v", called, err)
	}
}
func TestReleaseAssetValidation(t *testing.T) {
	for _, version := range []string{"", "../v1", "v1/other"} {
		t.Run(version, func(t *testing.T) {
			_, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: version, Target: "linux/amd64"})
			if err == nil {
				t.Fatal("unsafe release accepted")
			}
		})
	}
}
func TestInstallFailure(t *testing.T) {
	root := t.TempDir()
	_, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", Directory: t.TempDir()})
	if err == nil {
		t.Fatal("missing candidate accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".codex/hooks.json")); !os.IsNotExist(err) {
		t.Fatal("failed validation wrote assets")
	}
}

func candidateFiles(t *testing.T) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	version, commit := "v0.1.1", strings.Repeat("a", 40)
	for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
		m := release.Manifest{SchemaVersion: 1, Version: version, SourceCommit: commit, GoVersion: "go1.26.4"}
		for _, target := range release.Targets {
			binary, suffix := product, ".tar.gz"
			windows := strings.HasPrefix(target, "windows/")
			if windows {
				binary += ".exe"
				suffix = ".zip"
			}
			body := []byte(product + " " + target)
			raw, err := release.Archive(map[string][]byte{binary: body, "LICENSES/PRODUCT.txt": []byte("license"), "LICENSES/Go-LICENSE.txt": []byte("go"), "LICENSES/Go-PATENTS.txt": []byte("patents")}, binary, windows)
			if err != nil {
				t.Fatal(err)
			}
			name := product + "_" + version + "_" + strings.ReplaceAll(target, "/", "_") + suffix
			files[name] = raw
			m.Artifacts = append(m.Artifacts, release.Artifact{Target: target, Binary: binary, BinarySHA256: release.Hash(body), BinarySize: int64(len(body)), Archive: name, ArchiveSHA256: release.Hash(raw), ArchiveSize: int64(len(raw))})
		}
		name, sums := release.MetadataNames(product)
		files[name], _ = json.Marshal(m)
		lines := []string{release.Hash(files[name]) + "  " + name}
		for _, a := range m.Artifacts {
			lines = append(lines, a.ArchiveSHA256+"  "+a.Archive)
		}
		sort.Slice(lines, func(i, j int) bool {
			return strings.SplitN(lines[i], "  ", 2)[1] < strings.SplitN(lines[j], "  ", 2)[1]
		})
		files[sums] = []byte(strings.Join(lines, "\n") + "\n")
	}
	license, err := os.ReadFile("../../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	data, m, err := release.BuildData(version, commit, core.Files, codex.Files, license)
	if err != nil {
		t.Fatal(err)
	}
	files[m.Archive] = data
	files["aidlc-assets-manifest.json"], _ = json.Marshal(m)
	files["aidlc-assets-SHA256SUMS"] = []byte(release.Hash(files["aidlc-assets-manifest.json"]) + "  aidlc-assets-manifest.json\n" + release.Hash(data) + "  " + m.Archive + "\n")
	return files
}
func candidateFetch(files map[string][]byte) func(context.Context, string) ([]byte, error) {
	return func(_ context.Context, name string) ([]byte, error) {
		raw, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return raw, nil
	}
}
func TestReleaseAssetValidationCandidate(t *testing.T) {
	for _, mode := range []string{"valid", "binary hash", "missing source", "other version", "schema", "extra path"} {
		t.Run(mode, func(t *testing.T) {
			files := candidateFiles(t)
			switch mode {
			case "binary hash":
				files["okf_v0.1.1_linux_amd64.tar.gz"] = []byte("corrupted")
			case "missing source":
				delete(files, "aidlc-assets_v0.1.1.tar.gz")
			case "other version", "schema", "extra path":
				var m release.DataManifest
				json.Unmarshal(files["aidlc-assets-manifest.json"], &m)
				if mode == "other version" {
					m.Version = "v0.1.2"
				}
				if mode == "schema" {
					m.SchemaVersion = 2
				}
				if mode == "extra path" {
					m.Files = append(m.Files, release.DataFile{Path: "core/../state.json", SHA256: strings.Repeat("a", 64), Size: 1})
				}
				files["aidlc-assets-manifest.json"], _ = json.Marshal(m)
				files["aidlc-assets-SHA256SUMS"] = []byte(release.Hash(files["aidlc-assets-manifest.json"]) + "  aidlc-assets-manifest.json\n" + release.Hash(files[m.Archive]) + "  " + m.Archive + "\n")
			}
			root := t.TempDir()
			r, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(files)})
			if mode != "valid" {
				if err == nil || len(r.Paths) != 0 {
					t.Fatalf("invalid release: %+v %v", r, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Paths) != 83 {
				t.Fatalf("installed %d files", len(r.Paths))
			}
			for _, name := range []string{"aidlc", "okf", "natural-japanese-go"} {
				raw, err := os.ReadFile(filepath.Join(root, "aidlc/bin/v0.1.1", name))
				if err != nil || string(raw) != name+" linux/amd64" {
					t.Fatalf("%s %s %v", name, raw, err)
				}
			}
		})
	}
}
func TestInstallReservation(t *testing.T) {
	root := t.TempDir()
	entered, finish := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: func(context.Context, string) ([]byte, error) {
			close(entered)
			<-finish
			return nil, errors.New("stop")
		}})
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	_, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(candidateFiles(t))})
	close(finish)
	<-done
	if err == nil {
		t.Fatal("concurrent install accepted")
	}
}
func TestInstallFailurePartial(t *testing.T) {
	count := 0
	r, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(candidateFiles(t)), write: func(root, path string, raw []byte, mode uint32) error {
		count++
		if count == 2 {
			return errors.New("disk full")
		}
		return os.WriteFile(filepath.Join(root, path), raw, os.FileMode(mode))
	}})
	if err == nil || len(r.Paths) != 1 || len(r.Pending) == 0 {
		t.Fatalf("partial result %+v %v", r, err)
	}
}

func TestReleaseDownloadHTTP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		length string
		fail   bool
	}{{"ok", 200, "", false}, {"not found", 404, "", true}, {"oversized", 200, "9999999999", true}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.length != "" {
					w.Header().Set("Content-Length", tc.length)
				}
				w.WriteHeader(tc.status)
				w.Write([]byte("asset"))
			}))
			defer server.Close()
			raw, err := download(context.Background(), server.URL)
			if (err != nil) != tc.fail {
				t.Fatalf("%s %v", raw, err)
			}
		})
	}
}
func TestReleaseAssetValidationUnsafeArchives(t *testing.T) {
	for _, mode := range []string{"escape", "duplicate", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			var b bytes.Buffer
			g := gzip.NewWriter(&b)
			tr := tar.NewWriter(g)
			h := &tar.Header{Name: "aidlc", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}
			if mode == "escape" {
				h.Name = "../aidlc"
			}
			if mode == "symlink" {
				h.Typeflag = tar.TypeSymlink
				h.Linkname = "/outside"
				h.Size = 0
			}
			tr.WriteHeader(h)
			if h.Size == 1 {
				tr.Write([]byte("x"))
			}
			if mode == "duplicate" {
				tr.WriteHeader(h)
				tr.Write([]byte("x"))
			}
			tr.Close()
			g.Close()
			if _, err := release.Unpack(b.Bytes(), false, 1024); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
func TestReleaseAssetValidationOffline(t *testing.T) {
	files := candidateFiles(t)
	dir := t.TempDir()
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: "v0.1.1", Target: "linux/amd64", Directory: dir})
	if err != nil || len(r.Paths) != 83 {
		t.Fatalf("offline %+v %v", r, err)
	}
}

func TestInstallerCommandRelocation(t *testing.T) {
	parent := t.TempDir()
	old := filepath.Join(parent, "old")
	if err := os.Mkdir(old, 0755); err != nil {
		t.Fatal(err)
	}
	files := candidateFiles(t)
	r, err := InstallRelease(context.Background(), ReleaseOptions{Root: old, Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(files)})
	if err != nil {
		t.Fatal(err)
	}
	old, err = filepath.EvalSymlinks(old)
	if err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(parent, "new")
	if err := os.Rename(old, next); err != nil {
		t.Fatal(err)
	}
	out, err := InstallRelease(context.Background(), ReleaseOptions{Root: next, Version: "v0.1.1", Target: "linux/amd64", Relocate: true, FromProjectDir: old, FromBinary: r.Binaries.AIDLC, fetch: candidateFetch(files)})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Paths) != 6 {
		t.Fatalf("relocated files %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(next, ".codex/hooks.json"))
	if err != nil || bytes.Contains(raw, []byte(old)) {
		t.Fatalf("old hook path remains %s %v", raw, err)
	}
}

func TestReleaseLicenseRetention(t *testing.T) {
	for _, mode := range []string{"fresh", "collision", "partial", "relocate", "tampered", "missing"} {
		t.Run(mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			o := ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(candidateFiles(t))}
			path := "aidlc/bin/v0.1.1/licenses/okf/PRODUCT.txt"
			if mode == "collision" {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, path), []byte("user"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "partial" {
				o.write = func(root, path string, data []byte, mode uint32) error {
					if strings.HasPrefix(path, "aidlc/bin/v0.1.1/licenses/") {
						return errors.New("license write failed")
					}
					return os.WriteFile(filepath.Join(root, path), data, os.FileMode(mode))
				}
			}
			r, err := InstallRelease(t.Context(), o)
			if mode == "collision" {
				if err == nil || len(r.Paths) != 0 {
					t.Fatal("license collision accepted", r, err)
				}
				return
			}
			if mode == "partial" {
				if err == nil || len(r.Paths) == 0 || len(r.Pending) == 0 || !strings.HasPrefix(r.Pending[0], "aidlc/bin/v0.1.1/licenses/") {
					t.Fatal("license failure lost partial result", r, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
				raw, err := os.ReadFile(filepath.Join(root, "aidlc/bin/v0.1.1/licenses", product, "PRODUCT.txt"))
				if err != nil || string(raw) != "license" {
					t.Fatal("missing exact release license", product, string(raw), err)
				}
			}
			if mode == "fresh" {
				return
			}
			if mode == "tampered" {
				if err := os.WriteFile(filepath.Join(root, path), []byte("changed"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "missing" {
				if err := os.Remove(filepath.Join(root, path)); err != nil {
					t.Fatal(err)
				}
			}
			o.Relocate = true
			o.FromProjectDir = root
			o.FromBinary = r.Binaries.AIDLC
			r, err = InstallRelease(t.Context(), o)
			if mode == "tampered" || mode == "missing" {
				if err == nil || len(r.Paths) != 0 {
					t.Fatal("modified license accepted", r, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
