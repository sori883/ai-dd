package main

import (
	"bytes"
	"context"
	"encoding/json"
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
		e := fixture()
		e.Control = &agentProbeEvidence{Complete: true, MarkerAgent: "child", MarkerNonce: "control-nonce", MarkerCommand: "probe-helper control-nonce", MarkerEvent: agentProbeRecord{Raw: `{"hook_event_name":"PostToolUse","session_id":"child","tool_use_id":"marker-call","tool_name":"Bash","tool_input":{"command":"probe-helper control-nonce"},"tool_response":{"exit_code":0}}`}, Records: []agentProbeRecord{
			{Raw: `{"hook_event_name":"PreToolUse","session_id":"q","tool_use_id":"c2","tool_name":"spawn_agent","tool_input":{"agent_type":"probe_worker"}}`, Response: `{}`},
			{Raw: `{"hook_event_name":"SubagentStart","agent_id":"child"}`},
		}, Calls: []agentProbeCall{{ID: "c2", Name: "spawn_agent", Input: `{"agent_type":"probe_worker"}`, Output: `{"agent_id":"child"}`}}}
		if got := agentProbeEvaluate(e)["G0-1"]; got.Status != "pass" {
			t.Fatalf("deny plus observed control=%+v", got)
		}
		// A marker claim without its child tool invocation is not provenance.
		saved := e.Control.MarkerEvent
		e.Control.MarkerEvent = agentProbeRecord{}
		if got := agentProbeEvaluate(e)["G0-1"]; got.Status == "pass" {
			t.Errorf("unattributed marker passed: %+v", got)
		}
		e.Control.MarkerEvent = saved
		// An out-of-order duplicate is evidence ambiguity, never a second child.
		e.Control.Records = append(e.Control.Records, e.Control.Records[1])
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
