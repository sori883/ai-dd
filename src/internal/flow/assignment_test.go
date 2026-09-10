package flow

import (
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/assignment"
)

func TestUnitAssignment(t *testing.T) {
	s, st, worker := unitFixture(t)
	registry := assignment.Store{Root: s.Root}
	reg, err := registry.Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "known workers stopped"})
	if err != nil {
		t.Fatal(err)
	}
	req := UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Session: "worker", Root: worker, RegistryEpoch: reg.Epoch, RequestID: "claim-a", CoordinatorSession: "main"}
	s.write = func(string, string, []byte) error { return errors.New("progress failure") }
	if _, err := s.Unit(st.ID, st.Revision, req); err == nil {
		t.Fatal("save failure hidden")
	}
	saved, err := registry.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Reservations) != 1 {
		t.Fatalf("reservation must precede progress: %d", len(saved.Reservations))
	}
	first := saved.Reservations[0]
	if first.Unit != "a" || first.RunID == "" {
		t.Fatalf("missing unit identity: %+v", first)
	}
	changed := req
	changed.RequestID = "another"
	if _, err := s.Unit(st.ID, st.Revision, changed); err == nil {
		t.Fatal("different recovery accepted")
	}
	s.write = nil
	next, err := s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Unit(st.ID, st.Revision, req)
	if err != nil || again.Revision != next.Revision {
		t.Fatalf("lost response retry: %+v %v", again, err)
	}
	runtime, err := s.assignment(st.ID, "a")
	if err != nil || runtime.RunID != first.RunID {
		t.Fatalf("run changed: %+v %v", runtime, err)
	}
	runtime.Action = "result"
	runtime.Commit = st.Config.CodeRevision
	reported, err := s.Unit(st.ID, next.Revision, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if reported.Config.Units[0].Status != "reported" {
		t.Fatal("result not reported")
	}
	reserve := AssignmentRequest{RegistryEpoch: reg.Epoch, RequestID: "unitless", StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: worker, Session: "another"}
	if _, err := s.ReserveAssignment(st.ID, reported.Revision, "another-main", reserve); err == nil {
		t.Fatal("reported root released implicitly")
	}
}

func TestUnitAssignmentWithoutUnit(t *testing.T) {
	s, st, worker := unitFixture(t)
	reg, err := (assignment.Store{Root: s.Root}).Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	req := AssignmentRequest{RegistryEpoch: reg.Epoch, RequestID: "unitless", StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: worker, Session: "worker"}
	got, err := s.ReserveAssignment(st.ID, st.Revision, "main", req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unit != "" || got.IntentID != st.ID {
		t.Fatalf("unitless identity: %+v", got)
	}
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Root: worker, Session: "worker-a", RegistryEpoch: reg.Epoch, RequestID: "claim", CoordinatorSession: "other"}); err == nil {
		t.Fatal("unitless reservation bypassed")
	}
}

func TestUnitAssignmentReassign(t *testing.T) {
	s, st, worker := unitFixture(t)
	registry := assignment.Store{Root: s.Root}
	reg, err := registry.Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	req := UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Session: "worker", Root: worker, RegistryEpoch: reg.Epoch, RequestID: "claim", CoordinatorSession: "main"}
	st, err = s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "worker stopped"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "handoff"})
	if err != nil {
		t.Fatal(err)
	}
	req.Action = "reassign"
	req.RequestID = strings.Repeat("r", 160)
	req.Session = "new-worker"
	req.PreviousRunStopped = true
	req.Reason = "old worker and known commands stopped, results collected"
	req.Commit = st.Config.CodeRevision
	next, err := s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	r, err := registry.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Reservations) != 2 || r.Reservations[0].Status != "released" || r.Reservations[1].Status == "released" {
		t.Fatalf("reassign registry: %+v", r.Reservations)
	}
	retry, err := s.Unit(st.ID, st.Revision, req)
	if err != nil || retry.Revision != next.Revision {
		t.Fatalf("reassign response retry: %+v %v", retry, err)
	}
}
func TestUnitAssignmentLegacyReassign(t *testing.T) {
	s, st, worker := unitFixture(t)
	st.Config.Units[0].Status = "needs_confirmation"
	st, err := saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	req := UnitRequest{StepID: st.CurrentStepID, Action: "reassign", Unit: "a", Session: "new", Root: worker, PreviousRunStopped: true, Reason: "old stopped", Commit: st.Config.CodeRevision}
	if _, err := s.Unit(st.ID, st.Revision, req); err == nil {
		t.Fatal("legacy unit reservation backfill accepted")
	}
}

func TestAssignmentStage(t *testing.T) {
	s, st, _ := unitFixture(t)
	if _, err := s.AssignmentStage(st.ID, st.CurrentStepID, "aidlc-worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignmentStage(st.ID, st.CurrentStepID, "unknown"); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := s.AssignmentStage(st.ID, "old-step", "aidlc-worker"); err == nil {
		t.Fatal("old step accepted")
	}
	st, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "tdd", Reason: "test begin required after reopen"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Entry != nil {
		t.Fatal("reopened fixture has entry")
	}
	if _, err := s.AssignmentStage(st.ID, st.CurrentStepID, "aidlc-worker"); err == nil {
		t.Fatal("worker without begin accepted")
	}
	if _, err := s.AssignmentStage(st.ID, st.CurrentStepID, "aidlc-reviewer"); err != nil {
		t.Fatalf("read-only blocked by worker sensor: %v", err)
	}
}

func TestUnitAssignmentPendingMutation(t *testing.T) {
	for _, operation := range []string{"save", "begin", "reopen"} {
		t.Run(operation, func(t *testing.T) {
			s, st, worker := unitFixture(t)
			reg, err := (assignment.Store{Root: s.Root}).Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
			if err != nil {
				t.Fatal(err)
			}
			req := UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Session: "worker", Root: worker, RegistryEpoch: reg.Epoch, RequestID: "claim", CoordinatorSession: "main"}
			s.write = func(string, string, []byte) error { return errors.New("progress failure") }
			if _, err := s.Unit(st.ID, st.Revision, req); err == nil {
				t.Fatal("expected save failure")
			}
			s.write = nil
			switch operation {
			case "save":
				_, err = s.Save(st, st.Revision)
			case "begin":
				_, err = s.Begin(st.ID, st.Revision)
			case "reopen":
				_, err = s.Reopen(st.ID, st.Revision, st.CurrentStepID, "another request")
			}
			if err == nil {
				t.Fatal("pending reservation did not protect progress")
			}
			if _, err := s.Unit(st.ID, st.Revision, req); err != nil {
				t.Fatalf("identical recovery lost: %v", err)
			}
		})
	}
}

func TestUnitAssignmentLegacyResult(t *testing.T) {
	s, st, worker := unitFixture(t)
	st.Config.Units[0].Status = "running"
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(worker)
	if err != nil {
		t.Fatal(err)
	}
	legacy := UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Root: root, Session: "old", RunID: "old-run"}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := filestore.WriteFile(s.Root, s.assignmentPath(st.ID, "a"), raw); err != nil {
		t.Fatal(err)
	}
	legacy.Action = "result"
	legacy.Commit = st.Config.CodeRevision
	if _, err := s.Unit(st.ID, st.Revision, legacy); err == nil {
		t.Fatal("legacy worker result bypassed managed registry")
	}
}

func TestUnitAssignmentReassignAdmissionFailure(t *testing.T) {
	s, st, worker := unitFixture(t)
	reg, err := (assignment.Store{Root: s.Root}).Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	claim := UnitRequest{StepID: st.CurrentStepID, Action: "claim", Unit: "a", Session: "worker", Root: worker, RegistryEpoch: reg.Epoch, RequestID: "claim", CoordinatorSession: "main"}
	st, err = s.Unit(st.ID, st.Revision, claim)
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "stopped"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "handoff"})
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	flowGit(t, s.Root, "worktree", "add", "--detach", other, st.Config.CodeRevision)
	if _, err := s.ReserveAssignment(st.ID, st.Revision, "another-main", AssignmentRequest{RegistryEpoch: reg.Epoch, RequestID: "other", StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: other, Session: "other"}); err != nil {
		t.Fatal(err)
	}
	req := claim
	req.Action = "reassign"
	req.RequestID = "reassign"
	req.Root = other
	req.Session = "new"
	req.Commit = st.Config.CodeRevision
	req.PreviousRunStopped = true
	req.Reason = "old stopped and collected"
	if _, err := s.Unit(st.ID, st.Revision, req); err == nil {
		t.Fatal("occupied replacement accepted")
	}
	saved, err := (assignment.Store{Root: s.Root}).Read()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Reservations[0].Status == "released" {
		t.Fatal("failed replacement partially released old reservation")
	}
}
