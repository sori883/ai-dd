package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/core"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"github.com/sori883/ai-dd/src/internal/workflow"
)

func TestWorkflowDefinitionFresh(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Load(root); err != nil {
		t.Fatal("fresh definition unavailable", err)
	}
	completed, err := coreworkflow.Render(core.Files, map[string]string{"skill-root": ".agents/skills"})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range completed {
		got, err := os.ReadFile(filepath.Join(root, "aidlc/workflow", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("asset mismatch %s", name)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 || !strings.Contains(string(raw), "intent procedure") || strings.Contains(string(raw), "手順を**全文**") {
		t.Fatal("entry does not use current procedure")
	}
	if _, err = Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("existing deployment overwritten")
	}
}
