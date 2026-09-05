package main

import (
	"context"
	"errors"
	"os"
	"testing"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestHumanTurnHookRecordsValidPromptForActiveWorkflowWithoutPayload(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousRecord := humanTurnRecord
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnRecord = previousRecord
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	var calls int
	var gotIdentity string
	humanTurnRecord = func(_ context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root) error {
		calls++
		gotIdentity = identity.String()
		if projectRoot == nil || recordRoot == nil {
			t.Error("human turn recorder received nil root")
		}
		return nil
	}
	humanTurnRootCloser = func(*os.Root) error { return nil }

	err := runHumanTurnHook(
		func() ([]byte, error) { return []byte(`{"session_id":"not-an-authority","prompt":"Approve"}`), nil },
		nil,
		func(string) string { return "" },
	)
	if err != nil {
		t.Fatalf("runHumanTurnHook() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("human turn recorder calls = %d, want 1", calls)
	}
	if gotIdentity != fixture.input.Identity.String() {
		t.Errorf("record identity = %q, want %q", gotIdentity, fixture.input.Identity.String())
	}
}

func TestHumanTurnHookFailsOpenForMalformedEmptyUnattendedAndInactiveInput(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		readErr     error
		unattended  string
		resolverErr error
	}{
		{name: "empty", payload: []byte{}},
		{name: "malformed", payload: []byte("{"), resolverErr: errors.New("resolver must not run")},
		{name: "read failure", readErr: errors.New("stdin failure")},
		{name: "unattended", payload: []byte(`{"prompt":"Approve"}`), unattended: "1"},
		{name: "inactive workflow", payload: []byte(`{"prompt":"Approve"}`), resolverErr: errors.New("no active workflow")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			resolverCalls := 0
			oldResolver := humanTurnInputResolver
			oldRecord := humanTurnRecord
			t.Cleanup(func() {
				humanTurnInputResolver = oldResolver
				humanTurnRecord = oldRecord
			})
			humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
				resolverCalls++
				return deliverypkg.RunStageInput{}, nil, nil, tt.resolverErr
			}
			humanTurnRecord = func(context.Context, recordlock.Identity, *os.Root, *os.Root) error {
				calls++
				return nil
			}
			err := runHumanTurnHook(func() ([]byte, error) { return tt.payload, tt.readErr }, nil, func(string) string {
				return tt.unattended
			})
			if err != nil {
				t.Fatalf("runHumanTurnHook() error = %v, want fail-open nil", err)
			}
			if calls != 0 {
				t.Errorf("human turn recorder calls = %d, want 0", calls)
			}
			if (tt.name == "empty" || tt.name == "malformed" || tt.name == "read failure" || tt.name == "unattended") && resolverCalls != 0 {
				t.Errorf("resolver calls = %d, want 0 for early no-op", resolverCalls)
			}
		})
	}
}

func TestHumanTurnHookIgnoresNonObjectJSONPayload(t *testing.T) {
	previousResolver := humanTurnInputResolver
	t.Cleanup(func() { humanTurnInputResolver = previousResolver })
	resolverCalls := 0
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		resolverCalls++
		return deliverypkg.RunStageInput{}, nil, nil, errors.New("resolver must not run")
	}
	for _, payload := range []string{`"prompt"`, `[]`, `true`, `123`} {
		if err := runHumanTurnHook(func() ([]byte, error) { return []byte(payload), nil }, nil, nil); err != nil {
			t.Fatalf("runHumanTurnHook(%s) error = %v, want fail-open nil", payload, err)
		}
	}
	if resolverCalls != 0 {
		t.Fatalf("resolver calls = %d, want 0 for non-object JSON", resolverCalls)
	}
}

func TestHumanTurnHookSwallowsAppendAndRootFailures(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousRecord := humanTurnRecord
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnRecord = previousRecord
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	recordErr := errors.New("append failed")
	var recordCalls int
	humanTurnRecord = func(context.Context, recordlock.Identity, *os.Root, *os.Root) error {
		recordCalls++
		return recordErr
	}
	humanTurnRootCloser = func(*os.Root) error { return errors.New("root close failed") }
	if err := runHumanTurnHook(func() ([]byte, error) { return []byte(`{"prompt":"human"}`), nil }, nil, nil); err != nil {
		t.Fatalf("runHumanTurnHook() error = %v, want fail-open nil", err)
	}
	if recordCalls != 1 {
		t.Errorf("human turn recorder calls = %d, want 1", recordCalls)
	}
}
