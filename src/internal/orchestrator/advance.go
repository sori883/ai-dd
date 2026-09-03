package orchestrator

import (
	"fmt"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/state"
)

// deriveNextStage walks the enabled graph after completedSlug. The persisted
// stage suffix and marker are authoritative for routing; the scope grid and
// state Next Stage are used only for their independent integrity checks and
// never to select a caller-requested destination.
func deriveNextStage(current state.State, catalog graph.Snapshot, completedSlug string) (graph.Stage, bool, error) {
	scope := current.Scope()
	if scope == "" {
		return graph.Stage{}, false, fmt.Errorf("advance: state scope is empty: %w", ErrStateCatalogMismatch)
	}
	if _, ok := catalog.Scope(scope); !ok {
		return graph.Stage{}, false, fmt.Errorf("advance: scope %q is absent from graph: %w", scope, ErrStateCatalogMismatch)
	}
	if completedSlug == "" || completedSlug == "none" || current.CurrentStage() != completedSlug {
		return graph.Stage{}, false, fmt.Errorf("advance: completed stage %q is not the saved current stage: %w", completedSlug, ErrInvalidGate)
	}

	graphStages := catalog.Stages()
	stateStages := current.Stages()
	if len(graphStages) != len(stateStages) {
		return graph.Stage{}, false, fmt.Errorf("advance: graph has %d stages but state has %d rows: %w", len(graphStages), len(stateStages), ErrStateCatalogMismatch)
	}

	graphBySlug := make(map[string]graph.Stage, len(graphStages))
	graphIndex := make(map[string]int, len(graphStages))
	for index, stage := range graphStages {
		if stage.Slug == "" {
			return graph.Stage{}, false, fmt.Errorf("advance: graph contains an empty stage slug: %w", ErrStateCatalogMismatch)
		}
		if _, exists := graphBySlug[stage.Slug]; exists {
			return graph.Stage{}, false, fmt.Errorf("advance: graph contains duplicate stage %q: %w", stage.Slug, ErrStateCatalogMismatch)
		}
		graphBySlug[stage.Slug] = stage
		graphIndex[stage.Slug] = index
	}

	stateBySlug := make(map[string]state.StageProgress, len(stateStages))
	for _, progress := range stateStages {
		if _, exists := stateBySlug[progress.Slug]; exists {
			return graph.Stage{}, false, fmt.Errorf("advance: state contains duplicate stage %q: %w", progress.Slug, ErrStateCatalogMismatch)
		}
		if _, exists := graphBySlug[progress.Slug]; !exists {
			return graph.Stage{}, false, fmt.Errorf("advance: state stage %q is absent from graph: %w", progress.Slug, ErrStateCatalogMismatch)
		}
		stateBySlug[progress.Slug] = progress
	}
	for slug := range graphBySlug {
		if _, exists := stateBySlug[slug]; !exists {
			return graph.Stage{}, false, fmt.Errorf("advance: graph stage %q is absent from state: %w", slug, ErrStateCatalogMismatch)
		}
	}

	completed, ok := stateBySlug[completedSlug]
	if !ok || completed.PlanAction != state.PlanActionExecute || completed.CheckboxState != state.CheckboxStateCompleted || completed.CheckboxMarker != string(state.StageMarkerCompleted) {
		return graph.Stage{}, false, fmt.Errorf("advance: completed stage %q is not canonically completed: %w", completedSlug, ErrInvalidGate)
	}
	completedIndex := graphIndex[completedSlug]
	liveCount := 0
	executeCount := 0
	for index, stage := range graphStages {
		progress := stateBySlug[stage.Slug]
		switch progress.PlanAction {
		case state.PlanActionExecute:
			executeCount++
		case state.PlanActionSkip:
			if progress.CheckboxState != state.CheckboxStatePending && progress.CheckboxState != state.CheckboxStateSkipped {
				return graph.Stage{}, false, fmt.Errorf("advance: skipped stage %q has marker %q: %w", stage.Slug, progress.CheckboxMarker, ErrStateCatalogMismatch)
			}
		default:
			return graph.Stage{}, false, fmt.Errorf("advance: stage %q has unknown saved action: %w", stage.Slug, ErrStateCatalogMismatch)
		}
		if progress.CheckboxState == state.CheckboxStateInProgress || progress.CheckboxState == state.CheckboxStateAwaitingApproval || progress.CheckboxState == state.CheckboxStateRevising {
			liveCount++
		}

		if progress.PlanAction == state.PlanActionExecute {
			switch progress.CheckboxState {
			case state.CheckboxStatePending:
			case state.CheckboxStateCompleted, state.CheckboxStateSkipped:
			case state.CheckboxStateInProgress, state.CheckboxStateAwaitingApproval, state.CheckboxStateRevising:
			default:
				return graph.Stage{}, false, fmt.Errorf("advance: execute stage %q has unknown checkbox state: %w", stage.Slug, ErrStateCatalogMismatch)
			}
			if index < completedIndex && progress.CheckboxState != state.CheckboxStateCompleted && progress.CheckboxState != state.CheckboxStateSkipped {
				return graph.Stage{}, false, fmt.Errorf("advance: earlier execute stage %q is not settled: %w", stage.Slug, ErrInvalidGate)
			}
		}
	}
	if current.Summary().TotalStages != executeCount {
		return graph.Stage{}, false, fmt.Errorf("advance: state total stages %d differs from graph execute count %d: %w", current.Summary().TotalStages, executeCount, ErrStateCatalogMismatch)
	}
	if liveCount != 0 {
		return graph.Stage{}, false, fmt.Errorf("advance: state has %d live stages after approval: %w", liveCount, ErrInvalidGate)
	}

	for index := completedIndex + 1; index < len(graphStages); index++ {
		stage := graphStages[index]
		progress := stateBySlug[stage.Slug]
		if progress.PlanAction == state.PlanActionSkip || progress.CheckboxState == state.CheckboxStateCompleted || progress.CheckboxState == state.CheckboxStateSkipped {
			continue
		}
		if progress.PlanAction == state.PlanActionExecute && progress.CheckboxState == state.CheckboxStatePending {
			return stage, true, nil
		}
		return graph.Stage{}, false, fmt.Errorf("advance: next stage %q has unsupported routing state: %w", stage.Slug, ErrInvalidGate)
	}
	return graph.Stage{}, false, nil
}

// deriveFollowingStage selects the stage after the already selected next
// stage while preserving the same routing validation as deriveNextStage. The
// selected stage is treated as the transition destination for this lookahead;
// it is not written or marked by this helper.
func deriveFollowingStage(current state.State, catalog graph.Snapshot, completedSlug, selectedSlug string) (graph.Stage, bool, error) {
	first, found, err := deriveNextStage(current, catalog, completedSlug)
	if err != nil {
		return graph.Stage{}, false, err
	}
	if !found || first.Slug != selectedSlug {
		return graph.Stage{}, false, fmt.Errorf("advance: selected next stage %q is not the saved graph successor: %w", selectedSlug, ErrStateCatalogMismatch)
	}

	stages := catalog.Stages()
	bySlug := make(map[string]state.StageProgress, len(current.Stages()))
	for _, progress := range current.Stages() {
		bySlug[progress.Slug] = progress
	}
	selectedIndex := -1
	for index, stage := range stages {
		if stage.Slug == selectedSlug {
			selectedIndex = index
			break
		}
	}
	if selectedIndex < 0 {
		return graph.Stage{}, false, fmt.Errorf("advance: selected stage %q is absent from graph: %w", selectedSlug, ErrStateCatalogMismatch)
	}
	for index := selectedIndex + 1; index < len(stages); index++ {
		stage := stages[index]
		progress := bySlug[stage.Slug]
		if progress.PlanAction == state.PlanActionSkip || progress.CheckboxState == state.CheckboxStateCompleted || progress.CheckboxState == state.CheckboxStateSkipped {
			continue
		}
		if progress.PlanAction == state.PlanActionExecute && progress.CheckboxState == state.CheckboxStatePending {
			return stage, true, nil
		}
		return graph.Stage{}, false, fmt.Errorf("advance: following stage %q has unsupported routing state: %w", stage.Slug, ErrInvalidGate)
	}
	return graph.Stage{}, false, nil
}
