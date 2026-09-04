package steering

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAdvanceContinuationRejectsStaleBinding(t *testing.T) {
	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    false,
		Gate:          GateTrue,
	}
	current := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        claims.Bundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
	}
	chunks := [][]RuleContent{
		{{Path: "first.md", Text: "first"}},
		{{Path: "second.md", Text: "second"}},
	}

	tests := []struct {
		name   string
		field  StaleContinuationField
		mutate func(*ContinuationFreshness)
	}{
		{
			name:  "stage",
			field: StaleContinuationFieldStage,
			mutate: func(freshness *ContinuationFreshness) {
				freshness.Stage = "changed-stage"
			},
		},
		{
			name:  "scope",
			field: StaleContinuationFieldScope,
			mutate: func(freshness *ContinuationFreshness) {
				freshness.Scope = "changed-scope"
			},
		},
		{
			name:  "bundle",
			field: StaleContinuationFieldBundle,
			mutate: func(freshness *ContinuationFreshness) {
				freshness.Bundle = "changed-bundle"
			},
		},
		{
			name:  "directive hash",
			field: StaleContinuationFieldDirectiveHash,
			mutate: func(freshness *ContinuationFreshness) {
				freshness.DirectiveHash = "changed-directive-hash"
			},
		},
		{
			name:  "route hash",
			field: StaleContinuationFieldRouteHash,
			mutate: func(freshness *ContinuationFreshness) {
				freshness.RouteHash = "changed-route-hash"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			freshness := current
			test.mutate(&freshness)

			got, err := AdvanceContinuation(claims, freshness, chunks)
			if !errors.Is(err, ErrStaleContinuation) {
				t.Errorf("AdvanceContinuation() error = %v, want ErrStaleContinuation", err)
			} else {
				var staleErr *StaleContinuationError
				if !errors.As(err, &staleErr) {
					t.Errorf("AdvanceContinuation() error = %v, want *StaleContinuationError", err)
				} else if staleErr.Field != test.field {
					t.Errorf("StaleContinuationError.Field = %v, want %v", staleErr.Field, test.field)
				}
			}
			if !reflect.DeepEqual(got, ContinuationStep{}) {
				t.Errorf("AdvanceContinuation() step = %#v, want zero step", got)
			}
		})
	}
}

func TestAdvanceContinuationReturnsEachPartThenCompletes(t *testing.T) {
	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		Gate:          GateTrue,
	}
	freshness := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        claims.Bundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
	}
	content := []RuleContent{
		{Path: "first.md", Text: "# first\n" + strings.Repeat("日🚀", 2800)},
		{Path: "second.md", Text: "# second\n" + strings.Repeat("日🚀", 2800)},
		{Path: "third.md", Text: "# third\n" + strings.Repeat("日🚀", 2800)},
	}
	chunks := ChunkRules(content)
	if len(chunks) < 3 {
		t.Fatalf("ChunkRules() produced %d chunks, want at least 3", len(chunks))
	}

	nextClaims := claims
	for part := 2; part <= len(chunks); part++ {
		step, err := AdvanceContinuation(nextClaims, freshness, chunks)
		if err != nil {
			t.Fatalf("AdvanceContinuation() for part %d error = %v", part, err)
		}
		if step.Complete {
			t.Fatalf("AdvanceContinuation() for part %d marked complete", part)
		}
		if step.Part != part {
			t.Errorf("AdvanceContinuation() part = %d, want %d", step.Part, part)
		}
		if step.Parts != len(chunks) {
			t.Errorf("AdvanceContinuation() parts = %d, want %d", step.Parts, len(chunks))
		}
		if !reflect.DeepEqual(step.RulesContent, chunks[part-1]) {
			t.Errorf("AdvanceContinuation() rules for part %d = %#v, want %#v", part, step.RulesContent, chunks[part-1])
		}
		wantNext := nextClaims
		wantNext.NextPart++
		if !reflect.DeepEqual(step.Next, wantNext) {
			t.Errorf("AdvanceContinuation() next claims = %#v, want %#v", step.Next, wantNext)
		}
		nextClaims = step.Next
	}

	finalStep, err := AdvanceContinuation(nextClaims, freshness, chunks)
	if err != nil {
		t.Fatalf("AdvanceContinuation() completion error = %v", err)
	}
	if !reflect.DeepEqual(finalStep, ContinuationStep{Complete: true}) {
		t.Errorf("AdvanceContinuation() completion = %#v, want complete zero-payload step", finalStep)
	}

	maxInt := int(^uint(0) >> 1)
	boundaryTests := []struct {
		name   string
		claims ContinuationClaims
		chunks [][]RuleContent
	}{
		{name: "part above range", claims: func() ContinuationClaims { value := claims; value.NextPart = len(chunks) + 1; return value }(), chunks: chunks},
		{name: "part below range", claims: func() ContinuationClaims { value := claims; value.NextPart = 0; return value }(), chunks: chunks},
		{name: "part index overflow", claims: func() ContinuationClaims { value := claims; value.NextPart = maxInt; return value }(), chunks: chunks},
		{name: "empty chunks", claims: claims, chunks: nil},
	}
	for _, test := range boundaryTests {
		t.Run(test.name, func(t *testing.T) {
			got, err := AdvanceContinuation(test.claims, freshness, test.chunks)
			if !errors.Is(err, ErrInvalidContinuationPart) {
				t.Errorf("AdvanceContinuation() error = %v, want ErrInvalidContinuationPart", err)
			}
			if !reflect.DeepEqual(got, ContinuationStep{}) {
				t.Errorf("AdvanceContinuation() step = %#v, want zero step", got)
			}
		})
	}
}

func TestAdvanceContinuationChecksStateWhenStateAware(t *testing.T) {
	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    true,
		Gate:          GateTrue,
	}
	current := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        claims.Bundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
	}
	chunks := [][]RuleContent{
		{{Path: "first.md", Text: "first"}},
		{{Path: "second.md", Text: "second"}},
	}

	sameClaimState := "same-state"
	sameCurrentState := "same-state"
	stateAwareSameClaims := claims
	stateAwareSameClaims.StateHash = &sameClaimState
	stateAwareSameCurrent := current
	stateAwareSameCurrent.StateHash = &sameCurrentState

	stateMismatchClaim := "old-state"
	stateMismatchCurrent := "new-state"
	stateAwareMismatchClaims := claims
	stateAwareMismatchClaims.StateHash = &stateMismatchClaim
	stateAwareMismatchCurrent := current
	stateAwareMismatchCurrent.StateHash = &stateMismatchCurrent

	stateClaimsOnly := "claims-state"
	stateAwareClaimsOnly := claims
	stateAwareClaimsOnly.StateHash = &stateClaimsOnly

	stateCurrentOnly := "current-state"
	stateAwareCurrentOnly := current
	stateAwareCurrentOnly.StateHash = &stateCurrentOnly

	nonStateAwareClaims := claims
	nonStateAwareClaims.StateAware = false
	nonStateAwareClaims.StateHash = &stateMismatchClaim
	nonStateAwareCurrent := current
	nonStateAwareCurrent.StateHash = &stateMismatchCurrent

	nonStateAwareClaimsOnly := nonStateAwareClaims
	nonStateAwareCurrentOnly := current

	successTests := []struct {
		name    string
		claims  ContinuationClaims
		current ContinuationFreshness
	}{
		{name: "state-aware equal values", claims: stateAwareSameClaims, current: stateAwareSameCurrent},
		{name: "state-aware both nil", claims: claims, current: current},
		{name: "not state-aware changed values", claims: nonStateAwareClaims, current: nonStateAwareCurrent},
		{name: "not state-aware claims only", claims: nonStateAwareClaimsOnly, current: current},
		{name: "not state-aware current only", claims: claims, current: nonStateAwareCurrentOnly},
	}
	for _, test := range successTests {
		t.Run("success/"+test.name, func(t *testing.T) {
			got, err := AdvanceContinuation(test.claims, test.current, chunks)
			if err != nil {
				t.Fatalf("AdvanceContinuation() error = %v", err)
			}
			wantNext := test.claims
			wantNext.NextPart++
			want := ContinuationStep{
				Part:         2,
				Parts:        len(chunks),
				RulesContent: chunks[1],
				Next:         wantNext,
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("AdvanceContinuation() = %#v, want %#v", got, want)
			}
		})
	}

	staleTests := []struct {
		name    string
		claims  ContinuationClaims
		current ContinuationFreshness
	}{
		{name: "state-aware changed values", claims: stateAwareMismatchClaims, current: stateAwareMismatchCurrent},
		{name: "state-aware claims only", claims: stateAwareClaimsOnly, current: current},
		{name: "state-aware current only", claims: claims, current: stateAwareCurrentOnly},
	}
	for _, test := range staleTests {
		t.Run("stale/"+test.name, func(t *testing.T) {
			got, err := AdvanceContinuation(test.claims, test.current, chunks)
			if !errors.Is(err, ErrStaleContinuation) {
				t.Errorf("AdvanceContinuation() error = %v, want ErrStaleContinuation", err)
			}
			var staleErr *StaleContinuationError
			if !errors.As(err, &staleErr) {
				t.Errorf("AdvanceContinuation() error = %v, want *StaleContinuationError", err)
			} else if staleErr.Field != StaleContinuationFieldStateHash {
				t.Errorf("StaleContinuationError.Field = %v, want %v", staleErr.Field, StaleContinuationFieldStateHash)
			}
			if !reflect.DeepEqual(got, ContinuationStep{}) {
				t.Errorf("AdvanceContinuation() step = %#v, want zero step", got)
			}
		})
	}
}

func TestAdvanceContinuationRejectsChangedRuleMarkdown(t *testing.T) {
	oldContent := []RuleContent{
		{Path: "rules/guide.md", Text: "# 規則\n本文は旧版です🚀"},
	}
	newContent := []RuleContent{
		{Path: "rules/guide.md", Text: "# 規則\n本文は新版です🚀"},
	}
	oldBundle, err := BundleDigest(oldContent)
	if err != nil {
		t.Fatalf("BundleDigest(oldContent) error = %v", err)
	}
	newBundle, err := BundleDigest(newContent)
	if err != nil {
		t.Fatalf("BundleDigest(newContent) error = %v", err)
	}
	if oldBundle == newBundle {
		t.Fatalf("BundleDigest() old and new values are equal: %q", oldBundle)
	}

	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        oldBundle,
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		Gate:          GateTrue,
	}
	current := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        newBundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
	}
	chunks := [][]RuleContent{
		{{Path: "first.md", Text: "first"}},
		{{Path: "second.md", Text: "second"}},
	}

	got, err := AdvanceContinuation(claims, current, chunks)
	if !errors.Is(err, ErrStaleContinuation) {
		t.Errorf("AdvanceContinuation() error = %v, want ErrStaleContinuation", err)
	}
	var staleErr *StaleContinuationError
	if !errors.As(err, &staleErr) {
		t.Errorf("AdvanceContinuation() error = %v, want *StaleContinuationError", err)
	} else if staleErr.Field != StaleContinuationFieldBundle {
		t.Errorf("StaleContinuationError.Field = %v, want %v", staleErr.Field, StaleContinuationFieldBundle)
	}
	if !reflect.DeepEqual(got, ContinuationStep{}) {
		t.Errorf("AdvanceContinuation() step = %#v, want zero step", got)
	}
}

func TestAdvanceContinuationDoesNotConsumeToken(t *testing.T) {
	unit := "unit"
	unitKind := "unit-kind"
	nextStage := "next-stage"
	swarmSettled := true
	stateHash := "state-hash"
	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    false,
		Unit:          &unit,
		UnitKind:      &unitKind,
		Gate:          GateTrue,
		NextStage:     OptionalNullableString{Present: true, Value: &nextStage},
		SwarmSettled:  &swarmSettled,
		StateHash:     &stateHash,
	}
	currentStateHash := "current-state-hash"
	current := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        claims.Bundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
		StateHash:     &currentStateHash,
	}
	chunks := [][]RuleContent{
		{{Path: "first.md", Text: "first"}},
		{{Path: "second.md", Text: "second"}},
	}

	cloneString := func(value *string) *string {
		if value == nil {
			return nil
		}
		cloned := *value
		return &cloned
	}
	cloneBool := func(value *bool) *bool {
		if value == nil {
			return nil
		}
		cloned := *value
		return &cloned
	}
	claimsBefore := claims
	claimsBefore.Unit = cloneString(claims.Unit)
	claimsBefore.UnitKind = cloneString(claims.UnitKind)
	claimsBefore.NextStage.Value = cloneString(claims.NextStage.Value)
	claimsBefore.SwarmSettled = cloneBool(claims.SwarmSettled)
	claimsBefore.StateHash = cloneString(claims.StateHash)
	currentBefore := current
	currentBefore.StateHash = cloneString(current.StateHash)
	chunksBefore := make([][]RuleContent, len(chunks))
	for index, chunk := range chunks {
		if chunk != nil {
			chunksBefore[index] = make([]RuleContent, len(chunk))
			copy(chunksBefore[index], chunk)
		}
	}

	first, err := AdvanceContinuation(claims, current, chunks)
	if err != nil {
		t.Fatalf("first AdvanceContinuation() error = %v", err)
	}
	second, err := AdvanceContinuation(claims, current, chunks)
	if err != nil {
		t.Fatalf("second AdvanceContinuation() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("repeated AdvanceContinuation() results differ: first = %#v, second = %#v", first, second)
	}
	wantClaims := claims
	wantClaims.NextPart = 2
	want := ContinuationStep{
		Part:         2,
		Parts:        len(chunks),
		RulesContent: chunks[1],
		Next:         wantClaims,
	}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("AdvanceContinuation() = %#v, want %#v", first, want)
	}
	if !reflect.DeepEqual(claims, claimsBefore) {
		t.Errorf("claims changed after repeated AdvanceContinuation(): got %#v, want %#v", claims, claimsBefore)
	}
	if !reflect.DeepEqual(current, currentBefore) {
		t.Errorf("current freshness changed after repeated AdvanceContinuation(): got %#v, want %#v", current, currentBefore)
	}
	if !reflect.DeepEqual(chunks, chunksBefore) {
		t.Errorf("chunks changed after repeated AdvanceContinuation(): got %#v, want %#v", chunks, chunksBefore)
	}
}

func TestAdvanceContinuationOwnsReturnedContent(t *testing.T) {
	unit := "unit"
	unitKind := "unit-kind"
	nextStage := "next-stage"
	swarmSettled := true
	stateHash := "state-hash"
	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    true,
		Unit:          &unit,
		UnitKind:      &unitKind,
		Gate:          GateTrue,
		NextStage:     OptionalNullableString{Present: true, Value: &nextStage},
		SwarmSettled:  &swarmSettled,
		StateHash:     &stateHash,
	}
	currentStateHash := "state-hash"
	current := ContinuationFreshness{
		Stage:         claims.Stage,
		Scope:         claims.Scope,
		Bundle:        claims.Bundle,
		DirectiveHash: claims.DirectiveHash,
		RouteHash:     claims.RouteHash,
		StateHash:     &currentStateHash,
	}
	secondChunk := make([]RuleContent, 1, 2)
	secondChunk[0] = RuleContent{Path: "second.md", Text: "second"}
	chunks := [][]RuleContent{
		{{Path: "first.md", Text: "first"}},
		secondChunk,
	}

	cloneString := func(value *string) *string {
		if value == nil {
			return nil
		}
		cloned := *value
		return &cloned
	}
	cloneBool := func(value *bool) *bool {
		if value == nil {
			return nil
		}
		cloned := *value
		return &cloned
	}
	claimsBefore := claims
	claimsBefore.Unit = cloneString(claims.Unit)
	claimsBefore.UnitKind = cloneString(claims.UnitKind)
	claimsBefore.NextStage.Value = cloneString(claims.NextStage.Value)
	claimsBefore.SwarmSettled = cloneBool(claims.SwarmSettled)
	claimsBefore.StateHash = cloneString(claims.StateHash)
	currentBefore := current
	currentBefore.StateHash = cloneString(current.StateHash)
	chunksBefore := make([][]RuleContent, len(chunks))
	for index, chunk := range chunks {
		if chunk != nil {
			chunksBefore[index] = make([]RuleContent, len(chunk))
			copy(chunksBefore[index], chunk)
		}
	}
	secondBackingBefore := make([]RuleContent, cap(secondChunk))
	copy(secondBackingBefore, secondChunk[:cap(secondChunk)])

	first, err := AdvanceContinuation(claims, current, chunks)
	if err != nil {
		t.Fatalf("first AdvanceContinuation() error = %v", err)
	}
	wantClaims := claimsBefore
	wantClaims.NextPart = 2
	want := ContinuationStep{
		Part:         2,
		Parts:        len(chunks),
		RulesContent: chunksBefore[1],
		Next:         wantClaims,
	}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("first AdvanceContinuation() = %#v, want %#v", first, want)
	}

	mutatePointers := func(step *ContinuationStep, prefix string) {
		*step.Next.Unit = prefix + "-unit"
		*step.Next.UnitKind = prefix + "-unit-kind"
		*step.Next.NextStage.Value = prefix + "-next-stage"
		*step.Next.SwarmSettled = false
		*step.Next.StateHash = prefix + "-state-hash"
	}
	first.RulesContent[0] = RuleContent{Path: "first-mutated.md", Text: "first-mutated"}
	first.RulesContent = append(first.RulesContent, RuleContent{Path: "appended.md", Text: "appended"})
	mutatePointers(&first, "first")
	firstRulesAfterMutation := append([]RuleContent(nil), first.RulesContent...)
	firstUnitAfterMutation := *first.Next.Unit
	firstUnitKindAfterMutation := *first.Next.UnitKind
	firstNextStageAfterMutation := *first.Next.NextStage.Value
	firstSwarmSettledAfterMutation := *first.Next.SwarmSettled
	firstStateHashAfterMutation := *first.Next.StateHash

	if !reflect.DeepEqual(claims, claimsBefore) {
		t.Errorf("claims changed after mutating returned step: got %#v, want %#v", claims, claimsBefore)
	}
	if !reflect.DeepEqual(current, currentBefore) {
		t.Errorf("current freshness changed after mutating returned step: got %#v, want %#v", current, currentBefore)
	}
	if !reflect.DeepEqual(chunks, chunksBefore) {
		t.Errorf("chunks changed after mutating returned step: got %#v, want %#v", chunks, chunksBefore)
	}
	if !reflect.DeepEqual(chunks[1][:cap(chunks[1])], secondBackingBefore) {
		t.Errorf("chunk backing changed after appending returned rules: got %#v, want %#v", chunks[1][:cap(chunks[1])], secondBackingBefore)
	}

	second, err := AdvanceContinuation(claims, current, chunks)
	if err != nil {
		t.Fatalf("second AdvanceContinuation() error = %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second AdvanceContinuation() = %#v, want canonical %#v", second, want)
	}

	second.RulesContent[0] = RuleContent{Path: "second-mutated.md", Text: "second-mutated"}
	second.RulesContent = append(second.RulesContent, RuleContent{Path: "second-appended.md", Text: "second-appended"})
	mutatePointers(&second, "second")
	if !reflect.DeepEqual(first.RulesContent, firstRulesAfterMutation) {
		t.Errorf("second rules mutation changed first result: got %#v, want %#v", first.RulesContent, firstRulesAfterMutation)
	}
	if *first.Next.Unit != firstUnitAfterMutation {
		t.Errorf("second Unit mutation changed first result: got %q, want %q", *first.Next.Unit, firstUnitAfterMutation)
	}
	if *first.Next.UnitKind != firstUnitKindAfterMutation {
		t.Errorf("second UnitKind mutation changed first result: got %q, want %q", *first.Next.UnitKind, firstUnitKindAfterMutation)
	}
	if *first.Next.NextStage.Value != firstNextStageAfterMutation {
		t.Errorf("second NextStage mutation changed first result: got %q, want %q", *first.Next.NextStage.Value, firstNextStageAfterMutation)
	}
	if *first.Next.SwarmSettled != firstSwarmSettledAfterMutation {
		t.Errorf("second SwarmSettled mutation changed first result: got %t, want %t", *first.Next.SwarmSettled, firstSwarmSettledAfterMutation)
	}
	if *first.Next.StateHash != firstStateHashAfterMutation {
		t.Errorf("second StateHash mutation changed first result: got %q, want %q", *first.Next.StateHash, firstStateHashAfterMutation)
	}
	if !reflect.DeepEqual(claims, claimsBefore) {
		t.Errorf("claims changed after mutating second returned step: got %#v, want %#v", claims, claimsBefore)
	}
	if !reflect.DeepEqual(current, currentBefore) {
		t.Errorf("current freshness changed after mutating second returned step: got %#v, want %#v", current, currentBefore)
	}
	if !reflect.DeepEqual(chunks, chunksBefore) {
		t.Errorf("chunks changed after mutating second returned step: got %#v, want %#v", chunks, chunksBefore)
	}
}
