package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestNextWithGuard(t *testing.T) {
	project := t.TempDir()
	recordDir := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", recordDir, err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, "aidlc-state.md"), []byte(directiveRunningState), 0o600); err != nil {
		t.Fatalf("WriteFile(state): %v", err)
	}
	identity, err := recordlock.NewIdentity(project, "team", "build")
	if err != nil {
		t.Fatalf("recordlock.NewIdentity(): %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatalf("os.OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatalf("os.OpenRoot(record): %v", err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })

	var got NextResult
	err = recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		got, err = NextWithGuard(context.Background(), guard, NextInput{
			Identity:    identity,
			ProjectRoot: projectRoot,
			RecordRoot:  recordRoot,
			Catalog:     loadDirectiveGraph(t, directiveStage("intent-capture", "2.1", "ideation")),
		})
		if err != nil {
			return err
		}
		if !guard.Held() {
			t.Fatal("NextWithGuard() released the caller-owned guard")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recordlock.With(NextWithGuard) error = %v", err)
	}
	if got.Kind() != DirectiveKindRunStage {
		t.Fatalf("NextWithGuard().Kind() = %q, want %q", got.Kind(), DirectiveKindRunStage)
	}
	if _, ok := got.Stage(); !ok {
		t.Fatal("NextWithGuard().Stage() did not return the selected stage")
	}
}

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
