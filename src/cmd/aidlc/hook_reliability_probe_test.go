package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	var input minimalProbeInput
	_ = json.Unmarshal(raw, &input)
	r.Before = reliabilityReadSession(root, input.Session)
	cmd := exec.CommandContext(ctx, binary, "__minimal-hook", "--project-dir", root)
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
	binary := filepath.Join(t.TempDir(), "aidlc")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build product: %v: %s", err, out)
	}
	for _, tc := range []struct {
		name, raw, before, after string
		lock                     bool
	}{
		{"terminal", ` {"session_id":"session","turn_id":"turn","tool_use_id":"tool","tool_name":"Bash","hook_event_name":"PostToolUse","unknown":{"preserve":true}} `, "tool", "", false},
		{"duplicate", `{"session_id":"session","turn_id":"turn","tool_use_id":"tool","tool_name":"Bash","hook_event_name":"PostToolUse"}`, "", "", false},
		{"lock_failure", `{"session_id":"session","turn_id":"turn","tool_use_id":"tool","tool_name":"Bash","hook_event_name":"PostToolUse"}`, "tool", "tool", true},
		{"invalid_wire", "not json\n", "tool", "tool", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
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
			cmd := exec.CommandContext(t.Context(), binary, "__minimal-hook", "--project-dir", root)
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
			var input minimalProbeInput
			if tc.name != "invalid_wire" && (json.Unmarshal(got.Raw, &input) != nil || input.Session != "session" || input.Turn != "turn" || input.ID != "tool") {
				t.Fatal("lost correlation IDs")
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
	if e.Type != "item_completed" || e.Item.Type != "CommandExecution" || e.Item.Status != "completed" || e.Item.Exit == nil || e.Completed <= 0 || e.Session == "" || e.Turn == "" || e.Item.ID == "" {
		return unknown("unsupported or incomplete terminal evidence")
	}
	if len(records) < 2 {
		return unknown("missing Pre or Post")
	}
	last := time.Time{}
	var previous []byte
	failure := ""
	for i, r := range records {
		var input minimalProbeInput
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

func TestHookReliabilityProbeEvidence(t *testing.T) {
	t.Run("prepare_preserves_registration", func(t *testing.T) {
		root, evidence := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".codex"), 0700); err != nil {
			t.Fatal(err)
		}
		original := []byte(`{"hooks":{"PreToolUse":[{"matcher":"^Bash$","hooks":[{"type":"command","command":"product","timeout":10}]}]}}`)
		if err := os.WriteFile(filepath.Join(root, ".codex/hooks.json"), original, 0600); err != nil {
			t.Fatal(err)
		}
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if err := reliabilityPrepare(root, binary, binary, evidence); err != nil {
			t.Fatal(err)
		}
		candidate, err := os.ReadFile(filepath.Join(evidence, "hooks.candidate.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(candidate, []byte(`"matcher": "^Bash$"`)) || !bytes.Contains(candidate, []byte(`"timeout": 10`)) || !bytes.Contains(candidate, []byte("wrapper.sh")) {
			t.Fatalf("registration not preserved: %s", candidate)
		}
		unchanged, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
		if err != nil || !bytes.Equal(unchanged, original) {
			t.Fatal("prepare modified installed hooks")
		}
		wrapper, err := os.ReadFile(filepath.Join(evidence, "wrapper.sh"))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"bypass", "trust_level", "assignment init", "app-server"} {
			if bytes.Contains(wrapper, []byte(forbidden)) {
				t.Fatalf("unsafe prepare %s", forbidden)
			}
		}
	})
	// Inputs are literal protocol examples, independent of collector output.
	preRaw := []byte(`{"hook_event_name":"PreToolUse","session_id":"s","turn_id":"t","tool_use_id":"exec-1","tool_name":"Bash","tool_input":{"command":"printf done"}}`)
	postRaw := []byte(`{"hook_event_name":"PostToolUse","session_id":"s","turn_id":"t","tool_use_id":"exec-1","tool_name":"Bash","tool_response":"done"}`)
	terminal := []byte(`{"type":"event_msg","payload":{"type":"item_completed","thread_id":"s","turn_id":"t","completed_at_ms":2000,"item":{"type":"CommandExecution","id":"exec-1","status":"completed","exit_code":0}}}`)
	empty := []byte("space=\nintent=\nturn=t\ntool=\nrule_turn=\nrule_hash=\n")
	busy := []byte("space=\nintent=\nturn=t\ntool=exec-1\nrule_turn=\nrule_hash=\n")
	for _, tc := range []struct {
		name, observation, product string
		mutate                     func(*[]reliabilityRecord, *[]byte)
	}{
		{"cleared", "complete", "pass", nil},
		{"missing_post", "incomplete", "unknown", func(r *[]reliabilityRecord, _ *[]byte) { *r = (*r)[:1] }},
		{"missing_terminal", "incomplete", "unknown", func(_ *[]reliabilityRecord, b *[]byte) { *b = nil }},
		{"running", "incomplete", "unknown", func(_ *[]reliabilityRecord, b *[]byte) {
			*b = bytes.ReplaceAll(*b, []byte(`"completed"`), []byte(`"inProgress"`))
		}},
		{"wrong_id", "incomplete", "unknown", func(r *[]reliabilityRecord, _ *[]byte) {
			(*r)[1].Raw = bytes.ReplaceAll((*r)[1].Raw, []byte("exec-1"), []byte("exec-2"))
		}},
		{"wrong_turn", "incomplete", "unknown", func(_ *[]reliabilityRecord, b *[]byte) {
			*b = bytes.ReplaceAll(*b, []byte(`"turn_id":"t"`), []byte(`"turn_id":"other"`))
		}},
		{"missing_snapshot", "incomplete", "unknown", func(r *[]reliabilityRecord, _ *[]byte) { (*r)[1].After = reliabilitySnapshot{Missing: true} }},
		{"retained", "complete", "unrepaired", func(r *[]reliabilityRecord, _ *[]byte) { (*r)[1].After.Data = busy }},
		{"save_failure", "complete", "unrepaired", func(r *[]reliabilityRecord, _ *[]byte) {
			(*r)[1].After.Data = busy
			(*r)[1].Stdout = []byte(`{"continue":false,"stopReason":"lock unavailable"}`)
		}},
		{"hook_exit", "complete", "unrepaired", func(r *[]reliabilityRecord, _ *[]byte) { (*r)[1].Exit = 1 }},
		{"tool_failure", "complete", "pass", func(_ *[]reliabilityRecord, b *[]byte) {
			*b = bytes.ReplaceAll(*b, []byte(`"exit_code":0`), []byte(`"exit_code":7`))
		}},
		{"post_before_terminal", "incomplete", "unknown", func(r *[]reliabilityRecord, _ *[]byte) { (*r)[1].Started = time.UnixMilli(1500) }},
		{"duplicate_idempotent", "complete", "pass", func(r *[]reliabilityRecord, _ *[]byte) {
			d := (*r)[1]
			d.Before = d.After
			d.Started = time.UnixMilli(4000)
			d.Finished = time.UnixMilli(4100)
			*r = append(*r, d)
		}},
		{"duplicate_mutates", "complete", "unrepaired", func(r *[]reliabilityRecord, _ *[]byte) {
			d := (*r)[1]
			d.Before = d.After
			d.After.Data = busy
			d.Started = time.UnixMilli(4000)
			d.Finished = time.UnixMilli(4100)
			*r = append(*r, d)
		}},
		{"child_final_only", "incomplete", "unknown", func(r *[]reliabilityRecord, b *[]byte) {
			*r = nil
			*b = []byte(`{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"reported"}}`)
		}},
		{"listed_only", "incomplete", "unknown", func(r *[]reliabilityRecord, b *[]byte) { *r = nil; *b = []byte(`{"hooks":[{"event":"PreToolUse"}]}`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := []reliabilityRecord{
				{Raw: preRaw, Stdout: []byte("{}\n"), Before: reliabilitySnapshot{Data: empty}, After: reliabilitySnapshot{Data: busy}, Started: time.UnixMilli(1000), Finished: time.UnixMilli(1100)},
				{Raw: postRaw, Stdout: []byte("{}\n"), Before: reliabilitySnapshot{Data: busy}, After: reliabilitySnapshot{Data: empty}, Started: time.UnixMilli(2000), Finished: time.UnixMilli(2100)},
			}
			b := bytes.Clone(terminal)
			if tc.mutate != nil {
				tc.mutate(&r, &b)
			}
			got := reliabilityEvaluate(r, b)
			if got.Observation != tc.observation || got.Product != tc.product {
				t.Fatalf("got %+v, want %s/%s", got, tc.observation, tc.product)
			}
		})
	}
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
	var config struct {
		Hooks map[string][]struct {
			Matcher string           `json:"matcher,omitempty"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(original, &config); err != nil {
		return err
	}
	if len(config.Hooks) == 0 {
		return fmt.Errorf("no deployed hooks")
	}
	for _, groups := range config.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook["type"] != "command" {
					return fmt.Errorf("unsupported deployed hook")
				}
				hook["command"] = minimalProbeQuote(filepath.Join(evidence, "wrapper.sh"))
			}
		}
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
	wrapper := "#!/bin/sh\nrecord=$(mktemp " + minimalProbeQuote(filepath.Join(evidence, "record-XXXXXXXX")) + ") || exit 125\nAIDLC_RELIABILITY_HELPER=1 exec " + minimalProbeQuote(helper) + " '-test.run=^TestHookReliabilityProbeHelper$' -- " + minimalProbeQuote(binary) + " " + minimalProbeQuote(root) + " \"$record\"\n"
	manifest, err := json.MarshalIndent(map[string]any{"root": root, "binary": binary, "helper": helper, "product_sha256": reliabilityHash(product), "helper_sha256": reliabilityHash(worker), "original_hooks_sha256": reliabilityHash(original), "candidate_hooks_sha256": reliabilityHash(candidate), "model": "gpt-6-astra", "effort": "xhigh", "codex_version": "0.153.4", "case_seconds": 300, "total_seconds": 1500, "status": "prepared; normal hook trust and assignment confirmation required before run"}, "", "  ")
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
