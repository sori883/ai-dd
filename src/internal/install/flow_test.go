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
	for _, name := range []string{"aidlc/spaces/default/knowledge/adr/index.md", "aidlc/templates/adr.md", ".agents/skills/aidlc/SKILL.md", ".codex/agents/aidlc-reviewer.toml"} {
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
	procedure, err := os.ReadFile(filepath.Join(root, "aidlc/workflow/stages/tdd.md"))
	if err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/WORKFLOW.md"))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, common...)
	raw = append(raw, procedure...)
	for _, word := range []string{"discovery", "planning", "tdd", "integration", "intent review", "unit claim"} {
		if !strings.Contains(string(raw), word) {
			t.Errorf("missing %s", word)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "aidlc/templates/kdr.md")); !os.IsNotExist(err) {
		t.Fatal("obsolete KDR template deployed")
	}
}
func TestFlowInstallJapaneseProcedure(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(skill) > 4096 || !strings.Contains(string(skill), "WORKFLOW.md") || !strings.Contains(string(skill), "全文") {
		t.Fatal("Japanese bootstrap does not reach full procedure")
	}
	procedure, err := os.ReadFile(filepath.Join(root, "aidlc/workflow/stages/tdd.md"))
	if err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/WORKFLOW.md"))
	if err != nil {
		t.Fatal(err)
	}
	procedure = append(procedure, common...)
	for _, word := range []string{"実装計画", "独立レビュー", "intent review", "unit claim", "memory update"} {
		if !strings.Contains(string(procedure), word) {
			t.Errorf("missing %s", word)
		}
	}
}
