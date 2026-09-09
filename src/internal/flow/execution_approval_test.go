package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"strings"
	"testing"
)

func planDecision(st State, session, turn string) ApprovalDecision {
	a := st.ExecutionPlan.Draft.Approval
	return ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: session, Turn: turn, Quote: "approve"}
}
func TestExecutionPlanApprovalPlan(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("approve plan")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	if st.ExecutionPlan.Draft.Approval == nil {
		t.Fatal("draft has no human approval request")
	}
	decision := planDecision(st, "session", "A")
	if _, err = s.DecidePlan(st.ID, st.Revision, decision); err == nil {
		t.Fatal("uncaptured answer approved plan")
	}
	if err = s.CaptureApproval(st.ID, "session", "", "A", "I approve this plan"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, decision)
	if err != nil {
		t.Fatalf("captured plan rejected: %v", err)
	}
	if st.ExecutionPlan.Draft != nil || st.ExecutionPlan.Approved == nil || st.ExecutionPlan.Approved.Approval.Status != "approved" {
		t.Fatal("plan approval did not adopt draft")
	}
	if st.Stage != "initialization" || st.ExecutionPlan.Approved.Steps[0].Status != "pending" {
		t.Fatal("plan approval fabricated work")
	}
	candidate := st
	v := *st.ExecutionPlan.Approved
	candidate.ExecutionPlan.Approved = &v
	v.Approval = nil
	if err = s.persist(candidate); err == nil {
		t.Fatal("approved plan persisted without evidence")
	}
}
func TestExecutionPlanApprovalReplay(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("replay")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "session", "", "A", "approve"); err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "session", "A", "B", "clarification"); err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "session", "B", "A", "approve"); err == nil {
		t.Fatal("A B A replay accepted")
	}
	if err = s.CaptureApproval(st.ID, "session", "B", "C", strings.Repeat("x", 64*1024+1)); err == nil {
		t.Fatal("oversize answer accepted")
	}
}

func TestExecutionPlanApprovalBootstrapFinish(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("initialize")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Finish(st.ID, st.Revision); err == nil {
		t.Fatal("unstarted execution finished")
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", Root: root, Session: "reviewer", CoordinatorSession: "coordinator"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Root: root, Session: "reviewer", Target: st.Review.Target, Status: "pass", Summary: "verified settings"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Approval == nil || st.Approval.Status != "pending" {
		t.Fatal("review did not request human result approval")
	}
	if _, err = s.Finish(st.ID, st.Revision); err == nil {
		t.Fatal("review alone finished execution")
	}
	if err = s.CaptureApproval(st.ID, "user", "", "A", "I approve the initialization result"); err != nil {
		t.Fatal(err)
	}
	a := st.Approval
	st, err = s.Decide(st.ID, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: "A", Quote: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Finish(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if st.CurrentStepID != "s02" || st.Stage != "discovery" || st.ExecutionPlan.Approved != nil || st.Accepted["s01"].StepID != "s01" {
		t.Fatalf("bootstrap completion: %+v", st)
	}
}

func discoveryApprovalFixture(t *testing.T) (Store, State) {
	return discoveryApprovalFixtureOrder(t, nil)
}
func discoveryApprovalFixtureOrder(t *testing.T, order []string) (Store, State) {
	t.Helper()
	s := executionFixture(t)
	if len(order) > 0 {
		boundaryFile(t, s, "aidlc/workflow/stages/integration.md", "---\nstage_id: integration\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: integration-start\n  end: integration-end\n---\n# Inspect\nInspect existing code.\n")
	}
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "add", ".")
	flowGit(t, s.Root, "commit", "-qm", "installed")
	st, err := s.Create("discovery")
	if err != nil {
		t.Fatal(err)
	}
	st.ExecutionPlan.Bootstrap[0].Status = "completed"
	st.CurrentStepID = "s02"
	st.Stage = "discovery"
	st.Config = Config{Objective: "investigation", Scope: []string{"scope"}, Acceptance: []string{"criteria"}, CodeRevision: strings.TrimSpace(flowGit(t, s.Root, "rev-parse", "HEAD")), NoMaterialsReason: "no external material", ADR: ADR{Reason: "no design change"}}
	boundaryDoc(t, s, st, "Requirements")
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	request := PlanRequest{Reason: "investigation only", Steps: []PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}}}
	for _, stage := range order {
		request.Steps = append(request.Steps, PlanStepInput{Stage: stage})
	}
	for _, stage := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
		selected := false
		for _, chosen := range order {
			selected = selected || chosen == stage
		}
		if selected {
			continue
		}
		request.Omitted = append(request.Omitted, StageOmission{Stage: stage, Reason: "not needed"})
	}
	st, err = s.ProposePlan(st.ID, st.Revision, request)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	flowGit(t, s.Root, "worktree", "add", "--detach", root, "HEAD")
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", Root: root, Session: "reviewer", CoordinatorSession: "coordinator"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Root: root, Session: "reviewer", Target: st.Review.Target, Status: "pass", Summary: "requirements and selection reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}
func TestExecutionPlanApprovalSameAnswer(t *testing.T) {
	for _, order := range []string{"plan first", "result first"} {
		t.Run(order, func(t *testing.T) {
			s, st := discoveryApprovalFixture(t)
			before := st.Review.Target
			if st.Approval.PlanRevision != st.ExecutionPlan.Draft.Revision || st.Approval.PlanHash != PlanHash(*st.ExecutionPlan.Draft) {
				t.Fatal("result is not bound to presented draft")
			}
			pd := planDecision(st, "user", "answer")
			a := st.Approval
			rd := ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: "answer", Quote: "approve"}
			if err := s.CaptureApproval(st.ID, "user", "", "answer", "I approve both the plan and the discovery result"); err != nil {
				t.Fatal(err)
			}
			var err error
			if order == "plan first" {
				st, err = s.DecidePlan(st.ID, st.Revision, pd)
				if err == nil {
					st, err = s.Decide(st.ID, st.Revision, rd)
				}
			} else {
				st, err = s.Decide(st.ID, st.Revision, rd)
				if err == nil {
					st, err = s.DecidePlan(st.ID, st.Revision, pd)
				}
			}
			if err != nil {
				t.Fatalf("same presented draft could not share answer: %v", err)
			}
			if st.Review.Target != before {
				t.Fatal("unchanged draft invalidated review")
			}
			st, err = s.Finish(st.ID, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if st.Status != "completed" || len(st.ExecutionPlan.Approved.Steps) != 2 {
				t.Fatal("optional stages forced")
			}
		})
	}
}

func TestExecutionPlanApprovalDraftInvalidation(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve"); err != nil {
		t.Fatal(err)
	}
	old := planDecision(st, "user", "A")
	request := PlanRequest{Reason: "changed reason", Steps: []PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}}, Omitted: st.ExecutionPlan.Draft.Omitted}
	var err error
	st, err = s.ProposePlan(st.ID, st.Revision, request)
	if err != nil {
		t.Fatal(err)
	}
	if st.Approval != nil || st.Review.Status != "" || st.Sensor.Status != "" {
		t.Fatal("changed draft retained result evidence")
	}
	if _, err = s.DecidePlan(st.ID, st.Revision, old); err == nil {
		t.Fatal("old request accepted")
	}
	fresh := planDecision(st, "user", "A")
	if _, err = s.DecidePlan(st.ID, st.Revision, fresh); err == nil {
		t.Fatal("earlier answer reused for later request")
	}
}
func TestExecutionPlanApprovalOwnedState(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	copy := *st.Approval
	copy.Status = "approved"
	st.Approval = &copy
	if _, err := s.Save(st, st.Revision); err == nil {
		t.Fatal("configure forged human approval")
	}
}

func TestExecutionPlanApprovalArbitraryOrder(t *testing.T) {
	s, st := discoveryApprovalFixtureOrder(t, []string{"integration", "tdd"})
	if err := s.CaptureApproval(st.ID, "user", "", "discovery", "approve both"); err != nil {
		t.Fatal(err)
	}
	pd := planDecision(st, "user", "discovery")
	a := st.Approval
	var err error
	st, err = s.DecidePlan(st.ID, st.Revision, pd)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Decide(st.ID, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: "discovery", Quote: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Finish(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"integration", "tdd"} {
		if st.Stage != stage {
			t.Fatalf("wrong selected order: %s", st.Stage)
		}
		st.Config.Tests = []string{"inspect existing code"}
		st.Config.DirectCommit = st.Config.CodeRevision
		st.Config.TestResults = []string{"aidlc/evidence/" + st.CurrentStepID + ".json"}
		zero := 0
		result := resultDocument{StepID: st.CurrentStepID, Stage: stage, Runs: []resultRun{{Command: st.Config.Tests[0], Commit: st.Config.CodeRevision, ExitCode: &zero, OutputPath: "aidlc/evidence/output.txt"}}}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		boundaryFile(t, s, st.Config.TestResults[0], string(raw))
		boundaryFile(t, s, "aidlc/evidence/output.txt", "verified")
		st, err = s.Save(st, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
		st, err = s.Begin(st.ID, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		flowGit(t, s.Root, "worktree", "add", "--detach", root, "HEAD")
		st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", Root: root, Session: "reviewer", CoordinatorSession: "coordinator"})
		if err != nil {
			t.Fatal(err)
		}
		st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Root: root, Session: "reviewer", Target: st.Review.Target, Status: "pass", Summary: "actual evidence inspected"})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.CaptureApproval(st.ID, "user", "", stage, "approve result"); err != nil {
			t.Fatal(err)
		}
		a := st.Approval
		st, err = s.Decide(st.ID, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: stage, Quote: "approve"})
		if err != nil {
			t.Fatal(err)
		}
		st, err = s.Finish(st.ID, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
	}
	if st.Status != "completed" {
		t.Fatal("last selected execution did not complete")
	}
}

func TestExecutionPlanApprovalFinishDropsPreviousUnits(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("initialize")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Finish(st.ID, st.Revision); err == nil {
		t.Fatal("unstarted execution finished")
	}
	st.Config.Units = []Unit{{ID: "old", StepID: st.CurrentStepID, Status: "integrated"}}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", Root: root, Session: "reviewer", CoordinatorSession: "coordinator"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Root: root, Session: "reviewer", Target: st.Review.Target, Status: "pass", Summary: "verified settings"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Approval == nil || st.Approval.Status != "pending" {
		t.Fatal("review did not request human result approval")
	}
	if _, err = s.Finish(st.ID, st.Revision); err == nil {
		t.Fatal("review alone finished execution")
	}
	if err = s.CaptureApproval(st.ID, "user", "", "A", "I approve the initialization result"); err != nil {
		t.Fatal(err)
	}
	a := st.Approval
	st, err = s.Decide(st.ID, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: "A", Quote: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Finish(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if st.CurrentStepID != "s02" || st.Stage != "discovery" || st.ExecutionPlan.Approved != nil || st.Accepted["s01"].StepID != "s01" {
		t.Fatalf("bootstrap completion: %+v", st)
	}
	if len(st.Config.Units) != 0 {
		t.Fatal("previous run Units carried into next execution")
	}
}

func TestExecutionPlanApprovalAdoptionRechecksEvidence(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve both"); err != nil {
		t.Fatal(err)
	}
	name := s.documentPath(st, "Requirements")
	raw, err := filestore.ReadFile(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	if err = filestore.WriteFile(s.Root, name, append(raw, []byte("\nChanged accepted scope.\n")...)); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Approval != nil || st.Review.Status != "" {
		t.Fatal("changed evidence retained across initial plan adoption")
	}
}
