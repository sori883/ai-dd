package graph

import (
	"encoding/json"
	"strings"
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

const fixedIntentCaptureNodeJSON = `{"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","condition":"First stage of every workflow — establishes the initiative's foundation","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"requires_stage":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"scopes":["enterprise","feature","mvp","poc"],"reviewer":"aidlc-product-lead-agent","review_artifact":"intent-statement","reviewer_max_iterations":2,"review_class":"advisory","summary_confirmation":"required","inputs":"Authoritative project description (project-description utility), scope selection","outputs":"intent-statement.md, stakeholder-map.md, intent-capture-questions.md (under this stage's record dir, engine-resolved)","rules_in_context":[{"path":"aidlc/spaces/default/memory/org.md","scope":"org"},{"path":"aidlc/spaces/default/memory/team.md","scope":"team"},{"path":"aidlc/spaces/default/memory/project.md","scope":"project"},{"path":"aidlc/spaces/default/memory/phases/ideation.md","scope":"phase"}],"sensors_applicable":[{"id":"claim-sources","path":".codex/sensors/aidlc-claim-sources.md","fire_on":"gate","default_severity":"advisory","category":"document-provenance","matches":"**/{aidlc-docs,intents}/**"},{"id":"required-sections","path":".codex/sensors/aidlc-required-sections.md","fire_on":"gate","default_severity":"advisory","category":"document-shape","matches":"**/{aidlc-docs,intents}/**"},{"id":"upstream-coverage","path":".codex/sensors/aidlc-upstream-coverage.md","fire_on":"gate","default_severity":"advisory","category":"document-shape","matches":"**/{aidlc-docs,intents}/**"}]}`

var fixedDistributionRouteStages = []struct {
	number string
	slug   string
}{
	{number: "0.1", slug: "workspace-scaffold"},
	{number: "0.2", slug: "workspace-detection"},
	{number: "0.3", slug: "state-init"},
	{number: "1.1", slug: "intent-capture"},
	{number: "1.3", slug: "feasibility"},
	{number: "1.4", slug: "scope-definition"},
	{number: "1.6", slug: "rough-mockups"},
	{number: "2.1", slug: "reverse-engineering"},
	{number: "2.2", slug: "practices-discovery"},
	{number: "2.3", slug: "requirements-analysis"},
	{number: "2.4", slug: "user-stories"},
	{number: "2.5", slug: "refined-mockups"},
	{number: "2.6", slug: "domain-design"},
	{number: "2.7", slug: "units-generation"},
	{number: "2.8", slug: "contract-design"},
	{number: "2.9", slug: "delivery-planning"},
	{number: "3.1", slug: "functional-design"},
	{number: "3.2", slug: "nfr-requirements"},
	{number: "3.3", slug: "nfr-design"},
	{number: "3.4", slug: "infrastructure-design"},
	{number: "3.5", slug: "code-generation"},
	{number: "3.6", slug: "build-and-test"},
	{number: "3.7", slug: "ci-pipeline"},
}

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

func fixedDistributionRouteFS(t *testing.T) fstest.MapFS {
	t.Helper()
	type fixtureStage struct {
		Slug           string    `json:"slug"`
		Number         string    `json:"number"`
		Name           string    `json:"name"`
		Phase          string    `json:"phase"`
		Execution      string    `json:"execution"`
		LeadAgent      string    `json:"lead_agent"`
		SupportAgents  []string  `json:"support_agents"`
		Mode           string    `json:"mode"`
		Produces       []string  `json:"produces"`
		Consumes       []Consume `json:"consumes"`
		RequiresStages []string  `json:"requires_stage"`
	}
	type fixtureScope struct {
		Stages map[string]Action `json:"stages"`
	}

	nodes := make([]json.RawMessage, 0, len(fixedDistributionRouteStages))
	actions := make(map[string]Action, len(fixedDistributionRouteStages))
	for _, stage := range fixedDistributionRouteStages {
		actions[stage.slug] = ActionExecute
		if stage.slug == "intent-capture" {
			nodes = append(nodes, json.RawMessage(fixedIntentCaptureNodeJSON))
			continue
		}
		node, err := json.Marshal(fixtureStage{
			Slug:           stage.slug,
			Number:         stage.number,
			Name:           stage.slug,
			Phase:          "fixture",
			Execution:      "ALWAYS",
			LeadAgent:      "fixture-agent",
			SupportAgents:  []string{},
			Mode:           "inline",
			Produces:       []string{},
			Consumes:       []Consume{},
			RequiresStages: []string{},
		})
		if err != nil {
			t.Fatalf("json.Marshal(fixed stage %q) error = %v", stage.slug, err)
		}
		nodes = append(nodes, node)
	}

	stageGraph, err := json.Marshal(nodes)
	if err != nil {
		t.Fatalf("json.Marshal(fixed stage graph) error = %v", err)
	}
	scopeGrid, err := json.Marshal(map[string]fixtureScope{
		"mvp": {Stages: actions},
	})
	if err != nil {
		t.Fatalf("json.Marshal(fixed scope grid) error = %v", err)
	}
	return fstest.MapFS{
		"stage-graph.json": {Data: stageGraph},
		"scope-grid.json":  {Data: scopeGrid},
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

	t.Run("unicode escape spelling does not alter hash", func(t *testing.T) {
		unicodeEscapedNode := strings.Replace(routeTargetNodeJSON, `"Raw Stage"`, `"\u0052aw Stage"`, 1)
		unicodeEscapedSnapshot, err := Load(routeFixtureFS(unicodeEscapedNode, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(unicode-escaped node) error = %v", err)
		}
		got, err := unicodeEscapedSnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(unicode-escaped node) error = %v", err)
		}
		base, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(base) error = %v", err)
		}
		if got != base {
			t.Errorf("RouteHash() differs only by Unicode escape spelling: got %q, base %q", got, base)
		}
	})

	t.Run("duplicate name uses JSON parse last value", func(t *testing.T) {
		duplicateNameNode := strings.Replace(routeTargetNodeJSON, `"name":"Raw Stage"`, `"name":"Decoy","name":"Raw Stage"`, 1)
		duplicateNameSnapshot, err := Load(routeFixtureFS(duplicateNameNode, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(duplicate-name node) error = %v", err)
		}
		got, err := duplicateNameSnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(duplicate-name node) error = %v", err)
		}
		base, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(base) error = %v", err)
		}
		if got != base {
			t.Errorf("RouteHash() differs only by duplicate last-wins name: got %q, base %q", got, base)
		}
	})
}

func TestSnapshotRouteHashMatchesJavaScriptObjectAndStringEdges(t *testing.T) {
	t.Run("array-index properties sort before ordinary properties", func(t *testing.T) {
		inputNode := strings.Replace(routeTargetNodeJSON, `"review_detail":`, `"metadata":{"ordinary":"value","10":"ten","2":"two","1":"one"},"review_detail":`, 1)
		jsonStringifyNode := strings.Replace(routeTargetNodeJSON, `"review_detail":`, `"metadata":{"1":"one","2":"two","10":"ten","ordinary":"value"},"review_detail":`, 1)

		inputSnapshot, err := Load(routeFixtureFS(inputNode, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(input node) error = %v", err)
		}
		jsonStringifySnapshot, err := Load(routeFixtureFS(jsonStringifyNode, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(JSON.stringify node) error = %v", err)
		}

		got, err := inputSnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(input node) error = %v", err)
		}
		want, err := jsonStringifySnapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(JSON.stringify node) error = %v", err)
		}
		if got != want {
			t.Errorf("RouteHash() = %q, want JSON.stringify property order hash %q", got, want)
		}
	})

	t.Run("lone UTF-16 surrogates remain escaped", func(t *testing.T) {
		node := strings.Replace(routeTargetNodeJSON, `"review_detail":`, `"edge_cases":{"high":"\ud800","low":"\udc00","pair":"\ud83d\ude80","unicode":"日本語"},"review_detail":`, 1)
		snapshot, err := Load(routeFixtureFS(node, routeScopeGridJSON))
		if err != nil {
			t.Fatalf("Load(surrogate node) error = %v", err)
		}

		got, err := snapshot.RouteHash("raw-stage", "mvp")
		if err != nil {
			t.Fatalf("RouteHash(surrogate node) error = %v", err)
		}
		const want = "5f9d617d24007d1bcd19a9a5bb16a902adbef0e7aca63807cded009f71b923b0"
		if got != want {
			t.Errorf("RouteHash() = %q, want independent JSON.parse/JSON.stringify golden %q", got, want)
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

		fixedSnapshot, err := Load(fixedDistributionRouteFS(t))
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
