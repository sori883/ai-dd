package app

import (
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
	"testing"
)

func agentFixture(t *testing.T) (Service, flow.State) {
	t.Helper()
	s := Service{Root: t.TempDir(), Binary: "/opt/aidlc"}
	if _, err := install.Codex(s.Root, s.Binary); err != nil {
		t.Fatal(err)
	}
	st, err := (flow.Store{Root: s.Root, Space: "default"}).Create("agent work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (assignment.Store{Root: s.Root}).Init(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"}); err != nil {
		t.Fatal(err)
	}
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, st.ID)
	return s, st
}
func TestAssignmentStage(t *testing.T) {
	for _, tc := range []struct {
		name, agent string
		denied      bool
	}{
		{"read only without worker sensor", "aidlc-reviewer", false},
		{"planner present", "aidlc-stage-planner", false},
		{"unknown role", "unknown", true},
		{"worker not allowed", "aidlc-worker", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := agentFixture(t)
			in := HookInput{Event: "PreToolUse", Session: "session", Turn: "turn", Tool: "collaborationspawn_agent", ID: "spawn"}
			in.Input.AgentType = tc.agent
			in.Input.TaskName = "probe"
			out, err := s.Hook(in)
			if err != nil || deny(out) != tc.denied {
				t.Fatalf("denied=%v want %v: %+v %v", deny(out), tc.denied, out, err)
			}
		})
	}
}

func TestAssignmentDispatch(t *testing.T) {
	s, _ := agentFixture(t)
	in := HookInput{Event: "PreToolUse", Session: "session", Turn: "turn", Tool: "collaborationspawn_agent", ID: "spawn"}
	in.Input.AgentType = "aidlc-reviewer"
	in.Input.TaskName = "review"
	out, err := s.Hook(in)
	if err != nil || deny(out) {
		t.Fatalf("spawn: %+v %v", out, err)
	}
	state, err := s.Inspect("session")
	if err != nil || state.Tool != "" {
		t.Fatalf("agent lifetime stored in bash tool: %+v %v", state, err)
	}
	follow := HookInput{Event: "PreToolUse", Session: "session", Turn: "turn", Tool: "collaborationfollowup_task", ID: "follow"}
	follow.Input.Target = "review"
	out, err = s.Hook(follow)
	if err != nil || !deny(out) {
		t.Fatal("missing post followup accepted", out, err)
	}
	post := in
	post.Event = "PostToolUse"
	post.Response = []byte(`"{\"task_name\":\"/root/review\"}"`)
	if _, err := s.Hook(post); err != nil {
		t.Fatal(err)
	}
	out, err = s.Hook(follow)
	if err != nil || deny(out) {
		t.Fatalf("bound relative followup: %+v %v", out, err)
	}
	follow.Tool = "collaborationsend_message"
	follow.Input.Target = "/root/review"
	out, err = s.Hook(follow)
	if err != nil || deny(out) {
		t.Fatalf("canonical message: %+v %v", out, err)
	}
	// Stopping remains possible after the selected turn has lost its Rules.
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	follow.Tool = "collaborationinterrupt_agent"
	out, err = s.Hook(follow)
	if err != nil || deny(out) {
		t.Fatalf("interrupt blocked: %+v %v", out, err)
	}
	reg, err := (assignment.Store{Root: s.Root}).Read()
	if err != nil || len(reg.Dispatches) != 1 || reg.Dispatches[0].Status != "bound" {
		t.Fatalf("interrupt changed lifetime: %+v %v", reg, err)
	}
}
