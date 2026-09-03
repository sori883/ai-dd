package stageplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
)

func TestBuildPreservesOrderedStageEntriesAndMetadata(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("later", "1.2", map[string]any{
			"produces":          []string{"later-artifact"},
			"optional_produces": []string{"optional-artifact"},
			"consumes": []map[string]any{{
				"artifact": "first-artifact",
				"required": true,
			}},
			"requires_stage": []string{"first"},
		}),
		stageFixture("first", "1.1", map[string]any{
			"produces":       []string{"first-artifact"},
			"requires_stage": []string{},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"first": "EXECUTE",
			"later": "SKIP",
		}},
	})

	got, err := Build(Input{
		Graph:       snapshot,
		Scope:       "classic",
		ProjectType: "Brownfield",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := got.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries() length = %d, want 2", len(entries))
	}
	if entries[0].Stage.Slug != "first" || entries[1].Stage.Slug != "later" {
		t.Fatalf("Entries() slugs = (%q, %q), want (first, later)", entries[0].Stage.Slug, entries[1].Stage.Slug)
	}
	if entries[0].Action != graph.ActionExecute || entries[1].Action != graph.ActionSkip {
		t.Fatalf("Entries() actions = (%q, %q), want (EXECUTE, SKIP)", entries[0].Action, entries[1].Action)
	}
	if entries[0].Reason == "" || entries[1].Reason == "" {
		t.Fatal("Entries() reasons must explain each routing decision")
	}

	gotStage := entries[1].Stage
	wantStage := graph.Stage{
		Slug:             "later",
		Number:           "1.2",
		Name:             "later name",
		Phase:            "ideation",
		Execution:        "ALWAYS",
		LeadAgent:        "aidlc-product-agent",
		SupportAgents:    []string{},
		Mode:             "inline",
		Scopes:           nil,
		Enabled:          true,
		Produces:         []string{"later-artifact"},
		OptionalProduces: []string{"optional-artifact"},
		Consumes:         []graph.Consume{{Artifact: "first-artifact", Required: true}},
		RequiresStages:   []string{"first"},
	}
	if !reflect.DeepEqual(gotStage, wantStage) {
		t.Fatalf("entry metadata = %#v, want %#v", gotStage, wantStage)
	}

	entries[1].Stage.RequiresStages[0] = "mutated"
	again := got.Entries()
	if again[1].Stage.RequiresStages[0] != "first" {
		t.Fatalf("RequiresStages changed after returned metadata mutation: %#v", again[1].Stage.RequiresStages)
	}
	if got.Scope() != "classic" || got.ProjectType() != "Brownfield" {
		t.Fatalf("plan identity = (%q, %q), want (classic, Brownfield)", got.Scope(), got.ProjectType())
	}
}

func TestBuildAdjustsReverseEngineeringForGreenfield(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("reverse-engineering", "2.1", nil),
		stageFixture("requirements-analysis", "2.3", nil),
	}, map[string]any{
		"bugfix": map[string]any{"stages": map[string]any{
			"reverse-engineering":   "EXECUTE",
			"requirements-analysis": "EXECUTE",
		}},
	})

	got, err := Build(Input{
		Graph:       snapshot,
		Scope:       "bugfix",
		ProjectType: "Greenfield",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := got.Entries()
	if entries[0].Action != graph.ActionSkip || entries[0].Reason != "greenfield" {
		t.Fatalf("reverse-engineering entry = %#v, want greenfield SKIP", entries[0])
	}
	if entries[1].Action != graph.ActionExecute {
		t.Fatalf("requirements-analysis action = %q, want EXECUTE", entries[1].Action)
	}
	if !got.GreenfieldAdjusted() || !got.ReverseEngineeringSkippedGreenfield() {
		t.Fatal("plan did not report Greenfield reverse-engineering adjustment")
	}
}

func TestBuildRejectsUnknownInputsWithoutPartialPlan(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("stage", "1.1", nil),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"stage": "EXECUTE",
		}},
	})

	tests := []struct {
		name        string
		scope       string
		projectType string
		wantError   string
	}{
		{
			name:        "unknown scope",
			scope:       "missing",
			projectType: "Brownfield",
			wantError:   "unknown scope",
		},
		{
			name:        "unknown project type",
			scope:       "classic",
			projectType: "unknown",
			wantError:   "unknown project type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Build(Input{
				Graph:       snapshot,
				Scope:       tt.scope,
				ProjectType: tt.projectType,
			})
			requireErrorContains(t, err, tt.wantError)
			if len(got.Entries()) != 0 || got.Scope() != "" || got.ProjectType() != "" {
				t.Fatalf("failed Build() returned partial plan: %#v", got)
			}
		})
	}
}

func TestPlanAccessorsReturnDeepCopies(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("producer", "1.0", map[string]any{
			"produces": []string{"input"},
		}),
		stageFixture("stage", "1.1", map[string]any{
			"support_agents":    []string{"support"},
			"scopes":            []string{"classic"},
			"produces":          []string{"artifact"},
			"optional_produces": []string{"optional"},
			"consumes": []map[string]any{{
				"artifact": "input",
				"required": true,
			}},
			"requires_stage": []string{},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"producer": "EXECUTE",
			"stage":    "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entries := got.Entries()
	entries[1].Stage.SupportAgents[0] = "mutated"
	entries[1].Stage.Scopes[0] = "mutated"
	entries[1].Stage.Produces[0] = "mutated"
	entries[1].Stage.OptionalProduces[0] = "mutated"
	entries[1].Stage.Consumes[0].Artifact = "mutated"
	entries[1].Stage.RequiresStages = append(entries[1].Stage.RequiresStages, "mutated")

	again := got.Entries()
	if again[1].Stage.SupportAgents[0] != "support" ||
		again[1].Stage.Scopes[0] != "classic" ||
		again[1].Stage.Produces[0] != "artifact" ||
		again[1].Stage.OptionalProduces[0] != "optional" ||
		again[1].Stage.Consumes[0].Artifact != "input" ||
		len(again[1].Stage.RequiresStages) != 0 {
		t.Fatalf("Plan metadata changed after accessor mutation: %#v", again[1].Stage)
	}
}

func TestBuildRejectsRequiredArtifactWithoutProducer(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("consumer", "1.1", map[string]any{
			"consumes": []map[string]any{{
				"artifact": "missing-artifact",
				"required": true,
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"consumer": "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	requireErrorContains(t, err, "missing-artifact")
	if len(got.Entries()) != 0 {
		t.Fatalf("failed Build() returned %d entries, want no partial plan", len(got.Entries()))
	}
}

func TestBuildAdvisesWhenAllArtifactProducersAreSkipped(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("producer", "1.1", map[string]any{
			"produces": []string{"artifact"},
		}),
		stageFixture("consumer", "1.2", map[string]any{
			"produces": []string{"result"},
			"consumes": []map[string]any{{
				"artifact": "artifact",
				"required": true,
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"producer": "SKIP",
			"consumer": "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(got.ExecuteStages()) != 1 || got.ExecuteStages()[0].Stage.Slug != "consumer" {
		t.Fatalf("ExecuteStages() = %#v, want only consumer", got.ExecuteStages())
	}
	if len(got.SkipStages()) != 1 || got.SkipStages()[0].Stage.Slug != "producer" {
		t.Fatalf("SkipStages() = %#v, want only producer", got.SkipStages())
	}

	advisories := got.Advisories()
	if len(advisories) != 1 {
		t.Fatalf("Advisories() length = %d, want 1", len(advisories))
	}
	if advisories[0].StageSlug != "consumer" || advisories[0].Artifact != "artifact" ||
		!reflect.DeepEqual(advisories[0].ProducerSlugs, []string{"producer"}) {
		t.Fatalf("Advisories() = %#v, want structured off-path advisory", advisories)
	}
}

func TestBuildUsesOptionalProducersAndSkipsInactiveConsumes(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("optional-producer", "1.1", map[string]any{
			"optional_produces": []string{"optional-artifact"},
		}),
		stageFixture("consumer", "1.2", map[string]any{
			"consumes": []map[string]any{
				{"artifact": "optional-artifact", "required": true},
				{"artifact": "optional-consume", "required": false},
				{"artifact": "greenfield-only", "required": true, "conditional_on": "greenfield"},
			},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"optional-producer": "EXECUTE",
			"consumer":          "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if advisories := got.Advisories(); len(advisories) != 0 {
		t.Fatalf("Advisories() = %#v, want none for optional or inactive consumes", advisories)
	}
}

func TestBuildIgnoresMissingProducerForSkippedConsumer(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("consumer", "1.1", map[string]any{
			"consumes": []map[string]any{{
				"artifact": "missing-artifact",
				"required": true,
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"consumer": "SKIP",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v, want skipped consumer to bypass artifact validation", err)
	}
	if len(got.Advisories()) != 0 {
		t.Fatalf("Advisories() = %#v, want none for skipped consumer", got.Advisories())
	}
}

func TestBuildSuppressesAdvisoryWhenAnyArtifactProducerExecutes(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("skipped-producer", "1.1", map[string]any{
			"produces": []string{"artifact"},
		}),
		stageFixture("executed-producer", "1.2", map[string]any{
			"produces": []string{"artifact"},
		}),
		stageFixture("consumer", "1.3", map[string]any{
			"consumes": []map[string]any{{
				"artifact": "artifact",
				"required": true,
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"skipped-producer":  "SKIP",
			"executed-producer": "EXECUTE",
			"consumer":          "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v, want executing producer to satisfy artifact dependency", err)
	}
	if len(got.Advisories()) != 0 {
		t.Fatalf("Advisories() = %#v, want none when one producer executes", got.Advisories())
	}
}

func TestBuildMatchesRequiredConsumeConditionsToProjectType(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("consumer", "1.1", map[string]any{
			"consumes": []map[string]any{{
				"artifact":       "brownfield-artifact",
				"required":       true,
				"conditional_on": "brownfield",
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"consumer": "EXECUTE",
		}},
	})

	tests := []struct {
		name        string
		projectType string
		wantError   bool
	}{
		{name: "matching brownfield condition", projectType: "Brownfield", wantError: true},
		{name: "nonmatching greenfield condition", projectType: "Greenfield"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Build(Input{
				Graph:       snapshot,
				Scope:       "classic",
				ProjectType: tt.projectType,
			})
			if tt.wantError {
				requireErrorContains(t, err, "brownfield-artifact")
				if len(got.Entries()) != 0 {
					t.Fatalf("failed Build() returned partial plan: %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build() error = %v, want conditionally inactive consume to be ignored", err)
			}
		})
	}
}

func TestBuildDoesNotCloseOverSkippedRequiredStageDependencies(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("dependency", "1.1", map[string]any{
			"produces": []string{"dependency-artifact"},
		}),
		stageFixture("consumer", "1.2", map[string]any{
			"requires_stage": []string{"dependency"},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"dependency": "SKIP",
			"consumer":   "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(got.ExecuteStages()) != 1 || got.ExecuteStages()[0].Stage.Slug != "consumer" {
		t.Fatalf("ExecuteStages() = %#v, want no dependency closure", got.ExecuteStages())
	}
	if len(got.SkipStages()) != 1 || got.SkipStages()[0].Stage.Slug != "dependency" {
		t.Fatalf("SkipStages() = %#v, want skipped dependency", got.SkipStages())
	}
	if got.Entries()[1].Stage.RequiresStages[0] != "dependency" {
		t.Fatalf("RequiresStages = %#v, want metadata preserved", got.Entries()[1].Stage.RequiresStages)
	}
}

func TestPlanRoutingAccessorsReturnDeepCopies(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("producer", "1.1", map[string]any{
			"produces": []string{"artifact"},
		}),
		stageFixture("consumer", "1.2", map[string]any{
			"produces": []string{"result"},
			"consumes": []map[string]any{{
				"artifact": "artifact",
				"required": true,
			}},
		}),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"producer": "SKIP",
			"consumer": "EXECUTE",
		}},
	})

	got, err := Build(Input{Graph: snapshot, Scope: "classic", ProjectType: "Brownfield"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	execute := got.ExecuteStages()
	skip := got.SkipStages()
	advisories := got.Advisories()
	execute[0].Stage.Produces[0] = "mutated"
	skip[0].Stage.Produces[0] = "mutated"
	advisories[0].ProducerSlugs[0] = "mutated"

	againExecute := got.ExecuteStages()
	againSkip := got.SkipStages()
	againAdvisories := got.Advisories()
	if againExecute[0].Stage.Produces[0] != "result" ||
		againSkip[0].Stage.Produces[0] != "artifact" ||
		againAdvisories[0].ProducerSlugs[0] != "producer" {
		t.Fatalf("routing accessor mutation changed plan: execute=%#v skip=%#v advisories=%#v", againExecute, againSkip, againAdvisories)
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

func stageFixture(slug, number string, overrides map[string]any) map[string]any {
	fixture := map[string]any{
		"slug":           slug,
		"number":         number,
		"name":           slug + " name",
		"phase":          "ideation",
		"execution":      "ALWAYS",
		"lead_agent":     "aidlc-product-agent",
		"support_agents": []string{},
		"mode":           "inline",
		"produces":       []string{},
		"consumes":       []map[string]any{},
		"requires_stage": []string{},
	}
	for key, value := range overrides {
		fixture[key] = value
	}
	return fixture
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want context %q", err, want)
	}
}
