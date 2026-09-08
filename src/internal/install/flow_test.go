package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowInstallAssets(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aidlc/spaces/default/knowledge/ADR/index.md", "aidlc/templates/adr.md", ".agents/skills/aidlc/SKILL.md", ".codex/agents/aidlc-reviewer.toml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Error(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 {
		t.Fatalf("skill exceeds bootstrap: %d", len(raw))
	}
	for _, word := range []string{"discovery", "planning", "tdd", "integration", "intent review", "unit claim"} {
		if !strings.Contains(string(raw), word) {
			t.Errorf("missing %s", word)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "aidlc/templates/kdr.md")); !os.IsNotExist(err) {
		t.Fatal("obsolete KDR template deployed")
	}
}
