package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
	original := candidateFiles(t)
	for _, mode := range []string{"valid", "binary hash", "other version"} {
		t.Run(mode, func(t *testing.T) {
			files := maps.Clone(original)
			switch mode {
			case "binary hash":
				files["ai-dd_v0.1.1_linux_amd64.tar.gz"] = []byte("corrupted")
			case "other version":
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
				entries["manifest.json"], _ = json.Marshal(m)
				old := release.Hash(files[name])
				files[name], err = release.ArchiveModes(entries, release.BundlePaths("linux/amd64"), false)
				if err != nil {
					t.Fatal(err)
				}
				files["SHA256SUMS"] = []byte(strings.ReplaceAll(string(files["SHA256SUMS"]), old, release.Hash(files[name])))
			}
			root := t.TempDir()
			calls := []string{}
			r, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: func(ctx context.Context, name string) ([]byte, error) {
				calls = append(calls, name)
				return candidateFetch(files)(ctx, name)
			}})
			if mode != "valid" {
				if err == nil || len(r.Paths) != 0 {
					t.Fatalf("invalid release: %+v %v", r, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(calls, ",") != "SHA256SUMS,ai-dd_v0.1.1_linux_amd64.tar.gz" {
				t.Fatal("expected two downloads", calls)
			}
			for _, absent := range []string{"aidlc-install", "aidlc-dist"} {
				if _, err := os.Lstat(filepath.Join(root, "aidlc/bin/v0.1.1", absent)); !os.IsNotExist(err) {
					t.Fatal("non-runtime installed", absent, err)
				}
			}
			for _, required := range []string{".codex/hooks.json", ".agents/skills/aidlc/SKILL.md", ".agents/skills/okf-agent-memory/SKILL.md", ".agents/skills/natural-japanese-go/SKILL.md"} {
				raw, err := os.ReadFile(filepath.Join(root, required))
				if err != nil || len(raw) == 0 || !slices.Contains(r.Paths, required) {
					t.Fatal("missing release asset", required, err)
				}
			}
			for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
				name := "aidlc/bin/v0.1.1/licenses/" + product + "/PRODUCT.txt"
				raw, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(raw) != "license" || !slices.Contains(r.Paths, name) {
					t.Fatal("missing exact release license", name, err)
				}
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
	called := false
	_, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: func(context.Context, string) ([]byte, error) {
		called = true
		return nil, errors.New("unexpected fetch")
	}})
	close(finish)
	firstErr := <-done
	if err == nil || called || firstErr == nil {
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
func TestReleaseAssetValidationOffline(t *testing.T) {
	files := candidateFiles(t)
	for _, mode := range []string{"valid", "missing"} {
		t.Run(mode, func(t *testing.T) {
			dir, root := t.TempDir(), t.TempDir()
			if mode == "valid" {
				for _, name := range []string{"SHA256SUMS", "ai-dd_v0.1.1_linux_amd64.tar.gz"} {
					if err := os.WriteFile(filepath.Join(dir, name), files[name], 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			r, err := InstallRelease(context.Background(), ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", Directory: dir})
			if mode == "missing" {
				if err == nil || len(r.Paths) != 0 {
					t.Fatal("missing candidate accepted", r, err)
				}
				if _, err := os.Lstat(filepath.Join(root, ".codex/hooks.json")); !os.IsNotExist(err) {
					t.Fatal("validation wrote assets", err)
				}
			} else if err != nil || !slices.Contains(r.Paths, "aidlc/bin/v0.1.1/aidlc") {
				t.Fatalf("offline %+v %v", r, err)
			}
		})
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
	files := candidateFiles(t)
	for _, mode := range []string{"collision", "partial", "relocate", "tampered", "missing"} {
		t.Run(mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			o := ReleaseOptions{Root: root, Version: "v0.1.1", Target: "linux/amd64", fetch: candidateFetch(files)}
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

func TestReleaseDownloadRedirectMustRemainHTTPS(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("payload")) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer redirect.Close()
	if _, err := download(context.Background(), redirect.URL); err == nil {
		t.Fatal("HTTP redirect accepted")
	}
}
