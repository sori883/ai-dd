package main

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledRelease(t *testing.T) {
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	names, err := os.ReadDir(o.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 7 {
		t.Fatalf("public asset count=%d, want 7", len(names))
	}
	sums, err := release.ParseBundleSums(mustRead(t, filepath.Join(o.OutputDir, "SHA256SUMS")), o.Version)
	if err != nil {
		t.Fatal(err)
	}
	originals := map[string][]byte{}
	for _, target := range release.Targets {
		name := release.BundleName(o.Version, target)
		raw := mustRead(t, filepath.Join(o.OutputDir, name))
		originals[name] = raw
		m, entries, err := release.ValidateBundleArchive(raw, o.Version, target, sums[name])
		if err != nil {
			t.Fatal(target, err)
		}
		if m.SourceCommit != o.Commit || m.GoVersion != o.GoVersion {
			t.Fatal("build identity changed")
		}
		for _, product := range release.Products {
			binary := product
			if strings.HasPrefix(target, "windows/") {
				binary += ".exe"
			}
			if string(entries[binary]) != product+":"+target {
				t.Fatal("wrong binary", binary)
			}
		}
		if !bytes.Equal(entries["LICENSES/PRODUCT.txt"], mustRead(t, "../../../LICENSE")) {
			t.Fatal("source license changed")
		}
	}
	o.OutputDir = filepath.Join(t.TempDir(), "second")
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	for name, raw := range originals {
		if !bytes.Equal(raw, mustRead(t, filepath.Join(o.OutputDir, name))) {
			t.Fatal("nonreproducible", name)
		}
	}
}
