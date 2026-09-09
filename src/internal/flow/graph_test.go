package flow

import (
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func deployFlowDefinition(t *testing.T, s Store) {
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
		p := filepath.Join(s.Root, "aidlc/workflow", name)
		if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		return os.WriteFile(p, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestGraphTransitionUsesBoundDefinition(t *testing.T) {
	s, st := boundaryFixture(t)
	p := filepath.Join(s.Root, "aidlc/workflow/stage-graph.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p, append(raw, byte(10)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reopen(st.ID, st.Revision, st.CurrentStepID, "repeat"); err == nil {
		t.Fatal("changed bound definition allowed reopen")
	}
}
func TestGraphTransitionMissingDefinition(t *testing.T) {
	s, st := boundaryFixture(t)
	os.Remove(filepath.Join(s.Root, "aidlc/workflow/stage-graph.json"))
	if _, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "repeat"}); err == nil {
		t.Fatal("missing graph silently used fixed transition")
	}
}
