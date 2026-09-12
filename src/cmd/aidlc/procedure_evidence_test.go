package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func verifyProcedureEvidence(binary, id string, records []boundaryObservation, executions map[string]int, canary bool) error {
	fail := func() error {
		return fmt.Errorf("missing same-session procedure/begin/work/reopen/new-procedure evidence")
	}
	if !canary {
		return fail()
	}
	step := 0
	session := ""
	pending := map[string]app.HookInput{}
	for _, record := range records {
		var h app.HookInput
		if json.Unmarshal(record.Raw, &h) != nil {
			return fail()
		}
		if h.Tool != "Bash" {
			continue
		}
		var decision struct {
			Specific struct {
				Decision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal(record.Output, &decision) != nil {
			return fail()
		}
		key := h.Session + "/" + h.ID
		if h.Event == "PreToolUse" {
			if decision.Specific.Decision != "deny" {
				pending[key] = h
			}
			continue
		}
		if h.Event != "PostToolUse" {
			continue
		}
		pre, ok := pending[key]
		delete(pending, key)
		if !ok || pre.Input.Command != h.Input.Command {
			continue
		}
		code, ran := executions[h.Session+"/"+h.Input.Command]
		if !ran || code != 0 || !record.Bound {
			continue
		}
		if session != "" && session != h.Session {
			continue
		}
		var st flow.State
		for _, candidate := range record.States {
			if candidate.ID == id {
				st = candidate
			}
		}
		if st.ID == "" {
			continue
		}
		if step == 2 && h.Input.Command == "touch procedure-work.txt" && st.Stage == "tdd" && st.Entry != nil {
			step++
			continue
		}
		argv, ok := flowShellWords(h.Input.Command)
		if !ok || len(argv) < 2 || argv[0] != binary {
			continue
		}
		r, err := cli.ParseCommand(argv[1:])
		if err != nil || r.Command != "intent" || r.Target != id || r.Space != "default" {
			continue
		}
		switch step {
		case 0:
			if r.Action == "procedure" && st.Stage == "tdd" {
				session = h.Session
				step++
			}
		case 1:
			if r.Action == "begin" && st.Stage == "tdd" && st.Entry != nil {
				step++
			}
		case 3:
			if r.Action == "reopen" && r.Step == "s03" && st.ExecutionPlan.Draft != nil {
				step++
			}
		case 4:
			if r.Action == "plan-approval" && st.Stage == "planning" && st.Entry == nil {
				step++
			}
		case 5:
			if r.Action == "procedure" && st.Stage == "planning" {
				step++
			}
		}
	}
	if step != 6 {
		return fail()
	}
	return nil
}

func TestProcedureEvidenceSequence(t *testing.T) {
	binary, id := "/aidlc", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commands := []string{binary + " intent procedure " + id + " --space default", binary + " intent begin " + id + " --space default --expect 3", "touch procedure-work.txt", binary + " intent reopen " + id + " --space default --expect 4 --step s03 --reason reconsider", binary + " intent plan-approval " + id + " --space default --expect 5 --file decision.json", binary + " intent procedure " + id + " --space default"}
	records := []boundaryObservation{}
	executions := map[string]int{}
	for i, command := range commands {
		for _, event := range []string{"PreToolUse", "PostToolUse"} {
			h := app.HookInput{Event: event, Session: "session", ID: fmt.Sprint(i), Tool: "Bash"}
			h.Input.Command = command
			raw, _ := json.Marshal(h)
			stage := "tdd"
			if i == 5 || i == 4 && event == "PostToolUse" {
				stage = "planning"
			}
			st := flow.State{ID: id, Stage: stage}
			if i == 3 && event == "PostToolUse" {
				st.ExecutionPlan.Draft = &flow.PlanVersion{ReopenStepID: "s03"}
			}
			if (i == 1 && event == "PostToolUse") || i == 2 {
				st.Entry = &flow.StageEntry{Stage: stage}
			}
			records = append(records, boundaryObservation{Raw: raw, Output: json.RawMessage(`{}`), Bound: true, States: []flow.State{st}})
		}
		executions["session/"+command] = 0
	}
	if err := verifyProcedureEvidence(binary, id, records, executions, true); err != nil {
		t.Fatal(err)
	}
	for i := range records {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			copyRecords := append([]boundaryObservation(nil), records[:i]...)
			copyRecords = append(copyRecords, records[i+1:]...)
			if verifyProcedureEvidence(binary, id, copyRecords, executions, true) == nil {
				t.Fatal("missing observation accepted")
			}
		})
	}
	if verifyProcedureEvidence(binary, id, records, nil, true) == nil {
		t.Fatal("unexecuted commands accepted")
	}
	if verifyProcedureEvidence(binary, id, records, executions, false) == nil {
		t.Fatal("missing canary accepted")
	}
}
