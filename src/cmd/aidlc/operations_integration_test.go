//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
)

type operationsFixture struct {
	t            *testing.T
	binary, root string
}
type operationsResult struct {
	out, stderr []byte
	code        int
}

func operationsNew(t *testing.T) operationsFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := operationsFixture{t, buildAIDLCBinary(t), root}
	f.ok("install", "codex", "--project-dir", root)
	return f
}
func (f operationsFixture) run(args ...string) operationsResult {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.t.Context(), 30*time.Second)
	defer cancel()
	if result, ok := fixtureInstall(f.binary, f.root, args); ok {
		return result
	}
	binary, args := fixtureProduct(f.binary, args)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = f.root
	if productPath := os.Getenv("AIDLC_TEST_PRODUCT_PATH"); productPath != "" {
		cmd.Env = gitIndependentEnvironment(os.Environ(), productPath)
	}
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			f.t.Fatal(err)
		}
	}
	r := operationsResult{out.Bytes(), errout.Bytes(), code}
	f.t.Logf("CLI %q exit=%d stdout=%s stderr=%s", args, code, r.out, r.stderr)
	return r
}
func (f operationsFixture) ok(args ...string) []byte {
	f.t.Helper()
	r := f.run(args...)
	if r.code != 0 || len(r.stderr) != 0 {
		f.t.Fatalf("CLI failed: %+v", r)
	}
	return r.out
}
func (f operationsFixture) reject(message string, args ...string) {
	f.t.Helper()
	f.rejectCode(2, message, args...)
}
func (f operationsFixture) rejectCode(code int, message string, args ...string) {
	f.t.Helper()
	r := f.run(args...)
	if r.code != code || len(r.out) != 0 || !strings.Contains(string(r.stderr), message) {
		f.t.Fatalf("expected exit%d empty stdout and %q: %+v", code, message, r)
	}
}
func (f operationsFixture) git(args ...string) string {
	f.t.Helper()
	return strings.TrimSpace(string(runFixtureProcess(f.t, f.root, "git", args...)))
}
func (f operationsFixture) create(name string) flow.State {
	f.t.Helper()
	st := operationsState(f.t, f.ok("intent", "create", name, "--space", "default"))
	st = f.action(st, "begin")
	st = f.review(st)
	st = f.finish(st)
	return f.selectPlan(st)
}
func operationsState(t *testing.T, raw []byte) flow.State {
	t.Helper()
	var s flow.State
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func (f operationsFixture) path(s flow.State) string {
	return filepath.Join(f.root, "aidlc/spaces/default/intents", s.ID, "state.json")
}
func (f operationsFixture) bytes(s flow.State) []byte {
	f.t.Helper()
	raw, err := os.ReadFile(f.path(s))
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Logf("state id=%s sha256=%x", s.ID, sha256.Sum256(raw))
	return raw
}
func (f operationsFixture) show(s flow.State) flow.State {
	return operationsState(f.t, f.ok("intent", "show", s.ID, "--space", "default"))
}
func (f operationsFixture) action(s flow.State, action string, extra ...string) flow.State {
	f.t.Helper()
	args := []string{"intent", action, s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10)}
	return operationsState(f.t, f.ok(append(args, extra...)...))
}
func (f operationsFixture) request(value any) string {
	f.t.Helper()
	raw, err := marshalFlowRequest(value)
	if err != nil {
		f.t.Fatal(err)
	}
	p := filepath.Join(f.root, "aidlc/.runtime/request.json")
	writeAIDLCFixture(f.t, p, string(raw))
	return p
}
func (f operationsFixture) bind(s flow.State, session string) {
	f.t.Helper()
	f.ok("intent", "switch", "--id", s.ID, "--space", "default", "--session", session)
}
func (f operationsFixture) session(name string) app.Session {
	f.t.Helper()
	var s app.Session
	if err := json.Unmarshal(f.ok("session", "inspect", "--session", name), &s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func TestOperationsMultiIntent(t *testing.T) {
	f := operationsNew(t)
	a := operationsState(t, f.ok("intent", "create", "A", "--space", "default"))
	before := f.bytes(a)
	b := operationsState(t, f.ok("intent", "create", "B", "--space", "default"))
	if a.ID == b.ID || !bytes.Equal(before, f.bytes(a)) {
		t.Fatal("create lost Intent isolation")
	}
}

func (f operationsFixture) worktree() string {
	f.t.Helper()
	p, err := filepath.EvalSymlinks(f.t.TempDir())
	if err != nil {
		f.t.Fatal(err)
	}
	for _, name := range []string{".agents", ".codex"} {
		if err := os.CopyFS(filepath.Join(p, name), os.DirFS(filepath.Join(f.root, name))); err != nil {
			f.t.Fatal(err)
		}
	}
	for _, name := range []string{"a.go", "b.go"} {
		data, err := os.ReadFile(filepath.Join(f.root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, name), data, 0600); err != nil {
			f.t.Fatal(err)
		}
	}
	return p
}

func (f operationsFixture) review(s flow.State) flow.State {
	f.t.Helper()
	root := f.root
	s = f.action(s, "review", "--file", f.request(flow.ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: root}))
	var gate flow.Gate
	if err := json.Unmarshal(f.ok("intent", "check", s.ID, "--space", "default"), &gate); err != nil {
		f.t.Fatal(err)
	}
	if gate.Status != "pass" {
		f.t.Fatalf("fixture gate: %+v", gate)
	}
	return f.action(s, "review", "--file", f.request(flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: root, Target: gate.Target, Status: "pass", Summary: "Deterministic operations fixture review, not AI evidence"}))
}
func (f operationsFixture) tdd() flow.State {
	f.t.Helper()
	s := f.create("Unit work")
	writeAIDLCFixture(f.t, filepath.Join(f.root, "body.md"), "Current operation contract.\n")
	if _, err := os.Stat(filepath.Join(f.root, "aidlc/spaces/default/knowledge/codekb/current.md")); os.IsNotExist(err) {
		f.ok("memory", "create", "codekb/current", "--space", "default", "--body-file", "body.md", "--actor", "process:test", "--type", "Design", "--title", "Current", "--description", "Operations")
	}
	if _, err := os.Stat(filepath.Join(f.root, "aidlc/spaces/default/knowledge/adr/current.md")); os.IsNotExist(err) {
		f.ok("memory", "create", "adr/current", "--space", "default", "--body-file", "body.md", "--actor", "process:test", "--type", "adr", "--title", "Decision", "--description", "Rationale")
	}
	c := flow.Config{NoMaterialsReason: "fixture has no prior materials", Objective: "Unit operation", Scope: []string{"a.go", "b.go"}, Acceptance: []string{"operations are isolated"}, VerificationPaths: []string{"."}, ADR: flow.ADR{Reason: "No additional decision"}, Artifacts: []flow.Artifact{{Path: "aidlc/spaces/default/knowledge/codekb/current.md", Kind: "Knowledge", Stage: "discovery"}}}
	s = f.action(s, "configure", "--file", f.request(c))
	boundaryFixtureDocument(f.t, f.root, s.ID, "Requirements")
	s = f.action(s, "begin")
	s = f.review(s)
	s = f.finish(s)
	boundaryFixtureDocument(f.t, f.root, s.ID, "ImplementationPlan")
	s = f.action(s, "begin")
	c.Plan = "Separate workers"
	c.Tests = []string{"go test"}
	c.TestResults = []string{"aidlc/evidence/unit.json"}
	c.Units = []flow.Unit{{StepID: s.CurrentStepID, ID: "a", Bolt: "one", Scope: []string{"a.go"}, VerificationPaths: []string{"a.go"}, Tests: []string{"go test"}}, {StepID: s.CurrentStepID, ID: "b", Bolt: "one", Scope: []string{"b.go"}, VerificationPaths: []string{"b.go"}, Tests: []string{"go test"}}}
	s = f.action(s, "configure", "--file", f.request(c))
	s = f.review(s)
	s = f.finish(s)
	for i := range c.Units {
		c.Units[i].StepID = s.CurrentStepID
	}
	s = f.action(s, "configure", "--file", f.request(c))
	return f.action(s, "begin")
}
func (f operationsFixture) unit(s flow.State, action string, r flow.UnitRequest) flow.State {
	r.StepID = s.CurrentStepID
	f.t.Helper()
	if action == "claim" || action == "reassign" {
		registry := assignment.Store{Root: f.root}
		reg, err := registry.Read()
		if errors.Is(err, os.ErrNotExist) {
			raw := f.ok("assignment", "init", "--file", f.request(assignment.InitRequest{RequestID: "fixture-init", HumanConfirmed: true, Reason: "synthetic fixture human confirmation: known work stopped"}))
			if err := json.Unmarshal(raw, &reg); err != nil {
				f.t.Fatal(err)
			}
		} else if err != nil {
			f.t.Fatal(err)
		}
		r.RegistryEpoch = reg.Epoch
		r.CoordinatorSession = "coordinator"
		r.RequestID = fmt.Sprintf("%s-%s-%d", action, r.Unit, s.Revision)
	}

	if action == "result" || action == "confirm" {
		for _, unit := range s.Config.Units {
			if unit.ID != r.Unit {
				continue
			}
			paths := unit.VerificationPaths
			if len(paths) == 0 {
				paths = s.Config.VerificationPaths
			}
			digest, err := flow.ComputeVerification(r.Root, paths)
			if err != nil {
				f.t.Fatal(err)
			}
			r.VerificationSHA256 = digest.SHA256
			if action == "confirm" {
				break
			}
			runs := []map[string]any{}
			for i, command := range unit.Tests {
				output := fmt.Sprintf("aidlc/evidence/unit-%s-%d.txt", unit.ID, i)
				writeAIDLCFixture(f.t, filepath.Join(r.Root, output), "synthetic lifecycle fixture command output\n")
				runs = append(runs, map[string]any{"unit_id": unit.ID, "command": command, "exit_code": 0, "output_path": output})
			}
			raw, err := json.Marshal(map[string]any{"step_id": s.CurrentStepID, "stage": "tdd", "verification_scope": "unit", "verification_sha256": digest.SHA256, "unit_id": unit.ID, "run_id": r.RunID, "runs": runs})
			if err != nil {
				f.t.Fatal(err)
			}
			writeAIDLCFixture(f.t, filepath.Join(r.Root, "aidlc/evidence/unit.json"), string(raw))
		}
	}

	return operationsState(f.t, f.ok("unit", action, s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", f.request(r)))
}

func TestOperationsConcurrentCAS(t *testing.T) {
	f := operationsNew(t)
	s := f.create("Concurrent")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var commands [2]*exec.Cmd
	var outputs, errors [2]bytes.Buffer
	for i := range commands {
		commands[i] = exec.CommandContext(ctx, f.binary, "intent", "pause", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--reason", "writer-"+strconv.Itoa(i))
		commands[i].Dir = f.root
		commands[i].Stdout = &outputs[i]
		commands[i].Stderr = &errors[i]
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	successes := 0
	for i, cmd := range commands {
		err := cmd.Wait()
		code := 0
		if err != nil {
			e, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = e.ExitCode()
		}
		t.Logf("concurrent writer=%d exit=%d stdout=%s stderr=%s", i, code, &outputs[i], &errors[i])
		if code == 0 {
			successes++
			if operationsState(t, outputs[i].Bytes()).Revision != s.Revision+1 || errors[i].Len() != 0 {
				t.Fatal("bad winning state")
			}
		} else if code != 2 || outputs[i].Len() != 0 || !(strings.Contains(errors[i].String(), "revision conflict") || (strings.Contains(errors[i].String(), "lock flow-default unavailable") && strings.Contains(errors[i].String(), "file exists"))) {
			t.Fatal("not CAS conflict")
		}
	}
	current := f.show(s)
	raw := f.bytes(s)
	if successes != 1 || current.Revision != s.Revision+1 || !json.Valid(raw) {
		t.Fatal("CAS did not serialize writers")
	}
	f.reject("revision conflict", "intent", "pause", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--reason", "stale retry")
	current = f.action(current, "resume", "--reason", "re-read after conflict")
	if current.Revision != s.Revision+2 || current.Status != "active" {
		t.Fatal("explicit retry failed")
	}
}

func operationsRead(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func operationsReadOnly(t *testing.T, p string) func() {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if err := os.Chmod(p, info.Mode().Perm()); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(restore)
	if err := os.Chmod(p, 0500); err != nil {
		t.Fatal(err)
	}
	return restore
}
func TestOperationsSaveRecovery(t *testing.T) {
	f := operationsNew(t)
	s := f.create("Save recovery")
	before := f.bytes(s)
	restore := operationsReadOnly(t, filepath.Dir(f.path(s)))
	f.rejectCode(1, "permission denied", "intent", "pause", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--reason", "permission probe")
	if !bytes.Equal(before, f.bytes(s)) {
		t.Fatal("failed save changed state")
	}
	restore()
	s = f.show(s)
	previousRevision := s.Revision
	s = f.action(s, "pause", "--reason", "permission restored")
	if s.Revision != previousRevision+1 || s.Status != "paused" {
		t.Fatal("state retry failed")
	}

}

// Synthetic answer provenance is explicitly test-only; live tests use real hooks.
func (f operationsFixture) approve(s flow.State, plan bool) flow.State {
	f.t.Helper()
	a, action := s.Approval, "approval"
	if plan {
		a = s.ExecutionPlan.Draft.Approval
		action = "plan-approval"
	}
	store := flow.Store{Root: f.root, Space: "default"}
	if err := store.CaptureApproval(s.ID, "fixture", "", a.RequestID, "approve fixture target"); err != nil {
		f.t.Fatal(err)
	}
	return f.action(s, action, "--file", f.request(flow.ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "fixture", Turn: a.RequestID, Quote: "approve"}))
}
func (f operationsFixture) finish(s flow.State) flow.State {
	return f.action(f.approve(s, false), "finish")
}
func (f operationsFixture) selectPlan(s flow.State) flow.State {
	r := flow.PlanRequest{Reason: "deterministic full journey", Steps: []flow.PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}, {Stage: "planning"}, {Stage: "tdd"}, {Stage: "integration"}}, Omitted: []flow.StageOmission{{Stage: "architecture-analysis", Reason: "fresh arithmetic fixture"}}}
	return f.approve(f.action(s, "plan", "--file", f.request(r)), true)
}

func TestOperationsRuntimeBoundary(t *testing.T) {
	f := operationsNew(t)
	s := f.tdd()
	writeAIDLCFixture(t, filepath.Join(f.root, "a.go"), "package fixture\n")
	s.Config.Units[0].Status = "needs_confirmation"
	var err error
	s, err = (flow.Store{Root: f.root, Space: "default"}).Save(s, s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	f.bind(s, "old-session")
	for _, name := range []string{"units/default/" + s.ID + "/a.json", "reviews/default-" + s.ID + ".json"} {
		writeAIDLCFixture(t, filepath.Join(f.root, "aidlc/.runtime/flow", name), "{}")
	}
	f.git("init", "-q")
	f.git("add", "-A")
	tracked := strings.Split(f.git("ls-files"), "\n")
	statePath := "aidlc/spaces/default/intents/" + s.ID + "/state.json"
	hasState, hasKnowledge := false, false
	g := operationsFixture{t, f.binary, t.TempDir()}
	for _, name := range tracked {
		if strings.Contains(name, ".runtime/") {
			t.Fatalf("runtime tracked: %s", name)
		}
		hasState = hasState || name == statePath
		hasKnowledge = hasKnowledge || name == "aidlc/spaces/default/knowledge/codekb/current.md"
		writeAIDLCFixture(t, filepath.Join(g.root, name), string(operationsRead(t, filepath.Join(f.root, name))))
	}
	if !hasState || !hasKnowledge {
		t.Fatal("shared state/knowledge not tracked")
	}
	if g.session("old-session").Intent != "" {
		t.Fatal("old session shared")
	}
	g.bind(s, "new-session")
	before := g.bytes(s)
	digest, err := flow.ComputeVerification(g.root, []string{"a.go"})
	if err != nil {
		t.Fatal(err)
	}
	g.rejectCode(1, "aidlc/.runtime/flow/units", "unit", "confirm", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", g.request(flow.UnitRequest{StepID: s.CurrentStepID, Unit: "a", Session: "old-worker", Root: g.root, RunID: "old", VerificationSHA256: digest.SHA256}))
	var gate flow.Gate
	if err := json.Unmarshal(g.ok("intent", "check", s.ID, "--space", "default"), &gate); err != nil {
		t.Fatal(err)
	}
	g.rejectCode(1, "aidlc/.runtime/flow/reviews", "intent", "review", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", g.request(flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: g.root, Target: gate.Target, Status: "pass", Summary: "old report"}))
	if !bytes.Equal(before, g.bytes(s)) {
		t.Fatal("missing runtime changed state")
	}
}

func TestOperationsCorruptState(t *testing.T) {
	f := operationsNew(t)
	s := operationsState(t, f.ok("intent", "create", "Corrupt", "--space", "default"))
	broken := append([]byte("invalid JSON\n"), f.bytes(s)...)
	if err := os.WriteFile(f.path(s), broken, 0600); err != nil {
		t.Fatal(err)
	}
	f.rejectCode(2, "invalid JSON", "intent", "show", s.ID, "--space", "default")
	if !bytes.Equal(broken, f.bytes(s)) {
		t.Fatal("read changed corrupt state")
	}
}
