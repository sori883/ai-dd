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

func executionCLIFixture(t *testing.T) (Service, flow.State) {
	t.Helper()
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	stages := []map[string]string{}
	for _, stage := range []string{"initialization", "discovery", "architecture-analysis", "planning", "tdd", "integration"} {
		stages = append(stages, map[string]string{"id": stage, "name": stage, "procedure": "stages/" + stage + ".md"})
		if stage == "initialization" || stage == "architecture-analysis" {
			body := "---\nstage_id: " + stage + "\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: " + stage + "-start\n  end: " + stage + "-end\n---\n# Inspect\nInspect settings.\n"
			if err := os.WriteFile(filepath.Join(root, "aidlc/workflow/stages/"+stage+".md"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	raw, _ := json.Marshal(map[string]any{"schema_version": 2, "required_prefix": []string{"initialization", "discovery"}, "stages": stages})
	if err := os.WriteFile(filepath.Join(root, "aidlc/workflow/stage-graph.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	st, err := (flow.Store{Root: root, Space: "default"}).Create("execution cli")
	if err != nil {
		t.Fatal(err)
	}
	return Service{Root: root, Binary: "/opt/aidlc"}, st
}
func TestExecutionPlanCLIPlanAndPrompt(t *testing.T) {
	s, st := executionCLIFixture(t)
	r := cli.MinimalRequest{Command: "intent", Action: "plan", Target: st.ID, Space: "default"}
	if _, err := s.Execute(r); err != nil {
		t.Fatalf("plan read: %v", err)
	}
	plan := flow.PlanRequest{Reason: "investigate", Steps: []flow.PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}}}
	for _, stage := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
		plan.Omitted = append(plan.Omitted, flow.StageOmission{Stage: stage, Reason: "not needed"})
	}
	raw, _ := json.Marshal(plan)
	file := filepath.Join(s.Root, "plan.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	r.File = file
	r.Expect = strconv.FormatUint(st.Revision, 10)
	raw, err := s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if _, err = s.bindFlow("session", "default", st.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, turn := range []string{"A", "B", "A"} {
		out, err := s.Hook(HookInput{Event: "UserPromptSubmit", Session: "session", Turn: turn, Prompt: "approve"})
		if err != nil {
			t.Fatal(err)
		}
		if turn == "A" && len(out) > 0 {
			return
		}
	}
	t.Fatal("hook allowed A B A replay")
}
func TestExecutionPlanCLIPromptEscapedLimit(t *testing.T) {
	s, st := executionCLIFixture(t)
	if _, err := s.bindFlow("session", "default", st.ID, false); err != nil {
		t.Fatal(err)
	}
	out, err := s.Hook(HookInput{Event: "UserPromptSubmit", Session: "session", Turn: "A", Prompt: strings.Repeat("\x00", 64*1024)})
	if err != nil {
		t.Fatal(err)
	}
	if out["continue"] != false {
		t.Fatal("escaped source above capacity accepted")
	}
}

func TestExecutionPlanCLIPendingHook(t *testing.T) {
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
	}{{"touch code.go", true}, {"cat code.go", false}, {"/opt/aidlc intent plan " + st.ID + " --space default", false}, {"/opt/aidlc intent plan-approval " + st.ID + " --space default --expect " + strconv.FormatUint(st.Revision, 10) + " --file decision.json", false}} {
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

func TestExecutionPlanCLILegacyAdvance(t *testing.T) {
	s, st := executionCLIFixture(t)
	_, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "advance", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10)})
	if err == nil || !strings.Contains(err.Error(), "finish") {
		t.Fatalf("old advance lacks finish guidance: %v", err)
	}
}
func TestExecutionPlanCLIStrictDraft(t *testing.T) {
	s, st := executionCLIFixture(t)
	plan := flow.PlanRequest{Reason: "investigation", Steps: []flow.PlanStepInput{{ID: "s01", Stage: "initialization"}, {ID: "s02", Stage: "discovery"}}}
	for _, stage := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
		plan.Omitted = append(plan.Omitted, flow.StageOmission{Stage: stage, Reason: "not needed"})
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "{", "{\"reason\":\"duplicate\",", 1))
	file := filepath.Join(s.Root, "plan.json")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Execute(cli.MinimalRequest{Command: "intent", Action: "plan", Target: st.ID, Space: "default", Expect: strconv.FormatUint(st.Revision, 10), File: file}); err == nil {
		t.Fatal("duplicate plan JSON key accepted")
	}
}

func TestExecutionPlanCLIProcedureStep(t *testing.T) {
	s, st := executionCLIFixture(t)
	raw, err := s.Execute(cli.MinimalRequest{Command: "intent", Action: "procedure", Target: st.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err = json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if view["step_id"] != "s01" || view["advance"] != "s02" {
		t.Fatalf("procedure lost current/next execution: %s", raw)
	}
}
