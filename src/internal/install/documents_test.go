package install

import (
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"github.com/sori883/ai-dd/src/internal/workflow"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentDistributionRuleSelector(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	d, err := workflow.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ref := d.Procedures["discovery"].Inputs[0]
	if ref.Path != "${knowledge_root}/rules/rule.md" || ref.Match != nil || ref.Metadata == nil || ref.Metadata.Type != "Rule" || ref.Version != "current" {
		t.Fatalf("fixed Rule reference: %+v", ref)
	}
	raw, err := os.ReadFile(filepath.Join(root, "aidlc/spaces/default/knowledge/rules/rule.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := okfmemory.Parse(raw)
	if err != nil || !ref.Metadata.Matches(doc) {
		t.Fatalf("fresh Rule: %v", err)
	}
}
