package delivery

import (
	"context"
	"encoding/json"
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
		// read-context is a read-only operation. Provision its HMAC key while
		// publishing the directive so a later context read never mutates the
		// record transaction.
		if _, err := steering.ReadOrCreateContinuationKey(input.ProjectRoot, input.RecordRoot); err != nil {
			return DeliveryResult{}, fmt.Errorf("delivery next: read continuation key: %w", err)
		}
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
	if marker.Kind != ActiveDirectiveKindLoadSteering || marker.Stage != composition.Freshness.Stage ||
		!activeDirectiveSHA256Equal(marker.ContinueTokenSHA256, token) ||
		marker.Part != claims.NextPart || marker.Parts != len(composition.Chunks) || marker.CursorHarness != "codex" ||
		(marker.Delivery != ActiveDirectiveDeliveryIssued && marker.Delivery != ActiveDirectiveDeliveryDelivered) {
		return ErrDeliveryWorkflow
	}
	if marker.ActiveAttempt == nil || marker.ActiveAttempt.IssuedStateSHA256 != stateHash ||
		marker.ActiveAttempt.SessionID != marker.OwnerSession || marker.ActiveAttempt.OwnerEpoch != marker.OwnerEpoch ||
		marker.ActiveAttempt.ContextEpoch != marker.ContextEpoch || marker.ActiveAttempt.Status != ActiveDirectiveAttemptSettled {
		return ErrDeliveryWorkflow
	}
	if marker.ActiveAttempt.CursorInputSHA256 != "" && !activeDirectiveSHA256Equal(marker.ActiveAttempt.CursorInputSHA256, token) {
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
	if marker.ActiveAttempt.ResultSHA256 != "" {
		if !activeDirectiveSHA256Equal(marker.ActiveAttempt.ResultSHA256, string(currentWire)) {
			return ErrDeliveryWorkflow
		}
		if marker.ActiveAttempt.ResultRevision != marker.Revision {
			return ErrDeliveryWorkflow
		}
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
	ownerSession := "sessionless:" + projectHash[:16]
	context := ActiveDirectiveContext{
		ProjectSHA256: projectHash,
		IntentUUID:    intentUUID,
		StatePresent:  true,
		StateSHA256:   stateHash,
	}
	base := freshActiveDirectiveMarkerForComposition(input, composition, ownerSession, stateHash, intentUUID)
	baseValid := false
	if previous, found, err := ReadActiveDirectiveMarker(input.RecordRoot); err == nil && found && previous.Version == 2 &&
		activeDirectiveIdentityMatches(previous, context) {
		if command == ActiveDirectiveCommandNext || ValidateActiveDirectiveContext(previous, context) == nil {
			base = cloneActiveDirectiveMarker(previous)
			baseValid = true
		}
	}
	if baseValid {
		base.Revision++
	} else {
		base.Revision = 1
	}

	base.Version = 2
	base.Stage = composition.Freshness.Stage
	base.StateSHA256 = stateHash
	base.ProjectSHA256 = projectHash
	base.IntentUUID = intentUUID
	base.StatePresent = true
	base.Unit = ""
	base.Units = nil
	base.CodeGenerationSourceSHA256 = ""
	base.CodeGenerationAuthorityRevision = 0
	delete(base.present, "unit")
	delete(base.present, "units")
	delete(base.present, "code_generation_source_sha256")
	delete(base.present, "code_generation_authority_revision")
	base.CursorHarness = "codex"
	base.OwnerSession = ownerSession
	base.Kind = kind
	base.Part = part
	base.Parts = parts
	base.ContinueToken = token
	base.ContinueTokenSHA256 = continuationTokenSHA(token)
	base.Delivery = ActiveDirectiveDeliveryIssued
	base.NeedsRehydrate = false
	if part == 0 {
		delete(base.present, "part")
		delete(base.present, "parts")
	}
	if token == "" {
		delete(base.present, "continue_token")
		delete(base.present, "continue_token_sha256")
	}
	base.ActiveAttempt = activeDirectiveAttemptForPublication(base, baseValid, stateHash, ownerSession, wire, token)
	return base
}

func activeDirectiveIdentityMatches(marker ActiveDirectiveMarker, context ActiveDirectiveContext) bool {
	if marker.ProjectSHA256 != context.ProjectSHA256 {
		return false
	}
	if (marker.IntentUUID == nil) != (context.IntentUUID == nil) {
		return false
	}
	return marker.IntentUUID == nil || *marker.IntentUUID == *context.IntentUUID
}

func freshActiveDirectiveMarkerForComposition(
	input RunStageInput,
	composition RunStageComposition,
	ownerSession, stateHash string,
	intentUUID *string,
) ActiveDirectiveMarker {
	projectHash := sha256Hex(input.Identity.ProjectPath())
	return ActiveDirectiveMarker{
		Version:        2,
		Stage:          composition.Freshness.Stage,
		StateSHA256:    stateHash,
		ProjectSHA256:  projectHash,
		IntentUUID:     cloneDeliveryString(intentUUID),
		StatePresent:   true,
		CursorHarness:  "codex",
		OwnerSession:   ownerSession,
		OwnerEpoch:     0,
		ContextEpoch:   0,
		Kind:           ActiveDirectiveKindError,
		Delivery:       ActiveDirectiveDeliverySuperseded,
		NeedsRehydrate: true,
		ActiveAttempt: &ActiveDirectiveAttempt{
			ID:                "sessionless",
			CommandKind:       ActiveDirectiveCommandNext,
			CommandSHA256:     stateHash,
			IssuedStateSHA256: stateHash,
			SessionID:         ownerSession,
			OwnerEpoch:        0,
			ContextEpoch:      0,
			Status:            ActiveDirectiveAttemptSettled,
		},
	}
}

func activeDirectiveAttemptForPublication(
	base ActiveDirectiveMarker,
	baseValid bool,
	stateHash, ownerSession string,
	wire []byte,
	token string,
) *ActiveDirectiveAttempt {
	fresh := &ActiveDirectiveAttempt{
		ID:                "sessionless",
		CommandKind:       ActiveDirectiveCommandNext,
		CommandSHA256:     stateHash,
		IssuedStateSHA256: stateHash,
		SessionID:         ownerSession,
		OwnerEpoch:        base.OwnerEpoch,
		ContextEpoch:      base.ContextEpoch,
		Status:            ActiveDirectiveAttemptSettled,
		ResultSHA256:      sha256Hex(string(wire)),
		ResultRevision:    base.Revision,
	}
	if !baseValid || base.ActiveAttempt == nil {
		return fresh
	}
	attempt := cloneActiveDirectiveAttempt(*base.ActiveAttempt)
	if attempt.IssuedStateSHA256 != stateHash || attempt.SessionID != ownerSession ||
		attempt.OwnerEpoch != base.OwnerEpoch || attempt.ContextEpoch != base.ContextEpoch ||
		attempt.Status != ActiveDirectiveAttemptSettled {
		fresh.Extra = cloneActiveDirectiveRawMap(base.ActiveAttempt.Extra)
		return fresh
	}
	if attempt.CursorInputSHA256 != "" && !activeDirectiveSHA256Equal(attempt.CursorInputSHA256, token) {
		fresh.Extra = cloneActiveDirectiveRawMap(attempt.Extra)
		return fresh
	}
	if attempt.ResultSHA256 != "" && !activeDirectiveSHA256Equal(attempt.ResultSHA256, string(wire)) {
		fresh.Extra = cloneActiveDirectiveRawMap(attempt.Extra)
		return fresh
	}
	if attempt.ResultSHA256 == "" {
		attempt.ResultSHA256 = fresh.ResultSHA256
	}
	attempt.ResultRevision = base.Revision
	return &attempt
}

func cloneActiveDirectiveMarker(marker ActiveDirectiveMarker) ActiveDirectiveMarker {
	cloned := marker
	cloned.Units = append([]string(nil), marker.Units...)
	cloned.IntentUUID = cloneDeliveryString(marker.IntentUUID)
	cloned.Extra = cloneActiveDirectiveRawMap(marker.Extra)
	cloned.present = cloneActiveDirectivePresence(marker.present)
	if marker.ActiveAttempt != nil {
		attempt := cloneActiveDirectiveAttempt(*marker.ActiveAttempt)
		cloned.ActiveAttempt = &attempt
	}
	if marker.Resume != nil {
		resume := cloneActiveDirectiveResume(*marker.Resume)
		cloned.Resume = &resume
	}
	return cloned
}

func cloneActiveDirectiveAttempt(attempt ActiveDirectiveAttempt) ActiveDirectiveAttempt {
	cloned := attempt
	cloned.Extra = cloneActiveDirectiveRawMap(attempt.Extra)
	cloned.present = cloneActiveDirectivePresence(attempt.present)
	return cloned
}

func cloneActiveDirectiveResume(resume ActiveDirectiveResume) ActiveDirectiveResume {
	cloned := resume
	cloned.IssuingIntentUUID = cloneDeliveryString(resume.IssuingIntentUUID)
	cloned.Extra = cloneActiveDirectiveRawMap(resume.Extra)
	cloned.present = cloneActiveDirectivePresence(resume.present)
	return cloned
}

func cloneActiveDirectiveRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

func cloneActiveDirectivePresence(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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
