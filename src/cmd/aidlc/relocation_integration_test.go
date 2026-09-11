//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func relocationSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = filestore.Hash(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func relocationBinary(t *testing.T, old string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "aidlc")
	if err := os.WriteFile(p, operationsRead(t, old), 0755); err != nil {
		t.Fatal(err)
	}
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestRelocationCommand(t *testing.T) {
	f := operationsNew(t)
	canonicalBinary, canonicalErr := filepath.EvalSymlinks(f.binary)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	f.binary = canonicalBinary
	writeMinimalFixture(t, filepath.Join(f.root, "go.mod"), "module example.invalid/relocation\n\ngo 1.26\n")
	writeMinimalFixture(t, filepath.Join(f.root, "a.go"), "package relocation\nfunc A()int{return 0}\n")
	writeMinimalFixture(t, filepath.Join(f.root, "b.go"), "package relocation\nfunc B()int{return 0}\n")
	writeMinimalFixture(t, filepath.Join(f.root, "work_test.go"), "package relocation\nimport \"testing\"\nfunc TestA(t *testing.T){if A()!=1{t.Fatal(\"A\")}}\nfunc TestB(t *testing.T){if B()!=2{t.Fatal(\"B\")}}\n")
	st := f.tdd()
	planned := st.Config
	for i := range planned.Units {
		planned.Units[i].Tests = []string{"go test -count=1 -run ^Test" + strings.ToUpper(planned.Units[i].ID) + "$"}
	}
	st = f.action(st, "configure", "--file", f.request(planned))
	for _, id := range []string{"a", "b"} {
		st = f.unit(st, "claim", flow.UnitRequest{Unit: id, Session: "old-" + id, Root: f.worktree()})
	}
	st = f.action(st, "pause", "--reason", "old workers stopped before clone")
	hookPath := filepath.Join(f.root, ".codex/hooks.json")
	hooks := operationsRead(t, hookPath)
	hooks = bytes.Replace(hooks, []byte(`"hooks": {`), []byte(`"custom": {"keep": true}, "hooks": {"CustomEvent":[{"hooks":[{"type":"command","command":"echo unchanged"}]}],`), 1)
	if err := os.WriteFile(hookPath, hooks, 0644); err != nil {
		t.Fatal(err)
	}
	f.commit("handoff shared state")
	original := relocationSnapshot(t, f.root)
	clone := filepath.Join(t.TempDir(), "clone")
	f.git("clone", "-q", f.root, clone)
	clone, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	g := operationsFixture{t, relocationBinary(t, f.binary), clone}
	before := relocationSnapshot(t, clone)
	g.ok("install", "codex", "--relocate", "--project-dir", clone, "--from-project-dir", f.root, "--from-binary", f.binary)
	after := relocationSnapshot(t, clone)
	const runtimeIgnore = "aidlc/.runtime/.gitignore"
	if _, existed := before[runtimeIgnore]; !existed {
		got := operationsRead(t, filepath.Join(clone, runtimeIgnore))
		if string(got) != "*\n" {
			t.Fatalf("unexpected runtime ignore bytes: %q", got)
		}
		before[runtimeIgnore] = filestore.Hash([]byte("*\n"))
	}
	for p, hash := range before {
		if p == ".codex/hooks.json" || p == ".agents/skills/aidlc/SKILL.md" || p == ".agents/skills/aidlc-cli/SKILL.md" {
			continue
		}
		if after[p] != hash {
			t.Fatalf("changed unrelated asset %s", p)
		}
	}
	if len(after) != len(before) {
		t.Fatal("relocation added or removed assets")
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	for _, name := range []string{"aidlc", "aidlc-cli"} {
		p := filepath.Join(".agents/skills", name, "SKILL.md")
		old := operationsRead(t, filepath.Join(f.root, p))
		want := bytes.ReplaceAll(old, []byte(quote(f.binary)), []byte(quote(g.binary)))
		got := operationsRead(t, filepath.Join(clone, p))
		if bytes.Equal(old, want) || !bytes.Equal(got, want) {
			t.Fatalf("skill reference bytes not relocated: %s", p)
		}
	}
	changedHooks := operationsRead(t, filepath.Join(clone, ".codex/hooks.json"))
	if !bytes.Contains(changedHooks, []byte(`"custom": {"keep": true}`)) || bytes.Contains(changedHooks, []byte(f.root)) {
		t.Fatal("old refs or custom bytes corrupted")
	}
	// Execute the actual relocated handler command; it must initialize only clone runtime.
	var config struct {
		Hooks map[string][]struct{ Hooks []struct{ Command string } }
	}
	if err := json.Unmarshal(changedHooks, &config); err != nil {
		t.Fatal(err)
	}
	args, ok := flowShellWords(config.Hooks["UserPromptSubmit"][0].Hooks[0].Command)
	if !ok || args[0] != g.binary {
		t.Fatal("bad relocated command")
	}
	runMinimalCLI(t, args[0], clone, []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"new-coordinator","turn_id":"turn"}`), args[1:]...)
	g.bind(st, "new-coordinator")
	st = g.action(st, "resume", "--reason", "old workers stopped, clone inspected")
	// Clone has no local registry. Never attach new reservations to its old Unit runs.
	legacyWorker := filepath.Join(t.TempDir(), "legacy-worker")
	g.git("worktree", "add", "--detach", legacyWorker, "HEAD")
	g.rejectCode(1, "assignment registry unavailable", "unit", "reassign", st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10), "--file", g.request(flow.UnitRequest{StepID: st.CurrentStepID, Unit: "a", Root: legacyWorker, Session: "new", PreviousRunStopped: true, Reason: "old stopped", VerificationSHA256: g.git("rev-parse", "HEAD")}))
	st = g.tdd()
	planned = st.Config
	for i := range planned.Units {
		planned.Units[i].Tests = []string{"go test -count=1 -run ^Test" + strings.ToUpper(planned.Units[i].ID) + "$"}
	}
	st = g.action(st, "configure", "--file", g.request(planned))
	var runs []map[string]any
	for i, id := range []string{"a", "b"} {
		worker := filepath.Join(t.TempDir(), "new-"+id)
		g.git("worktree", "add", "--detach", worker, "HEAD")
		worker, err = filepath.EvalSymlinks(worker)
		if err != nil {
			t.Fatal(err)
		}
		st = g.unit(st, "claim", flow.UnitRequest{Unit: id, Session: "new-" + id, Root: worker})
		raw := operationsRead(t, filepath.Join(clone, "aidlc/.runtime/flow/units/default", st.ID, id+".json"))
		var assignment flow.UnitRequest
		if err := json.Unmarshal(raw, &assignment); err != nil {
			t.Fatal(err)
		}
		writeMinimalFixture(t, filepath.Join(worker, id+".go"), fmt.Sprintf("package relocation\nfunc %s()int{return %d}\n", strings.ToUpper(id), i+1))
		output := runMinimalProcess(t, worker, "go", "test", "-count=1", "-run", "^Test"+strings.ToUpper(id)+"$")
		w := operationsFixture{t, g.binary, worker}
		commit := w.commit("verified " + id)
		outputPath := "aidlc/evidence/relocated-" + id + ".log"
		writeMinimalFixture(t, filepath.Join(clone, outputPath), string(output))
		runs = append(runs, map[string]any{"unit_id": id, "command": st.Config.Units[i].Tests[0], "exit_code": 0, "output_path": outputPath})
		st = g.unit(st, "result", flow.UnitRequest{Unit: id, Session: "new-" + id, Root: worker, RunID: assignment.RunID, VerificationSHA256: commit})
		g.git("-c", "user.name=Relocation", "-c", "user.email=relocation@example.invalid", "merge", "--no-edit", commit)
		st = g.unit(st, "integrate", flow.UnitRequest{Unit: id, VerificationSHA256: g.git("rev-parse", "HEAD")})
	}
	output := runMinimalProcess(t, clone, "go", "test", "-count=1")
	// Record the latest whole-project regression output before independent fixture review.
	writeMinimalFixture(t, filepath.Join(clone, "aidlc/evidence/relocated-all.log"), string(output))
	g.commit("integration evidence")
	c := st.Config
	for _, run := range runs {
		run["output_path"] = "aidlc/evidence/relocated-all.log"
	}
	resultPath := "aidlc/evidence/relocated-tdd.json"
	digest, err := flow.ComputeVerification(clone, c.VerificationPaths)
	if err != nil {
		t.Fatal(err)
	}
	resultJSON, err := json.Marshal(map[string]any{"step_id": st.CurrentStepID, "stage": "tdd", "verification_scope": "intent", "verification_sha256": digest.SHA256, "runs": runs})
	if err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(clone, resultPath), string(resultJSON))
	c.TestResults = []string{resultPath}
	c.Artifacts = append(c.Artifacts, flow.Artifact{Path: "aidlc/evidence/relocated-all.log", Kind: "test", Stage: "tdd"})
	st = g.action(st, "configure", "--file", g.request(c))
	st = g.review(st)
	if st.Review.Status != "pass" || st.Config.Units[0].Status != "integrated" || st.Config.Units[1].Status != "integrated" {
		t.Fatal("relocation did not finish verification")
	}
	if !reflect.DeepEqual(original, relocationSnapshot(t, f.root)) {
		t.Fatal("source project changed")
	}
}
