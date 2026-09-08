package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func reassignFixture(t *testing.T) (Store, State, UnitRequest) {
	t.Helper()
	s, st, worker := unitFixture(t)
	st.Config.Units[0].Status = "needs_confirmation"
	st.Config.Units[1].Status = "needs_confirmation"
	var err error
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, UnitRequest{Action: "reassign", Unit: "a", Session: "new-worker", Root: worker, Commit: st.Config.CodeRevision, Reason: "old worker stopped", PreviousRunStopped: true}
}
func TestFlowUnitReassignStoppedAndIdentity(t *testing.T) {
	s, st, r := reassignFixture(t)
	for _, tc := range []struct {
		name   string
		change func(*UnitRequest)
	}{{"not stopped", func(r *UnitRequest) { r.PreviousRunStopped = false }}, {"reason", func(r *UnitRequest) { r.Reason = "" }}, {"run input", func(r *UnitRequest) { r.RunID = "old" }}, {"head", func(r *UnitRequest) { r.Commit = "bad" }}} {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			tc.change(&bad)
			if _, err := s.Unit(st.ID, st.Revision, bad); err == nil {
				t.Fatal("accepted invalid reassign")
			}
		})
	}
	next, err := s.Unit(st.ID, st.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if next.Config.Units[0].Status != "running" || next.Config.Units[0].BaseCommit != st.Config.Units[0].BaseCommit || a.RunID == "" || a.Reason != r.Reason || !a.PreviousRunStopped {
		t.Fatal("identity/progress lost")
	}
	old := r
	old.Action = "result"
	old.RunID = "previous-run"
	if _, err := s.Unit(st.ID, next.Revision, old); err == nil {
		t.Fatal("old run accepted")
	}
	// A later explicit pause/resume must allow a fresh reassignment after a successful one.
	next, err = s.Transition(st.ID, next.Revision, TransitionRequest{Action: "pause", Reason: "later stop"})
	if err != nil {
		t.Fatal(err)
	}
	next, err = s.Transition(st.ID, next.Revision, TransitionRequest{Action: "resume", Reason: "later handoff"})
	if err != nil {
		t.Fatal(err)
	}
	r.Session = "later-worker"
	next, err = s.Unit(st.ID, next.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	later, _ := s.assignment(st.ID, "a")
	if later.RunID == a.RunID {
		t.Fatal("reused successful old run")
	}
}
func TestFlowUnitReassignStateFailureRetry(t *testing.T) {
	s, st, r := reassignFixture(t)
	before, err := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if err != nil {
		t.Fatal(err)
	}
	s.write = func(string, string, []byte) error { return errors.New("state save failure") }
	if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
		t.Fatal("state failure hidden")
	}
	a, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if !bytes.Equal(before, after) {
		t.Fatal("state failure changed state")
	}
	bad := r
	bad.Session = "different"
	if _, err := s.Unit(st.ID, st.Revision, bad); err == nil {
		t.Fatal("overwrote incomplete reassignment")
	}
	result := r
	result.Action = "result"
	result.RunID = a.RunID
	if _, err := s.Unit(st.ID, st.Revision, result); err == nil {
		t.Fatal("result accepted before state save")
	}
	s.write = nil
	next, err := s.Unit(st.ID, st.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	retry, _ := s.assignment(st.ID, "a")
	if retry.RunID != a.RunID || next.Config.Units[0].Status != "running" {
		t.Fatal("retry changed run")
	}
}
func TestFlowUnitReassignMissingRuntimeScope(t *testing.T) {
	s, st, r := reassignFixture(t)
	st.Config.Units[1].Scope = []string{"a.txt"}
	var err error
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
		t.Fatal("missing runtime bypassed overlapping scope")
	}
}

func TestFlowUnitReassignOtherAssignments(t *testing.T) {
	for _, tc := range []struct{ name, otherStatus, raw string }{{"running missing", "running", ""}, {"corrupt paused", "needs_confirmation", "{"}, {"empty paused", "needs_confirmation", "{}"}} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, r := reassignFixture(t)
			st.Config.Units[1].Status = tc.otherStatus
			var err error
			st, err = s.Save(st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != "" {
				p := filepath.Join(s.Root, s.assignmentPath(st.ID, "b"))
				os.MkdirAll(filepath.Dir(p), 0700)
				os.WriteFile(p, []byte(tc.raw), 0600)
			}
			if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
				t.Fatal("accepted invalid other assignment")
			}
		})
	}
}
func TestFlowUnitReassignRuntimeFailure(t *testing.T) {
	s, st, r := reassignFixture(t)
	p := filepath.Join(s.Root, s.assignmentPath(st.ID, "a"))
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
		t.Fatal("runtime failure hidden")
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if !bytes.Equal(before, after) {
		t.Fatal("runtime failure saved state")
	}
}
func TestFlowUnitReassignScopeAndDependency(t *testing.T) {
	for _, name := range []string{"scope", "dependency"} {
		t.Run(name, func(t *testing.T) {
			s, st, r := reassignFixture(t)
			if name == "dependency" {
				st.Config.Units[0].DependsOn = []string{"b"}
				var err error
				st, err = s.Save(st, st.Revision)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				os.WriteFile(filepath.Join(r.Root, "outside.txt"), []byte("outside"), 0600)
				flowGit(t, r.Root, "add", "outside.txt")
				flowGit(t, r.Root, "commit", "-qm", "outside")
				r.Commit = flowGit(t, r.Root, "rev-parse", "HEAD")
			}
			if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
				t.Fatal("accepted invalid handoff")
			}
		})
	}
}

func TestFlowUnitReassignOwnCorruptAssignment(t *testing.T) {
	s, st, r := reassignFixture(t)
	p := filepath.Join(s.Root, s.assignmentPath(st.ID, "a"))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, r); err == nil {
		t.Fatal("overwrote corrupt target assignment")
	}
}

func TestFlowUnitReassignReplacesOldRunWithoutOldAccess(t *testing.T) {
	s, st, r := reassignFixture(t)
	st.Config.Units[0].ResultCommit = st.Config.CodeRevision
	var err error
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	old := UnitRequest{Action: "claim", Unit: "a", Session: "stopped", Root: "/missing/old/worker", RunID: "old-run"}
	raw, _ := json.Marshal(old)
	p := filepath.Join(s.Root, s.assignmentPath(st.ID, "a"))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	next, err := s.Unit(st.ID, st.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != st.ID || next.Config.Units[0].ResultCommit != st.Config.Units[0].ResultCommit {
		t.Fatal("lost identity or prior result")
	}
}
