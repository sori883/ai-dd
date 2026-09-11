package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"os"
	"path/filepath"
	"testing"
)

func unitWithoutGitFixture(t *testing.T) (Store, State, assignment.Registry) {
	t.Helper()
	s, st := sensorFixture(t)
	fixtureExecutionStage(t, s, &st, "tdd")
	prepareBoundaryStage(t, s, &st)
	st.Config.VerificationPaths = []string{"code"}
	st.Config.Units = []Unit{{ID: "a", Bolt: "one", Status: "pending", Scope: []string{"code"}, Tests: []string{"test"}}, {ID: "b", Bolt: "two", Status: "pending", DependsOn: []string{"a"}, Scope: []string{"code"}, Tests: []string{"test"}}}
	st.Config.TestResults = []string{"aidlc/evidence/unit.json"}
	st, err := saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Join(s.Root, ".git")); err != nil {
		t.Fatal(err)
	}
	registry, err := (assignment.Store{Root: s.Root}).Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "new project"})
	if err != nil {
		t.Fatal(err)
	}
	return s, st, registry
}
func unitWithoutGitEvidence(t *testing.T, s Store, st State, r UnitRequest) UnitRequest {
	t.Helper()
	digest, err := ComputeVerification(r.Root, st.Config.VerificationPaths)
	if err != nil {
		t.Fatal(err)
	}
	r.VerificationSHA256 = digest.SHA256
	zero := 0
	doc := resultDocument{StepID: st.CurrentStepID, Stage: st.Stage, VerificationScope: "unit", VerificationSHA256: digest.SHA256, UnitID: r.Unit, RunID: r.RunID, Runs: []resultRun{{UnitID: r.Unit, Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/output"}}}
	if err := os.MkdirAll(filepath.Join(r.Root, "aidlc/evidence"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "aidlc/evidence/output"), []byte("pass"), 0644); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(r.Root, "aidlc/evidence/unit.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestUnitWithoutGit(t *testing.T) {
	s, st, reg := unitWithoutGitFixture(t)
	req := UnitRequest{Action: "claim", StepID: st.CurrentStepID, Unit: "a", Root: s.Root, Session: "worker-a", CoordinatorSession: "main", RegistryEpoch: reg.Epoch, RequestID: "claim-a"}
	dependent := req
	dependent.Unit = "b"
	if _, err := s.Unit(st.ID, st.Revision, dependent); err == nil {
		t.Fatal("dependency accepted early")
	}
	var err error
	st, err = s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	run.Action = "result"
	run = unitWithoutGitEvidence(t, s, st, run)
	bad := run
	bad.RunID = "other"
	if _, err = s.Unit(st.ID, st.Revision, bad); err == nil {
		t.Fatal("foreign run accepted")
	}
	st, err = s.Unit(st.ID, st.Revision, run)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	integrate := UnitRequest{Action: "integrate", StepID: st.CurrentStepID, Unit: "a"}
	if _, err = s.Unit(st.ID, st.Revision, integrate); err == nil {
		t.Fatal("stale submission integrated")
	}
	run = unitWithoutGitEvidence(t, s, st, run)
	st, err = s.Unit(st.ID, st.Revision, run)
	if err != nil {
		t.Fatalf("reported resubmit: %v", err)
	}
	st, err = s.Unit(st.ID, st.Revision, integrate)
	if err != nil {
		t.Fatal(err)
	}
	occupied := req
	occupied.Unit = "b"
	occupied.RequestID = "claim-b"
	occupied.Session = "worker-b"
	if _, err = s.Unit(st.ID, st.Revision, occupied); err == nil {
		t.Fatal("result auto-released worker")
	}
	registry := assignment.Store{Root: s.Root}
	saved, err := registry.Read()
	if err != nil {
		t.Fatal(err)
	}
	v := saved.Reservations[0]
	if _, err = registry.Release(v.ID, "main", v.EntryRevision, assignment.ReleaseRequest{RegistryEpoch: reg.Epoch, RequestID: "release-a", PreviousRunStopped: true, NoMoreRequests: true, Reason: "collected"}); err != nil {
		t.Fatal(err)
	}
	st, err = s.Unit(st.ID, st.Revision, occupied)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("B changes shared content"), 0644); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Read(st.ID)
	if err != nil || latest.Config.Units[0].Status != "integrated" {
		t.Fatalf("past integration invalidated %+v %v", latest, err)
	}
}
func TestUnitWithoutGitReassign(t *testing.T) {
	s, st, reg := unitWithoutGitFixture(t)
	req := UnitRequest{Action: "claim", StepID: st.CurrentStepID, Unit: "a", Root: s.Root, Session: "worker", CoordinatorSession: "main", RegistryEpoch: reg.Epoch, RequestID: "claim"}
	var err error
	st, err = s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "interrupted"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "continue"})
	if err != nil {
		t.Fatal(err)
	}
	req.Action = "reassign"
	req.RequestID = "reassign"
	req.Session = "new-worker"
	req.Reason = "old process ended"
	if _, err = s.Unit(st.ID, st.Revision, req); err == nil {
		t.Fatal("missing stopped confirmation accepted")
	}
	req.PreviousRunStopped = true
	st, err = s.Unit(st.ID, st.Revision, req)
	if err != nil {
		t.Fatal(err)
	}
	if st.Config.Units[0].Status != "running" {
		t.Fatal("not reassigned")
	}
}
