//go:build integration && diagnostic

package main

import (
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"path/filepath"

	"testing"
)

type boundaryObservation struct {
	Raw, Output json.RawMessage
	States      []flow.State
	Bound       bool
}

func verifyBoundaryEvidence(binary string, records []boundaryObservation, executions map[string]int, beforeExists, afterExists bool) error {
	fail := func() error {
		return fmt.Errorf("missing denial, document repair, begin, or real post-begin operation")
	}
	if beforeExists || !afterExists {
		return fail()
	}
	denied, repaired, begun, worked := false, false, false, false
	pending := map[string]boundaryObservation{}
	id, session := "", ""
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
			if h.Input.Command == "touch boundary-before.txt" && decision.Specific.Decision == "deny" && record.Bound {
				for _, st := range record.States {
					if st.Entry == nil {
						denied = true
						id = st.ID
						session = h.Session
					}
				}
			}
			if decision.Specific.Decision != "deny" {
				pending[key] = record
			}
			continue
		}
		if h.Event != "PostToolUse" {
			continue
		}
		pre, ok := pending[key]
		if !ok {
			continue
		}
		delete(pending, key)
		var before app.HookInput
		if json.Unmarshal(pre.Raw, &before) != nil || before.Input.Command != h.Input.Command {
			return fail()
		}
		exit, ran := executions[h.Session+"/"+h.Input.Command]
		if !ran || exit != 0 || h.Session != session {
			continue
		}
		if h.Input.Command == "touch boundary-after.txt" {
			if begun {
				for _, st := range record.States {
					if st.ID == id && st.Entry != nil {
						worked = true
					}
				}
			}
			continue
		}
		argv, ok := flowShellWords(h.Input.Command)
		if !ok || len(argv) < 2 || (argv[0] != binary && argv[0] != filepath.Join(filepath.Dir(binary), "okf")) {
			continue
		}
		r, err := cli.ParseCommand(argv[1:])
		if argv[0] == filepath.Join(filepath.Dir(binary), "okf") {
			var q okfcli.CommandRequest
			q, err = okfcli.ParseCommand(argv[1:])
			r = cli.CommandRequest{Command: "memory", Action: q.Action, Target: q.Target, Space: q.Space}
		}
		if err != nil {
			continue
		}
		if denied && r.Command == "memory" && (r.Action == "create" || r.Action == "update") && r.Space == "default" && r.Target == "codekb/current-analysis" {
			repaired = true
		}
		if repaired && r.Command == "intent" && r.Action == "begin" && r.Target == id {
			for _, st := range record.States {
				if st.ID == id && st.Entry != nil {
					begun = true
				}
			}
		}
	}
	if !denied || !repaired || !begun || !worked {
		return fail()
	}
	return nil
}

func TestBoundaryEvidenceSequence(t *testing.T) {
	// Synthetic checker fixture; not a live observation.
	binary := "/aidlc"
	st := flow.State{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Stage: "discovery"}
	var records []boundaryObservation
	execs := map[string]int{}
	add := func(event, id, command, decision string, started bool) {
		h := app.HookInput{Event: event, Session: "session", Tool: "Bash", ID: id}
		h.Input.Command = command
		raw, _ := json.Marshal(h)
		out, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{"permissionDecision": decision}})
		state := st
		if started {
			state.Entry = &flow.StageEntry{Stage: "discovery"}
		}
		records = append(records, boundaryObservation{Raw: raw, Output: out, States: []flow.State{state}, Bound: true})
		execs["session/"+command] = 0
	}
	add("PreToolUse", "deny", "touch boundary-before.txt", "deny", false)
	for i, command := range []string{"/okf update codekb/current-analysis --space default --body-file draft --actor process:a --expect hash", "/aidlc intent begin " + st.ID + " --space default --expect 2", "touch boundary-after.txt"} {
		add("PreToolUse", fmt.Sprint(i), command, "", i == 2)
		add("PostToolUse", fmt.Sprint(i), command, "", i >= 1)
	}
	if err := verifyBoundaryEvidence(binary, records, execs, false, true); err != nil {
		t.Fatal(err)
	}
	if verifyBoundaryEvidence(binary, records, nil, false, true) == nil {
		t.Fatal("self report accepted")
	}
	if verifyBoundaryEvidence(binary, records, execs, true, true) == nil {
		t.Fatal("forbidden canary accepted")
	}
}

// boundaryEvidenceCommand matches the fixed CLI's known shell envelopes only.
func boundaryEvidenceCommand(command string) string {
	if args, ok := flowShellWords(command); ok && len(args) == 3 && (args[0] == "/bin/zsh" || args[0] == "/bin/bash") && (args[1] == "-lc" || args[1] == "-c") {
		return args[2]
	}
	return command
}

func TestBoundaryEvidenceCommand(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"/bin/bash -lc 'touch boundary-after.txt'", "touch boundary-after.txt"}, {"/bin/sh -c 'touch boundary-after.txt'", "/bin/sh -c 'touch boundary-after.txt'"}} {
		if got := boundaryEvidenceCommand(tc.input); got != tc.want {
			t.Errorf("%q -> %q", tc.input, got)
		}
	}
}
