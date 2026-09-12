package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// These observations are test evidence, never product audit records.
type hookProbeEvent struct {
	Input hookProbeInput    `json:"input"`
	Files map[string]string `json:"files"`
}
type hookProbeInput struct {
	Event   string `json:"hook_event_name"`
	Session string `json:"session_id"`
	Turn    string `json:"turn_id"`
	Tool    string `json:"tool_name"`
	ID      string `json:"tool_use_id"`
	Input   struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	Response   json.RawMessage `json:"tool_response"`
	Active     bool            `json:"stop_hook_active"`
	Transcript string          `json:"transcript_path"`
}
type hookProbeCall struct {
	ID, Name, Arguments, Output string
}

var hookProbeCommands = []string{
	"touch probe-forbidden",
	"printf success > probe-success; exit 0",
	"printf failure > probe-failure; exit 7",
	"sleep 3; printf async-success > probe-async-success; exit 0",
	"sleep 3; printf async-failure > probe-async-failure; exit 7",
	"*** Begin Patch\n*** Add File: probe-patch\n+patch\n*** End Patch",
}
var hookProbeFiles = []string{"probe-forbidden", "probe-success", "probe-failure", "probe-async-success", "probe-async-failure", "probe-patch"}
var hookProbeContents = []string{"", "success", "failure", "async-success", "async-failure", "patch\n"}

func TestHookProbeVerify(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*[]hookProbeEvent, *[]hookProbeCall, map[string]string)
	}{
		{"complete", nil},
		{"missing_start", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { *e = (*e)[1:] }},
		{"missing_prompt", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) {
			*e = append((*e)[:1], (*e)[2:]...)
		}},
		{"missing_post", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) {
			*e = append((*e)[:4], (*e)[5:]...)
		}},
		{"mismatched_id", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { (*e)[4].Input.ID = "wrong" }},
		{"wrong_turn", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { (*e)[4].Input.Turn = "wrong" }},
		{"early_post", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { (*e)[8].Files = nil }},
		{"pending_transport", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[3].Output = `{"session_id":42}`
		}},
		{"wrong_exit", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[1].Output = `{"exit_code":0}`
		}},
		{"forbidden_created", func(_ *[]hookProbeEvent, _ *[]hookProbeCall, f map[string]string) { f["probe-forbidden"] = "" }},
		{"missing_patch", func(_ *[]hookProbeEvent, _ *[]hookProbeCall, f map[string]string) { delete(f, "probe-patch") }},
		{"missing_reentry", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { *e = (*e)[:len(*e)-1] }},
		{"stop_not_active", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) {
			(*e)[len(*e)-1].Input.Active = false
		}},
		{"not_async", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[2].Output = `{"exit_code":0}`
		}},
		{"missing_poll", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			*c = append((*c)[:3], (*c)[4:]...)
		}},
		{"wrong_poll_session", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[3].Arguments = `{"session_id":99}`
		}},
		{"poll_not_terminal", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[3].Output = `{"output":""}`
		}},
		{"missing_sync", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) { *c = (*c)[1:] }},
		{"reordered_transport", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[0], (*c)[1] = (*c)[1], (*c)[0]
		}},
		{"pending_and_exit", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[2].Output = `{"session_id":0,"exit_code":0}`
		}},
		{"wrong_async_exit", func(_ *[]hookProbeEvent, c *[]hookProbeCall, _ map[string]string) {
			(*c)[5].Output = `{"exit_code":0}`
		}},
		{"duplicate_post", func(e *[]hookProbeEvent, _ *[]hookProbeCall, _ map[string]string) { *e = append(*e, (*e)[4]) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, calls, files := hookProbeFixture()
			if tt.mutate != nil {
				tt.mutate(&events, &calls, files)
			}
			err := hookProbeVerify(events, calls, files)
			if (err != nil) != (tt.mutate != nil) {
				t.Fatalf("verification error = %v, want rejection = %v", err, tt.mutate != nil)
			}
		})
	}
}

func hookProbeFixture() ([]hookProbeEvent, []hookProbeCall, map[string]string) {
	events := []hookProbeEvent{{Input: hookProbeInput{Event: "SessionStart", Session: "session"}}, {Input: hookProbeInput{Event: "UserPromptSubmit", Session: "session", Turn: "turn"}}}
	files := map[string]string{}
	var calls []hookProbeCall
	for i, command := range hookProbeCommands {
		input := hookProbeInput{Event: "PreToolUse", Session: "session", Turn: "turn", Tool: "Bash", ID: hookProbeFiles[i]}
		input.Input.Command = command
		if i == 5 {
			input.Tool = "apply_patch"
		}
		events = append(events, hookProbeEvent{Input: input})
		if i == 0 {
			continue
		}
		input.Event = "PostToolUse"
		code := "0"
		if i == 2 || i == 4 {
			code = "7"
		}
		input.Response = json.RawMessage(`""`)
		if i == 5 {
			input.Response = json.RawMessage(`"Success. Updated the following files: A probe-patch"`)
		}
		events = append(events, hookProbeEvent{Input: input, Files: map[string]string{hookProbeFiles[i]: hookProbeContents[i]}})
		files[hookProbeFiles[i]] = hookProbeContents[i]
		if i != 5 {
			args, _ := json.Marshal(map[string]any{"cmd": command, "yield_time_ms": 1})
			output := `{"exit_code":` + code + `}`
			if i == 3 || i == 4 {
				output = `{"session_id":` + code + `}`
			}
			calls = append(calls, hookProbeCall{ID: "outer-" + input.ID, Name: "exec_command", Arguments: string(args), Output: output})
			if i == 3 || i == 4 {
				calls = append(calls, hookProbeCall{ID: "poll" + code, Name: "write_stdin", Arguments: `{"session_id":` + code + `}`, Output: `{"exit_code":` + code + `}`})
			}
		}
	}
	events = append(events, hookProbeEvent{Input: hookProbeInput{Event: "Stop", Session: "session", Turn: "turn"}}, hookProbeEvent{Input: hookProbeInput{Event: "Stop", Session: "session", Turn: "next", Active: true}})
	return events, calls, files
}

func hookProbeVerify(events []hookProbeEvent, calls []hookProbeCall, files map[string]string) error {
	if _, ok := files["probe-forbidden"]; ok {
		return fmt.Errorf("blocked canary exists")
	}
	if len(events) < 2 || events[0].Input.Event != "SessionStart" || events[0].Input.Session == "" {
		return fmt.Errorf("missing SessionStart")
	}
	session := events[0].Input.Session
	prompt := false
	stopFirst, stopAgain := false, false
	pre := map[string]hookProbeInput{}
	post := map[string]bool{}
	for _, observed := range events {
		e := observed.Input
		if e.Session != session {
			return fmt.Errorf("session mismatch: %s", e.Event)
		}
		switch e.Event {
		case "UserPromptSubmit":
			if e.Turn == "" {
				return fmt.Errorf("prompt has no turn")
			}
			prompt = true
		case "Stop":
			if !stopFirst {
				if e.Active {
					return fmt.Errorf("first Stop is already active")
				}
				stopFirst = true
			} else if e.Active {
				stopAgain = true
			}
		case "PreToolUse", "PostToolUse":
			for i, command := range hookProbeCommands {
				if e.Input.Command != command {
					continue
				}
				if !prompt || e.ID == "" || e.Turn == "" {
					return fmt.Errorf("tool lacks prompt/id/turn")
				}
				tool := "Bash"
				if i == 5 {
					tool = "apply_patch"
				}
				if e.Tool != tool {
					return fmt.Errorf("wrong tool for %s", hookProbeFiles[i])
				}
				if e.Event == "PreToolUse" {
					pre[command] = e
					continue
				}
				before, ok := pre[command]
				if !ok || before.ID != e.ID || before.Turn != e.Turn {
					return fmt.Errorf("unmatched Post for %s", hookProbeFiles[i])
				}
				if i == 0 {
					return fmt.Errorf("blocked command has Post")
				}
				if observed.Files[hookProbeFiles[i]] != hookProbeContents[i] {
					return fmt.Errorf("early Post or missing effect for %s", hookProbeFiles[i])
				}
				if post[command] {
					return fmt.Errorf("duplicate Post for %s", hookProbeFiles[i])
				}
				if i != 5 {
					var stdout string
					if json.Unmarshal(e.Response, &stdout) != nil || stdout != "" {
						return fmt.Errorf("unexpected Bash stdout shape for %s: %s", hookProbeFiles[i], e.Response)
					}
				}
				post[command] = true
			}
		}
	}
	if !prompt || !stopFirst || !stopAgain {
		return fmt.Errorf("missing prompt or Stop reentry")
	}
	for i, command := range hookProbeCommands {
		_, ok := pre[command]
		if !ok {
			return fmt.Errorf("missing Pre for %s", hookProbeFiles[i])
		}
		if i == 0 {
			continue
		}
		if !post[command] || files[hookProbeFiles[i]] != hookProbeContents[i] {
			return fmt.Errorf("missing terminal Post/effect for %s", hookProbeFiles[i])
		}
	}
	// The fixed hook exposes stdout, not exit_code. Hook pairing above proves
	// the terminal notification and its side effect; transport below proves
	// success/failure and an actual asynchronous start followed by polling.
	at := 0
	for i := 1; i <= 4; i++ {
		for at < len(calls) && calls[at].Name == "exec_command" && hookProbeCommand(calls[at]) == hookProbeCommands[0] {
			at++
		}
		if at >= len(calls) || calls[at].Name != "exec_command" || hookProbeCommand(calls[at]) != hookProbeCommands[i] {
			return fmt.Errorf("missing or reordered transport for %s", hookProbeFiles[i])
		}
		result := calls[at].Output
		at++
		expected := 0
		if i == 2 || i == 4 {
			expected = 7
		}
		if i < 3 {
			if !hookProbeExited(result, expected) {
				return fmt.Errorf("wrong synchronous exit for %s: %s", hookProbeFiles[i], result)
			}
			continue
		}
		pending := hookProbeRunning(result)
		if pending == "" {
			return fmt.Errorf("missing asynchronous start for %s", hookProbeFiles[i])
		}
		terminal := false
		for at < len(calls) && calls[at].Name == "write_stdin" {
			call := calls[at]
			at++
			var args struct {
				Session json.Number `json:"session_id"`
			}
			if json.Unmarshal([]byte(call.Arguments), &args) != nil || args.Session.String() != pending {
				return fmt.Errorf("poll session mismatch")
			}
			if hookProbeExited(call.Output, expected) {
				terminal = true
				break
			}
			if hookProbeRunning(call.Output) != pending {
				return fmt.Errorf("unknown or wrong poll result: %s", call.Output)
			}
		}
		if !terminal {
			return fmt.Errorf("missing terminal poll for %s", hookProbeFiles[i])
		}
	}
	if at < len(calls) && (len(calls)-at != 1 || calls[at].Name != "apply_patch") {
		return fmt.Errorf("unexpected trailing transport")
	}
	return nil
}

func hookProbeCommand(call hookProbeCall) string {
	var args struct {
		Command string `json:"cmd"`
	}
	if json.Unmarshal([]byte(call.Arguments), &args) != nil {
		return ""
	}
	return args.Command
}
func hookProbeExited(output string, code int) bool {
	var result struct {
		Exit    *int `json:"exit_code"`
		Session *int `json:"session_id"`
	}
	return json.Unmarshal([]byte(output), &result) == nil && result.Exit != nil && *result.Exit == code && result.Session == nil
}
func hookProbeRunning(output string) string {
	var result struct {
		Exit    *int        `json:"exit_code"`
		Session json.Number `json:"session_id"`
	}
	if json.Unmarshal([]byte(output), &result) != nil || result.Exit != nil {
		return ""
	}
	if _, err := result.Session.Int64(); err != nil {
		return ""
	}
	return result.Session.String()
}
