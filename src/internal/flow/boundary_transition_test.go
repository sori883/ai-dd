package flow

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func TestBoundaryTransitionBegin(t *testing.T) {
	s, st := boundaryFixture(t)
	if err := s.CheckWork(st.ID); err == nil {
		t.Fatal("unstarted work accepted")
	}
	started, err := s.Begin(st.ID, st.Revision)
	if err != nil || started.Revision != st.Revision+1 || started.Entry == nil {
		t.Fatalf("begin: %+v %v", started, err)
	}
	same, err := s.Begin(st.ID, started.Revision)
	if err != nil || !reflect.DeepEqual(same, started) {
		t.Fatalf("nonidempotent begin: %+v %v", same, err)
	}
	if _, err = s.Begin(st.ID, st.Revision); err == nil {
		t.Fatal("stale begin accepted")
	}
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatal(err)
	}
	paused, err := s.Transition(st.ID, started.Revision, TransitionRequest{Action: "pause", Reason: "break"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err == nil {
		t.Fatal("paused work accepted")
	}
	resumed, err := s.Transition(st.ID, paused.Revision, TransitionRequest{Action: "resume", Reason: "back"})
	if err != nil || !reflect.DeepEqual(resumed.Entry, started.Entry) {
		t.Fatalf("resume lost entry: %+v %v", resumed, err)
	}
	reopened, err := s.Transition(st.ID, resumed.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "revise"})
	if err != nil || reopened.Entry != nil {
		t.Fatalf("reopen kept entry: %+v %v", reopened, err)
	}
}
func TestBoundaryTransitionFailure(t *testing.T) {
	s, st := boundaryFixture(t)
	s.write = func(string, string, []byte) error { return fs.ErrPermission }
	if _, err := s.Begin(st.ID, st.Revision); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("write failure: %v", err)
	}
	got, err := s.Read(st.ID)
	if err != nil || got.Entry != nil || got.Revision != st.Revision {
		t.Fatalf("partial begin: %+v %v", got, err)
	}
}
func TestBoundaryTransitionAcceptAndImmutable(t *testing.T) {
	s, st := boundaryFixture(t)
	req := boundaryDoc(t, s, st, "Requirements")
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: flowGit(t, s.Root, "rev-parse", "HEAD")}
	var err error
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	st.Review = Gate{Status: "pass", Target: gate.Target}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "advance"})
	if err != nil || st.Stage != "planning" || st.Entry != nil || len(st.Accepted) != 1 {
		t.Fatalf("advance: %+v %v", st, err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, "src/new.go", "package changed")
	boundaryDoc(t, s, st, "CurrentAnalysis")
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatalf("legitimate changes frozen: %v", err)
	}
	boundaryFile(t, s, req, "changed")
	if err = s.CheckWork(st.ID); err == nil {
		t.Fatal("changed accepted input allowed")
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "requirements changed"})
	if err != nil || len(st.Accepted) != 0 || st.Entry != nil {
		t.Fatalf("reopen acceptances: %+v %v", st, err)
	}
}
func TestBoundaryTransitionMaterialsConfigure(t *testing.T) {
	s, st := boundaryFixture(t)
	var err error
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st.Config.NoMaterialsReason = "new"
	st, err = s.Save(st, st.Revision)
	if err != nil || st.Entry != nil {
		t.Fatalf("material change did not invalidate: %+v %v", st, err)
	}
}

func TestBoundaryTransitionIntegrationDoesNotFreezeMaterials(t *testing.T) {
	s, st := boundaryFixture(t)
	req := boundaryDoc(t, s, st, "Requirements")
	plan := boundaryDoc(t, s, st, "ImplementationPlan")
	boundaryDoc(t, s, st, "CurrentAnalysis")
	boundaryDoc(t, s, st, "Architecture")
	boundaryFile(t, s, "inputs/source", "before")
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Stage = "tdd"
	st.Entry = &StageEntry{Stage: "tdd"}
	st.Accepted = map[string]StageAcceptance{"discovery": {Stage: "discovery", ReviewTarget: strings.Repeat("a", 64), Outputs: []FileVersion{boundaryVersion(t, s, req)}}, "planning": {Stage: "planning", ReviewTarget: strings.Repeat("b", 64), Outputs: []FileVersion{boundaryVersion(t, s, plan)}}}
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, MaterialSources: []string{"inputs"}, ADR: ADR{Reason: "none"}, CodeRevision: head, DirectCommit: head, Plan: "Implement", Tests: []string{"go test"}}
	prepareBoundaryResults(t, s, &st)
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("tdd: %+v %v", g, err)
	}
	st.Review = Gate{Status: "pass", Target: g.Target}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "advance"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatalf("integration cannot start with material directory: %v", err)
	}
	boundaryFile(t, s, "inputs/source", "after")
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatalf("mutable sources frozen: %v", err)
	}
}
