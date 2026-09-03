package graph

import (
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadPreservesEnabledStageOrder(t *testing.T) {
	t.Parallel()

	first := stageFixture("first", "1.1")
	first["support_agents"] = []string{"reviewer"}
	first["scopes"] = []string{"classic"}
	first["future_stage_field"] = map[string]any{"ignored": true}
	disabled := stageFixture("disabled", "1.2")
	disabled["enabled"] = false
	last := stageFixture("last", "1.3")
	last["execution"] = "CONDITIONAL"

	snapshot, err := Load(fixtureFS(t, []any{first, disabled, last}, map[string]any{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []Stage{
		{
			Slug:           "first",
			Number:         "1.1",
			Name:           "first name",
			Phase:          "inception",
			Execution:      "ALWAYS",
			LeadAgent:      "orchestrator",
			SupportAgents:  []string{"reviewer"},
			Mode:           "inline",
			Scopes:         []string{"classic"},
			Enabled:        true,
			Produces:       []string{},
			Consumes:       []Consume{},
			RequiresStages: []string{},
		},
		{
			Slug:           "last",
			Number:         "1.3",
			Name:           "last name",
			Phase:          "inception",
			Execution:      "CONDITIONAL",
			LeadAgent:      "orchestrator",
			SupportAgents:  []string{},
			Mode:           "inline",
			Scopes:         []string{},
			Enabled:        true,
			Produces:       []string{},
			Consumes:       []Consume{},
			RequiresStages: []string{},
		},
	}
	if got := snapshot.Stages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stages() = %#v, want %#v", got, want)
	}
}

func TestLoadPreservesStageArtifactMetadata(t *testing.T) {
	t.Parallel()

	first := stageFixture("first", "1.1")
	first["produces"] = []string{"intent-statement", "stakeholder-map"}
	first["optional_produces"] = []string{"questions"}
	first["consumes"] = []map[string]any{
		{"artifact": "project-description", "required": true},
		{"artifact": "workspace-summary", "required": false, "conditional_on": "brownfield"},
	}
	first["requires_stage"] = []string{}
	second := stageFixture("second", "1.2")
	second["produces"] = []string{"requirements"}
	second["consumes"] = []map[string]any{
		{"artifact": "intent-statement", "required": true, "conditional_on": "greenfield"},
	}
	second["requires_stage"] = []string{"first"}

	snapshot, err := Load(fixtureFS(t, []any{first, second}, map[string]any{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []Stage{
		{
			Slug:             "first",
			Number:           "1.1",
			Name:             "first name",
			Phase:            "inception",
			Execution:        "ALWAYS",
			LeadAgent:        "orchestrator",
			SupportAgents:    []string{},
			Mode:             "inline",
			Scopes:           []string{},
			Enabled:          true,
			Produces:         []string{"intent-statement", "stakeholder-map"},
			OptionalProduces: []string{"questions"},
			Consumes: []Consume{
				{Artifact: "project-description", Required: true},
				{Artifact: "workspace-summary", Required: false, ConditionalOn: "brownfield"},
			},
			RequiresStages: []string{},
		},
		{
			Slug:          "second",
			Number:        "1.2",
			Name:          "second name",
			Phase:         "inception",
			Execution:     "ALWAYS",
			LeadAgent:     "orchestrator",
			SupportAgents: []string{},
			Mode:          "inline",
			Scopes:        []string{},
			Enabled:       true,
			Produces:      []string{"requirements"},
			Consumes: []Consume{
				{Artifact: "intent-statement", Required: true, ConditionalOn: "greenfield"},
			},
			RequiresStages: []string{"first"},
		},
	}
	if got := snapshot.Stages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stages() = %#v, want %#v", got, want)
	}
}

func TestLoadPreservesStageCompletionMetadata(t *testing.T) {
	t.Parallel()

	stage := stageFixture("completion", "1.1")
	stage["for_each"] = "unit-of-work"
	stage["workspace_requires"] = true
	stage["reviewer"] = "reviewer-agent"
	stage["summary_confirmation"] = "required"
	stage["sensors"] = []string{"required-sections", "blocking-check"}
	stage["produces"] = []string{"artifact"}
	stage["produces_kinds"] = map[string]any{"artifact": []string{"service", "ui"}}

	snapshot, err := Load(fixtureFS(t, []any{stage}, map[string]any{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := snapshot.Stages()[0]
	if got.ForEach != "unit-of-work" {
		t.Errorf("Stage.ForEach = %q, want unit-of-work", got.ForEach)
	}
	if !got.WorkspaceRequires {
		t.Error("Stage.WorkspaceRequires = false, want true")
	}
	if got.Reviewer != "reviewer-agent" {
		t.Errorf("Stage.Reviewer = %q, want reviewer-agent", got.Reviewer)
	}
	if got.SummaryConfirmation != "required" {
		t.Errorf("Stage.SummaryConfirmation = %q, want required", got.SummaryConfirmation)
	}
	if got.Sensors == nil || !reflect.DeepEqual(got.Sensors, []string{"required-sections", "blocking-check"}) {
		t.Errorf("Stage.Sensors = %#v, want required-sections and blocking-check", got.Sensors)
	}
	if got.ProducesKinds == nil || !reflect.DeepEqual(got.ProducesKinds, map[string][]string{"artifact": {"service", "ui"}}) {
		t.Errorf("Stage.ProducesKinds = %#v, want artifact applicability", got.ProducesKinds)
	}
}

func TestLoadRejectsInvalidStageCompletionMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value any
	}{
		{name: "for_each", field: "for_each", value: true},
		{name: "workspace_requires", field: "workspace_requires", value: "true"},
		{name: "reviewer", field: "reviewer", value: false},
		{name: "summary_confirmation", field: "summary_confirmation", value: "sometimes"},
		{name: "sensors", field: "sensors", value: "blocking-check"},
		{name: "produces_kinds", field: "produces_kinds", value: "artifact"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := stageFixture("completion", "1.1")
			stage[tt.field] = tt.value
			if _, err := Load(fixtureFS(t, []any{stage}, map[string]any{})); err == nil {
				t.Fatalf("Load() error = nil for invalid %s", tt.field)
			}
		})
	}
}

func TestLoadValidatesStageConsumes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		consumes any
		wantErr  string
	}{
		{
			name:     "missing artifact",
			consumes: []any{map[string]any{"required": true}},
			wantErr:  "artifact is required",
		},
		{
			name:     "null artifact",
			consumes: []any{map[string]any{"artifact": nil, "required": true}},
			wantErr:  `field "artifact" must be a string`,
		},
		{
			name:     "empty artifact",
			consumes: []any{map[string]any{"artifact": "", "required": true}},
			wantErr:  "artifact is required",
		},
		{
			name:     "missing required",
			consumes: []any{map[string]any{"artifact": "intent-statement"}},
			wantErr:  "required is required",
		},
		{
			name:     "null required",
			consumes: []any{map[string]any{"artifact": "intent-statement", "required": nil}},
			wantErr:  `field "required" must be a boolean`,
		},
		{
			name:     "invalid conditional",
			consumes: []any{map[string]any{"artifact": "intent-statement", "required": true, "conditional_on": "any"}},
			wantErr:  "conditional_on",
		},
		{
			name:     "empty conditional",
			consumes: []any{map[string]any{"artifact": "intent-statement", "required": true, "conditional_on": ""}},
			wantErr:  "conditional_on",
		},
		{
			name:     "wrong case artifact key",
			consumes: []any{map[string]any{"Artifact": "intent-statement", "required": true}},
			wantErr:  "artifact is required",
		},
		{
			name:     "wrong case required key",
			consumes: []any{map[string]any{"artifact": "intent-statement", "Required": true}},
			wantErr:  "required is required",
		},
		{
			name: "required false is preserved",
			consumes: []any{map[string]any{
				"artifact": "intent-statement", "required": false,
			}},
		},
		{
			name: "conditional values are preserved",
			consumes: []any{map[string]any{
				"artifact": "intent-statement", "required": true, "conditional_on": "greenfield",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := stageFixture("stage", "1.1")
			stage["consumes"] = tt.consumes
			got, err := Load(fixtureFS(t, []any{stage}, map[string]any{}))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, snapshot = %#v", got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Load() error = %q, want context %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			consume := got.Stages()[0].Consumes[0]
			if tt.name == "required false is preserved" && consume.Required {
				t.Errorf("Consumes()[0].Required = true, want false")
			}
			if tt.name == "conditional values are preserved" && consume.ConditionalOn != "greenfield" {
				t.Errorf("Consumes()[0].ConditionalOn = %q, want greenfield", consume.ConditionalOn)
			}
		})
	}
}

func TestLoadRejectsMalformedConsumeRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		consumes string
		wantErr  string
	}{
		{name: "null row", consumes: `[null]`, wantErr: "must be an object"},
		{name: "array row", consumes: `[[]]`, wantErr: "cannot unmarshal array"},
		{name: "non-string artifact", consumes: `[{"artifact":1,"required":true}]`, wantErr: "artifact"},
		{name: "non-boolean required", consumes: `[{"artifact":"intent-statement","required":"true"}]`, wantErr: "required"},
		{name: "non-string conditional", consumes: `[{"artifact":"intent-statement","required":true,"conditional_on":1}]`, wantErr: "conditional_on"},
		{name: "null conditional", consumes: `[{"artifact":"intent-statement","required":true,"conditional_on":null}]`, wantErr: "conditional_on"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := stageFixture("stage", "1.1")
			stage["consumes"] = json.RawMessage(tt.consumes)
			got, err := Load(fixtureFS(t, []any{stage}, map[string]any{}))
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want context %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadAllowsMissingOrEmptyOptionalProduces(t *testing.T) {
	t.Parallel()

	missing := stageFixture("missing", "1.1")
	empty := stageFixture("empty", "1.2")
	empty["optional_produces"] = []string{}

	snapshot, err := Load(fixtureFS(t, []any{missing, empty}, map[string]any{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	stages := snapshot.Stages()
	if stages[0].OptionalProduces != nil {
		t.Errorf("missing OptionalProduces = %#v, want nil", stages[0].OptionalProduces)
	}
	if stages[1].OptionalProduces == nil || len(stages[1].OptionalProduces) != 0 {
		t.Errorf("empty OptionalProduces = %#v, want non-nil empty slice", stages[1].OptionalProduces)
	}
}

func TestLoadValidatesRequiresStageEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stages     []any
		wantErr    string
		wantStages int
	}{
		{
			name: "unknown dependency",
			stages: []any{
				stageWithRequires("stage", "1.2", "missing"),
			},
			wantErr: "unknown stage",
		},
		{
			name: "forward dependency",
			stages: []any{
				stageWithRequires("first", "1.1", "second"),
				stageFixture("second", "1.2"),
			},
			wantErr: "must precede",
		},
		{
			name: "self dependency",
			stages: []any{
				stageWithRequires("stage", "1.1", "stage"),
			},
			wantErr: "must precede",
		},
		{
			name: "duplicate dependency",
			stages: []any{
				stageWithRequires("first", "1.1"),
				stageWithRequires("second", "1.2", "first", "first"),
			},
			wantErr: "duplicates",
		},
		{
			name: "disabled dependency remains known",
			stages: []any{
				stageWithRequires("disabled", "1.1"),
				stageWithRequires("second", "1.2", "disabled"),
			},
			wantStages: 1,
		},
		{
			name: "input order does not replace number order",
			stages: []any{
				stageWithRequires("second", "1.2", "first"),
				stageFixture("first", "1.1"),
			},
			wantStages: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.name == "disabled dependency remains known" {
				testStage := tt.stages[0].(map[string]any)
				testStage["enabled"] = false
			}
			got, err := Load(fixtureFS(t, tt.stages, map[string]any{}))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, snapshot = %#v", got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Load() error = %q, want context %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if gotStages := len(got.Stages()); gotStages != tt.wantStages {
				t.Errorf("Stages() length = %d, want %d", gotStages, tt.wantStages)
			}
		})
	}
}

func stageWithRequires(slug, number string, dependencies ...string) map[string]any {
	stage := stageFixture(slug, number)
	if dependencies == nil {
		dependencies = []string{}
	}
	stage["requires_stage"] = dependencies
	return stage
}

func TestLoadRequiresExactStageFieldNames(t *testing.T) {
	t.Parallel()

	const remainingFields = `,"number":"1.1","name":"stage name","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":[],"consumes":[],"requires_stage":[]}`
	tests := []struct {
		name       string
		slugFields string
		wantSlug   string
		wantError  bool
	}{
		{
			name:       "wrong case alone is unknown",
			slugFields: `"Slug":"wrong"`,
			wantError:  true,
		},
		{
			name:       "exact key before alias wins",
			slugFields: `"slug":"exact","Slug":"wrong"`,
			wantSlug:   "exact",
		},
		{
			name:       "exact key after alias wins",
			slugFields: `"Slug":"wrong","slug":"exact"`,
			wantSlug:   "exact",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataFS := fstest.MapFS{
				"stage-graph.json": {Data: []byte(`[{` + tt.slugFields + remainingFields + `]`)},
				"scope-grid.json":  {Data: []byte(`{}`)},
			}
			snapshot, err := Load(dataFS)
			if tt.wantError {
				if err == nil {
					t.Fatalf("Load() error = nil, Stages() = %#v", snapshot.Stages())
				}
				if !strings.Contains(err.Error(), "slug is required") {
					t.Errorf("Load() error = %q, want slug-required context", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got := snapshot.Stages()[0].Slug; got != tt.wantSlug {
				t.Errorf("Stages()[0].Slug = %q, want %q", got, tt.wantSlug)
			}
		})
	}
}

func TestLoadRequiresExactMetadataFieldNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		field      string
		wrongKey   string
		wrongValue any
	}{
		{name: "produces", field: "produces", wrongKey: "Produces", wrongValue: []string{}},
		{name: "consumes", field: "consumes", wrongKey: "Consumes", wrongValue: []any{}},
		{name: "requires stage", field: "requires_stage", wrongKey: "Requires_Stage", wrongValue: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := stageFixture("stage", "1.1")
			delete(stage, tt.field)
			stage[tt.wrongKey] = tt.wrongValue
			got, err := Load(fixtureFS(t, []any{stage}, map[string]any{}))
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if !strings.Contains(err.Error(), tt.field) {
				t.Errorf("Load() error = %q, want field context %q", err, tt.field)
			}
		})
	}

	stage := stageFixture("optional", "1.1")
	stage["Optional_Produces"] = []string{"ignored"}
	snapshot, err := Load(fixtureFS(t, []any{stage}, map[string]any{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := snapshot.Stages()[0].OptionalProduces; got != nil {
		t.Errorf("OptionalProduces = %#v, want nil for unknown case-variant key", got)
	}
}

func TestLoadRoutesScopes(t *testing.T) {
	t.Parallel()

	first := stageFixture("first", "1.1")
	disabled := stageFixture("disabled", "1.2")
	disabled["enabled"] = false
	last := stageFixture("last", "1.3")
	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: mustJSON(t, []any{first, disabled, last})},
		"scope-grid.json": {Data: []byte(`{
			"feature":{"stages":{"first":"EXECUTE","disabled":"EXECUTE","last":"SKIP"},"future_scope_metadata":{"ignored":true}},
			"classic":{"stages":{"first":"SKIP"}}
		}`)},
	}

	snapshot, err := Load(dataFS)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := snapshot.ScopeNames(), []string{"classic", "feature"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ScopeNames() = %v, want %v", got, want)
	}

	feature, ok := snapshot.Scope("feature")
	if !ok {
		t.Fatal("Scope(\"feature\") not found")
	}
	if got := feature.Action("first"); got != ActionExecute {
		t.Errorf("Action(first) = %q, want %q", got, ActionExecute)
	}
	if got := feature.Action("last"); got != ActionSkip {
		t.Errorf("Action(last) = %q, want %q", got, ActionSkip)
	}
	if got := feature.Action("missing"); got != ActionSkip {
		t.Errorf("Action(missing) = %q, want default %q", got, ActionSkip)
	}
	if _, exists := feature.Actions()["disabled"]; exists {
		t.Error("Actions() contains disabled stage")
	}
	if _, ok := snapshot.Scope("unknown"); ok {
		t.Error("Scope(\"unknown\") found, want false")
	}
}

func TestLoadSortsScopeNamesByJavaScriptUTF16Order(t *testing.T) {
	t.Parallel()

	stage := stageFixture("stage", "1.1")
	stage["scopes"] = []string{"\ue000", "😀"}
	graphData := mustJSON(t, []any{stage})
	tests := []struct {
		name     string
		gridData []byte
	}{
		{
			name: "explicit grid",
			gridData: []byte(`{
				"\ue000":{"stages":{"stage":"SKIP"}},
				"😀":{"stages":{"stage":"EXECUTE"}}
			}`),
		},
		{name: "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataFS := fstest.MapFS{"stage-graph.json": {Data: graphData}}
			if tt.gridData != nil {
				dataFS["scope-grid.json"] = &fstest.MapFile{Data: tt.gridData}
			}
			snapshot, err := Load(dataFS)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got, want := snapshot.ScopeNames(), []string{"😀", "\ue000"}; !reflect.DeepEqual(got, want) {
				t.Errorf("ScopeNames() = %q, want JavaScript UTF-16 order %q", got, want)
			}
		})
	}
}

func TestLoadFallsBackToStageScopesForUnavailableGrid(t *testing.T) {
	t.Parallel()

	first := stageFixture("first", "1.1")
	first["scopes"] = []string{"zeta", "alpha"}
	disabled := stageFixture("disabled", "1.2")
	disabled["enabled"] = false
	disabled["scopes"] = []string{"beta"}
	last := stageFixture("last", "1.3")
	last["scopes"] = []string{"alpha"}
	graphData := mustJSON(t, []any{first, disabled, last})

	tests := []struct {
		name     string
		gridData []byte
	}{
		{name: "missing"},
		{name: "invalid JSON", gridData: []byte(`{"alpha":`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataFS := fstest.MapFS{"stage-graph.json": {Data: graphData}}
			if tt.gridData != nil {
				dataFS["scope-grid.json"] = &fstest.MapFile{Data: tt.gridData}
			}

			snapshot, err := Load(dataFS)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got, want := snapshot.ScopeNames(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("ScopeNames() = %v, want %v", got, want)
			}
			alpha, ok := snapshot.Scope("alpha")
			if !ok {
				t.Fatal("Scope(\"alpha\") not found")
			}
			if got := alpha.Actions(); !reflect.DeepEqual(got, map[string]Action{
				"first": ActionExecute,
				"last":  ActionExecute,
			}) {
				t.Errorf("alpha.Actions() = %v", got)
			}
			zeta, ok := snapshot.Scope("zeta")
			if !ok {
				t.Fatal("Scope(\"zeta\") not found")
			}
			if got := zeta.Actions(); !reflect.DeepEqual(got, map[string]Action{
				"first": ActionExecute,
				"last":  ActionSkip,
			}) {
				t.Errorf("zeta.Actions() = %v", got)
			}
		})
	}
}

func TestLoadReportsStageGraphErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dataFS      fstest.MapFS
		wantContext string
		wantCause   error
	}{
		{
			name:        "missing",
			dataFS:      fstest.MapFS{},
			wantContext: "load stage graph: read stage-graph.json",
			wantCause:   fs.ErrNotExist,
		},
		{
			name: "invalid JSON",
			dataFS: fstest.MapFS{
				"stage-graph.json": {Data: []byte(`[{`)},
			},
			wantContext: "load stage graph: decode stage-graph.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Load(tt.dataFS)
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantContext) {
				t.Errorf("Load() error = %q, want context %q", err, tt.wantContext)
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Errorf("errors.Is(%v, %v) = false", err, tt.wantCause)
			}
			if !reflect.DeepEqual(got, Snapshot{}) {
				t.Errorf("Load() snapshot on error = %#v, want zero value", got)
			}
		})
	}
}

func TestLoadRejectsInvalidStageGraphStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		stages      any
		wantContext string
	}{
		{name: "null graph", stages: nil, wantContext: "top-level array"},
		{name: "missing slug", stages: []any{stageWithout("slug")}, wantContext: "slug"},
		{name: "missing number", stages: []any{stageWithout("number")}, wantContext: "number"},
		{name: "missing name", stages: []any{stageWithout("name")}, wantContext: "name"},
		{name: "missing phase", stages: []any{stageWithout("phase")}, wantContext: "phase"},
		{name: "missing execution", stages: []any{stageWithout("execution")}, wantContext: "execution"},
		{name: "missing lead agent", stages: []any{stageWithout("lead_agent")}, wantContext: "lead_agent"},
		{name: "missing support agents", stages: []any{stageWithout("support_agents")}, wantContext: "support_agents"},
		{name: "null support agents", stages: []any{stageWith("support_agents", nil)}, wantContext: "support_agents"},
		{name: "missing mode", stages: []any{stageWithout("mode")}, wantContext: "mode"},
		{name: "missing produces", stages: []any{stageWithout("produces")}, wantContext: "produces"},
		{name: "null produces", stages: []any{stageWith("produces", nil)}, wantContext: "produces"},
		{name: "null optional produces", stages: []any{stageWith("optional_produces", nil)}, wantContext: "optional_produces"},
		{name: "missing consumes", stages: []any{stageWithout("consumes")}, wantContext: "consumes"},
		{name: "null consumes", stages: []any{stageWith("consumes", nil)}, wantContext: "consumes"},
		{name: "missing requires stage", stages: []any{stageWithout("requires_stage")}, wantContext: "requires_stage"},
		{name: "null requires stage", stages: []any{stageWith("requires_stage", nil)}, wantContext: "requires_stage"},
		{name: "invalid execution", stages: []any{stageWith("execution", "SOMETIMES")}, wantContext: "execution"},
		{
			name: "duplicate slug",
			stages: []any{
				stageFixture("same", "1.1"),
				stageFixture("same", "1.2"),
			},
			wantContext: "duplicate slug",
		},
		{
			name: "duplicate number",
			stages: []any{
				stageFixture("first", "1.1"),
				stageFixture("second", "1.1"),
			},
			wantContext: "duplicate number",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Load(fixtureFS(t, tt.stages, map[string]any{}))
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if !strings.Contains(err.Error(), tt.wantContext) {
				t.Errorf("Load() error = %q, want context %q", err, tt.wantContext)
			}
			if !reflect.DeepEqual(got, Snapshot{}) {
				t.Errorf("Load() snapshot on error = %#v, want zero value", got)
			}
		})
	}
}

func TestLoadRejectsWrongStageJSONTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		graph string
	}{
		{name: "top-level object", graph: `{}`},
		{name: "support agents object", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":{},"mode":"inline"}]`},
		{name: "support agent non-string", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[1],"mode":"inline"}]`},
		{name: "produces object", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":{},"consumes":[],"requires_stage":[]}]`},
		{name: "produces non-string", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":[1],"consumes":[],"requires_stage":[]}]`},
		{name: "optional produces object", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":[],"optional_produces":{},"consumes":[],"requires_stage":[]}]`},
		{name: "consumes object", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":[],"consumes":{},"requires_stage":[]}]`},
		{name: "requires stage object", graph: `[{"slug":"stage","number":"1.1","name":"stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","produces":[],"consumes":[],"requires_stage":{}}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataFS := fstest.MapFS{"stage-graph.json": {Data: []byte(tt.graph)}}
			got, err := Load(dataFS)
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if want := "load stage graph: decode stage-graph.json"; !strings.Contains(err.Error(), want) {
				t.Errorf("Load() error = %q, want context %q", err, want)
			}
		})
	}
}

func TestLoadRejectsStructurallyInvalidScopeGrid(t *testing.T) {
	t.Parallel()

	stage := stageFixture("stage", "1.1")
	stage["scopes"] = []string{"fallback-must-not-hide-errors"}
	graphData := mustJSON(t, []any{stage})
	tests := []struct {
		name string
		grid string
	}{
		{name: "null grid", grid: `null`},
		{name: "array grid", grid: `[]`},
		{name: "null scope", grid: `{"classic":null}`},
		{name: "array scope", grid: `{"classic":[]}`},
		{name: "missing stages", grid: `{"classic":{}}`},
		{name: "null stages", grid: `{"classic":{"stages":null}}`},
		{name: "array stages", grid: `{"classic":{"stages":[]}}`},
		{name: "non-string action", grid: `{"classic":{"stages":{"stage":1}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dataFS := fstest.MapFS{
				"stage-graph.json": {Data: graphData},
				"scope-grid.json":  {Data: []byte(tt.grid)},
			}
			got, err := Load(dataFS)
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if want := "load scope grid: validate scope-grid.json"; !strings.Contains(err.Error(), want) {
				t.Errorf("Load() error = %q, want context %q", err, want)
			}
			if !reflect.DeepEqual(got, Snapshot{}) {
				t.Errorf("Load() snapshot on error = %#v, want zero value", got)
			}
		})
	}
}

func TestLoadRejectsInvalidScopeActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		actions     map[string]any
		wantContext string
	}{
		{
			name:        "unknown action",
			actions:     map[string]any{"stage": "RUN"},
			wantContext: "invalid action",
		},
		{
			name:        "unknown stage",
			actions:     map[string]any{"unknown": "EXECUTE"},
			wantContext: "unknown stage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			grid := map[string]any{
				"classic": map[string]any{"stages": tt.actions},
			}
			got, err := Load(fixtureFS(t, []any{stageFixture("stage", "1.1")}, grid))
			if err == nil {
				t.Fatalf("Load() error = nil, snapshot = %#v", got)
			}
			if !strings.Contains(err.Error(), tt.wantContext) {
				t.Errorf("Load() error = %q, want context %q", err, tt.wantContext)
			}
			if !reflect.DeepEqual(got, Snapshot{}) {
				t.Errorf("Load() snapshot on error = %#v, want zero value", got)
			}
		})
	}
}

func TestSnapshotReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	stage := stageFixture("stage", "1.2")
	stage["support_agents"] = []string{"reviewer"}
	stage["scopes"] = []string{"classic"}
	stage["produces"] = []string{"intent-statement"}
	stage["optional_produces"] = []string{"questions"}
	stage["consumes"] = []map[string]any{{"artifact": "project-description", "required": true}}
	stage["requires_stage"] = []string{"dependency"}
	dependency := stageFixture("dependency", "1.1")
	grid := map[string]any{
		"classic": map[string]any{
			"stages": map[string]any{"stage": "EXECUTE", "dependency": "EXECUTE"},
		},
	}
	snapshot, err := Load(fixtureFS(t, []any{stage, dependency}, grid))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	stages := snapshot.Stages()
	stages[0].Slug = "changed"
	stages[0].SupportAgents[0] = "changed"
	stages[0].Scopes[0] = "changed"
	stages[0].Produces[0] = "changed"
	stages[0].OptionalProduces[0] = "changed"
	stages[0].Consumes[0].Artifact = "changed"
	stages[0].RequiresStages[0] = "changed"
	if got, want := snapshot.Stages()[0], (Stage{
		Slug:             "stage",
		Number:           "1.2",
		Name:             "stage name",
		Phase:            "inception",
		Execution:        "ALWAYS",
		LeadAgent:        "orchestrator",
		SupportAgents:    []string{"reviewer"},
		Mode:             "inline",
		Scopes:           []string{"classic"},
		Enabled:          true,
		Produces:         []string{"intent-statement"},
		OptionalProduces: []string{"questions"},
		Consumes:         []Consume{{Artifact: "project-description", Required: true}},
		RequiresStages:   []string{"dependency"},
	}); !reflect.DeepEqual(got, want) {
		t.Errorf("Stages() after caller mutation = %#v, want %#v", got, want)
	}

	names := snapshot.ScopeNames()
	names[0] = "changed"
	if got, want := snapshot.ScopeNames(), []string{"classic"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ScopeNames() after caller mutation = %v, want %v", got, want)
	}

	scope, ok := snapshot.Scope("classic")
	if !ok {
		t.Fatal("Scope(\"classic\") not found")
	}
	actions := scope.Actions()
	actions["stage"] = ActionSkip
	actions["new"] = ActionExecute
	if got := scope.Action("stage"); got != ActionExecute {
		t.Errorf("Action(stage) after Actions mutation = %q, want %q", got, ActionExecute)
	}
	if _, exists := scope.Actions()["new"]; exists {
		t.Error("Actions() retained caller-added map entry")
	}
}

func TestLoadRejectsNilFSWithoutPanicking(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Load(nil) panicked: %v", recovered)
		}
	}()
	got, err := Load(nil)
	if err == nil {
		t.Fatal("Load(nil) error = nil")
	}
	if !strings.Contains(err.Error(), "nil filesystem") {
		t.Errorf("Load(nil) error = %q, want nil filesystem context", err)
	}
	if !reflect.DeepEqual(got, Snapshot{}) {
		t.Errorf("Load(nil) snapshot = %#v, want zero value", got)
	}
}

func stageWithout(field string) map[string]any {
	stage := stageFixture("stage", "1.1")
	delete(stage, field)
	return stage
}

func stageWith(field string, value any) map[string]any {
	stage := stageFixture("stage", "1.1")
	stage[field] = value
	return stage
}

func stageFixture(slug, number string) map[string]any {
	return map[string]any{
		"slug":           slug,
		"number":         number,
		"name":           slug + " name",
		"phase":          "inception",
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

func fixtureFS(t *testing.T, stages, scopes any) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{
		"stage-graph.json": {Data: mustJSON(t, stages)},
		"scope-grid.json":  {Data: mustJSON(t, scopes)},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%#v): %v", value, err)
	}
	return data
}
