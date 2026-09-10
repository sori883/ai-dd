package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixtureTargets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}

func archiveFixture(t *testing.T) options {
	t.Helper()
	input := t.TempDir()
	for _, target := range fixtureTargets {
		name := "aidlc-" + strings.ReplaceAll(target, "/", "-")
		if strings.HasPrefix(target, "windows/") {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(input, name), []byte("binary for "+target), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return options{InputDir: input, OutputDir: filepath.Join(t.TempDir(), "candidate"), Version: "dev-abcdef0", Commit: strings.Repeat("a", 40), GoVersion: "go1.26.4", Targets: append([]string{}, fixtureTargets...)}
}
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestArchiveLayout(t *testing.T) {
	t.Parallel()
	o := archiveFixture(t)
	if err := packageArchives(o); err != nil {
		t.Fatal(err)
	}
	for _, target := range fixtureTargets {
		t.Run(target, func(t *testing.T) {
			stem := "aidlc_" + o.Version + "_" + strings.ReplaceAll(target, "/", "_")
			want := []byte("binary for " + target)
			if strings.HasPrefix(target, "windows/") {
				z, err := zip.OpenReader(filepath.Join(o.OutputDir, stem+".zip"))
				if err != nil {
					t.Fatal(err)
				}
				defer z.Close()
				if len(z.File) == 1 && z.File[0].Modified.Year() < 1980 {
					t.Fatal("invalid DOS ZIP timestamp", z.File[0].Modified)
				}
				if len(z.File) != 1 || z.File[0].Name != "aidlc.exe" || !z.File[0].Mode().IsRegular() {
					t.Fatalf("zip layout: %+v", z.File)
				}
				r, err := z.File[0].Open()
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				got, err := io.ReadAll(r)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("zip payload %q %v", got, err)
				}
			} else {
				raw := mustRead(t, filepath.Join(o.OutputDir, stem+".tar.gz"))
				gz, err := gzip.NewReader(bytes.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				defer gz.Close()
				tr := tar.NewReader(gz)
				h, err := tr.Next()
				if err != nil {
					t.Fatal(err)
				}
				if h.Name != "aidlc" || h.Typeflag != tar.TypeReg || h.Mode != 0755 {
					t.Fatalf("tar layout: %+v", h)
				}
				got, err := io.ReadAll(tr)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("tar payload %q %v", got, err)
				}
				if _, err := tr.Next(); err != io.EOF {
					t.Fatalf("extra tar entry: %v", err)
				}
			}
		})
	}
}

func TestArchiveRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *options)
	}{
		{"empty version", func(t *testing.T, o *options) { o.Version = "" }},
		{"version path", func(t *testing.T, o *options) { o.Version = "a/b" }},
		{"version backslash", func(t *testing.T, o *options) { o.Version = `a\b` }},
		{"version dots", func(t *testing.T, o *options) { o.Version = "v..1" }},
		{"version prefix", func(t *testing.T, o *options) { o.Version = "-v1" }},
		{"version long", func(t *testing.T, o *options) { o.Version = strings.Repeat("v", 129) }},
		{"commit uppercase", func(t *testing.T, o *options) { o.Commit = strings.Repeat("A", 40) }},
		{"commit short", func(t *testing.T, o *options) { o.Commit = "abc" }},
		{"toolchain empty", func(t *testing.T, o *options) { o.GoVersion = "" }},
		{"toolchain space", func(t *testing.T, o *options) { o.GoVersion = "go1.26.4 other" }},
		{"toolchain path", func(t *testing.T, o *options) { o.GoVersion = "go1.26/4" }},
		{"toolchain non-go1", func(t *testing.T, o *options) { o.GoVersion = "go2.0" }},
		{"target unknown", func(t *testing.T, o *options) { o.Targets = []string{"linux/riscv64"} }},
		{"target duplicate", func(t *testing.T, o *options) { o.Targets = []string{"linux/amd64", "linux/amd64"} }},
		{"target empty", func(t *testing.T, o *options) { o.Targets = nil }},
		{"empty binary", func(t *testing.T, o *options) {
			if err := os.WriteFile(filepath.Join(o.InputDir, "aidlc-windows-arm64.exe"), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing last binary", func(t *testing.T, o *options) {
			if err := os.Remove(filepath.Join(o.InputDir, "aidlc-windows-arm64.exe")); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory binary", func(t *testing.T, o *options) {
			p := filepath.Join(o.InputDir, "aidlc-linux-amd64")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(p, 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink binary", func(t *testing.T, o *options) {
			p := filepath.Join(o.InputDir, "aidlc-linux-amd64")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("aidlc-linux-arm64", p); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := archiveFixture(t)
			tc.change(t, &o)
			if err := packageArchives(o); err == nil {
				t.Error("invalid input accepted")
			}
			if _, err := os.Lstat(o.OutputDir); !os.IsNotExist(err) {
				t.Errorf("invalid input created output: %v", err)
			}
		})
	}
	t.Run("existing output preserved", func(t *testing.T) {
		o := archiveFixture(t)
		if err := os.Mkdir(o.OutputDir, 0700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(o.OutputDir, "keep")
		if err := os.WriteFile(p, []byte("unchanged"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := packageArchives(o); err == nil {
			t.Fatal("existing output accepted")
		}
		files, err := os.ReadDir(o.OutputDir)
		if err != nil || len(files) != 1 || string(mustRead(t, p)) != "unchanged" {
			t.Fatal("existing output modified", err)
		}
	})
	t.Run("save failure retained as incomplete", func(t *testing.T) {
		o := archiveFixture(t)
		writes := 0
		o.writeFile = func(path string, raw []byte) error {
			writes++
			if writes == 2 {
				return os.ErrPermission
			}
			return os.WriteFile(path, raw, 0644)
		}
		if err := packageArchives(o); err == nil {
			t.Fatal("save failure reported success")
		}
		files, err := os.ReadDir(o.OutputDir)
		if err != nil || len(files) != 1 {
			t.Fatalf("partial output not retained: %v %v", files, err)
		}
		if _, err := os.Stat(filepath.Join(o.OutputDir, "SHA256SUMS")); !os.IsNotExist(err) {
			t.Fatal("incomplete output got final sums")
		}
	})
}
