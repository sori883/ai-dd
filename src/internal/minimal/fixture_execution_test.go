package minimal

import (
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// executionFixtureState supplies an explicit approved context for isolated
// command/hook tests. Public approval lifecycle has separate execution tests.
func executionFixtureState(t *testing.T, store flow.Store, st flow.State, stage string) flow.State {
	t.Helper()
	stages := []string{"initialization", "discovery", "architecture-analysis", "planning", "tdd", "integration"}
	p := &flow.PlanVersion{Revision: 1, Reason: "isolated hook fixture"}
	found := false
	for i, name := range stages {
		status := "completed"
		if name == stage {
			found = true
			st.CurrentStepID = fmt.Sprintf("s%02d", i+1)
		}
		if found {
			status = "pending"
		}
		p.Steps = append(p.Steps, flow.ExecutionStep{ID: fmt.Sprintf("s%02d", i+1), Stage: name, Status: status})
	}
	st.Stage = stage
	st.ExecutionPlan = flow.ExecutionPlan{Revision: 1, NextID: 7, Approved: p}
	hash := flow.PlanHash(*p)
	p.Approval = &flow.Approval{RequestID: st.ID, Target: hash, StepID: st.CurrentStepID, PlanRevision: 1, PlanHash: hash, DefinitionHash: st.DefinitionHash, Status: "approved", Session: "fixture", Turn: "fixture", Quote: "synthetic fixture approval", PromptHash: st.DefinitionHash, At: time.Now().UTC().Format(time.RFC3339Nano)}
	if st.Entry != nil {
		st.Entry.StepID = st.CurrentStepID
		st.Entry.Stage = stage
	}
	return writeExecutionFixture(t, store, st)
}
func writeExecutionFixture(t *testing.T, store flow.Store, st flow.State) flow.State {
	t.Helper()
	if st.Status == "completed" {
		for i := range st.ExecutionPlan.Approved.Steps {
			st.ExecutionPlan.Approved.Steps[i].Status = "completed"
		}
		st.CurrentStepID = "s06"
		st.Stage = "integration"
		st.Entry = nil
	}
	for i := range st.Config.Units {
		st.Config.Units[i].StepID = st.CurrentStepID
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(store.Root, "aidlc/spaces", store.Space, "intents", st.ID, "state.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func approveFixturePlan(t *testing.T, store flow.Store, st flow.State) flow.State {
	t.Helper()
	a := st.ExecutionPlan.Draft.Approval
	if err := store.CaptureApproval(st.ID, "fixture", "", a.RequestID, "approve fixture plan"); err != nil {
		t.Fatal(err)
	}
	next, err := store.DecidePlan(st.ID, st.Revision, flow.ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "fixture", Turn: a.RequestID, Quote: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	return next
}
