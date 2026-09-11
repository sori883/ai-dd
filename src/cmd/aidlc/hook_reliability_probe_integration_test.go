//go:build integration

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// Prepare produces reviewable assets only. Run uses an already prepared and
// normally trusted project; this entry never initializes assignments or trust.
func TestHookReliabilityProbeLive(t *testing.T) {
	root := os.Getenv("AIDLC_HOOK_RELIABILITY_ROOT")
	binary := os.Getenv("AIDLC_HOOK_RELIABILITY_BINARY")
	helper := os.Getenv("AIDLC_HOOK_RELIABILITY_HELPER_BINARY")
	evidence := os.Getenv("AIDLC_HOOK_RELIABILITY_EVIDENCE")
	if os.Getenv("AIDLC_HOOK_RELIABILITY_PREPARE") == "1" {
		if err := reliabilityPrepare(root, binary, helper, evidence); err != nil {
			t.Fatal(err)
		}
		t.Logf("Prepared candidate assets in %s; install/review and normal trust/Intent preparation remain pending", evidence)
		return
	}
	if os.Getenv("AIDLC_HOOK_RELIABILITY_LIVE") != "1" {
		t.Skip("live observation not requested")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("requires fixed macOS arm64 environment")
	}
	if os.Getenv("AIDLC_HOOK_RELIABILITY_READY") != "1" {
		t.Fatal("parent must confirm prepared assets, normal trust, and Intent preparation before run")
	}
	manifestRaw, err := os.ReadFile(filepath.Join(evidence, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ key, path, hash string }{{"root", root, ""}, {"binary", binary, "product_sha256"}, {"helper", helper, "helper_sha256"}} {
		if manifest[entry.key] != entry.path {
			t.Fatalf("prepared %s changed", entry.key)
		}
		if entry.hash != "" {
			data, err := os.ReadFile(entry.path)
			if err != nil || manifest[entry.hash] != reliabilityHash(data) {
				t.Fatalf("prepared %s bytes changed", entry.key)
			}
		}
	}
	installed, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil || manifest["candidate_hooks_sha256"] != reliabilityHash(installed) {
		t.Fatal("installed hooks differ from prepared candidate")
	}
	old, err := filepath.Glob(filepath.Join(evidence, "record-*"))
	if err != nil || len(old) != 0 {
		t.Fatal("run requires unused evidence directory; preserve earlier attempts separately")
	}
	space, intent := os.Getenv("AIDLC_HOOK_RELIABILITY_SPACE"), os.Getenv("AIDLC_HOOK_RELIABILITY_INTENT")
	if space == "" || intent == "" {
		t.Fatal("parent must supply an existing Space and Intent")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Minute)
	defer cancel()
	version, err := exec.CommandContext(ctx, "codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex version unavailable: %s", version)
	}
	cases := []struct{ name, request string }{
		{"success", "Execute the finite command `printf hook-probe-success` and observe exit 0."},
		{"failure", "Execute the finite command `printf hook-probe-failure; exit 7` and observe exit 7."},
		{"poll_success", "Execute `sleep 3; printf hook-probe-poll-success` with yield_time_ms=1, then poll the returned process session until terminal."},
		{"poll_failure", "Execute `sleep 3; printf hook-probe-poll-failure; exit 7` with yield_time_ms=1, then poll the returned process session until terminal."},
		{"failed_patch", "Attempt exactly one apply_patch Update File for `hook-probe-nonexistent-file` with an intentionally nonexistent context. Do not create that file or retry. Preserve the expected failure."},
		{"parallel_reads", "Request two read-only commands in parallel: `printf hook-probe-read-one` and `printf hook-probe-read-two`. Keep any hook denial as evidence; do not retry."},
	}
	extraMode := os.Getenv("AIDLC_HOOK_RELIABILITY_EXTRA") == "1"
	if extraMode {
		cases = cases[4:]
	} else {
		cases = cases[:4]
	}
	childMode := os.Getenv("AIDLC_HOOK_RELIABILITY_CHILD") == "1"
	if childMode {
		if os.Getenv("AIDLC_HOOK_RELIABILITY_ASSIGNMENT_READY") != "1" {
			t.Fatal("child observation requires the parent's confirmation that normal assignment initialization is complete")
		}
		request, err := reliabilityChildRequest(os.Getenv("AIDLC_HOOK_RELIABILITY_PARENT_TASK"), os.Getenv("AIDLC_HOOK_RELIABILITY_CHILD_ROLE"))
		if err != nil {
			t.Fatal(err)
		}
		cases = []struct{ name, request string }{{"child_report", request}}
	}
	runIncomplete := false
	completedCases := 0
	for _, scenario := range cases {
		caseCtx, caseCancel := context.WithTimeout(ctx, 5*time.Minute)
		prompt := fmt.Sprintf("This is an authorized diagnostic in the prepared project. Use existing Space %q and Intent %q; read its state and Rules through the installed CLI before the operation. Follow the normal product hooks. Never approve on behalf of a human, initialize assignments, recover sessions, alter hooks/trust, or repair an observed failure. Only an explicitly requested child may be started. Keep expected denials as evidence without retries. %s", space, intent, scenario.request)
		cmd := exec.CommandContext(caseCtx, "codex", "exec", "--json", "-C", root, "-s", "workspace-write", "-m", "gpt-6-astra", "-c", `model_reasoning_effort="xhigh"`, prompt)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		runErr := cmd.Run()
		if runErr != nil || caseCtx.Err() != nil {
			runIncomplete = true
		}
		completedCases++
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		markers := map[string][]string{"success": {"hook-probe-success"}, "failure": {"hook-probe-failure"}, "poll_success": {"hook-probe-poll-success"}, "poll_failure": {"hook-probe-poll-failure"}, "failed_patch": {"hook-probe-nonexistent-file"}, "parallel_reads": {"hook-probe-read-one", "hook-probe-read-two"}}
		result := map[string]any{"exit": code, "timeout": caseCtx.Err() != nil, "codex_version": strings.TrimSpace(string(version)), "go_version": runtime.Version(), "scenario": scenario.name, "markers": markers[scenario.name]}
		if runErr != nil {
			result["error"] = runErr.Error()
		}
		caseCancel()
		for _, file := range []struct {
			name string
			data []byte
		}{{scenario.name + ".stdout.jsonl", stdout.Bytes()}, {scenario.name + ".stderr", stderr.Bytes()}} {
			if err := os.WriteFile(filepath.Join(evidence, file.name), file.data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(filepath.Join(evidence, scenario.name+".execution.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("case %s completed; process exit=%d, observation files retained", scenario.name, code)
		if ctx.Err() != nil {
			break
		}
	}
	if childMode {
		data, err := json.MarshalIndent(map[string]reliabilityResult{"child_report": {"incomplete", "unknown", "raw observations collected; strict structured parent receipt assessment is required"}}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, "summary.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Fatal("child observation retained without a pass claim; inspect child-generated nonce, send hook, and structured parent receipt before assessment")
	}
	summary, err := reliabilityCollect(evidence)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "summary.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if extraMode {
		t.Fatal("failed-patch and parallel-denial evidence collected for explicit assessment; unsupported terminal formats are not a product pass")
	}
	// Collection is not a claim that the product is repaired or that all cases
	// ran. The parent checks requests against the retained terminal inventory.
	t.Logf("Observation retained at %s; scenario coverage, child reports, failed-patch terminal format, and unwrapped verification require assessment", evidence)
	if len(summary) == 0 {
		t.Fatal("no paired terminal observations; live observation incomplete")
	}
	if runIncomplete || completedCases != len(cases) {
		t.Fatal("one or more diagnostic processes failed or timed out; observation incomplete")
	}
	// A successful CLI exit can still mean the requested operation was never
	// attempted. Require tool-input evidence for every requested case.
	recordFiles, err := filepath.Glob(filepath.Join(evidence, "record-*"))
	if err != nil {
		t.Fatal(err)
	}
	var inputs []byte
	for _, name := range recordFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var record reliabilityRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		var input minimalProbeInput
		if err := json.Unmarshal(record.Raw, &input); err != nil {
			t.Fatal(err)
		}
		if input.Event == "PreToolUse" {
			inputs = append(inputs, []byte(input.Input.Command)...)
		}
	}
	for _, marker := range []string{"hook-probe-success", "hook-probe-failure", "hook-probe-poll-success", "hook-probe-poll-failure"} {
		if !bytes.Contains(inputs, []byte(marker)) {
			t.Fatalf("requested case absent from hook inputs: %s", marker)
		}
	}
	for _, result := range summary {
		if result.Observation != "complete" {
			t.Fatal("live observation incomplete; inspect summary.json")
		}
	}
}

// This read-only assessment accepts a nonce already linked by the parent to
// the child's generation result and successful send hook. It does not infer
// those facts from a final answer or start another Codex session.
func TestHookReliabilityProbeReceipt(t *testing.T) {
	name := os.Getenv("AIDLC_HOOK_RELIABILITY_RECEIPT_FILE")
	if name == "" {
		t.Skip("explicit receipt assessment not requested")
	}
	parent, child, nonce := os.Getenv("AIDLC_HOOK_RELIABILITY_PARENT_TASK"), os.Getenv("AIDLC_HOOK_RELIABILITY_CHILD_TASK"), os.Getenv("AIDLC_HOOK_RELIABILITY_NONCE")
	if parent == "" || child == "" || nonce == "" {
		t.Fatal("verified parent, child canonical path and child-generated nonce required")
	}
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	found := false
	for scanner.Scan() {
		if reliabilityChildReceipt(scanner.Bytes(), parent, child, nonce) {
			found = true
			t.Logf("matching structured MESSAGE row sha256=%s", reliabilityHash(scanner.Bytes()))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("matching structured parent receipt absent or unsupported; final answers are not receipts")
	}
}

func reliabilityCollect(evidence string) (map[string]reliabilityResult, error) {
	caseFiles, err := filepath.Glob(filepath.Join(evidence, "*.execution.json"))
	if err != nil {
		return nil, err
	}
	var markers []string
	for _, name := range caseFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var entry struct {
			Markers []string `json:"markers"`
		}
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, err
		}
		for _, marker := range entry.Markers {
			if marker == "" {
				return nil, fmt.Errorf("empty case marker")
			}
			markers = append(markers, marker)
		}
	}
	if len(markers) == 0 {
		return nil, fmt.Errorf("missing case records with explicit markers")
	}
	files, err := filepath.Glob(filepath.Join(evidence, "record-*"))
	if err != nil {
		return nil, err
	}
	groups := map[string][]reliabilityRecord{}
	transcripts := map[string]bool{}
	selected := map[string]bool{}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var record reliabilityRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("incomplete wrapper record %s: %w", name, err)
		}
		var input minimalProbeInput
		if err := json.Unmarshal(record.Raw, &input); err != nil {
			return nil, err
		}
		if input.Transcript != "" {
			transcripts[input.Transcript] = true
		}
		if input.Event == "PreToolUse" || input.Event == "PostToolUse" {
			key := input.Session + "/" + input.Turn + "/" + input.ID
			groups[key] = append(groups[key], record)
			if input.Event == "PreToolUse" {
				for _, marker := range markers {
					if strings.Contains(input.Input.Command, marker) {
						selected[key] = true
					}
				}
			}
		}
	}
	for key := range groups {
		if !selected[key] {
			delete(groups, key)
		}
	}
	terminals := map[string][]byte{}
	for name := range transcripts {
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 8<<20)
		for scanner.Scan() {
			var row struct {
				Type    string `json:"type"`
				Payload struct {
					Type    string `json:"type"`
					Session string `json:"thread_id"`
					Turn    string `json:"turn_id"`
					Item    struct {
						ID   string `json:"id"`
						Type string `json:"type"`
					} `json:"item"`
				} `json:"payload"`
			}
			if json.Unmarshal(scanner.Bytes(), &row) != nil {
				continue
			}
			if row.Type != "event_msg" || row.Payload.Type != "item_completed" || row.Payload.Item.Type != "CommandExecution" {
				continue
			}
			key := row.Payload.Session + "/" + row.Payload.Turn + "/" + row.Payload.Item.ID
			if _, ok := groups[key]; ok {
				terminals[key] = bytes.Clone(scanner.Bytes())
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	summary := map[string]reliabilityResult{}
	for key, records := range groups {
		sort.Slice(records, func(i, j int) bool { return records[i].Started.Before(records[j].Started) })
		summary[key] = reliabilityEvaluate(records, terminals[key])
	}
	return summary, nil
}

func TestHookReliabilityProbeCollectedEvidence(t *testing.T) {
	evidence := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "events.jsonl")
	// Diagnostic transport is independent of the hook record producer. A final
	// answer containing a matching ID must never replace a terminal item.
	terminal := `{"type":"event_msg","payload":{"type":"item_completed","thread_id":"s","turn_id":"t","completed_at_ms":2000,"item":{"type":"CommandExecution","id":"exec-1","status":"failed","exit_code":7}}}` + "\n" + `{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"exec-1 completed"}}` + "\n"
	if err := os.WriteFile(filepath.Join(evidence, "success.execution.json"), []byte(`{"scenario":"success","markers":["hook-probe-success"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte(terminal), 0600); err != nil {
		t.Fatal(err)
	}
	before := []byte("space=\nintent=\nturn=t\ntool=\nrule_turn=\nrule_hash=\n")
	busy := []byte("space=\nintent=\nturn=t\ntool=exec-1\nrule_turn=\nrule_hash=\n")
	for i, event := range []string{"PreToolUse", "PostToolUse"} {
		raw := []byte(fmt.Sprintf(`{"hook_event_name":%q,"session_id":"s","turn_id":"t","tool_use_id":"exec-1","tool_name":"Bash","tool_input":{"command":"printf hook-probe-success"},"transcript_path":%q}`, event, transcript))
		r := reliabilityRecord{Raw: raw, Stdout: []byte("{}\n"), Before: reliabilitySnapshot{Data: before}, After: reliabilitySnapshot{Data: busy}, Started: time.UnixMilli(1000), Finished: time.UnixMilli(1100)}
		if i == 1 {
			r.Before.Data, r.After.Data = busy, before
			r.Started, r.Finished = time.UnixMilli(2000), time.UnixMilli(2100)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("record-%d", i)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	bootstrap := reliabilityRecord{Raw: []byte(`{"hook_event_name":"PreToolUse","session_id":"s","turn_id":"t","tool_use_id":"bootstrap","tool_name":"Bash","tool_input":{"command":"aidlc session inspect s"}}`), Stdout: []byte("{}\n"), Before: reliabilitySnapshot{Data: before}, After: reliabilitySnapshot{Data: before}}
	bootstrapData, _ := json.Marshal(bootstrap)
	if err := os.WriteFile(filepath.Join(evidence, "record-bootstrap"), bootstrapData, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := reliabilityCollect(evidence)
	if err != nil || len(got) != 1 || got["s/t/exec-1"].Product != "pass" {
		t.Fatalf("terminal collection=%+v, error=%v", got, err)
	}
	if err := os.WriteFile(transcript, []byte(`{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"exec-1 completed"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = reliabilityCollect(evidence)
	if err != nil || got["s/t/exec-1"].Observation != "incomplete" {
		t.Fatalf("final-only collection=%+v, error=%v", got, err)
	}
}
