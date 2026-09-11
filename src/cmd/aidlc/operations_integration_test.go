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

	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/minimal"
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
	f := operationsFixture{t, buildMinimalBinary(t), root}
	f.git("init", "-q")
	f.git("-c", "user.name=Operations", "-c", "user.email=operations@example.invalid", "commit", "--allow-empty", "-qm", "base")
	f.ok("install", "codex", "--project-dir", root)
	return f
}
func (f operationsFixture) run(args ...string) operationsResult {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, args...)
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
	return strings.TrimSpace(string(runMinimalProcess(f.t, f.root, "git", args...)))
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
	writeMinimalFixture(f.t, p, string(raw))
	return p
}
func (f operationsFixture) bind(s flow.State, session string) {
	f.t.Helper()
	f.ok("intent", "switch", "--id", s.ID, "--space", "default", "--session", session)
}
func (f operationsFixture) session(name string) minimal.Session {
	f.t.Helper()
	var s minimal.Session
	if err := json.Unmarshal(f.ok("session", "inspect", "--session", name), &s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func TestOperationsMultiIntent(t *testing.T) {
	f := operationsNew(t)
	a, b := f.create("A"), f.create("B")
	f.bind(a, "session-a")
	f.bind(b, "session-b")
	bRaw := f.bytes(b)
	a = f.action(a, "configure", "--file", f.request(flow.Config{Objective: "Only A"}))
	a = f.action(a, "pause", "--reason", "handoff")
	if !bytes.Equal(bRaw, f.bytes(b)) || f.session("session-a").Intent != a.ID || f.session("session-b").Intent != b.ID {
		t.Fatal("Intent isolation lost")
	}
	f.bind(a, "new-a")
	a = f.action(a, "resume", "--reason", "new session inspected A")
	a = f.action(a, "wait", "--reason", "question", "--resume-condition", "answer")
	f.bind(a, "answer-a")
	a = f.action(a, "resume", "--reason", "answer received")
	if a.Status != "active" || a.Config.Objective != "Only A" || f.session("answer-a").Intent != a.ID {
		t.Fatal("resume lost identity/config")
	}
	f.create("Duplicate")
	f.create("Duplicate")
	before := f.session("session-b")
	f.reject("ambiguous", "intent", "switch", "Duplicate", "--space", "default", "--session", "session-b")
	if f.session("session-b") != before {
		t.Fatal("ambiguous selection changed session")
	}
}

func (f operationsFixture) commit(message string) string {
	f.t.Helper()
	f.git("add", ".")
	f.git("-c", "user.name=Operations", "-c", "user.email=operations@example.invalid", "commit", "-qm", message)
	return f.git("rev-parse", "HEAD")
}
func (f operationsFixture) worktree() string {
	f.t.Helper()
	p := filepath.Join(f.t.TempDir(), "worker")
	f.git("worktree", "add", "--detach", p, "HEAD")
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		f.t.Fatal(err)
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
	writeMinimalFixture(f.t, filepath.Join(f.root, "body.md"), "Current operation contract.\n")
	if _, err := os.Stat(filepath.Join(f.root, "aidlc/spaces/default/knowledge/codekb/current.md")); os.IsNotExist(err) {
		f.ok("memory", "create", "codekb/current", "--space", "default", "--body-file", "body.md", "--actor", "process:test", "--type", "Design", "--title", "Current", "--description", "Operations")
	}
	if _, err := os.Stat(filepath.Join(f.root, "aidlc/spaces/default/knowledge/adr/current.md")); os.IsNotExist(err) {
		f.ok("memory", "create", "adr/current", "--space", "default", "--body-file", "body.md", "--actor", "process:test", "--type", "adr", "--title", "Decision", "--description", "Rationale")
	}
	f.commit("shared assets")
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
				writeMinimalFixture(f.t, filepath.Join(r.Root, output), "synthetic lifecycle fixture command output\n")
				runs = append(runs, map[string]any{"unit_id": unit.ID, "command": command, "exit_code": 0, "output_path": output})
			}
			raw, err := json.Marshal(map[string]any{"step_id": s.CurrentStepID, "stage": "tdd", "verification_scope": "unit", "verification_sha256": digest.SHA256, "unit_id": unit.ID, "run_id": r.RunID, "runs": runs})
			if err != nil {
				f.t.Fatal(err)
			}
			writeMinimalFixture(f.t, filepath.Join(r.Root, "aidlc/evidence/unit.json"), string(raw))
		}
	}

	return operationsState(f.t, f.ok("unit", action, s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", f.request(r)))
}
func TestOperationsGitHandoff(t *testing.T) {
	f := operationsNew(t)
	s := f.tdd()
	worker := f.worktree()
	s = f.unit(s, "claim", flow.UnitRequest{Unit: "a", Session: "old-worker", Root: worker})
	s = f.action(s, "pause", "--reason", "transfer")
	f.bind(s, "old-session")
	saved := f.bytes(s)
	knowledge := filepath.Join("aidlc", "spaces", "default", "knowledge", "codekb", "current.md")
	adr := filepath.Join("aidlc", "spaces", "default", "knowledge", "ADR", "current.md")
	originals := map[string][]byte{}
	for _, p := range []string{knowledge, adr, ".codex/hooks.json"} {
		raw, err := os.ReadFile(filepath.Join(f.root, p))
		if err != nil {
			t.Fatal(err)
		}
		originals[p] = raw
	}
	f.commit("handoff state")
	clone := filepath.Join(t.TempDir(), "clone")
	f.git("clone", "-q", f.root, clone)
	clone, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	g := operationsFixture{t, f.binary, clone}
	if !bytes.Equal(saved, g.bytes(s)) {
		t.Fatal("state changed in clone")
	}
	for p, want := range originals {
		got, err := os.ReadFile(filepath.Join(clone, p))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("clone bytes %s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(clone, "aidlc/.runtime")); !os.IsNotExist(err) {
		t.Fatalf("runtime shared: %v", err)
	}
	if g.session("old-session").Intent != "" {
		t.Fatal("old session shared")
	}
	g.bind(s, "new-session")
	s = g.action(s, "resume", "--reason", "clone inspected")
	before := g.bytes(s)
	g.rejectCode(1, "aidlc/.runtime/flow/units", "unit", "confirm", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", g.request(flow.UnitRequest{StepID: s.CurrentStepID, Unit: "a", Session: "old-worker", Root: worker, RunID: "old", VerificationSHA256: f.git("rev-parse", "HEAD")}))
	if !bytes.Equal(before, g.bytes(s)) || s.Config.Units[0].Status != "needs_confirmation" {
		t.Fatal("old assignment accepted")
	}
	g.rejectCode(1, "aidlc/.runtime/flow/reviews", "intent", "review", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", g.request(flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: g.worktree(), Target: "old", Status: "pass", Summary: "old report"}))
	if !bytes.Contains(originals[".codex/hooks.json"], []byte(f.root)) {
		t.Fatal("expected installed absolute source root")
	}
	t.Log("Git preserves state/Knowledge/ADR; installed hooks still refer to source root and require separate relocation work. No hook rewrite performed.")
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

func TestOperationsUnitConflicts(t *testing.T) {
	f := operationsNew(t)
	s := f.tdd()
	a, b := f.worktree(), f.worktree()
	// A third Unit depends on A, while a fourth deliberately overlaps A's scope.
	c := s.Config
	c.Units = append(c.Units, flow.Unit{StepID: s.CurrentStepID, ID: "dependent", Bolt: "two", DependsOn: []string{"a"}, Scope: []string{"c.go"}, Tests: []string{"go test"}}, flow.Unit{StepID: s.CurrentStepID, ID: "overlap", Bolt: "one", Scope: []string{"a.go"}, Tests: []string{"go test"}})
	s = f.action(s, "configure", "--file", f.request(c))
	s = f.unit(s, "claim", flow.UnitRequest{Unit: "a", Session: "worker-a", Root: a})
	for _, tc := range []struct{ name, unit, message string }{{"duplicate", "a", "Unit already assigned"}, {"scope", "overlap", "concurrent Unit scopes overlap"}, {"dependency", "dependent", "dependency not integrated"}} {
		t.Run(tc.name, func(t *testing.T) {
			g := f
			g.t = t
			before := g.bytes(s)
			g.reject(tc.message, "unit", "claim", s.ID, "--space", "default", "--expect", strconv.FormatUint(s.Revision, 10), "--file", g.request(flow.UnitRequest{StepID: s.CurrentStepID, Unit: tc.unit, Session: "worker-b", Root: b}))
			if !bytes.Equal(before, g.bytes(s)) {
				t.Fatal("rejected claim changed state")
			}
		})
	}
	s = f.unit(s, "claim", flow.UnitRequest{Unit: "b", Session: "worker-b", Root: b})
	if s.Config.Units[0].Status != "running" || s.Config.Units[1].Status != "running" {
		t.Fatal("independent assignments lost")
	}
	for _, tc := range []struct{ unit, root, session string }{{"a", a, "worker-a"}, {"b", b, "worker-b"}} {
		raw, err := os.ReadFile(filepath.Join(f.root, "aidlc/.runtime/flow/units/default", s.ID, tc.unit+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var assignment flow.UnitRequest
		if err := json.Unmarshal(raw, &assignment); err != nil {
			t.Fatal(err)
		}
		if assignment.Root != tc.root || assignment.Session != tc.session || assignment.RunID == "" {
			t.Fatalf("wrong assignment %+v", assignment)
		}
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
	body := filepath.Join(f.root, "body.md")
	writeMinimalFixture(t, body, "Before\n")
	create := []string{"memory", "create", "codekb/save", "--space", "default", "--body-file", body, "--actor", "process:test", "--type", "Design", "--title", "Save", "--description", "Recovery"}
	f.ok(create...)
	concept := filepath.Join(f.root, "aidlc/spaces/default/knowledge/codekb/save.md")
	original := operationsRead(t, concept)
	hash := sha256.Sum256(original)
	update := []string{"memory", "update", "codekb/save", "--space", "default", "--body-file", body, "--actor", "process:test", "--expect", fmt.Sprintf("%x", hash)}
	writeMinimalFixture(t, body, "After\n")
	restore = operationsReadOnly(t, filepath.Dir(concept))
	f.rejectCode(1, "permission denied", update...)
	if !bytes.Equal(original, operationsRead(t, concept)) {
		t.Fatal("Knowledge pre-save failure changed document")
	}
	restore()
	f.ok("memory", "show", "codekb/save", "--space", "default")
	f.ok(update...)
	// A real directory collision makes bookkeeping fail after the Concept commit.
	index := filepath.Join(filepath.Dir(concept), "index.md")
	indexBytes := operationsRead(t, index)
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(index, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(index); os.WriteFile(index, indexBytes, 0644) })
	create[2] = "codekb/partial"
	result := f.run(create...)
	if result.code != 2 || !json.Valid(result.out) || !strings.Contains(string(result.stderr), "Concept saved; bookkeeping failed: not regular") {
		t.Fatalf("partial save hidden: %+v", result)
	}
	partial := filepath.Join(filepath.Dir(concept), "partial.md")
	saved := operationsRead(t, partial)
	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result.out, &response); err != nil {
		t.Fatal(err)
	}
	if response.Hash != fmt.Sprintf("%x", sha256.Sum256(saved)) {
		t.Fatal("partial saved hash missing")
	}
	f.ok("memory", "show", "codekb/partial", "--space", "default")
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, indexBytes, 0644); err != nil {
		t.Fatal(err)
	}
	f.ok("memory", "update", "codekb/partial", "--space", "default", "--body-file", body, "--actor", "process:test", "--expect", response.Hash, "--description", "Recovered bookkeeping")
	f.ok("memory", "check", "--space", "default")
	if !bytes.Contains(operationsRead(t, index), []byte("partial.md")) {
		t.Fatal("index not recovered")
	}
}

func TestOperationsGitConflict(t *testing.T) {
	f := operationsNew(t)
	s := f.create("Git conflict")
	f.commit("base state")
	base := f.git("rev-parse", "HEAD")
	f.git("checkout", "-qb", "left")
	left := f.action(s, "pause", "--reason", "left decision")
	chosen := f.bytes(left)
	f.commit("left state")
	f.git("checkout", "-qb", "right", base)
	f.action(s, "wait", "--reason", "right question", "--resume-condition", "answer")
	f.commit("right state")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "user.name=Operations", "-c", "user.email=operations@example.invalid", "merge", "left")
	cmd.Dir = f.root
	out, err := cmd.CombinedOutput()
	e, ok := err.(*exec.ExitError)
	if !ok || e.ExitCode() != 1 {
		t.Fatalf("expected Git conflict: %v %s", err, out)
	}
	t.Logf("git merge exit=1 %s", out)
	conflict := f.bytes(s)
	if !bytes.Contains(conflict, []byte("<<<<<<<")) || f.git("ls-files", "-u") == "" {
		t.Fatal("conflict not present")
	}
	result := f.run("intent", "show", s.ID, "--space", "default")
	if result.code != 1 || len(result.out) != 0 || !strings.Contains(string(result.stderr), "invalid character") {
		t.Fatalf("unresolved JSON accepted: %+v", result)
	}
	if !bytes.Equal(conflict, f.bytes(s)) {
		t.Fatal("read modified conflict")
	}
	// Explicit fixture policy chooses the complete left state, never synthesizes fields.
	if err := os.WriteFile(f.path(s), chosen, 0644); err != nil {
		t.Fatal(err)
	}
	f.commit("resolve by choosing complete left state")
	resolved := f.show(s)
	if resolved.ID != left.ID || resolved.Revision != left.Revision || resolved.Status != "paused" || resolved.Reason != "left decision" || !bytes.Equal(chosen, f.bytes(s)) || f.git("ls-files", "-u") != "" {
		t.Fatal("explicit conflict resolution lost state")
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
