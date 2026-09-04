package orchestrator

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestResolveDirectiveRunningStage(t *testing.T) {
	t.Parallel()

	runningState := strings.Replace(directiveRunningState, "**In Progress**: intent-capture", "**In Progress**: summary-only-stage", 1)
	runningState = strings.Replace(runningState, "**Next Stage**: none", "**Next Stage**: next-only-stage", 1)
	parsed := parseDirectiveState(t, runningState)
	snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation"))

	got, err := ResolveDirective(parsed, snapshot)
	if err != nil {
		t.Fatalf("ResolveDirective() error = %v", err)
	}
	if got.Kind() != DirectiveKindRunStage {
		t.Fatalf("Directive.Kind() = %q, want %q", got.Kind(), DirectiveKindRunStage)
	}
	stage, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage")
	}
	if stage.Slug != "intent-capture" || stage.Phase != "ideation" || stage.LeadAgent != "orchestrator" {
		t.Fatalf("Directive.Stage() = %#v, want intent-capture stage metadata", stage)
	}
}

func TestResolveDirectiveRejectsInvalidRunningState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "current stage is completed",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [x] intent-capture — EXECUTE", 1),
		},
		{
			name:    "current stage is skipped",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [S] intent-capture — SKIP", 1),
		},
		{
			name:    "current plan action is skip",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [-] intent-capture — SKIP", 1),
		},
		{
			name:    "multiple live stages",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [-] intent-capture — EXECUTE\n- [-] another-stage — EXECUTE", 1),
		},
		{
			name:    "current stage is absent",
			content: strings.Replace(directiveRunningState, "**Current Stage**: intent-capture", "**Current Stage**: missing-stage", 1),
		},
		{
			name:    "current stage is none",
			content: strings.Replace(directiveRunningState, "**Current Stage**: intent-capture", "**Current Stage**: none", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, tt.content)
			snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation"))
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrInvalidState)
		})
	}
}

func TestResolveDirectiveReportsUnsupportedCurrentState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "pending",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [ ] intent-capture — EXECUTE", 1),
		},
		{
			name:    "awaiting approval",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [?] intent-capture — EXECUTE", 1),
		},
		{
			name:    "revising",
			content: strings.Replace(directiveRunningState, "- [-] intent-capture — EXECUTE", "- [R] intent-capture — EXECUTE", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, tt.content)
			snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation"))
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrUnsupportedState)
		})
	}
}

func TestResolveDirectiveRejectsUnsupportedCurrentCombinations(t *testing.T) {
	t.Parallel()

	markers := []string{"[ ]", "[?]", "[R]"}
	for _, marker := range markers {
		marker := marker
		t.Run(marker+" with another live marker", func(t *testing.T) {
			t.Parallel()

			currentRow := "- " + marker + " intent-capture — EXECUTE"
			content := strings.Replace(
				directiveRunningState,
				"- [-] intent-capture — EXECUTE",
				currentRow+"\n- [-] another-stage — EXECUTE",
				1,
			)
			parsed := parseDirectiveState(t, content)
			snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation"))
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrInvalidState)
		})

		t.Run(marker+" with skip action", func(t *testing.T) {
			t.Parallel()

			content := strings.Replace(
				directiveRunningState,
				"- [-] intent-capture — EXECUTE",
				"- "+marker+" intent-capture — SKIP",
				1,
			)
			parsed := parseDirectiveState(t, content)
			snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation"))
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrInvalidState)
		})
	}
}

func TestResolveDirectiveRejectsZeroState(t *testing.T) {
	t.Parallel()

	got, err := ResolveDirective(state.State{}, graph.Snapshot{})
	expectDirectiveError(t, got, err, ErrInvalidState)
}

func TestResolveDirectiveMapsCanonicalGraphPhases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		graphPhase string
		statePhase string
	}{
		{name: "initialization", graphPhase: "initialization", statePhase: "INITIALIZATION"},
		{name: "ideation", graphPhase: "ideation", statePhase: "IDEATION"},
		{name: "inception", graphPhase: "inception", statePhase: "INCEPTION"},
		{name: "construction", graphPhase: "construction", statePhase: "CONSTRUCTION"},
		{name: "operation", graphPhase: "operation", statePhase: "OPERATION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := strings.Replace(directiveRunningState, "**Lifecycle Phase**: IDEATION", "**Lifecycle Phase**: "+tt.statePhase, 1)
			parsed := parseDirectiveState(t, content)
			snapshot := loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", tt.graphPhase))
			got, err := ResolveDirective(parsed, snapshot)
			if err != nil {
				t.Fatalf("ResolveDirective() error = %v", err)
			}
			if got.Kind() != DirectiveKindRunStage {
				t.Fatalf("Directive.Kind() = %q, want %q", got.Kind(), DirectiveKindRunStage)
			}
			stage, ok := got.Stage()
			if !ok || stage.Phase != tt.graphPhase {
				t.Fatalf("Directive.Stage() = %#v, want phase %q", stage, tt.graphPhase)
			}
		})
	}
}

func TestResolveDirectiveCompletedActionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "execute skipped",
			content: strings.Replace(directiveCompletedState, "- [x] intent-capture — EXECUTE", "- [S] intent-capture — EXECUTE", 1),
		},
		{
			name:    "skip pending",
			content: strings.Replace(directiveCompletedState, "- [S] future-stage — SKIP", "- [ ] future-stage — SKIP", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, tt.content)
			snapshot := loadDirectiveGraph(
				t,
				directiveStage("workspace-scaffold", "1.1", "initialization"),
				directiveStage("intent-capture", "2.1", "ideation"),
				directiveStage("future-stage", "2.2", "ideation"),
			)
			got, err := ResolveDirective(parsed, snapshot)
			if err != nil {
				t.Fatalf("ResolveDirective() error = %v", err)
			}
			if got.Kind() != DirectiveKindWorkflowComplete {
				t.Fatalf("Directive.Kind() = %q, want %q", got.Kind(), DirectiveKindWorkflowComplete)
			}
			if _, ok := got.Stage(); ok {
				t.Fatal("Directive.Stage() reports a stage for workflow completion")
			}
		})
	}
}

func TestResolveDirectiveRejectsStateCatalogMismatch(t *testing.T) {
	t.Parallel()

	disabledStage := directiveStage("intent-capture", "2.1", "ideation")
	disabledStage["enabled"] = false
	tests := []struct {
		name   string
		stages []map[string]any
	}{
		{
			name:   "current stage is absent",
			stages: []map[string]any{directiveStage("other-stage", "2.2", "ideation")},
		},
		{
			name:   "phase differs",
			stages: []map[string]any{directiveStage("intent-capture", "2.1", "inception")},
		},
		{
			name:   "phase is not canonical lowercase",
			stages: []map[string]any{directiveStage("intent-capture", "2.1", "IDEATION")},
		},
		{
			name:   "stage is disabled",
			stages: []map[string]any{disabledStage},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, directiveRunningState)
			snapshot := loadDirectiveGraph(t, tt.stages...)
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrStateCatalogMismatch)
		})
	}
}

func TestResolveDirectiveWorkflowComplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "current stage is none",
			content: directiveCompletedState,
		},
		{
			name:    "current stage is settled execute",
			content: strings.Replace(directiveCompletedState, "**Current Stage**: none", "**Current Stage**: intent-capture", 1),
		},
		{
			name:    "current stage is settled skip",
			content: strings.Replace(directiveCompletedState, "**Current Stage**: none", "**Current Stage**: future-stage", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, tt.content)
			snapshot := loadDirectiveGraph(
				t,
				directiveStage("workspace-scaffold", "1.1", "initialization"),
				directiveStage("intent-capture", "2.1", "ideation"),
				directiveStage("future-stage", "2.2", "ideation"),
			)
			got, err := ResolveDirective(parsed, snapshot)
			if err != nil {
				t.Fatalf("ResolveDirective() error = %v", err)
			}
			if got.Kind() != DirectiveKindWorkflowComplete {
				t.Fatalf("Directive.Kind() = %q, want %q", got.Kind(), DirectiveKindWorkflowComplete)
			}
			if _, ok := got.Stage(); ok {
				t.Fatal("Directive.Stage() reports a stage for workflow completion")
			}
		})
	}

	parsed := parseDirectiveState(t, directiveCompletedState)
	got, err := ResolveDirective(parsed, graph.Snapshot{})
	if err != nil {
		t.Fatalf("ResolveDirective() without catalog error = %v", err)
	}
	if got.Kind() != DirectiveKindWorkflowComplete {
		t.Fatalf("Directive.Kind() without catalog = %q, want %q", got.Kind(), DirectiveKindWorkflowComplete)
	}
}

func TestDirectiveStageOwnership(t *testing.T) {
	t.Parallel()

	graphStage := directiveStage("intent-capture", "2.1", "ideation")
	graphStage["support_agents"] = []string{"reviewer"}
	graphStage["scopes"] = []string{"classic"}
	graphStage["produces"] = []string{"intent-statement"}
	graphStage["optional_produces"] = []string{"questions"}
	graphStage["consumes"] = []map[string]any{
		{"artifact": "project-description", "required": true, "conditional_on": "brownfield"},
	}
	graphStage["requires_stage"] = []string{"workspace-scaffold"}
	snapshot := loadDirectiveGraph(t, directiveStage("workspace-scaffold", "1.1", "initialization"), graphStage)
	parsed := parseDirectiveState(t, directiveRunningState)

	got, err := ResolveDirective(parsed, snapshot)
	if err != nil {
		t.Fatalf("ResolveDirective() error = %v", err)
	}
	first, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage")
	}
	want := graph.Stage{
		Slug:             "intent-capture",
		Number:           "2.1",
		Name:             "intent-capture name",
		Phase:            "ideation",
		Execution:        "ALWAYS",
		LeadAgent:        "orchestrator",
		SupportAgents:    []string{"reviewer"},
		Mode:             "inline",
		Scopes:           []string{"classic"},
		Enabled:          true,
		Produces:         []string{"intent-statement"},
		OptionalProduces: []string{"questions"},
		Consumes: []graph.Consume{
			{Artifact: "project-description", Required: true, ConditionalOn: "brownfield"},
		},
		RequiresStages: []string{"workspace-scaffold"},
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("Directive.Stage() = %#v, want %#v", first, want)
	}

	first.SupportAgents[0] = "mutated-agent"
	first.Scopes[0] = "mutated-scope"
	first.Produces[0] = "mutated-artifact"
	first.OptionalProduces[0] = "mutated-optional-artifact"
	first.Consumes[0].Artifact = "mutated-input"
	first.RequiresStages[0] = "mutated-dependency"

	second, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage on second read")
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("Directive.Stage() after mutation = %#v, want %#v", second, want)
	}

	resolvedAgain, err := ResolveDirective(parsed, snapshot)
	if err != nil {
		t.Fatalf("ResolveDirective() second call error = %v", err)
	}
	third, ok := resolvedAgain.Stage()
	if !ok {
		t.Fatal("second Directive.Stage() reports no stage")
	}
	if !reflect.DeepEqual(third, want) {
		t.Fatalf("catalog stage after mutation = %#v, want %#v", third, want)
	}

	var zero Directive
	if zero.Kind() != DirectiveKindUnknown {
		t.Errorf("zero Directive.Kind() = %q, want %q", zero.Kind(), DirectiveKindUnknown)
	}
	if _, ok := zero.Stage(); ok {
		t.Error("zero Directive.Stage() reports a stage")
	}
}

func TestDirectiveStageOwnershipIncludesSensorsAndProducesKinds(t *testing.T) {
	t.Parallel()

	graphStage := directiveStage("intent-capture", "2.1", "ideation")
	graphStage["sensors"] = []string{"quality"}
	graphStage["produces_kinds"] = map[string][]string{"report": {"summary"}}
	snapshot := loadDirectiveGraph(t, graphStage)
	parsed := parseDirectiveState(t, directiveRunningState)

	got, err := ResolveDirective(parsed, snapshot)
	if err != nil {
		t.Fatalf("ResolveDirective() error = %v", err)
	}
	first, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage")
	}
	first.Sensors[0] = "mutated-sensor"
	first.ProducesKinds["report"][0] = "mutated-kind"

	second, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage on second read")
	}
	if second.Sensors[0] != "quality" || second.ProducesKinds["report"][0] != "summary" {
		t.Fatalf("Directive.Stage() after mutation = %#v, want independent sensor/kind copies", second)
	}
}

func TestDirectiveRulesInContextOwnership(t *testing.T) {
	t.Parallel()

	graphStage := directiveStage("intent-capture", "2.1", "ideation")
	graphStage["rules_in_context"] = []map[string]any{{"path": "/memory/org.md", "scope": "org"}}
	snapshot := loadDirectiveGraph(t, graphStage)
	parsed := parseDirectiveState(t, directiveRunningState)

	got, err := ResolveDirective(parsed, snapshot)
	if err != nil {
		t.Fatalf("ResolveDirective() error = %v", err)
	}
	first, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage")
	}
	first.RulesInContext[0].Path = "changed"

	second, ok := got.Stage()
	if !ok {
		t.Fatal("Directive.Stage() reports no stage on second read")
	}
	if got := second.RulesInContext[0].Path; got != "/memory/org.md" {
		t.Fatalf("Directive.Stage() exposed RulesInContext storage: got %q", got)
	}
}

func TestResolveDirectiveRejectsInvalidCompletedState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "next stage remains",
			content: strings.Replace(directiveCompletedState, "**Next Stage**: none", "**Next Stage**: future-stage", 1),
		},
		{
			name:    "summary remains in progress",
			content: strings.Replace(directiveCompletedState, "**In Progress**: none", "**In Progress**: intent-capture", 1),
		},
		{
			name:    "execute row is pending",
			content: strings.Replace(directiveCompletedState, "- [x] intent-capture — EXECUTE", "- [ ] intent-capture — EXECUTE", 1),
		},
		{
			name:    "skip row is completed",
			content: strings.Replace(directiveCompletedState, "- [S] future-stage — SKIP", "- [x] future-stage — SKIP", 1),
		},
		{
			name:    "live row remains",
			content: strings.Replace(directiveCompletedState, "- [x] intent-capture — EXECUTE", "- [-] intent-capture — EXECUTE", 1),
		},
		{
			name:    "current stage is absent",
			content: strings.Replace(directiveCompletedState, "**Current Stage**: none", "**Current Stage**: missing-stage", 1),
		},
		{
			name: "current stage is not settled",
			content: strings.Replace(
				strings.Replace(directiveCompletedState, "**Current Stage**: none", "**Current Stage**: future-stage", 1),
				"- [S] future-stage — SKIP", "- [ ] future-stage — SKIP", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed := parseDirectiveState(t, tt.content)
			snapshot := loadDirectiveGraph(
				t,
				directiveStage("workspace-scaffold", "1.1", "initialization"),
				directiveStage("intent-capture", "2.1", "ideation"),
				directiveStage("future-stage", "2.2", "ideation"),
			)
			got, err := ResolveDirective(parsed, snapshot)
			expectDirectiveError(t, got, err, ErrInvalidState)
		})
	}
}

const directiveRunningState = `# AI-DLC State Tracking

## Project Information
- **Project Type**: Brownfield
- **Scope**: classic
- **State Version**: 8

## Execution Plan Summary
- **Total Stages**: 2
- **Completed**: 0
- **In Progress**: intent-capture

## Phase Progress
- **Initialization**: Pending
- **Ideation**: Active
- **Inception**: Pending
- **Construction**: Pending
- **Operation**: Pending

## Stage Progress
### Initialization
- [ ] workspace-scaffold — EXECUTE
### Ideation
- [-] intent-capture — EXECUTE

## Current Status
- **Lifecycle Phase**: IDEATION
- **Current Stage**: intent-capture
- **Next Stage**: none
- **Status**: Running
`

const directiveCompletedState = `# AI-DLC State Tracking

## Project Information
- **Project Type**: Brownfield
- **Scope**: classic
- **State Version**: 8

## Execution Plan Summary
- **Total Stages**: 3
- **Completed**: 3
- **In Progress**: none

## Phase Progress
- **Initialization**: Verified
- **Ideation**: Verified
- **Inception**: Verified
- **Construction**: Verified
- **Operation**: Verified

## Stage Progress
### Initialization
- [x] workspace-scaffold — EXECUTE
### Ideation
- [x] intent-capture — EXECUTE
- [S] future-stage — SKIP

## Current Status
- **Lifecycle Phase**: OPERATION
- **Current Stage**: none
- **Next Stage**: none
- **Status**: Completed
`

func parseDirectiveState(t *testing.T, content string) state.State {
	t.Helper()

	parsed, err := state.Parse([]byte(content))
	if err != nil {
		t.Fatalf("state.Parse() error = %v", err)
	}
	return parsed
}

func loadDirectiveGraph(t *testing.T, stages ...map[string]any) graph.Snapshot {
	t.Helper()

	data, err := json.Marshal(stages)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: data},
		"scope-grid.json":  {Data: []byte(`{"classic":{"stages":{}}}`)},
	}
	snapshot, err := graph.Load(dataFS)
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	return snapshot
}

func directiveStage(slug, number, phase string) map[string]any {
	return map[string]any{
		"slug":           slug,
		"number":         number,
		"name":           slug + " name",
		"phase":          phase,
		"execution":      "ALWAYS",
		"lead_agent":     "orchestrator",
		"support_agents": []string{},
		"mode":           "inline",
		"scopes":         []string{},
		"produces":       []string{},
		"consumes":       []map[string]any{},
		"requires_stage": []string{},
	}
}

func expectDirectiveError(t *testing.T, got Directive, err error, sentinel error) {
	t.Helper()

	if err == nil {
		t.Fatalf("ResolveDirective() error = nil, directive = %#v", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("ResolveDirective() error = %v, want errors.Is(..., %v)", err, sentinel)
	}
	if err.Error() != strings.ToLower(err.Error()) {
		t.Errorf("ResolveDirective() error = %q, want lowercase text", err)
	}
	if got.Kind() != DirectiveKindUnknown {
		t.Errorf("Directive.Kind() on error = %q, want %q", got.Kind(), DirectiveKindUnknown)
	}
	if _, ok := got.Stage(); ok {
		t.Error("Directive.Stage() on error reports a stage")
	}
	if !reflect.DeepEqual(got, Directive{}) {
		t.Errorf("Directive on error = %#v, want zero value", got)
	}
}
