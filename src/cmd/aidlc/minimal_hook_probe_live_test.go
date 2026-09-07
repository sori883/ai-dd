package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMinimalHookProbeHelper is the hook executable, using the current Go test
// binary so the probe does not install a runtime or copy credentials.
func TestMinimalHookProbeHelper(t *testing.T) {
	split := -1
	for i, arg := range os.Args {
		if arg == "--" {
			split = i
			break
		}
	}
	if split < 0 {
		t.Skip("hook subprocess only")
	}
	args := os.Args[split+1:]
	if len(args) != 3 || args[0] != "minimal-hook" {
		t.Fatal("invalid hook helper arguments")
	}
	root, evidence := args[1], args[2]
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	var input minimalProbeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	observed := minimalProbeEvent{Input: input, Files: minimalProbeReadFiles(root)}
	// Keep the entire wire payload, including unknown fields, for compatibility diagnosis.
	record := struct {
		minimalProbeEvent
		Raw json.RawMessage `json:"raw"`
	}{observed, raw}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(evidence, fmt.Sprintf("event-%020d-*.json", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := `{}`
	if input.Event == "PreToolUse" && input.Input.Command == minimalProbeCommands[0] {
		output = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Expected probe denial. Continue with the next numbered probe; do not retry or create probe-forbidden another way."}}`
	}
	if input.Event == "Stop" {
		marker, err := os.OpenFile(filepath.Join(evidence, "stop-blocked"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			if err := marker.Close(); err != nil {
				t.Fatal(err)
			}
			output = `{"decision":"block","reason":"The one-time Stop probe is complete. Reply with a short final acknowledgement now. Do not call any tools or repeat any probe."}`
		} else if !os.IsExist(err) {
			t.Fatal(err)
		}
	}
	fmt.Fprintln(os.Stdout, output)
	os.Exit(0) // Suppress Go test PASS text: hook stdout must contain only JSON.
}

func TestMinimalHookProbeHelperProtocol(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, event, command, want string }{
		{"session", "SessionStart", "", ""},
		{"deny", "PreToolUse", minimalProbeCommands[0], "deny"},
		{"stop_first", "Stop", "", "block"},
		{"stop_second", "Stop", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := minimalProbeInput{Event: tc.event, Session: "test-session"}
			input.Input.Command = tc.command
			raw, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestMinimalHookProbeHelper$", "--", "minimal-hook", root, evidence)
			cmd.Stdin = bytes.NewReader(raw)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("helper: %v: %s", err, output)
			}
			var result struct {
				Decision string `json:"decision"`
				Hook     struct {
					Decision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("invalid hook stdout: %s", output)
			}
			if result.Decision+result.Hook.Decision != tc.want {
				t.Fatalf("decision: %s", output)
			}
		})
	}
	events, err := minimalProbeLoadEvents(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("saved %d events, want 4", len(events))
	}
}

// Deliberately opt-in without a build tag: this exact command is the approved
// preflight entry point. It never runs a model during ordinary Go tests.
func TestMinimalHookProbeLive(t *testing.T) {
	if os.Getenv("AIDLC_MINIMAL_HOOK_LIVE") != "1" {
		t.Skip("set AIDLC_MINIMAL_HOOK_LIVE=1 for the parent-authorized live preflight")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("live preflight requires macOS arm64")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	version, err := exec.CommandContext(ctx, "codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex version required: %s (%v)", version, err)
	}
	// This directory intentionally survives failures and t.TempDir cleanup.
	evidence, err := os.MkdirTemp("", "aidlc-minimal-hook-probe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved probe evidence: %s", evidence)
	root := filepath.Join(evidence, "repo")
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	root, err = minimalProbeCanonicalRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("canonical project root and trust key: %s", root)
	if output, err := exec.CommandContext(ctx, "git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hookCommand := minimalProbeQuote(binary) + " -test.run='^TestMinimalHookProbeHelper$' -- minimal-hook " + minimalProbeQuote(root) + " " + minimalProbeQuote(evidence)
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": hookCommand, "timeout": 10}}}
		if event == "PreToolUse" || event == "PostToolUse" {
			group["matcher"] = "^(Bash|apply_patch)$"
		}
		hooks[event] = []any{group}
	}
	config, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "hooks.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte("[features]\nhooks = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	prompt := `Perform this bounded hook compatibility probe in the current repository. Do not inspect other files or use subagents, MCP, or the network. Run each numbered operation exactly once and in order. Do not change the commands or retry expected denials/failures. Use exec_command for Bash operations, and apply_patch directly for the patch. Wait for each operation to finish before the next. For async operations use yield_time_ms=1, then write_stdin to poll the returned session_id until the process exits. Never substitute a shell sleep for write_stdin polling. The failure exit statuses are intentional.
1. exec_command cmd: ` + minimalProbeCommands[0] + `
2. exec_command cmd: ` + minimalProbeCommands[1] + `
3. exec_command cmd: ` + minimalProbeCommands[2] + `
4. exec_command cmd: ` + minimalProbeCommands[3] + ` (yield_time_ms=1; poll until terminal)
5. exec_command cmd: ` + minimalProbeCommands[4] + ` (yield_time_ms=1; poll until terminal)
6. apply_patch input exactly:
` + minimalProbeCommands[5] + `
Then give a short final response. The Stop hook will request one acknowledgement. Do not modify probe-forbidden by another method. Do not create any extra files.`
	if err := os.WriteFile(filepath.Join(evidence, "prompt.txt"), []byte(prompt), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", "workspace-write", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(root), "-C", root, "--json", prompt}
	command := exec.CommandContext(ctx, "codex", args...)
	command.WaitDelay = 5 * time.Second
	stdout, err := os.Create(filepath.Join(evidence, "stdout.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.Create(filepath.Join(evidence, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	command.Stdout, command.Stderr = stdout, stderr
	t.Log("model=gpt-6-astra effort=medium sandbox=workspace-write approval=never; HOME/CODEX_HOME inherited; user hooks may coexist (parent inspected), their output is not probe evidence")
	runErr := command.Run()
	events, err := minimalProbeLoadEvents(evidence)
	if err != nil {
		t.Fatal(err)
	}
	var transcript string
	for _, e := range events {
		if e.Input.Transcript != "" {
			transcript = e.Input.Transcript
		}
	}
	var calls []minimalProbeCall
	if transcript != "" {
		raw, err := os.ReadFile(transcript)
		if err != nil {
			t.Errorf("read probe transcript: %v", err)
		} else {
			if err := os.WriteFile(filepath.Join(evidence, "transcript.jsonl"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			calls, err = minimalProbeCalls(raw)
			if err != nil {
				t.Errorf("parse probe transcript: %v", err)
			}
		}
	}
	if runErr != nil {
		t.Errorf("Codex failed: %v; see %s", runErr, evidence)
	}
	if err := minimalProbeVerify(events, calls, minimalProbeReadFiles(root)); err != nil {
		t.Fatalf("hook compatibility unproven: %v; raw evidence: %s", err, evidence)
	}
}

func minimalProbeQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
func strconvQuote(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }
func minimalProbeReadFiles(root string) map[string]string {
	files := map[string]string{}
	for _, name := range minimalProbeFiles {
		if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			files[name] = string(data)
		} else if _, err := os.Lstat(filepath.Join(root, name)); err == nil {
			files[name] = "<not readable as a regular probe file>"
		}
	}
	return files
}
func minimalProbeLoadEvents(dir string) ([]minimalProbeEvent, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "event-*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var events []minimalProbeEvent
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var event minimalProbeEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// Transcript is diagnostic transport evidence, not a stable hook interface. If
// the fixed CLI uses another shape, missing calls fail verification explicitly.
func minimalProbeCalls(raw []byte) ([]minimalProbeCall, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var calls []minimalProbeCall
	positions := map[string]int{}
	completed := map[string]bool{}
	for scanner.Scan() {
		var row struct {
			Type    string `json:"type"`
			Payload struct {
				Type   string          `json:"type"`
				ID     string          `json:"call_id"`
				Name   string          `json:"name"`
				Input  string          `json:"input"`
				Output json.RawMessage `json:"output"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return nil, err
		}
		p := row.Payload
		if row.Type != "response_item" {
			continue
		}
		switch p.Type {
		case "custom_tool_call":
			if p.Name != "exec" || p.ID == "" {
				return nil, fmt.Errorf("unknown recorded tool %q", p.Name)
			}
			if _, ok := positions[p.ID]; ok {
				return nil, fmt.Errorf("duplicate call %s", p.ID)
			}
			call, err := minimalProbeLiteralCall(p.Input)
			if err != nil {
				return nil, err
			}
			call.ID = p.ID
			positions[p.ID] = len(calls)
			calls = append(calls, call)
		case "custom_tool_call_output":
			at, ok := positions[p.ID]
			if !ok || completed[p.ID] {
				return nil, fmt.Errorf("unmatched recorded output %s", p.ID)
			}
			var blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(p.Output, &blocks); err != nil {
				return nil, err
			}
			if len(blocks) != 2 || blocks[0].Type != "input_text" || blocks[1].Type != "input_text" {
				return nil, fmt.Errorf("unknown recorded output blocks")
			}
			result := blocks[1].Text
			blocked := calls[at].Name == "exec_command" && minimalProbeCommand(calls[at]) == minimalProbeCommands[0]
			if blocked {
				if !strings.HasPrefix(blocks[0].Text, "Script failed\n") || !strings.HasPrefix(result, "Script error:\nCommand blocked by PreToolUse hook:") {
					return nil, fmt.Errorf("missing recorded hook denial")
				}
			} else {
				if !strings.HasPrefix(blocks[0].Text, "Script completed\n") || !json.Valid([]byte(result)) {
					return nil, fmt.Errorf("unknown recorded result: %s", result)
				}
			}
			calls[at].Output = result
			completed[p.ID] = true
		case "function_call", "function_call_output":
			return nil, fmt.Errorf("unobserved function-call transport")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for _, call := range calls {
		if !completed[call.ID] {
			return nil, fmt.Errorf("missing output for %s", call.ID)
		}
	}
	return calls, nil
}

// Match only the literal wrappers observed in 0.153.4. This is deliberately
// not a JavaScript parser, and never evaluates transcript code.
func minimalProbeLiteralCall(input string) (minimalProbeCall, error) {
	input = strings.TrimSpace(input)
	for i, command := range minimalProbeCommands {
		literal := strconv.Quote(command)
		if i == 5 {
			if input == "text(await tools.apply_patch("+literal+"));" {
				return minimalProbeCall{Name: "apply_patch"}, nil
			}
			continue
		}
		for _, suffix := range []string{"", ",yield_time_ms:1"} {
			if input == "text(await tools.exec_command({cmd:"+literal+",shell:\"/bin/bash\""+suffix+"}));" {
				return minimalProbeCall{Name: "exec_command", Arguments: `{"cmd":` + literal + `}`}, nil
			}
		}
	}
	match := regexp.MustCompile(`^text\(await tools\.write_stdin\(\{session_id:([0-9]+),chars:"",yield_time_ms:1000\}\)\);$`).FindStringSubmatch(input)
	if len(match) == 2 {
		return minimalProbeCall{Name: "write_stdin", Arguments: `{"session_id":` + match[1] + `}`}, nil
	}
	return minimalProbeCall{}, fmt.Errorf("unknown recorded literal wrapper: %s", input)
}

func TestMinimalHookProbeCanonicalRoot(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "repo")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := minimalProbeCanonicalRoot(alias)
	if err != nil {
		t.Fatal(err)
	}
	if got != expected {
		t.Fatalf("root = %q, want canonical trust/workdir root %q", got, expected)
	}
}

func minimalProbeCanonicalRoot(root string) (string, error) { return filepath.EvalSymlinks(root) }

func TestMinimalHookProbeTrustConfig(t *testing.T) {
	root := "/private/tmp/probe repo"
	got := minimalProbeTrustConfig(root)
	want := `projects={"/private/tmp/probe repo"={trust_level="trusted"}}`
	if got != want {
		t.Fatalf("trust override = %q, want whole projects map %q", got, want)
	}
}

func minimalProbeTrustConfig(root string) string {
	return "projects={" + strconvQuote(root) + `={trust_level="trusted"}}`
}

func TestMinimalHookProbeObservedTransport(t *testing.T) {
	// Sanitized recording of the fixed CLI's code-mode wrapper and result blocks.
	const wire = `{"type":"response_item","payload":{"type":"custom_tool_call","call_id":"outer-start","name":"exec","input":"text(await tools.exec_command({cmd:\"sleep 3; printf async-success > probe-async-success; exit 0\",shell:\"/bin/bash\",yield_time_ms:1}));\n"}}
{"type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"outer-start","output":[{"type":"input_text","text":"Script completed\nWall time 0.4 seconds\nOutput:\n"},{"type":"input_text","text":"{\"session_id\":42,\"output\":\"\"}"}]}}
{"type":"response_item","payload":{"type":"custom_tool_call","call_id":"outer-poll","name":"exec","input":"text(await tools.write_stdin({session_id:42,chars:\"\",yield_time_ms:1000}));\n"}}
{"type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"outer-poll","output":[{"type":"input_text","text":"Script completed\nWall time 0.0 seconds\nOutput:\n"},{"type":"input_text","text":"{\"exit_code\":0,\"output\":\"\"}"}]}}`
	for _, tc := range []struct {
		name, wire string
		reject     bool
	}{
		{"observed", wire, false},
		{"unknown_wrapper", strings.Replace(wire, "text(await", "other(await", 1), true},
		{"missing_output", strings.Join(strings.Split(wire, "\n")[:3], "\n"), true},
		{"wrong_output_id", strings.Replace(wire, `"call_id":"outer-poll","output"`, `"call_id":"unknown","output"`, 1), true},
		{"non_json_result", strings.Replace(wire, `{\"exit_code\":0,\"output\":\"\"}`, `not JSON`, 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := minimalProbeCalls([]byte(tc.wire))
			if tc.reject {
				if err == nil {
					t.Fatalf("accepted invalid recorded transport: %+v", calls)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 2 {
				t.Fatalf("calls = %d, want 2", len(calls))
			}
			if calls[0].ID != "outer-start" || calls[0].Name != "exec_command" || minimalProbeCommand(calls[0]) != minimalProbeCommands[3] || minimalProbeRunning(calls[0].Output) != "42" {
				t.Fatalf("start = %+v", calls[0])
			}
			if calls[1].Name != "write_stdin" || calls[1].Arguments != `{"session_id":42}` || !minimalProbeExited(calls[1].Output, 0) {
				t.Fatalf("poll = %+v", calls[1])
			}
		})
	}
}

func TestMinimalHookProbeReplay(t *testing.T) {
	evidence := os.Getenv("AIDLC_MINIMAL_HOOK_EVIDENCE")
	if evidence == "" {
		t.Skip("set AIDLC_MINIMAL_HOOK_EVIDENCE to replay a preserved live recording")
	}
	events, err := minimalProbeLoadEvents(evidence)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(evidence, "transcript.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	calls, err := minimalProbeCalls(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := minimalProbeVerify(events, calls, minimalProbeReadFiles(filepath.Join(evidence, "repo"))); err != nil {
		t.Fatal(err)
	}
	t.Logf("verified preserved recording: %d hook events, %d transport calls; no model invocation", len(events), len(calls))
}
