package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestClassifyNextCompletedUsesStateOnly(t *testing.T) {
	current := parseDirectiveState(t, directiveCompletedState)

	got, err := classifyNext(state.Document{State: current}, graph.Snapshot{})
	if err != nil {
		t.Fatalf("classifyNext() error = %v", err)
	}
	if got.Kind() != DirectiveKindWorkflowComplete {
		t.Fatalf("Directive.Kind() = %q, want %q", got.Kind(), DirectiveKindWorkflowComplete)
	}
	if _, ok := got.Stage(); ok {
		t.Fatal("Directive.Stage() reports a stage for workflow completion")
	}
}

func TestClassifyNextLiveMarkerKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marker string
		kind   DirectiveKind
	}{
		{name: "in progress", marker: "[-]", kind: DirectiveKindRunStage},
		{name: "awaiting approval", marker: "[?]", kind: DirectiveKindAwaitingApproval},
		{name: "revising", marker: "[R]", kind: DirectiveKindRevising},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content := strings.Replace(directiveRunningState, "[-]", test.marker, 1)
			current := parseDirectiveState(t, content)
			directive, err := classifyNext(state.Document{State: current}, loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation")))
			if err != nil {
				t.Fatalf("classifyNext() error = %v", err)
			}
			if directive.Kind() != test.kind {
				t.Fatalf("Directive.Kind() = %q, want %q", directive.Kind(), test.kind)
			}
			stage, ok := directive.Stage()
			if !ok || stage.Slug != "intent-capture" {
				t.Fatalf("Directive.Stage() = (%#v, %v), want intent-capture", stage, ok)
			}
		})
	}
}

func TestClassifyNextRejectsUnsupportedLivePhase(t *testing.T) {
	content := strings.Replace(directiveRunningState, "IDEATION", "CONSTRUCTION", 1)
	current := parseDirectiveState(t, content)
	_, err := classifyNext(state.Document{State: current}, loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "construction")))
	if !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("classifyNext() error = %v, want ErrUnsupportedGate", err)
	}
}
