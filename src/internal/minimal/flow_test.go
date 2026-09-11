package minimal

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFlowIntentCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "aidlc/spaces/default"), 0700); err != nil {
		t.Fatal(err)
	}
	deployProcedureFixture(t, root)
	s := Service{Root: root}
	raw, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "create", Target: "Work", Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var st flow.State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.Stage != "initialization" || len(st.ID) != 32 {
		t.Fatalf("state %s", raw)
	}
	raw, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "show", Target: st.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.Name != "Work" {
		t.Fatalf("show %s", raw)
	}
}
func TestFlowStopNoRecordObligation(t *testing.T) {
	s := Service{Root: t.TempDir()}
	out, err := s.Hook(HookInput{Event: "Stop", Session: "s"})
	if err != nil || out["decision"] == "block" {
		t.Fatalf("unused session forced record: %+v %v", out, err)
	}
}
func TestFlowHookSelectionRulesAndRecovery(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root, Binary: "/opt/aidlc"}
	st, err := (flow.Store{Root: root, Space: "default"}).Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	st, err = (flow.Store{Root: root, Space: "default"}).Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	hook(t, s, "SessionStart", "", "", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if !deny(hook(t, s, "PreToolUse", "Bash", "unbound", "touch code.go", false)) {
		t.Fatal("unbound allowed")
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: st.ID, Space: "default", Session: "session"}); err != nil {
		t.Fatal(err)
	}
	if deny(hook(t, s, "PreToolUse", "Bash", "running", "sleep 2", false)) {
		t.Fatal("bound state operation denied")
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "other", "touch code.go", false)) {
		t.Fatal("overlap allowed")
	}
	hook(t, s, "PostToolUse", "Bash", "wrong", "", false)
	current, err := s.Inspect("session")
	if err != nil || current.Tool != "running" {
		t.Fatal("wrong Post cleared slot")
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: st.ID, Space: "other", Session: "session", Recover: true}); err == nil {
		t.Fatal("foreign recover accepted")
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: st.ID, Space: "default", Session: "session", Recover: true}); err != nil {
		t.Fatal(err)
	}
	if out := hook(t, s, "Stop", "", "", "", false); out["decision"] == "block" {
		t.Fatalf("forced KDR record %+v", out)
	}
}
func TestFlowConfigurePreservesActiveAssignment(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "aidlc/spaces/default"), 0700)
	deployProcedureFixture(t, root)
	store := flow.Store{Root: root, Space: "default"}
	st, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	st.Config.Units = []flow.Unit{{ID: "a", Status: "running", Scope: []string{"a.go"}}}
	st = writeExecutionFixture(t, store, st)
	err = nil
	if err != nil {
		t.Fatal(err)
	}
	config := st.Config
	config.Units = nil
	raw, _ := json.Marshal(config)
	file := filepath.Join(root, "config.json")
	os.WriteFile(file, raw, 0600)
	_, err = (Service{Root: root}).Execute(cli.MinimalRequest{Command: "intent", Action: "configure", Space: "default", Target: st.ID, Expect: strconv.FormatUint(st.Revision, 10), File: file})
	if err == nil {
		t.Fatal("removed active Unit assignment")
	}
}
func TestFlowConfigureCannotForgeProgress(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		existing     bool
		remove       bool
	}{{"new integrated", "integrated", false, false}, {"new running", "running", false, false}, {"pending forged", "integrated", true, false}, {"reported removed", "reported", true, true}, {"integrated reset", "pending", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			os.MkdirAll(filepath.Join(root, "aidlc/spaces/default"), 0700)
			deployProcedureFixture(t, root)
			store := flow.Store{Root: root, Space: "default"}
			st, err := store.Create("Work")
			if err != nil {
				t.Fatal(err)
			}
			if tc.existing {
				status := "pending"
				if tc.remove {
					status = "reported"
				}
				if tc.name == "integrated reset" {
					status = "integrated"
				}
				st.Config.Units = []flow.Unit{{ID: "a", Status: status, ResultSHA256: strings.Repeat("a", 64)}}
				st = writeExecutionFixture(t, store, st)
				err = nil
				if err != nil {
					t.Fatal(err)
				}
			}
			config := st.Config
			config.Units = []flow.Unit{{ID: "a", Status: tc.status}}
			if tc.remove {
				config.Units = nil
			}
			raw, _ := json.Marshal(config)
			file := filepath.Join(root, "config.json")
			os.WriteFile(file, raw, 0600)
			_, err = (Service{Root: root}).Execute(cli.MinimalRequest{Command: "intent", Action: "configure", Space: "default", Target: st.ID, Expect: strconv.FormatUint(st.Revision, 10), File: file})
			if err == nil {
				t.Fatal("forged or lost progress")
			}
		})
	}
}

func TestFlowInactiveWorkflowReadAndResume(t *testing.T) {
	for _, status := range []string{"completed", "waiting", "paused"} {
		t.Run(status, func(t *testing.T) {
			s, st := setup(t)
			store := flow.Store{Root: s.Root, Space: "default"}
			st.Status = status
			st.Stage = "discovery"
			var err error
			st = writeExecutionFixture(t, store, st)
			err = nil
			if err != nil {
				t.Fatal(err)
			}
			hook(t, s, "SessionStart", "", "", "", false)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			bind(t, s, st.ID)
			for _, command := range []string{"cat .agents/skills/aidlc-cli/SKILL.md", "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md"} {
				if out := hook(t, s, "PreToolUse", "Bash", "read", command, false); deny(out) {
					t.Fatalf("placed procedure denied: %+v", out)
				}
				session, err := s.Inspect("session")
				if err != nil || session.Tool != "read" {
					t.Fatalf("read did not hold tool slot: %+v %v", session, err)
				}
				if !deny(hook(t, s, "PreToolUse", "Bash", "overlap", command, false)) {
					t.Fatal("overlap allowed")
				}
				hook(t, s, "PostToolUse", "Bash", "read", "", false)
			}
			for _, command := range []string{"touch code.go", "cat code.go", "cat .agents/skills/aidlc-cli/SKILL.md > code.go", "cat .agents/skills/aidlc-cli/SKILL.md; touch code.go", "cat .agents/skills/aidlc-cli/SKILL.md other.md", "cat .agents/skills/aidlc-cli/../aidlc-cli/SKILL.md", "/opt/aidlc intent reopen " + st.ID + " --space default --to integration --reason retry"} {
				if !deny(hook(t, s, "PreToolUse", "Bash", "bad", command, false)) {
					t.Fatalf("unsafe or invalid command allowed: %s", command)
				}
			}
			action := "resume"
			command := "/opt/aidlc intent resume " + st.ID + " --space default --expect " + strconv.FormatUint(st.Revision, 10) + " --reason retry"
			if status == "completed" {
				action = "reopen"
				command = "/opt/aidlc intent reopen " + st.ID + " --space default --expect " + strconv.FormatUint(st.Revision, 10) + " --reason retry --step s02"
			}
			if out := hook(t, s, "PreToolUse", "Bash", "resume", command, false); deny(out) {
				t.Fatalf("valid resume denied: %+v", out)
			}
			_, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: action, Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), Reason: "retry", Step: "s02"})
			if err != nil {
				t.Fatal(err)
			}
			current, err := store.Read(st.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.ExecutionPlan.Draft != nil {
				current = approveFixturePlan(t, store, current)
			}
			if _, err = store.Begin(st.ID, current.Revision); err != nil {
				t.Fatal(err)
			}
			if out := hook(t, s, "PreToolUse", "Bash", "work", "go test ./...", false); deny(out) {
				t.Fatalf("resumed operation denied: %+v", out)
			}
		})
	}
}

func TestFlowWorkflowReadRequiresCurrentRulesAndRealFiles(t *testing.T) {
	for _, mode := range []string{"unbound", "new turn", "changed Rule", "missing", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			s, st := setup(t)
			st.Status = "completed"
			st = writeExecutionFixture(t, flow.Store{Root: s.Root, Space: "default"}, st)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			if mode != "unbound" {
				bind(t, s, st.ID)
			}
			switch mode {
			case "new turn":
				hook(t, s, "UserPromptSubmit", "", "", "", false)
			case "changed Rule":
				file := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, append(raw, []byte("\nChanged\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing", "symlink":
				file := filepath.Join(s.Root, ".agents/skills/aidlc-cli/SKILL.md")
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					other := filepath.Join(t.TempDir(), "other.md")
					if err := os.WriteFile(other, []byte("outside"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(other, file); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !deny(hook(t, s, "PreToolUse", "Bash", "read", "cat .agents/skills/aidlc-cli/SKILL.md", false)) {
				t.Fatal("unsafe read allowed")
			}
		})
	}
}
