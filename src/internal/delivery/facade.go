package delivery

import (
	"context"
	"errors"
	"fmt"

	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/steering"
)

var (
	ErrInvalidDelivery          = errors.New("delivery: invalid request")
	ErrDeliveryWorkflow         = errors.New("delivery: workflow continuation rejected")
	activeDirectiveMarkerCommit = WriteActiveDirectiveMarker
)

// WorkflowError marks a validly parsed request whose requested workflow
// continuation is no longer admissible. The CLI renders it as a terminal
// {"kind":"error",...} directive.
type WorkflowError struct {
	Message string
	Cause   error
}

func (err *WorkflowError) Error() string { return err.Message }
func (err *WorkflowError) Unwrap() error { return err.Cause }

func IsWorkflowError(err error) bool {
	var workflowErr *WorkflowError
	return errors.As(err, &workflowErr)
}

// DeliveryResult is one committed directive publication. Wire is owned by
// the result and is returned only after the marker transaction succeeds.
type DeliveryResult struct {
	Kind          ActiveDirectiveKind
	Part          int
	Parts         int
	Wire          []byte
	ContinueToken string
	Marker        ActiveDirectiveMarker
}

type NextInput = RunStageInput

// Next composes a fresh run-stage view and commits its active marker before
// returning the canonical directive wire.
func Next(ctx context.Context, input RunStageInput) (DeliveryResult, error) {
	if err := validateDeliveryInput(ctx, input); err != nil {
		return DeliveryResult{}, err
	}
	var result DeliveryResult
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		result, err = nextWithGuard(ctx, guard, input)
		return err
	})
	if err != nil {
		return DeliveryResult{}, err
	}
	return result, nil
}

func Continue(ctx context.Context, input RunStageInput, token string) (DeliveryResult, error) {
	if err := validateDeliveryInput(ctx, input); err != nil {
		return DeliveryResult{}, err
	}
	if token == "" {
		return DeliveryResult{}, fmt.Errorf("delivery continue: token is required: %w", ErrInvalidDelivery)
	}
	var result DeliveryResult
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		result, err = continueWithGuard(ctx, guard, input, token)
		return err
	})
	if err != nil {
		return DeliveryResult{}, err
	}
	return result, nil
}

func validateDeliveryInput(ctx context.Context, input RunStageInput) error {
	if ctx == nil {
		return fmt.Errorf("delivery: context is nil: %w", ErrInvalidDelivery)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("delivery: project and record roots are required: %w", ErrInvalidDelivery)
	}
	return nil
}

func nextWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput) (DeliveryResult, error) {
	composition, err := ComposeRunStageWithGuard(ctx, guard, input)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery next: compose run-stage: %w", err)
	}
	if len(composition.Chunks) == 0 {
		marker := activeDirectiveMarkerForComposition(input, composition, ActiveDirectiveKindRunStage, ActiveDirectiveCommandNext, composition.Wire, "", 0, 0)
		if err := activeDirectiveMarkerCommit(input.RecordRoot, marker); err != nil {
			return DeliveryResult{}, fmt.Errorf("delivery next: commit run-stage marker: %w", err)
		}
		return DeliveryResult{
			Kind:   ActiveDirectiveKindRunStage,
			Wire:   append([]byte(nil), composition.Wire...),
			Marker: marker,
		}, nil
	}
	key, err := steering.ReadOrCreateContinuationKey(input.ProjectRoot, input.RecordRoot)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery next: read continuation key: %w", err)
	}
	claims := composition.Claims
	token, err := steering.EncodeContinuationToken(key, claims)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery next: encode continuation token: %w", err)
	}
	part := 1
	loadWire, err := steering.MarshalLoad(steering.LoadDirective{
		Stage:         composition.Freshness.Stage,
		Bundle:        composition.Bundle,
		Part:          part,
		Parts:         len(composition.Chunks),
		RulesContent:  composition.Chunks[0],
		ContinueToken: token,
	})
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery next: marshal load-steering: %w", err)
	}
	marker := activeDirectiveMarkerForComposition(input, composition, ActiveDirectiveKindLoadSteering, ActiveDirectiveCommandNext, loadWire, token, part, len(composition.Chunks))
	if err := activeDirectiveMarkerCommit(input.RecordRoot, marker); err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery next: commit load-steering marker: %w", err)
	}
	return DeliveryResult{
		Kind:          ActiveDirectiveKindLoadSteering,
		Part:          part,
		Parts:         len(composition.Chunks),
		Wire:          loadWire,
		ContinueToken: token,
		Marker:        marker,
	}, nil
}

func continueWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput, token string) (DeliveryResult, error) {
	composition, err := ComposeRunStageWithGuard(ctx, guard, input)
	if err != nil {
		if errors.Is(err, ErrSelectionMismatch) {
			return DeliveryResult{}, newWorkflowError("active selection no longer matches continuation", err)
		}
		return DeliveryResult{}, fmt.Errorf("delivery continue: compose run-stage: %w", err)
	}
	if len(composition.Chunks) == 0 {
		return DeliveryResult{}, newWorkflowError("continuation has no active steering parts", ErrDeliveryWorkflow)
	}
	marker, found, err := ReadActiveDirectiveMarker(input.RecordRoot)
	if err != nil {
		return DeliveryResult{}, newWorkflowError("active continuation marker is invalid", err)
	}
	if !found {
		return DeliveryResult{}, newWorkflowError("no active continuation marker exists", ErrActiveDirectiveNotFound)
	}
	key, err := steering.ReadOrCreateContinuationKey(input.ProjectRoot, input.RecordRoot)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery continue: read continuation key: %w", err)
	}
	claims, err := steering.DecodeContinuationToken(key, token)
	if err != nil {
		return DeliveryResult{}, newWorkflowError("continuation token is invalid", err)
	}
	if err := validateContinuationMarker(marker, input, composition, claims, token); err != nil {
		return DeliveryResult{}, newWorkflowError("continuation is not the active cursor", err)
	}
	step, err := steering.AdvanceContinuation(claims, composition.Freshness, composition.Chunks)
	if err != nil {
		return DeliveryResult{}, newWorkflowError("continuation is stale or invalid", err)
	}
	if step.Complete {
		finalMarker := activeDirectiveMarkerForComposition(input, composition, ActiveDirectiveKindRunStage, ActiveDirectiveCommandContinue, composition.Wire, "", 0, 0)
		if err := activeDirectiveMarkerCommit(input.RecordRoot, finalMarker); err != nil {
			return DeliveryResult{}, fmt.Errorf("delivery continue: commit run-stage marker: %w", err)
		}
		return DeliveryResult{
			Kind:   ActiveDirectiveKindRunStage,
			Wire:   append([]byte(nil), composition.Wire...),
			Marker: finalMarker,
		}, nil
	}
	nextToken, err := steering.EncodeContinuationToken(key, step.Next)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery continue: encode successor token: %w", err)
	}
	loadWire, err := steering.MarshalLoad(steering.LoadDirective{
		Stage:         composition.Freshness.Stage,
		Bundle:        composition.Bundle,
		Part:          step.Part,
		Parts:         step.Parts,
		RulesContent:  step.RulesContent,
		ContinueToken: nextToken,
	})
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery continue: marshal successor load-steering: %w", err)
	}
	nextMarker := activeDirectiveMarkerForComposition(input, composition, ActiveDirectiveKindLoadSteering, ActiveDirectiveCommandContinue, loadWire, nextToken, step.Part, step.Parts)
	if err := activeDirectiveMarkerCommit(input.RecordRoot, nextMarker); err != nil {
		return DeliveryResult{}, fmt.Errorf("delivery continue: commit successor marker: %w", err)
	}
	return DeliveryResult{
		Kind:          ActiveDirectiveKindLoadSteering,
		Part:          step.Part,
		Parts:         step.Parts,
		Wire:          loadWire,
		ContinueToken: nextToken,
		Marker:        nextMarker,
	}, nil
}

func validateContinuationMarker(
	marker ActiveDirectiveMarker,
	input RunStageInput,
	composition RunStageComposition,
	claims steering.ContinuationClaims,
	token string,
) error {
	stateHash := ""
	if composition.Freshness.StateHash != nil {
		stateHash = *composition.Freshness.StateHash
	}
	if marker.Kind != ActiveDirectiveKindLoadSteering || marker.ContinueToken != token ||
		marker.Part != claims.NextPart || marker.Parts != len(composition.Chunks) || marker.CursorHarness != "codex" ||
		(marker.Delivery != ActiveDirectiveDeliveryIssued && marker.Delivery != ActiveDirectiveDeliveryDelivered) {
		return ErrDeliveryWorkflow
	}
	if marker.ActiveAttempt == nil || marker.ActiveAttempt.IssuedStateSHA256 != stateHash ||
		marker.ActiveAttempt.SessionID != marker.OwnerSession || marker.ActiveAttempt.OwnerEpoch != marker.OwnerEpoch ||
		marker.ActiveAttempt.ContextEpoch != marker.ContextEpoch || marker.ActiveAttempt.Status != ActiveDirectiveAttemptSettled ||
		marker.ActiveAttempt.CursorInputSHA256 != continuationTokenSHA(token) {
		return ErrDeliveryWorkflow
	}
	if err := ValidateActiveDirectiveContext(marker, ActiveDirectiveContext{
		ProjectSHA256: sha256Hex(input.Identity.ProjectPath()),
		IntentUUID:    input.IntentUUID,
		StatePresent:  true,
		StateSHA256:   stateHash,
	}); err != nil {
		return err
	}
	if marker.Part < 1 || marker.Part > len(composition.Chunks) {
		return ErrDeliveryWorkflow
	}
	currentWire, err := steering.MarshalLoad(steering.LoadDirective{
		Stage:         composition.Freshness.Stage,
		Bundle:        composition.Bundle,
		Part:          marker.Part,
		Parts:         len(composition.Chunks),
		RulesContent:  composition.Chunks[marker.Part-1],
		ContinueToken: token,
	})
	if err != nil {
		return fmt.Errorf("validate continuation marker wire: %w", err)
	}
	if marker.ActiveAttempt.ResultSHA256 != sha256Hex(string(currentWire)) || marker.ActiveAttempt.ResultRevision != marker.Revision {
		return ErrDeliveryWorkflow
	}
	return nil
}

func newWorkflowError(message string, cause error) error {
	if cause == nil {
		cause = ErrDeliveryWorkflow
	} else if !errors.Is(cause, ErrDeliveryWorkflow) {
		cause = errors.Join(ErrDeliveryWorkflow, cause)
	}
	return &WorkflowError{Message: message, Cause: cause}
}

func activeDirectiveMarkerForComposition(
	input RunStageInput,
	composition RunStageComposition,
	kind ActiveDirectiveKind,
	command ActiveDirectiveCommandKind,
	wire []byte,
	token string,
	part, parts int,
) ActiveDirectiveMarker {
	stateHash := ""
	if composition.Freshness.StateHash != nil {
		stateHash = *composition.Freshness.StateHash
	}
	projectHash := sha256Hex(input.Identity.ProjectPath())
	intentUUID := cloneDeliveryString(input.IntentUUID)
	revision := 0
	if previous, found, err := ReadActiveDirectiveMarker(input.RecordRoot); err == nil && found {
		revision = previous.Revision + 1
	}
	ownerSession := "sessionless:" + projectHash[:16]
	return ActiveDirectiveMarker{
		Version:             2,
		Stage:               composition.Freshness.Stage,
		StateSHA256:         stateHash,
		Revision:            revision,
		ProjectSHA256:       projectHash,
		IntentUUID:          intentUUID,
		StatePresent:        true,
		CursorHarness:       "codex",
		OwnerSession:        ownerSession,
		OwnerEpoch:          0,
		ContextEpoch:        0,
		Kind:                kind,
		Part:                part,
		Parts:               parts,
		ContinueToken:       token,
		ContinueTokenSHA256: continuationTokenSHA(token),
		Delivery:            ActiveDirectiveDeliveryIssued,
		NeedsRehydrate:      false,
		ActiveAttempt: &ActiveDirectiveAttempt{
			CommandKind:       command,
			CommandSHA256:     sha256Hex(string(command)),
			IssuedStateSHA256: stateHash,
			SessionID:         ownerSession,
			OwnerEpoch:        0,
			ContextEpoch:      0,
			Status:            ActiveDirectiveAttemptSettled,
			ClaimRevision:     revision,
			CursorInputSHA256: continuationTokenSHA(token),
			ResultSHA256:      sha256Hex(string(wire)),
			ResultRevision:    revision,
		},
		EventSequence:        revision,
		HumanSequence:        0,
		EngineSequence:       0,
		ConversationSequence: revision,
		StopCount:            0,
	}
}

func continuationTokenSHA(token string) string {
	if token == "" {
		return ""
	}
	return sha256Hex(token)
}

func cloneDeliveryString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
