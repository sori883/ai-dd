package artifact_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
)

func TestResolvePathsProducesCanonicalPathsInDeclarationOrder(t *testing.T) {
	t.Run("required then optional paths preserve declaration order", func(t *testing.T) {
		stage := graph.Stage{
			Phase: "inception",
			Slug:  "requirements-analysis",
			Produces: []string{
				"intent-statement",
				"traceability",
				"build-test-results",
				"load-test-results",
				"intent-statement",
			},
			OptionalProduces: []string{
				"questions",
				"traceability",
				"load-test-results",
			},
		}
		want := []string{
			"inception/requirements-analysis/intent-statement.md",
			"inception/requirements-analysis/traceability.json",
			"inception/requirements-analysis/test-results.md",
			"inception/requirements-analysis/test-results.md",
			"inception/requirements-analysis/intent-statement.md",
			"inception/requirements-analysis/questions.md",
			"inception/requirements-analysis/traceability.json",
			"inception/requirements-analysis/test-results.md",
		}

		got, err := artifact.ResolvePaths(stage, graph.Snapshot{}, "brownfield")
		if err != nil {
			t.Fatalf("ResolvePaths() error = %v", err)
		}
		if !slices.Equal(got.Produces, want) {
			t.Errorf("ResolvePaths().Produces = %q, want %q", got.Produces, want)
		}
	})

	t.Run("empty outputs return non-nil empty slices", func(t *testing.T) {
		got, err := artifact.ResolvePaths(
			graph.Stage{Phase: "inception", Slug: "requirements-analysis"},
			graph.Snapshot{},
			"unknown-project-type",
		)
		if err != nil {
			t.Fatalf("ResolvePaths() error = %v", err)
		}
		if got.Consumes == nil || len(got.Consumes) != 0 {
			t.Errorf("ResolvePaths().Consumes = %#v, want non-nil empty slice", got.Consumes)
		}
		if got.Produces == nil || len(got.Produces) != 0 {
			t.Errorf("ResolvePaths().Produces = %#v, want non-nil empty slice", got.Produces)
		}
	})
}

func TestResolvePathsConsumesFromFirstProducerOrConsumerFallback(t *testing.T) {
	t.Parallel()

	firstProducer := pathTestStage("first-producer", "ideation", "1.1")
	firstProducer["produces"] = []string{"shared-artifact"}
	optionalProducer := pathTestStage("optional-producer", "construction", "2.1")
	optionalProducer["optional_produces"] = []string{"optional-artifact"}
	secondProducer := pathTestStage("second-producer", "operation", "3.1")
	secondProducer["produces"] = []string{"shared-artifact"}
	consumer := pathTestStage("consumer", "operation", "4.1")
	consumer["consumes"] = []map[string]any{
		{"artifact": "shared-artifact", "required": true},
		{"artifact": "optional-artifact", "required": false},
		{"artifact": "orphan", "required": true},
		{"artifact": "shared-artifact", "required": false},
	}
	catalog := loadPathTestCatalog(t, []map[string]any{
		firstProducer,
		optionalProducer,
		secondProducer,
		consumer,
	})

	got, err := artifact.ResolvePaths(consumerStage(catalog), catalog, "unknown-project-type")
	if err != nil {
		t.Fatalf("ResolvePaths() error = %v", err)
	}
	want := []artifact.Input{
		{Artifact: "shared-artifact", Path: "ideation/first-producer/shared-artifact.md", Required: true},
		{Artifact: "optional-artifact", Path: "construction/optional-producer/optional-artifact.md", Required: false},
		{Artifact: "orphan", Path: "operation/consumer/orphan.md", Required: true},
		{Artifact: "shared-artifact", Path: "ideation/first-producer/shared-artifact.md", Required: false},
	}
	if !slices.Equal(got.Consumes, want) {
		t.Errorf("ResolvePaths().Consumes = %#v, want %#v", got.Consumes, want)
	}
	if got.Produces == nil || len(got.Produces) != 0 {
		t.Errorf("ResolvePaths().Produces = %#v, want empty result", got.Produces)
	}
}

func pathTestStage(slug, phase, number string) map[string]any {
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

func loadPathTestCatalog(t *testing.T, stages []map[string]any) graph.Snapshot {
	t.Helper()
	stageGraph, err := json.Marshal(stages)
	if err != nil {
		t.Fatalf("json.Marshal(stages): %v", err)
	}
	catalog, err := graph.Load(fstest.MapFS{
		"stage-graph.json": {Data: stageGraph},
		"scope-grid.json":  {Data: []byte(`{}`)},
	})
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	return catalog
}

func TestResolvePathsFiltersConditionalConsumesForKnownProjectType(t *testing.T) {
	stage := graph.Stage{
		Phase: "ideation",
		Slug:  "consumer",
		Consumes: []graph.Consume{
			{Artifact: "unconditional", Required: true},
			{Artifact: "brownfield-only", Required: false, ConditionalOn: "brownfield"},
			{Artifact: "greenfield-only", Required: true, ConditionalOn: "greenfield"},
		},
	}
	all := []artifact.Input{
		{Artifact: "unconditional", Path: "ideation/consumer/unconditional.md", Required: true},
		{Artifact: "brownfield-only", Path: "ideation/consumer/brownfield-only.md", Required: false},
		{Artifact: "greenfield-only", Path: "ideation/consumer/greenfield-only.md", Required: true},
	}

	tests := []struct {
		name        string
		projectType string
		want        []artifact.Input
	}{
		{
			name:        "brownfield is case insensitive",
			projectType: "BrOwNfIeLd",
			want: []artifact.Input{
				all[0],
				all[1],
			},
		},
		{
			name:        "greenfield is case insensitive",
			projectType: "GrEeNfIeLd",
			want: []artifact.Input{
				all[0],
				all[2],
			},
		},
		{name: "empty project type keeps all", projectType: "", want: all},
		{name: "unknown project type keeps all", projectType: "prototype", want: all},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputStage := stage
			inputStage.Consumes = append([]graph.Consume(nil), stage.Consumes...)
			beforeConsumes := append([]graph.Consume(nil), inputStage.Consumes...)
			got, err := artifact.ResolvePaths(inputStage, graph.Snapshot{}, test.projectType)
			if err != nil {
				t.Fatalf("ResolvePaths() error = %v", err)
			}
			if !slices.Equal(got.Consumes, test.want) {
				t.Errorf("ResolvePaths().Consumes = %#v, want %#v", got.Consumes, test.want)
			}
			if !slices.Equal(inputStage.Consumes, beforeConsumes) {
				t.Errorf("ResolvePaths() mutated input consumes: %#v", inputStage.Consumes)
			}
		})
	}
}

func TestResolvePathsRejectsInvalidMetadataOrUnsupportedPlacement(t *testing.T) {
	invalidProducerPhase := pathTestStage("selected-producer", "ideation", "1.1")
	invalidProducerPhase["phase"] = "bad_phase"
	invalidProducerPhaseCatalog := pathTestCatalogWithProducer(t, invalidProducerPhase)

	invalidProducerSlug := pathTestStage("selected-producer", "ideation", "1.1")
	invalidProducerSlug["slug"] = "bad_slug"
	invalidProducerSlugCatalog := pathTestCatalogWithProducer(t, invalidProducerSlug)

	unsupportedProducerForEach := pathTestStage("selected-producer", "ideation", "1.1")
	unsupportedProducerForEach["for_each"] = "unit"
	unsupportedProducerForEachCatalog := pathTestCatalogWithProducer(t, unsupportedProducerForEach)

	unsupportedProducerCodeKB := pathTestStage("reverse-engineering", "inception", "1.1")
	unsupportedProducerCodeKBCatalog := pathTestCatalogWithProducer(t, unsupportedProducerCodeKB)

	unsupportedProducerKinds := pathTestStage("selected-producer", "ideation", "1.1")
	unsupportedProducerKinds["produces_kinds"] = map[string][]string{}
	unsupportedProducerKindsCatalog := pathTestCatalogWithProducer(t, unsupportedProducerKinds)

	tests := []struct {
		name        string
		stage       graph.Stage
		catalog     graph.Snapshot
		projectType string
		wantErr     error
	}{
		{
			name:    "invalid current phase",
			stage:   graph.Stage{Phase: "bad_phase", Slug: "requirements-analysis"},
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name:    "invalid current slug with path separator",
			stage:   graph.Stage{Phase: "inception", Slug: "requirements-analysis/part"},
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name: "invalid later required output does not expose partial paths",
			stage: graph.Stage{
				Phase:    "inception",
				Slug:     "requirements-analysis",
				Produces: []string{"valid-artifact", "invalid_artifact"},
			},
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name: "invalid optional output",
			stage: graph.Stage{
				Phase:            "inception",
				Slug:             "requirements-analysis",
				OptionalProduces: []string{"invalid_artifact"},
			},
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name: "invalid consume artifact",
			stage: graph.Stage{
				Phase:    "inception",
				Slug:     "requirements-analysis",
				Consumes: []graph.Consume{{Artifact: "invalid_artifact", Required: true}},
			},
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name: "invalid conditional on is rejected before filtering",
			stage: graph.Stage{
				Phase: "inception",
				Slug:  "requirements-analysis",
				Consumes: []graph.Consume{{
					Artifact:      "conditional-artifact",
					Required:      true,
					ConditionalOn: "prototype",
				}},
			},
			projectType: "BrOwNfIeLd",
			wantErr:     artifact.ErrInvalidMetadata,
		},
		{
			name: "mixed-case conditional on is not canonical",
			stage: graph.Stage{
				Phase: "inception",
				Slug:  "requirements-analysis",
				Consumes: []graph.Consume{{
					Artifact:      "conditional-artifact",
					Required:      true,
					ConditionalOn: "Brownfield",
				}},
			},
			projectType: "brownfield",
			wantErr:     artifact.ErrInvalidMetadata,
		},
		{
			name:    "invalid selected producer phase",
			stage:   consumerStage(invalidProducerPhaseCatalog),
			catalog: invalidProducerPhaseCatalog,
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name:    "invalid selected producer slug",
			stage:   consumerStage(invalidProducerSlugCatalog),
			catalog: invalidProducerSlugCatalog,
			wantErr: artifact.ErrInvalidMetadata,
		},
		{
			name:    "current nonempty for each",
			stage:   graph.Stage{Phase: "inception", Slug: "requirements-analysis", ForEach: "unit"},
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name:    "current known per-unit slug",
			stage:   graph.Stage{Phase: "construction", Slug: "functional-design"},
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name:    "current CodeKB slug",
			stage:   graph.Stage{Phase: "inception", Slug: "reverse-engineering"},
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name: "current nonnil produces kinds",
			stage: graph.Stage{
				Phase:         "inception",
				Slug:          "requirements-analysis",
				ProducesKinds: map[string][]string{},
			},
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name:    "selected producer nonempty for each",
			stage:   consumerStage(unsupportedProducerForEachCatalog),
			catalog: unsupportedProducerForEachCatalog,
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name:    "selected producer CodeKB slug",
			stage:   consumerStage(unsupportedProducerCodeKBCatalog),
			catalog: unsupportedProducerCodeKBCatalog,
			wantErr: artifact.ErrUnsupportedPlacement,
		},
		{
			name:    "selected producer nonnil produces kinds",
			stage:   consumerStage(unsupportedProducerKindsCatalog),
			catalog: unsupportedProducerKindsCatalog,
			wantErr: artifact.ErrUnsupportedPlacement,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := artifact.ResolvePaths(test.stage, test.catalog, test.projectType)
			if !errors.Is(err, test.wantErr) {
				t.Errorf("ResolvePaths() error = %v, want errors.Is(..., %v)", err, test.wantErr)
			}
			if !reflect.DeepEqual(got, artifact.Paths{}) {
				t.Errorf("ResolvePaths() paths = %#v, want zero Paths", got)
			}
		})
	}
}

func TestResolvePathsOwnsResultsAndInputs(t *testing.T) {
	producer := pathTestStage("producer", "ideation", "1.1")
	producer["produces"] = []string{"shared-artifact"}
	producer["optional_produces"] = []string{"optional-artifact"}
	consumer := pathTestStage("consumer", "operation", "2.1")
	consumer["produces"] = []string{"consumer-output"}
	consumer["optional_produces"] = []string{"consumer-optional"}
	consumer["consumes"] = []map[string]any{
		{"artifact": "shared-artifact", "required": true},
		{"artifact": "optional-artifact", "required": false},
		{"artifact": "orphan", "required": true},
	}
	catalog := loadPathTestCatalog(t, []map[string]any{producer, consumer})
	stage := consumerStage(catalog)

	beforeStage := stage
	beforeStage.Produces = slices.Clone(stage.Produces)
	beforeStage.OptionalProduces = slices.Clone(stage.OptionalProduces)
	beforeStage.Consumes = slices.Clone(stage.Consumes)
	beforeCatalog := catalog.Stages()
	want := artifact.Paths{
		Consumes: []artifact.Input{
			{Artifact: "shared-artifact", Path: "ideation/producer/shared-artifact.md", Required: true},
			{Artifact: "optional-artifact", Path: "ideation/producer/optional-artifact.md", Required: false},
			{Artifact: "orphan", Path: "operation/consumer/orphan.md", Required: true},
		},
		Produces: []string{
			"operation/consumer/consumer-output.md",
			"operation/consumer/consumer-optional.md",
		},
	}

	first, err := artifact.ResolvePaths(stage, catalog, "unknown-project-type")
	if err != nil {
		t.Fatalf("first ResolvePaths() error = %v", err)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("first ResolvePaths() = %#v, want %#v", first, want)
	}
	first.Consumes[0].Path = "mutated-first-input"
	first.Produces[0] = "mutated-first-output"
	first.Consumes = append(first.Consumes, artifact.Input{Artifact: "appended", Path: "appended.md"})
	first.Produces = append(first.Produces, "appended.md")

	second, err := artifact.ResolvePaths(stage, catalog, "unknown-project-type")
	if err != nil {
		t.Fatalf("second ResolvePaths() error = %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second ResolvePaths() = %#v, want %#v after first result mutation", second, want)
	}
	second.Consumes[1].Path = "mutated-second-input"
	second.Produces[1] = "mutated-second-output"
	if first.Consumes[0].Path != "mutated-first-input" || first.Consumes[1].Path != want.Consumes[1].Path {
		t.Errorf("second result mutation affected first Consumes: %#v", first.Consumes)
	}
	if first.Produces[0] != "mutated-first-output" || first.Produces[1] != want.Produces[1] {
		t.Errorf("second result mutation affected first Produces: %#v", first.Produces)
	}
	if !reflect.DeepEqual(stage, beforeStage) {
		t.Errorf("ResolvePaths() mutated input stage: %#v, want %#v", stage, beforeStage)
	}
	if afterCatalog := catalog.Stages(); !reflect.DeepEqual(afterCatalog, beforeCatalog) {
		t.Errorf("ResolvePaths() mutated catalog stages: %#v, want %#v", afterCatalog, beforeCatalog)
	}
}

func consumerStage(catalog graph.Snapshot) graph.Stage {
	for _, stage := range catalog.Stages() {
		if stage.Slug == "consumer" {
			return stage
		}
	}
	return graph.Stage{}
}

func pathTestCatalogWithProducer(t *testing.T, producer map[string]any) graph.Snapshot {
	t.Helper()
	producer["produces"] = []string{"selected-artifact"}
	consumer := pathTestStage("consumer", "operation", "2.1")
	consumer["consumes"] = []map[string]any{
		{"artifact": "selected-artifact", "required": true},
	}
	return loadPathTestCatalog(t, []map[string]any{producer, consumer})
}
