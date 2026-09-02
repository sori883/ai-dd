package state

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/scope"
)

func TestBuildInitialBrownfieldGolden(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("intent-capture", "1.1", "ideation"),
		stageFixture("market-research", "1.2", "ideation"),
		stageFixture("reverse-engineering", "2.1", "inception"),
		stageFixture("code-generation", "3.5", "construction"),
		stageFixture("deployment-pipeline", "4.1", "operation"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold":  "EXECUTE",
			"state-init":          "EXECUTE",
			"intent-capture":      "EXECUTE",
			"market-research":     "SKIP",
			"reverse-engineering": "EXECUTE",
			"code-generation":     "EXECUTE",
			"deployment-pipeline": "SKIP",
		}},
	})

	got, err := BuildInitial(Input{
		Graph:                     snapshot,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard"},
		Workspace:                 WorkspaceInfo{ProjectType: "Brownfield", Languages: "Go", Frameworks: "Unknown", BuildSystem: "go"},
		ProjectRoot:               "/project",
		ProjectDescription:        "raw project description",
		ProjectDescriptionPreview: "Sample project",
		StartDate:                 "2026-09-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}

	const wantState = `# AI-DLC State Tracking

## Project Information
- **Project**: Sample project
- **Project Description Source**: project-description.json
- **Project Type**: Brownfield
- **Scope**: classic
- **Start Date**: 2026-09-02T00:00:00Z
- **State Version**: 8
- **Active Agent**: aidlc-product-agent
- **Worktree Path**:
- **Bolt Refs**:
- **Practices Affirmed Timestamp**:

## Scope Configuration
- **Stages to Execute**: 0.1, 0.3, 1.1, 2.1, 3.5
- **Stages to Skip**: 1.2 (market-research), 4.1 (deployment-pipeline)
- **Depth**: Standard
- **Test Strategy**: Standard
- **Review Override**: 

## Workspace State
- **Project Root**: /project
- **Languages**: Go
- **Frameworks**: Unknown
- **Build System**: go

## Execution Plan Summary
- **Total Stages**: 5
- **Completed**: 2
- **In Progress**: intent-capture

## Runtime State
- **Revision Count**: 0

## Phase Progress
<!-- Status values: Pending, Active, Verified, Skipped -->

- **Initialization**: Verified
- **Ideation**: Active
- **Inception**: Pending
- **Construction**: Pending
- **Operation**: Skipped

## Stage Progress
<!-- Checkbox states: [ ] not started, [-] in progress, [?] awaiting approval (gate open), [R] revising (user rejected gate), [x] completed, [S] skipped via --stage/--phase jump -->

### INITIALIZATION PHASE
- [x] workspace-scaffold — EXECUTE
- [x] state-init — EXECUTE

### IDEATION PHASE
- [-] intent-capture — EXECUTE
- [ ] market-research — SKIP

### INCEPTION PHASE
- [ ] reverse-engineering — EXECUTE

### CONSTRUCTION PHASE
Per unit: [TBD]
- [ ] code-generation — EXECUTE

### OPERATION PHASE
- [ ] deployment-pipeline — SKIP

## Current Status
- **Lifecycle Phase**: IDEATION
- **Current Stage**: intent-capture
- **Next Stage**: reverse-engineering
- **Status**: Running
- **Last Updated**: 2026-09-02T00:00:00Z

## Session Resume Point
- **Last Completed Stage**: state-init
- **Next Action**: Execute intent-capture
- **Pending Artifacts**: none
`
	if got.StateContent != wantState {
		t.Errorf("StateContent = %q, want %q", got.StateContent, wantState)
	}
	if got.ProjectDescriptionJSON != "\"raw project description\"\n" {
		t.Errorf("ProjectDescriptionJSON = %q, want JSON string with LF", got.ProjectDescriptionJSON)
	}
}

func TestBuildInitialProjectDescriptionJSONMatchesJSONStringifyEscaping(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold": "EXECUTE",
		}},
	})
	rawDescription := "<>&\u2028\u2029\"\\\b\f\n\r\t\x00 literal \\u003c"

	got, err := BuildInitial(Input{
		Graph:                     snapshot,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard"},
		Workspace:                 WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               "/project",
		ProjectDescription:        rawDescription,
		ProjectDescriptionPreview: "preview",
		StartDate:                 "2026-09-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}

	want := "\"<>&\u2028\u2029\\\"\\\\\\b\\f\\n\\r\\t\\u0000 literal \\\\u003c\"\n"
	if got.ProjectDescriptionJSON != want {
		t.Errorf("ProjectDescriptionJSON = %q, want JSON.stringify-compatible output %q", got.ProjectDescriptionJSON, want)
	}
}

func TestBuildInitialRoutingAndOwnership(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("intent-capture", "1.1", "ideation"),
		stageFixture("reverse-engineering", "2.1", "inception"),
		stageFixture("requirements-analysis", "2.3", "inception"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold":  "EXECUTE",
			"state-init":          "EXECUTE",
			"intent-capture":      "EXECUTE",
			"reverse-engineering": "EXECUTE",
		}},
	})

	input := Input{
		Graph:                     snapshot,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "standard"},
		Workspace:                 WorkspaceInfo{ProjectType: "brownfield"},
		ProjectRoot:               "/project",
		ProjectDescription:        "raw",
		ProjectDescriptionPreview: "preview",
		StartDate:                 "2026-09-02T00:00:00Z",
	}
	got, err := BuildInitial(input)
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}

	if got.Routing.EffectiveDepth != "Standard" {
		t.Errorf("EffectiveDepth = %q, want Standard", got.Routing.EffectiveDepth)
	}
	if got.Routing.FirstStage != "intent-capture" {
		t.Errorf("FirstStage = %q, want intent-capture", got.Routing.FirstStage)
	}
	if got.Routing.NextStage != "reverse-engineering" {
		t.Errorf("NextStage = %q, want reverse-engineering", got.Routing.NextStage)
	}
	if got.Routing.TotalStages != 4 {
		t.Errorf("TotalStages = %d, want 4", got.Routing.TotalStages)
	}
	if got.Routing.CompletedInitializationStages != 2 {
		t.Errorf("CompletedInitializationStages = %d, want 2", got.Routing.CompletedInitializationStages)
	}
	if got.Routing.SkipStages[0].Slug != "requirements-analysis" ||
		got.Routing.SkipStages[0].Action != graph.ActionSkip {
		t.Errorf("SkipStages[0] = %#v, want missing-cell SKIP route", got.Routing.SkipStages[0])
	}

	got.Routing.ExecuteStages[0].Slug = "mutated"
	if got.Routing.SkipStages[0].Slug != "requirements-analysis" {
		t.Errorf("SkipStages[0].Slug changed after execute mutation: %q", got.Routing.SkipStages[0].Slug)
	}
	if got.StateContent == "" {
		t.Error("StateContent is empty")
	}
}

func TestBuildInitialGreenfieldAdjustsReverseEngineering(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("reverse-engineering", "2.1", "inception"),
		stageFixture("requirements-analysis", "2.3", "inception"),
	}, map[string]any{
		"bugfix": map[string]any{"stages": map[string]any{
			"workspace-scaffold":    "EXECUTE",
			"state-init":            "EXECUTE",
			"reverse-engineering":   "EXECUTE",
			"requirements-analysis": "EXECUTE",
		}},
	})

	got, err := BuildInitial(Input{
		Graph:                     snapshot,
		Scope:                     "bugfix",
		ScopeMetadata:             scope.Metadata{Name: "bugfix", Depth: "Minimal"},
		Workspace:                 WorkspaceInfo{ProjectType: "GREENFIELD"},
		ProjectRoot:               "/project",
		ProjectDescription:        "raw",
		ProjectDescriptionPreview: "preview",
		StartDate:                 "2026-09-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}

	if !got.Routing.ReverseEngineeringSkippedGreenfield {
		t.Error("ReverseEngineeringSkippedGreenfield = false, want true")
	}
	if !got.Routing.IncrementalGreenfieldWarning {
		t.Error("IncrementalGreenfieldWarning = false, want true")
	}
	if got.Routing.FirstStage != "requirements-analysis" {
		t.Errorf("FirstStage = %q, want requirements-analysis", got.Routing.FirstStage)
	}
	if len(got.Routing.ExecuteStages) != 3 {
		t.Fatalf("ExecuteStages length = %d, want 3", len(got.Routing.ExecuteStages))
	}
	if len(got.Routing.SkipStages) != 1 {
		t.Fatalf("SkipStages length = %d, want 1", len(got.Routing.SkipStages))
	}
	greenfieldSkip := got.Routing.SkipStages[0]
	if greenfieldSkip.Slug != "reverse-engineering" {
		t.Errorf("greenfield skip slug = %q, want reverse-engineering", greenfieldSkip.Slug)
	}
	if greenfieldSkip.Reason != "greenfield" {
		t.Errorf("greenfield skip reason = %q, want greenfield", greenfieldSkip.Reason)
	}
	wantLine := "- **Stages to Skip**: 2.1 (reverse-engineering — greenfield)"
	if !strings.Contains(got.StateContent, wantLine) {
		t.Errorf("StateContent does not contain %q", wantLine)
	}
}

func TestBuildInitialNextUsesRawScopeAfterGreenfieldAdjustment(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("intent-capture", "1.1", "ideation"),
		stageFixture("reverse-engineering", "2.1", "inception"),
		stageFixture("requirements-analysis", "2.3", "inception"),
	}, map[string]any{
		"bugfix": map[string]any{"stages": map[string]any{
			"workspace-scaffold":  "EXECUTE",
			"state-init":          "EXECUTE",
			"intent-capture":      "EXECUTE",
			"reverse-engineering": "EXECUTE",
		}},
	})

	got, err := BuildInitial(Input{
		Graph:                     snapshot,
		Scope:                     "bugfix",
		ScopeMetadata:             scope.Metadata{Name: "bugfix", Depth: "minimal"},
		Workspace:                 WorkspaceInfo{ProjectType: "Greenfield"},
		ProjectRoot:               "/project",
		ProjectDescription:        "raw",
		ProjectDescriptionPreview: "preview",
		StartDate:                 "2026-09-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}
	if got.Routing.FirstStage != "intent-capture" {
		t.Errorf("FirstStage = %q, want intent-capture", got.Routing.FirstStage)
	}
	if got.Routing.NextStage != "reverse-engineering" {
		t.Errorf("NextStage = %q, want raw-scope reverse-engineering", got.Routing.NextStage)
	}
	if strings.Contains(got.StateContent, "- [ ] reverse-engineering — EXECUTE") {
		t.Error("greenfield-adjusted reverse-engineering remained EXECUTE in state content")
	}
}

func TestBuildInitialResolvesOverridesAndReview(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("intent-capture", "1.1", "ideation"),
		stageFixture("functional-design", "3.1", "construction"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold": "EXECUTE",
			"state-init":         "EXECUTE",
			"intent-capture":     "EXECUTE",
			"functional-design":  "EXECUTE",
		}},
	})

	tests := []struct {
		name                 string
		depthOverride        string
		testStrategyOverride string
		reviewOverride       string
		wantDepth            string
		wantTestStrategy     string
		wantReview           string
	}{
		{
			name:                 "all explicit values are canonicalized",
			depthOverride:        "COMPREHENSIVE",
			testStrategyOverride: "mInImAl",
			reviewOverride:       "ADVISORY",
			wantDepth:            "Comprehensive",
			wantTestStrategy:     "Minimal",
			wantReview:           "advisory",
		},
		{
			name:             "scope test strategy is canonicalized",
			reviewOverride:   "adversarial",
			wantDepth:        "Standard",
			wantTestStrategy: "Comprehensive",
		},
		{
			name:             "none review is stored",
			reviewOverride:   "NoNe",
			wantDepth:        "Standard",
			wantTestStrategy: "Comprehensive",
			wantReview:       "none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := Input{
				Graph:                     snapshot,
				Scope:                     "classic",
				ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "sTaNdArD", TestStrategy: "cOmPrEhEnSiVe"},
				Workspace:                 WorkspaceInfo{ProjectType: "Brownfield", Languages: "Go", Frameworks: "Unknown", BuildSystem: "go"},
				ProjectRoot:               "/project",
				ProjectDescription:        "raw\ndescription",
				ProjectDescriptionPreview: "safe preview",
				StartDate:                 "2026-09-02T00:00:00Z",
				DepthOverride:             tt.depthOverride,
				TestStrategyOverride:      tt.testStrategyOverride,
				ReviewOverride:            tt.reviewOverride,
			}
			got, err := BuildInitial(input)
			if err != nil {
				t.Fatalf("BuildInitial() error = %v", err)
			}
			if got.Routing.EffectiveDepth != tt.wantDepth {
				t.Errorf("EffectiveDepth = %q, want %q", got.Routing.EffectiveDepth, tt.wantDepth)
			}
			if got.Routing.EffectiveTestStrategy != tt.wantTestStrategy {
				t.Errorf("EffectiveTestStrategy = %q, want %q", got.Routing.EffectiveTestStrategy, tt.wantTestStrategy)
			}
			if got.Routing.ReviewOverride != tt.wantReview {
				t.Errorf("ReviewOverride = %q, want %q", got.Routing.ReviewOverride, tt.wantReview)
			}
			if !strings.Contains(got.StateContent, "- **Review Override**: "+tt.wantReview+"\n") {
				t.Errorf("StateContent has unexpected review override: %q", got.StateContent)
			}
			if got.ProjectDescriptionJSON != "\"raw\\ndescription\"\n" {
				t.Errorf("ProjectDescriptionJSON = %q, want escaped raw description", got.ProjectDescriptionJSON)
			}
			if !strings.Contains(got.StateContent, "- **Project**: safe preview\n") {
				t.Error("state content did not use the separately resolved preview")
			}
			if !strings.Contains(got.StateContent, "### CONSTRUCTION PHASE\nPer unit: [TBD]\n") {
				t.Error("construction phase is missing the canonical per-unit marker")
			}
		})
	}
}

func TestBuildInitialFallbackAndErrors(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("market-research", "1.2", "ideation"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold": "EXECUTE",
			"state-init":         "EXECUTE",
		}},
	})

	baseInput := Input{
		Graph:                     snapshot,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "standard"},
		Workspace:                 WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               "/project",
		ProjectDescription:        "raw",
		ProjectDescriptionPreview: "preview",
		StartDate:                 "2026-09-02T00:00:00Z",
	}
	got, err := BuildInitial(baseInput)
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}
	if got.Routing.FirstStage != "intent-capture" || got.Routing.NextStage != "none" {
		t.Errorf("fallback routing = (%q, %q), want (intent-capture, none)", got.Routing.FirstStage, got.Routing.NextStage)
	}
	if got.Routing.FirstPhase != "IDEATION" || got.Routing.FirstAgent != "aidlc-product-agent" {
		t.Errorf("fallback phase/agent = (%q, %q), want (IDEATION, aidlc-product-agent)", got.Routing.FirstPhase, got.Routing.FirstAgent)
	}
	if got.Routing.ExecuteStages == nil || got.Routing.SkipStages == nil {
		t.Fatal("routing slices must be initialized for caller ownership")
	}
	if len(got.Routing.ExecuteStages) != 2 || len(got.Routing.SkipStages) != 1 {
		t.Errorf("fallback route counts = (%d, %d), want (2, 1)", len(got.Routing.ExecuteStages), len(got.Routing.SkipStages))
	}
	if strings.Contains(got.StateContent, "- [-] intent-capture") {
		t.Error("fallback first stage was marked in progress despite not being in the graph")
	}

	tests := []struct {
		name   string
		mutate func(*Input)
		want   string
	}{
		{
			name: "unknown scope",
			mutate: func(input *Input) {
				input.Scope = "missing"
			},
			want: "unknown scope",
		},
		{
			name: "invalid depth override",
			mutate: func(input *Input) {
				input.DepthOverride = "extreme"
			},
			want: "invalid depth",
		},
		{
			name: "invalid test strategy override",
			mutate: func(input *Input) {
				input.TestStrategyOverride = "extreme"
			},
			want: "invalid test strategy",
		},
		{
			name: "invalid review override",
			mutate: func(input *Input) {
				input.ReviewOverride = "strict"
			},
			want: "invalid review override",
		},
		{
			name: "metadata scope mismatch",
			mutate: func(input *Input) {
				input.ScopeMetadata.Name = "other"
			},
			want: "does not match scope",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := baseInput
			tt.mutate(&input)
			_, err := BuildInitial(input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildInitial() error = %v, want context %q", err, tt.want)
			}
		})
	}
}

func loadTestSnapshot(t *testing.T, stages []map[string]any, grid map[string]any) graph.Snapshot {
	t.Helper()
	data := fstest.MapFS{
		"stage-graph.json": {Data: mustJSON(t, stages)},
		"scope-grid.json":  {Data: mustJSON(t, grid)},
	}
	snapshot, err := graph.Load(data)
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	return snapshot
}

func stageFixture(slug, number, phase string) map[string]any {
	return map[string]any{
		"slug":           slug,
		"number":         number,
		"name":           slug,
		"phase":          phase,
		"execution":      "ALWAYS",
		"lead_agent":     leadAgentForPhase(phase),
		"support_agents": []string{},
		"mode":           "inline",
	}
}

func leadAgentForPhase(phase string) string {
	if phase == "initialization" {
		return "orchestrator"
	}
	return "aidlc-product-agent"
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
