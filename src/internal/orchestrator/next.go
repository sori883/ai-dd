package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

var ErrInvalidNext = errors.New("orchestrator: invalid next request")

const (
	// DirectiveKindAwaitingApproval identifies a stage waiting for a human
	// decision at its gate.
	DirectiveKindAwaitingApproval DirectiveKind = "awaiting-approval"
	// DirectiveKindRevising identifies a stage whose work must be revised.
	DirectiveKindRevising DirectiveKind = "revising"
)

// NextInput contains the identity-bound roots and graph used to classify the
// current workflow position. Roots remain owned by the caller.
type NextInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
	Catalog     graph.Snapshot
}

// NextResult contains the fresh state snapshot and read-only directive. The
// content is an owned copy of the state file bytes.
type NextResult struct {
	Directive Directive
	State     state.State
	Content   []byte
}

// Kind returns the classified directive kind.
func (r NextResult) Kind() DirectiveKind { return r.Directive.Kind() }

// Stage returns an independently mutable copy of the selected stage.
func (r NextResult) Stage() (graph.Stage, bool) { return r.Directive.Stage() }

// Next reads and classifies one identity-bound state snapshot without
// changing state, audit, cursors, or registry data. Binding is checked on both
// sides of the state read; the helper itself does not inspect audit content.
func Next(ctx context.Context, input NextInput) (result NextResult, err error) {
	if err := validateNextInput(ctx, input); err != nil {
		return NextResult{}, err
	}
	err = recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		if err := audit.ValidateRecordBinding(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("next: validate initial binding: %w", err)
		}
		document, err := state.ReadDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("next: read state: %w", err)
		}
		if err := audit.ValidateRecordBinding(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("next: validate binding after state read: %w", err)
		}
		directive, err := classifyNext(document, input.Catalog)
		if err != nil {
			return err
		}
		result = NextResult{
			Directive: directive,
			State:     document.State,
			Content:   slices.Clone(document.Content),
		}
		return nil
	})
	if err != nil {
		return NextResult{}, err
	}
	return result, err
}

func validateNextInput(ctx context.Context, input NextInput) error {
	if ctx == nil {
		return fmt.Errorf("next: nil context: %w", ErrInvalidNext)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("next: project and record roots are required: %w", ErrInvalidNext)
	}
	return nil
}

func classifyNext(document state.Document, catalog graph.Snapshot) (Directive, error) {
	current := document.State
	switch current.WorkflowStatus() {
	case state.WorkflowStatusCompleted:
		// Completion is a state-only decision. In particular, a broken or
		// unavailable graph must not prevent a valid terminal state from being
		// reported as complete.
		return ResolveDirective(current, graph.Snapshot{})
	case state.WorkflowStatusRunning:
		return classifyRunningNext(document, catalog)
	default:
		return directiveError(ErrInvalidState, "workflow status is not recognized")
	}
}

func classifyRunningNext(document state.Document, catalog graph.Snapshot) (Directive, error) {
	current := document.State
	currentSlug := current.CurrentStage()
	if currentSlug == "" || currentSlug == "none" {
		return directiveError(ErrInvalidState, "running state has no current stage")
	}
	progress, found := currentProgress(current, currentSlug)
	if !found {
		return directiveError(ErrInvalidState, "running state current stage is absent")
	}

	if progress.CheckboxState == state.CheckboxStateInProgress {
		directive, err := ResolveDirective(current, catalog)
		if err != nil {
			return Directive{}, err
		}
		stage, ok := directive.Stage()
		if !ok {
			return directiveError(ErrInvalidState, "running stage directive has no stage")
		}
		if err := validateNextStage(document, stage); err != nil {
			return Directive{}, err
		}
		directive.stage = cloneGateStage(stage)
		return directive, nil
	}

	if progress.CheckboxState != state.CheckboxStateAwaitingApproval && progress.CheckboxState != state.CheckboxStateRevising {
		return ResolveDirective(current, catalog)
	}
	stage, validatedProgress, err := resolveGateState(current, GateInput{
		Catalog: catalog,
	})
	if err != nil {
		return Directive{}, err
	}
	if validatedProgress.CheckboxState != progress.CheckboxState {
		return directiveError(ErrInvalidState, "current stage changed while classifying")
	}
	if err := validateNextStage(document, stage); err != nil {
		return Directive{}, err
	}
	kind := DirectiveKindAwaitingApproval
	if progress.CheckboxState == state.CheckboxStateRevising {
		kind = DirectiveKindRevising
	}
	return Directive{
		kind:     kind,
		stage:    cloneGateStage(stage),
		hasStage: true,
	}, nil
}

func currentProgress(current state.State, slug string) (state.StageProgress, bool) {
	var found state.StageProgress
	for _, progress := range current.Stages() {
		if progress.Slug != slug {
			continue
		}
		if found.Slug != "" {
			return state.StageProgress{}, false
		}
		found = progress
	}
	return found, found.Slug != ""
}

func validateNextStage(document state.Document, stage graph.Stage) error {
	if err := validateGateCapabilities(stage); err != nil {
		return err
	}
	if err := validateGatePhaseState(document, stage); err != nil {
		return err
	}
	return nil
}
