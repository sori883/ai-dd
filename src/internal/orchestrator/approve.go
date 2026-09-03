package orchestrator

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

// ApproveInput contains the identity-bound roots and graph selection for a
// human approval. Current identifies the explicit stage being approved;
// Choice is validated against the fresh state revision and audit receipt.
// Roots remain caller-owned and are never closed by ApproveGate.
type ApproveInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
	Current     graph.Stage
	Catalog     graph.Snapshot
	Choice      string
}

// ApproveResult reports both durable phases of approval. ApprovalSaved stays
// true after the first state save, even when the downstream audit or state
// transition fails; FinalTransitionComplete is true only after the second
// state save. Guard is intentionally absent from this result.
type ApproveResult struct {
	State                   state.State
	Content                 []byte
	Stage                   graph.Stage
	Changed                 bool
	ApprovalSaved           bool
	FinalTransitionComplete bool
}

// ApproveGate commits an ordinary stage approval and, in the same record lock,
// advances to the next supported stage or completes the workflow. It does not
// mint HUMAN_TURN; the receipt must already exist in the identity-bound audit
// ledger.
func ApproveGate(ctx context.Context, input ApproveInput) (result ApproveResult, err error) {
	return approveGateWithOps(ctx, input, gateOps{})
}

func approveGateWithOps(ctx context.Context, input ApproveInput, injected gateOps) (result ApproveResult, err error) {
	ops := mergeGateOps(systemGateOps(), injected)
	if err := validateApproveRoots(ctx, input); err != nil {
		return ApproveResult{}, err
	}

	err = recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		if _, err := audit.ReadEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("approve gate: validate audit binding: %w", err)
		}
		document, err := ops.readDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("approve gate: read state: %w", err)
		}
		records, err := audit.ReadEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot)
		if err != nil {
			return fmt.Errorf("approve gate: reread audit after state: %w", err)
		}

		gateInput := GateInput{
			Identity:    input.Identity,
			ProjectRoot: input.ProjectRoot,
			RecordRoot:  input.RecordRoot,
			Current:     input.Current,
			Catalog:     input.Catalog,
		}
		stage, progress, err := resolveGateState(document.State, gateInput)
		if err != nil {
			return err
		}
		if progress.CheckboxState != state.CheckboxStateAwaitingApproval || progress.CheckboxMarker != string(state.StageMarkerAwaitingApproval) {
			return fmt.Errorf("approve gate: stage %q is not awaiting approval: %w", stage.Slug, ErrInvalidGate)
		}
		if err := validateGateCapabilities(stage); err != nil {
			return err
		}
		if err := validateGatePhaseState(document, stage); err != nil {
			return err
		}
		if _, ok := input.Catalog.Scope(document.State.Scope()); !ok {
			return fmt.Errorf("approve gate: scope %q is absent from graph: %w", document.State.Scope(), ErrStateCatalogMismatch)
		}

		lastUpdated, err := document.LastUpdated()
		if err != nil {
			return fmt.Errorf("approve gate: read last updated: %w", err)
		}
		lastCompleted, err := document.LastCompletedStage()
		if err != nil {
			return fmt.Errorf("approve gate: read last completed stage: %w", err)
		}
		if _, err := document.ActiveAgent(); err != nil {
			return fmt.Errorf("approve gate: read active agent: %w", err)
		}
		if _, err := document.NextAction(); err != nil {
			return fmt.Errorf("approve gate: read next action: %w", err)
		}
		if decision := EvaluateStageCompletion(CompletionInput{
			Current:  stage,
			Catalog:  input.Catalog,
			RecordFS: input.RecordRoot.FS(),
		}); !decision.Ready {
			return fmt.Errorf("approve gate: completion is not ready (%s): %s: %w", decision.Blocker, decision.Reason, ErrGateNotReady)
		}

		needsBackstop, err := revisionBackstopRequired(records, stage)
		if err != nil {
			return err
		}
		if needsBackstop {
			return fmt.Errorf("approve gate: unrecorded revision evidence requires unsupported recovery: %w", ErrUnsupportedGate)
		}
		choice, err := validateApprovalGateDecision(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot, document.Content, progress, input.Choice)
		if err != nil {
			return err
		}

		completedCount := countCompletedStages(document.State.Stages()) + 1
		updatedAt := canonicalApprovalTimestamp(ops.now())
		firstReplacement, err := state.Patch(document.Content, state.PatchRequest{
			Fields: []state.FieldPatch{
				{Field: state.CanonicalFieldCompleted, Expected: strconv.Itoa(document.State.Summary().Completed), Replacement: strconv.Itoa(completedCount)},
				{Field: state.CanonicalFieldLastUpdated, Expected: lastUpdated, Replacement: updatedAt},
				{Field: state.CanonicalFieldLastCompletedStage, Expected: lastCompleted, Replacement: stage.Slug},
			},
			StageMarkers: []state.StageMarkerPatch{{
				Slug:        stage.Slug,
				Expected:    state.StageMarkerAwaitingApproval,
				Replacement: state.StageMarkerCompleted,
			}},
		})
		if err != nil {
			return fmt.Errorf("approve gate: patch approval state: %w", err)
		}
		firstState, err := state.Parse(firstReplacement)
		if err != nil {
			return fmt.Errorf("approve gate: parse approval state: %w", err)
		}

		if err := ops.appendAudit(ctx, guard, input.ProjectRoot, input.RecordRoot, []audit.Event{
			{Event: "GATE_APPROVED", Fields: map[string]string{"Stage": stage.Slug, "User Input": choice}},
			{Event: "STAGE_COMPLETED", Fields: map[string]string{"Stage": stage.Slug, "Details": fmt.Sprintf("Stage %s approved by gate", stage.Name)}},
		}); err != nil {
			return fmt.Errorf("approve gate: append approval audit: %w", err)
		}
		if err := ops.writeState(input.RecordRoot, firstReplacement); err != nil {
			return fmt.Errorf("approve gate: write approval state: %w", err)
		}
		result = ApproveResult{
			State:         firstState,
			Content:       slices.Clone(firstReplacement),
			Stage:         cloneGateStage(stage),
			Changed:       true,
			ApprovalSaved: true,
		}

		fresh, err := ops.readDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("approve gate: reread approval state: %w", err)
		}
		if _, err := audit.ReadEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("approve gate: revalidate audit after approval: %w", err)
		}
		next, hasNext, err := deriveNextStage(fresh.State, input.Catalog, stage.Slug)
		if err != nil {
			return err
		}

		secondReplacement, secondEvents, err := buildApprovalTransition(fresh, input.Catalog, stage, next, hasNext, ops.now)
		if err != nil {
			return err
		}
		if err := ops.appendAudit(ctx, guard, input.ProjectRoot, input.RecordRoot, secondEvents); err != nil {
			return fmt.Errorf("approve gate: append transition audit: %w", err)
		}
		if err := ops.writeState(input.RecordRoot, secondReplacement); err != nil {
			return fmt.Errorf("approve gate: write transition state: %w", err)
		}
		finalState, err := state.Parse(secondReplacement)
		if err != nil {
			return fmt.Errorf("approve gate: parse transition state: %w", err)
		}
		result.State = finalState
		result.Content = slices.Clone(secondReplacement)
		result.FinalTransitionComplete = true
		return nil
	})
	return result, err
}

func validateApproveRoots(ctx context.Context, input ApproveInput) error {
	if ctx == nil {
		return fmt.Errorf("approve gate: nil context: %w", ErrInvalidGate)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("approve gate: project and record roots are required: %w", ErrInvalidGate)
	}
	if input.Current.Slug == "" {
		return fmt.Errorf("approve gate: explicit current stage is required: %w", ErrInvalidGate)
	}
	return nil
}

func countCompletedStages(stages []state.StageProgress) int {
	count := 0
	for _, stage := range stages {
		if stage.CheckboxState == state.CheckboxStateCompleted {
			count++
		}
	}
	return count
}

func canonicalApprovalTimestamp(now time.Time) string {
	return now.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func buildApprovalTransition(document state.Document, catalog graph.Snapshot, completed graph.Stage, next graph.Stage, hasNext bool, now func() time.Time) ([]byte, []audit.Event, error) {
	lastUpdated, err := document.LastUpdated()
	if err != nil {
		return nil, nil, fmt.Errorf("approve gate: read transition last updated: %w", err)
	}
	lastCompleted, err := document.LastCompletedStage()
	if err != nil {
		return nil, nil, fmt.Errorf("approve gate: read transition last completed stage: %w", err)
	}
	activeAgent, err := document.ActiveAgent()
	if err != nil {
		return nil, nil, fmt.Errorf("approve gate: read transition active agent: %w", err)
	}
	nextAction, err := document.NextAction()
	if err != nil {
		return nil, nil, fmt.Errorf("approve gate: read transition next action: %w", err)
	}
	updatedAt := canonicalApprovalTimestamp(now())
	completedCount := strconv.Itoa(countCompletedStages(document.State.Stages()))
	fields := []state.FieldPatch{
		{Field: state.CanonicalFieldCompleted, Expected: strconv.Itoa(document.State.Summary().Completed), Replacement: completedCount},
		{Field: state.CanonicalFieldLastUpdated, Expected: lastUpdated, Replacement: updatedAt},
		{Field: state.CanonicalFieldLastCompletedStage, Expected: lastCompleted, Replacement: completed.Slug},
	}
	var events []audit.Event
	if hasNext {
		if err := validateGateCapabilities(next); err != nil {
			return nil, nil, err
		}
		if err := validateGatePhaseState(document, next); err != nil {
			return nil, nil, err
		}
		following, hasFollowing, err := deriveFollowingStage(document.State, catalog, completed.Slug, next.Slug)
		if err != nil {
			return nil, nil, err
		}
		nextStageValue := "none"
		if hasFollowing {
			nextStageValue = following.Slug
		}
		fields = append(fields,
			state.FieldPatch{Field: state.CanonicalFieldCurrentStage, Expected: document.State.CurrentStage(), Replacement: next.Slug},
			state.FieldPatch{Field: state.CanonicalFieldInProgress, Expected: document.State.Summary().InProgress, Replacement: next.Slug},
			state.FieldPatch{Field: state.CanonicalFieldLifecyclePhase, Expected: string(document.State.LifecyclePhase()), Replacement: strings.ToUpper(next.Phase)},
			state.FieldPatch{Field: state.CanonicalFieldNextStage, Expected: document.State.NextStage(), Replacement: nextStageValue},
			state.FieldPatch{Field: state.CanonicalFieldActiveAgent, Expected: activeAgent, Replacement: next.LeadAgent},
			state.FieldPatch{Field: state.CanonicalFieldNextAction, Expected: nextAction, Replacement: fmt.Sprintf("Execute %s", next.Name)},
		)
		if completed.Phase != next.Phase {
			fromPhase, ok := lifecyclePhaseForGraphPhase(completed.Phase)
			if !ok {
				return nil, nil, fmt.Errorf("approve gate: completed phase %q is unknown: %w", completed.Phase, ErrStateCatalogMismatch)
			}
			toPhase, ok := lifecyclePhaseForGraphPhase(next.Phase)
			if !ok {
				return nil, nil, fmt.Errorf("approve gate: next phase %q is unknown: %w", next.Phase, ErrUnsupportedGate)
			}
			phaseStatuses := phaseStatusMap(document.State.PhaseProgress())
			if phaseStatuses[fromPhase] != state.PhaseStatusActive || phaseStatuses[toPhase] != state.PhaseStatusPending {
				return nil, nil, fmt.Errorf("approve gate: phase progress does not describe boundary: %w", ErrStateCatalogMismatch)
			}
			phasePatches := []state.PhaseProgressPatch{
				{Phase: fromPhase, Expected: state.PhaseStatusActive, Replacement: state.PhaseStatusVerified},
				{Phase: toPhase, Expected: state.PhaseStatusPending, Replacement: state.PhaseStatusActive},
			}
			replacement, err := state.Patch(document.Content, state.PatchRequest{
				Fields: fields, PhaseProgress: phasePatches,
				StageMarkers: []state.StageMarkerPatch{{Slug: next.Slug, Expected: state.StageMarkerPending, Replacement: state.StageMarkerInProgress}},
			})
			if err != nil {
				return nil, nil, fmt.Errorf("approve gate: patch phase transition: %w", err)
			}
			events = []audit.Event{
				{Event: "PHASE_COMPLETED", Fields: map[string]string{"From phase": completed.Phase, "To phase": next.Phase, "Stages completed": completedCount}},
				{Event: "PHASE_VERIFIED", Fields: map[string]string{"Phase boundary": fmt.Sprintf("%s → %s", completed.Phase, next.Phase)}},
				{Event: "PHASE_STARTED", Fields: map[string]string{"Phase": next.Phase, "Scope": document.State.Scope()}},
				{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": next.Slug, "Agent": next.LeadAgent}},
			}
			return replacement, events, nil
		}
		replacement, err := state.Patch(document.Content, state.PatchRequest{
			Fields:       fields,
			StageMarkers: []state.StageMarkerPatch{{Slug: next.Slug, Expected: state.StageMarkerPending, Replacement: state.StageMarkerInProgress}},
		})
		if err != nil {
			return nil, nil, fmt.Errorf("approve gate: patch stage transition: %w", err)
		}
		events = []audit.Event{{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": next.Slug, "Agent": next.LeadAgent}}}
		return replacement, events, nil
	}

	fields = append(fields,
		state.FieldPatch{Field: state.CanonicalFieldStatus, Expected: string(document.State.WorkflowStatus()), Replacement: string(state.WorkflowStatusCompleted)},
		state.FieldPatch{Field: state.CanonicalFieldInProgress, Expected: document.State.Summary().InProgress, Replacement: "none"},
		state.FieldPatch{Field: state.CanonicalFieldNextStage, Expected: document.State.NextStage(), Replacement: "none"},
		state.FieldPatch{Field: state.CanonicalFieldNextAction, Expected: nextAction, Replacement: "Workflow complete"},
	)
	phase, ok := lifecyclePhaseForGraphPhase(completed.Phase)
	if !ok {
		return nil, nil, fmt.Errorf("approve gate: terminal phase %q is unknown: %w", completed.Phase, ErrStateCatalogMismatch)
	}
	phaseStatuses := phaseStatusMap(document.State.PhaseProgress())
	if phaseStatuses[phase] != state.PhaseStatusActive {
		return nil, nil, fmt.Errorf("approve gate: terminal phase is not active: %w", ErrStateCatalogMismatch)
	}
	replacement, err := state.Patch(document.Content, state.PatchRequest{
		Fields:        fields,
		PhaseProgress: []state.PhaseProgressPatch{{Phase: phase, Expected: state.PhaseStatusActive, Replacement: state.PhaseStatusVerified}},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("approve gate: patch terminal transition: %w", err)
	}
	events = []audit.Event{
		{Event: "PHASE_COMPLETED", Fields: map[string]string{"From phase": completed.Phase, "To phase": "(end)", "Stages completed": completedCount}},
		{Event: "PHASE_VERIFIED", Fields: map[string]string{"Phase boundary": fmt.Sprintf("%s → end", completed.Phase)}},
		{Event: "WORKFLOW_COMPLETED", Fields: map[string]string{"Scope": document.State.Scope(), "Details": fmt.Sprintf("Scope: %s, %s stages completed", document.State.Scope(), completedCount)}},
	}
	return replacement, events, nil
}

func phaseStatusMap(progress []state.PhaseProgress) map[state.LifecyclePhase]state.PhaseStatus {
	result := make(map[state.LifecyclePhase]state.PhaseStatus, len(progress))
	for _, row := range progress {
		result[row.Phase] = row.Status
	}
	return result
}
