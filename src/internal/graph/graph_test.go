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
			Slug:          "first",
			Number:        "1.1",
			Name:          "first name",
			Phase:         "inception",
			Execution:     "ALWAYS",
			LeadAgent:     "orchestrator",
			SupportAgents: []string{"reviewer"},
			Mode:          "inline",
			Scopes:        []string{"classic"},
			Enabled:       true,
		},
		{
			Slug:          "last",
			Number:        "1.3",
			Name:          "last name",
			Phase:         "inception",
			Execution:     "CONDITIONAL",
			LeadAgent:     "orchestrator",
			SupportAgents: []string{},
			Mode:          "inline",
			Scopes:        []string{},
			Enabled:       true,
		},
	}
	if got := snapshot.Stages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stages() = %#v, want %#v", got, want)
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

	stage := stageFixture("stage", "1.1")
	stage["support_agents"] = []string{"reviewer"}
	stage["scopes"] = []string{"classic"}
	grid := map[string]any{
		"classic": map[string]any{
			"stages": map[string]any{"stage": "EXECUTE"},
		},
	}
	snapshot, err := Load(fixtureFS(t, []any{stage}, grid))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	stages := snapshot.Stages()
	stages[0].Slug = "changed"
	stages[0].SupportAgents[0] = "changed"
	stages[0].Scopes[0] = "changed"
	if got, want := snapshot.Stages()[0], (Stage{
		Slug:          "stage",
		Number:        "1.1",
		Name:          "stage name",
		Phase:         "inception",
		Execution:     "ALWAYS",
		LeadAgent:     "orchestrator",
		SupportAgents: []string{"reviewer"},
		Mode:          "inline",
		Scopes:        []string{"classic"},
		Enabled:       true,
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
