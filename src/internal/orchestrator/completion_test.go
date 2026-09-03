package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
)

func TestEvaluateStageCompletionArtifact(t *testing.T) {
	t.Parallel()

	stage := loadCompletionStage(t, map[string]any{
		"produces": []string{"artifact"},
	})

	tests := []struct {
		name    string
		files   fstest.MapFS
		ready   bool
		blocker CompletionBlocker
	}{
		{
			name:    "required artifact missing",
			files:   fstest.MapFS{},
			ready:   false,
			blocker: CompletionBlockerArtifact,
		},
		{
			name: "required artifact present",
			files: fstest.MapFS{
				"ideation/completion/artifact.md": {Data: []byte("content")},
			},
			ready: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EvaluateStageCompletion(CompletionInput{
				Current:  stage,
				Catalog:  loadCompletionCatalog(t, map[string]any{"produces": []string{"artifact"}}),
				RecordFS: tt.files,
			})
			if got.Ready != tt.ready {
				t.Errorf("CompletionDecision.Ready = %v, want %v", got.Ready, tt.ready)
			}
			if got.Blocker != tt.blocker {
				t.Errorf("CompletionDecision.Blocker = %q, want %q", got.Blocker, tt.blocker)
			}
			if !tt.ready && !strings.Contains(got.Reason, "artifact") {
				t.Errorf("CompletionDecision.Reason = %q, want artifact context", got.Reason)
			}
		})
	}
}

func TestEvaluateStageCompletionGateOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fields   map[string]any
		evidence CompletionEvidence
		blocker  CompletionBlocker
	}{
		{
			name: "summary before pipeline",
			fields: map[string]any{
				"produces":             []string{"artifact"},
				"summary_confirmation": "required",
				"mode":                 "pipeline",
				"reviewer":             "reviewer-agent",
				"sensors":              []string{"required-sections"},
			},
			blocker: CompletionBlockerSummary,
		},
		{
			name: "pipeline before review",
			fields: map[string]any{
				"produces": []string{"artifact"},
				"mode":     "pipeline",
				"reviewer": "reviewer-agent",
				"sensors":  []string{"required-sections"},
			},
			evidence: CompletionEvidence{
				SummaryConfirmed: true,
			},
			blocker: CompletionBlockerPipeline,
		},
		{
			name: "review before sensor",
			fields: map[string]any{
				"produces": []string{"artifact"},
				"reviewer": "reviewer-agent",
				"sensors":  []string{"required-sections"},
			},
			evidence: CompletionEvidence{
				SummaryConfirmed: true,
				PipelineLinked:   true,
			},
			blocker: CompletionBlockerReview,
		},
		{
			name: "sensor before blocking",
			fields: map[string]any{
				"produces": []string{"artifact"},
				"sensors":  []string{"required-sections"},
			},
			evidence: CompletionEvidence{
				SummaryConfirmed: true,
				PipelineLinked:   true,
				ReviewReceipt:    true,
			},
			blocker: CompletionBlockerSensor,
		},
		{
			name: "blocking is last",
			fields: map[string]any{
				"produces": []string{"artifact"},
				"sensors":  []string{"required-sections"},
			},
			evidence: CompletionEvidence{
				SummaryConfirmed:     true,
				PipelineLinked:       true,
				ReviewReceipt:        true,
				SensorsPassed:        true,
				BlockingSensorsClear: false,
			},
			blocker: CompletionBlockerBlocking,
		},
		{
			name: "all evidence present",
			fields: map[string]any{
				"produces":             []string{"artifact"},
				"summary_confirmation": "required",
				"mode":                 "pipeline",
				"reviewer":             "reviewer-agent",
				"sensors":              []string{"required-sections"},
			},
			evidence: CompletionEvidence{
				SummaryConfirmed:     true,
				PipelineLinked:       true,
				ReviewReceipt:        true,
				SensorsPassed:        true,
				BlockingSensorsClear: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := loadCompletionStage(t, tt.fields)
			got := EvaluateStageCompletion(CompletionInput{
				Current: stage,
				Catalog: loadCompletionCatalog(t, tt.fields),
				RecordFS: fstest.MapFS{
					"ideation/completion/artifact.md": {Data: []byte("content")},
				},
				Evidence: tt.evidence,
			})
			if got.Blocker != tt.blocker {
				t.Errorf("CompletionDecision.Blocker = %q, want %q (reason: %s)", got.Blocker, tt.blocker, got.Reason)
			}
			if tt.blocker == CompletionBlockerNone && !got.Ready {
				t.Errorf("CompletionDecision.Ready = false, want true (reason: %s)", got.Reason)
			}
		})
	}
}

func TestEvaluateStageCompletionRejectsUnsupportedOrInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stage   map[string]any
		mutate  func(graph.Stage) graph.Stage
		blocker CompletionBlocker
	}{
		{
			name: "per-unit stage",
			stage: map[string]any{
				"for_each": "unit-of-work",
			},
			blocker: CompletionBlockerPerUnit,
		},
		{
			name:    "known per-unit stage without marker",
			stage:   map[string]any{"slug": "nfr-requirements"},
			blocker: CompletionBlockerPerUnit,
		},
		{
			name:    "CodeKB stage",
			stage:   map[string]any{"slug": "reverse-engineering"},
			blocker: CompletionBlockerCodeKB,
		},
		{
			name:  "catalog mismatch",
			stage: map[string]any{},
			mutate: func(stage graph.Stage) graph.Stage {
				stage.Phase = "construction"
				return stage
			},
			blocker: CompletionBlockerStageMismatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stage := loadCompletionStage(t, tt.stage)
			current := stage
			if tt.mutate != nil {
				current = tt.mutate(current)
			}
			got := EvaluateStageCompletion(CompletionInput{
				Current: current,
				Catalog: loadCompletionCatalog(t, tt.stage),
				RecordFS: fstest.MapFS{
					"ideation/completion/artifact.md": {Data: []byte("content")},
				},
			})
			if got.Ready {
				t.Fatalf("CompletionDecision.Ready = true, want false (reason: %s)", got.Reason)
			}
			if got.Blocker != tt.blocker {
				t.Errorf("CompletionDecision.Blocker = %q, want %q (reason: %s)", got.Blocker, tt.blocker, got.Reason)
			}
		})
	}

	zero := EvaluateStageCompletion(CompletionInput{})
	if zero.Ready {
		t.Error("zero CompletionInput produced a ready decision")
	}
	if zero.Blocker != CompletionBlockerInvalidInput {
		t.Errorf("zero CompletionInput blocker = %q, want %q", zero.Blocker, CompletionBlockerInvalidInput)
	}
	var zeroDecision CompletionDecision
	if zeroDecision.Ready {
		t.Error("zero CompletionDecision.Ready = true, want false")
	}
}

func loadCompletionStage(t *testing.T, fields map[string]any) graph.Stage {
	t.Helper()
	return loadCompletionCatalog(t, fields).Stages()[0]
}

func loadCompletionCatalog(t *testing.T, fields map[string]any) graph.Snapshot {
	t.Helper()
	stage := map[string]any{
		"slug":           "completion",
		"number":         "2.1",
		"name":           "Completion",
		"phase":          "ideation",
		"execution":      "ALWAYS",
		"lead_agent":     "orchestrator",
		"support_agents": []string{},
		"mode":           "inline",
		"scopes":         []string{},
		"produces":       []string{},
		"consumes":       []map[string]any{},
		"requires_stage": []string{},
	}
	for name, value := range fields {
		stage[name] = value
	}
	data, err := json.Marshal([]map[string]any{stage})
	if err != nil {
		t.Fatalf("json.Marshal(stage): %v", err)
	}
	snapshot, err := graph.Load(fstest.MapFS{
		"stage-graph.json": {Data: data},
		"scope-grid.json":  {Data: []byte(`{}`)},
	})
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	return snapshot
}
