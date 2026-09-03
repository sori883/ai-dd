//go:build integration

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestOpenGateIntegrationTransitionsInProgressStage(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	if _, err := OpenGate(context.Background(), GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}

	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [?] intent-capture — EXECUTE") {
		t.Fatalf("state does not contain awaiting marker: %s", content)
	}

	var records []audit.AuditRecord
	err = recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = audit.ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	})
	if err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(records) != 1 || records[0].Event != "STAGE_AWAITING_APPROVAL" {
		t.Fatalf("audit records = %#v, want one STAGE_AWAITING_APPROVAL", records)
	}
	if records[0].Fields["Stage"] != "intent-capture" {
		t.Errorf("audit Stage = %q, want intent-capture", records[0].Fields["Stage"])
	}
}

func TestOpenGateIntegrationRevalidatesAlreadyAwaitingWithoutMutation(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("initial OpenGate() error = %v", err)
	}
	beforeState, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	beforeRecords := gateIntegrationReadRecords(t, fixture)

	result, err := OpenGate(context.Background(), input)
	if err != nil {
		t.Fatalf("already-awaiting OpenGate() error = %v", err)
	}
	if !result.AlreadyAwaiting || result.Changed {
		t.Fatalf("already-awaiting result = %+v, want unchanged success", result)
	}
	afterState, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(afterState) != string(beforeState) {
		t.Fatalf("already-awaiting state changed:\n got %q\nwant %q", afterState, beforeState)
	}
	afterRecords := gateIntegrationReadRecords(t, fixture)
	if len(afterRecords) != len(beforeRecords) {
		t.Fatalf("already-awaiting audit count = %d, want %d", len(afterRecords), len(beforeRecords))
	}
}

func TestOpenGateIntegrationLeavesStateUnchangedWhenArtifactIsMissing(t *testing.T) {
	fixture := newGateIntegrationFixtureWithProduces(t, `["artifact"]`)
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}
	if _, err := OpenGate(context.Background(), input); !errors.Is(err, ErrGateNotReady) {
		t.Fatalf("OpenGate() error = %v, want ErrGateNotReady", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("artifact failure changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, fixture); len(records) != 0 {
		t.Fatalf("artifact failure audit records = %#v, want empty", records)
	}
}

func TestOpenGateIntegrationLeavesStateUnchangedWhenAuditAppendFails(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	if err := os.WriteFile(filepath.Join(fixture.projectDir, "aidlc", ".aidlc-clone-id"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}
	if _, err := OpenGate(context.Background(), input); err == nil {
		t.Fatal("OpenGate() error = nil, want audit append failure")
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("audit failure changed state:\n got %q\nwant %q", after, before)
	}
	if _, err := fixture.recordRoot.Stat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("audit after append failure = %v, want absent", err)
	}
}

func TestOpenGateIntegrationLeavesAuditWhenStateSaveFails(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	saveErr := errors.New("injected state save failure")
	_, err = openGateWithOps(context.Background(), input, gateOps{
		writeState: func(*os.Root, []byte) error { return saveErr },
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("OpenGate() error = %v, want injected state save failure", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("state-save failure changed state:\n got %q\nwant %q", after, before)
	}
	records := gateIntegrationReadRecords(t, fixture)
	if len(records) != 1 || records[0].Event != "STAGE_AWAITING_APPROVAL" {
		t.Fatalf("state-save failure audit records = %#v, want retained STAGE_AWAITING_APPROVAL", records)
	}
}

func TestOpenGateIntegrationHonorsSameRecordCompetition(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	}
	beforeState, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	competitor, err := recordlock.Acquire(context.Background(), fixture.identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if competitor != nil {
			_ = competitor.Release()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := OpenGate(ctx, input); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("OpenGate() error = %v, want context deadline from same-record lock", err)
	}
	afterState, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(afterState) != string(beforeState) {
		t.Fatalf("lock competition changed state:\n got %q\nwant %q", afterState, beforeState)
	}
	if err := competitor.Release(); err != nil {
		t.Fatal(err)
	}
	competitor = nil
	if records := gateIntegrationReadRecords(t, fixture); len(records) != 0 {
		t.Fatalf("lock competition audit records = %#v, want empty", records)
	}
}

func TestRejectGateIntegrationDoesNotRequireCompletionEvidence(t *testing.T) {
	fixture := newGateIntegrationFixtureWithProduces(t, `["artifact"]`)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "The required artifact is not ready yet.",
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	if _, err := RejectGate(context.Background(), input); err != nil {
		t.Fatalf("RejectGate() error = %v, want rejection despite missing artifact", err)
	}
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [R] intent-capture — EXECUTE") {
		t.Fatalf("state does not contain revising marker: %s", content)
	}
}

func TestRejectGateIntegrationRejectsRevisionOverflowWithoutMutation(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	statePath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "aidlc-state.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	maxRevision := strconv.Itoa(int(^uint(0) >> 1))
	content = []byte(strings.Replace(string(content), "- **Revision Count**: 0", "- **Revision Count**: "+maxRevision, 1))
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RejectGate(context.Background(), input); !errors.Is(err, ErrInvalidGate) {
		t.Fatalf("RejectGate() error = %v, want ErrInvalidGate", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("overflow rejection changed state:\n got %q\nwant %q", after, before)
	}
}

func TestRejectGateIntegrationRejectsMalformedRevisionCountWithoutMutation(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	statePath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "aidlc-state.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	content = []byte(strings.Replace(string(content), "- **Revision Count**: 0", "- **Revision Count**: 01", 1))
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RejectGate(context.Background(), input); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("malformed Revision Count RejectGate() error = %v, want fs.ErrInvalid", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("malformed Revision Count changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, fixture); len(records) != 1 || records[0].Event != "HUMAN_TURN" {
		t.Fatalf("malformed Revision Count audit records = %#v, want only HUMAN_TURN", records)
	}
}

func TestRejectGateIntegrationRequiresFreshHumanTurn(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RejectGate(context.Background(), input); !errors.Is(err, ErrStaleHumanTurn) {
		t.Fatalf("RejectGate() error = %v, want ErrStaleHumanTurn", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("stale RejectGate() changed state:\n got %q\nwant %q", after, before)
	}
}

func TestRejectGateIntegrationRecordsRevisionAfterFreshHumanTurn(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	if _, err := RejectGate(context.Background(), input); err != nil {
		t.Fatalf("RejectGate() error = %v", err)
	}

	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "- [R] intent-capture — EXECUTE") {
		t.Fatalf("state does not contain revising marker: %s", content)
	}
	if !strings.Contains(text, "- **Revision Count**: 1") {
		t.Fatalf("state does not contain incremented revision count: %s", content)
	}
	records := gateIntegrationReadRecords(t, fixture)
	if len(records) != 4 {
		t.Fatalf("audit records = %d, want open, human, and two reject records", len(records))
	}
	if records[1].Event != "HUMAN_TURN" || records[2].Event != "GATE_REJECTED" || records[3].Event != "STAGE_REVISING" {
		t.Fatalf("audit events = %#v, want HUMAN_TURN then GATE_REJECTED then STAGE_REVISING", records)
	}
}

func TestRejectGateIntegrationRejectsRepeatedRevision(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation again.",
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	if _, err := RejectGate(context.Background(), input); err != nil {
		t.Fatalf("initial RejectGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RejectGate(context.Background(), input); !errors.Is(err, ErrInvalidGate) {
		t.Fatalf("repeated RejectGate() error = %v, want ErrInvalidGate", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("repeated rejection changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, fixture); len(records) != 5 {
		t.Fatalf("repeated rejection audit records = %d, want the existing five records", len(records))
	}
}

func TestReviseGateIntegrationReentersApprovalAfterRevision(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	if _, err := RejectGate(context.Background(), input); err != nil {
		t.Fatalf("RejectGate() error = %v", err)
	}
	if _, err := ReviseGate(context.Background(), input); err != nil {
		t.Fatalf("ReviseGate() error = %v", err)
	}

	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [?] intent-capture — EXECUTE") {
		t.Fatalf("state does not contain re-entered awaiting marker: %s", content)
	}
	records := gateIntegrationReadRecords(t, fixture)
	if len(records) != 5 {
		t.Fatalf("audit records = %d, want open, human, reject pair, and re-entry", len(records))
	}
	if records[4].Event != "STAGE_AWAITING_APPROVAL" || records[4].Fields["Details"] != "Re-entering gate after revision" {
		t.Fatalf("re-entry audit = %#v, want STAGE_AWAITING_APPROVAL with revision details", records[4])
	}
}

func TestApprovalValidationIntegrationNeedsNewReceiptAfterRevision(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Request Changes",
		Feedback:    "Please revise the implementation.",
	}
	if _, err := OpenGate(context.Background(), input); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	oldContent, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	freshSnapshot := gateIntegrationReadRecords(t, fixture)
	if !audit.HumanTurnFresh(freshSnapshot) {
		t.Fatalf("pre-revision audit snapshot = %#v, want fresh HUMAN_TURN", freshSnapshot)
	}
	if _, err := RejectGate(context.Background(), input); err != nil {
		t.Fatalf("RejectGate() error = %v", err)
	}
	if _, err := ReviseGate(context.Background(), input); err != nil {
		t.Fatalf("ReviseGate() error = %v", err)
	}
	progress := state.StageProgress{CheckboxState: state.CheckboxStateAwaitingApproval, CheckboxMarker: "[?]"}
	err = recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		if _, err := validateApprovalGateDecision(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, oldContent, progress, "Approve"); !errors.Is(err, ErrStaleHumanTurn) {
			return fmt.Errorf("approval with old state snapshot error = %v, want ErrStaleHumanTurn", err)
		}
		if err := audit.Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []audit.Event{{
			Event:  "HUMAN_TURN",
			Fields: map[string]string{"Prompt": "approval"},
		}}); err != nil {
			return fmt.Errorf("append HUMAN_TURN: %w", err)
		}
		if _, err := validateApprovalGateDecision(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, oldContent, progress, "Approve"); err != nil {
			return fmt.Errorf("approval with fresh ledger error = %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApprovalValidationIntegrationRejectsEmptyLedger(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	progress := state.StageProgress{CheckboxState: state.CheckboxStateAwaitingApproval, CheckboxMarker: "[?]"}
	err = recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		_, err := validateApprovalGateDecision(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, content, progress, "Approve")
		if !errors.Is(err, ErrStaleHumanTurn) {
			return fmt.Errorf("approval with empty ledger error = %v, want ErrStaleHumanTurn", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApprovalValidationIntegrationRejectsUnboundRootOrGuard(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	other := newGateIntegrationFixture(t)
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	progress := state.StageProgress{CheckboxState: state.CheckboxStateAwaitingApproval, CheckboxMarker: "[?]"}
	err = recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		_, err := validateApprovalGateDecision(context.Background(), other.identity, guard, other.projectRoot, other.recordRoot, content, progress, "Approve")
		if !errors.Is(err, audit.ErrGuardIdentity) {
			return fmt.Errorf("approval with mismatched Guard error = %v, want audit.ErrGuardIdentity", err)
		}
		_, err = validateApprovalGateDecision(context.Background(), fixture.identity, guard, fixture.projectRoot, other.recordRoot, content, progress, "Approve")
		if !errors.Is(err, audit.ErrInvalidRoot) {
			return fmt.Errorf("approval with mismatched record root error = %v, want audit.ErrInvalidRoot", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func gateIntegrationAppendHumanTurn(t *testing.T, fixture gateIntegrationFixture) {
	t.Helper()
	err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return audit.Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []audit.Event{{
			Event:  "HUMAN_TURN",
			Fields: map[string]string{"Prompt": "approval"},
		}})
	})
	if err != nil {
		t.Fatalf("append HUMAN_TURN: %v", err)
	}
}

func gateIntegrationReadRecords(t *testing.T, fixture gateIntegrationFixture) []audit.AuditRecord {
	t.Helper()
	var records []audit.AuditRecord
	err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = audit.ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	})
	if err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	return records
}

type gateIntegrationFixture struct {
	projectDir  string
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
	catalog     graph.Snapshot
	stage       graph.Stage
}

func newGateIntegrationFixture(t *testing.T) gateIntegrationFixture {
	return newGateIntegrationFixtureWithProduces(t, "[]")
}

func newGateIntegrationFixtureWithProduces(t *testing.T, produces string) gateIntegrationFixture {
	t.Helper()
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "aidlc", ".aidlc-clone-id"), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: []byte(fmt.Sprintf(`[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"aidlc-orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":%s,"consumes":[],"requires_stage":[]}
]`, produces))},
		"scope-grid.json": {Data: []byte(`{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE"}}}`)},
	}
	catalog, err := graph.Load(dataFS)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:         catalog,
		Scope:         "classic",
		ScopeMetadata: scope.Metadata{Name: "classic", Depth: "Standard"},
		Workspace:     state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:   projectDir,
		StartDate:     "2026-09-04T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, "aidlc-state.md"), []byte(initial.StateContent), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	var stage graph.Stage
	for _, candidate := range catalog.Stages() {
		if candidate.Slug == "intent-capture" {
			stage = candidate
			break
		}
	}
	return gateIntegrationFixture{
		projectDir:  projectDir,
		identity:    identity,
		projectRoot: projectRoot,
		recordRoot:  recordRoot,
		catalog:     catalog,
		stage:       stage,
	}
}
