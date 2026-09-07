//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/kdr"
	"github.com/sori883/ai-dd/src/internal/minimal"
)

func TestMinimalJourney(t *testing.T) {
	binary := buildMinimalBinary(t)
	root := t.TempDir()
	runMinimalProcess(t, root, "git", "init", "--quiet")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	runMinimalCLI(t, binary, root, nil, "space", "create", "Team Alpha", "--project-dir", root)
	if _, err := os.Stat(filepath.Join(root, "aidlc/active-space")); !os.IsNotExist(err) {
		t.Fatalf("Space creation selected a Space: %v", err)
	}
	event := func(name, session, id, tool, command string, active bool) map[string]any {
		input := minimal.HookInput{Event: name, Session: session, Turn: "turn-1", ID: id, Tool: tool, Active: active}
		input.Input.Command = command
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		out := runMinimalCLI(t, binary, root, raw, "__minimal-hook", "--project-dir", root)
		var response map[string]any
		if err := json.Unmarshal(out, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	event("SessionStart", "writer", "", "", "", false)
	event("UserPromptSubmit", "writer", "", "", "", false)
	denied := event("PreToolUse", "writer", "blocked", "Bash", "touch forbidden", false)
	specific, _ := denied["hookSpecificOutput"].(map[string]any)
	if specific["permissionDecision"] != "deny" {
		t.Fatalf("unbound guard = %+v", denied)
	}
	template := runMinimalCLI(t, binary, root, nil, "kdr", "template", "--space", "team-alpha", "--project-dir", root)
	raw := string(template)
	replacements := map[string]string{"<Intentの名前>": "Add numbers", "<目的と記録内容の要約>": "Test integer addition", "<利用者が得る具体的な結果と許可範囲を記入>": "Add two integers with tests; user approved this small change.", "<読んだ設計と必須ルールの参照を記入>": "Read rules/rule.", "<不明点と次の進め方を記入>": "No questions; write the test first.", "<判断・結果、または未実施の理由を記入>": "Not implemented: preparing test.", "<対象版・検証・レビュー、または未実施の理由を記入>": "Not run: preparing test.", "<残件・質問待ち・再開点を記入>": "Write first test."}
	for before, after := range replacements {
		raw = strings.ReplaceAll(raw, before, after)
	}
	draft := filepath.Join(root, "aidlc/.runtime/drafts/writer.md")
	writeMinimalFixture(t, draft, raw)
	createdRaw := runMinimalCLI(t, binary, root, nil, "intent", "create", "Add numbers", "--space", "team-alpha", "--file", draft, "--actor", "process:journey", "--project-dir", root)
	var saved kdr.Saved
	if err := json.Unmarshal(createdRaw, &saved); err != nil {
		t.Fatal(err)
	}
	bound := runMinimalCLI(t, binary, root, nil, "intent", "switch", "Add numbers", "--space", "team-alpha", "--session", "writer", "--project-dir", root)
	if !strings.Contains(string(bound), saved.ID) || !strings.Contains(string(bound), "作業の合意") {
		t.Fatalf("bind did not deliver KDR/Rules: %s", bound)
	}
	writeMinimalFixture(t, filepath.Join(root, "go.mod"), "module journey\n\ngo 1.26.0\n")
	writeMinimalFixture(t, filepath.Join(root, "add.go"), "package journey\nfunc Add(a,b int)int{return 0}\n")
	writeMinimalFixture(t, filepath.Join(root, "add_test.go"), "package journey\nimport \"testing\"\nfunc TestAdd(t *testing.T){if got:=Add(2,3);got!=5{t.Fatalf(\"Add = %d, want 5\",got)}}\n")
	event("PreToolUse", "writer", "red", "Bash", "go test -run TestAdd", false)
	red := exec.Command("go", "test", "-count=1", "-run", "TestAdd")
	red.Dir = root
	output, err := red.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "want 5") {
		t.Fatalf("fixture RED = %s, %v", output, err)
	}
	event("PostToolUse", "writer", "red", "Bash", "go test -run TestAdd", false)
	event("PreToolUse", "writer", "patch", "apply_patch", "*** Begin Patch\n*** Update File: add.go\n@@\n-func Add(a,b int)int{return 0}\n+func Add(a,b int)int{return a+b}\n*** End Patch", false)
	writeMinimalFixture(t, filepath.Join(root, "add.go"), "package journey\nfunc Add(a,b int)int{return a+b}\n")
	event("PostToolUse", "writer", "patch", "apply_patch", "", false)
	event("PreToolUse", "writer", "green", "Bash", "go test -run TestAdd", false)
	runMinimalProcess(t, root, "go", "test", "-count=1", "-run", "TestAdd")
	event("PostToolUse", "writer", "green", "Bash", "go test -run TestAdd", false)
	runMinimalProcess(t, root, "git", "add", ".")
	runMinimalProcess(t, root, "git", "-c", "user.name=Journey", "-c", "user.email=journey@example.invalid", "commit", "--quiet", "-m", "fixture implementation")
	head := strings.TrimSpace(string(runMinimalProcess(t, root, "git", "rev-parse", "HEAD")))
	review := filepath.Join(t.TempDir(), "review")
	runMinimalProcess(t, root, "git", "clone", "--quiet", "--local", root, review)
	if err := os.Remove(filepath.Join(review, ".codex/hooks.json")); err != nil {
		t.Fatal(err)
	}
	reviewer, err := os.ReadFile(filepath.Join(review, ".codex/agents/aidlc-reviewer.toml"))
	if err != nil || !strings.Contains(string(reviewer), `sandbox_mode = "read-only"`) {
		t.Fatalf("reviewer configuration: %s, %v", reviewer, err)
	}
	// Non-live review fixture: a separate checkout/process checks the fixed code.
	runMinimalProcess(t, review, "go", "test", "-count=1", "-run", "TestAdd")
	current := runMinimalCLI(t, binary, root, nil, "kdr", "show", saved.ID, "--space", "team-alpha", "--project-dir", root)
	if err := json.Unmarshal(current, &saved); err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(saved.Raw), "Not run: preparing test.", "Code "+head+": RED observed, then TestAdd passed; separate checkout non-live review fixture passed. Live independent review not performed.", 1)
	updated = strings.Replace(updated, "Write first test.", "Knowledge proposal: integer addition test; approved by fixture input. Next: resume in another conversation.", 1)
	writeMinimalFixture(t, draft, updated)
	runMinimalCLI(t, binary, root, nil, "kdr", "update", saved.ID, "--space", "team-alpha", "--file", draft, "--expect", saved.Hash, "--session", "writer", "--actor", "process:journey", "--project-dir", root)
	if stop := event("Stop", "writer", "", "", "", false); stop["decision"] == "block" {
		t.Fatalf("recorded Stop blocked: %+v", stop)
	}
	knowledge := filepath.Join(root, "aidlc/.runtime/drafts/knowledge.md")
	writeMinimalFixture(t, knowledge, "---\ntype: Note\ntitle: Addition test\ndescription: Small deterministic integer example\n---\nAdd(2,3) is checked against 5.\n")
	runMinimalCLI(t, binary, root, nil, "memory", "create", "knowledge/addition", "--space", "team-alpha", "--file", knowledge, "--actor", "process:journey", "--project-dir", root)
	found := runMinimalCLI(t, binary, root, nil, "memory", "search", "--space", "team-alpha", "--intent-id", saved.ID, "--project-dir", root)
	if !strings.Contains(string(found), saved.ID) {
		t.Fatalf("ID search: %s", found)
	}
	event("SessionStart", "resumed", "", "", "", false)
	event("UserPromptSubmit", "resumed", "", "", "", false)
	resumed := runMinimalCLI(t, binary, root, nil, "intent", "switch", "Add numbers", "--space", "team-alpha", "--session", "resumed", "--project-dir", root)
	if !strings.Contains(string(resumed), head) {
		t.Fatalf("resume lost recorded version: %s", resumed)
	}
}
func buildMinimalBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "aidlc")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	return binary
}
func runMinimalCLI(t *testing.T, binary, root string, input []byte, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aidlc %v: %v: %s", args, err, output)
	}
	return output
}
func runMinimalProcess(t *testing.T, root, name string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, output)
	}
	return output
}
func writeMinimalFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// This deterministic executable journey covers failure/recovery boundaries;
// actual asynchronous Codex transport pairing is separately live-probed.
func TestMinimalJourneyBoundaries(t *testing.T) {
	binary := buildMinimalBinary(t)
	root := t.TempDir()
	runMinimalProcess(t, root, "git", "init", "--quiet")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	store := kdr.Store{Root: root, Space: "default", Bundle: filepath.Join(root, "aidlc/spaces/default/knowledge")}
	draft := "---\ntype: KDR\ntitle: Boundary work\ndescription: Diagnose recovery\n---\n## 目的と完成条件\nVerify boundaries.\n## 参照する設計・ルール\nUse default Rule.\n## 不明点と進め方\nPrecision choice pending.\n## 判断と結果\nNot started.\n## 検証・レビュー\nNot verified.\n## 残件と再開\nAsk precision choice.\n"
	saved, err := store.Create([]byte(draft), "process:fixture")
	if err != nil {
		t.Fatal(err)
	}
	service := minimal.Service{Root: root, Binary: binary}
	event := func(kind, session, id, command string, active bool) map[string]any {
		t.Helper()
		input := minimal.HookInput{Event: kind, Session: session, Turn: "turn", ID: id, Tool: "Bash", Active: active}
		input.Input.Command = command
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		output := runMinimalCLI(t, binary, root, raw, "__minimal-hook", "--project-dir", root)
		var result map[string]any
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	inspect := func() minimal.Session {
		t.Helper()
		s, err := service.Inspect("boundary")
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	bind := func() {
		t.Helper()
		runMinimalCLI(t, binary, root, nil, "session", "bind", saved.ID, "--space", "default", "--session", "boundary", "--project-dir", root)
	}
	event("SessionStart", "boundary", "", "", false)
	event("UserPromptSubmit", "boundary", "", "", false)
	bind()
	draftPath := filepath.Join(root, "aidlc/.runtime/drafts/boundary.md")
	question := strings.Replace(string(saved.Raw), "Not started.", "Waiting for user: integer precision or decimal precision? No implementation before the answer.", 1)
	writeMinimalFixture(t, draftPath, question)
	runMinimalCLI(t, binary, root, nil, "kdr", "update", saved.ID, "--space", "default", "--session", "boundary", "--file", draftPath, "--expect", saved.Hash, "--actor", "process:fixture", "--project-dir", root)
	if inspect().Dirty {
		t.Fatal("question record did not clear dirty")
	}
	if result := event("Stop", "boundary", "", "", false); result["decision"] == "block" {
		t.Fatalf("recorded question blocked: %v", result)
	}
	event("UserPromptSubmit", "boundary", "", "", false)
	if !inspect().Dirty {
		t.Fatal("answer did not request another record")
	}
	if result := event("Stop", "boundary", "", "", false); result["decision"] != "block" {
		t.Fatal("unrecorded answer not blocked")
	}
	if result := event("Stop", "boundary", "", "", true); result["decision"] == "block" || !inspect().Dirty {
		t.Fatal("Stop reentry lost dirty or blocked again")
	}
	bind()
	event("PreToolUse", "boundary", "long-tool", "sleep 1", false)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 1")
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	current, err := store.Read(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "kdr", "update", saved.ID, "--space", "default", "--session", "boundary", "--file", draftPath, "--expect", current.Hash, "--actor", "process:fixture", "--project-dir", root)
	output, updateErr := cmd.CombinedOutput()
	if updateErr == nil || !strings.Contains(string(output), "still running") {
		t.Fatalf("running update accepted: %v %s", updateErr, output)
	}
	if inspect().Tool != "long-tool" {
		t.Fatal("failed update released running slot")
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	event("PostToolUse", "boundary", "wrong-id", "", false)
	if inspect().Tool != "long-tool" {
		t.Fatal("unmatched completion released slot")
	}
	event("PostToolUse", "boundary", "long-tool", "", false)
	if inspect().Tool != "" || !inspect().Dirty {
		t.Fatal("terminal completion lost dirty or retained slot")
	}
	rulePath := filepath.Join(store.Bundle, "rules/rule.md")
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rulePath); err != nil {
		t.Fatal(err)
	}
	denied := event("PreToolUse", "boundary", "missing-rule", "touch rule-forbidden", false)
	specific, _ := denied["hookSpecificOutput"].(map[string]any)
	if specific["permissionDecision"] != "deny" || !inspect().Dirty {
		t.Fatalf("missing Rule did not diagnose/block: %v", denied)
	}
	writeMinimalFixture(t, rulePath, string(rule))
	bind()
	// Missing executable is an installation fault, not an OS write-denial claim.
	// Its process diagnostic must be visible; persisted dirty state remains intact.
	before := inspect()
	if err := os.Rename(binary, binary+".offline"); err != nil {
		t.Fatal(err)
	}
	missing := exec.CommandContext(ctx, binary, "session", "inspect", "--session", "boundary", "--project-dir", root)
	if _, err := missing.CombinedOutput(); err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("missing CLI not diagnosed: %v", err)
	}
	if after := inspect(); after != before {
		t.Fatal("CLI fault changed persistent state")
	}
	if err := os.Rename(binary+".offline", binary); err != nil {
		t.Fatal(err)
	}
	bind()
	event("SessionStart", "another", "", "", false)
	event("UserPromptSubmit", "another", "", "", false)
	resumed := runMinimalCLI(t, binary, root, nil, "intent", "switch", "Boundary work", "--space", "default", "--session", "another", "--project-dir", root)
	if !strings.Contains(string(resumed), "Waiting for user") || !strings.Contains(string(resumed), saved.ID) {
		t.Fatalf("interrupted question lost on resume: %s", resumed)
	}
}
