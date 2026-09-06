package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func TestComposeRunStageOKFKnowledgeCutover(t *testing.T) {
	fixture := newRunStageFixture(t)
	project := fixture.identity.ProjectRoot()
	knowledgeDir := filepath.Join(project, "aidlc", "spaces", "team", "knowledge")
	for _, relative := range []string{"aidlc/spaces/team/knowledge/aidlc-shared/legacy.md", ".codex/knowledge/aidlc-shared/framework.md"} {
		filename := filepath.Join(project, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		writeRunStageFile(t, filename, "knowledge body")
	}
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	before, err := ComposeRunStage(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before.Wire), "legacy.md") {
		t.Fatal("missing legacy fallback")
	}
	if err := os.Mkdir(filepath.Join(knowledgeDir, "okf"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"empty", "invalid"} {
		t.Run(name, func(t *testing.T) {
			if name == "invalid" {
				writeRunStageFile(t, filepath.Join(knowledgeDir, "okf", "bad.md"), "BODY_SECRET")
			}
			got, err := ComposeRunStage(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			text := string(got.Wire)
			if strings.Contains(text, "legacy.md") || strings.Contains(text, "BODY_SECRET") || !strings.Contains(text, "framework.md") || !strings.Contains(text, "aidlc knowledge search") {
				t.Fatalf("cutover wire %s", text)
			}
		})
	}
}

func TestComposeRunStageOKFUnsafeRoot(t *testing.T) {
	fixture := newRunStageFixture(t)
	filename := filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", "team", "knowledge", "okf")
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		t.Fatal(err)
	}
	writeRunStageFile(t, filename, "not a directory")
	if _, err := ComposeRunStage(t.Context(), RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}); err == nil {
		t.Fatal("accepted unsafe okf root")
	}
}

const runStageGraphWithReviewerJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"reviewer":"reviewer","review_artifact":"intent-statement","reviewer_max_iterations":2,"review_class":"advisory","produces":["intent-statement"],"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

func TestBuildRunStageWireIncludesReviewContract(t *testing.T) {
	fixture := newRunStageFixture(t)
	for _, relative := range []string{
		".codex/aidlc-common/protocols/stage-protocol.md",
		".codex/aidlc-common/protocols/stage-protocol-reviewer.md",
		".codex/skills/aidlc/question-rendering.md",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(fixture.identity.ProjectRoot(), filepath.FromSlash(relative))), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", relative, err)
		}
		writeRunStageFile(t, filepath.Join(fixture.identity.ProjectRoot(), filepath.FromSlash(relative)), "required context\n")
	}
	writeRunStageFile(t, filepath.Join(fixture.identity.ProjectRoot(), "aidlc", "spaces", "team", "intents", "build", "project-description.json"), `"run-stage fixture"`)
	graphJSON := strings.Replace(
		runStageGraphJSON,
		`"phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[]`,
		`"phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"reviewer":"aidlc-product-lead-agent","review_artifact":"intent-statement","reviewer_max_iterations":2,"review_class":"adversarial","produces":[]`,
		1,
	)
	writeRunStageFile(t, fixture.stageGraphPath, graphJSON)
	scopePath := filepath.Join(fixture.identity.ProjectRoot(), ".codex", "scopes", "classic.md")
	if err := os.MkdirAll(filepath.Dir(scopePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(scopePath), err)
	}
	writeRunStageFile(t, scopePath, "---\nname: classic\nreview_cap: advisory\n---\n")

	catalog, err := graph.Load(os.DirFS(filepath.Dir(fixture.stageGraphPath)))
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	var stage graph.Stage
	for _, candidate := range catalog.Stages() {
		if candidate.Slug == "intent-capture" {
			stage = candidate
			break
		}
	}
	if stage.Slug == "" {
		t.Fatal("intent-capture stage missing")
	}
	current, err := state.Read(fixture.recordRoot)
	if err != nil {
		t.Fatalf("state.Read() error = %v", err)
	}
	wireBytes, err := buildRunStageWire(fixture.identity, stage, current, catalog, fixture.projectRoot, fixture.recordRoot, nil, knowledge.Roster{})
	if err != nil {
		t.Fatalf("buildRunStageWire() error = %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(wireBytes, &wire); err != nil {
		t.Fatalf("json.Unmarshal(wire): %v", err)
	}
	if got, want := wire["reviewer"], "aidlc-product-lead-agent"; got != want {
		t.Errorf("wire reviewer = %#v, want %q", got, want)
	}
	if got, want := wire["review_artifact"], "intent-statement"; got != want {
		t.Errorf("wire review_artifact = %#v, want %q", got, want)
	}
	if got, want := wire["review_class"], "advisory"; got != want {
		t.Errorf("wire review_class = %#v, want %q", got, want)
	}
	if got, want := wire["reviewer_max_iterations"], float64(1); got != want {
		t.Errorf("wire reviewer_max_iterations = %#v, want %v", got, want)
	}
	wireText := string(wireBytes)
	for _, field := range []string{"next_stage", "reviewer", "review_artifact", "review_class", "reviewer_max_iterations", "narration"} {
		if !strings.Contains(wireText, `"`+field+`"`) {
			t.Errorf("wire missing %q: %s", field, wireText)
		}
	}
	if strings.Index(wireText, `"reviewer"`) < strings.Index(wireText, `"next_stage"`) {
		t.Errorf("review fields must follow next_stage: %s", wireText)
	}
}

func TestBuildRunStageWireRejectsMissingRequiredIntentCaptureContext(t *testing.T) {
	fixture := newRunStageFixture(t)
	stage := graph.Stage{
		Slug: "intent-capture", Phase: "ideation", Execution: "ALWAYS", LeadAgent: "aidlc-product-agent", Mode: "inline",
		Scopes: []string{"enterprise", "feature", "mvp", "poc"}, Enabled: true,
		Reviewer: "aidlc-product-lead-agent", ReviewArtifact: "intent-statement", ReviewerMaxIterations: 2,
		ReviewClass: graph.ReviewClassAdvisory, SummaryConfirmation: "required",
		Sensors:       []string{"claim-sources", "required-sections", "upstream-coverage"},
		Produces:      []string{"intent-statement", "stakeholder-map", "intent-capture-questions"},
		SupportAgents: []string{"aidlc-architect-agent"},
	}
	current, err := state.Read(fixture.recordRoot)
	if err != nil {
		t.Fatalf("state.Read(): %v", err)
	}
	catalog, err := graph.Load(os.DirFS(filepath.Dir(fixture.stageGraphPath)))
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	if _, err := buildRunStageWire(fixture.identity, stage, current, catalog, fixture.projectRoot, fixture.recordRoot, nil, knowledge.Roster{}); err == nil {
		t.Fatal("buildRunStageWire() error = nil, want missing required intent-capture context failure")
	}
}

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

func TestComposeRunStageWithGuard(t *testing.T) {
	fixture := newRunStageFixture(t)
	var got RunStageComposition
	err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		got, err = ComposeRunStageWithGuard(context.Background(), guard, RunStageInput{
			Identity:    fixture.identity,
			ProjectRoot: fixture.projectRoot,
			RecordRoot:  fixture.recordRoot,
		})
		if err != nil {
			return err
		}
		if !guard.Held() {
			t.Fatal("ComposeRunStageWithGuard() released the caller-owned guard")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recordlock.With(ComposeRunStageWithGuard) error = %v", err)
	}
	if got.Directive.Kind != orchestrator.DirectiveKindRunStage {
		t.Fatalf("ComposeRunStageWithGuard().Directive.Kind = %q, want %q", got.Directive.Kind, orchestrator.DirectiveKindRunStage)
	}
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

func TestComposeRunStageBindsCanonicalHashesAndOwnsResult(t *testing.T) {
	t.Run("canonical hashes and ownership", func(t *testing.T) {
		fixture := newRunStageFixture(t)
		writeRunStageFile(t, fixture.stageGraphPath, runStageRulesGraphJSON)

		memoryDir := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", fixture.identity.Space(), "memory")
		if err := os.MkdirAll(memoryDir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", memoryDir, err)
		}
		alphaText := "組織ルール 🚀\n"
		projectRuleText := "プロジェクト rule\n"
		betaText := "空間ルール 日本語\n"
		writeRunStageFile(t, filepath.Join(memoryDir, "alpha.md"), alphaText)
		writeRunStageFile(t, filepath.Join(fixture.identity.ProjectPath(), "project-rule.md"), projectRuleText)
		writeRunStageFile(t, filepath.Join(memoryDir, "beta.md"), betaText)

		conductorPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "conductor.md")
		if err := os.MkdirAll(filepath.Dir(conductorPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(conductorPath), err)
		}
		conductorText := "司令塔 <>&" + string(rune(0x2028)) + string(rune(0x2029)) + "\"\\\x00\v\b\f\n\r\t"
		writeRunStageFile(t, conductorPath, conductorText)
		conductorJSON := "司令塔 <>&" + string(rune(0x2028)) + string(rune(0x2029)) + `\"\\\u0000\u000b\b\f\n\r\t`

		dataDir := filepath.Dir(fixture.stageGraphPath)
		catalog, err := graph.Load(os.DirFS(dataDir))
		if err != nil {
			t.Fatalf("graph.Load(hashes fixture): %v", err)
		}
		var stage graph.Stage
		for _, candidate := range catalog.Stages() {
			if candidate.Slug == "intent-capture" {
				stage = candidate
				break
			}
		}
		if stage.Slug == "" {
			t.Fatal("hashes fixture current stage is absent")
		}
		resolvedRules, err := steering.ResolveRulePaths(
			fixture.identity.ProjectPath(),
			fixture.identity.Space(),
			"",
			stage.RulesInContext,
		)
		if err != nil {
			t.Fatalf("steering.ResolveRulePaths(hashes fixture): %v", err)
		}
		wantRules := []steering.RuleContent{
			{Path: resolvedRules.Entries[0].Path, Text: alphaText},
			{Path: resolvedRules.Entries[1].Path, Text: projectRuleText},
			{Path: resolvedRules.Entries[2].Path, Text: betaText},
		}
		wantBundle, err := steering.BundleDigest(wantRules)
		if err != nil {
			t.Fatalf("steering.BundleDigest(hashes fixture): %v", err)
		}
		wantChunks := steering.ChunkRules(wantRules)

		wireWant := `{"kind":"run-stage","stage":"intent-capture","phase":"ideation","lead_agent":"orchestrator","support_agents":[],"mode":"inline","inline_context_paths":[],"gate":true,"memory_path":"aidlc/spaces/team/intents/build/ideation/intent-capture/memory.md","consumes":[],"produces":[],"rules_in_context":["aidlc/spaces/team/memory/alpha.md","project-rule.md","aidlc/spaces/team/memory/beta.md"],"sensors_applicable":[],"stage_file":".codex/aidlc-common/stages/ideation/intent-capture.md","next_stage":"Next Stage","conductor_persona":"` + conductorJSON + `","narration":"Starting the classic plan for this project. First step is Intent Capture, and I will stop for your review before anything is final."}`
		directiveDigest := sha256.Sum256([]byte(wireWant))
		directiveHash := hex.EncodeToString(directiveDigest[:])
		stateBytesBefore := stateBytesForRunStage(t, fixture.recordRoot)
		stateDigest := sha256.Sum256(stateBytesBefore)
		stateHash := hex.EncodeToString(stateDigest[:])
		routeHash, err := catalog.RouteHash(stage.Slug, "classic")
		if err != nil {
			t.Fatalf("catalog.RouteHash(hashes fixture): %v", err)
		}
		freshStateHash := stateHash
		claimStateHash := stateHash
		nextStage := "Next Stage"
		swarmSettled := false
		wantFreshness := steering.ContinuationFreshness{
			Stage:         stage.Slug,
			Scope:         "classic",
			Bundle:        wantBundle,
			DirectiveHash: directiveHash,
			RouteHash:     routeHash,
			StateHash:     &freshStateHash,
		}
		wantClaims := steering.ContinuationClaims{
			Version:       1,
			Stage:         stage.Slug,
			Scope:         "classic",
			NextPart:      1,
			Bundle:        wantBundle,
			DirectiveHash: directiveHash,
			RouteHash:     routeHash,
			StateAware:    true,
			Gate:          steering.GateTrue,
			NextStage:     steering.OptionalNullableString{Present: true, Value: &nextStage},
			SwarmSettled:  &swarmSettled,
			StateHash:     &claimStateHash,
		}
		input := RunStageInput{
			Identity:    fixture.identity,
			ProjectRoot: fixture.projectRoot,
			RecordRoot:  fixture.recordRoot,
		}

		first, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(first hashes fixture) error = %v, want nil", err)
		}
		second, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(second hashes fixture) error = %v, want nil", err)
		}
		assertRunStageHashComposition(t, "first", first, stage, wireWant, wantRules, wantChunks, wantBundle, wantFreshness, wantClaims)
		assertRunStageHashComposition(t, "second", second, stage, wireWant, wantRules, wantChunks, wantBundle, wantFreshness, wantClaims)
		if first.Freshness.StateHash == first.Claims.StateHash {
			t.Errorf("ComposeRunStage(first) Freshness.StateHash and Claims.StateHash share a pointer")
		}

		if len(first.Wire) != 0 {
			first.Wire[0] = 'X'
		}
		if len(first.Directive.Stage.SupportAgents) != 0 {
			first.Directive.Stage.SupportAgents[0] = "mutated-support"
		}
		if len(first.Directive.Stage.RulesInContext) != 0 {
			first.Directive.Stage.RulesInContext[0].Path = "mutated-rule-path"
		}
		if first.Directive.Stage.ProducesKinds != nil {
			first.Directive.Stage.ProducesKinds["mutated"] = []string{"mutated"}
		}
		if len(first.Rules) != 0 {
			first.Rules[0].Text = "mutated-rule-text"
		}
		if len(first.Chunks) != 0 && len(first.Chunks[0]) != 0 {
			first.Chunks[0][0].Text = "mutated-chunk-text"
		}
		if first.Freshness.StateHash != nil {
			*first.Freshness.StateHash = "mutated-freshness-state"
		} else {
			t.Errorf("ComposeRunStage(first).Freshness.StateHash = nil, want owned state hash")
		}
		if first.Claims.StateHash != nil {
			*first.Claims.StateHash = "mutated-claims-state"
		} else {
			t.Errorf("ComposeRunStage(first).Claims.StateHash = nil, want owned state hash")
		}
		if first.Claims.NextStage.Value != nil {
			*first.Claims.NextStage.Value = "mutated-next-stage"
		} else {
			t.Errorf("ComposeRunStage(first).Claims.NextStage.Value = nil, want Next Stage")
		}
		if first.Claims.SwarmSettled != nil {
			*first.Claims.SwarmSettled = true
		} else {
			t.Errorf("ComposeRunStage(first).Claims.SwarmSettled = nil, want owned false")
		}
		assertRunStageHashComposition(t, "second after first mutation", second, stage, wireWant, wantRules, wantChunks, wantBundle, wantFreshness, wantClaims)

		if len(second.Wire) != 0 {
			second.Wire[0] = 'Y'
		}
		if len(second.Rules) != 0 {
			second.Rules[0].Path = "second-mutated-rule"
		}
		if len(second.Chunks) != 0 && len(second.Chunks[0]) != 0 {
			second.Chunks[0][0].Path = "second-mutated-chunk"
		}
		third, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(third hashes fixture) error = %v, want nil", err)
		}
		assertRunStageHashComposition(t, "third after result mutations", third, stage, wireWant, wantRules, wantChunks, wantBundle, wantFreshness, wantClaims)
		stateBytesAfter := stateBytesForRunStage(t, fixture.recordRoot)
		if !bytes.Equal(stateBytesAfter, stateBytesBefore) {
			t.Errorf("ComposeRunStage changed aidlc-state.md bytes")
		}
	})

	t.Run("directive byte cap", func(t *testing.T) {
		fixture := newRunStageFixture(t)
		conductorPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "conductor.md")
		if err := os.MkdirAll(filepath.Dir(conductorPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(conductorPath), err)
		}
		writeRunStageFile(t, conductorPath, "")
		input := RunStageInput{
			Identity:    fixture.identity,
			ProjectRoot: fixture.projectRoot,
			RecordRoot:  fixture.recordRoot,
		}
		base, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Fatalf("ComposeRunStage(empty conductor) error = %v, want nil", err)
		}
		if len(base.Wire) >= 28*1024 {
			t.Fatalf("empty conductor wire length = %d, want below 28 KiB", len(base.Wire))
		}
		exactText := strings.Repeat("p", 28*1024-len(base.Wire))
		writeRunStageFile(t, conductorPath, exactText)
		exact, err := ComposeRunStage(context.Background(), input)
		if err != nil {
			t.Errorf("ComposeRunStage(exact cap) error = %v, want nil", err)
		}
		if len(exact.Wire) != 28*1024 {
			t.Errorf("ComposeRunStage(exact cap) wire length = %d, want %d", len(exact.Wire), 28*1024)
		}

		writeRunStageFile(t, conductorPath, exactText+"p")
		over, err := ComposeRunStage(context.Background(), input)
		if err == nil {
			t.Error("ComposeRunStage(over cap) error = nil, want ErrDirectiveTooLarge")
		} else if !errors.Is(err, ErrDirectiveTooLarge) {
			t.Errorf("ComposeRunStage(over cap) error = %v, want ErrDirectiveTooLarge", err)
		}
		assertZeroRunStageComposition(t, "over directive byte cap", over)
	})
}

func assertRunStageHashComposition(
	t *testing.T,
	label string,
	got RunStageComposition,
	wantStage graph.Stage,
	wantWire string,
	wantRules []steering.RuleContent,
	wantChunks [][]steering.RuleContent,
	wantBundle string,
	wantFreshness steering.ContinuationFreshness,
	wantClaims steering.ContinuationClaims,
) {
	t.Helper()
	if string(got.Wire) != wantWire {
		t.Errorf("ComposeRunStage(%s).Wire = %q, want canonical JSON wire %q", label, got.Wire, wantWire)
	}
	if !reflect.DeepEqual(got.Directive.Stage, wantStage) {
		t.Errorf("ComposeRunStage(%s).Directive.Stage = %#v, want %#v", label, got.Directive.Stage, wantStage)
	}
	if !reflect.DeepEqual(got.Rules, wantRules) {
		t.Errorf("ComposeRunStage(%s).Rules = %#v, want %#v", label, got.Rules, wantRules)
	}
	if !reflect.DeepEqual(got.Chunks, wantChunks) {
		t.Errorf("ComposeRunStage(%s).Chunks = %#v, want %#v", label, got.Chunks, wantChunks)
	}
	if got.Bundle != wantBundle {
		t.Errorf("ComposeRunStage(%s).Bundle = %q, want %q", label, got.Bundle, wantBundle)
	}
	if !reflect.DeepEqual(got.Freshness, wantFreshness) {
		t.Errorf("ComposeRunStage(%s).Freshness = %#v, want %#v", label, got.Freshness, wantFreshness)
	}
	if !reflect.DeepEqual(got.Claims, wantClaims) {
		t.Errorf("ComposeRunStage(%s).Claims = %#v, want %#v", label, got.Claims, wantClaims)
	}
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
