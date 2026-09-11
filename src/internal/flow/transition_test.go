package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func passReview(t *testing.T, s Store, st State) State {
	t.Helper()
	if st.Entry == nil {
		var err error
		st, err = s.Begin(st.ID, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
	}
	root := flowReviewRoot(t, s.Root)
	st, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Session: "reviewer", Root: root, Target: gate.Target, Status: "pass", Summary: "Independent check passed"})
	if err != nil {
		t.Fatal(err)
	}
	return st
}
func TestFlowTransitionGatesAndStages(t *testing.T) {
	s, st := sensorFixture(t)
	if _, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"}); err == nil {
		t.Fatal("advanced without review")
	}
	st = passReview(t, s, st)
	previous := st.Revision
	var err error
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"})
	if err != nil || st.Stage != "planning" {
		t.Fatalf("planning %+v %v", st, err)
	}
	if _, err := transitionExecutionFixture(t, s, st.ID, previous, TransitionRequest{Action: "advance"}); err == nil {
		t.Fatal("duplicate advance accepted")
	}
	boundaryDoc(t, s, st, "ImplementationPlan")
	st.Config.Plan = "Direct change"
	st.Config.Tests = []string{"go test"}
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st = passReview(t, s, st)
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"})
	if err != nil || st.Stage != "tdd" {
		t.Fatalf("tdd %+v %v", st, err)
	}
	file := "results.txt"
	if err := os.WriteFile(filepath.Join(s.Root, file), []byte("acceptance PASS"), 0600); err != nil {
		t.Fatal(err)
	}
	st.Config.Artifacts = append(st.Config.Artifacts, Artifact{Path: file, Kind: "test", Stage: "tdd"})
	prepareBoundaryResults(t, s, &st)
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"integration", "completed"} {
		if st.Stage == "integration" {
			boundaryDoc(t, s, st, "CurrentAnalysis")
			boundaryDoc(t, s, st, "Architecture")
			st, err = s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{boundaryDeclaration(t, s, st, "Knowledge")}})
			if err != nil {
				t.Fatal(err)
			}
			prepareBoundaryResults(t, s, &st)
			st, err = saveExecutionFixture(t, s, st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
		}
		st = passReview(t, s, st)
		st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"})
		if err != nil {
			t.Fatal(err)
		}
		if want == "completed" && st.Status != want || want != "completed" && st.Stage != want {
			t.Fatalf("want %s: %+v", want, st)
		}
	}
	if _, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"}); err == nil {
		t.Fatal("completed advanced")
	}
}
func TestFlowTransitionWaitPauseResumeReopen(t *testing.T) {
	s, st := sensorFixture(t)
	for _, r := range []TransitionRequest{{Action: "wait"}, {Action: "pause"}, {Action: "reopen", Stage: "tdd"}} {
		if _, err := transitionExecutionFixture(t, s, st.ID, st.Revision, r); err == nil {
			t.Fatal("missing reason accepted")
		}
	}
	var err error
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "wait", Reason: "Need user answer", ResumeCondition: "Answer provided"})
	if err != nil || st.Status != "waiting" {
		t.Fatalf("wait %+v %v", st, err)
	}
	if _, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"}); err == nil {
		t.Fatal("waiting advanced")
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "Answer provided"})
	if err != nil || st.Status != "active" {
		t.Fatalf("resume %+v %v", st, err)
	}
	st.Config.Units = []Unit{{ID: "one", Status: "running"}}
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "Interruption"})
	if err != nil || st.Config.Units[0].Status != "needs_confirmation" {
		t.Fatalf("pause %+v %v", st, err)
	}
	if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "Revise purpose"}); err == nil {
		t.Fatal("unconfirmed worker allowed plan change")
	}
	// Isolated fixture: the worker has now been checked and stopped.
	st.Config.Units = nil
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "Revise purpose"})
	if err != nil || st.Status != "active" || st.Stage != "discovery" || st.Review.Status != "" {
		t.Fatalf("reopen %+v %v", st, err)
	}
}
