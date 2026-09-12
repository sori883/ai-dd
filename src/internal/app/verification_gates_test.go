package app

import (
	"github.com/sori883/ai-dd/src/internal/flow"
	"strconv"
	"testing"
)

func TestVerificationGatesPendingHook(t *testing.T) {
	s, st := executionCLIFixture(t)
	store := flow.Store{Root: s.Root, Space: "default"}
	var err error
	st, err = store.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	request := flow.PlanRequest{Reason: "investigate", Steps: []flow.PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}}}
	for _, stage := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
		request.Omitted = append(request.Omitted, flow.StageOmission{Stage: stage, Reason: "not needed"})
	}
	st, err = store.ProposePlan(st.ID, st.Revision, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.bindFlow("session", "default", st.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Hook(HookInput{Event: "UserPromptSubmit", Session: "session", Turn: "A", Prompt: "consider this plan"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.bindFlow("session", "default", st.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		command string
		deny    bool
	}{{"/opt/aidlc intent hash " + st.ID + " --space default", false}, {"touch code.go", true}, {"cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md", false}, {"cat code.go", false}, {"/opt/aidlc intent plan " + st.ID + " --space default", false}, {"/opt/aidlc intent plan-approval " + st.ID + " --space default --expect " + strconv.FormatUint(st.Revision, 10) + " --file decision.json", false}} {
		input := HookInput{Event: "PreToolUse", Session: "session", Turn: "A", Tool: "Bash", ID: "tool"}
		input.Input.Command = tc.command
		out, err := s.Hook(input)
		if err != nil {
			t.Fatal(err)
		}
		specific, _ := out["hookSpecificOutput"].(map[string]any)
		denied := specific["permissionDecision"] == "deny"
		if denied != tc.deny {
			t.Errorf("command %q denied=%v: %+v", tc.command, denied, out)
		}
		if !denied {
			if _, err = s.Hook(HookInput{Event: "PostToolUse", Session: "session", ID: "tool"}); err != nil {
				t.Fatal(err)
			}
		}
	}
}
