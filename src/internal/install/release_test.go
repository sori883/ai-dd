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
	version, commit := "v0.1.1", strings.Repeat("a", 40)
	license, err := os.ReadFile("../../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := release.BuildData(version, commit, core.Files, codex.Files, license)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := release.Unpack(data, false, release.MaxSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	var lines []string
	for _, target := range release.Targets {
		entries := map[string][]byte{}
		modes := release.BundlePaths(target)
		for p := range modes {
			entries[p] = []byte("license")
		}
		for p, b := range sources {
			entries[p] = b
		}
		for _, product := range release.Products {
			binary := product
			if strings.HasPrefix(target, "windows/") {
				binary += ".exe"
			}
			entries[binary] = []byte(product + " " + target)
		}
		m := release.BundleManifest{SchemaVersion: 2, Version: version, SourceCommit: commit, GoVersion: "go1.26.4", Target: target}
		var names []string
		for p := range entries {
			names = append(names, p)
		}
		sort.Strings(names)
		for _, p := range names {
			m.Files = append(m.Files, release.BundleFile{Path: p, Size: int64(len(entries[p])), SHA256: release.Hash(entries[p]), Mode: modes[p]})
		}
		entries["manifest.json"], _ = json.Marshal(m)
		raw, err := release.ArchiveModes(entries, modes, strings.HasPrefix(target, "windows/"))
		if err != nil {
			t.Fatal(err)
		}
		name := release.BundleName(version, target)
		files[name] = raw
		lines = append(lines, release.Hash(raw)+"  "+name)
	}
	files["SHA256SUMS"] = []byte(strings.Join(lines, "\n") + "\n")
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
				files["ai-dd_v0.1.1_linux_amd64.tar.gz"] = []byte("corrupted")
			case "missing source":
				delete(files, "ai-dd_v0.1.1_linux_amd64.tar.gz")
			case "other version", "schema", "extra path":
				name := "ai-dd_v0.1.1_linux_amd64.tar.gz"
				entries, err := release.Unpack(files[name], false, release.MaxArchiveBytes)
				if err != nil {
					t.Fatal(err)
				}
				var m release.BundleManifest
				json.Unmarshal(entries["manifest.json"], &m)
				if mode == "other version" {
					m.Version = "v0.1.2"
				}
				if mode == "schema" {
					m.SchemaVersion = 1
				}
				if mode == "extra path" {
					m.Files = append(m.Files, release.BundleFile{Path: "core/../state.json", SHA256: strings.Repeat("a", 64), Size: 1, Mode: 0644})
				}
				entries["manifest.json"], _ = json.Marshal(m)
				old := release.Hash(files[name])
				files[name], err = release.ArchiveModes(entries, release.BundlePaths("linux/amd64"), false)
				if err != nil {
					t.Fatal(err)
				}
				files["SHA256SUMS"] = []byte(strings.ReplaceAll(string(files["SHA256SUMS"]), old, release.Hash(files[name])))
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
			if len(r.Paths) != 120 {
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
		if name != "SHA256SUMS" && name != "ai-dd_v0.1.1_linux_amd64.tar.gz" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: "v0.1.1", Target: "linux/amd64", Directory: dir})
	if err != nil || len(r.Paths) != 120 {
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

func TestReleaseBundleDownloads(t *testing.T) {
	files := candidateFiles(t)
	var calls []string
	_, err := InstallRelease(context.Background(), ReleaseOptions{Root: t.TempDir(), Version: "v0.1.1", Target: "linux/amd64", fetch: func(ctx context.Context, name string) ([]byte, error) {
		calls = append(calls, name)
		return candidateFetch(files)(ctx, name)
	}})
	if err != nil {
		t.Fatal("complete bundle must install", err)
	}
	if strings.Join(calls, ",") != "SHA256SUMS,ai-dd_v0.1.1_linux_amd64.tar.gz" {
		t.Fatal("expected only two downloads", calls)
	}
}

func TestReleaseDownloadRedirectMustRemainHTTPS(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("payload")) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer redirect.Close()
	if _, err := download(context.Background(), redirect.URL); err == nil {
		t.Fatal("HTTP redirect accepted")
	}
}
