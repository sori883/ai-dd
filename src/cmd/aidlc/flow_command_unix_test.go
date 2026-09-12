//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	core "github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
)

func TestFlowCommandFailureOutput(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s: %v", out, err)
		}
	}
	git("init", "-q")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "fixture")
	if err := os.MkdirAll(filepath.Join(root, "aidlc/spaces/default"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	git("add", ".agents", ".codex")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "installed fixture assets")
	store := flow.Store{Root: root, Space: "default"}
	st, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := core.Files.ReadFile("knowledge/rules/rule.md")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"rules/rule.md": string(rule), "design/" + st.ID + "/requirements.md": "---\ntype: Requirements\ntitle: Req\ndescription: Req\nintent_id: " + st.ID + "\n---\n## 目的\nBuild\n## 範囲\nScope\n## 要件\nWork\n## 受入条件\nPass\n## 未確定事項\nなし\n"} {
		p := filepath.Join(root, "aidlc/spaces/default/knowledge", name)
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err = exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	st.Config = flow.Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, VerificationPaths: []string{"."}, ADR: flow.ADR{Reason: "none"}}
	st, err = store.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = store.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	reviewRoot := filepath.Join(t.TempDir(), "review")
	git("worktree", "add", "--detach", reviewRoot, "HEAD")
	reviewRoot, err = filepath.EvalSymlinks(reviewRoot)
	if err != nil {
		t.Fatal(err)
	}
	st, err = store.Review(st.ID, st.Revision, flow.ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: reviewRoot})
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: reviewRoot, Target: "stale", Status: "pass", Summary: "actual report"})
	if err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(root, "aidlc/.runtime/accept.json")
	if err := os.WriteFile(draft, request, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		args       []string
		diagnostic string
	}{
		{"stale", []string{"intent", "review", st.ID, "--expect", strconv.FormatUint(st.Revision, 10), "--file", draft}, "unassigned or stale review"},
		{"conflict", []string{"intent", "pause", st.ID, "--expect", "999", "--reason", "pause"}, "revision conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(tc.args, "--space", "default", "--project-dir", root)
			cmd := mainProcess(t, args...)
			var out, errout bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errout
			code := runMainProcess(t, cmd).ExitCode()
			if code != 2 || out.Len() != 0 || !strings.HasPrefix(errout.String(), "aidlc: ") || (tc.diagnostic != "" && errout.String() != "aidlc: "+tc.diagnostic+": invalid argument\n") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
			}
		})
	}
	if err = os.Remove(filepath.Join(root, ".codex/hooks.json")); err != nil {
		t.Fatal(err)
	}
	st, err = store.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	cmd := mainProcess(t, "intent", "check", st.ID, "--space", "default", "--project-dir", root)
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	code := runMainProcess(t, cmd).ExitCode()
	var gate flow.Gate
	if err := json.Unmarshal(out.Bytes(), &gate); err != nil || gate.Status != "fail" || gate.Target == "" || gate.Summary == "" || code != 0 || errout.Len() != 0 {
		t.Fatalf("valid failed Sensor lost: exit=%d out=%s err=%s decode=%v", code, &out, &errout, err)
	}
}
