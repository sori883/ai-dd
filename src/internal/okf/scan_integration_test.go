//go:build integration

package okf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanBundleSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\ntype: A\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.md", filepath.Join(dir, "link.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, err := ScanBundle(root.FS(), "bundle")
	if err != nil || len(got.Concepts) != 1 || len(got.Warnings) != 1 {
		t.Fatalf("scan %+v %v", got, err)
	}
}
