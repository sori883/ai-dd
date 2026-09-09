package install

import (
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"github.com/sori883/ai-dd/src/internal/workflow"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentDistribution(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "aidlc/spaces/default/knowledge/adr/index.md")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "aidlc/spaces/default/knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name() == "ADR" {
			t.Fatal("uppercase directory deployed")
		}
		if entry.Name() == "adr" {
			found = true
		}
	}
	if !found {
		t.Fatal("lowercase directory missing")
	}
	raw, err := os.ReadFile(filepath.Join(root, "aidlc/templates/adr.md"))
	if err != nil || !strings.Contains(string(raw), "type: adr") {
		t.Fatal("adr type")
	}
	d, err := workflow.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Procedures {
		if !strings.Contains(p.Text, "WORKFLOW.md") {
			t.Fatal("common document operation reference missing")
		}
	}
	common, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/WORKFLOW.md"))
	if err != nil || !strings.Contains(string(common), "intent documents") {
		t.Fatal("common document operations missing", err)
	}
	raw, err = os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil || len(raw) > 4096 {
		t.Fatal("bootstrap cap")
	}
}

func TestDocumentDistributionRuleSelector(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	d, err := workflow.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := okfmemory.SelectDocuments(filepath.Join(root, "aidlc/spaces/default/knowledge"), *d.Procedures["discovery"].Inputs[0].Match, "one")
	if err != nil || len(docs) != 1 || docs[0].Path != "rules/rule.md" {
		t.Fatalf("fresh Rule selector: %+v %v", docs, err)
	}
}
