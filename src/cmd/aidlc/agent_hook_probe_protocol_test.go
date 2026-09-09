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
		{"observed_deny", "PreToolUse", "collaborationspawn_agent", "probe_worker", "deny", true},
		{"observed_other_role", "PreToolUse", "collaborationspawn_agent", "other", "deny", false},
		{"observed_post", "PostToolUse", "collaborationspawn_agent", "probe_worker", "deny", false},
		{"similar_name", "PreToolUse", "collaboration_spawn_agent", "probe_worker", "deny", false},
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

func TestAgentHookProbeFixtureYield(t *testing.T) {
	for _, scenario := range agentProbeScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "probe")
			if _, err := agentProbePrepare(dir, "/test/binary", scenario); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			want, other := "yield_time_ms=20000", "yield_time_ms=1"
			if scenario.Name == "lifecycle" {
				want, other = "yield_time_ms=1", "yield_time_ms=20000"
			}
			if !strings.Contains(string(data), want) || strings.Contains(string(data), other) {
				t.Fatalf("%s prompt must specify %s only; got %s", scenario.Name, want, data)
			}
		})
	}
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
	for _, mutation := range []string{"", "crlf_metadata", "duplicate_metadata", "metadata_parent", "metadata_fork", "metadata_agent", "metadata_path", "missing_metadata", "duplicate_start", "bash_agent", "missing_bash_pre", "bash_call", "missing_process"} {
		name := mutation
		if name == "" {
			name = "observed_control"
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
			want := "inconclusive"
			if mutation == "" || mutation == "crlf_metadata" {
				want = "pass"
			}
			if got := agentProbeAggregate(denied, allowed)["G0-1"]; got.Status != want {
				t.Fatalf("observed wire=%+v want %s", got, want)
			}
			if mutation == "" || mutation == "crlf_metadata" {
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
	denyDir, allowDir := agentProbeRecordedControl(t, "string", "")
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
				if mutation == "duplicate_start" {
					raw, _ := json.Marshal(event)
					if err := agentProbeWriteJSON(filepath.Join(dir, "events", "event-start-duplicate.json"), agentProbeRecord{Raw: string(raw), Response: "{}"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if allow && event["tool_name"] == "Bash" {
				event["session_id"] = parent
				event["agent_id"] = "child1"
				event["turn_id"] = "child-turn"
				event["tool_use_id"] = "exec-internal1"
				event["tool_response"] = ""
				if mutation == "bash_agent" {
					event["agent_id"] = "other-child"
				}
				if mutation != "missing_bash_pre" {
					pre := map[string]any{}
					for k, v := range event {
						pre[k] = v
					}
					pre["hook_event_name"] = "PreToolUse"
					delete(pre, "tool_response")
					if mutation == "bash_call" {
						pre["tool_use_id"] = "exec-other"
					}
					raw, _ := json.Marshal(pre)
					if err := agentProbeWriteJSON(filepath.Join(dir, "events", "event-bash-pre.json"), agentProbeRecord{Raw: string(raw), Response: "{}"}); err != nil {
						t.Fatal(err)
					}
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
			case "metadata_fork":
				meta["forked_from_id"] = "unrelated"
			case "metadata_agent":
				meta["id"] = "unrelated"
			case "metadata_path":
				meta["agent_path"] = "/root/unrelated"
			}
			row, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": meta})
			child := append(row, '\n')
			child = append(child, []byte(`{"type":"response_item","payload":{"type":"custom_tool_call","name":"exec","call_id":"outer-child-call","input":"opaque JavaScript, not parsed"}}`+"\n")...)
			if mutation == "crlf_metadata" {
				child = bytes.Replace(child, []byte("\n"), []byte("\r\n"), 1)
			}
			if mutation == "duplicate_metadata" {
				child = append(child, append(row, '\n')...)
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

func TestAgentHookProbeFixtureObservedSchema(t *testing.T) {
	for _, scenario := range agentProbeScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "probe")
			if _, err := agentProbePrepare(dir, "/test/binary", scenario); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
			if err != nil {
				t.Fatal(err)
			}
			prompt := string(raw)
			if !strings.Contains(prompt, `fork_turns="none"`) || !strings.Contains(prompt, "if the actual schema exposes fork_turns") {
				t.Error("spawn inheritance must be conditional on the actual schema and use none")
			}
			if scenario.Name == "parallel-roots" && (!strings.Contains(prompt, "task_name worker_a") || !strings.Contains(prompt, "task_name worker_b")) {
				t.Error("parallel task names must be valid identifiers distinct from root paths")
			}
			if scenario.Name == "lifecycle" {
				for _, term := range []string{"followup_task", "send_message", "interrupt_agent", "list_agents", "Do not run the finite helper again", "Do not treat completion or interruption as close"} {
					if !strings.Contains(prompt, term) {
						t.Errorf("lifecycle instruction missing %q", term)
					}
				}
			}
		})
	}
}
