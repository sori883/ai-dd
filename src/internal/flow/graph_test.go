package flow

import (
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
func TestGraphTransitionUsesReopenDefinition(t *testing.T) {
	s, st := boundaryFixture(t)
	deployFlowDefinition(t, s)
	p := filepath.Join(s.Root, "aidlc/workflow/stage-graph.json")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(raw), `"allow_current": true`, `"allow_current": false`, 1)), 0644)
	st, err := s.Create("bound no-current graph")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "repeat"}); err == nil {
		t.Fatal("graph disallows current reopen but transition accepted")
	}
}
func TestGraphTransitionMissingDefinition(t *testing.T) {
	s, st := boundaryFixture(t)
	os.Remove(filepath.Join(s.Root, "aidlc/workflow/stage-graph.json"))
	if _, err := s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "repeat"}); err == nil {
		t.Fatal("missing graph silently used fixed transition")
	}
}
