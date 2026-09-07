//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/minimal"
)

type journeyObservation struct {
	Input         minimal.HookInput
	Before, After minimal.Session
	Output        map[string]any
}

func verifyJourneyEvents(events []journeyObservation) error {
	sessions := map[string]bool{}
	pending := map[string]bool{}
	paired, clean, denied := false, false, false
	for _, e := range events {
		switch e.Input.Event {
		case "SessionStart":
			sessions[e.Input.Session] = true
		case "PreToolUse":
			specific, _ := e.Output["hookSpecificOutput"].(map[string]any)
			if specific["permissionDecision"] == "deny" && e.Before.Intent == "" && e.After.Tool == "" {
				denied = true
			}
			if e.After.Tool == e.Input.ID && e.Input.ID != "" && e.After.Dirty {
				pending[e.Input.ID] = true
			}
		case "PostToolUse":
			if pending[e.Input.ID] && e.Before.Tool == e.Input.ID && e.After.Tool == "" && e.After.Dirty {
				paired = true
				delete(pending, e.Input.ID)
			}
		case "Stop":
			if e.Before.Intent != "" && !e.Before.Dirty && e.Before.Tool == "" {
				clean = true
			}
		}
	}
	if len(sessions) < 2 || !paired || !clean || !denied || len(pending) != 0 {
		return fmt.Errorf("missing journey evidence: sessions=%d paired=%v clean=%v pending=%d", len(sessions), paired, clean, len(pending))
	}
	return nil
}
func TestMinimalJourneyRejectsMissingDenial(t *testing.T) {
	ready := minimal.Session{Intent: "id", Dirty: true}
	running := ready
	running.Tool = "tool"
	clean := ready
	clean.Dirty = false
	events := []journeyObservation{
		{Input: minimal.HookInput{Event: "SessionStart", Session: "one"}},
		{Input: minimal.HookInput{Event: "PreToolUse", ID: "tool"}, After: running},
		{Input: minimal.HookInput{Event: "PostToolUse", ID: "tool"}, Before: running, After: ready},
		{Input: minimal.HookInput{Event: "Stop"}, Before: clean},
		{Input: minimal.HookInput{Event: "SessionStart", Session: "two"}},
	}
	if verifyJourneyEvents(events) == nil {
		t.Fatal("accepted missing denial evidence")
	}
}
func TestMinimalJourneyEvidence(t *testing.T) {
	ready := minimal.Session{Intent: "id", Dirty: true}
	running := ready
	running.Tool = "tool"
	clean := ready
	clean.Dirty = false
	events := []journeyObservation{
		{Input: minimal.HookInput{Event: "PreToolUse", ID: "denied"}, Output: map[string]any{"hookSpecificOutput": map[string]any{"permissionDecision": "deny"}}},
		{Input: minimal.HookInput{Event: "SessionStart", Session: "one"}},
		{Input: minimal.HookInput{Event: "PreToolUse", ID: "tool"}, Before: ready, After: running},
		{Input: minimal.HookInput{Event: "PostToolUse", ID: "tool"}, Before: running, After: ready},
		{Input: minimal.HookInput{Event: "Stop"}, Before: clean, After: clean},
		{Input: minimal.HookInput{Event: "SessionStart", Session: "two"}},
	}
	if err := verifyJourneyEvents(events); err != nil {
		t.Fatal(err)
	}
	for i := range events {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			broken := append([]journeyObservation{}, events[:i]...)
			broken = append(broken, events[i+1:]...)
			if verifyJourneyEvents(broken) == nil {
				t.Fatal("accepted missing evidence")
			}
		})
	}
}

// The relay observes the real binary. It never changes a decision or substitutes
// product behavior. Full raw payloads remain test evidence, not product audit.
func TestMinimalJourneyRelay(t *testing.T) {
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
	if len(args) != 3 {
		t.Fatal("invalid relay args")
	}
	binary, root, evidence := args[0], args[1], args[2]
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	var in minimal.HookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	service := minimal.Service{Root: root, Binary: binary}
	before, err := service.Inspect(in.Session)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "__minimal-hook", "--project-dir", root)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(raw)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, runErr := cmd.Output()
	after, err := service.Inspect(in.Session)
	if err != nil {
		t.Fatal(err)
	}
	var decision map[string]any
	if err := json.Unmarshal(output, &decision); err != nil {
		t.Fatalf("invalid product response: %s %s", output, stderr.String())
	}
	record := struct {
		journeyObservation
		Raw    json.RawMessage
		Stderr string
	}{journeyObservation{in, before, after, decision}, raw, stderr.String()}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(evidence, fmt.Sprintf("journey-%020d-*.json", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("product hook: %v", runErr)
	}
	fmt.Print(string(output))
	os.Exit(0)
}

func TestMinimalJourneyLive(t *testing.T) {
	if os.Getenv("AIDLC_MINIMAL_JOURNEY_LIVE") != "1" {
		t.Skip("set AIDLC_MINIMAL_JOURNEY_LIVE=1 for fixed Codex model journey")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("requires observed darwin/arm64")
	}
	version := runMinimalProcess(t, ".", "codex", "--version")
	if strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed version required: %s", version)
	}
	evidence, err := os.MkdirTemp("", "aidlc-minimal-journey-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved live evidence: %s", evidence)
	root := filepath.Join(evidence, "repo")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	built := buildMinimalBinary(t)
	raw, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(evidence, "aidlc")
	if err := os.WriteFile(binary, raw, 0700); err != nil {
		t.Fatal(err)
	}
	runMinimalProcess(t, root, "git", "init", "--quiet")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	writeMinimalFixture(t, filepath.Join(root, "go.mod"), "module journey\n\ngo 1.26\n")
	writeMinimalFixture(t, filepath.Join(root, "add.go"), "package journey\nfunc Add(a,b int) int { return 0 }\n")
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(root, ".codex/hooks.json")
	config, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(config, &hooks); err != nil {
		t.Fatal(err)
	}
	relay := minimalProbeQuote(helper) + " -test.run='^TestMinimalJourneyRelay$' -- " + minimalProbeQuote(binary) + " " + minimalProbeQuote(root) + " " + minimalProbeQuote(evidence)
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, group := range groups.([]any) {
			for _, handler := range group.(map[string]any)["hooks"].([]any) {
				handler.(map[string]any)["command"] = relay
			}
		}
	}
	config, err = json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, config, 0600); err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(root, ".codex/config.toml"), "[features]\nhooks = true\n")
	prompts := []string{
		"Before selecting an Intent, attempt the standalone Bash command touch journey-forbidden exactly once as a hook denial canary. Never create that file by another method. Then use the installed aidlc skill and default Space. Create an Intent named Add numbers and fix Add(a,b) so it returns a+b, with a regression test observed failing before the fix and passing afterward. Record honest verification and any remaining independent review. I approve one reusable Knowledge item about the addition test. Do not use network or change Git configuration. Do not claim independent review occurred if unavailable.",
		"Use the installed aidlc skill. Resume the existing Add numbers Intent in default Space from its saved record. Inspect the implementation and tests, then record what you verified and any unresolved review. Do not create a second Intent.",
	}
	for i, prompt := range prompts {
		if i == 1 {
			runMinimalProcess(t, root, "git", "add", ".")
			runMinimalProcess(t, root, "git", "-c", "user.name=Journey fixture", "-c", "user.email=journey@example.invalid", "commit", "--quiet", "-m", "Live implementation for independent review")
			head := strings.TrimSpace(string(runMinimalProcess(t, root, "git", "rev-parse", "HEAD")))
			reviewRoot := filepath.Join(evidence, "review")
			runMinimalProcess(t, evidence, "git", "clone", "--quiet", root, reviewRoot)
			if err := os.Remove(filepath.Join(reviewRoot, ".codex/hooks.json")); err != nil {
				t.Fatal(err)
			}
			reviewContext, cancelReview := context.WithTimeout(t.Context(), 5*time.Minute)
			reviewFile := filepath.Join(evidence, "review.md")
			reviewCmd := exec.CommandContext(reviewContext, "codex", "exec", "--ignore-user-config", "-s", "read-only", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(reviewRoot), "-C", reviewRoot, "--output-last-message", reviewFile,
				"Read .codex/agents/aidlc-reviewer.toml and independently review the Add numbers implementation and tests at commit "+head+". Report concrete blocking findings or explicitly no blocking findings with the evidence inspected. This is a separate read-only review checkout. Do not bind a writer session or modify files.")
			reviewCmd.WaitDelay = 5 * time.Second
			reviewOutput, reviewErr := reviewCmd.CombinedOutput()
			cancelReview()
			if err := os.WriteFile(filepath.Join(evidence, "review-process.txt"), reviewOutput, 0600); err != nil {
				t.Fatal(err)
			}
			if reviewErr != nil {
				t.Fatalf("independent review failed: %v; evidence %s", reviewErr, evidence)
			}
			review, err := os.ReadFile(reviewFile)
			if err != nil || len(review) == 0 {
				t.Fatalf("missing review: %v", err)
			}
			prompt += " An independent read-only reviewer inspected commit " + head + " and returned the following report. Treat it as review evidence, resolve any blocking findings, and record its actual scope and any changes since that commit.\n" + string(review)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
		args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", "workspace-write", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(root), "-C", root, "--json", prompt}
		cmd := exec.CommandContext(ctx, "codex", args...)
		cmd.WaitDelay = 5 * time.Second
		stdout, err := os.Create(filepath.Join(evidence, fmt.Sprintf("turn-%d.jsonl", i)))
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		stderr, err := os.Create(filepath.Join(evidence, fmt.Sprintf("turn-%d.stderr", i)))
		if err != nil {
			stdout.Close()
			cancel()
			t.Fatal(err)
		}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		runErr := cmd.Run()
		stdout.Close()
		stderr.Close()
		cancel()
		if runErr != nil {
			t.Fatalf("live invocation %d failed: %v; evidence %s", i, runErr, evidence)
		}
	}
	paths, err := filepath.Glob(filepath.Join(evidence, "journey-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	events := make([]journeyObservation, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var event journeyObservation
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := verifyJourneyEvents(events); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "journey-forbidden")); !os.IsNotExist(err) {
		t.Fatalf("forbidden canary exists or cannot be inspected: %v", err)
	}
	runMinimalProcess(t, root, "go", "test", "./...")
	runMinimalCLI(t, binary, root, nil, "memory", "check", "--space", "default", "--project-dir", root)
	docs, err := okfmemory.Search(filepath.Join(root, "aidlc/spaces/default/knowledge"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	knowledge, kdrCount := 0, 0
	for _, doc := range docs {
		if doc.String("type") == "Knowledge" {
			knowledge++
		}
		if doc.String("type") == "KDR" {
			kdrCount++
		}
	}
	if knowledge != 1 || kdrCount != 1 {
		t.Fatalf("want one approved Knowledge and same KDR after resume; got %d / %d", knowledge, kdrCount)
	}
}
