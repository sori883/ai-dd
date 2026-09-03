package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

var (
	// ErrInvalidGate indicates that a gate request or its state precondition is
	// malformed.
	ErrInvalidGate = errors.New("orchestrator: invalid gate")
	// ErrUnsupportedGate indicates a stage capability outside this walking
	// skeleton's approved gate boundary.
	ErrUnsupportedGate = errors.New("orchestrator: unsupported gate capability")
	// ErrGateNotReady indicates that the stage cannot enter a gate yet.
	ErrGateNotReady = errors.New("orchestrator: gate not ready")
	// ErrStaleHumanTurn indicates that no trusted HUMAN_TURN follows the latest
	// workflow resolution boundary.
	ErrStaleHumanTurn = errors.New("orchestrator: stale human turn")
)

// GateInput contains the identity-bound roots and immutable graph selection
// used by a gate transaction. Choice and Feedback are consumed only by the
// corresponding decision operations; OpenGate ignores them.
type GateInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
	Current     graph.Stage
	Catalog     graph.Snapshot
	Choice      string
	Feedback    string
}

// GateResult is the validated state snapshot produced by a gate operation.
// Content is a newly owned copy of the exact state bytes returned by the
// operation. Guard is intentionally not part of this result.
type GateResult struct {
	State           state.State
	Content         []byte
	Stage           graph.Stage
	Changed         bool
	AlreadyAwaiting bool
}

// gateOps is a private seam for transaction failure tests. The production
// entry points always use systemGateOps; no filesystem dependency or Guard is
// exposed as part of the gate API.
type gateOps struct {
	readEvents   func(context.Context, recordlock.Identity, *recordlock.Guard, *os.Root, *os.Root) ([]audit.AuditRecord, error)
	readDocument func(*os.Root) (state.Document, error)
	appendAudit  func(context.Context, *recordlock.Guard, *os.Root, *os.Root, []audit.Event) error
	writeState   func(*os.Root, []byte) error
	now          func() time.Time
}

func systemGateOps() gateOps {
	return gateOps{
		readEvents:   audit.ReadEvents,
		readDocument: state.ReadDocument,
		appendAudit:  audit.Append,
		writeState:   state.WriteState,
		now:          time.Now,
	}
}

func mergeGateOps(base, override gateOps) gateOps {
	if override.readEvents != nil {
		base.readEvents = override.readEvents
	}
	if override.readDocument != nil {
		base.readDocument = override.readDocument
	}
	if override.appendAudit != nil {
		base.appendAudit = override.appendAudit
	}
	if override.writeState != nil {
		base.writeState = override.writeState
	}
	if override.now != nil {
		base.now = override.now
	}
	return base
}

// OpenGate validates the current stage and records its approval gate. The
// transaction obtains its own record lock; callers never supply or receive a
// Guard. Audit is appended before the state replacement, matching the
// asymmetric durability contract of the fixed lifecycle.
func OpenGate(ctx context.Context, input GateInput) (result GateResult, err error) {
	return openGateWithOps(ctx, input, gateOps{})
}

func openGateWithOps(ctx context.Context, input GateInput, injected gateOps) (result GateResult, err error) {
	ops := mergeGateOps(systemGateOps(), injected)
	if err := validateGateRoots(ctx, input); err != nil {
		return GateResult{}, err
	}
	err = recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		// ReadEvents performs the identity/root binding checks while this
		// transaction owns the lock. It deliberately does not acquire a lease.
		if _, err := ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("open gate: validate audit binding: %w", err)
		}
		document, err := ops.readDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("open gate: read state: %w", err)
		}
		if _, err := ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("open gate: revalidate audit binding after state read: %w", err)
		}
		stage, progress, err := resolveGateState(document.State, input)
		if err != nil {
			return err
		}
		if err := validateGateCapabilities(stage); err != nil {
			return err
		}
		if err := validateGatePhaseState(document, stage); err != nil {
			return err
		}
		if progress.CheckboxState != state.CheckboxStateInProgress && progress.CheckboxState != state.CheckboxStateAwaitingApproval {
			return fmt.Errorf("open gate: current stage %q is not active or awaiting approval: %w", progress.Slug, ErrInvalidGate)
		}
		if decision := EvaluateStageCompletion(CompletionInput{
			Current:  stage,
			Catalog:  input.Catalog,
			RecordFS: input.RecordRoot.FS(),
		}); !decision.Ready {
			return fmt.Errorf("open gate: completion is not ready (%s): %s: %w", decision.Blocker, decision.Reason, ErrGateNotReady)
		}

		if progress.CheckboxState == state.CheckboxStateAwaitingApproval {
			result = GateResult{
				State:           document.State,
				Content:         slices.Clone(document.Content),
				Stage:           cloneGateStage(stage),
				Changed:         false,
				AlreadyAwaiting: true,
			}
			return nil
		}

		lastUpdated, err := document.LastUpdated()
		if err != nil {
			return fmt.Errorf("open gate: read last updated: %w", err)
		}
		updatedAt := ops.now().UTC().Format(time.RFC3339)
		replacement, err := state.Patch(document.Content, state.PatchRequest{
			Fields: []state.FieldPatch{{
				Field:       state.CanonicalFieldLastUpdated,
				Expected:    lastUpdated,
				Replacement: updatedAt,
			}},
			StageMarkers: []state.StageMarkerPatch{{
				Slug:        progress.Slug,
				Expected:    state.StageMarkerInProgress,
				Replacement: state.StageMarkerAwaitingApproval,
			}},
		})
		if err != nil {
			return fmt.Errorf("open gate: patch state: %w", err)
		}
		if err := ops.appendAudit(ctx, guard, input.ProjectRoot, input.RecordRoot, []audit.Event{{
			Event:  "STAGE_AWAITING_APPROVAL",
			Fields: map[string]string{"Stage": stage.Slug},
		}}); err != nil {
			return fmt.Errorf("open gate: append audit: %w", err)
		}
		updatedState, err := state.Parse(replacement)
		if err != nil {
			return fmt.Errorf("open gate: parse replacement: %w", err)
		}
		if err := ops.writeState(input.RecordRoot, replacement); err != nil {
			return fmt.Errorf("open gate: write state: %w", err)
		}
		result = GateResult{
			State:   updatedState,
			Content: slices.Clone(replacement),
			Stage:   cloneGateStage(stage),
			Changed: true,
		}
		return nil
	})
	return result, err
}

// RejectGate records a human Request Changes decision and moves an active
// stage to revising. It deliberately does not evaluate completion: missing
// artifacts are precisely what a rejection may ask the conductor to repair.
// A fresh HUMAN_TURN is required, but this API never creates one from caller
// text.
func RejectGate(ctx context.Context, input GateInput) (result GateResult, err error) {
	return rejectGateWithOps(ctx, input, gateOps{})
}

func rejectGateWithOps(ctx context.Context, input GateInput, injected gateOps) (result GateResult, err error) {
	ops := mergeGateOps(systemGateOps(), injected)
	if err := validateGateRoots(ctx, input); err != nil {
		return GateResult{}, err
	}
	choice, feedback, err := validateRejectionDecision(input.Choice, input.Feedback)
	if err != nil {
		return GateResult{}, err
	}
	_ = choice
	err = recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		records, err := ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot)
		if err != nil {
			return fmt.Errorf("reject gate: read audit: %w", err)
		}
		document, err := ops.readDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("reject gate: read state: %w", err)
		}
		records, err = ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot)
		if err != nil {
			return fmt.Errorf("reject gate: revalidate audit binding after state read: %w", err)
		}
		if !audit.HumanTurnFresh(records) {
			return fmt.Errorf("reject gate: no fresh HUMAN_TURN receipt: %w", ErrStaleHumanTurn)
		}
		stage, progress, err := resolveGateState(document.State, input)
		if err != nil {
			return err
		}
		if err := validateGateCapabilities(stage); err != nil {
			return err
		}
		if err := validateGatePhaseState(document, stage); err != nil {
			return err
		}
		if progress.CheckboxState != state.CheckboxStateInProgress && progress.CheckboxState != state.CheckboxStateAwaitingApproval {
			return fmt.Errorf("reject gate: current stage %q is not active or awaiting approval: %w", progress.Slug, ErrInvalidGate)
		}
		currentRevision, err := document.RevisionCount()
		if err != nil {
			return fmt.Errorf("reject gate: read revision count: %w", err)
		}
		if currentRevision == maxIntValue() {
			return fmt.Errorf("reject gate: revision count overflows: %w", ErrInvalidGate)
		}
		lastUpdated, err := document.LastUpdated()
		if err != nil {
			return fmt.Errorf("reject gate: read last updated: %w", err)
		}
		updatedAt := ops.now().UTC().Format(time.RFC3339)
		replacement, err := state.Patch(document.Content, state.PatchRequest{
			Fields: []state.FieldPatch{
				{
					Field:       state.CanonicalFieldRevisionCount,
					Expected:    strconv.Itoa(currentRevision),
					Replacement: strconv.Itoa(currentRevision + 1),
				},
				{
					Field:       state.CanonicalFieldLastUpdated,
					Expected:    lastUpdated,
					Replacement: updatedAt,
				},
			},
			StageMarkers: []state.StageMarkerPatch{{
				Slug:        progress.Slug,
				Expected:    state.StageMarker(progress.CheckboxMarker),
				Replacement: state.StageMarkerRevising,
			}},
		})
		if err != nil {
			return fmt.Errorf("reject gate: patch state: %w", err)
		}
		if err := ops.appendAudit(ctx, guard, input.ProjectRoot, input.RecordRoot, []audit.Event{
			{
				Event: "GATE_REJECTED",
				Fields: map[string]string{
					"Stage":    stage.Slug,
					"Feedback": feedback,
				},
			},
			{
				Event: "STAGE_REVISING",
				Fields: map[string]string{
					"Stage":          stage.Slug,
					"Revision count": strconv.Itoa(currentRevision + 1),
				},
			},
		}); err != nil {
			return fmt.Errorf("reject gate: append audit: %w", err)
		}
		updatedState, err := state.Parse(replacement)
		if err != nil {
			return fmt.Errorf("reject gate: parse replacement: %w", err)
		}
		if err := ops.writeState(input.RecordRoot, replacement); err != nil {
			return fmt.Errorf("reject gate: write state: %w", err)
		}
		result = GateResult{
			State:   updatedState,
			Content: slices.Clone(replacement),
			Stage:   cloneGateStage(stage),
			Changed: true,
		}
		return nil
	})
	return result, err
}

// ReviseGate revalidates a revised stage and reopens its approval gate. The
// preceding rejection is itself the freshness boundary, so this operation
// does not mint or require a HUMAN_TURN; a later approval must provide a new
// receipt.
func ReviseGate(ctx context.Context, input GateInput) (result GateResult, err error) {
	return reviseGateWithOps(ctx, input, gateOps{})
}

func reviseGateWithOps(ctx context.Context, input GateInput, injected gateOps) (result GateResult, err error) {
	ops := mergeGateOps(systemGateOps(), injected)
	if err := validateGateRoots(ctx, input); err != nil {
		return GateResult{}, err
	}
	err = recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		if _, err := ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("revise gate: validate audit binding: %w", err)
		}
		document, err := ops.readDocument(input.RecordRoot)
		if err != nil {
			return fmt.Errorf("revise gate: read state: %w", err)
		}
		if _, err := ops.readEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
			return fmt.Errorf("revise gate: revalidate audit binding after state read: %w", err)
		}
		stage, progress, err := resolveGateState(document.State, input)
		if err != nil {
			return err
		}
		if progress.CheckboxState != state.CheckboxStateRevising {
			return fmt.Errorf("revise gate: current stage %q is not revising: %w", progress.Slug, ErrInvalidGate)
		}
		if err := validateGateCapabilities(stage); err != nil {
			return err
		}
		if err := validateGatePhaseState(document, stage); err != nil {
			return err
		}
		if decision := EvaluateStageCompletion(CompletionInput{
			Current:  stage,
			Catalog:  input.Catalog,
			RecordFS: input.RecordRoot.FS(),
		}); !decision.Ready {
			return fmt.Errorf("revise gate: completion is not ready (%s): %s: %w", decision.Blocker, decision.Reason, ErrGateNotReady)
		}
		lastUpdated, err := document.LastUpdated()
		if err != nil {
			return fmt.Errorf("revise gate: read last updated: %w", err)
		}
		updatedAt := ops.now().UTC().Format(time.RFC3339)
		replacement, err := state.Patch(document.Content, state.PatchRequest{
			Fields: []state.FieldPatch{{
				Field:       state.CanonicalFieldLastUpdated,
				Expected:    lastUpdated,
				Replacement: updatedAt,
			}},
			StageMarkers: []state.StageMarkerPatch{{
				Slug:        progress.Slug,
				Expected:    state.StageMarkerRevising,
				Replacement: state.StageMarkerAwaitingApproval,
			}},
		})
		if err != nil {
			return fmt.Errorf("revise gate: patch state: %w", err)
		}
		if err := ops.appendAudit(ctx, guard, input.ProjectRoot, input.RecordRoot, []audit.Event{{
			Event: "STAGE_AWAITING_APPROVAL",
			Fields: map[string]string{
				"Stage":   stage.Slug,
				"Details": "Re-entering gate after revision",
			},
		}}); err != nil {
			return fmt.Errorf("revise gate: append audit: %w", err)
		}
		updatedState, err := state.Parse(replacement)
		if err != nil {
			return fmt.Errorf("revise gate: parse replacement: %w", err)
		}
		if err := ops.writeState(input.RecordRoot, replacement); err != nil {
			return fmt.Errorf("revise gate: write state: %w", err)
		}
		result = GateResult{
			State:   updatedState,
			Content: slices.Clone(replacement),
			Stage:   cloneGateStage(stage),
			Changed: true,
		}
		return nil
	})
	return result, err
}

func maxIntValue() int { return int(^uint(0) >> 1) }

func validateGateRoots(ctx context.Context, input GateInput) error {
	if ctx == nil {
		return fmt.Errorf("gate: nil context: %w", ErrInvalidGate)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("gate: project and record roots are required: %w", ErrInvalidGate)
	}
	return nil
}

func resolveGateState(current state.State, input GateInput) (graph.Stage, state.StageProgress, error) {
	if current.WorkflowStatus() != state.WorkflowStatusRunning {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: workflow status is not Running: %w", ErrInvalidGate)
	}
	slug := current.CurrentStage()
	if slug == "" || slug == "none" {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage is empty or none: %w", ErrInvalidGate)
	}
	if input.Current.Slug != "" && input.Current.Slug != slug {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: requested stage %q differs from current stage %q: %w", input.Current.Slug, slug, ErrInvalidGate)
	}
	stage, found := completionCatalogStage(input.Catalog, slug)
	if !found {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage %q is absent or duplicated in graph: %w", slug, ErrStateCatalogMismatch)
	}
	if input.Current.Slug != "" && !sameCompletionStage(input.Current, stage) {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: requested stage metadata does not match graph: %w", ErrStateCatalogMismatch)
	}
	var (
		progress   state.StageProgress
		foundStage bool
		liveCount  int
	)
	for _, candidate := range current.Stages() {
		switch candidate.CheckboxState {
		case state.CheckboxStateInProgress, state.CheckboxStateAwaitingApproval, state.CheckboxStateRevising:
			liveCount++
		}
		if candidate.Slug != slug {
			continue
		}
		if foundStage {
			return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage has duplicate state rows: %w", ErrInvalidGate)
		}
		progress = candidate
		foundStage = true
	}
	if !foundStage {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage %q is absent from state: %w", slug, ErrInvalidGate)
	}
	if progress.PlanAction != state.PlanActionExecute {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage %q is not executable: %w", slug, ErrInvalidGate)
	}
	switch progress.CheckboxState {
	case state.CheckboxStateInProgress, state.CheckboxStateAwaitingApproval, state.CheckboxStateRevising:
		// All gate operations share the one-live-stage and graph checks. Each
		// operation then narrows the permitted marker for its transition.
	default:
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: current stage %q is not a live gate marker: %w", slug, ErrInvalidGate)
	}
	if liveCount != 1 {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: state has %d live stages, want one: %w", liveCount, ErrInvalidGate)
	}
	phase, ok := lifecyclePhaseForGraphPhase(stage.Phase)
	if !ok || phase != current.LifecyclePhase() {
		return graph.Stage{}, state.StageProgress{}, fmt.Errorf("gate: graph phase does not match lifecycle phase: %w", ErrStateCatalogMismatch)
	}
	return stage, progress, nil
}

func validateGatePhaseState(_ state.Document, stage graph.Stage) error {
	if stage.Phase == "initialization" || stage.Phase == "construction" {
		return fmt.Errorf("gate: %s phase gate is unsupported in this walking skeleton: %w", stage.Phase, ErrUnsupportedGate)
	}
	return nil
}

func cloneGateStage(stage graph.Stage) graph.Stage {
	stage.SupportAgents = slices.Clone(stage.SupportAgents)
	stage.Scopes = slices.Clone(stage.Scopes)
	stage.Sensors = slices.Clone(stage.Sensors)
	stage.ProducesKinds = cloneGateKinds(stage.ProducesKinds)
	stage.Produces = slices.Clone(stage.Produces)
	stage.OptionalProduces = slices.Clone(stage.OptionalProduces)
	stage.Consumes = slices.Clone(stage.Consumes)
	stage.RequiresStages = slices.Clone(stage.RequiresStages)
	return stage
}

func cloneGateKinds(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	clone := make(map[string][]string, len(values))
	for key, entries := range values {
		clone[key] = slices.Clone(entries)
	}
	return clone
}

func validateGateCapabilities(stage graph.Stage) error {
	if stage.SummaryConfirmation != "" {
		return fmt.Errorf("stage %q declares unsupported summary confirmation %q: %w", stage.Slug, stage.SummaryConfirmation, ErrUnsupportedGate)
	}
	if stage.Mode == "pipeline" {
		return fmt.Errorf("stage %q declares unsupported pipeline mode: %w", stage.Slug, ErrUnsupportedGate)
	}
	if stage.Reviewer != "" {
		return fmt.Errorf("stage %q declares unsupported reviewer %q: %w", stage.Slug, stage.Reviewer, ErrUnsupportedGate)
	}
	if len(stage.Sensors) != 0 {
		return fmt.Errorf("stage %q declares unsupported sensors: %w", stage.Slug, ErrUnsupportedGate)
	}
	if stage.WorkspaceRequires {
		return fmt.Errorf("stage %q declares unsupported workspace evidence: %w", stage.Slug, ErrUnsupportedGate)
	}
	if stage.Mode == "agent-team" || isUnsupportedPerUnitStage(stage) {
		return fmt.Errorf("stage %q declares unsupported dispatcher or per-unit execution: %w", stage.Slug, ErrUnsupportedGate)
	}
	if stage.ProducesKinds != nil || stage.Slug == "reverse-engineering" {
		return fmt.Errorf("stage %q declares unsupported artifact applicability: %w", stage.Slug, ErrUnsupportedGate)
	}
	return nil
}
