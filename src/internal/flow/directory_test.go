package flow

import (
	"github.com/sori883/ai-dd/src/internal/assignment"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryAssignment(t *testing.T) {
	s, st := sensorFixture(t)
	fixtureExecutionStage(t, s, &st, "tdd")
	prepareBoundaryStage(t, s, &st)
	st, err := saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Join(s.Root, ".git")); err != nil {
		t.Fatal(err)
	}
	registry := assignment.Store{Root: s.Root}
	r, err := registry.Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "new project"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.ReserveAssignment(st.ID, st.Revision, "main", AssignmentRequest{RegistryEpoch: r.Epoch, RequestID: "reserve", StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: s.Root, Session: "worker"})
	if err != nil {
		t.Fatal(err)
	}
}
