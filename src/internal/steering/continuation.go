package steering

import (
	"errors"
	"fmt"
)

// ContinuationFreshness identifies the current values bound to a continuation.
type ContinuationFreshness struct {
	Stage         string
	Scope         string
	Bundle        string
	DirectiveHash string
	RouteHash     string
	StateHash     *string
}

// ContinuationStep is the next continuation result.
type ContinuationStep struct {
	Complete     bool
	Part         int
	Parts        int
	RulesContent []RuleContent
	Next         ContinuationClaims
}

// ErrStaleContinuation indicates that a continuation no longer matches the current run.
var ErrStaleContinuation = errors.New("steering: stale continuation")

// ErrInvalidContinuationPart indicates that a continuation refers to no available part.
var ErrInvalidContinuationPart = errors.New("steering: invalid continuation part")

// StaleContinuationField identifies the freshness binding that changed.
type StaleContinuationField uint8

const (
	StaleContinuationFieldUnknown StaleContinuationField = iota
	StaleContinuationFieldStage
	StaleContinuationFieldScope
	StaleContinuationFieldBundle
	StaleContinuationFieldDirectiveHash
	StaleContinuationFieldRouteHash
	StaleContinuationFieldStateHash
)

// StaleContinuationError identifies the freshness binding that made a continuation stale.
type StaleContinuationError struct {
	Field StaleContinuationField
}

func (e *StaleContinuationError) Error() string {
	return ErrStaleContinuation.Error()
}

func (e *StaleContinuationError) Unwrap() error {
	return ErrStaleContinuation
}

// AdvanceContinuation advances a continuation after validating its freshness binding.
func AdvanceContinuation(claims ContinuationClaims, current ContinuationFreshness, chunks [][]RuleContent) (ContinuationStep, error) {
	if claims.Stage != current.Stage {
		return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldStage}
	}
	if claims.Scope != current.Scope {
		return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldScope}
	}
	if claims.Bundle != current.Bundle {
		return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldBundle}
	}
	if claims.DirectiveHash != current.DirectiveHash {
		return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldDirectiveHash}
	}
	if claims.RouteHash != current.RouteHash {
		return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldRouteHash}
	}
	if claims.StateAware {
		stateHashMismatch := (claims.StateHash == nil) != (current.StateHash == nil)
		if !stateHashMismatch && claims.StateHash != nil && *claims.StateHash != *current.StateHash {
			stateHashMismatch = true
		}
		if stateHashMismatch {
			return ContinuationStep{}, &StaleContinuationError{Field: StaleContinuationFieldStateHash}
		}
	}
	if len(chunks) == 0 {
		return ContinuationStep{}, fmt.Errorf("continuation has no parts: %w", ErrInvalidContinuationPart)
	}
	if claims.NextPart < 1 || claims.NextPart > len(chunks) {
		return ContinuationStep{}, fmt.Errorf("continuation part %d is outside 1..%d: %w", claims.NextPart, len(chunks), ErrInvalidContinuationPart)
	}
	if claims.NextPart == len(chunks) {
		return ContinuationStep{Complete: true}, nil
	}

	rules := chunks[claims.NextPart]
	var rulesCopy []RuleContent
	if rules != nil {
		rulesCopy = make([]RuleContent, len(rules))
		copy(rulesCopy, rules)
	}
	next := cloneContinuationClaims(claims)
	next.NextPart++
	return ContinuationStep{
		Part:         claims.NextPart + 1,
		Parts:        len(chunks),
		RulesContent: rulesCopy,
		Next:         next,
	}, nil
}

func cloneContinuationClaims(claims ContinuationClaims) ContinuationClaims {
	cloned := claims
	cloned.Unit = cloneContinuationString(claims.Unit)
	cloned.UnitKind = cloneContinuationString(claims.UnitKind)
	cloned.NextStage.Value = cloneContinuationString(claims.NextStage.Value)
	cloned.SwarmSettled = cloneContinuationBool(claims.SwarmSettled)
	cloned.StateHash = cloneContinuationString(claims.StateHash)
	return cloned
}

func cloneContinuationString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneContinuationBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
