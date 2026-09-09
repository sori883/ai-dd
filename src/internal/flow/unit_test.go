package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func unitFixture(t *testing.T) (Store, State, string) {
	t.Helper()
	s, st := sensorFixture(t)
	st.Stage = "tdd"
	prepareBoundaryStage(t, s, &st)
	st.Config.Units = []Unit{{ID: "a", Bolt: "one", Scope: []string{"a.txt"}, Tests: []string{"verify a"}, BaseCommit: st.Config.CodeRevision, Status: "pending"}, {ID: "b", Bolt: "one", Scope: []string{"b.txt"}, Tests: []string{"verify b"}, BaseCommit: st.Config.CodeRevision, Status: "pending"}, {ID: "c", Bolt: "two", DependsOn: []string{"a", "b"}, Scope: []string{"c.txt"}, Tests: []string{"verify c"}, BaseCommit: st.Config.CodeRevision, Status: "pending"}}
	st, err := s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(t.TempDir(), "worker")
	flowGit(t, s.Root, "worktree", "add", "--detach", worker, st.Config.CodeRevision)
	return s, st, worker
}
func TestFlowUnitClaimDependencyAndOverlap(t *testing.T) {
	s, st, worker := unitFixture(t)
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "c", Session: "worker-c", Root: worker}); err == nil {
		t.Fatal("dependent unit started")
	}
	var err error
	st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "a", Session: "worker-a", Root: worker})
	if err != nil || st.Config.Units[0].Status != "running" {
		t.Fatalf("claim %+v %v", st, err)
	}
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "a", Session: "other", Root: worker}); err == nil {
		t.Fatal("double claim")
	}
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "b", Session: "worker-b", Root: worker}); err == nil {
		t.Fatal("shared worktree accepted")
	}
}
func TestFlowUnitResultIntegrationAndResume(t *testing.T) {
	s, st, worker := unitFixture(t)
	var err error
	st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "a", Session: "worker-a", Root: worker})
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := s.assignment(st.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worker, "a.txt"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	flowGit(t, worker, "add", "a.txt")
	flowGit(t, worker, "commit", "-qm", "unit a")
	commit := flowGit(t, worker, "rev-parse", "HEAD")
	request := UnitRequest{Action: "result", Unit: "a", Session: "worker-a", Root: worker, RunID: assignment.RunID, Commit: commit}
	bad := request
	bad.RunID = "wrong"
	if _, err := s.Unit(st.ID, st.Revision, bad); err == nil {
		t.Fatal("foreign result accepted")
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "interrupted"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "inspect current worker"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, request); err == nil {
		t.Fatal("unconfirmed result accepted")
	}
	confirm := request
	confirm.Action = "confirm"
	st, err = s.Unit(st.ID, st.Revision, confirm)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Unit(st.ID, st.Revision, request)
	if err != nil || st.Config.Units[0].Status != "reported" {
		t.Fatalf("result %+v %v", st, err)
	}
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{Action: "integrate", Unit: "a", Commit: st.Config.CodeRevision}); err == nil {
		t.Fatal("unintegrated result accepted")
	}
	flowGit(t, s.Root, "merge", "--ff-only", commit)
	st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "integrate", Unit: "a", Commit: commit})
	if err != nil || st.Config.Units[0].Status != "integrated" {
		t.Fatalf("integration %+v %v", st, err)
	}
}
func TestFlowUnitTwoParallelThenDependent(t *testing.T) {
	s, st, one := unitFixture(t)
	two := filepath.Join(t.TempDir(), "worker-b")
	flowGit(t, s.Root, "worktree", "add", "--detach", two, st.Config.CodeRevision)
	var err error
	for _, r := range []UnitRequest{{Action: "claim", Unit: "a", Session: "a", Root: one}, {Action: "claim", Unit: "b", Session: "b", Root: two}} {
		st, err = s.Unit(st.ID, st.Revision, r)
		if err != nil {
			t.Fatal(err)
		}
	}
	if st.Config.Units[0].Status != "running" || st.Config.Units[1].Status != "running" {
		t.Fatal("parallel assignments missing")
	}
	for i, root := range []string{one, two} {
		unit := []string{"a", "b"}[i]
		if err := os.WriteFile(filepath.Join(root, unit+".txt"), []byte(unit), 0600); err != nil {
			t.Fatal(err)
		}
		flowGit(t, root, "add", unit+".txt")
		flowGit(t, root, "commit", "-qm", unit)
		commit := flowGit(t, root, "rev-parse", "HEAD")
		a, err := s.assignment(st.ID, unit)
		if err != nil {
			t.Fatal(err)
		}
		st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "result", Unit: unit, Session: unit, Root: root, RunID: a.RunID, Commit: commit})
		if err != nil {
			t.Fatal(err)
		}
		flowGit(t, s.Root, "merge", "--no-edit", commit)
		head := flowGit(t, s.Root, "rev-parse", "HEAD")
		st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "integrate", Unit: unit, Commit: head})
		if err != nil {
			t.Fatal(err)
		}
	}
	three := filepath.Join(t.TempDir(), "worker-c")
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	flowGit(t, s.Root, "worktree", "add", "--detach", three, head)
	st.Config.Units[2].BaseCommit = head
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "c", Session: "c", Root: three})
	if err != nil || st.Config.Units[2].Status != "running" {
		t.Fatalf("dependent start %+v %v", st, err)
	}
}
func TestFlowUnitRejectsPreIntegrationBase(t *testing.T) {
	s, st, worker := unitFixture(t)
	old := st.Config.CodeRevision
	os.WriteFile(filepath.Join(s.Root, "a.txt"), []byte("integrated"), 0600)
	flowGit(t, s.Root, "add", "a.txt")
	flowGit(t, s.Root, "commit", "-qm", "dependency")
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Config.Units[0].Status = "integrated"
	st.Config.Units[0].ResultCommit = head
	st.Config.Units[0].IntegratedCommit = head
	st.Config.Units[1].Status = "integrated"
	st.Config.Units[1].ResultCommit = head
	st.Config.Units[1].IntegratedCommit = head
	st.Config.Units[2].BaseCommit = old
	st, err := s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unit(st.ID, st.Revision, UnitRequest{Action: "claim", Unit: "c", Session: "worker-c", Root: worker}); err == nil {
		t.Fatal("worker base excludes integrated dependencies")
	}
}
