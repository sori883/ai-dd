//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/minimal"
)

// TestAssignmentLiveHook is an isolated recorder around the real installed CLI.
func TestAssignmentLiveHook(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "assignment-live-hook" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	args := os.Args[marker+1:]
	if len(args) != 4 {
		os.Exit(64)
	}
	root, binary, records, mode := args[0], args[1], args[2], args[3]
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1024*1024))
	if err != nil {
		os.Exit(74)
	}
	var input minimal.HookInput
	_ = json.Unmarshal(raw, &input)
	skip := mode == "missing-post" && input.Event == "PostToolUse" && (input.Tool == "spawn_agent" || input.Tool == "collaborationspawn_agent")
	fault := mode == "save-failure" && input.Event == "PreToolUse" && (input.Tool == "spawn_agent" || input.Tool == "collaborationspawn_agent")
	if fault {
		if err := os.Chmod(filepath.Join(root, "aidlc/.runtime/assignments"), 0500); err != nil {
			os.Exit(74)
		}
	}
	var out, errout bytes.Buffer
	code := 0
	observeOnly := input.Event == "SubagentStart" || input.Event == "SubagentStop"
	if !skip && !observeOnly {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		cmd := exec.CommandContext(ctx, binary, "__minimal-hook", "--project-dir", root)
		cmd.Dir = root
		cmd.Stdin = bytes.NewReader(raw)
		cmd.Stdout = &out
		cmd.Stderr = &errout
		err = cmd.Run()
		cancel()
		if err != nil {
			code = 1
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			}
		}
	}
	if fault {
		if err := os.Chmod(filepath.Join(root, "aidlc/.runtime/assignments"), 0700); err != nil {
			os.Exit(74)
		}
	}
	file, err := os.CreateTemp(records, "hook-*.json")
	if err != nil {
		os.Exit(74)
	}
	record := map[string]any{"input": json.RawMessage(raw), "stdout": out.String(), "stderr": errout.String(), "exit_code": code, "post_deliberately_omitted": skip, "product_invoked": !skip && !observeOnly, "save_failure_injected": fault, "observed_at": time.Now().UTC()}
	if err := json.NewEncoder(file).Encode(record); err != nil {
		file.Close()
		os.Exit(74)
	}
	if file.Close() != nil {
		os.Exit(74)
	}
	fmt.Print(out.String())
	fmt.Fprint(os.Stderr, errout.String())
	os.Exit(code)
}

// TestAssignmentLive records actual native calls; model success alone is not gate evidence.
func TestAssignmentLive(t *testing.T) {
	if os.Getenv("AIDLC_ASSIGNMENT_LIVE") != "1" {
		t.Skip("set AIDLC_ASSIGNMENT_LIVE=1 for fixed isolated native-agent measurement")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("requires macOS arm64")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	version, err := exec.CommandContext(ctx, "codex", "--version").CombinedOutput()
	cancel()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex required: %s %v", version, err)
	}
	base, err := os.MkdirTemp("", "aidlc-assignment-live-")
	if err != nil {
		t.Fatal(err)
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent raw evidence: %s", base)
	binary := buildMinimalBinary(t)
	overall, stop := context.WithTimeout(t.Context(), 25*time.Minute)
	defer stop()
	for _, mode := range []string{"allow-parallel", "deny", "missing-post", "save-failure"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(base, mode)
			root := filepath.Join(dir, "repo")
			events := filepath.Join(dir, "events")
			processes := filepath.Join(dir, "processes")
			for _, p := range []string{root, events, processes} {
				if err := os.MkdirAll(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			f := operationsFixture{t, binary, root}
			f.git("init", "-q")
			f.git("-c", "user.name=Assignment", "-c", "user.email=assignment@example.invalid", "commit", "--allow-empty", "-qm", "base")
			f.ok("install", "codex", "--project-dir", root)
			st := f.tdd()
			var reg assignment.Registry
			if err := json.Unmarshal(f.ok("assignment", "init", "--file", f.request(assignment.InitRequest{RequestID: "fixture-init", HumanConfirmed: true, Reason: "user-authorized isolated fixture has no existing workers"})), &reg); err != nil {
				t.Fatal(err)
			}
			workers := []string{}
			for _, name := range []string{"worker_a", "worker_b"} {
				p := filepath.Join(dir, name)
				f.git("worktree", "add", "--detach", p, st.Config.CodeRevision)
				workers = append(workers, p)
			}
			requestPaths := []string{}
			for i, p := range workers {
				name := filepath.Join(root, "aidlc/.runtime", fmt.Sprintf("reserve-%d.json", i))
				if err := agentProbeWriteJSON(name, flow.AssignmentRequest{RegistryEpoch: reg.Epoch, RequestID: fmt.Sprintf("reserve-%d", i), StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: p, Session: fmt.Sprintf("worker-%d", i)}); err != nil {
					t.Fatal(err)
				}
				requestPaths = append(requestPaths, name)
			}
			hookCommand := minimalProbeQuote(os.Args[0]) + " -test.run='^TestAssignmentLiveHook$' -- assignment-live-hook " + minimalProbeQuote(root) + " " + minimalProbeQuote(binary) + " " + minimalProbeQuote(events) + " " + minimalProbeQuote(mode)
			hooks := map[string]any{}
			for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "SubagentStart", "SubagentStop", "Stop"} {
				hooks[event] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": hookCommand, "timeout": 10, "additionalContextLimit": 8192}}}}
			}
			if err := agentProbeWriteJSON(filepath.Join(root, ".codex/hooks.json"), map[string]any{"hooks": hooks}); err != nil {
				t.Fatal(err)
			}
			config := "[features]\nhooks = true\n[agents]\nenabled = true\nmax_concurrent_threads_per_session = 4\n"
			if err := os.WriteFile(filepath.Join(root, ".codex/config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			// The deployed worker remains the product role; fixture model/deadline settings are explicit.
			workerFile := filepath.Join(root, ".codex/agents/aidlc-worker.toml")
			raw, err := os.ReadFile(workerFile)
			if err != nil {
				t.Fatal(err)
			}
			raw = bytes.Replace(raw, []byte("sandbox_mode ="), []byte("model = \"gpt-6-astra\"\nmodel_reasoning_effort = \"xhigh\"\nsandbox_mode ="), 1)
			if err := os.WriteFile(workerFile, raw, 0600); err != nil {
				t.Fatal(err)
			}
			// Hook-file replacement affects the initialization input snapshot, so keep the TDD inputs unchanged.
			processCommand := minimalProbeQuote(os.Args[0]) + " -test.run='^TestAgentHookProbeProcess$' -- agent-process " + minimalProbeQuote(processes) + " NONCE 15000"
			if mode == "allow-parallel" {
				processCommand = minimalProbeQuote(os.Args[0]) + " -test.run='^TestAssignmentProcess$' -- assignment-process " + minimalProbeQuote(processes) + " NONCE PEER 90000 15000"
			}
			prompt := fmt.Sprintf(`This is an authorized isolated product observation. A=%s. Intent=%s revision=%d step=%s. Use the actual SessionStart session ID as MAIN. First bind this Intent to MAIN, read the full Rules and intent procedure. The host initialized registry epoch %s after fixture confirmation. Do not change Rules, state, registry, hooks or configs directly. CLI must never launch children.
For allow-parallel, read the first prepared reserve JSON %s and create a new draft for unit claim with its registry_epoch/request_id/step_id/root/session plus unit="a" and coordinator_session=actual MAIN, omitting agent. Run A unit claim ID --space default --expect %d --file CLAIM_DRAFT. Then use the second prepared reserve JSON %s for A assignment reserve with actual MAIN and the NEW current Intent revision from intent show. Read assignment list/show to obtain both exact task_name values, then directly call native spawn_agent twice with agent_type aidlc-worker. Set fork_turns=none if available. Tell each child its registered root, Rule text, and ask it ONLY to run the finite test helper once from that root: %s with yield_time_ms=1000. Use distinct nonce childa and childb, with PEER set to the other nonce. Spawn both children promptly before waiting for either completion. The allow-parallel helper records readiness, waits at most 90 seconds for its peer, then performs 15 seconds of work. Each child must poll its own returned process session to terminal and report the actual exit; never rerun or overwrite a nonce. Children must not bind/switch/update the parent session, reread Rules by parent session bind, mutate product state, or delegate; the provided Rules and task are their input. Do not ask children to implement code or rerun helpers. Wait for actual completion. Before additional requests, use assignment check. Use actual followup_task or send_message targeting the returned relative task_name for a short acknowledgement only; if canonical paths are available, also attempt an acknowledgement by that exact path. No helper reruns. After the first child finishes, submit unit result for unit a using its exact assigned session/root/run_id, current step_id, and that root's unchanged HEAD (this finite-process fixture intentionally does not edit source). Verify status reported. Attempt competing Unitless reservation with a fresh request_id of that reported Unit's occupied root and record rejection. Read assignment show, then release each with current entry_revision and explicit no-more-requests/stopped/collected confirmations only after the helpers ended. Finally reserve one root with a new request_id. Preserve raw tool results and report unperformed operations.
Current case is %s. For deny, attempt exactly one spawn_agent with agent_type aidlc-worker and task_name unregistered_worker without a reservation; do not retry or substitute after denial. For missing-post, use the first original reserve JSON directly with assignment reserve (no Unit claim) and spawn once; the fixture intentionally omits the product spawn Post. Attempt followup by the relative task_name and observe rejection; keep the uncertain reservation, do not release or respawn. Wait for actual child completion through available status/wait operations before ending; do not treat the omitted Post as a stop. For save-failure, use the first original reserve JSON directly with assignment reserve (no Unit claim), then spawn once: the recorder temporarily makes the registry directory read-only during the product Pre save. Record the actual hook/tool response; do not retry or release the retained reservation. Report observations without claiming actual-root or full-process enforcement.`, minimalProbeQuote(binary), st.ID, st.Revision, st.CurrentStepID, reg.Epoch, requestPaths[0], st.Revision, requestPaths[1], processCommand, mode)
			args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", "workspace-write", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="xhigh"`, "-c", minimalProbeTrustConfig(root), "--add-dir", dir, "-C", root, "--json", prompt}
			budget := 5 * time.Minute
			if mode == "allow-parallel" {
				budget = 7 * time.Minute
			}
			caseCtx, caseCancel := context.WithTimeout(overall, budget)
			defer caseCancel()
			cmd := exec.CommandContext(caseCtx, "codex", args...)
			cmd.Dir = root
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()
			code := 0
			if runErr != nil {
				code = -1
				if exit, ok := runErr.(*exec.ExitError); ok {
					code = exit.ExitCode()
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), stdout.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "stderr.txt"), stderr.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			manifest := map[string]any{"case": mode, "model_exit": code, "deadline_reached": caseCtx.Err() != nil, "case_limit_seconds": budget.Seconds(), "hooks": events, "processes": processes, "transcript": filepath.Join(dir, "transcript.jsonl"), "root": root, "workers": workers, "initial_epoch": reg.Epoch, "intent_id": st.ID, "automatic_gate": "inconclusive: inspect actual hook and native tool results", "parallel_measurement": "allow-parallel: childa/childb .ready and .process.json; compare actual WorkStartedAt/EndedAt, not readiness alone; 90s wait + 15s work; Cwd is helper observation, not hook identity"}
			if err := agentProbeWriteJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
				t.Fatal(err)
			}
			if runErr != nil {
				t.Errorf("model exited %d; retained raw: %s", code, dir)
			}
		})
	}
}
