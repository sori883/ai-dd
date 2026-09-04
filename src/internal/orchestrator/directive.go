package orchestrator

import (
	"errors"
	"fmt"
	"slices"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

// DirectiveKind identifies the next action exposed by the orchestrator.
type DirectiveKind string

const (
	// DirectiveKindUnknown is the zero value for an unresolved directive.
	DirectiveKindUnknown DirectiveKind = ""
	// DirectiveKindRunStage directs the caller to run one stage.
	DirectiveKindRunStage DirectiveKind = "run-stage"
	// DirectiveKindWorkflowComplete indicates that no stage remains to run.
	DirectiveKindWorkflowComplete DirectiveKind = "workflow-complete"
)

var (
	// ErrInvalidState indicates an internally inconsistent state snapshot.
	ErrInvalidState = errors.New("orchestrator: invalid state")
	// ErrUnsupportedState indicates a valid state form that this resolver cannot route.
	ErrUnsupportedState = errors.New("orchestrator: unsupported state")
	// ErrStateCatalogMismatch indicates that state routing metadata disagrees with the graph.
	ErrStateCatalogMismatch = errors.New("orchestrator: state catalog mismatch")
)

// Directive is the read-only routing result returned by ResolveDirective.
// Its fields are private so callers can observe only the validated kind and
// an owned copy of the selected stage.
type Directive struct {
	kind     DirectiveKind
	stage    graph.Stage
	hasStage bool
}

// Kind returns the directive kind.
func (d Directive) Kind() DirectiveKind { return d.kind }

// Stage returns an independently mutable copy of the selected stage.
func (d Directive) Stage() (graph.Stage, bool) {
	if !d.hasStage {
		return graph.Stage{}, false
	}
	return cloneDirectiveStage(d.stage), true
}

// ResolveDirective combines a parsed state and graph snapshot into one
// read-only orchestration directive.
func ResolveDirective(current state.State, catalog graph.Snapshot) (Directive, error) {
	switch current.WorkflowStatus() {
	case state.WorkflowStatusRunning:
		return resolveRunningDirective(current, catalog)
	case state.WorkflowStatusCompleted:
		return resolveCompletedDirective(current)
	default:
		return directiveError(ErrInvalidState, "workflow status is not recognized")
	}
}

func resolveRunningDirective(current state.State, catalog graph.Snapshot) (Directive, error) {
	currentSlug := current.CurrentStage()
	if currentSlug == "" || currentSlug == "none" {
		return directiveError(ErrInvalidState, "running state has no current stage")
	}

	stages := current.Stages()
	var (
		currentStage   state.StageProgress
		foundCurrent   bool
		liveCount      int
		otherLiveCount int
	)
	for _, stage := range stages {
		switch stage.CheckboxState {
		case state.CheckboxStateInProgress,
			state.CheckboxStateAwaitingApproval,
			state.CheckboxStateRevising:
			liveCount++
			if stage.Slug != currentSlug {
				otherLiveCount++
			}
		}
		if stage.Slug == currentSlug {
			if foundCurrent {
				return directiveError(ErrInvalidState, "running state has duplicate current stage")
			}
			currentStage = stage
			foundCurrent = true
		}
	}
	if !foundCurrent {
		return directiveError(ErrInvalidState, "running state current stage is absent")
	}

	if currentStage.PlanAction != state.PlanActionExecute {
		return directiveError(ErrInvalidState, "running current stage is not marked execute")
	}
	if otherLiveCount != 0 {
		return directiveError(ErrInvalidState, "running state has another live stage")
	}

	switch currentStage.CheckboxState {
	case state.CheckboxStatePending,
		state.CheckboxStateAwaitingApproval,
		state.CheckboxStateRevising:
		return directiveError(ErrUnsupportedState, "running current stage is not executable")
	case state.CheckboxStateCompleted, state.CheckboxStateSkipped:
		return directiveError(ErrInvalidState, "running current stage is already settled")
	case state.CheckboxStateInProgress:
		if currentStage.CheckboxMarker != "[-]" {
			return directiveError(ErrInvalidState, "running current stage has a noncanonical marker")
		}
	default:
		return directiveError(ErrInvalidState, "running current stage has an unknown checkbox state")
	}
	if liveCount != 1 {
		return directiveError(ErrInvalidState, "running state does not have one live stage")
	}

	var selected graph.Stage
	foundCatalogStage := false
	for _, stage := range catalog.Stages() {
		if stage.Slug != currentSlug {
			continue
		}
		if foundCatalogStage {
			return directiveError(ErrStateCatalogMismatch, "graph contains duplicate current stage")
		}
		selected = stage
		foundCatalogStage = true
	}
	if !foundCatalogStage {
		return directiveError(ErrStateCatalogMismatch, "current stage is absent from enabled graph")
	}
	phase, ok := lifecyclePhaseForGraphPhase(selected.Phase)
	if !ok || phase != current.LifecyclePhase() {
		return directiveError(ErrStateCatalogMismatch, "graph phase does not match lifecycle phase")
	}

	return Directive{
		kind:     DirectiveKindRunStage,
		stage:    cloneDirectiveStage(selected),
		hasStage: true,
	}, nil
}

func lifecyclePhaseForGraphPhase(phase string) (state.LifecyclePhase, bool) {
	switch phase {
	case "initialization":
		return state.LifecyclePhaseInitialization, true
	case "ideation":
		return state.LifecyclePhaseIdeation, true
	case "inception":
		return state.LifecyclePhaseInception, true
	case "construction":
		return state.LifecyclePhaseConstruction, true
	case "operation":
		return state.LifecyclePhaseOperation, true
	default:
		return state.LifecyclePhaseUnknown, false
	}
}

func resolveCompletedDirective(current state.State) (Directive, error) {
	if current.NextStage() != "none" {
		return directiveError(ErrInvalidState, "completed state has a next stage")
	}
	if current.Summary().InProgress != "none" {
		return directiveError(ErrInvalidState, "completed state has an in-progress stage")
	}

	stages := current.Stages()
	bySlug := make(map[string]state.StageProgress, len(stages))
	for _, stage := range stages {
		if _, exists := bySlug[stage.Slug]; exists {
			return directiveError(ErrInvalidState, "completed state has duplicate stage rows")
		}
		bySlug[stage.Slug] = stage

		switch stage.CheckboxState {
		case state.CheckboxStateInProgress,
			state.CheckboxStateAwaitingApproval,
			state.CheckboxStateRevising:
			return directiveError(ErrInvalidState, "completed state has a live stage")
		}

		switch stage.PlanAction {
		case state.PlanActionExecute:
			if stage.CheckboxState != state.CheckboxStateCompleted && stage.CheckboxState != state.CheckboxStateSkipped {
				return directiveError(ErrInvalidState, "completed execute stage is not settled")
			}
		case state.PlanActionSkip:
			if stage.CheckboxState != state.CheckboxStatePending && stage.CheckboxState != state.CheckboxStateSkipped {
				return directiveError(ErrInvalidState, "completed skip stage has an invalid marker")
			}
		default:
			return directiveError(ErrInvalidState, "completed stage has an unknown plan action")
		}
	}

	currentSlug := current.CurrentStage()
	if currentSlug == "" {
		return directiveError(ErrInvalidState, "completed state has no current stage value")
	}
	if currentSlug != "none" {
		stage, exists := bySlug[currentSlug]
		if !exists {
			return directiveError(ErrInvalidState, "completed current stage is absent")
		}
		if stage.CheckboxState != state.CheckboxStateCompleted && stage.CheckboxState != state.CheckboxStateSkipped {
			return directiveError(ErrInvalidState, "completed current stage is not settled")
		}
	}

	return Directive{kind: DirectiveKindWorkflowComplete}, nil
}

func cloneDirectiveStage(stage graph.Stage) graph.Stage {
	stage.SupportAgents = slices.Clone(stage.SupportAgents)
	stage.Scopes = slices.Clone(stage.Scopes)
	stage.Sensors = slices.Clone(stage.Sensors)
	stage.ProducesKinds = cloneGateKinds(stage.ProducesKinds)
	stage.Produces = slices.Clone(stage.Produces)
	stage.OptionalProduces = slices.Clone(stage.OptionalProduces)
	stage.Consumes = slices.Clone(stage.Consumes)
	stage.RequiresStages = slices.Clone(stage.RequiresStages)
	stage.RulesInContext = slices.Clone(stage.RulesInContext)
	return stage
}

func directiveError(kind error, reason string) (Directive, error) {
	return Directive{}, fmt.Errorf("resolve directive: %s: %w", reason, kind)
}
