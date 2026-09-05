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
