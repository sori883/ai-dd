package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/release"
	"io"
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
func TestFiveProductManifest(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(o.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 43 {
		t.Fatalf("public asset count=%d", len(entries))
	}
	for _, product := range products {
		name := "manifest.json"
		if product != "aidlc" {
			name = product + "-" + name
		}
		raw := mustRead(t, filepath.Join(o.OutputDir, name))
		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if len(m.Artifacts) != 6 || m.Version != o.Version || m.SourceCommit != o.Commit {
			t.Fatalf("manifest %s: %+v", product, m)
		}
	}
}
func TestFiveProductArchive(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	for _, product := range products {
		raw := mustRead(t, filepath.Join(o.OutputDir, product+"_"+o.Version+"_linux_amd64.tar.gz"))
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		tr := tar.NewReader(gz)
		seen := map[string]bool{}
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			seen[h.Name] = true
		}
		gz.Close()
		for _, name := range []string{product, "LICENSES/PRODUCT.txt", "LICENSES/Go-LICENSE.txt", "LICENSES/Go-PATENTS.txt"} {
			if !seen[name] {
				t.Errorf("%s missing %s", product, name)
			}
		}
	}
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
func TestVersionedAssets(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	raw := mustRead(t, filepath.Join(o.OutputDir, "aidlc-assets-manifest.json"))
	var m struct {
		Schema  int    `json:"schema_version"`
		Version string `json:"version"`
		Files   []any  `json:"files"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Schema != 1 || m.Version != o.Version || len(m.Files) == 0 {
		t.Fatalf("assets manifest %+v", m)
	}
}

func TestVersionedAssetsLicense(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	raw := mustRead(t, filepath.Join(o.OutputDir, "aidlc-assets_"+o.Version+".tar.gz"))
	files, err := release.Unpack(raw, false, release.MaxSourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["LICENSES/PRODUCT.txt"], mustRead(t, "../../../LICENSE")) {
		t.Fatal("data archive missing canonical product license")
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
