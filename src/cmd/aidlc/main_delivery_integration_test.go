//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
)

const deliveryJourneyGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const deliveryJourneyScopeGridJSON = `{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE","next-stage":"EXECUTE"}}}`

func TestDeliveryJourney(t *testing.T) {
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "aidlc")
	build := exec.Command("go", "build", "-o", binaryPath, "./src/cmd/aidlc")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build aidlc: %v\n%s", err, output)
	}

	project := newDeliveryJourneyProject(t)
	next := runDeliveryBinary(t, binaryPath, project, "next")
	if next.kind != "load-steering" || next.token == "" || next.parts < 2 {
		t.Fatalf("next = %#v, want load-steering with continuation", next)
	}
	markerBytes, err := os.ReadFile(filepath.Join(project, "aidlc", "spaces", "team", "intents", "build", ".aidlc-active-directive.json"))
	if err != nil {
		t.Fatalf("ReadFile(active marker): %v", err)
	}
	var marker struct {
		IntentUUID *string `json:"intent_uuid"`
	}
	if err := json.Unmarshal(markerBytes, &marker); err != nil {
		t.Fatalf("Unmarshal(active marker): %v", err)
	}
	if marker.IntentUUID == nil || *marker.IntentUUID != "delivery-journey-intent" {
		t.Fatalf("active marker intent_uuid = %v, want delivery-journey-intent", marker.IntentUUID)
	}
	firstToken := next.token
	for next.kind == "load-steering" {
		next = runDeliveryBinary(t, binaryPath, project, "continue", next.token)
	}
	if next.kind != "run-stage" {
		t.Fatalf("final directive = %#v, want run-stage", next)
	}

	replay := runDeliveryBinary(t, binaryPath, project, "continue", firstToken)
	if replay.kind != "error" {
		t.Fatalf("replayed token = %#v, want terminal error", replay)
	}

	staleProject := newDeliveryJourneyProject(t)
	stale := runDeliveryBinary(t, binaryPath, staleProject, "next")
	if stale.kind != "load-steering" {
		t.Fatalf("stale setup next = %#v, want load-steering", stale)
	}
	rulePath := filepath.Join(staleProject, "delivery-rule.md")
	if err := os.WriteFile(rulePath, []byte("changed rule\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed rule): %v", err)
	}
	staleResult := runDeliveryBinary(t, binaryPath, staleProject, "continue", stale.token)
	if staleResult.kind != "error" {
		t.Fatalf("stale rule result = %#v, want terminal error", staleResult)
	}

	missingProject := newDeliveryJourneyProject(t)
	if err := os.Remove(filepath.Join(missingProject, "delivery-rule.md")); err != nil {
		t.Fatalf("Remove(required rule): %v", err)
	}
	missing := runDeliveryBinary(t, binaryPath, missingProject, "next")
	if missing.exitCode != 1 || missing.stdout.Len() != 0 {
		t.Fatalf("missing required rule result = code %d stdout %q, want internal fail-closed", missing.exitCode, missing.stdout.String())
	}

}

func TestReportHumanTurnJourney(t *testing.T) {
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "aidlc")
	build := exec.Command("go", "build", "-o", binaryPath, "./src/cmd/aidlc")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build aidlc: %v\n%s", err, output)
	}

	noHookProject := newDeliveryJourneyProject(t)
	assertNoCodexHookConfig(t, noHookProject)
	noHookStateBefore, err := os.ReadFile(filepath.Join(noHookProject, "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md"))
	if err != nil {
		t.Fatalf("ReadFile(state before report): %v", err)
	}
	noHook := runReportBinary(t, binaryPath, noHookProject, nil,
		"report", "--stage", "intent-capture", "--result", "rejected",
		"--user-input", "Request Changes", "--reason", "revise")
	if noHook.exitCode != 0 || noHook.kind != "error" || noHook.stderr.Len() != 0 {
		t.Fatalf("report without hook = %#v, want terminal workflow error", noHook)
	}
	if !strings.Contains(noHook.message, "HUMAN_TURN") {
		t.Fatalf("report without hook message = %q, want stale HUMAN_TURN", noHook.message)
	}
	noHookStateAfter, err := os.ReadFile(filepath.Join(noHookProject, "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md"))
	if err != nil {
		t.Fatalf("ReadFile(state after report): %v", err)
	}
	if string(noHookStateAfter) != string(noHookStateBefore) {
		t.Fatal("public report without a human turn changed state")
	}
	if _, err := os.Stat(filepath.Join(noHookProject, "aidlc", "spaces", "team", "intents", "build", "audit")); !os.IsNotExist(err) {
		t.Fatalf("public report without a human turn audit = %v, want absent", err)
	}

	installCodexHookConfig(t, moduleRoot, noHookProject)
	hook := runConfiguredHumanTurnHook(t, binaryPath, noHookProject, []byte(`{"session_id":"s1","prompt":"Request Changes"}`), nil)
	if hook.exitCode != 0 || hook.stdout.Len() != 0 || hook.stderr.Len() != 0 {
		t.Fatalf("human-turn hook = code %d stdout %q stderr %q, want silent success", hook.exitCode, hook.stdout.String(), hook.stderr.String())
	}
	rejected := runReportBinary(t, binaryPath, noHookProject, nil,
		"report", "--stage", "intent-capture", "--result", "rejected",
		"--user-input", "Request Changes", "--reason", "revise")
	if rejected.kind != "print" || rejected.exitCode != 0 || !strings.Contains(rejected.message, "Recorded rejected") {
		t.Fatalf("report rejected = %#v, want print", rejected)
	}
	revised := runReportBinary(t, binaryPath, noHookProject, nil,
		"report", "--stage", "intent-capture", "--result", "revised")
	if revised.kind != "print" || revised.exitCode != 0 || !strings.Contains(revised.message, "Recorded revised") {
		t.Fatalf("report revised = %#v, want print", revised)
	}
	staleApproval := runReportBinary(t, binaryPath, noHookProject, nil,
		"report", "--stage", "intent-capture", "--result", "approved", "--user-input", "Approve")
	if staleApproval.kind != "error" || staleApproval.exitCode != 0 || !strings.Contains(staleApproval.message, "HUMAN_TURN") {
		t.Fatalf("approve without new hook = %#v, want stale workflow error", staleApproval)
	}
	_ = runConfiguredHumanTurnHook(t, binaryPath, noHookProject, []byte(`{"prompt":"Approve"}`), nil)
	approved := runReportBinary(t, binaryPath, noHookProject, nil,
		"report", "--stage", "intent-capture", "--result", "approved", "--user-input", "Approve")
	if approved.kind != "done" || approved.exitCode != 0 || !strings.Contains(approved.reason, "State advanced; run next to continue.") {
		t.Fatalf("approve after hook = %#v, want done", approved)
	}

	noWorkflowProject := t.TempDir()
	installCodexHookConfig(t, moduleRoot, noWorkflowProject)
	noWorkflowHook := runConfiguredHumanTurnHook(t, binaryPath, noWorkflowProject, []byte(`{"prompt":"Approve"}`), nil)
	if noWorkflowHook.exitCode != 0 || noWorkflowHook.stdout.Len() != 0 || noWorkflowHook.stderr.Len() != 0 {
		t.Fatalf("hook without workflow = code %d stdout %q stderr %q, want silent no-op", noWorkflowHook.exitCode, noWorkflowHook.stdout.String(), noWorkflowHook.stderr.String())
	}

	unattendedProject := newDeliveryJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, unattendedProject)
	unattendedHook := runConfiguredHumanTurnHook(t, binaryPath, unattendedProject, []byte(`{"prompt":"Approve"}`), []string{"AIDLC_UNATTENDED=1"})
	if unattendedHook.exitCode != 0 || unattendedHook.stdout.Len() != 0 || unattendedHook.stderr.Len() != 0 {
		t.Fatalf("unattended hook = code %d stdout %q stderr %q, want silent success", unattendedHook.exitCode, unattendedHook.stdout.String(), unattendedHook.stderr.String())
	}
	unattendedAwaiting := runReportBinary(t, binaryPath, unattendedProject, nil,
		"report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if unattendedAwaiting.kind != "print" {
		t.Fatalf("unattended awaiting report = %#v, want print", unattendedAwaiting)
	}
	unattendedApproval := runReportBinary(t, binaryPath, unattendedProject, nil,
		"report", "--stage", "intent-capture", "--result", "approved", "--user-input", "Approve")
	if unattendedApproval.kind != "error" || !strings.Contains(unattendedApproval.message, "HUMAN_TURN") {
		t.Fatalf("unattended approval = %#v, want stale HUMAN_TURN error", unattendedApproval)
	}
}

func assertNoCodexHookConfig(t *testing.T, project string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(project, ".codex", "hooks.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("fresh project hooks.json stat = %v, want absent", err)
	}
}

func installCodexHookConfig(t *testing.T, moduleRoot, project string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(moduleRoot, "src", "harness", "codex", "hooks.json"))
	if err != nil {
		t.Fatalf("ReadFile(source hooks.json): %v", err)
	}
	configDir := filepath.Join(project, ".codex")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%s): %v", configDir, err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "hooks.json"), content, 0o600); err != nil {
		t.Fatalf("WriteFile(project hooks.json): %v", err)
	}
}

func configuredUserPromptSubmitCommand(t *testing.T, project string) []string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(project, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("ReadFile(project hooks.json): %v", err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("Unmarshal(project hooks.json): %v", err)
	}
	entries := document.Hooks["UserPromptSubmit"]
	if len(entries) != 1 || len(entries[0].Hooks) != 1 {
		t.Fatalf("UserPromptSubmit hooks = %#v, want one command hook", entries)
	}
	hook := entries[0].Hooks[0]
	if hook.Type != "command" {
		t.Fatalf("UserPromptSubmit hook type = %q, want command", hook.Type)
	}
	parts := strings.Fields(hook.Command)
	if len(parts) != 2 || parts[0] != "aidlc" || parts[1] != "__codex-user-prompt-submit" {
		t.Fatalf("UserPromptSubmit command = %q, want fixed PATH aidlc command", hook.Command)
	}
	return parts
}

func runConfiguredHumanTurnHook(t *testing.T, binaryPath, project string, stdin []byte, extraEnv []string) reportBinaryResult {
	t.Helper()
	commandParts := configuredUserPromptSubmitCommand(t, project)
	pathDir := t.TempDir()
	pathAidlc := filepath.Join(pathDir, "aidlc")
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile(built aidlc): %v", err)
	}
	if err := os.WriteFile(pathAidlc, binary, 0o700); err != nil {
		t.Fatalf("WriteFile(PATH aidlc): %v", err)
	}
	pathValue := pathDir + string(os.PathListSeparator) + os.Getenv("PATH")
	resolved, err := resolveConfiguredPathCommand(commandParts[0], pathValue)
	if err != nil {
		t.Fatalf("resolve configured hook command %q: %v", commandParts[0], err)
	}
	// exec.Command resolves the executable using the test process's PATH
	// before Env is applied. Resolve the fixed command into the first PATH
	// entry explicitly, then retain that PATH for the child process.
	command := exec.Command(resolved, commandParts[1:]...)
	command.Dir = project
	command.Env = appendWithEnv(os.Environ(), append([]string{"PATH=" + pathValue}, extraEnv...)...)
	command.Stdin = bytes.NewReader(stdin)
	var result reportBinaryResult
	command.Stdout = &result.stdout
	command.Stderr = &result.stderr
	err = command.Run()
	if err == nil {
		result.exitCode = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
	} else {
		t.Fatalf("run configured hook %q: %v", commandParts, err)
	}
	return result
}

func resolveConfiguredPathCommand(name, pathValue string) (string, error) {
	if name == "" || strings.ContainsRune(name, os.PathSeparator) {
		return "", fmt.Errorf("unsafe command name %q", name)
	}
	for _, directory := range strings.Split(pathValue, string(os.PathListSeparator)) {
		if directory == "" {
			continue
		}
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("command %q is not present in PATH", name)
}

func appendWithEnv(base []string, overrides ...string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			result = append(result, entry)
			continue
		}
		replaced := false
		for _, override := range overrides {
			overrideName, _, hasValue := strings.Cut(override, "=")
			if hasValue && name == overrideName {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, entry)
		}
	}
	return append(result, overrides...)
}

type reportBinaryResult struct {
	exitCode int
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	kind     string
	message  string
	reason   string
}

func runReportBinary(t *testing.T, binaryPath, project string, stdin []byte, args ...string) reportBinaryResult {
	return runReportBinaryWithEnv(t, binaryPath, project, stdin, nil, args...)
}

func runReportBinaryWithEnv(t *testing.T, binaryPath, project string, stdin []byte, extraEnv []string, args ...string) reportBinaryResult {
	t.Helper()
	command := exec.Command(binaryPath, args...)
	command.Dir = project
	command.Env = append(os.Environ(), extraEnv...)
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var result reportBinaryResult
	command.Stdout = &result.stdout
	command.Stderr = &result.stderr
	err := command.Run()
	if err == nil {
		result.exitCode = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
	} else {
		t.Fatalf("run %q: %v", args, err)
	}
	if result.stdout.Len() != 0 {
		if strings.Count(result.stdout.String(), "\n") != 1 {
			t.Fatalf("run %q stdout = %q, want one JSON line", args, result.stdout.String())
		}
		var wire struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			Reason  string `json:"reason"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(result.stdout.Bytes()), &wire); err != nil {
			t.Fatalf("run %q JSON: %v; stdout=%q", args, err, result.stdout.String())
		}
		result.kind, result.message, result.reason = wire.Kind, wire.Message, wire.Reason
	}
	return result
}

func TestDeliveryOperationCloseFailureIsInternal(t *testing.T) {
	rootPath := t.TempDir()
	projectRoot, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordPath := t.TempDir()
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	previousResolver := deliveryInputResolver
	deliveryInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliverypkg.RunStageInput{Identity: recordlock.Identity{}, ProjectRoot: projectRoot, RecordRoot: recordRoot}, projectRoot, recordRoot, nil
	}
	previousCloser := deliveryRootCloser
	deliveryRootCloser = func(root *os.Root) error {
		if root == recordRoot {
			return errors.New("injected root close failure")
		}
		return previousCloser(root)
	}
	defer func() {
		deliveryInputResolver = previousResolver
		deliveryRootCloser = previousCloser
	}()

	wire, err := runDeliveryOperation(nil, nil, "", func(context.Context, deliverypkg.RunStageInput) (deliverypkg.DeliveryResult, error) {
		return deliverypkg.DeliveryResult{}, &deliverypkg.WorkflowError{Message: "continuation is stale"}
	})
	if wire != nil {
		t.Errorf("runDeliveryOperation() wire = %q, want nil", wire)
	}
	if err == nil {
		t.Fatal("runDeliveryOperation() error = nil, want cleanup failure")
	}
	if deliverypkg.IsWorkflowError(err) {
		t.Errorf("runDeliveryOperation() error = %v, must not remain a workflow error when cleanup fails", err)
	}
	if !strings.Contains(err.Error(), "injected root close failure") {
		t.Errorf("runDeliveryOperation() error = %v, want injected cleanup cause", err)
	}
}

type deliveryBinaryResult struct {
	exitCode int
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	kind     string
	token    string
	parts    int
}

func runDeliveryBinary(t *testing.T, binaryPath, project string, args ...string) deliveryBinaryResult {
	t.Helper()
	commandArgs := append([]string{}, args...)
	commandArgs = append(commandArgs, "--project-dir", project)
	command := exec.Command(binaryPath, commandArgs...)
	var result deliveryBinaryResult
	command.Stdout = &result.stdout
	command.Stderr = &result.stderr
	err := command.Run()
	if err == nil {
		result.exitCode = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
	} else {
		t.Fatalf("run %q: %v", commandArgs, err)
	}
	if result.stdout.Len() > 0 {
		if strings.Count(result.stdout.String(), "\n") != 1 {
			t.Fatalf("run %q stdout = %q, want one JSON line", commandArgs, result.stdout.String())
		}
		var wire struct {
			Kind          string `json:"kind"`
			ContinueToken string `json:"continue_token"`
			Parts         int    `json:"parts"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(result.stdout.Bytes()), &wire); err != nil {
			t.Fatalf("run %q JSON: %v; stdout=%q", commandArgs, err, result.stdout.String())
		}
		result.kind, result.token, result.parts = wire.Kind, wire.ContinueToken, wire.Parts
	}
	return result
}

func newDeliveryJourneyProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	dataDir := filepath.Join(project, ".codex", "tools", "data")
	recordDir := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	for _, directory := range []string{
		dataDir,
		recordDir,
		filepath.Join(project, "aidlc", "spaces", "team", "knowledge"),
		filepath.Join(project, "aidlc", "spaces", "team", "memory"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", directory, err)
		}
	}
	writeDeliveryJourneyFile(t, filepath.Join(project, "aidlc", "active-space"), "team\n")
	writeDeliveryJourneyFile(t, filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"), `[{"uuid":"delivery-journey-intent","slug":"delivery-journey","status":"planning","dirName":"build"}]`)
	writeDeliveryJourneyFile(t, filepath.Join(recordDir, "active-intent"), "build\n")
	writeDeliveryJourneyFile(t, filepath.Join(dataDir, "scope-grid.json"), deliveryJourneyScopeGridJSON)
	graphJSON := strings.Replace(deliveryJourneyGraphJSON,
		`"consumes":[]`, `"consumes":[],"rules_in_context":[{"path":"delivery-rule.md","scope":"project"}]`, 2)
	writeDeliveryJourneyFile(t, filepath.Join(dataDir, "stage-graph.json"), graphJSON)
	writeDeliveryJourneyFile(t, filepath.Join(project, "delivery-rule.md"), strings.Repeat("delivery rule content\n", 3000))
	catalog, err := graph.Load(os.DirFS(dataDir))
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     catalog,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               project,
		ProjectDescription:        "delivery journey",
		ProjectDescriptionPreview: "delivery journey",
		StartDate:                 "2026-09-05T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	writeDeliveryJourneyFile(t, filepath.Join(recordDir, "aidlc-state.md"), initial.StateContent)
	return project
}

func writeDeliveryJourneyFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func deliveryModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if root == "" {
		t.Fatal(fmt.Errorf("module root is empty"))
	}
	return root
}
