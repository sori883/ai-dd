//go:build integration && diagnostic

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/install"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type reliabilitySnapshot struct {
	Data    []byte
	Missing bool
	Error   string
}

type reliabilityRecord struct {
	Raw, Stdout, Stderr []byte
	Exit                int
	Before, After       reliabilitySnapshot
	Started, Finished   time.Time
	Error               string
}

func reliabilityCapture(ctx context.Context, binary, root string, raw []byte) reliabilityRecord {
	r := reliabilityRecord{Raw: raw, Exit: -1, Started: time.Now()}
	var input hookProbeInput
	_ = json.Unmarshal(raw, &input)
	r.Before = reliabilityReadSession(root, input.Session)
	cmd := observerHookCommand(ctx, binary, root)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(raw)
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	err := cmd.Run()
	r.Stdout, r.Stderr = out.Bytes(), errout.Bytes()
	if cmd.ProcessState != nil {
		r.Exit = cmd.ProcessState.ExitCode()
	}
	if err != nil && (cmd.ProcessState == nil || ctx.Err() != nil) {
		r.Error = err.Error()
	}
	r.After = reliabilityReadSession(root, input.Session)
	r.Finished = time.Now()
	return r
}

func reliabilityReadSession(root, session string) reliabilitySnapshot {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`).MatchString(session) {
		return reliabilitySnapshot{Error: "invalid session ID"}
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return reliabilitySnapshot{Error: err.Error()}
	}
	defer dir.Close()
	data, err := dir.ReadFile("aidlc/.runtime/flow/sessions/" + session + ".txt")
	if os.IsNotExist(err) {
		return reliabilitySnapshot{Missing: true}
	}
	if err != nil {
		return reliabilitySnapshot{Error: err.Error()}
	}
	return reliabilitySnapshot{Data: data}
}

func TestHookReliabilityProbeHelper(t *testing.T) {
	if os.Getenv("AIDLC_RELIABILITY_HELPER") != "1" {
		t.Skip("subprocess only")
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 4 {
		os.Exit(125)
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(125)
	}
	r := reliabilityCapture(t.Context(), args[1], args[2], raw)
	data, err := json.Marshal(r)
	if err == nil {
		_ = os.WriteFile(args[3], data, 0600)
	}
	// Observation failure cannot replace the product's wire result. The
	// collector treats a missing record as incomplete evidence.
	_, _ = os.Stdout.Write(r.Stdout)
	_, _ = os.Stderr.Write(r.Stderr)
	if r.Exit < 0 {
		os.Exit(125)
	}
	os.Exit(r.Exit)
}

func TestHookReliabilityProbeProtocol(t *testing.T) {
	binary := buildAIDLCBinary(t)
	for _, tc := range []struct {
		name, raw, before, after string
		lock                     bool
	}{
		{"terminal", ` {"session_id":"session","turn_id":"turn","tool_use_id":"tool","tool_name":"Bash","hook_event_name":"PostToolUse","unknown":{"preserve":true}} `, "tool", "", false},
		{"invalid_wire", "not json\n", "tool", "tool", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(root, "aidlc/.runtime/flow/sessions/session.txt")
			if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
				t.Fatal(err)
			}
			before := []byte("space=\nintent=\nturn=turn\ntool=" + tc.before + "\nrule_turn=\nrule_hash=\n")
			after := []byte("space=\nintent=\nturn=turn\ntool=" + tc.after + "\nrule_turn=\nrule_hash=\n")
			if err := os.WriteFile(name, before, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.lock {
				if err := os.MkdirAll(filepath.Join(root, "aidlc/.runtime/locks/session-session"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// The independent product invocation supplies the expected wire result.
			cmd := observerHookCommand(t.Context(), binary, root)
			cmd.Stdin = bytes.NewBufferString(tc.raw)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			_ = cmd.Run()
			if cmd.ProcessState == nil {
				t.Fatal("product did not execute")
			}
			wantExit := cmd.ProcessState.ExitCode()
			if err := os.WriteFile(name, before, 0600); err != nil {
				t.Fatal(err)
			}
			got := reliabilityCapture(t.Context(), binary, root, []byte(tc.raw))
			if !bytes.Equal(got.Raw, []byte(tc.raw)) || !bytes.Equal(got.Stdout, stdout.Bytes()) || !bytes.Equal(got.Stderr, stderr.Bytes()) || got.Exit != wantExit {
				t.Fatalf("wrapper changed raw input or product stdout/stderr/exit: got %+v, expected %q/%q/%d", got, stdout.Bytes(), stderr.Bytes(), wantExit)
			}
			if tc.name != "invalid_wire" && (!bytes.Equal(got.Before.Data, before) || !bytes.Equal(got.After.Data, after) || got.Before.Error != "" || got.After.Error != "") {
				t.Fatalf("snapshot mismatch: %+v", got)
			}
			if got.Started.IsZero() || got.Finished.Before(got.Started) {
				t.Fatal("missing ordered capture times")
			}
			if err := os.WriteFile(name, before, 0600); err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			evidence := filepath.Join(t.TempDir(), "record.json")
			wrapped := exec.CommandContext(t.Context(), helper, "-test.run=^TestHookReliabilityProbeHelper$", "--", binary, root, evidence)
			wrapped.Env = append(os.Environ(), "AIDLC_RELIABILITY_HELPER=1")
			wrapped.Stdin = bytes.NewBufferString(tc.raw)
			var wrappedOut, wrappedErr bytes.Buffer
			wrapped.Stdout, wrapped.Stderr = &wrappedOut, &wrappedErr
			_ = wrapped.Run()
			if wrapped.ProcessState == nil || wrapped.ProcessState.ExitCode() != wantExit || !bytes.Equal(wrappedOut.Bytes(), stdout.Bytes()) || !bytes.Equal(wrappedErr.Bytes(), stderr.Bytes()) {
				t.Fatalf("helper altered product result: %q/%q", wrappedOut.Bytes(), wrappedErr.Bytes())
			}
			data, err := os.ReadFile(evidence)
			if err != nil {
				t.Fatal(err)
			}
			var saved reliabilityRecord
			if json.Unmarshal(data, &saved) != nil || !bytes.Equal(saved.Raw, []byte(tc.raw)) || saved.Exit != wantExit {
				t.Fatal("helper lost record")
			}
			var input hookProbeInput
			if tc.name != "invalid_wire" && (json.Unmarshal(got.Raw, &input) != nil || input.Session != "session" || input.Turn != "turn" || input.ID != "tool") {
				t.Fatal("lost correlation IDs")
			}
			if err := os.MkdirAll(filepath.Join(root, ".codex"), 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := install.Codex(root, binary); err != nil {
				t.Fatal(err)
			}
			hookFile := filepath.Join(root, ".codex/hooks.json")
			installed, err := os.ReadFile(hookFile)
			if err != nil {
				t.Fatal(err)
			}
			var registration map[string]any
			if err := json.Unmarshal(installed, &registration); err != nil {
				t.Fatal(err)
			}
			groups := registration["hooks"].(map[string]any)["PreToolUse"].([]any)
			group := groups[0].(map[string]any)
			handlers := group["hooks"].([]any)
			handlers[0].(map[string]any)["timeout"] = 10
			group["hooks"] = append(handlers, map[string]any{"type": "command", "command": "echo user-hook", "timeout": 4})
			encoded, err := json.Marshal(registration)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(hookFile, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			writeAIDLCFixture(t, filepath.Join(root, ".codex/config.toml"), "# existing fixture trust configuration\n")
			trustBefore := operationsRead(t, filepath.Join(root, ".codex/config.toml"))
			original, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if err != nil {
				t.Fatal(err)
			}
			assetDir := t.TempDir()
			if err := reliabilityPrepare(root, binary, helper, assetDir); err != nil {
				t.Fatal(err)
			}
			candidate := operationsRead(t, filepath.Join(assetDir, "hooks.candidate.json"))
			if !bytes.Contains(candidate, []byte("echo user-hook")) || !bytes.Contains(candidate, []byte(`"timeout": 10`)) || !bytes.Equal(trustBefore, operationsRead(t, filepath.Join(root, ".codex/config.toml"))) {
				t.Fatal("prepare changed user hook/timeout/trust")
			}
			unchanged, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if err != nil || !bytes.Equal(unchanged, original) {
				t.Fatal("prepare altered registration")
			}
			wrapper, err := os.ReadFile(filepath.Join(assetDir, "wrapper.sh"))
			if err != nil {
				t.Fatal(err)
			}
			standalone := filepath.Join(t.TempDir(), "wrapper.sh")
			if err := os.WriteFile(standalone, wrapper, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(assetDir); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name, before, 0600); err != nil {
				t.Fatal(err)
			}
			failedCapture := exec.CommandContext(t.Context(), "sh", standalone)
			failedCapture.Stdin = bytes.NewBufferString(tc.raw)
			var fallbackOut, fallbackErr bytes.Buffer
			failedCapture.Stdout, failedCapture.Stderr = &fallbackOut, &fallbackErr
			_ = failedCapture.Run()
			if failedCapture.ProcessState == nil || failedCapture.ProcessState.ExitCode() != wantExit || !bytes.Equal(fallbackOut.Bytes(), stdout.Bytes()) || !bytes.Equal(fallbackErr.Bytes(), stderr.Bytes()) {
				t.Fatalf("evidence failure changed product output/exit: %q/%q", fallbackOut.Bytes(), fallbackErr.Bytes())
			}
		})
	}
}

type reliabilityResult struct{ Observation, Product, Reason string }

func reliabilityEvaluate(records []reliabilityRecord, terminal []byte) reliabilityResult {
	unknown := func(reason string) reliabilityResult { return reliabilityResult{"incomplete", "unknown", reason} }
	var event struct {
		Type    string `json:"type"`
		Payload struct {
			Type      string `json:"type"`
			Session   string `json:"thread_id"`
			Turn      string `json:"turn_id"`
			Completed int64  `json:"completed_at_ms"`
			Item      struct {
				Type   string `json:"type"`
				ID     string `json:"id"`
				Status string `json:"status"`
				Exit   *int   `json:"exit_code"`
			} `json:"item"`
		} `json:"payload"`
	}
	if json.Unmarshal(terminal, &event) != nil || event.Type != "event_msg" {
		return unknown("missing terminal event")
	}
	e := event.Payload
	if e.Type != "item_completed" || e.Item.Type != "CommandExecution" || e.Item.Exit == nil || e.Completed <= 0 || e.Session == "" || e.Turn == "" || e.Item.ID == "" {
		return unknown("unsupported or incomplete terminal evidence")
	}
	if (*e.Item.Exit == 0 && e.Item.Status != "completed") || (*e.Item.Exit != 0 && e.Item.Status != "failed") {
		return unknown("terminal status and exit code disagree")
	}
	if len(records) < 2 {
		return unknown("missing Pre or Post")
	}
	last := time.Time{}
	var previous []byte
	failure := ""
	for i, r := range records {
		var input hookProbeInput
		if json.Unmarshal(r.Raw, &input) != nil || input.Session != e.Session || input.Turn != e.Turn || input.ID != e.Item.ID || input.Tool != "Bash" {
			return unknown("hook/terminal IDs differ")
		}
		wantEvent := "PostToolUse"
		if i == 0 {
			wantEvent = "PreToolUse"
		}
		if input.Event != wantEvent || r.Started.IsZero() || r.Finished.Before(r.Started) || r.Started.Before(last) || r.Error != "" {
			return unknown("invalid capture ordering or execution")
		}
		last = r.Finished
		before, ok := reliabilitySessionTool(r.Before)
		if !ok {
			return unknown("missing or invalid before snapshot")
		}
		after, ok := reliabilitySessionTool(r.After)
		if !ok {
			return unknown("missing or invalid after snapshot")
		}
		var out map[string]json.RawMessage
		if json.Unmarshal(r.Stdout, &out) != nil && r.Exit == 0 {
			return unknown("invalid hook output")
		}
		if i == 0 {
			if r.Exit != 0 || len(out) != 0 || before != "" || after != e.Item.ID || r.Finished.UnixMilli() > e.Completed {
				return unknown("Pre did not record execution")
			}
		} else {
			if r.Started.UnixMilli() < e.Completed {
				return unknown("Post precedes actual terminal")
			}
			if !bytes.Equal(previous, r.Before.Data) {
				return unknown("intervening session change")
			}
			if r.Exit != 0 || len(out) != 0 || after != "" {
				failure = "terminal notification failed or Tool retained"
			}
			if i > 1 && !bytes.Equal(r.Before.Data, r.After.Data) {
				failure = "duplicate Post changed session"
			}
		}
		previous = r.After.Data
	}
	if failure != "" {
		return reliabilityResult{"complete", "unrepaired", failure}
	}
	return reliabilityResult{"complete", "pass", "matching terminal cleared Tool; duplicate Posts were idempotent"}
}

func reliabilitySessionTool(snapshot reliabilitySnapshot) (string, bool) {
	if snapshot.Missing || snapshot.Error != "" {
		return "", false
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(snapshot.Data), "\n"), "\n") {
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return "", false
		}
		if _, exists := values[key]; exists {
			return "", false
		}
		value, err := url.QueryUnescape(raw)
		if err != nil {
			return "", false
		}
		values[key] = value
	}
	if len(values) != 6 {
		return "", false
	}
	for _, key := range []string{"space", "intent", "turn", "tool", "rule_turn", "rule_hash"} {
		if _, ok := values[key]; !ok {
			return "", false
		}
	}
	return values["tool"], true
}

func reliabilityChildRequest(parent, role string) (string, error) {
	if !regexp.MustCompile(`^/root(/[a-z][a-z0-9_]*)*$`).MatchString(parent) || len(parent) > 512 || role != "aidlc-reviewer" && role != "aidlc-stage-planner" {
		return "", fmt.Errorf("explicit canonical parent and eligible read-only role required")
	}
	return fmt.Sprintf("After checking current-stage eligibility, use native spawn_agent to create exactly one %s child named hook_reliability_report. The known parent target is %s. Have the child generate its own random nonce through a read-only command; keep this child-generated nonce out of the initial parent request. Ask the child first to attempt one send_message to %s/unregistered_sibling as a denial control, then send an intermediate report with the same nonce to %s. A denied control is expected and must not trigger respawn or recovery. Keep the child alive long enough for the parent to receive the intermediate message. Preserve the structured parent notification and corresponding hook input and response. A final answer or claimed receipt alone is not evidence; report unavailable structured evidence explicitly. Never write shared state or approvals to perform the report.", role, parent, parent, parent), nil
}

func reliabilityChildReceipt(raw []byte, parent, child, nonce string) bool {
	var row struct {
		Type    string `json:"type"`
		Payload struct {
			Type      string `json:"type"`
			Author    string `json:"author"`
			Recipient string `json:"recipient"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"payload"`
	}
	if json.Unmarshal(raw, &row) != nil || row.Type != "response_item" || row.Payload.Type != "agent_message" || row.Payload.Author != child || row.Payload.Recipient != parent || nonce == "" || len(row.Payload.Content) != 1 {
		return false
	}
	want := "Message Type: MESSAGE\nTask name: " + parent + "\nSender: " + child + "\nPayload:\n" + nonce
	return row.Payload.Content[0].Type == "input_text" && row.Payload.Content[0].Text == want
}

func reliabilityPrepare(root, binary, helper, evidence string) error {
	for _, name := range []string{root, binary, helper, evidence} {
		if !filepath.IsAbs(name) {
			return fmt.Errorf("explicit absolute paths required")
		}
	}
	if rel, err := filepath.Rel(root, evidence); err != nil || rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence must be outside project root")
	}
	original, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(original, &config); err != nil {
		return err
	}
	hooks, ok := config["hooks"].(map[string]any)
	if !ok || len(hooks) == 0 {
		return fmt.Errorf("no deployed hooks")
	}
	owned := 0
	productCommand := hookProbeQuote(binary) + " __hook --project-dir " + hookProbeQuote(root) + " --okf-binary " + hookProbeQuote(filepath.Join(filepath.Dir(binary), "okf"))
	for _, value := range hooks {
		groups, ok := value.([]any)
		if !ok {
			return fmt.Errorf("unsupported hook registration")
		}
		for _, value := range groups {
			group, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("unsupported hook group")
			}
			handlers, ok := group["hooks"].([]any)
			if !ok {
				return fmt.Errorf("unsupported hook handlers")
			}
			for _, value := range handlers {
				hook, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("unsupported hook handler")
				}
				if hook["type"] != "command" || hook["command"] != productCommand {
					continue
				}
				hook["command"] = hookProbeQuote(filepath.Join(evidence, "wrapper.sh"))
				owned++
			}
		}
	}
	if owned == 0 {
		return fmt.Errorf("no exact product-owned hook command found")
	}
	candidate, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	product, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	worker, err := os.ReadFile(helper)
	if err != nil {
		return err
	}
	wrapper := "#!/bin/sh\nrecord=$(mktemp " + hookProbeQuote(filepath.Join(evidence, "record-XXXXXXXX")) + " 2>/dev/null) || exec " + productCommand + "\nAIDLC_RELIABILITY_HELPER=1 exec " + hookProbeQuote(helper) + " '-test.run=^TestHookReliabilityProbeHelper$' -- " + hookProbeQuote(binary) + " " + hookProbeQuote(root) + " \"$record\"\n"
	manifest, err := json.MarshalIndent(map[string]any{"root": root, "binary": binary, "helper": helper, "product_sha256": reliabilityHash(product), "helper_sha256": reliabilityHash(worker), "original_hooks_sha256": reliabilityHash(original), "candidate_hooks_sha256": reliabilityHash(candidate), "model": "gpt-6-astra", "effort": "xhigh", "codex_version": "0.153.4", "case_seconds": 300, "total_seconds": 1500, "status": "prepared; normal hook trust and Intent preparation required before run"}, "", "  ")
	if err != nil {
		return err
	}
	for _, file := range []struct {
		name string
		data []byte
		mode os.FileMode
	}{{"wrapper.sh", []byte(wrapper), 0700}, {"hooks.candidate.json", candidate, 0600}, {"manifest.json", manifest, 0600}} {
		f, err := os.OpenFile(filepath.Join(evidence, file.name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.mode)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(file.data)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func reliabilityHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
