package flow

import (
	"errors"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"os"
	"path/filepath"
	"testing"
)

func unitFixture(t *testing.T) (Store, State, string) {
	t.Helper()
	s, st := sensorFixture(t)
	fixtureExecutionStage(t, s, &st, "tdd")
	prepareBoundaryStage(t, s, &st)
	st.Config.TestResults = []string{"aidlc/evidence/unit.json"}
	st.Config.Units = []Unit{{ID: "a", Bolt: "one", Scope: []string{"a.txt"}, Tests: []string{"verify a"}, Status: "pending"}, {ID: "b", Bolt: "one", Scope: []string{"b.txt"}, Tests: []string{"verify b"}, Status: "pending"}, {ID: "c", Bolt: "two", DependsOn: []string{"a", "b"}, Scope: []string{"c.txt"}, Tests: []string{"verify c"}, Status: "pending"}}
	for i := range st.Config.Units {
		st.Config.Units[i].VerificationPaths = append([]string{}, st.Config.Units[i].Scope...)
	}
	st, err := saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(t.TempDir(), "worker")
	flowGit(t, s.Root, "worktree", "add", "--detach", worker, flowGit(t, s.Root, "rev-parse", "HEAD"))
	return s, st, worker
}
func TestFlowUnitClaimDependencyAndOverlap(t *testing.T) {
	s, st, worker := unitFixture(t)
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "c", Session: "worker-c", Root: worker}); err == nil {
		t.Fatal("dependent unit started")
	}
	var err error
	st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "a", Session: "worker-a", Root: worker})
	if err != nil || st.Config.Units[0].Status != "running" {
		t.Fatalf("claim %+v %v", st, err)
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "a", Session: "other", Root: worker}); err == nil {
		t.Fatal("double claim")
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "b", Session: "worker-b", Root: worker}); err == nil {
		t.Fatal("shared worktree accepted")
	}
}
func TestFlowUnitResultIntegrationAndResume(t *testing.T) {
	s, st, worker := unitFixture(t)
	var err error
	st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "a", Session: "worker-a", Root: worker})
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
	request := UnitRequest{StepID: "s04", Action: "result", Unit: "a", Session: "worker-a", Root: worker, RunID: assignment.RunID}
	bad := request
	bad.RunID = "wrong"
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, bad); err == nil {
		t.Fatal("foreign result accepted")
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "interrupted"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "resume", Reason: "inspect current worker"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, request); err == nil {
		t.Fatal("unconfirmed result accepted")
	}
	confirm := request
	confirm.Action = "confirm"
	st, err = assignmentUnit(t, s, st.ID, st.Revision, confirm)
	if err != nil {
		t.Fatal(err)
	}
	st, err = assignmentUnit(t, s, st.ID, st.Revision, request)
	if err != nil || st.Config.Units[0].Status != "reported" {
		t.Fatalf("result %+v %v", st, err)
	}
	if _, err := assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "integrate", Unit: "a"}); err == nil {
		t.Fatal("unintegrated result accepted")
	}
	flowGit(t, s.Root, "merge", "--ff-only", commit)
	st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "integrate", Unit: "a"})
	if err != nil || st.Config.Units[0].Status != "integrated" {
		t.Fatalf("integration %+v %v", st, err)
	}
}
func TestFlowUnitTwoParallelThenDependent(t *testing.T) {
	s, st, one := unitFixture(t)
	two := filepath.Join(t.TempDir(), "worker-b")
	flowGit(t, s.Root, "worktree", "add", "--detach", two, flowGit(t, s.Root, "rev-parse", "HEAD"))
	var err error
	for _, r := range []UnitRequest{{StepID: "s04", Action: "claim", Unit: "a", Session: "a", Root: one}, {StepID: "s04", Action: "claim", Unit: "b", Session: "b", Root: two}} {
		st, err = assignmentUnit(t, s, st.ID, st.Revision, r)
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
		st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "result", Unit: unit, Session: unit, Root: root, RunID: a.RunID})
		if err != nil {
			t.Fatal(err)
		}
		flowGit(t, s.Root, "merge", "--no-edit", commit)
		st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "integrate", Unit: unit})
		if err != nil {
			t.Fatal(err)
		}
	}
	three := filepath.Join(t.TempDir(), "worker-c")
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	flowGit(t, s.Root, "worktree", "add", "--detach", three, head)
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = assignmentUnit(t, s, st.ID, st.Revision, UnitRequest{StepID: "s04", Action: "claim", Unit: "c", Session: "c", Root: three})
	if err != nil || st.Config.Units[2].Status != "running" {
		t.Fatalf("dependent start %+v %v", st, err)
	}
}
func TestFlowUnitDependencyContent(t *testing.T) { TestUnitWithoutGit(t) }

func assignmentUnit(t *testing.T, s Store, id string, expect uint64, r UnitRequest) (State, error) {
	t.Helper()
	if r.Action == "claim" || r.Action == "reassign" {
		registry := assignment.Store{Root: s.Root}
		reg, err := registry.Read()
		if errors.Is(err, os.ErrNotExist) {
			reg, err = registry.Init(assignment.InitRequest{RequestID: "fixture-init", HumanConfirmed: true, Reason: "synthetic test confirms known workers stopped"})
		}
		if err != nil {
			return State{}, err
		}
		if r.RegistryEpoch == "" {
			r.RegistryEpoch = reg.Epoch
		}
		if r.CoordinatorSession == "" {
			r.CoordinatorSession = "fixture-main"
		}
		if r.RequestID == "" {
			r.RequestID = fmt.Sprintf("%s-%s-%d", r.Action, r.Unit, expect)
		}
	}
	if (r.Action == "result" || r.Action == "confirm") && r.VerificationSHA256 == "" {
		st, err := s.Read(id)
		if err != nil {
			return State{}, err
		}
		r = prepareUnitResultFixture(t, s, st, r)
	}
	return s.Unit(id, expect, r)
}
