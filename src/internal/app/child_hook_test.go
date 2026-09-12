package app

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func childInput(t *testing.T, event, role, tool, command string) HookInput {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"hook_event_name": event, "session_id": "session", "turn_id": "child-turn", "agent_id": "01a08950-c474-70c2-82ce-694129814dd5", "agent_type": role, "cwd": "/parent/root", "tool_name": tool, "tool_use_id": "parent-tool", "tool_input": map[string]string{"command": command}, "tool_response": `{"task_name":"/root/child"}`, "prompt": "approve"})
	if err != nil {
		t.Fatal(err)
	}
	var in HookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in
}
func childSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(filepath.Join(root, "aidlc"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		files[path] = string(raw)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
func TestChildHookNotifications(t *testing.T) {
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop", "SubagentStart", "SubagentStop", "NativePost"} {
		t.Run(event, func(t *testing.T) {
			s, st := agentFixture(t)
			if _, err := (assignment.Store{Root: s.Root}).PreSpawn(assignment.DispatchRequest{Session: "session", Turn: "turn", ToolID: "parent-tool", TaskName: "child", Agent: "aidlc-reviewer", Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash}); err != nil {
				t.Fatal(err)
			}
			state, err := s.Inspect("session")
			if err != nil {
				t.Fatal(err)
			}
			state.Tool = "parent-tool"
			if err := s.save("session", state); err != nil {
				t.Fatal(err)
			}
			before := childSnapshot(t, s.Root)
			in := childInput(t, event, "aidlc-reviewer", "collaborationspawn_agent", "")
			if event == "PostToolUse" {
				in.Tool = "Bash"
			}
			if event == "NativePost" {
				in.Event = "PostToolUse"
			}
			out, err := s.Hook(in)
			if err != nil || len(out) != 0 {
				t.Errorf("child notification affected parent: %+v %v", out, err)
			}
			if after := childSnapshot(t, s.Root); !reflect.DeepEqual(before, after) {
				t.Fatal("child notification changed parent files")
			}
		})
	}
}

// This isolated catalog permits a worker in initialization so its real Begin
// and CheckWork gates can be exercised without synthesizing entry state.
func childWorkerFixture(t *testing.T, begun bool) (Service, flow.State) {
	t.Helper()
	s := Service{Root: t.TempDir(), Binary: "/opt/aidlc"}
	if _, err := install.Codex(s.Root, s.Binary); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Root, "aidlc/workflow/stages/initialization.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "agents:\n", "agents:\n  - role: implementation\n    agent: aidlc-worker\n", 1))
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	store := flow.Store{Root: s.Root, Space: "default"}
	st, err := store.Create("child gate")
	if err != nil {
		t.Fatal(err)
	}
	if begun {
		st, err = store.Begin(st.ID, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CheckWork(st.ID); err != nil {
			t.Fatal(err)
		}
	}
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, st.ID)
	return s, st
}
func TestChildHookEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, role, tool string
		begun, denied    bool
	}{
		{"worker begun", "aidlc-worker", "Bash", true, false},
		{"worker patch", "aidlc-worker", "apply_patch", true, false},
		{"worker before begin", "aidlc-worker", "Bash", false, true},
		{"read only before begin", "aidlc-reviewer", "Bash", false, false},
		{"planner before begin", "aidlc-stage-planner", "Bash", false, false},
		{"missing role", "", "Bash", true, true},
		{"unknown role", "other", "Bash", true, true},
		{"role not in stage", "aidlc-researcher", "Bash", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := childWorkerFixture(t, tc.begun)
			state, err := s.Inspect("session")
			if err != nil {
				t.Fatal(err)
			}
			state.Tool = "parent-tool"
			state.RuleTurn = "older-parent-turn"
			if err := s.save("session", state); err != nil {
				t.Fatal(err)
			}
			before := childSnapshot(t, s.Root)
			command := "echo child"
			if tc.tool == "apply_patch" {
				command = "*** Begin Patch\n*** Add File: worker.go\n+package worker\n*** End Patch"
			}
			in := childInput(t, "PreToolUse", tc.role, tc.tool, command)
			out, err := s.Hook(in)
			if err != nil || deny(out) != tc.denied {
				t.Errorf("child deny=%v want %v: %+v %v", deny(out), tc.denied, out, err)
			}
			if after := childSnapshot(t, s.Root); !reflect.DeepEqual(before, after) {
				t.Fatal("child pre mutated parent")
			}
			in.AgentID = ""
			in.AgentType = ""
			out, err = s.Hook(in)
			if err != nil || !deny(out) {
				t.Fatal("parent busy/rule gate changed", out, err)
			}
		})
	}
}

func TestChildHookCommandBoundary(t *testing.T) {
	s, st := childWorkerFixture(t, true)
	before := childSnapshot(t, s.Root)
	for _, tc := range []struct {
		name, tool, command string
		denied              bool
	}{
		{"help", "Bash", "/opt/aidlc unit claim --help", false},
		{"state read", "Bash", "/opt/aidlc intent show " + st.ID + " --space default", false},
		{"rules read", "Bash", "/opt/aidlc memory rules --space default", false},
		{"diagnostics", "Bash", "/opt/aidlc assignment list", false},
		{"plan read", "Bash", "/opt/aidlc intent plan " + st.ID + " --space default", false},
		{"parent bind", "Bash", "/opt/aidlc session bind " + st.ID + " --space default --session session", true},
		{"reservation release", "Bash", "/opt/aidlc assignment release id --session session --expect 1 --file r.json", true},
		{"unit claim", "Bash", "/opt/aidlc unit claim " + st.ID + " --space default --expect 1 --file r.json", true},
		{"knowledge update", "Bash", "/opt/aidlc memory update codekb/x --space default --body-file r.md --actor process:x --expect hash", true},
		{"approval", "Bash", "/opt/aidlc intent approval " + st.ID + " --space default --expect 1 --file r.json", true},
		{"plan change", "Bash", "/opt/aidlc intent plan " + st.ID + " --space default --expect 1 --file r.json", true},
		{"space outside parser", "Bash", "/opt/aidlc space create other", true},
		{"unknown operation", "Bash", "/opt/aidlc future-mutation", true},
		{"native spawn", "collaborationspawn_agent", "", true},
		{"native request", "collaborationfollowup_task", "", true},
		{"native interrupt", "collaborationinterrupt_agent", "", true},
		{"protected patch", "apply_patch", "*** Begin Patch\n*** Update File: aidlc/.runtime/flow/sessions/session.txt\n+x\n*** End Patch", true},
		{"ordinary patch", "apply_patch", "*** Begin Patch\n*** Add File: code.go\n+package code\n*** End Patch", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := s.Hook(childInput(t, "PreToolUse", "aidlc-worker", tc.tool, tc.command))
			if err != nil || deny(out) != tc.denied {
				t.Errorf("deny=%v want %v: %+v %v", deny(out), tc.denied, out, err)
			}
			if after := childSnapshot(t, s.Root); !reflect.DeepEqual(before, after) {
				t.Fatal("child tool mutated parent")
			}
		})
	}
}
