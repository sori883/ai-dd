package flow

import (
	"strings"
	"testing"
)

func TestBoundaryReviewRequiresEndPass(t *testing.T) {
	s, st := boundaryFixture(t)
	reviewRoot := t.TempDir()
	flowGit(t, s.Root, "worktree", "add", "--detach", reviewRoot, "HEAD")
	request := ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: reviewRoot}
	if _, err := s.Review(st.ID, st.Revision, request); err == nil {
		t.Fatal("unstarted incomplete review assigned")
	}
	var err error
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(st.ID, st.Revision, request); err == nil {
		t.Fatal("failed end Sensor review assigned")
	}
	boundaryDoc(t, s, st, "Requirements")
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: flowGit(t, s.Root, "rev-parse", "HEAD")}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, request)
	if err != nil {
		t.Fatal(err)
	}
	target := st.Review.Target
	request.Action = "accept"
	request.Target = target
	request.Status = "pass"
	request.Summary = "reviewed current requirements"
	name := s.documentPath(st, "Requirements")
	boundaryFile(t, s, name, "changed")
	if _, err = s.Review(st.ID, st.Revision, request); err == nil {
		t.Fatal("stale target accepted")
	}
	boundaryDoc(t, s, st, "Requirements")
	st, err = s.Review(st.ID, st.Revision, request)
	if err != nil || st.Review.Status != "pass" || !strings.EqualFold(st.Review.Target, target) {
		t.Fatalf("current report: %+v %v", st, err)
	}
}
