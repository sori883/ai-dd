package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var products = []string{"aidlc-install", "aidlc", "okf", "natural-japanese-go", "aidlc-dist"}

func releaseFixture(t *testing.T) options {
	t.Helper()
	o := options{InputDir: t.TempDir(), OutputDir: filepath.Join(t.TempDir(), "candidate"), Version: "dev-abcdef0", Commit: strings.Repeat("a", 40), Targets: append([]string{}, fixtureTargets...)}
	o.Product = "all"
	o.LicenseDir = t.TempDir()
	o.GoVersion = runtime.Version()
	for name, source := range map[string]string{"PRODUCT.txt": "../../../LICENSE", "Go-LICENSE.txt": filepath.Join(runtime.GOROOT(), "LICENSE"), "Go-PATENTS.txt": filepath.Join(runtime.GOROOT(), "PATENTS"), "yaml-LICENSE.txt": "../../distribution/licenses/yaml-LICENSE.txt", "yaml-NOTICE.txt": "../../distribution/licenses/yaml-NOTICE.txt", "Apache-2.0.txt": "../../distribution/licenses/Apache-2.0.txt"} {
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(o.LicenseDir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(o.LicenseDir, "GO_VERSION"), []byte(runtime.Version()), 0600); err != nil {
		t.Fatal(err)
	}
	for _, p := range products {
		for _, target := range fixtureTargets {
			name := p + "-" + strings.ReplaceAll(target, "/", "-")
			if strings.HasPrefix(target, "windows/") {
				name += ".exe"
			}
			if err := os.WriteFile(filepath.Join(o.InputDir, name), []byte(p+":"+target), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return o
}
func TestReleaseLicenseInputs(t *testing.T) {
	o := releaseFixture(t)
	o.LicenseDir = t.TempDir()
	o.GoVersion = runtime.Version()
	if err := packageRelease(o); err == nil {
		t.Fatal("empty license inputs accepted")
	}
	if _, err := os.Stat(o.OutputDir); !os.IsNotExist(err) {
		t.Fatal("invalid input wrote candidate")
	}
}

func TestFiveProductManifestRequiresSixTargets(t *testing.T) {
	o := releaseFixture(t)
	o.Targets = []string{"linux/amd64"}
	if err := packageRelease(o); err == nil {
		t.Fatal("incomplete release candidate accepted")
	}
	if _, err := os.Stat(o.OutputDir); !os.IsNotExist(err) {
		t.Fatal("invalid candidate created output", err)
	}
}

func TestReleaseInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *options)
	}{
		{"valid", func(t *testing.T, o *options) {}},
		{"unknown product", func(t *testing.T, o *options) { o.Product = "../escape" }},
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
			o := releaseFixture(t)
			o.Product = "aidlc"
			tc.change(t, &o)
			_, err := validateInputs(o)
			if (err == nil) != (tc.name == "valid") {
				t.Fatalf("validateInputs %s: %v", tc.name, err)
			}
			if _, err := os.Lstat(o.OutputDir); !os.IsNotExist(err) {
				t.Errorf("invalid input created output: %v", err)
			}
		})
	}
}

func TestReleasePreflight(t *testing.T) {
	for _, mode := range []string{"missing late binary", "existing output", "partial save"} {
		t.Run(mode, func(t *testing.T) {
			o := releaseFixture(t)
			switch mode {
			case "missing late binary":
				if err := os.Remove(filepath.Join(o.InputDir, "aidlc-dist-windows-arm64.exe")); err != nil {
					t.Fatal(err)
				}
			case "existing output":
				if err := os.Mkdir(o.OutputDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(o.OutputDir, "keep"), []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
			case "partial save":
				writes := 0
				o.writeFile = func(path string, raw []byte) error {
					writes++
					if writes == 2 {
						return os.ErrPermission
					}
					return os.WriteFile(path, raw, 0644)
				}
			}
			if err := packageRelease(o); err == nil {
				t.Fatal("invalid/partial release accepted")
			}
			if mode == "missing late binary" {
				if _, err := os.Lstat(o.OutputDir); !os.IsNotExist(err) {
					t.Fatal("preflight wrote output", err)
				}
				return
			}
			entries, err := os.ReadDir(o.OutputDir)
			if err != nil || len(entries) != 1 {
				t.Fatal("unexpected retained output", entries, err)
			}
			if mode == "existing output" && string(mustRead(t, filepath.Join(o.OutputDir, "keep"))) != "unchanged" {
				t.Fatal("changed existing output")
			}
			if mode == "partial save" {
				if len(entries) >= len(fixtureTargets) {
					t.Fatal("partial output appears complete")
				}
			}
		})
	}
}

var fixtureTargets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
