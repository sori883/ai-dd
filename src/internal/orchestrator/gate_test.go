package orchestrator

import (
	"errors"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestGateCapabilityValidationFailsClosed(t *testing.T) {
	t.Parallel()

	base := graph.Stage{
		Slug:    "intent-capture",
		Phase:   "ideation",
		Mode:    "subagent",
		Enabled: true,
	}
	tests := []struct {
		name  string
		stage graph.Stage
	}{
		{name: "summary required", stage: withGateSummary(base, "required")},
		{name: "summary if present", stage: withGateSummary(base, "if-present")},
		{name: "pipeline", stage: withGateMode(base, "pipeline")},
		{name: "reviewer", stage: withGateReviewer(base, "reviewer")},
		{name: "sensor", stage: withGateSensors(base, []string{"sensor"})},
		{name: "agent team", stage: withGateMode(base, "agent-team")},
		{name: "per unit", stage: withGateForEach(base, "unit")},
		{name: "workspace", stage: withGateWorkspace(base)},
		{name: "per kind", stage: withGateKinds(base)},
		{name: "CodeKB", stage: withGateSlug(base, "reverse-engineering")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateGateCapabilities(tt.stage); !errors.Is(err, ErrUnsupportedGate) {
				t.Fatalf("validateGateCapabilities() error = %v, want ErrUnsupportedGate", err)
			}
		})
	}
	if err := validateGateCapabilities(base); err != nil {
		t.Fatalf("validateGateCapabilities(ordinary) error = %v, want nil", err)
	}
}

func TestGatePhaseValidationFailsClosedForInitializationAndConstruction(t *testing.T) {
	t.Parallel()

	content := stringsForDecisionState(t)
	document := state.Document{Content: content}
	construction := graph.Stage{Slug: "functional-design", Phase: "construction"}
	if err := validateGatePhaseState(document, construction); !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("construction error = %v, want ErrUnsupportedGate", err)
	}

	initialization := graph.Stage{Slug: "workspace-scaffold", Phase: "initialization"}
	if err := validateGatePhaseState(document, initialization); !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("initialization error = %v, want ErrUnsupportedGate", err)
	}
}

func withGateSummary(stage graph.Stage, value string) graph.Stage {
	stage.SummaryConfirmation = value
	return stage
}

func withGateMode(stage graph.Stage, value string) graph.Stage {
	stage.Mode = value
	return stage
}

func withGateReviewer(stage graph.Stage, value string) graph.Stage {
	stage.Reviewer = value
	return stage
}

func withGateSensors(stage graph.Stage, value []string) graph.Stage {
	stage.Sensors = value
	return stage
}

func withGateForEach(stage graph.Stage, value string) graph.Stage {
	stage.ForEach = value
	return stage
}

func withGateWorkspace(stage graph.Stage) graph.Stage {
	stage.WorkspaceRequires = true
	return stage
}

func withGateKinds(stage graph.Stage) graph.Stage {
	stage.ProducesKinds = map[string][]string{"artifact": {"kind"}}
	return stage
}

func withGateSlug(stage graph.Stage, value string) graph.Stage {
	stage.Slug = value
	return stage
}
