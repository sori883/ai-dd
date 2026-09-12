package app

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func deployProcedureFixture(t *testing.T, root string) {
	t.Helper()
	err := fs.WalkDir(coreworkflow.Files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := coreworkflow.Files.ReadFile(name)
		if err != nil {
			return err
		}
		p := filepath.Join(root, "aidlc/workflow", name)
		if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		return os.WriteFile(p, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestProcedureReadBeforeBeginAndWaiting(t *testing.T) {
	s, st := setup(t)
	for _, waiting := range []bool{false, true} {
		t.Run(map[bool]string{false: "unstarted", true: "waiting"}[waiting], func(t *testing.T) {
			if waiting {
				store := flow.Store{Root: s.Root, Space: "default"}
				var err error
				st, err = store.Transition(st.ID, st.Revision, flow.TransitionRequest{Action: "wait", Reason: "question", ResumeCondition: "answer"})
				if err != nil {
					t.Fatal(err)
				}
			}
			out := hook(t, s, "PreToolUse", "Bash", "procedure", "/opt/aidlc intent procedure "+st.ID+" --space default", false)
			if deny(out) {
				t.Fatal("procedure read denied")
			}
			raw, err := s.Execute(cli.CommandRequest{Command: "intent", Action: "procedure", Target: st.ID, Space: "default"})
			if err != nil {
				t.Fatal(err)
			}
			var view map[string]any
			if json.Unmarshal(raw, &view) != nil || view["stage"] != "discovery" || !strings.Contains(string(raw), "stage_id") {
				t.Fatalf("wrong procedure: %s", raw)
			}
		})
	}
}

func TestProcedureDriftBlocksDocumentRepair(t *testing.T) {
	s, st := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, st.ID)
	p := filepath.Join(s.Root, "aidlc/workflow/stages/discovery.md")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, append(raw, []byte("changed\n")...), 0644)
	out := hook(t, s, "PreToolUse", "Bash", "repair", "/opt/aidlc memory create codekb/current-analysis --space default --body-file draft --actor process:test --type CurrentAnalysis --title Current --description Current", false)
	if !deny(out) {
		t.Fatal("definition drift allowed document repair")
	}
}
