package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestHumanTurnHookRecordsValidPromptForActiveWorkflowWithoutPayload(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousObserve := humanTurnObserve
	previousRecordIfCurrent := humanTurnRecordIfCurrent
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnObserve = previousObserve
		humanTurnRecordIfCurrent = previousRecordIfCurrent
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	var calls int
	var gotIdentity string
	humanTurnObserve = func(context.Context, recordlock.Identity, *os.Root, *os.Root) (audit.HumanTurnObservation, error) {
		return audit.HumanTurnObservation{}, nil
	}
	humanTurnRecordIfCurrent = func(_ context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, _ audit.HumanTurnObservation) error {
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

func TestHumanTurnHookFailsOpenForReadUnattendedAndInactiveInput(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		readErr     error
		unattended  string
		resolverErr error
	}{
		{name: "read failure", readErr: errors.New("stdin failure")},
		{name: "unattended", payload: []byte(`{"prompt":"Approve"}`), unattended: "1"},
		{name: "inactive workflow", payload: []byte(`{"prompt":"Approve"}`), resolverErr: errors.New("no active workflow")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			resolverCalls := 0
			oldResolver := humanTurnInputResolver
			oldRecordIfCurrent := humanTurnRecordIfCurrent
			t.Cleanup(func() {
				humanTurnInputResolver = oldResolver
				humanTurnRecordIfCurrent = oldRecordIfCurrent
			})
			humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
				resolverCalls++
				return deliverypkg.RunStageInput{}, nil, nil, tt.resolverErr
			}
			humanTurnRecordIfCurrent = func(context.Context, recordlock.Identity, *os.Root, *os.Root, audit.HumanTurnObservation) error {
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
			if (tt.name == "read failure" || tt.name == "unattended") && resolverCalls != 0 {
				t.Errorf("resolver calls = %d, want 0 for early no-op", resolverCalls)
			}
		})
	}
}

func TestHumanTurnHookAcceptsPayloadVariantsAndBoundsInput(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousObserve := humanTurnObserve
	previousRecordIfCurrent := humanTurnRecordIfCurrent
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnObserve = previousObserve
		humanTurnRecordIfCurrent = previousRecordIfCurrent
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	var calls int
	humanTurnObserve = func(context.Context, recordlock.Identity, *os.Root, *os.Root) (audit.HumanTurnObservation, error) {
		return audit.HumanTurnObservation{}, nil
	}
	humanTurnRecordIfCurrent = func(context.Context, recordlock.Identity, *os.Root, *os.Root, audit.HumanTurnObservation) error {
		calls++
		return nil
	}
	humanTurnRootCloser = func(*os.Root) error { return nil }

	for _, payload := range [][]byte{nil, []byte("{"), []byte(`"legacy prompt"`), []byte(`[]`), []byte(`true`), []byte(`123`), []byte("legacy prompt")} {
		if err := runHumanTurnHook(func() ([]byte, error) { return payload, nil }, nil, nil); err != nil {
			t.Fatalf("runHumanTurnHook(%q) error = %v, want silent success", payload, err)
		}
	}
	reader := &trackingHumanTurnReader{reader: strings.NewReader(strings.Repeat("x", 128*1024))}
	if err := humanTurnHook(reader, nil, nil); err != nil {
		t.Fatalf("humanTurnHook(oversized) error = %v, want silent success", err)
	}
	if calls != 8 {
		t.Fatalf("human turn recorder calls = %d, want one per invocation", calls)
	}
	if reader.maxRead > 64*1024 {
		t.Fatalf("hook read size = %d, want bounded at 64 KiB", reader.maxRead)
	}
}

func TestHumanTurnObservationRejectsAdvancedStateOrAudit(t *testing.T) {
	tests := []struct {
		name          string
		advance       func(*testing.T, reportAdapterFixture)
		wantAuditSize int
	}{
		{
			name: "state advances",
			advance: func(t *testing.T, fixture reportAdapterFixture) {
				statePath := filepath.Join(fixture.input.Identity.ProjectRoot(), "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md")
				stateBytes, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatalf("ReadFile(state): %v", err)
				}
				advanced := strings.Replace(string(stateBytes), "- **Current Stage**: intent-capture", "- **Current Stage**: workspace-scaffold", 1)
				if advanced == string(stateBytes) {
					t.Fatal("state fixture did not contain current stage")
				}
				if err := recordlock.With(context.Background(), fixture.input.Identity, func(*recordlock.Guard) error {
					return state.WriteState(fixture.recordRoot, []byte(advanced))
				}); err != nil {
					t.Fatalf("advance state: %v", err)
				}
			},
			wantAuditSize: 0,
		},
		{
			name: "audit resolution advances",
			advance: func(t *testing.T, fixture reportAdapterFixture) {
				if err := recordlock.With(context.Background(), fixture.input.Identity, func(guard *recordlock.Guard) error {
					return audit.AppendForIdentity(context.Background(), fixture.input.Identity, guard, fixture.projectRoot, fixture.recordRoot, []audit.Event{{Event: "STAGE_AWAITING_APPROVAL", Fields: map[string]string{"Stage": "intent-capture"}}})
				}); err != nil {
					t.Fatalf("advance audit: %v", err)
				}
			},
			wantAuditSize: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newReportAdapterFixture(t)
			observation, err := audit.ObserveHumanTurn(context.Background(), fixture.input.Identity, fixture.projectRoot, fixture.recordRoot)
			if err != nil {
				t.Fatalf("ObserveHumanTurn() error = %v", err)
			}
			tt.advance(t, fixture)
			if err := audit.RecordHumanTurnIfCurrent(context.Background(), fixture.input.Identity, fixture.projectRoot, fixture.recordRoot, observation); !errors.Is(err, audit.ErrHumanTurnObservationStale) {
				t.Fatalf("RecordHumanTurnIfCurrent() error = %v, want ErrHumanTurnObservationStale", err)
			}
			var records []audit.AuditRecord
			if err := recordlock.With(context.Background(), fixture.input.Identity, func(guard *recordlock.Guard) error {
				var err error
				records, err = audit.ReadEvents(context.Background(), fixture.input.Identity, guard, fixture.projectRoot, fixture.recordRoot)
				return err
			}); err != nil {
				t.Fatalf("ReadEvents() error = %v", err)
			}
			if len(records) != tt.wantAuditSize {
				t.Fatalf("audit records = %#v, want %d records", records, tt.wantAuditSize)
			}
		})
	}
}

func TestHumanTurnHookSkipsWhenStateAdvancesAfterObservation(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousObserve := humanTurnObserve
	previousRecordIfCurrent := humanTurnRecordIfCurrent
	previousAfterObservation := humanTurnAfterObservation
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnObserve = previousObserve
		humanTurnRecordIfCurrent = previousRecordIfCurrent
		humanTurnAfterObservation = previousAfterObservation
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	humanTurnObserve = audit.ObserveHumanTurn
	humanTurnRecordIfCurrent = audit.RecordHumanTurnIfCurrent
	humanTurnRootCloser = func(*os.Root) error { return nil }
	observed := make(chan struct{})
	continueHook := make(chan struct{})
	humanTurnAfterObservation = func() {
		close(observed)
		<-continueHook
	}
	hookDone := make(chan error, 1)
	go func() {
		hookDone <- runHumanTurnHook(func() ([]byte, error) { return []byte("legacy prompt"), nil }, nil, nil)
	}()
	select {
	case <-observed:
	case <-time.After(2 * time.Second):
		t.Fatal("hook did not expose its observation boundary")
	}
	statePath := filepath.Join(fixture.input.Identity.ProjectRoot(), "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md")
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state): %v", err)
	}
	advanced := strings.Replace(string(stateBytes), "- **Current Stage**: intent-capture", "- **Current Stage**: workspace-scaffold", 1)
	if advanced == string(stateBytes) {
		t.Fatal("state fixture did not contain current stage")
	}
	if err := recordlock.With(context.Background(), fixture.input.Identity, func(*recordlock.Guard) error {
		return state.WriteState(fixture.recordRoot, []byte(advanced))
	}); err != nil {
		t.Fatalf("advance state: %v", err)
	}
	close(continueHook)
	select {
	case err := <-hookDone:
		if err != nil {
			t.Fatalf("runHumanTurnHook() error = %v, want fail-open nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("hook did not finish after observation race")
	}
	if _, err := fixture.recordRoot.Lstat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("audit after stale hook = %v, want absent", err)
	}
}

type trackingHumanTurnReader struct {
	reader  *strings.Reader
	maxRead int
}

func (r *trackingHumanTurnReader) Read(p []byte) (int, error) {
	if len(p) > r.maxRead {
		r.maxRead = len(p)
	}
	return r.reader.Read(p)
}

func TestHumanTurnHookSwallowsAppendAndRootFailures(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := humanTurnInputResolver
	previousObserve := humanTurnObserve
	previousRecordIfCurrent := humanTurnRecordIfCurrent
	previousCloser := humanTurnRootCloser
	t.Cleanup(func() {
		humanTurnInputResolver = previousResolver
		humanTurnObserve = previousObserve
		humanTurnRecordIfCurrent = previousRecordIfCurrent
		humanTurnRootCloser = previousCloser
	})
	humanTurnInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	recordErr := errors.New("append failed")
	var recordCalls int
	humanTurnObserve = func(context.Context, recordlock.Identity, *os.Root, *os.Root) (audit.HumanTurnObservation, error) {
		return audit.HumanTurnObservation{}, nil
	}
	humanTurnRecordIfCurrent = func(context.Context, recordlock.Identity, *os.Root, *os.Root, audit.HumanTurnObservation) error {
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
