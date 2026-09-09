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
		{"deny", "PreToolUse", "spawn_agent", "probe_worker", "deny", true},
		{"allow_control", "PreToolUse", "spawn_agent", "probe_worker", "observe", false},
		{"other_agent", "PreToolUse", "spawn_agent", "other", "deny", false},
		{"other_tool", "PreToolUse", "send_input", "probe_worker", "deny", false},
		{"post", "PostToolUse", "spawn_agent", "probe_worker", "deny", false},
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
	t.Run("parallel_unique", func(t *testing.T) {
		dir := t.TempDir()
		t.Run("writers", func(t *testing.T) {
			for i := 0; i < 12; i++ {
				t.Run(string(rune('a'+i)), func(t *testing.T) {
					t.Parallel()
					_, _, err := agentProbeInvoke(t, dir, "observe", `{"hook_event_name":"SubagentStart"}`)
					if err != nil {
						t.Error(err)
					}
				})
			}
		})
		files, _ := filepath.Glob(filepath.Join(dir, "event-*.json"))
		if len(files) != 12 {
			t.Fatalf("records=%d want 12", len(files))
		}
	})
	t.Run("save_failure", func(t *testing.T) {
		out, stderr, err := agentProbeInvoke(t, filepath.Join(t.TempDir(), "absent"), "deny", `{"hook_event_name":"PreToolUse","tool_name":"spawn_agent","tool_input":{"agent_type":"probe_worker"}}`)
		if err == nil || len(out) != 0 || !strings.Contains(string(stderr), "save evidence") {
			t.Fatalf("save failure: stdout=%q stderr=%q err=%v", out, stderr, err)
		}
	})
}

func TestAgentHookProbeEvidence(t *testing.T) {
	// These wire samples exercise the evaluator, not claims about fixed Codex support.
	fixture := func() agentProbeEvidence {
		return agentProbeEvidence{
			Complete: true,
			Records: []agentProbeRecord{
				{Raw: `{"hook_event_name":"PreToolUse","session_id":"p","tool_use_id":"c1","tool_name":"spawn_agent","tool_input":{"agent_type":"probe_worker"}}`, Response: `{"hookSpecificOutput":{"permissionDecision":"deny"}}`},
			},
			Calls: []agentProbeCall{{ID: "c1", Name: "spawn_agent", Input: `{"agent_type":"probe_worker"}`, Output: `{"error":"Expected G0 probe denial. Do not retry or substitute another agent."}`}},
		}
	}
	for _, tc := range []struct {
		name       string
		change     func(*agentProbeEvidence)
		gate, want string
	}{
		{"deny_without_control", func(e *agentProbeEvidence) {}, "G0-1", "inconclusive"},
		{"deny_with_child_start", func(e *agentProbeEvidence) {
			e.Records = append(e.Records, agentProbeRecord{Raw: `{"hook_event_name":"SubagentStart","agent_id":"child"}`})
		}, "G0-1", "inconclusive"},
		{"duplicate_call", func(e *agentProbeEvidence) { e.Calls = append(e.Calls, e.Calls[0]) }, "G0-1", "inconclusive"},
		{"missing_call_result", func(e *agentProbeEvidence) { e.Calls[0].Output = "" }, "G0-1", "inconclusive"},
		{"unfinished_capture", func(e *agentProbeEvidence) { e.Complete = false }, "G0-1", "inconclusive"},
		{"parent_cwd_is_not_worker_root", func(e *agentProbeEvidence) {
			e.Records = append(e.Records, agentProbeRecord{Raw: `{"hook_event_name":"SubagentStart","agent_id":"child","cwd":"/parent"}`})
		}, "G0-2", "inconclusive"},
		{"prompt_root_is_not_worker_root", func(e *agentProbeEvidence) { e.Calls[0].Input = `{"prompt":"work in /worker"}` }, "G0-2", "inconclusive"},
		{"stop_with_remaining_process", func(e *agentProbeEvidence) {
			e.Records = append(e.Records, agentProbeRecord{Raw: `{"hook_event_name":"SubagentStop","agent_id":"child"}`})
			e.ProcessAfterStop = true
		}, "G0-4", "inconclusive"},
		{"stop_without_process_observation", func(e *agentProbeEvidence) {
			e.Records = append(e.Records, agentProbeRecord{Raw: `{"hook_event_name":"SubagentStop","agent_id":"child"}`})
		}, "G0-4", "inconclusive"},
		{"unperformed_resume", func(e *agentProbeEvidence) {}, "G0-3", "inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := fixture()
			tc.change(&e)
			got := agentProbeEvaluate(e)
			if got[tc.gate].Status != tc.want {
				t.Fatalf("%s=%+v want %s", tc.gate, got[tc.gate], tc.want)
			}
		})
	}
	t.Run("measured_deny_and_allow_control", func(t *testing.T) {
		denyDir, allowDir := agentProbeRecordedControl(t, "object", "")
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

func TestAgentHookProbeFixture(t *testing.T) {
	t.Run("opt_in_and_platform", func(t *testing.T) {
		for _, tc := range []struct {
			name, enabled, os, arch, version string
			want                             bool
		}{
			{"default_off", "", "darwin", "arm64", "codex-cli 0.153.4", false},
			{"fixed", "1", "darwin", "arm64", "codex-cli 0.153.4", true},
			{"wrong_version", "1", "darwin", "arm64", "codex-cli 0.153.5", false},
			{"wrong_os", "1", "linux", "arm64", "codex-cli 0.153.4", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := agentProbeLiveReady(tc.enabled, tc.os, tc.arch, tc.version); got != tc.want {
					t.Fatalf("ready=%v want %v", got, tc.want)
				}
			})
		}
	})
	t.Run("isolated_setup", func(t *testing.T) {
		base := t.TempDir()
		sentinel := filepath.Join(base, "user-config")
		if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
			t.Fatal(err)
		}
		fixture, err := agentProbePrepare(filepath.Join(base, "probe"), "/test/binary", agentProbeScenario{Name: "deny", Mode: "deny"})
		if err != nil {
			t.Fatal(err)
		}
		canonicalBase, err := filepath.EvalSymlinks(base)
		if err != nil {
			t.Fatal(err)
		}
		if fixture.Root == "" || !strings.HasPrefix(fixture.Root, filepath.Join(canonicalBase, "probe")) {
			t.Fatalf("root=%q", fixture.Root)
		}
		data, _ := os.ReadFile(sentinel)
		if string(data) != "unchanged" {
			t.Fatal("outside fixture modified")
		}
		for _, name := range []string{"prompt.txt", "command.json", "manifest.json", "repo/.codex/hooks.json", "repo/.codex/agents/probe-worker.toml"} {
			if _, err := os.Stat(filepath.Join(base, "probe", name)); err != nil {
				t.Error(err)
			}
		}
		args := strings.Join(fixture.Args, " ")
		for _, part := range []string{"--ignore-user-config", "gpt-6-astra", `model_reasoning_effort="xhigh"`, "--dangerously-bypass-hook-trust"} {
			if !strings.Contains(args, part) {
				t.Errorf("command missing %s", part)
			}
		}
		if strings.Contains(args, "CODEX_HOME") || strings.Contains(args, "medium") {
			t.Fatalf("unexpected command %s", args)
		}
	})
	t.Run("actual_worktrees", func(t *testing.T) {
		fixture, err := agentProbePrepare(filepath.Join(t.TempDir(), "probe"), "/test/binary", agentProbeScenario{Name: "parallel-roots", Mode: "observe"})
		if err != nil {
			t.Fatal(err)
		}
		if err := agentProbeInitGit(t.Context(), fixture.Root); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"worker-a", "worker-b"} {
			data, err := os.ReadFile(filepath.Join(fixture.Root, name, ".git"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(data), "gitdir: ") {
				t.Fatalf("%s is not a linked worktree: %s", name, data)
			}
		}
	})
	t.Run("finite_process_cleanup", func(t *testing.T) {
		dir := t.TempDir()
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestAgentHookProbeProcess$", "--", "agent-process", dir, "nonce123", "100")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("process: %v %s", err, out)
		}
		data, err := os.ReadFile(filepath.Join(dir, "process-nonce123.json"))
		if err != nil {
			t.Fatal(err)
		}
		var state agentProbeProcessState
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		if state.Nonce != "nonce123" || state.PID <= 0 || state.EndedAt.IsZero() {
			t.Fatalf("finite exit evidence=%+v", state)
		}
		if err := agentProbeCleanup(dir); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("fault_modes", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			wantExit bool
		}{{"missing", false}, {"nonzero", true}, {"save-failure", true}} {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				out, _, err := agentProbeInvoke(t, dir, tc.name, `{"hook_event_name":"PreToolUse","tool_name":"spawn_agent","tool_input":{"agent_type":"probe_worker"}}`)
				if (err != nil) != tc.wantExit || len(out) != 0 {
					t.Fatalf("fault output=%q err=%v", out, err)
				}
				files, _ := filepath.Glob(filepath.Join(dir, "event-*.json"))
				if len(files) != 1 {
					t.Fatalf("fault intent evidence=%v", files)
				}
			})
		}
	})
	t.Run("all_cases_have_requests", func(t *testing.T) {
		cases := agentProbeScenarios()
		if len(cases) < 9 {
			t.Fatalf("cases=%d", len(cases))
		}
		joined := ""
		for _, c := range cases {
			joined += c.Request
		}
		for _, term := range []string{"spawn_agent", "send_input", "resume_agent", "close_agent", "interrupt", "Unit", "alias", "root"} {
			if !strings.Contains(joined, term) {
				t.Errorf("no request for %s", term)
			}
		}
	})
}

func TestAgentHookProbeFixtureCapture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"events", "processes"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	transcript := filepath.Join(dir, "rollout-session123.jsonl")
	raw := `{"type":"response_item","payload":{"type":"function_call","call_id":"c1","name":"spawn_agent","arguments":"{\"agent_type\":\"probe_worker\"}"}}
{"type":"response_item","payload":{"type":"function_call_output","call_id":"c1","output":"result with unknown fields"}}
`
	if err := os.WriteFile(transcript, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	hook, _ := json.Marshal(map[string]any{"hook_event_name": "SessionStart", "session_id": "session123", "transcript_path": transcript, "future": true})
	if err := agentProbeWriteJSON(filepath.Join(dir, "events", "event-one.json"), agentProbeRecord{Raw: string(hook), Response: "{}"}); err != nil {
		t.Fatal(err)
	}
	state := agentProbeProcessState{Nonce: "capturenonce", PID: 123, StartedAt: time.Now().UTC(), ObservedAt: time.Now().UTC()}
	if err := agentProbeWriteJSON(filepath.Join(dir, "processes", "process-capturenonce.json"), state); err != nil {
		t.Fatal(err)
	}
	if err := agentProbeCollect(dir, true); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(dir, "transcript-session123.jsonl"))
	if err != nil || string(copied) != raw {
		t.Fatalf("transcript lost: %q %v", copied, err)
	}
	calls, err := os.ReadFile(filepath.Join(dir, "calls.json"))
	if err != nil || !strings.Contains(string(calls), "spawn_agent") || !strings.Contains(string(calls), "unknown fields") {
		t.Fatalf("tool inventory lost: %q %v", calls, err)
	}
	snapshot, err := os.ReadFile(filepath.Join(dir, "process-observations.json"))
	if err != nil || !strings.Contains(string(snapshot), "capturenonce") {
		t.Fatalf("pre-cleanup process evidence lost: %q %v", snapshot, err)
	}
}

func TestAgentHookProbeFixtureBudget(t *testing.T) {
	if got := agentProbeCaseTimeout("lifecycle"); got != 5*time.Minute {
		t.Fatalf("lifecycle budget=%s want 5m", got)
	}
	if got := agentProbeCaseTimeout("deny"); got != 2*time.Minute {
		t.Fatalf("ordinary budget=%s want 2m", got)
	}
}

func TestAgentHookProbeEvidenceCollectedControl(t *testing.T) {
	for _, tc := range []struct{ name, wrapper, mutation, want string }{
		{"object", "object", "", "pass"},
		{"function_string", "string", "", "pass"},
		{"custom_array", "array", "", "pass"},
		{"unknown_wrapper", "unknown", "", "inconclusive"},
		{"duplicate_spawn", "string", "duplicate_spawn", "inconclusive"},
		{"ambiguous_session", "string", "ambiguous_session", "inconclusive"},
		{"missing_process", "string", "missing_process", "inconclusive"},
		{"wrong_nonce", "string", "wrong_nonce", "inconclusive"},
		{"missing_child_call", "string", "missing_child_call", "inconclusive"},
		{"opaque_code_mode", "string", "opaque_code_mode", "inconclusive"},
		{"duplicate_output", "string", "duplicate_output", "inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			denyDir, allowDir := agentProbeRecordedControl(t, tc.wrapper, tc.mutation)
			denied, err := agentProbeCollectEvidence(denyDir, true)
			if err != nil {
				t.Fatal(err)
			}
			allowed, err := agentProbeCollectEvidence(allowDir, true)
			if err != nil {
				t.Fatal(err)
			}
			got := agentProbeAggregate(denied, allowed)["G0-1"]
			if got.Status != tc.want {
				t.Fatalf("collector→aggregate G0-1=%+v want %s", got, tc.want)
			}
			// Original output wrappers must remain available, even after evaluation.
			copied, err := os.ReadFile(filepath.Join(allowDir, "calls.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(copied), "child1") {
				t.Fatalf("raw tool output lost: %s", copied)
			}
		})
	}
}

func agentProbeRecordedControl(t *testing.T, wrapper, mutation string) (string, string) {
	t.Helper()
	base := t.TempDir()
	dirs := []string{filepath.Join(base, "deny"), filepath.Join(base, "allow")}
	const nonce = "probe123"
	const command = "'/probe/test-binary' -test.run='^TestAgentHookProbeProcess$' -- agent-process '/probe/processes' probe123 15000"
	wrap := func(value map[string]any, denied bool) any {
		data, _ := json.Marshal(value)
		switch wrapper {
		case "string":
			return string(data)
		case "array":
			header, text := "Script completed\nWall time 0.1 seconds\nOutput:\n", string(data)
			if denied {
				header = "Script failed\nWall time 0.1 seconds\n"
				text = "Script error:\nCommand blocked by PreToolUse hook: Expected G0 probe denial. Do not retry or substitute another agent."
			}
			return []any{map[string]any{"type": "input_text", "text": header}, map[string]any{"type": "input_text", "text": text}}
		case "unknown":
			return map[string]any{"unrecognized_wrapper": value}
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
			if wrapper == "array" {
				callType, outType, inputKey = "custom_tool_call", "custom_tool_call_output", "input"
			}
			*rows = append(*rows, map[string]any{"type": "response_item", "payload": map[string]any{"type": callType, "name": name, "call_id": id, inputKey: string(data)}}, map[string]any{"type": "response_item", "payload": map[string]any{"type": outType, "call_id": id, "output": output}})
		}
		spawnName := "spawn_agent"
		if mutation == "opaque_code_mode" && i == 1 {
			spawnName = "exec"
		}
		output := wrap(map[string]any{"error": "Expected G0 probe denial. Do not retry or substitute another agent."}, true)
		if i == 1 {
			output = wrap(map[string]any{"agent_id": "child1"}, false)
		}
		appendCall(&parentRows, spawnName, "spawn1", map[string]any{"agent_type": "probe_worker"}, output)
		if i == 1 {
			appendCall(&parentRows, "wait", "wait1", map[string]any{"ids": []string{"child1"}}, wrap(map[string]any{"status": "completed"}, false))
			appendCall(&parentRows, "close_agent", "close1", map[string]any{"id": "child1"}, wrap(map[string]any{"status": "closed"}, false))
			if mutation == "duplicate_spawn" {
				appendCall(&parentRows, "spawn_agent", "spawn2", map[string]any{"agent_type": "probe_worker"}, output)
			}
			if mutation == "duplicate_output" {
				parentRows = append(parentRows, parentRows[1])
			}
			if mutation != "missing_child_call" {
				appendCall(&childRows, "exec_command", "mark1", map[string]any{"cmd": command}, wrap(map[string]any{"exit_code": 0}, false))
			}
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
			if mutation == "ambiguous_session" {
				markerSession = parent
			}
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
		if i == 1 && mutation != "missing_process" {
			processNonce := nonce
			if mutation == "wrong_nonce" {
				processNonce = "anothernonce"
			}
			now := time.Now().UTC()
			state := agentProbeProcessState{Nonce: processNonce, PID: 123, StartedAt: now.Add(-time.Second), ObservedAt: now, EndedAt: now}
			if err := agentProbeWriteJSON(filepath.Join(dir, "processes", "process-"+processNonce+".json"), state); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dirs[0], dirs[1]
}
