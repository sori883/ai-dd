package main

import (
	"bytes"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/release"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCandidateLicense(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	raw := mustRead(t, filepath.Join(o.OutputDir, release.BundleName(o.Version, "linux/amd64")))
	_, entries, err := release.ValidateBundleArchive(raw, o.Version, "linux/amd64", release.Hash(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range []string{"aidlc-install", "aidlc", "okf", "natural-japanese-go", "aidlc-dist"} {
		t.Run(product, func(t *testing.T) {
			if err := validateCandidateLicenses(product, entries, false); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct{ name, product, path string }{
		{"missing", "okf", "LICENSES/okf/yaml-NOTICE.txt"},
		{"modified", "natural-japanese-go", "LICENSES/natural-japanese-go/UniDic-NOTICE.txt"},
		{"extra", "aidlc", "LICENSES/aidlc/extra.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := maps.Clone(entries)
			if tc.name == "missing" {
				delete(changed, tc.path)
			} else {
				changed[tc.path] = []byte("changed")
			}
			if err := validateCandidateLicenses(tc.product, changed, false); err == nil {
				t.Fatal("incorrect candidate license accepted")
			}
		})
	}
}

// Read source files independently of the packaging implementation. The release
// entry point already requires this checkout's commit and build Go version.
func validateCandidateLicenses(product string, bundle map[string][]byte, windows bool) error {
	entries := map[string][]byte{}
	binary := product
	if windows {
		binary += ".exe"
	}
	entries[binary] = bundle[binary]
	if product == "natural-japanese-go" {
		entries["README.md"] = bundle["README.md"]
	}
	prefix := "LICENSES/" + product + "/"
	for path, raw := range bundle {
		if strings.HasPrefix(path, prefix) {
			entries["LICENSES/"+strings.TrimPrefix(path, prefix)] = raw
		}
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
