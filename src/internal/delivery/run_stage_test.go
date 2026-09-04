package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/steering"
)

const runStageGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const runStageGraphWithReviewerJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"reviewer":"reviewer","produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const runStageScopeGridJSON = `{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE","next-stage":"EXECUTE"}}}`

const runStageWireGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const runStageRulesGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"rules_in_context":[{"path":"prefix/memory/alpha.md","scope":"org"},{"path":"project-rule.md","scope":"project"},{"path":"prefix/memory/beta.md","scope":"team"}],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

func TestComposeRunStageBuildsFreshRequiredRuleBundle(t *testing.T) {
	fixture := newRunStageFixture(t)
	writeRunStageFile(t, fixture.stageGraphPath, runStageRulesGraphJSON)

	projectDir := fixture.identity.ProjectPath()
	memoryDir := filepath.Join(projectDir, "aidlc", "spaces", fixture.identity.Space(), "memory")
	if err := os.MkdirAll(memoryDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", memoryDir, err)
	}
	alphaPath := filepath.Join(memoryDir, "alpha.md")
	betaPath := filepath.Join(memoryDir, "beta.md")
	projectRulePath := filepath.Join(projectDir, "project-rule.md")
	alphaText := "組織ルール 🚀\n"
	projectRuleText := "プロジェクト rule\n"
	betaText := "空間ルール 日本語\n"
	writeRunStageFile(t, alphaPath, alphaText)
	writeRunStageFile(t, projectRulePath, projectRuleText)
	writeRunStageFile(t, betaPath, betaText)

	var stage graph.Stage
	catalog, err := graph.Load(os.DirFS(filepath.Dir(fixture.stageGraphPath)))
	if err != nil {
		t.Fatalf("graph.Load(rules fixture): %v", err)
	}
	for _, candidate := range catalog.Stages() {
		if candidate.Slug == "intent-capture" {
			stage = candidate
			break
		}
	}
	if stage.Slug == "" {
		t.Fatal("rules fixture current stage is absent")
	}
	resolved, err := steering.ResolveRulePaths(projectDir, fixture.identity.Space(), "", stage.RulesInContext)
	if err != nil {
		t.Fatalf("steering.ResolveRulePaths(rules fixture): %v", err)
	}
	if len(resolved.Entries) != 3 {
		t.Fatalf("steering.ResolveRulePaths() entries = %d, want 3", len(resolved.Entries))
	}
	wantPaths := make([]string, len(resolved.Entries))
	for index, entry := range resolved.Entries {
		wantPaths[index] = entry.Path
	}
	wantRules := []steering.RuleContent{
		{Path: wantPaths[0], Text: alphaText},
		{Path: wantPaths[1], Text: projectRuleText},
		{Path: wantPaths[2], Text: betaText},
	}
	wantBundle, err := steering.BundleDigest(wantRules)
	if err != nil {
		t.Fatalf("steering.BundleDigest(first): %v", err)
	}
	wantChunks := steering.ChunkRules(wantRules)

	input := RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	}
	first, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(first rules read) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(first.Rules, wantRules) {
		t.Errorf("ComposeRunStage(first).Rules = %#v, want %#v", first.Rules, wantRules)
	}
	if first.Bundle != wantBundle {
		t.Errorf("ComposeRunStage(first).Bundle = %q, want %q", first.Bundle, wantBundle)
	}
	if !reflect.DeepEqual(first.Chunks, wantChunks) {
		t.Errorf("ComposeRunStage(first).Chunks = %#v, want %#v", first.Chunks, wantChunks)
	}
	assertRunStageRulesInContext(t, "first", first.Wire, wantPaths)

	updatedAlphaText := "更新済み組織ルール 🚀\n"
	writeRunStageFile(t, alphaPath, updatedAlphaText)
	updatedRules := []steering.RuleContent{
		{Path: wantPaths[0], Text: updatedAlphaText},
		{Path: wantPaths[1], Text: projectRuleText},
		{Path: wantPaths[2], Text: betaText},
	}
	updatedBundle, err := steering.BundleDigest(updatedRules)
	if err != nil {
		t.Fatalf("steering.BundleDigest(updated): %v", err)
	}
	updatedChunks := steering.ChunkRules(updatedRules)
	second, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(updated rules read) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(second.Rules, updatedRules) {
		t.Errorf("ComposeRunStage(updated).Rules = %#v, want %#v", second.Rules, updatedRules)
	}
	if second.Bundle != updatedBundle {
		t.Errorf("ComposeRunStage(updated).Bundle = %q, want %q", second.Bundle, updatedBundle)
	}
	if !reflect.DeepEqual(second.Chunks, updatedChunks) {
		t.Errorf("ComposeRunStage(updated).Chunks = %#v, want %#v", second.Chunks, updatedChunks)
	}
	if reflect.DeepEqual(second.Rules, first.Rules) || second.Bundle == first.Bundle || reflect.DeepEqual(second.Chunks, first.Chunks) {
		t.Error("ComposeRunStage(updated) did not refresh rule content, bundle, and chunks")
	}
	assertRunStageRulesInContext(t, "updated", second.Wire, wantPaths)

	if err := os.Remove(betaPath); err != nil {
		t.Fatalf("Remove(required rule): %v", err)
	}
	missing, err := ComposeRunStage(context.Background(), input)
	if err == nil {
		t.Error("ComposeRunStage(missing required rule) error = nil, want rule read error")
	}
	assertZeroRunStageComposition(t, "missing required rule", missing)

	writeRunStageFile(t, betaPath, betaText)
	if err := os.WriteFile(alphaPath, []byte{0xff}, 0o600); err != nil {
		t.Fatalf("WriteFile(invalid UTF-8 rule): %v", err)
	}
	invalid, err := ComposeRunStage(context.Background(), input)
	if err == nil {
		t.Error("ComposeRunStage(invalid UTF-8 rule) error = nil, want rule read error")
	}
	assertZeroRunStageComposition(t, "invalid UTF-8 rule", invalid)
}

func assertRunStageRulesInContext(t *testing.T, label string, data []byte, want []string) {
	t.Helper()
	var wire struct {
		RulesInContext []string `json:"rules_in_context"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Errorf("ComposeRunStage(%s) wire unmarshal error = %v", label, err)
		return
	}
	if !reflect.DeepEqual(wire.RulesInContext, want) {
		t.Errorf("ComposeRunStage(%s) wire rules_in_context = %#v, want %#v", label, wire.RulesInContext, want)
	}
}

func TestComposeRunStageBuildsFreshKnowledgeRoster(t *testing.T) {
	fixture := newRunStageFixture(t)
	dataDir := filepath.Dir(fixture.stageGraphPath)
	writeRunStageFile(t, fixture.stageGraphPath, runStageKnowledgeGraphJSON("inline"))

	stateBytes, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatalf("ReadFile(aidlc-state.md): %v", err)
	}
	minimalState := strings.Replace(string(stateBytes), "- **Depth**: Standard\n", "- **Depth**: Minimal\n", 1)
	if minimalState == string(stateBytes) {
		t.Fatal("fixture state did not contain canonical Standard depth")
	}
	writeRunStageFile(t, filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), "aidlc-state.md"), minimalState)

	projectDir := fixture.identity.ProjectPath()
	frameworkDir := filepath.Join(projectDir, ".codex")
	spaceKnowledgeDir := filepath.Join(projectDir, "aidlc", "spaces", fixture.identity.Space(), "knowledge")
	for _, directory := range []string{
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
	productPersonaPath := filepath.Join(frameworkDir, "agents", "aidlc-product-agent.md")
	architectPersonaPath := filepath.Join(frameworkDir, "agents", "aidlc-architect-agent.md")
	writeRunStageFile(t, productPersonaPath, "product persona 日本語\n")
	if err := os.WriteFile(architectPersonaPath, []byte{0xff}, 0o600); err != nil {
		t.Fatalf("WriteFile(invalid support persona): %v", err)
	}
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "verification.md"), "verification\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-shared", "audit-format.md"), "not selected at Minimal depth\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-product-agent", "requirements-elicitation.md"), "product knowledge\n")
	writeRunStageFile(t, filepath.Join(frameworkDir, "knowledge", "aidlc-architect-agent", "architecture-guide.md"), "architect knowledge\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-shared", "space.md"), "space shared knowledge 日本語\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-product-agent", "space.md"), "space product knowledge\n")
	writeRunStageFile(t, filepath.Join(spaceKnowledgeDir, "aidlc-architect-agent", "space.md"), "space architect knowledge\n")

	input := RunStageInput{
		Identity:       fixture.identity,
		ProjectRoot:    fixture.projectRoot,
		RecordRoot:     fixture.recordRoot,
		EnabledPlugins: []string{"example-plugin"},
	}
	depth, err := state.Depth(stateBytesForRunStage(t, fixture.recordRoot))
	if err != nil {
		t.Fatalf("state.Depth(fixture): %v", err)
	}
	if depth != "Minimal" {
		t.Fatalf("state.Depth(fixture) = %q, want Minimal", depth)
	}

	loadStage := func(label string) graph.Stage {
		t.Helper()
		catalog, err := graph.Load(os.DirFS(dataDir))
		if err != nil {
			t.Fatalf("graph.Load(%s): %v", label, err)
		}
		for _, candidate := range catalog.Stages() {
			if candidate.Slug == "intent-capture" {
				return candidate
			}
		}
		t.Fatalf("graph.Load(%s) did not return intent-capture", label)
		return graph.Stage{}
	}
	buildWantRoster := func(label string, stage graph.Stage) knowledge.Roster {
		t.Helper()
		projectFS := fixture.projectRoot.FS()
		frameworkFS, err := fs.Sub(projectFS, ".codex")
		if err != nil {
			t.Fatalf("fs.Sub(framework, %s): %v", label, err)
		}
		spaceFS, err := fs.Sub(projectFS, path.Join("aidlc", "spaces", fixture.identity.Space(), "knowledge"))
		if err != nil {
			t.Fatalf("fs.Sub(space knowledge, %s): %v", label, err)
		}
		roster, err := knowledge.BuildRoster(knowledge.RosterInput{
			Stage:        stage,
			Depth:        depth,
			Framework:    knowledge.Source{FS: frameworkFS, DisplayPrefix: ".codex"},
			FrameworkDir: frameworkDir,
			SpaceKnowledge: &knowledge.Source{
				FS:            spaceFS,
				DisplayPrefix: path.Join("aidlc", "spaces", fixture.identity.Space(), "knowledge"),
			},
			EnabledPlugins: input.EnabledPlugins,
		})
		if err != nil {
			t.Fatalf("knowledge.BuildRoster(%s): %v", label, err)
		}
		return roster
	}
	parseWire := func(label string, data []byte) ([]string, []string) {
		t.Helper()
		var wire struct {
			InlineContextPaths []string `json:"inline_context_paths"`
			ContextWarnings    []string `json:"context_warnings"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Errorf("ComposeRunStage(%s) wire unmarshal error = %v", label, err)
			return nil, nil
		}
		return wire.InlineContextPaths, wire.ContextWarnings
	}

	inlineStage := loadStage("initial inline")
	initialWant := buildWantRoster("initial inline", inlineStage)
	if len(initialWant.Warnings) == 0 {
		t.Fatal("knowledge.BuildRoster(initial inline) warnings = empty, want missing/invalid optional warning")
	}
	if len(initialWant.Paths) == 0 {
		t.Fatal("knowledge.BuildRoster(initial inline) paths = empty, want available knowledge")
	}
	if !containsRunStagePath(initialWant.Paths, ".codex/knowledge/aidlc-shared/verification.md") {
		t.Errorf("Minimal roster paths = %#v, want known shipped verification.md", initialWant.Paths)
	}
	if containsRunStagePath(initialWant.Paths, ".codex/knowledge/aidlc-shared/audit-format.md") {
		t.Errorf("Minimal roster paths = %#v, did not expect audit-format.md", initialWant.Paths)
	}
	first, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(initial knowledge) error = %v, want nil", err)
	}
	firstPaths, firstWarnings := parseWire("initial knowledge", first.Wire)
	if !reflect.DeepEqual(firstPaths, initialWant.Paths) {
		t.Errorf("ComposeRunStage(initial).inline_context_paths = %#v, want %#v", firstPaths, initialWant.Paths)
	}
	if !reflect.DeepEqual(firstWarnings, initialWant.Warnings) {
		t.Errorf("ComposeRunStage(initial).context_warnings = %#v, want %#v", firstWarnings, initialWant.Warnings)
	}
	stageFileIndex := strings.Index(string(first.Wire), `"stage_file"`)
	warningsIndex := strings.Index(string(first.Wire), `"context_warnings"`)
	if stageFileIndex < 0 || warningsIndex <= stageFileIndex {
		t.Errorf("ComposeRunStage(initial) wire warning order = %q, want context_warnings after stage_file", first.Wire)
	}

	writeRunStageFile(t, architectPersonaPath, "architect persona 日本語\n")
	writeRunStageFile(t, productPersonaPath, "product persona 更新 🚀\n")
	secondWant := buildWantRoster("updated inline", inlineStage)
	if len(secondWant.Warnings) != 0 {
		t.Fatalf("knowledge.BuildRoster(updated inline) warnings = %#v, want empty", secondWant.Warnings)
	}
	second, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(updated knowledge) error = %v, want nil", err)
	}
	secondPaths, secondWarnings := parseWire("updated knowledge", second.Wire)
	if !reflect.DeepEqual(secondPaths, secondWant.Paths) {
		t.Errorf("ComposeRunStage(updated).inline_context_paths = %#v, want %#v", secondPaths, secondWant.Paths)
	}
	if len(secondWarnings) != 0 {
		t.Errorf("ComposeRunStage(updated).context_warnings = %#v, want empty", secondWarnings)
	}
	if reflect.DeepEqual(firstPaths, secondPaths) || reflect.DeepEqual(firstWarnings, secondWarnings) {
		t.Error("ComposeRunStage(updated) did not refresh paths and warnings")
	}
	if strings.Contains(string(second.Wire), `"context_warnings"`) {
		t.Errorf("ComposeRunStage(updated) wire = %q, want empty context_warnings field omitted", second.Wire)
	}

	writeRunStageFile(t, fixture.stageGraphPath, runStageKnowledgeGraphJSON("mob"))
	mobStage := loadStage("mob")
	mobWant := buildWantRoster("mob", mobStage)
	mob, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(mob knowledge) error = %v, want nil", err)
	}
	mobPaths, mobWarnings := parseWire("mob knowledge", mob.Wire)
	if !reflect.DeepEqual(mobPaths, mobWant.Paths) || len(mobWarnings) != 0 {
		t.Errorf("ComposeRunStage(mob) roster = paths %#v warnings %#v, want paths %#v warnings %#v", mobPaths, mobWarnings, mobWant.Paths, mobWant.Warnings)
	}
	if containsRunStagePath(mobPaths, ".codex/agents/aidlc-architect-agent.md") {
		t.Errorf("ComposeRunStage(mob) paths = %#v, want lead persona only", mobPaths)
	}

	writeRunStageFile(t, fixture.stageGraphPath, runStageKnowledgeGraphJSON("subagent"))
	subagentStage := loadStage("subagent")
	subagentWant := buildWantRoster("subagent", subagentStage)
	if len(subagentWant.Paths) != 0 || len(subagentWant.Warnings) != 0 {
		t.Fatalf("knowledge.BuildRoster(subagent) = %#v, want empty dispatched roster", subagentWant)
	}
	subagent, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(subagent knowledge) error = %v, want nil", err)
	}
	subagentPaths, subagentWarnings := parseWire("subagent knowledge", subagent.Wire)
	if len(subagentPaths) != 0 || len(subagentWarnings) != 0 {
		t.Errorf("ComposeRunStage(subagent) roster = paths %#v warnings %#v, want empty", subagentPaths, subagentWarnings)
	}
}

func runStageKnowledgeGraphJSON(mode string) string {
	return `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"` + mode + `","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`
}

func stateBytesForRunStage(t *testing.T, recordRoot *os.Root) []byte {
	t.Helper()
	content, err := recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatalf("ReadFile(aidlc-state.md): %v", err)
	}
	return content
}

func containsRunStagePath(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

const runStageArtifactGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":["out-one","out-two"],"consumes":[{"artifact":"present-required","required":true},{"artifact":"present-optional","required":false},{"artifact":"missing-optional","required":false},{"artifact":"orphan-required","required":true},{"artifact":"directory-required","required":true},{"artifact":"symlink-required","required":true},{"artifact":"unbuilt-required","required":true}],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"source-producer","number":"1.3","name":"Source Producer","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":["present-required"],"optional_produces":["present-optional"],"consumes":[],"requires_stage":[]},
  {"slug":"unbuilt-producer","number":"1.4","name":"Unbuilt Producer","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":["unbuilt-required"],"consumes":[],"requires_stage":[]},
  {"slug":"out-of-scope-producer","number":"1.5","name":"Out of Scope Producer","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["other"],"enabled":true,"produces":["orphan-required","directory-required","symlink-required"],"consumes":[],"requires_stage":[]}
]`

const runStageArtifactScopeGridJSON = `{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE","next-stage":"EXECUTE","source-producer":"EXECUTE","unbuilt-producer":"EXECUTE","out-of-scope-producer":"SKIP"}}}`

func TestComposeRunStageRejectsAmbiguousSkippedProducer(t *testing.T) {
	fixture := newRunStageFixture(t)
	dataDir := filepath.Dir(fixture.stageGraphPath)
	writeRunStageFile(t, fixture.stageGraphPath, runStageArtifactGraphJSON)
	writeRunStageFile(t, filepath.Join(dataDir, "scope-grid.json"), runStageArtifactScopeGridJSON)

	catalog, err := graph.Load(os.DirFS(dataDir))
	if err != nil {
		t.Fatalf("graph.Load(ambiguous fixture): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     catalog,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               fixture.identity.ProjectRoot(),
		ProjectDescription:        "run-stage ambiguous fixture",
		ProjectDescriptionPreview: "run-stage ambiguous fixture",
		StartDate:                 "2026-09-05T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(ambiguous fixture): %v", err)
	}
	skippedState := strings.Replace(
		initial.StateContent,
		"- [ ] unbuilt-producer — EXECUTE",
		"- [S] unbuilt-producer — EXECUTE",
		1,
	)
	if skippedState == initial.StateContent {
		t.Fatal("ambiguous fixture did not find unbuilt producer row")
	}
	recordDir := filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", "team", "intents", "build")
	writeRunStageFile(t, filepath.Join(recordDir, "aidlc-state.md"), skippedState)

	got, err := ComposeRunStage(context.Background(), RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	})
	if err == nil {
		t.Error("ComposeRunStage(ambiguous skipped producer) error = nil, want ErrUnsupportedConsumeProvenance")
	} else if !errors.Is(err, ErrUnsupportedConsumeProvenance) {
		t.Errorf("ComposeRunStage(ambiguous skipped producer) error = %v, want ErrUnsupportedConsumeProvenance", err)
	}
	assertZeroRunStageComposition(t, "ambiguous skipped producer", got)
}

func TestComposeRunStageClassifiesArtifactPresence(t *testing.T) {
	fixture := newRunStageFixture(t)
	dataDir := filepath.Dir(fixture.stageGraphPath)
	writeRunStageFile(t, fixture.stageGraphPath, runStageArtifactGraphJSON)
	writeRunStageFile(t, filepath.Join(dataDir, "scope-grid.json"), runStageArtifactScopeGridJSON)

	catalog, err := graph.Load(os.DirFS(dataDir))
	if err != nil {
		t.Fatalf("graph.Load(artifact fixture): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     catalog,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               fixture.identity.ProjectRoot(),
		ProjectDescription:        "run-stage artifact fixture",
		ProjectDescriptionPreview: "run-stage artifact fixture",
		StartDate:                 "2026-09-05T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(artifact fixture): %v", err)
	}
	recordDir := filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", "team", "intents", "build")
	writeRunStageFile(t, filepath.Join(recordDir, "aidlc-state.md"), initial.StateContent)

	sourceDir := filepath.Join(recordDir, "ideation", "source-producer")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", sourceDir, err)
	}
	writeRunStageFile(t, filepath.Join(sourceDir, "present-required.md"), "required\n")
	writeRunStageFile(t, filepath.Join(sourceDir, "present-optional.md"), "optional\n")

	outOfScopeDir := filepath.Join(recordDir, "ideation", "out-of-scope-producer")
	if err := os.MkdirAll(outOfScopeDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", outOfScopeDir, err)
	}
	if err := os.Mkdir(filepath.Join(outOfScopeDir, "directory-required.md"), 0o700); err != nil {
		t.Fatalf("Mkdir(directory-required.md): %v", err)
	}
	outsideDir := t.TempDir()
	outsideTarget := filepath.Join(outsideDir, "outside.md")
	writeRunStageFile(t, outsideTarget, "outside\n")
	if err := os.Symlink(outsideTarget, filepath.Join(outOfScopeDir, "symlink-required.md")); err != nil {
		t.Fatalf("Symlink(symlink-required.md): %v", err)
	}

	var current graph.Stage
	for _, stage := range catalog.Stages() {
		if stage.Slug == "intent-capture" {
			current = stage
			break
		}
	}
	if current.Slug == "" {
		t.Fatal("artifact fixture current stage is absent")
	}
	resolved, err := artifact.ResolvePaths(current, catalog, "Brownfield")
	if err != nil {
		t.Fatalf("artifact.ResolvePaths(artifact fixture): %v", err)
	}
	recordPrefix := path.Join("aidlc", "spaces", "team", "intents", "build")
	consumePath := func(name string) string {
		t.Helper()
		for _, input := range resolved.Consumes {
			if input.Artifact == name {
				return path.Join(recordPrefix, input.Path)
			}
		}
		t.Fatalf("artifact.ResolvePaths() did not resolve %q", name)
		return ""
	}
	if len(resolved.Produces) != 2 {
		t.Fatalf("artifact.ResolvePaths().Produces length = %d, want 2", len(resolved.Produces))
	}
	presentRequired := consumePath("present-required")
	presentOptional := consumePath("present-optional")
	orphanRequired := consumePath("orphan-required")
	directoryRequired := consumePath("directory-required")
	symlinkRequired := consumePath("symlink-required")
	unbuiltRequired := consumePath("unbuilt-required")
	producesOne := path.Join(recordPrefix, resolved.Produces[0])
	producesTwo := path.Join(recordPrefix, resolved.Produces[1])

	got, err := ComposeRunStage(context.Background(), RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	})
	if err != nil {
		t.Fatalf("ComposeRunStage(artifact fixture) error = %v, want nil", err)
	}
	want := `{"kind":"run-stage","stage":"intent-capture","phase":"ideation","lead_agent":"orchestrator","support_agents":[],"mode":"inline","inline_context_paths":[],"gate":true,"memory_path":"aidlc/spaces/team/intents/build/ideation/intent-capture/memory.md","consumes":["` + presentRequired + `","` + presentOptional + `"],"produces":["` + producesOne + `","` + producesTwo + `"],"rules_in_context":[],"sensors_applicable":[],"stage_file":".codex/aidlc-common/stages/ideation/intent-capture.md","consumes_absent":[{"path":"` + orphanRequired + `","expected":true},{"path":"` + directoryRequired + `","expected":true},{"path":"` + symlinkRequired + `","expected":true},{"path":"` + unbuiltRequired + `","expected":false}],"next_stage":"Next Stage","narration":"Starting the classic plan for this project. First step is Intent Capture, and I will stop for your review before anything is final."}`
	if string(got.Wire) != want {
		t.Errorf("ComposeRunStage() artifact wire = %q, want %q", got.Wire, want)
	}
}

func TestComposeRunStageBuildsCanonicalRequiredWire(t *testing.T) {
	fixture := newRunStageFixture(t)
	writeRunStageFile(t, fixture.stageGraphPath, runStageWireGraphJSON)

	got, err := ComposeRunStage(context.Background(), RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	})
	if err != nil {
		t.Fatalf("ComposeRunStage() error = %v, want nil", err)
	}

	want := `{"kind":"run-stage","stage":"intent-capture","phase":"ideation","lead_agent":"orchestrator","support_agents":[],"mode":"inline","inline_context_paths":[],"gate":true,"memory_path":"aidlc/spaces/team/intents/build/ideation/intent-capture/memory.md","consumes":[],"produces":[],"rules_in_context":[],"sensors_applicable":[],"stage_file":".codex/aidlc-common/stages/ideation/intent-capture.md","next_stage":"Next Stage","narration":"Starting the classic plan for this project. First step is Intent Capture, and I will stop for your review before anything is final."}`
	if string(got.Wire) != want {
		t.Errorf("ComposeRunStage() wire = %q, want %q", got.Wire, want)
	}
}

func TestComposeRunStageBuildsSupportedOptionalFields(t *testing.T) {
	t.Run("first substantive subagent", func(t *testing.T) {
		fixture := newRunStageFixture(t)
		writeRunStageFile(t, fixture.stageGraphPath, runStageKnowledgeGraphJSON("subagent"))

		conductorPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "conductor.md")
		if err := os.MkdirAll(filepath.Dir(conductorPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(conductorPath), err)
		}
		writeRunStageFile(t, conductorPath, "司令塔 persona 日本語 🚀\n")

		input := RunStageInput{
			Identity:    fixture.identity,
			ProjectRoot: fixture.projectRoot,
			RecordRoot:  fixture.recordRoot,
		}
		want := `{"kind":"run-stage","stage":"intent-capture","phase":"ideation","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"subagent","inline_context_paths":[],"gate":true,"memory_path":"aidlc/spaces/team/intents/build/ideation/intent-capture/memory.md","consumes":[],"produces":[],"rules_in_context":[],"sensors_applicable":[],"stage_file":".codex/aidlc-common/stages/ideation/intent-capture.md","next_stage":"Next Stage","protocol_modules":["ensemble"],"conductor_persona":"司令塔 persona 日本語 🚀\n","narration":"Bringing in the product manager to work on Intent Capture."}`
		first, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(first optional fields) error = %v, want nil", err)
		}
		if string(first.Wire) != want {
			t.Errorf("ComposeRunStage(first optional fields) wire = %q, want %q", first.Wire, want)
		}

		writeRunStageFile(t, conductorPath, "更新された司令塔 persona 日本語 🚀\n")
		updatedWant := strings.Replace(want, `"conductor_persona":"司令塔 persona 日本語 🚀\n"`, `"conductor_persona":"更新された司令塔 persona 日本語 🚀\n"`, 1)
		second, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(updated optional fields) error = %v, want nil", err)
		}
		if string(second.Wire) != updatedWant {
			t.Errorf("ComposeRunStage(updated optional fields) wire = %q, want %q", second.Wire, updatedWant)
		}
		if string(first.Wire) == string(second.Wire) {
			t.Error("ComposeRunStage(updated optional fields) did not refresh conductor persona")
		}

		if err := os.Remove(conductorPath); err != nil {
			t.Fatalf("Remove(conductor persona): %v", err)
		}
		withoutConductorWant := strings.Replace(updatedWant, `,"conductor_persona":"更新された司令塔 persona 日本語 🚀\n"`, "", 1)
		third, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(missing conductor persona) error = %v, want nil", err)
		}
		if string(third.Wire) != withoutConductorWant {
			t.Errorf("ComposeRunStage(missing conductor persona) wire = %q, want %q", third.Wire, withoutConductorWant)
		}
	})

	t.Run("terminal substantive stage", func(t *testing.T) {
		fixture := newRunStageFixture(t)
		terminalGraph := `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"subagent","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":false,"produces":[],"consumes":[],"requires_stage":[]}
]`
		writeRunStageFile(t, fixture.stageGraphPath, terminalGraph)
		statePath := filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), "aidlc-state.md")
		stateContent, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", statePath, err)
		}
		terminalState := strings.Replace(string(stateContent), "- **Next Stage**: next-stage\n", "- **Next Stage**: none\n", 1)
		if terminalState == string(stateContent) {
			t.Fatal("terminal fixture did not find next stage field")
		}
		writeRunStageFile(t, statePath, terminalState)

		got, err := ComposeRunStage(context.Background(), RunStageInput{
			Identity:    fixture.identity,
			ProjectRoot: fixture.projectRoot,
			RecordRoot:  fixture.recordRoot,
		})
		if err != nil {
			t.Fatalf("ComposeRunStage(terminal optional fields) error = %v, want nil", err)
		}
		var wire struct {
			NextStage json.RawMessage `json:"next_stage"`
		}
		if err := json.Unmarshal(got.Wire, &wire); err != nil {
			t.Fatalf("ComposeRunStage(terminal optional fields) wire unmarshal error = %v", err)
		}
		if string(wire.NextStage) != "null" {
			t.Errorf("ComposeRunStage(terminal optional fields) next_stage = %s, want null", wire.NextStage)
		}
	})
}

func TestRunStageNarrationMatchesCanonicalRolesAndPeopleClause(t *testing.T) {
	t.Run("specialist roles", func(t *testing.T) {
		cases := []struct {
			agent string
			want  string
		}{
			{agent: "aidlc-aws-platform-agent", want: "Bringing in the platform engineer to work on Intent Capture."},
			{agent: "aidlc-devsecops-agent", want: "Bringing in the security engineer to work on Intent Capture."},
			{agent: "aidlc-pipeline-deploy-agent", want: "Bringing in the release engineer to work on Intent Capture."},
			{agent: "aidlc-operations-agent", want: "Bringing in the operations engineer to work on Intent Capture."},
			{agent: "aidlc-strategy-agent", want: "Bringing in the strategy to work on Intent Capture."},
		}
		for _, tc := range cases {
			t.Run(tc.agent, func(t *testing.T) {
				got := runStageNarration(recordlock.Identity{}, graph.Stage{
					Name:      "Intent Capture",
					Phase:     "ideation",
					LeadAgent: tc.agent,
					Mode:      "subagent",
				}, state.State{}, graph.Snapshot{})
				if got != tc.want {
					t.Errorf("runStageNarration(%q) = %q, want %q", tc.agent, got, tc.want)
				}
			})
		}
	})

	t.Run("later inline and mob people clause", func(t *testing.T) {
		for _, mode := range []string{"inline", "mob"} {
			t.Run(mode, func(t *testing.T) {
				fixture := newRunStageFixture(t)
				writeRunStageFile(t, fixture.stageGraphPath, runStageNarrationGraphJSON(mode))

				statePath := filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), "aidlc-state.md")
				stateContent, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatalf("ReadFile(%q): %v", statePath, err)
				}
				laterState := strings.Replace(string(stateContent), "- [ ] next-stage — EXECUTE\n", "- [x] next-stage — EXECUTE\n", 1)
				if laterState == string(stateContent) {
					t.Fatal("later narration fixture did not find next-stage progress row")
				}
				writeRunStageFile(t, statePath, laterState)

				got, err := ComposeRunStage(context.Background(), RunStageInput{
					Identity:    fixture.identity,
					ProjectRoot: fixture.projectRoot,
					RecordRoot:  fixture.recordRoot,
				})
				if err != nil {
					t.Fatalf("ComposeRunStage(%s later narration) error = %v, want nil", mode, err)
				}
				var wire struct {
					Narration string `json:"narration"`
				}
				if err := json.Unmarshal(got.Wire, &wire); err != nil {
					t.Fatalf("ComposeRunStage(%s later narration) wire unmarshal error = %v", mode, err)
				}
				want := "Now working on Later Stage, wearing the product manager hat, with the architect and quality engineer on hand."
				if wire.Narration != want {
					t.Errorf("ComposeRunStage(%s) narration = %q, want %q", mode, wire.Narration, want)
				}
			})
		}
	})

	t.Run("canonical product lead role", func(t *testing.T) {
		got := runStageNarration(recordlock.Identity{}, graph.Stage{
			Name:      "Intent Capture",
			Phase:     "ideation",
			LeadAgent: "aidlc-product-lead-agent",
			Mode:      "subagent",
		}, state.State{}, graph.Snapshot{})
		want := "Bringing in the product lead to work on Intent Capture."
		if got != want {
			t.Errorf("runStageNarration(product lead) = %q, want %q", got, want)
		}
	})
}

func runStageNarrationGraphJSON(mode string) string {
	return `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Later Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent","aidlc-quality-agent"],"mode":"` + mode + `","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`
}

func TestComposeRunStageUsesFreshSelectionAndValidatedNext(t *testing.T) {
	fixture := newRunStageFixture(t)
	input := RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
	}

	initial, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Errorf("ComposeRunStage(valid selection) error = %v, want nil", err)
	} else {
		if initial.Directive.Kind != orchestrator.DirectiveKindRunStage {
			t.Errorf("ComposeRunStage(valid selection) kind = %q, want %q", initial.Directive.Kind, orchestrator.DirectiveKindRunStage)
		}
		if initial.Directive.Stage.Slug != "intent-capture" {
			t.Errorf("ComposeRunStage(valid selection) stage = %q, want intent-capture", initial.Directive.Stage.Slug)
		}
	}

	writeRunStageFile(t, fixture.activeSpacePath, "other\n")
	spaceMismatch, err := ComposeRunStage(context.Background(), input)
	if err == nil {
		t.Error("ComposeRunStage(active space changed) error = nil, want selection mismatch")
	} else if !errors.Is(err, ErrSelectionMismatch) {
		t.Errorf("ComposeRunStage(active space changed) error = %v, want ErrSelectionMismatch", err)
	}
	assertZeroRunStageComposition(t, "active space changed", spaceMismatch)

	writeRunStageFile(t, fixture.activeSpacePath, "team\n")
	writeRunStageFile(t, fixture.activeIntentPath, "other\n")
	intentMismatch, err := ComposeRunStage(context.Background(), input)
	if err == nil {
		t.Error("ComposeRunStage(active intent changed) error = nil, want selection mismatch")
	} else if !errors.Is(err, ErrSelectionMismatch) {
		t.Errorf("ComposeRunStage(active intent changed) error = %v, want ErrSelectionMismatch", err)
	}
	assertZeroRunStageComposition(t, "active intent changed", intentMismatch)

	writeRunStageFile(t, fixture.activeIntentPath, "build\n")
	writeRunStageFile(t, fixture.stageGraphPath, runStageGraphWithReviewerJSON)
	unsupported, err := ComposeRunStage(context.Background(), input)
	if err == nil {
		t.Error("ComposeRunStage(unsupported graph capability) error = nil, want ErrUnsupportedGate")
	} else if !errors.Is(err, orchestrator.ErrUnsupportedGate) {
		t.Errorf("ComposeRunStage(unsupported graph capability) error = %v, want ErrUnsupportedGate", err)
	}
	assertZeroRunStageComposition(t, "unsupported graph capability", unsupported)

	if _, err := fixture.projectRoot.Stat("."); err != nil {
		t.Errorf("ComposeRunStage closed caller project root: %v", err)
	}
	if _, err := fixture.recordRoot.Stat("."); err != nil {
		t.Errorf("ComposeRunStage closed caller record root: %v", err)
	}
}

func assertZeroRunStageComposition(t *testing.T, label string, got RunStageComposition) {
	t.Helper()
	if !reflect.DeepEqual(got, RunStageComposition{}) {
		t.Errorf("ComposeRunStage(%s) result = %#v, want zero composition", label, got)
	}
}

type runStageFixture struct {
	identity         recordlock.Identity
	projectRoot      *os.Root
	recordRoot       *os.Root
	activeSpacePath  string
	activeIntentPath string
	stageGraphPath   string
}

func newRunStageFixture(t *testing.T) runStageFixture {
	t.Helper()
	project := t.TempDir()
	dataDir := filepath.Join(project, ".codex", "tools", "data")
	recordDir := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	activeIntentDir := filepath.Dir(recordDir)
	otherRecordDir := filepath.Join(project, "aidlc", "spaces", "team", "intents", "other")
	otherSpaceIntentDir := filepath.Join(project, "aidlc", "spaces", "other", "intents", "build")
	for _, directory := range []string{dataDir, recordDir, otherRecordDir, otherSpaceIntentDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", directory, err)
		}
	}
	activeSpacePath := filepath.Join(project, "aidlc", "active-space")
	activeIntentPath := filepath.Join(activeIntentDir, "active-intent")
	stageGraphPath := filepath.Join(dataDir, "stage-graph.json")
	writeRunStageFile(t, activeSpacePath, "team\n")
	writeRunStageFile(t, activeIntentPath, "build\n")
	writeRunStageFile(t, filepath.Join(dataDir, "scope-grid.json"), runStageScopeGridJSON)
	writeRunStageFile(t, stageGraphPath, runStageGraphJSON)

	catalog, err := graph.Load(os.DirFS(dataDir))
	if err != nil {
		t.Fatalf("graph.Load(fixture): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     catalog,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               project,
		ProjectDescription:        "run-stage fixture",
		ProjectDescriptionPreview: "run-stage fixture",
		StartDate:                 "2026-09-05T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(fixture): %v", err)
	}
	writeRunStageFile(t, filepath.Join(recordDir, "aidlc-state.md"), initial.StateContent)
	writeRunStageFile(t, filepath.Join(otherRecordDir, "aidlc-state.md"), initial.StateContent)
	writeRunStageFile(t, filepath.Join(otherSpaceIntentDir, "aidlc-state.md"), initial.StateContent)

	identity, err := recordlock.NewIdentity(project, "team", "build")
	if err != nil {
		t.Fatalf("recordlock.NewIdentity(): %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatalf("os.OpenRoot(project): %v", err)
	}
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("os.OpenRoot(record): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return runStageFixture{
		identity:         identity,
		projectRoot:      projectRoot,
		recordRoot:       recordRoot,
		activeSpacePath:  activeSpacePath,
		activeIntentPath: activeIntentPath,
		stageGraphPath:   stageGraphPath,
	}
}

func writeRunStageFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
