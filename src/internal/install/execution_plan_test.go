package install

import (
	"github.com/sori883/ai-dd/src/internal/workflow"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionPlanDistribution(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	d, err := workflow.Load(root)
	if err != nil {
		t.Fatalf("fresh six-stage catalog: %v", err)
	}
	if len(d.Graph.Stages) != 6 {
		t.Fatal("missing stage")
	}
	for _, stage := range d.Graph.Stages {
		text := d.Procedures[stage.ID].Text
		for _, word := range []string{"step_id", "finish", "approval"} {
			if !strings.Contains(text, word) {
				t.Errorf("%s procedure lacks %s", stage.ID, word)
			}
		}
		if strings.Contains(text, "intent advance") || strings.Contains(text, "--stage STAGE") {
			t.Errorf("old transition in %s", stage.ID)
		}
	}
	for _, name := range []string{".agents/skills/aidlc/WORKFLOW.md", "aidlc/spaces/default/knowledge/rules/rule.md"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "plan-approval") {
			t.Errorf("missing plan approval in %s", name)
		}
	}
}
