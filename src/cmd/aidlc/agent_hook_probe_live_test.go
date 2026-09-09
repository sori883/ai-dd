package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type agentProbeScenario struct{ Name, Mode, Request string }
type agentProbeFixture struct {
	Root string
	Args []string
}
type agentProbeProcessState struct {
	Nonce      string    `json:"nonce"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	ObservedAt time.Time `json:"observed_at"`
	EndedAt    time.Time `json:"ended_at"`
}

func agentProbeLiveReady(enabled, goos, arch, version string) bool {
	return enabled == "1" && goos == "darwin" && arch == "arm64" && strings.TrimSpace(version) == "codex-cli 0.153.4"
}

func agentProbeScenarios() []agentProbeScenario {
	spawn := "Call spawn_agent exactly once with agent_type probe_worker and task_name g0_probe if that field is required. Ask the child to run the finite process command below once and return. Wait for the actual child if created, then close_agent if available. Do not substitute other agents after denial."
	return []agentProbeScenario{
		{"deny", "deny", spawn},
		{"allow", "observe", spawn},
		{"parallel-roots", "observe", "Attempt two concurrent spawn_agent calls of probe_worker: use task_name worker_a for root worker-a and task_name worker_b for root worker-b. Assign roots using structured root/worktree parameters only if the actual tool schema provides them. Do not invent tool fields. If there is no structured root parameter, report that limitation and still attempt the two ordinary children, asking each to run the finite process command once. Wait and close both."},
		{"lifecycle", "observe", "Call spawn_agent for probe_worker with task_name lifecycle_worker if required. Ask it to run the finite helper asynchronously exactly once and return immediately. Wait for its reply, then inspect available agent-operation schemas by their documented meanings. Use list_agents or an actual status operation if available to record the current state. Attempt an additional message using send_message if exposed, noting that it may not start an idle child. Attempt an idle-to-running follow-up using followup_task if its schema starts an idle child; send_input or resume_agent may be used only if actually exposed and appropriate. Request only a short acknowledgement. Do not run the finite helper again or overwrite its nonce record. If a follow-up is running, attempt interrupt_agent or the actual interruption operation, recording the status and response. Do not infer interruption of running work when the child was already idle. Attempt close_agent only if a real close operation exists. Do not treat completion or interruption as close. Missing tool names alone do not prove the semantic operation is unavailable: inspect available schemas, then report operations not performed without substituting incompatible actions. Preserve every actual result. A reply, an interruption, or Stop does not prove process termination."},
		{"unit-and-alias", "observe", "Attempt two probe_worker children with distinct valid task names unit_worker and plain_worker, one described as Unit fixture-u1 and one without Unit. This is metadata only; do not call product Unit APIs. Attempt structured root assignments to worker-a and worker-alias (a symlink to worker-a), then worker-a from coordination-b. Do not infer root support from a prompt or chdir. If schema has no such field, say so. Children run the finite process command once. Wait and close each real child."},
		{"missing-response", "missing", spawn},
		{"nonzero-exit", "nonzero", spawn},
		{"save-failure", "save-failure", spawn},
		{"hook-timeout", "timeout", spawn},
	}
}

func agentProbeWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func agentProbePrepare(dir, binary string, scenario agentProbeScenario) (agentProbeFixture, error) {
	var fixture agentProbeFixture
	if !filepath.IsAbs(dir) || !filepath.IsAbs(binary) {
		return fixture, fmt.Errorf("absolute fixture and binary paths required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fixture, err
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fixture, err
	}
	dir = canonical
	root := filepath.Join(dir, "repo")
	for _, name := range []string{filepath.Join(root, ".codex", "agents"), filepath.Join(root, "worker-a"), filepath.Join(root, "worker-b"), filepath.Join(root, "coordination-b"), filepath.Join(dir, "processes"), filepath.Join(dir, "events")} {
		if err := os.MkdirAll(name, 0700); err != nil {
			return fixture, err
		}
	}
	if err := os.Symlink("worker-a", filepath.Join(root, "worker-alias")); err != nil {
		return fixture, err
	}
	var nonceBytes [16]byte
	if _, err := rand.Read(nonceBytes[:]); err != nil {
		return fixture, err
	}
	nonce := hex.EncodeToString(nonceBytes[:])
	processCommand := minimalProbeQuote(binary) + " -test.run='^TestAgentHookProbeProcess$' -- agent-process " + minimalProbeQuote(filepath.Join(dir, "processes")) + " " + nonce + " 15000"
	hookCommand := minimalProbeQuote(binary) + " -test.run='^TestAgentHookProbeHelper$' -- agent-hook " + minimalProbeQuote(filepath.Join(dir, "events")) + " " + minimalProbeQuote(scenario.Mode)
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "SubagentStart", "SubagentStop", "Stop"} {
		hooks[event] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": hookCommand, "timeout": 1}}}}
	}
	if err := agentProbeWriteJSON(filepath.Join(root, ".codex", "hooks.json"), map[string]any{"hooks": hooks}); err != nil {
		return fixture, err
	}
	config := "[features]\nhooks = true\n[agents]\nenabled = true\nmax_concurrent_threads_per_session = 4\n"
	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte(config), 0600); err != nil {
		return fixture, err
	}
	agent := `name = "probe_worker"
description = "Bounded G0 compatibility probe child."
model = "gpt-6-astra"
model_reasoning_effort = "xhigh"
sandbox_mode = "workspace-write"
developer_instructions = "Run only the provided finite probe command once. Do not spawn children, use network/MCP, read authentication files, or modify user settings. Keep all other activity within the temporary probe root. Follow the parent request about returning before or after process completion."
`
	if err := os.WriteFile(filepath.Join(root, ".codex", "agents", "probe-worker.toml"), []byte(agent), 0600); err != nil {
		return fixture, err
	}
	waitInstruction := "use exec_command with yield_time_ms=20000 so the finite helper finishes before the initial call returns"
	if scenario.Name == "lifecycle" {
		waitInstruction = "use exec_command with yield_time_ms=1 to return asynchronously while the finite helper is running"
	}
	prompt := "Perform only this bounded G0 Codex compatibility experiment in the temporary repository. Use actual provided tools, not simulated calls. Do not use MCP/network, read credentials or external files, modify settings, or create unbounded/background shell processes. Do not retry failed or denied requests. Do not kill any process. There is no product Unit API operation in this experiment. For each spawn, if the actual schema exposes fork_turns, set fork_turns=\"none\" and provide a self-contained child request. Use only lowercase letters, digits, and underscores in task names; task names are distinct from root paths. Do not reuse a task name in this experiment.\n" + scenario.Request + "\nFinite process command (" + waitInstruction + "):\n" + processCommand + "\nEach child must append a different lowercase alphanumeric suffix to the nonce argument if multiple children run. Do not change any other argument. The helper creates its own bounded observation record. End with a concise list of operations unavailable or not performed; prose is diagnostic only, not pass evidence."
	if err := os.WriteFile(filepath.Join(dir, "prompt.txt"), []byte(prompt), 0600); err != nil {
		return fixture, err
	}
	args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", "workspace-write", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="xhigh"`, "-c", minimalProbeTrustConfig(root), "--add-dir", dir, "-C", root, "--json", prompt}
	if err := agentProbeWriteJSON(filepath.Join(dir, "command.json"), append([]string{"codex"}, args...)); err != nil {
		return fixture, err
	}
	if err := agentProbeWriteJSON(filepath.Join(dir, "manifest.json"), map[string]any{"scenario": scenario, "nonce": nonce, "root": root, "model": "gpt-6-astra", "effort": "xhigh", "hook_trust_bypass": "temporary experiment only", "authentication": "inherited through normal CLI; no credential file reads/copies", "case_timeout_seconds": agentProbeCaseTimeout(scenario.Name).Seconds(), "process_command": processCommand, "process_maximum_ms": 20000, "hook_records": filepath.Join(dir, "events"), "process_records": filepath.Join(dir, "processes"), "transcripts": filepath.Join(dir, "transcript-SESSION.jsonl"), "operation_inventory": filepath.Join(dir, "calls.json"), "execution_result": filepath.Join(dir, "execution.json"), "summary": filepath.Join(dir, "summary.json")}); err != nil {
		return fixture, err
	}
	return agentProbeFixture{root, args}, nil
}

func TestAgentHookProbeProcess(t *testing.T) {
	split := -1
	for i, arg := range os.Args {
		if arg == "--" {
			split = i
			break
		}
	}
	if split < 0 {
		t.Skip("finite process subprocess only")
	}
	args := os.Args[split+1:]
	if len(args) != 4 || args[0] != "agent-process" {
		os.Exit(64)
	}
	dir, nonce := args[1], args[2]
	for _, r := range nonce {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			os.Exit(64)
		}
	}
	ms, err := strconv.Atoi(args[3])
	if err != nil || ms < 1 || ms > 20000 || nonce == "" || !filepath.IsAbs(dir) {
		os.Exit(64)
	}
	// Exclusive creation prevents two children reusing a nonce and overwriting evidence.
	path := filepath.Join(dir, "process-"+nonce+".json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(73)
	}
	if err := file.Close(); err != nil {
		os.Exit(74)
	}
	state := agentProbeProcessState{Nonce: nonce, PID: os.Getpid(), StartedAt: time.Now().UTC()}
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	for {
		state.ObservedAt = time.Now().UTC()
		_, stopErr := os.Stat(filepath.Join(dir, "stop-"+nonce))
		ended := !time.Now().Before(deadline) || stopErr == nil
		if ended {
			state.EndedAt = state.ObservedAt
		}
		tmp, err := os.CreateTemp(dir, "state-*.tmp")
		if err != nil {
			os.Exit(74)
		}
		data, err := json.Marshal(state)
		if err == nil {
			_, err = tmp.Write(data)
		}
		closeErr := tmp.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmp.Name(), path)
		}
		if err != nil {
			os.Exit(74)
		}
		if ended {
			os.Exit(0)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func agentProbeCleanup(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "process-*.json"))
	if err != nil {
		return err
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var state agentProbeProcessState
		if err := json.Unmarshal(data, &state); err != nil {
			return err
		}
		if filepath.Base(path) != "process-"+state.Nonce+".json" || strings.ContainsAny(state.Nonce, "/\\") {
			return fmt.Errorf("invalid process nonce")
		}
		if err := os.WriteFile(filepath.Join(dir, "stop-"+state.Nonce), nil, 0600); err != nil {
			return err
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		pending := false
		for _, path := range files {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var state agentProbeProcessState
			if err := json.Unmarshal(data, &state); err != nil {
				return err
			}
			if state.EndedAt.IsZero() {
				pending = true
			}
		}
		if !pending {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process terminal evidence missing; finite deadline is at most 20 seconds; records preserved")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Opt-in is the approved entry point. Ordinary tests never start a model.
func TestAgentHookProbeLive(t *testing.T) {
	if os.Getenv("AIDLC_AGENT_HOOK_LIVE") != "1" {
		t.Skip("set AIDLC_AGENT_HOOK_LIVE=1 for parent-authorized live")
	}
	versionCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	version, err := exec.CommandContext(versionCtx, "codex", "--version").CombinedOutput()
	cancel()
	if err != nil || !agentProbeLiveReady("1", runtime.GOOS, runtime.GOARCH, string(version)) {
		t.Fatalf("requires Codex 0.153.4 macOS arm64: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-agent-hook-probe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved G0 raw evidence: %s", evidence)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	caseEvidence := map[string]agentProbeEvidence{}
	for _, scenario := range agentProbeScenarios() {
		t.Run(scenario.Name, func(t *testing.T) {
			dir := filepath.Join(evidence, scenario.Name)
			fixture, err := agentProbePrepare(dir, binary, scenario)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), agentProbeCaseTimeout(scenario.Name))
			defer cancel()
			if err := agentProbeInitGit(ctx, fixture.Root); err != nil {
				t.Fatal(err)
			}
			stdout, err := os.Create(filepath.Join(dir, "stdout.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			stderr, err := os.Create(filepath.Join(dir, "stderr.txt"))
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close()
			command := exec.CommandContext(ctx, "codex", fixture.Args...)
			command.Stdout, command.Stderr = stdout, stderr
			command.WaitDelay = 3 * time.Second
			runErr := command.Run()
			if err := agentProbeWriteJSON(filepath.Join(dir, "execution.json"), map[string]any{"exit": agentProbeExitCode(command), "error": fmt.Sprint(runErr), "context_error": fmt.Sprint(ctx.Err()), "completed_at": time.Now().UTC()}); err != nil {
				t.Error(err)
			}
			captured, collectErr := agentProbeCollectEvidence(dir, runErr == nil)
			if collectErr != nil {
				t.Errorf("collect evidence: %v", collectErr)
			} else {
				caseEvidence[scenario.Name] = captured
			}
			cleanupErr := agentProbeCleanup(filepath.Join(dir, "processes"))
			if err := agentProbeWriteJSON(filepath.Join(dir, "cleanup.json"), map[string]any{"completed_at": time.Now().UTC(), "error": fmt.Sprint(cleanupErr), "mechanism": "nonce-specific stop files only; no PID kill; finite helper maximum 20 seconds"}); err != nil {
				t.Error(err)
			}
			if cleanupErr != nil {
				t.Errorf("cleanup: %v", cleanupErr)
			}
			// A failed runner is not a skipped or successful experiment. Unperformed model
			// operations remain visible as absent calls in summary.json and the transcript.
			if runErr != nil {
				t.Errorf("Codex experiment failed: %v (raw %s)", runErr, dir)
			}
			t.Logf("case raw and operation inventory: %s; gate conclusions require parent evidence review", dir)
		})
	}
	gates := agentProbeAggregate(caseEvidence["deny"], caseEvidence["allow"])
	if err := agentProbeWriteJSON(filepath.Join(evidence, "aggregate-summary.json"), map[string]any{"gates": gates, "deny_case": "deny", "allow_case": "allow", "assessment": "G0-1 uses captured independent controls; other gates require parent raw review"}); err != nil {
		t.Error(err)
	}
	t.Logf("aggregate G0-1=%s; %s", gates["G0-1"].Status, filepath.Join(evidence, "aggregate-summary.json"))
}

func agentProbeExitCode(command *exec.Cmd) int {
	if command.ProcessState == nil {
		return -1
	}
	return command.ProcessState.ExitCode()
}

func agentProbeCollect(dir string, complete bool) error {
	_, err := agentProbeCollectEvidence(dir, complete)
	return err
}

func agentProbeCollectEvidence(dir string, complete bool) (e agentProbeEvidence, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "events", "event-*.json"))
	if err != nil {
		return e, err
	}
	eventCounts, toolCounts := map[string]int{}, map[string]int{}
	var records []agentProbeRecord
	transcripts := map[string]string{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return e, err
		}
		var record agentProbeRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return e, err
		}
		records = append(records, record)
		var input agentProbeInput
		if err := json.Unmarshal([]byte(record.Raw), &input); err != nil {
			return e, err
		}
		eventCounts[input.Event]++
		if input.Tool != "" {
			toolCounts[input.Event+":"+input.Tool]++
		}
		var raw struct{ Session, Transcript string }
		var wire map[string]json.RawMessage
		if err := json.Unmarshal([]byte(record.Raw), &wire); err != nil {
			return e, err
		}
		_ = json.Unmarshal(wire["session_id"], &raw.Session)
		_ = json.Unmarshal(wire["transcript_path"], &raw.Transcript)
		// Read only paths delivered by this probe's own hooks, with a matching session
		// basename. Never discover global transcripts or inspect authentication data.
		if raw.Session != "" && filepath.IsAbs(raw.Transcript) && strings.Contains(filepath.Base(raw.Transcript), raw.Session) {
			transcripts[raw.Transcript] = raw.Session
		}
	}
	var calls []agentProbeCall
	for path, session := range transcripts {
		file, err := os.Open(path)
		if err != nil {
			return e, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 32<<20))
		closeErr := file.Close()
		if readErr != nil {
			return e, readErr
		}
		if closeErr != nil {
			return e, closeErr
		}
		if len(data) >= 32<<20 {
			return e, fmt.Errorf("probe transcript exceeds capture bound")
		}
		if strings.ContainsAny(session, "/\\") {
			return e, fmt.Errorf("invalid session identifier")
		}
		if err := os.WriteFile(filepath.Join(dir, "transcript-"+session+".jsonl"), data, 0600); err != nil {
			return e, err
		}
		parsed, err := agentProbeTranscriptCalls(data)
		if err != nil {
			return e, err
		}
		for i := range parsed {
			parsed[i].Session = session
		}
		calls = append(calls, parsed...)
	}
	children := map[string]agentProbeChildMetadata{}
	candidates := map[string]agentProbeChildCandidate{}
	invalidChildren := false
	for _, record := range records {
		var input agentProbeInput
		_ = json.Unmarshal([]byte(record.Raw), &input)
		childPath := ""
		if input.Event == "SubagentStart" {
			childPath = input.Transcript
		}
		if input.Event == "SubagentStop" {
			childPath = input.AgentTranscript
		}
		if childPath == "" || input.Agent == "" {
			continue
		}
		candidate := agentProbeChildCandidate{Path: childPath, Parent: input.Session, Agent: input.Agent}
		candidate.Task = agentProbeObservedTask(records, input.Session)
		if existing, ok := candidates[input.Agent]; ok && existing != candidate {
			invalidChildren = true
		}
		candidates[input.Agent] = candidate
	}
	if !invalidChildren {
		for agent, candidate := range candidates {
			data, meta, valid, readErr := agentProbeReadChild(candidate)
			if readErr != nil {
				return e, readErr
			}
			if !valid {
				invalidChildren = true
				continue
			}
			children[agent] = meta
			if err := os.WriteFile(filepath.Join(dir, "transcript-"+agent+".jsonl"), data, 0600); err != nil {
				return e, err
			}
			parsed, err := agentProbeTranscriptCalls(data)
			if err != nil {
				return e, err
			}
			for i := range parsed {
				parsed[i].Session = agent
			}
			calls = append(calls, parsed...)
		}
	}
	if err := agentProbeWriteJSON(filepath.Join(dir, "child-metadata.json"), map[string]any{"validated": children, "inconclusive": invalidChildren}); err != nil {
		return e, err
	}
	if err := agentProbeWriteJSON(filepath.Join(dir, "calls.json"), calls); err != nil {
		return e, err
	}
	processFiles, err := filepath.Glob(filepath.Join(dir, "processes", "process-*.json"))
	if err != nil {
		return e, err
	}
	processSnapshots := map[string]json.RawMessage{}
	for _, path := range processFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return e, err
		}
		if !json.Valid(data) {
			return e, fmt.Errorf("invalid process observation %s", path)
		}
		processSnapshots[filepath.Base(path)] = data
	}
	if err := agentProbeWriteJSON(filepath.Join(dir, "process-observations.json"), map[string]any{"captured_at": time.Now().UTC(), "before_cleanup": true, "processes": processSnapshots}); err != nil {
		return e, err
	}
	e = agentProbeEvidence{Complete: complete && !invalidChildren, Records: records, Calls: calls, Processes: processSnapshots, Children: children}
	manifest, readErr := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if readErr == nil {
		var m struct {
			Nonce   string `json:"nonce"`
			Command string `json:"process_command"`
		}
		if err := json.Unmarshal(manifest, &m); err != nil {
			return e, err
		}
		e.ExpectedNonce = m.Nonce
		e.ExpectedCommand = m.Command
	} else if !os.IsNotExist(readErr) {
		return e, readErr
	}
	summary := map[string]any{"capture_complete": complete, "hook_events": eventCounts, "hook_tools": toolCounts, "transcript_count": len(transcripts), "call_count": len(calls), "gates": agentProbeEvaluate(agentProbeEvidence{Complete: complete, Records: records, Calls: calls}), "assessment": "inconclusive defaults require parent raw review; absent operations are not unsupported; opaque code-mode calls are retained without invented inner schema"}
	return e, agentProbeWriteJSON(filepath.Join(dir, "summary.json"), summary)
}

func agentProbeTranscriptCalls(raw []byte) ([]agentProbeCall, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var calls []agentProbeCall
	for scanner.Scan() {
		var row struct {
			Type    string `json:"type"`
			Payload struct {
				Type, Name string
				ID         string          `json:"call_id"`
				Arguments  string          `json:"arguments"`
				Input      string          `json:"input"`
				Output     json.RawMessage `json:"output"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, err
		}
		if row.Type != "response_item" {
			continue
		}
		p := row.Payload
		switch p.Type {
		case "function_call", "custom_tool_call":
			input := p.Arguments
			if input == "" {
				input = p.Input
			}
			calls = append(calls, agentProbeCall{ID: p.ID, Name: p.Name, Input: input})
		case "function_call_output", "custom_tool_call_output":
			// Keep duplicate and orphan outputs, rather than silently choosing a match.
			matched := false
			for i := range calls {
				if calls[i].ID == p.ID && calls[i].Output == "" {
					calls[i].Output = string(p.Output)
					matched = true
					break
				}
			}
			if !matched {
				calls = append(calls, agentProbeCall{ID: p.ID, Output: string(p.Output)})
			}
		}
	}
	return calls, scanner.Err()
}

func agentProbeInitGit(ctx context.Context, root string) error {
	run := func(args ...string) error {
		options := []string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "-c", "user.name=G0 Fixture", "-c", "user.email=g0-fixture@example.invalid"}
		output, err := exec.CommandContext(ctx, "git", append(options, args...)...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("temporary Git fixture: %w: %s", err, output)
		}
		return nil
	}
	if err := run("init", "--quiet", root); err != nil {
		return err
	}
	if err := run("-C", root, "commit", "--quiet", "--allow-empty", "-m", "G0 temporary worktree fixture"); err != nil {
		return err
	}
	for _, name := range []string{"worker-a", "worker-b"} {
		path := filepath.Join(root, name)
		if err := os.Remove(path); err != nil {
			return err
		} // prepare created an empty directory only.
		if err := run("-C", root, "worktree", "add", "--quiet", "--detach", path, "HEAD"); err != nil {
			return err
		}
	}
	return run("init", "--quiet", filepath.Join(root, "coordination-b"))
}

func agentProbeCaseTimeout(name string) time.Duration {
	if name == "lifecycle" {
		return 5 * time.Minute
	}
	return 2 * time.Minute
}

func agentProbeAggregate(denied, allowed agentProbeEvidence) map[string]agentProbeResult {
	denied.Control = &allowed
	return agentProbeEvaluate(denied)
}

type agentProbeChildCandidate struct{ Path, Parent, Agent, Task string }

func agentProbeObservedTask(records []agentProbeRecord, parent string) string {
	task := ""
	count := 0
	for _, record := range records {
		var input agentProbeInput
		_ = json.Unmarshal([]byte(record.Raw), &input)
		if input.Event != "PostToolUse" || !agentProbeSpawnTool(input.Tool) || input.Session != parent {
			continue
		}
		var response struct {
			Task string `json:"task_name"`
		}
		if json.Unmarshal(agentProbeResultObject(string(input.Response)), &response) != nil {
			return ""
		}
		task = response.Task
		count++
	}
	if count != 1 {
		return ""
	}
	return task
}

func agentProbeReadChild(candidate agentProbeChildCandidate) ([]byte, agentProbeChildMetadata, bool, error) {
	var meta agentProbeChildMetadata
	if candidate.Parent == "" || candidate.Agent == "" || candidate.Task == "" || strings.ContainsAny(candidate.Agent, "/\\") || !filepath.IsAbs(candidate.Path) || !strings.Contains(filepath.Base(candidate.Path), candidate.Agent) {
		return nil, meta, false, nil
	}
	file, err := os.Open(candidate.Path)
	if err != nil {
		return nil, meta, false, err
	}
	defer file.Close()
	// Inspect only the bounded first metadata line before accepting any child
	// transcript body. A hook-provided path alone is not sufficient provenance.
	reader := bufio.NewReader(io.LimitReader(file, 32<<20))
	var first []byte
	for len(first) <= 1<<20 {
		part, readErr := reader.ReadSlice('\n')
		first = append(first, part...)
		if readErr == bufio.ErrBufferFull {
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return nil, meta, false, readErr
		}
		break
	}
	if len(first) > 1<<20 {
		return nil, meta, false, nil
	}
	var row struct {
		Type    string                  `json:"type"`
		Payload agentProbeChildMetadata `json:"payload"`
	}
	if json.Unmarshal(first, &row) != nil || row.Type != "session_meta" {
		return nil, meta, false, nil
	}
	meta = row.Payload
	if meta.ID != candidate.Agent || meta.Parent != candidate.Parent || meta.ForkedFrom != candidate.Parent || meta.AgentPath != candidate.Task {
		return nil, meta, false, nil
	}
	rest, err := io.ReadAll(reader)
	if err != nil {
		return nil, meta, false, err
	}
	data := append(first, rest...)
	if len(data) >= 32<<20 {
		return nil, meta, false, fmt.Errorf("child transcript exceeds capture bound")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(rest)))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		var next struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(scanner.Bytes(), &next) != nil {
			return nil, meta, false, nil
		}
		if next.Type == "session_meta" {
			return nil, meta, false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, meta, false, err
	}
	return data, meta, true, nil
}
