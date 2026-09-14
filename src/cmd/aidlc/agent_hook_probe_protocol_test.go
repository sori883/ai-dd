//go:build integration && diagnostic

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func agentProbeInvoke(t *testing.T, dir, mode, raw string) ([]byte, []byte, error) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestAgentHookProbeHelper$", "--", "agent-hook", dir, mode)
	cmd.Stdin = strings.NewReader(raw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return out, stderr.Bytes(), err
}

func TestAgentHookProbeProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, event, tool, agent, mode string
		deny                           bool
	}{
		{"observed_deny", "PreToolUse", "collaborationspawn_agent", "probe_worker", "deny", true},
		{"observed_other_role", "PreToolUse", "collaborationspawn_agent", "other", "deny", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			raw := `{"hook_event_name":"` + tc.event + `","tool_name":"` + tc.tool + `","tool_input":{"agent_type":"` + tc.agent + `"},"unknown":{"kept":42}}`
			out, stderr, err := agentProbeInvoke(t, dir, tc.mode, raw)
			if err != nil {
				t.Fatalf("helper: %v stderr=%s", err, stderr)
			}
			if !json.Valid(out) || strings.Contains(string(out), "PASS") {
				t.Fatalf("stdout is not standalone JSON: %q", out)
			}
			if strings.Contains(string(out), `"permissionDecision":"deny"`) != tc.deny {
				t.Fatalf("deny=%v stdout=%s", tc.deny, out)
			}
			files, err := filepath.Glob(filepath.Join(dir, "event-*.json"))
			if err != nil || len(files) != 1 {
				t.Fatalf("records=%v err=%v", files, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			var record agentProbeRecord
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if record.Raw != raw || record.Response != strings.TrimSpace(string(out)) || record.Exit != 0 {
				t.Fatalf("record lost input/output: %+v", record)
			}
		})
	}

}

func TestAgentHookProbeEvidence(t *testing.T) {
	t.Run("measured_deny_and_allow_control", func(t *testing.T) {
		denyDir, allowDir := agentProbeRecordedControl(t, "object")
		e, err := agentProbeCollectEvidence(denyDir, true)
		if err != nil {
			t.Fatal(err)
		}
		control, err := agentProbeCollectEvidence(allowDir, true)
		if err != nil {
			t.Fatal(err)
		}
		e.Control = &control
		if got := agentProbeEvaluate(e)["G0-1"]; got.Status != "pass" {
			t.Fatalf("deny plus observed control=%+v", got)
		}
		saved := append([]agentProbeRecord(nil), control.Records...)
		control.Records = nil
		for _, record := range saved {
			var input agentProbeInput
			_ = json.Unmarshal([]byte(record.Raw), &input)
			if input.Event != "PostToolUse" {
				control.Records = append(control.Records, record)
			}
		}
		if got := agentProbeEvaluate(e)["G0-1"]; got.Status == "pass" {
			t.Errorf("unattributed marker passed: %+v", got)
		}
		control.Records = saved
		for _, record := range saved {
			var input agentProbeInput
			_ = json.Unmarshal([]byte(record.Raw), &input)
			if input.Event == "SubagentStart" {
				control.Records = append(control.Records, record)
				break
			}
		}
		if got := agentProbeEvaluate(e)["G0-1"]; got.Status == "pass" {
			t.Fatalf("duplicate Start passed: %+v", got)
		}
	})
}

func agentProbeRecordedControl(t *testing.T, wrapper string) (string, string) {
	t.Helper()
	base := t.TempDir()
	dirs := []string{filepath.Join(base, "deny"), filepath.Join(base, "allow")}
	const nonce = "probe123"
	const command = "'/probe/test-binary' -test.run='^TestAgentHookProbeProcess$' -- agent-process '/probe/processes' probe123 15000"
	wrap := func(value map[string]any) any {
		data, _ := json.Marshal(value)
		switch wrapper {
		case "string":
			return string(data)
		default:
			return value
		}
	}
	for i, dir := range dirs {
		for _, sub := range []string{"events", "processes"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
				t.Fatal(err)
			}
		}
		parent := "parent-deny"
		if i == 1 {
			parent = "parent-allow"
		}
		parentPath := filepath.Join(dir, "rollout-"+parent+".jsonl")
		childPath := filepath.Join(dir, "rollout-child1.jsonl")
		var parentRows, childRows []map[string]any
		appendCall := func(rows *[]map[string]any, name, id string, input map[string]any, output any) {
			data, _ := json.Marshal(input)
			callType, outType, inputKey := "function_call", "function_call_output", "arguments"

			*rows = append(*rows, map[string]any{"type": "response_item", "payload": map[string]any{"type": callType, "name": name, "call_id": id, inputKey: string(data)}}, map[string]any{"type": "response_item", "payload": map[string]any{"type": outType, "call_id": id, "output": output}})
		}
		spawnName := "spawn_agent"

		output := wrap(map[string]any{"error": "Expected G0 probe denial. Do not retry or substitute another agent."})
		if i == 1 {
			output = wrap(map[string]any{"agent_id": "child1"})
		}
		appendCall(&parentRows, spawnName, "spawn1", map[string]any{"agent_type": "probe_worker"}, output)
		if i == 1 {
			appendCall(&parentRows, "wait", "wait1", map[string]any{"ids": []string{"child1"}}, wrap(map[string]any{"status": "completed"}))
			appendCall(&parentRows, "close_agent", "close1", map[string]any{"id": "child1"}, wrap(map[string]any{"status": "closed"}))

			appendCall(&childRows, "exec_command", "mark1", map[string]any{"cmd": command}, wrap(map[string]any{"exit_code": 0}))

		}
		writeRows := func(path string, rows []map[string]any) {
			var data []byte
			for _, row := range rows {
				b, _ := json.Marshal(row)
				data = append(data, b...)
				data = append(data, '\n')
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		writeRows(parentPath, parentRows)
		writeRows(childPath, childRows)
		var hooks []map[string]any
		hooks = append(hooks, map[string]any{"hook_event_name": "SessionStart", "session_id": parent, "transcript_path": parentPath}, map[string]any{"hook_event_name": "PreToolUse", "session_id": parent, "tool_use_id": "spawn1", "tool_name": "spawn_agent", "tool_input": map[string]any{"agent_type": "probe_worker"}})
		if i == 1 {
			markerSession := "child1"

			hooks = append(hooks, map[string]any{"hook_event_name": "SubagentStart", "session_id": parent, "agent_id": "child1"}, map[string]any{"hook_event_name": "SessionStart", "session_id": "child1", "transcript_path": childPath}, map[string]any{"hook_event_name": "PostToolUse", "session_id": markerSession, "tool_name": "Bash", "tool_use_id": "mark1", "tool_input": map[string]any{"command": command}, "tool_response": map[string]any{"exit_code": 0}})
		}
		for at, hook := range hooks {
			raw, _ := json.Marshal(hook)
			response := "{}"
			if i == 0 && hook["hook_event_name"] == "PreToolUse" {
				response = `{"hookSpecificOutput":{"permissionDecision":"deny"}}`
			}
			if err := agentProbeWriteJSON(filepath.Join(dir, "events", fmt.Sprintf("event-%d.json", at)), agentProbeRecord{Raw: string(raw), Response: response}); err != nil {
				t.Fatal(err)
			}
		}
		if err := agentProbeWriteJSON(filepath.Join(dir, "manifest.json"), map[string]any{"nonce": nonce, "process_command": command}); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			processNonce := nonce

			now := time.Now().UTC()
			state := agentProbeProcessState{Nonce: processNonce, PID: 123, StartedAt: now.Add(-time.Second), ObservedAt: now, EndedAt: now}
			if err := agentProbeWriteJSON(filepath.Join(dir, "processes", "process-"+processNonce+".json"), state); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dirs[0], dirs[1]
}

func TestAgentHookProbeProtocolObservedFault(t *testing.T) {
	for _, mode := range []string{"missing", "nonzero", "save-failure", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			timeout := 3 * time.Second
			if mode == "timeout" {
				timeout = time.Second
			}
			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestAgentHookProbeHelper$", "--", "agent-hook", dir, mode)
			cmd.Stdin = strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"collaborationspawn_agent","tool_input":{"agent_type":"probe_worker"}}`)
			out, err := cmd.Output()
			if len(out) != 0 {
				t.Fatalf("mode=%s unexpected output=%q", mode, out)
			}
			if mode == "timeout" {
				if err == nil || ctx.Err() != context.DeadlineExceeded {
					t.Fatalf("timeout must reach its deadline: err=%v context=%v", err, ctx.Err())
				}
			} else {
				if ctx.Err() != nil {
					t.Fatalf("mode=%s must finish without deadline cancellation: %v", mode, ctx.Err())
				}
				wantExit := 0
				switch mode {
				case "nonzero":
					wantExit = 2
				case "save-failure":
					wantExit = 74
				}
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantExit || (err != nil) != (wantExit != 0) {
					t.Fatalf("mode=%s want exit %d: state=%v err=%v", mode, wantExit, cmd.ProcessState, err)
				}
			}
			files, _ := filepath.Glob(filepath.Join(dir, "event-*.json"))
			if len(files) != 1 {
				t.Fatalf("fault intent records=%v", files)
			}
		})
	}
}

func TestAgentHookProbeEvidenceObservedWire(t *testing.T) {
	for _, mutation := range []string{"crlf_metadata", "metadata_parent", "missing_metadata", "missing_process", "unfinished_capture"} {
		name := mutation
		if name == "crlf_metadata" {
			name = "synthetic_crlf_control"
		}
		t.Run(name, func(t *testing.T) {
			denyDir, allowDir := agentProbeObservedControl(t, mutation)
			denied, err := agentProbeCollectEvidence(denyDir, true)
			if err != nil {
				t.Fatal(err)
			}
			allowed, err := agentProbeCollectEvidence(allowDir, true)
			if err != nil {
				t.Fatal(err)
			}
			if mutation == "unfinished_capture" {
				allowed.Complete = false
			}
			want := "inconclusive"
			if mutation == "crlf_metadata" {
				want = "pass"
			}
			if got := agentProbeAggregate(denied, allowed)["G0-1"]; got.Status != want {
				t.Fatalf("observed wire=%+v want %s", got, want)
			}
			if mutation == "crlf_metadata" {
				data, err := os.ReadFile(filepath.Join(allowDir, "transcript-child1.jsonl"))
				if err != nil || !strings.Contains(string(data), `"session_meta"`) {
					t.Fatalf("explicit child transcript not captured: %s %v", data, err)
				}
				source, readErr := os.ReadFile(filepath.Join(allowDir, "rollout-child1.jsonl"))
				if readErr != nil || !bytes.Equal(source, data) {
					t.Fatalf("child raw changed during capture: %v", readErr)
				}
				if got := agentProbeAggregate(denied, allowed)["G0-4"]; got.Status != "inconclusive" {
					t.Fatalf("empty Bash result inferred termination: %+v", got)
				}
			}
		})
	}
}

func agentProbeObservedControl(t *testing.T, mutation string) (string, string) {
	t.Helper()
	denyDir, allowDir := agentProbeRecordedControl(t, "string")
	for _, dir := range []string{denyDir, allowDir} {
		allow := dir == allowDir
		parent := "parent-deny"
		if allow {
			parent = "parent-allow"
		}
		transcript := filepath.Join(dir, "rollout-"+parent+".jsonl")
		data, err := os.ReadFile(transcript)
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				t.Fatal(err)
			}
			payload := row["payload"].(map[string]any)
			if payload["name"] == "spawn_agent" {
				var input map[string]any
				_ = json.Unmarshal([]byte(payload["arguments"].(string)), &input)
				input["task_name"] = "g0_probe"
				encoded, _ := json.Marshal(input)
				payload["arguments"] = string(encoded)
			}
			if allow && payload["type"] == "function_call_output" && payload["call_id"] == "spawn1" {
				payload["output"] = `{"task_name":"/root/g0_probe"}`
			}
			rows = append(rows, row)
		}
		// An unrelated outer exec is recorded, never decoded into virtual inner tools.
		rows = append(rows, map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "tool-discovery", "input": "text(ALL_TOOLS);"}}, map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call_output", "call_id": "tool-discovery", "output": []any{map[string]any{"type": "input_text", "text": "opaque inventory"}}}})
		var rewritten []byte
		for _, row := range rows {
			encoded, _ := json.Marshal(row)
			rewritten = append(rewritten, encoded...)
			rewritten = append(rewritten, '\n')
		}
		if err := os.WriteFile(transcript, rewritten, 0600); err != nil {
			t.Fatal(err)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "events", "event-*.json"))
		for _, path := range files {
			data, _ := os.ReadFile(path)
			var record agentProbeRecord
			_ = json.Unmarshal(data, &record)
			var event map[string]any
			_ = json.Unmarshal([]byte(record.Raw), &event)
			if event["tool_name"] == "spawn_agent" {
				event["tool_name"] = "collaborationspawn_agent"
				event["tool_input"].(map[string]any)["task_name"] = "g0_probe"
				if allow {
					post := map[string]any{}
					for k, v := range event {
						post[k] = v
					}
					post["hook_event_name"] = "PostToolUse"
					post["tool_response"] = `{"task_name":"/root/g0_probe"}`
					raw, _ := json.Marshal(post)
					if err := agentProbeWriteJSON(filepath.Join(dir, "events", "event-spawn-post.json"), agentProbeRecord{Raw: string(raw), Response: "{}"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if allow && event["hook_event_name"] == "SessionStart" && event["session_id"] == "child1" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if allow && event["hook_event_name"] == "SubagentStart" {
				event["transcript_path"] = filepath.Join(dir, "rollout-child1.jsonl")

			}
			if allow && event["tool_name"] == "Bash" {
				event["session_id"] = parent
				event["agent_id"] = "child1"
				event["turn_id"] = "child-turn"
				event["tool_use_id"] = "exec-internal1"
				event["tool_response"] = ""

				pre := map[string]any{}
				for k, v := range event {
					pre[k] = v
				}
				pre["hook_event_name"] = "PreToolUse"
				delete(pre, "tool_response")

				raw, _ := json.Marshal(pre)
				if err := agentProbeWriteJSON(filepath.Join(dir, "events", "event-bash-pre.json"), agentProbeRecord{Raw: string(raw), Response: "{}"}); err != nil {
					t.Fatal(err)
				}

			}
			raw, _ := json.Marshal(event)
			record.Raw = string(raw)
			if err := agentProbeWriteJSON(path, record); err != nil {
				t.Fatal(err)
			}
		}
		if allow {
			meta := map[string]any{"id": "child1", "agent_path": "/root/g0_probe", "parent_thread_id": parent, "forked_from_id": parent}
			switch mutation {
			case "metadata_parent":
				meta["parent_thread_id"] = "unrelated"
			}
			row, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": meta})
			child := append(row, '\n')
			child = append(child, []byte(`{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","call_id":"outer-child-call","input":"opaque JavaScript, not parsed"}}`+"\n")...)
			if mutation == "crlf_metadata" {
				child = bytes.Replace(child, []byte("\n"), []byte("\r\n"), 1)
			}

			if strings.HasPrefix(mutation, "metadata_") {
				child = append(row, []byte("\nTHIS BODY MUST NOT BE PARSED\n")...)
			}
			if mutation == "missing_metadata" {
				child = []byte(`{"type":"response_item","payload":{}}` + "\n")
			}
			if err := os.WriteFile(filepath.Join(dir, "rollout-child1.jsonl"), child, 0600); err != nil {
				t.Fatal(err)
			}
			if mutation == "missing_process" {
				files, _ := filepath.Glob(filepath.Join(dir, "processes", "process-*.json"))
				for _, path := range files {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	return denyDir, allowDir
}
