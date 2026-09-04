package graph

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
)

const routeTargetNodeJSON = `{"slug":"raw-stage","number":"1.1","name":"Raw Stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["mvp"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[],"inputs":["requirements.md"],"outputs":["design.md"],"sensors_applicable":["review-gate"],"review_detail":{"owner":"reviewer","required":true}}`

const routeTargetNodeChangedJSON = `{"slug":"raw-stage","number":"1.1","name":"Raw Stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["mvp"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[],"inputs":["changed.md"],"outputs":["design.md"],"sensors_applicable":["review-gate"],"review_detail":{"owner":"reviewer","required":true}}`

const routeLaterNodeJSON = `{"slug":"later-stage","number":"1.2","name":"Later Stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["mvp"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}`
const routeDisabledNodeJSON = `{"slug":"disabled-stage","number":"1.3","name":"Disabled Stage","phase":"inception","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["mvp"],"enabled":false,"produces":[],"consumes":[],"requires_stage":[]}`

const routeScopeGridJSON = `{"mvp":{"stages":{"raw-stage":"EXECUTE","later-stage":"EXECUTE"}}}`
const routeScopeGridChangedJSON = `{"mvp":{"stages":{"raw-stage":"EXECUTE","later-stage":"SKIP"}}}`
const routeScopeGridWithDisabledJSON = `{"mvp":{"stages":{"raw-stage":"EXECUTE","later-stage":"EXECUTE","disabled-stage":"EXECUTE"}}}`

func routeFixtureFS(stageNode, scopeGrid string) fstest.MapFS {
	return fstest.MapFS{
		"stage-graph.json": {Data: []byte("[" + stageNode + "," + routeLaterNodeJSON + "]")},
		"scope-grid.json":  {Data: []byte(scopeGrid)},
	}
}

func routeFixtureWithDisabledStageFS() fstest.MapFS {
	return fstest.MapFS{
		"stage-graph.json": {Data: []byte("[" + routeTargetNodeJSON + "," + routeLaterNodeJSON + "," + routeDisabledNodeJSON + "]")},
		"scope-grid.json":  {Data: []byte(routeScopeGridWithDisabledJSON)},
	}
}

func TestSnapshotRouteHashBindsCanonicalNodeAndScopeStages(t *testing.T) {
	const wantGolden = "58a329deb0b35850d96e9a7209a848c06fc681af1a8ced1fa582a51c815dc2e6"

	snapshot, err := Load(routeFixtureFS(routeTargetNodeJSON, routeScopeGridJSON))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("raw node and execute stages use fixed canonical golden", func(t *testing.T) {
		got, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash() error = %v", err)
		}
		if got != wantGolden {
			t.Errorf("RouteHash() = %q, want independent golden %q", got, wantGolden)
		}
	})

	t.Run("raw field changes alter hash", func(t *testing.T) {
		changedSnapshot, err := Load(routeFixtureFS(routeTargetNodeChangedJSON, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(changed node) error = %v", err)
		}
		changed, err := changedSnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(changed node) error = %v", err)
		}
		base, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(base) error = %v", err)
		}
		if changed == base {
			t.Errorf("RouteHash() unchanged after retained raw field changed: %q", changed)
		}
	})

	t.Run("scope execute route changes alter hash", func(t *testing.T) {
		changedSnapshot, err := Load(routeFixtureFS(routeTargetNodeJSON, routeScopeGridChangedJSON))
		if err != nil {
			t.Fatalf("Load(changed scope) error = %v", err)
		}
		changed, err := changedSnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(changed scope) error = %v", err)
		}
		base, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(base) error = %v", err)
		}
		if changed == base {
			t.Errorf("RouteHash() unchanged after scope EXECUTE route changed: %q", changed)
		}
	})
}

func TestSnapshotRouteHashRejectsUnknownRoute(t *testing.T) {
	snapshot, err := Load(routeFixtureFS(routeTargetNodeJSON, routeScopeGridJSON))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name  string
		stage string
		scope string
	}{
		{name: "unknown stage", stage: "missing-stage", scope: "mvp"},
		{name: "unknown scope", stage: "raw-stage", scope: "missing-scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := snapshot.RouteHash(tt.stage, tt.scope)
			if err == nil {
				t.Fatalf("RouteHash(%q, %q) error = nil", tt.stage, tt.scope)
			}
			if got != "" {
				t.Errorf("RouteHash(%q, %q) hash = %q, want empty on error", tt.stage, tt.scope, got)
			}
		})
	}

	t.Run("disabled stage is not a route target", func(t *testing.T) {
		disabledSnapshot, err := Load(routeFixtureWithDisabledStageFS())
		if err != nil {
			t.Fatalf("Load(disabled fixture) error = %v", err)
		}
		got, err := disabledSnapshot.RouteHash("disabled-stage", "mvp")
		if err == nil {
			t.Fatal("RouteHash(disabled-stage, mvp) error = nil")
		}
		if got != "" {
			t.Errorf("RouteHash(disabled-stage, mvp) hash = %q, want empty on error", got)
		}
	})

	t.Run("fixed distribution golden", func(t *testing.T) {
		const want = "b2b7deca926d64c0e55225db06e10e202c06ac6f0c26f759070f825146525d23"

		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("runtime.Caller(0) failed")
		}
		dataRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "docs", "配布_ai-dlc", ".codex", "tools", "data")
		fixedSnapshot, err := Load(os.DirFS(dataRoot))
		if err != nil {
			t.Fatalf("Load(fixed distribution) error = %v", err)
		}
		got, err := fixedSnapshot.RouteHash("intent-capture", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(intent-capture, mvp) error = %v", err)
		}
		if got != want {
			t.Errorf("RouteHash(intent-capture, mvp) = %q, want independent golden %q", got, want)
		}
	})
}
