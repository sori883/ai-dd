package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlowSpaceADR(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateSpace(RootInput{ExplicitDir: root}, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "aidlc/spaces/new/knowledge/ADR/index.md")); err != nil {
		t.Fatal(err)
	}
}
