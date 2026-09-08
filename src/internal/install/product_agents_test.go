package install

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	codex "github.com/sori883/ai-dd/src/harness/codex/minimal"
)

func TestProductAgentAssets(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	names := []string{"aidlc-researcher", "aidlc-requirements", "aidlc-worker", "aidlc-reviewer"}
	entries, err := os.ReadDir(filepath.Join(root, ".codex/agents"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Errorf("agent definitions=%d, want %d", len(entries), len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			placed, err := os.ReadFile(filepath.Join(root, ".codex/agents", name+".toml"))
			if err != nil {
				t.Fatal(err)
			}
			source, err := fs.ReadFile(codex.Files, "agents/"+name+".toml")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(placed, source) {
				t.Fatal("deployed definition differs from embedded source")
			}
		})
	}
}
