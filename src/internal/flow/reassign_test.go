package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func reassignFixture(t *testing.T) (Store, State, UnitRequest) {
	t.Helper()
	s, st, worker := unitFixture(t)
	var claimErr error
	st, claimErr = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Session: "old-worker", Root: worker})
	if claimErr != nil {
		t.Fatal(claimErr)
	}
	st.Config.Units[0].Status = "needs_confirmation"
	st.Config.Units[1].Status = "needs_confirmation"
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, UnitRequest{StepID: "s04", Action: "reassign", Unit: "a", Session: "new-worker", Root: worker, Reason: "old worker stopped", PreviousRunStopped: true}
}
func TestFlowUnitReassignStoppedAndIdentity(t *testing.T) {
	s, st, r := reassignFixture(t)
	for _, tc := range []struct {
		name   string
		change func(*UnitRequest)
	}{{"not stopped", func(r *UnitRequest) { r.PreviousRunStopped = false }}, {"reason", func(r *UnitRequest) { r.Reason = "" }}, {"run input", func(r *UnitRequest) { r.RunID = "old" }}} {
		t.Run(tc.name, func(t *testing.T) {
			bad := r
			tc.change(&bad)
			if _, err := assignmentUnit(t, s, st.ID, st.Revision, bad); err == nil {
				t.Fatal("accepted invalid reassign")
			}
		})
	}
	next, err := assignmentUnit(t, s, st.ID, st.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if next.Config.Units[0].Status != "running" || a.RunID == "" || a.Reason != r.Reason || !a.PreviousRunStopped {
		t.Fatal("identity/progress lost")
	}
	old := r
	old.Action = "result"
	old.RunID = "previous-run"
	if _, err := assignmentUnit(t, s, st.ID, next.Revision, old); err == nil {
		t.Fatal("old run accepted")
	}
	// A later explicit pause/resume must allow a fresh reassignment after a successful one.
	next, err = transitionExecutionFixture(t, s, st.ID, next.Revision, TransitionRequest{Action: "pause", Reason: "later stop"})
	if err != nil {
		t.Fatal(err)
	}
	next, err = transitionExecutionFixture(t, s, st.ID, next.Revision, TransitionRequest{Action: "resume", Reason: "later handoff"})
	if err != nil {
		t.Fatal(err)
	}
	r.Session = "later-worker"
	next, err = assignmentUnit(t, s, st.ID, next.Revision, r)
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
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
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
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, bad); err == nil {
		t.Fatal("overwrote incomplete reassignment")
	}
	result := r
	result.Action = "result"
	result.RunID = a.RunID
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, result); err == nil {
		t.Fatal("result accepted before state save")
	}
	s.write = nil
	next, err := assignmentUnit(t, s, st.ID, st.Revision, r)
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
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
		t.Fatal("missing runtime bypassed overlapping scope")
	}
}

func TestFlowUnitReassignOtherAssignments(t *testing.T) {
	for _, tc := range []struct{ name, otherStatus, raw string }{{"running missing", "running", ""}, {"corrupt paused", "needs_confirmation", "{"}, {"empty paused", "needs_confirmation", "{}"}} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, r := reassignFixture(t)
			st.Config.Units[1].Status = tc.otherStatus
			var err error
			st, err = saveExecutionFixture(t, s, st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != "" {
				p := filepath.Join(s.Root, s.assignmentPath(st.ID, "b"))
				os.MkdirAll(filepath.Dir(p), 0700)
				os.WriteFile(p, []byte(tc.raw), 0600)
			}
			if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
				t.Fatal("accepted invalid other assignment")
			}
		})
	}
}
func TestFlowUnitReassignRuntimeFailure(t *testing.T) {
	s, st, r := reassignFixture(t)
	p := filepath.Join(s.Root, s.assignmentPath(st.ID, "a"))
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
		t.Fatal("runtime failure hidden")
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if !bytes.Equal(before, after) {
		t.Fatal("runtime failure saved state")
	}
}
func TestFlowUnitReassignScopeAndDependency(t *testing.T) {
	s, st, r := reassignFixture(t)
	st.Config.Units[0].DependsOn = []string{"b"}
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
		t.Fatal("accepted invalid dependency handoff")
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
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
		t.Fatal("overwrote corrupt target assignment")
	}
}

func TestFlowUnitReassignReplacesOldRunWithoutOldAccess(t *testing.T) {
	s, st, r := reassignFixture(t)
	st.Config.Units[0].ResultSHA256 = verificationTestSHA(t, s.Root, st.Config.VerificationPaths)
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	old.Root = "/missing/old/worker"
	raw, _ := json.Marshal(old)
	p := filepath.Join(s.Root, s.assignmentPath(st.ID, "a"))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	next, err := assignmentUnit(t, s, st.ID, st.Revision, r)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != st.ID || next.Config.Units[0].ResultSHA256 != st.Config.Units[0].ResultSHA256 {
		t.Fatal("lost identity or prior result")
	}
}

func TestFlowUnitReassignPendingBlocksStateUpdates(t *testing.T) {
	for _, operation := range []string{"configure", "other_unit", "pause"} {
		t.Run(operation, func(t *testing.T) {
			s, st, r := reassignFixture(t)
			s.write = func(string, string, []byte) error { return errors.New("state save failure") }
			if _, err := assignmentUnit(t, s, st.ID, st.Revision, r); err == nil {
				t.Fatal("expected save failure")
			}
			original, err := s.assignment(st.ID, "a")
			if err != nil {
				t.Fatal(err)
			}
			before := mustReassignBytes(t, s.Root, s.path(st.ID))
			s.write = nil
			switch operation {
			case "configure":
				_, err = saveExecutionFixture(t, s, st, st.Revision)
			case "other_unit":
				worker := filepath.Join(t.TempDir(), "worker-b")
				if err := os.MkdirAll(worker, 0700); err != nil {
					t.Fatal(err)
				}
				other := r
				other.Unit = "b"
				other.Session = "b"
				other.Root = worker
				_, err = assignmentUnit(t, s, st.ID, st.Revision, other)
			case "pause":
				_, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "unrelated update"})
			}
			if err == nil || !strings.Contains(err.Error(), "incomplete") {
				t.Fatalf("pending request did not block %s: %v", operation, err)
			}
			if !bytes.Equal(before, mustReassignBytes(t, s.Root, s.path(st.ID))) {
				t.Fatal("pending state changed")
			}
			if _, err := os.Stat(filepath.Join(s.Root, s.assignmentPath(st.ID, "b"))); !os.IsNotExist(err) {
				t.Fatalf("other runtime side effect: %v", err)
			}
			_, err = assignmentUnit(t, s, st.ID, st.Revision, r)
			if err != nil {
				t.Fatal(err)
			}
			recovered, _ := s.assignment(st.ID, "a")
			if recovered.RunID != original.RunID {
				t.Fatal("recovery issued another run")
			}

		})
	}
}
func mustReassignBytes(t *testing.T, root, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFlowUnitResultLiteralPaths(t *testing.T) {
	s, st, worker := unitFixture(t)
	name := "src/ 日本\nfile.go"
	st.Config.Units[0].Scope = []string{name}
	st.Config.Units[0].VerificationPaths = []string{name}
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "a", Session: "a", Root: worker})
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(worker, "src"), 0700)
	if err := os.WriteFile(filepath.Join(worker, name), []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	assignment, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	request := prepareUnitResultFixture(t, s, st, UnitRequest{StepID: "s04", Action: "result", Unit: "a", Root: worker, Session: "a", RunID: assignment.RunID})
	if err := os.WriteFile(filepath.Join(worker, name), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, request); err == nil {
		t.Fatal("changed literal bytes accepted")
	}
	if err := os.WriteFile(filepath.Join(worker, name), []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, request); err != nil {
		t.Fatal(err)
	}
}
