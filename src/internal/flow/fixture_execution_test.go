package flow

import "testing"

// fixtureExecutionStage supplies an already approved execution context for tests
// isolated from plan/approval lifecycle. Lifecycle is tested through public
// operations in execution_approval_test.go and execution_history_test.go.
func fixtureExecutionStage(t *testing.T, s Store, st *State, stage string) {
	t.Helper()
	ids := []string{"initialization", "discovery", "planning", "tdd", "integration"}
	steps := []ExecutionStep{}
	seen := false
	for _, name := range ids {
		status := "completed"
		if name == stage {
			seen = true
		}
		if seen {
			status = "pending"
		}
		steps = append(steps, ExecutionStep{ID: fixtureStepID(name), Stage: name, Status: status})
	}
	st.ExecutionPlan = ExecutionPlan{Revision: 1, NextID: 6, Approved: &PlanVersion{Revision: 1, Reason: "isolated behavior fixture", Steps: steps, Omitted: []StageOmission{{Stage: "architecture-analysis", Reason: "not selected by fixture"}}}}
	st.CurrentStepID = fixtureStepID(stage)
	st.Stage = stage
	if st.Entry != nil {
		st.Entry.Stage = stage
		st.Entry.StepID = st.CurrentStepID
	}
	for i := range st.Config.Units {
		st.Config.Units[i].StepID = st.CurrentStepID
	}
	st.Review = Gate{}
	st.Sensor = Gate{}
	st.Approval = nil
	fixturePlanApproval(t, st)
	if err := s.persist(*st); err != nil {
		t.Fatal(err)
	}
}
func fixtureStepID(stage string) string {
	return map[string]string{"initialization": "s01", "discovery": "s02", "planning": "s03", "tdd": "s04", "integration": "s05"}[stage]
}
func saveExecutionFixture(t *testing.T, s Store, st State, expect uint64) (State, error) {
	t.Helper()
	for i := range st.Config.Units {
		if st.Config.Units[i].StepID == "" {
			st.Config.Units[i].StepID = st.CurrentStepID
		}
	}
	return s.Save(st, expect)
}

func createExecutionFixture(t *testing.T, s Store, name string) (State, error) {
	t.Helper()
	st, err := s.Create(name)
	if err != nil {
		return st, err
	}
	fixtureExecutionStage(t, s, &st, "discovery")
	return st, nil
}

// transitionExecutionFixture provides an explicit synthetic human decision for
// older tests isolating Sensor/review/transition behavior, without bypassing the
// current finish checks. Human-source authorization has dedicated tests.
func transitionExecutionFixture(t *testing.T, s Store, id string, expect uint64, r TransitionRequest) (State, error) {
	t.Helper()
	if r.Action != "advance" && r.Action != "reopen" {
		return s.Transition(id, expect, r)
	}
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if st.Revision != expect {
		return State{}, invalid("revision conflict")
	}
	if r.Action == "advance" {
		if st.Review.Status == "pass" && (st.Approval == nil || st.Approval.Status != "approved") {
			rev, hash := evidencePlan(st)
			a, err := newApproval(st, st.Review.Target, rev, hash)
			if err != nil {
				return State{}, err
			}
			a.Status = "approved"
			a.Session = "test-fixture"
			a.Turn = "test-fixture"
			a.Quote = "synthetic transition fixture approval"
			a.PromptHash = st.DefinitionHash
			st.Approval = a
			if err = s.persist(st); err != nil {
				return State{}, err
			}
		}
		return s.Finish(id, expect)
	}
	target := ""
	for _, step := range executionSteps(st) {
		if step.Stage == r.Stage {
			target = step.ID
		}
	}
	if st.PendingReopen == nil && st.ExecutionPlan.Draft == nil {
		st, err = s.Reopen(id, expect, target, r.Reason)
		if err != nil {
			return State{}, err
		}
	}
	if st.ExecutionPlan.Draft == nil {
		return State{}, invalid("reopen draft required")
	}
	if st.ExecutionPlan.Draft.Reason != r.Reason {
		return State{}, invalid("different reopen request")
	}
	turn := st.ExecutionPlan.Draft.Approval.RequestID
	if err = s.CaptureApproval(id, "test-fixture", "", turn, "approve synthetic reopen fixture"); err != nil {
		return State{}, err
	}
	a := st.ExecutionPlan.Draft.Approval
	return s.DecidePlan(id, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "test-fixture", Turn: turn, Quote: "approve"})
}

func prepareReopenFixture(t *testing.T, s Store, st State, r TransitionRequest) State {
	t.Helper()
	target := ""
	for _, step := range executionSteps(st) {
		if step.Stage == r.Stage {
			target = step.ID
		}
	}
	next, err := s.Reopen(st.ID, st.Revision, target, r.Reason)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
