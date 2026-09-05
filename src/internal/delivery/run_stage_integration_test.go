package delivery

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/state"
)

const runStageIntegrationFreshnessGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"rules_in_context":[{"path":"prefix/memory/rule.md","scope":"team"}],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

func TestComposeRunStageIntegrationFreshness(t *testing.T) {
	fixture := newRunStageFixture(t)
	writeRunStageFile(t, fixture.stageGraphPath, runStageIntegrationFreshnessGraphJSON)
	writeRunStageFile(t, filepath.Join(filepath.Dir(fixture.stageGraphPath), "scope-grid.json"), runStageScopeGridJSON)

	projectDir := fixture.identity.ProjectPath()
	memoryDir := filepath.Join(projectDir, "aidlc", "spaces", fixture.identity.Space(), "memory")
	spaceKnowledgeDir := filepath.Join(projectDir, "aidlc", "spaces", fixture.identity.Space(), "knowledge")
	frameworkDir := filepath.Join(projectDir, ".codex")
	for _, directory := range []string{
		memoryDir,
		spaceKnowledgeDir,
		filepath.Join(frameworkDir, "agents"),
		filepath.Join(frameworkDir, "knowledge", "aidlc-shared"),
		filepath.Join(frameworkDir, "knowledge", "aidlc-product-agent"),
		filepath.Join(frameworkDir, "knowledge", "aidlc-architect-agent"),
		filepath.Join(spaceKnowledgeDir, "aidlc-shared"),
		filepath.Join(spaceKnowledgeDir, "aidlc-product-agent"),
		filepath.Join(spaceKnowledgeDir, "aidlc-architect-agent"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", directory, err)
		}
	}
	rulePath := filepath.Join(memoryDir, "rule.md")
	writeRunStageFile(t, rulePath, "ルール初版 🚀\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "agents", "aidlc-product-agent.md"), "product persona\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "agents", "aidlc-architect-agent.md"), "architect persona\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "base.md"), "shared framework knowledge\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-product-agent", "product.md"), "product framework knowledge\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-architect-agent", "architect.md"), "architect framework knowledge\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-shared", "base.md"), "shared space knowledge\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-product-agent", "product.md"), "product space knowledge\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-architect-agent", "architect.md"), "architect space knowledge\n")

	statePath := filepath.Join(projectDir, "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), "aidlc-state.md")
	addedKnowledgePath := filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "added.md")
	input := RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	}
	trackedPaths := []string{
		fixture.stageGraphPath,
		filepath.Join(filepath.Dir(fixture.stageGraphPath), "scope-grid.json"),
		fixture.activeSpacePath,
		fixture.activeIntentPath,
		statePath,
		rulePath,
		filepath.Join(frameworkDir, "agents", "aidlc-product-agent.md"),
		filepath.Join(frameworkDir, "agents", "aidlc-architect-agent.md"),
		filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "base.md"),
		filepath.Join(spaceKnowledgeDir, "aidlc-shared", "base.md"),
		addedKnowledgePath,
	}
	compose := func(label string) RunStageComposition {
		t.Helper()
		before := snapshotRunStageInputs(t, trackedPaths)
		got, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(%s) error = %v, want nil", label, err)
		}
		after := snapshotRunStageInputs(t, trackedPaths)
		if !reflect.DeepEqual(after, before) {
			t.Errorf("ComposeRunStage(%s) changed tracked input contents or modes", label)
		}
		if _, err := fixture.projectRoot.Stat("."); err != nil {
			t.Errorf("ComposeRunStage(%s) closed caller project root: %v", label, err)
		}
		if _, err := fixture.recordRoot.Stat("."); err != nil {
			t.Errorf("ComposeRunStage(%s) closed caller record root: %v", label, err)
		}
		return got
	}

	first := compose("initial")
	if len(first.Rules) != 1 || len(first.Chunks) == 0 || first.Bundle == "" {
		t.Fatalf("initial rule composition = rules %#v chunks %#v bundle %q, want one fresh rule bundle", first.Rules, first.Chunks, first.Bundle)
	}
	if first.Freshness.Bundle != first.Bundle || first.Claims.Bundle != first.Bundle {
		t.Errorf("initial bundle bindings = freshness %q claims %q bundle %q", first.Freshness.Bundle, first.Claims.Bundle, first.Bundle)
	}

	writeRunStageFile(t, rulePath, "ルール更新版 日本語 🚀\n")
	second := compose("updated rule")
	if reflect.DeepEqual(second.Rules, first.Rules) || reflect.DeepEqual(second.Chunks, first.Chunks) {
		t.Errorf("updated rule composition did not refresh Rules/Chunks: first=%#v second=%#v", first.Rules, second.Rules)
	}
	if second.Bundle == first.Bundle || second.Freshness.Bundle == first.Freshness.Bundle || second.Claims.Bundle == first.Claims.Bundle {
		t.Errorf("updated rule composition did not refresh bundle bindings: first=%q/%q/%q second=%q/%q/%q", first.Bundle, first.Freshness.Bundle, first.Claims.Bundle, second.Bundle, second.Freshness.Bundle, second.Claims.Bundle)
	}
	if first.Freshness.DirectiveHash != second.Freshness.DirectiveHash {
		t.Errorf("rule-only update changed DirectiveHash = %q -> %q, want wire-independent rule update", first.Freshness.DirectiveHash, second.Freshness.DirectiveHash)
	}

	writeRunStageFile(t, addedKnowledgePath, "追加された知識\n")
	third := compose("added knowledge")
	if reflect.DeepEqual(third.Wire, second.Wire) {
		t.Errorf("added knowledge did not refresh wire")
	}
	if third.Freshness.DirectiveHash == second.Freshness.DirectiveHash {
		t.Errorf("added knowledge did not refresh DirectiveHash = %q", third.Freshness.DirectiveHash)
	}
	if !strings.Contains(string(third.Wire), "added.md") {
		t.Errorf("added knowledge wire = %q, want added.md roster path", third.Wire)
	}

	graphWithRouteChange := strings.Replace(
		runStageIntegrationFreshnessGraphJSON,
		`{"slug":"intent-capture","number":"1.1"`,
		`{"route_annotation":"route-v2","slug":"intent-capture","number":"1.1"`,
		1,
	)
	if graphWithRouteChange == runStageIntegrationFreshnessGraphJSON {
		t.Fatal("route-change fixture did not add unknown raw graph property")
	}
	writeRunStageFile(t, fixture.stageGraphPath, graphWithRouteChange)
	fourth := compose("updated graph route metadata")
	if fourth.Freshness.RouteHash == third.Freshness.RouteHash || fourth.Claims.RouteHash == third.Claims.RouteHash {
		t.Errorf("raw graph metadata update did not refresh route hash: first=%q/%q second=%q/%q", third.Freshness.RouteHash, third.Claims.RouteHash, fourth.Freshness.RouteHash, fourth.Claims.RouteHash)
	}

	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", statePath, err)
	}
	updatedState := append(append([]byte(nil), stateBytes...), []byte("\n## Freshness Marker\nchanged without routing\n")...)
	if _, err := state.Parse(updatedState); err != nil {
		t.Fatalf("state.Parse(updated state fixture): %v", err)
	}
	if err := os.WriteFile(statePath, updatedState, 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", statePath, err)
	}
	fifth := compose("updated valid state bytes")
	if fifth.Freshness.StateHash == fourth.Freshness.StateHash || fifth.Claims.StateHash == fourth.Claims.StateHash {
		t.Errorf("valid state byte update did not refresh state hash: first=%v/%v second=%v/%v", fourth.Freshness.StateHash, fourth.Claims.StateHash, fifth.Freshness.StateHash, fifth.Claims.StateHash)
	}
}

type runStageInputSnapshot struct {
	Exists  bool
	Mode    fs.FileMode
	Content []byte
}

func snapshotRunStageInputs(t *testing.T, paths []string) map[string]runStageInputSnapshot {
	t.Helper()
	snapshot := make(map[string]runStageInputSnapshot, len(paths))
	for _, name := range paths {
		info, err := os.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			snapshot[name] = runStageInputSnapshot{}
			continue
		}
		if err != nil {
			t.Fatalf("Lstat(%q): %v", name, err)
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", name, err)
		}
		snapshot[name] = runStageInputSnapshot{
			Exists:  true,
			Mode:    info.Mode(),
			Content: content,
		}
	}
	return snapshot
}
