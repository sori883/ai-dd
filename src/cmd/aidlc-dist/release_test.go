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
	o := archiveFixture(t)
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
	if err := packageArchives(o); err == nil {
		t.Fatal("incomplete release candidate accepted")
	}
	if _, err := os.Stat(o.OutputDir); !os.IsNotExist(err) {
		t.Fatal("invalid candidate created output", err)
	}
}
