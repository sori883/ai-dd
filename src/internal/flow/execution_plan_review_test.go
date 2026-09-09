package flow

import "testing"

func TestExecutionPlanReviewReopenRequiresReplay(t *testing.T) {
	for _, mode := range []string{"log only", "initialization without discovery", "initialization without origin"} {
		t.Run(mode, func(t *testing.T) {
			s, st := discoveryApprovalFixture(t)
			if err := s.CaptureApproval(st.ID, "user", "", "A", "approve"); err != nil {
				t.Fatal(err)
			}
			st, err := s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
			if err != nil {
				t.Fatal(err)
			}
			r := PlanRequest{Reason: "reopen", ReopenStepID: "s01", Omitted: st.ExecutionPlan.Approved.Omitted}
			for _, step := range st.ExecutionPlan.Approved.Steps {
				r.Steps = append(r.Steps, PlanStepInput{ID: step.ID, Stage: step.Stage})
			}
			if mode != "log only" {
				r.Steps = append(r.Steps, PlanStepInput{Stage: "initialization"})
			}
			if mode == "initialization without origin" {
				r.ReopenStepID = ""
			}
			if _, err = s.ProposePlan(st.ID, st.Revision, r); err == nil {
				t.Fatal("reopen without valid replay accepted")
			}
		})
	}
}
func TestExecutionPlanReviewReopenRepeatedInitialization(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve"); err != nil {
		t.Fatal(err)
	}
	st, err := s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Reopen(st.ID, st.Revision, "s01", "reinitialize")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "", "B", "approve"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "B"))
	if err != nil {
		t.Fatal(err)
	}
	target := st.CurrentStepID
	st, err = s.Reopen(st.ID, st.Revision, target, "repeat initialization")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "", "C", "approve"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "C"))
	if err != nil {
		t.Fatal(err)
	}
	if st.CurrentStepID == target || currentExecution(&st).Reopens != target {
		t.Fatal("repeated initialization lost origin")
	}
	steps := executionSteps(st)
	if steps[len(steps)-1].Stage != "discovery" {
		t.Fatal("discovery not retained after initialization")
	}
}

func TestExecutionPlanReviewReopenAfterCompletedDiscovery(t *testing.T) {
	s, st := completedDiscovery(t)
	old := append([]ExecutionStep{}, executionSteps(st)...)
	st, err := s.Reopen(st.ID, st.Revision, "s01", "reinitialize completed work")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "", "B", "approve"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "B"))
	if err != nil {
		t.Fatal(err)
	}
	steps := executionSteps(st)
	if len(steps) != 4 || steps[0] != old[0] || steps[1] != old[1] || steps[2].Stage != "initialization" || steps[3].Stage != "discovery" || steps[3].ID == old[1].ID || steps[3].Status != "pending" {
		t.Fatalf("invalid replay plan: %+v", steps)
	}
}

func TestExecutionPlanReviewReopenCompletedInvalidPlan(t *testing.T) {
	s, st := completedDiscovery(t)
	r := PlanRequest{Reason: "invalid reinitialization", ReopenStepID: "s01", Omitted: st.ExecutionPlan.Approved.Omitted}
	for _, step := range executionSteps(st) {
		r.Steps = append(r.Steps, PlanStepInput{ID: step.ID, Stage: step.Stage})
	}
	r.Steps = append(r.Steps, PlanStepInput{Stage: "initialization"})
	if _, err := s.ProposePlan(st.ID, st.Revision, r); err == nil {
		t.Fatal("completed discovery followed by initialization alone accepted")
	}
}
