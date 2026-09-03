//go:build integration

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func TestApproveGateIntegrationAdvancesSamePhaseAfterFreshHumanTurn(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := ApproveInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
		Choice:      "Approve",
	}
	if _, err := OpenGate(context.Background(), GateInput{
		Identity:    input.Identity,
		ProjectRoot: input.ProjectRoot,
		RecordRoot:  input.RecordRoot,
		Current:     input.Current,
		Catalog:     input.Catalog,
	}); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{
		identity:    fixture.identity,
		projectRoot: fixture.projectRoot,
		recordRoot:  fixture.recordRoot,
	})

	result, err := ApproveGate(context.Background(), input)
	if err != nil {
		t.Fatalf("ApproveGate() error = %v", err)
	}
	if !result.ApprovalSaved || !result.FinalTransitionComplete {
		t.Fatalf("ApproveGate() result = %+v, want completed transaction", result)
	}
	if result.State.CurrentStage() != "next-stage" || result.State.Summary().InProgress != "next-stage" {
		t.Fatalf("ApproveGate() state current/in-progress = %q/%q, want next-stage", result.State.CurrentStage(), result.State.Summary().InProgress)
	}
	lastCompleted, err := state.LastCompletedStage(result.Content)
	if err != nil || lastCompleted != "intent-capture" {
		t.Fatalf("LastCompletedStage() = (%q, %v), want intent-capture", lastCompleted, err)
	}
	activeAgent, err := state.ActiveAgent(result.Content)
	if err != nil || activeAgent != "aidlc-product-agent" {
		t.Fatalf("ActiveAgent() = (%q, %v), want aidlc-product-agent", activeAgent, err)
	}
	nextAction, err := state.NextAction(result.Content)
	if err != nil || nextAction != "Execute Next Stage" {
		t.Fatalf("NextAction() = (%q, %v), want Execute Next Stage", nextAction, err)
	}
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "- [x] intent-capture — EXECUTE") || !strings.Contains(text, "- [-] next-stage — EXECUTE") {
		t.Fatalf("state markers after approval = %s", content)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{
		identity:    fixture.identity,
		projectRoot: fixture.projectRoot,
		recordRoot:  fixture.recordRoot,
	})
	if len(records) != 5 {
		t.Fatalf("audit records = %d, want SAA, HUMAN_TURN, approval, completion, start", len(records))
	}
	for index, want := range []string{"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED", "STAGE_STARTED"} {
		if records[index].Event != want {
			t.Errorf("audit[%d].Event = %q, want %q", index, records[index].Event, want)
		}
	}
}

func TestApproveGateIntegrationCompletesPhaseBoundaryInOrder(t *testing.T) {
	fixture := newApproveIntegrationFixtureWithNextPhase(t, "inception")
	input := ApproveInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog, Choice: "Approve",
	}
	if _, err := OpenGate(context.Background(), GateInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog,
	}); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{
		identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot,
	})
	result, err := ApproveGate(context.Background(), input)
	if err != nil {
		t.Fatalf("ApproveGate() error = %v", err)
	}
	if result.State.CurrentStage() != "next-stage" || result.State.LifecyclePhase() != state.LifecyclePhaseInception {
		t.Fatalf("boundary state current/phase = %q/%q, want next-stage/INCEPTION", result.State.CurrentStage(), result.State.LifecyclePhase())
	}
	content := string(result.Content)
	if !strings.Contains(content, "- [x] intent-capture — EXECUTE") || !strings.Contains(content, "- [-] next-stage — EXECUTE") ||
		!strings.Contains(content, "- **Ideation**: Verified") || !strings.Contains(content, "- **Inception**: Active") {
		t.Fatalf("boundary state = %s", content)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{
		identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot,
	})
	if len(records) != 8 {
		t.Fatalf("boundary audit records = %d, want 8", len(records))
	}
	for index, want := range []string{
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED",
		"PHASE_COMPLETED", "PHASE_VERIFIED", "PHASE_STARTED", "STAGE_STARTED",
	} {
		if records[index].Event != want {
			t.Errorf("audit[%d].Event = %q, want %q", index, records[index].Event, want)
		}
	}
	if records[4].Fields["From phase"] != "ideation" || records[4].Fields["To phase"] != "inception" {
		t.Errorf("PHASE_COMPLETED fields = %#v", records[4].Fields)
	}
}

func TestApproveGateIntegrationCompletesTerminalWorkflow(t *testing.T) {
	fixture := newGateIntegrationFixture(t)
	input := ApproveInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog, Choice: "Approve",
	}
	if _, err := OpenGate(context.Background(), GateInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog,
	}); err != nil {
		t.Fatalf("OpenGate() error = %v", err)
	}
	gateIntegrationAppendHumanTurn(t, fixture)
	result, err := ApproveGate(context.Background(), input)
	if err != nil {
		t.Fatalf("ApproveGate() error = %v", err)
	}
	if result.State.WorkflowStatus() != state.WorkflowStatusCompleted || result.State.CurrentStage() != "intent-capture" ||
		result.State.Summary().InProgress != "none" || result.State.NextStage() != "none" {
		t.Fatalf("terminal state = %#v, want Completed with retained current and no next", result.State)
	}
	content := string(result.Content)
	if !strings.Contains(content, "- **Next Action**: Workflow complete") || !strings.Contains(content, "- **Ideation**: Verified") {
		t.Fatalf("terminal state fields = %s", content)
	}
	records := gateIntegrationReadRecords(t, fixture)
	if len(records) != 7 {
		t.Fatalf("terminal audit records = %d, want 7", len(records))
	}
	for index, want := range []string{
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED",
		"PHASE_COMPLETED", "PHASE_VERIFIED", "WORKFLOW_COMPLETED",
	} {
		if records[index].Event != want {
			t.Errorf("audit[%d].Event = %q, want %q", index, records[index].Event, want)
		}
	}
	if records[6].Fields["Details"] != "Scope: classic, 2 stages completed" {
		t.Errorf("WORKFLOW_COMPLETED Details = %q", records[6].Fields["Details"])
	}
	lastCompleted, err := state.LastCompletedStage(result.Content)
	if err != nil || lastCompleted != "intent-capture" {
		t.Fatalf("terminal LastCompletedStage() = (%q, %v), want intent-capture", lastCompleted, err)
	}
	activeAgent, err := state.ActiveAgent(result.Content)
	if err != nil || activeAgent != "aidlc-product-agent" {
		t.Fatalf("terminal ActiveAgent() = (%q, %v), want aidlc-product-agent", activeAgent, err)
	}
}

func TestApproveGateIntegrationRequiresFreshHumanAndExactChoice(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApproveGate(context.Background(), input); !errors.Is(err, ErrStaleHumanTurn) {
		t.Fatalf("ApproveGate() without human error = %v, want ErrStaleHumanTurn", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("stale receipt changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot}); len(records) != 1 {
		t.Fatalf("stale receipt audit records = %d, want one gate-open row", len(records))
	}
}

func TestApproveGateIntegrationRejectsNonExactChoiceWithoutMutation(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApproveGate(context.Background(), input); !errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("ApproveGate() non-exact choice error = %v, want ErrInvalidDecision", err)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid choice changed state:\n got %q\nwant %q", after, before)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 2 || records[1].Event != "HUMAN_TURN" {
		t.Fatalf("invalid choice audit records = %#v, want gate-open then HUMAN_TURN", records)
	}
}

func TestApproveGateIntegrationRejectsUnrecordedRevisionBeforeApprovalAudit(t *testing.T) {
	fixture := newApproveIntegrationFixtureWithProduces(t, "[]")
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if err := os.WriteFile(filepath.Join(fixture.projectDir, "aidlc", ".aidlc-clone-id"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApproveGate(context.Background(), input)
	if err == nil {
		t.Fatal("ApproveGate() error = nil, want audit-backed failure")
	}
	if result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("revision failure result = %+v, want no saved approval", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("revision failure changed state:\n got %q\nwant %q", after, before)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 2 {
		t.Fatalf("revision failure audit records = %d, want gate-open and human only", len(records))
	}
}

func TestApproveGateIntegrationRetainsAuditWhenFirstStateSaveFails(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	saveErr := errors.New("injected first state save failure")
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := approveGateWithOps(context.Background(), input, gateOps{
		writeState: func(*os.Root, []byte) error { return saveErr },
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("approveGateWithOps() error = %v, want injected state failure", err)
	}
	if result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("first state failure result = %+v, want no saved approval", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("first state failure changed state:\n got %q\nwant %q", after, before)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 4 || records[2].Event != "GATE_APPROVED" || records[3].Event != "STAGE_COMPLETED" {
		t.Fatalf("first state failure audit = %#v, want approval audit retained", records)
	}
}

func TestApproveGateIntegrationLeavesStateWhenFirstApprovalAuditFails(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	auditErr := errors.New("injected first approval audit failure")
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := approveGateWithOps(context.Background(), input, gateOps{
		appendAudit: func(context.Context, *recordlock.Guard, *os.Root, *os.Root, []audit.Event) error {
			return auditErr
		},
	})
	if !errors.Is(err, auditErr) {
		t.Fatalf("approveGateWithOps() error = %v, want injected audit failure", err)
	}
	if result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("first audit failure result = %+v, want no saved approval", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("first audit failure changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot}); len(records) != 2 {
		t.Fatalf("first audit failure records = %d, want gate-open and human only", len(records))
	}
}

func TestApproveGateIntegrationReportsPartialResultWhenSecondAuditFails(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	auditErr := errors.New("injected second audit failure")
	appendCalls := 0
	result, err := approveGateWithOps(context.Background(), input, gateOps{
		appendAudit: func(ctx context.Context, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, events []audit.Event) error {
			appendCalls++
			if appendCalls == 2 {
				return auditErr
			}
			return audit.Append(ctx, guard, projectRoot, recordRoot, events)
		},
	})
	if !errors.Is(err, auditErr) {
		t.Fatalf("approveGateWithOps() error = %v, want injected second audit failure", err)
	}
	if !result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("second audit failure result = %+v, want saved intermediate only", result)
	}
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [x] intent-capture — EXECUTE") || !strings.Contains(string(content), "- **Current Stage**: intent-capture") {
		t.Fatalf("second audit failure state = %s, want first approval state", content)
	}
	if records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot}); len(records) != 4 {
		t.Fatalf("second audit failure audit records = %d, want approval pair only", len(records))
	}
}

func TestApproveGateIntegrationReportsPartialResultWhenSecondStateSaveFails(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	saveErr := errors.New("injected second state save failure")
	saveCalls := 0
	result, err := approveGateWithOps(context.Background(), input, gateOps{
		writeState: func(root *os.Root, content []byte) error {
			saveCalls++
			if saveCalls == 2 {
				return saveErr
			}
			return state.WriteState(root, content)
		},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("approveGateWithOps() error = %v, want injected second state failure", err)
	}
	if !result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("second state failure result = %+v, want saved intermediate only", result)
	}
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [x] intent-capture — EXECUTE") || !strings.Contains(string(content), "- **Current Stage**: intent-capture") {
		t.Fatalf("second state failure state = %s, want first approval state", content)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 5 || records[4].Event != "STAGE_STARTED" {
		t.Fatalf("second state failure audit = %#v, want transition audit retained", records)
	}
}

func TestApproveGateIntegrationPreservesUnknownBytesAndRevisionCount(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	statePath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "aidlc-state.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	content = []byte(strings.Replace(string(content), "- **Revision Count**: 0", "- **Revision Count**: 2", 1) + "\n## Unknown Extension\nkeep exact bytes  \n")
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	result, err := ApproveGate(context.Background(), input)
	if err != nil {
		t.Fatalf("ApproveGate() error = %v", err)
	}
	if got, err := state.RevisionCount(result.Content); err != nil || got != 2 {
		t.Fatalf("RevisionCount() = (%d, %v), want (2, nil)", got, err)
	}
	if !strings.Contains(string(result.Content), "## Unknown Extension\nkeep exact bytes  \n") {
		t.Fatalf("unknown bytes were not preserved: %q", result.Content)
	}
}

func TestApproveGateIntegrationRejectsReapprovalWithoutDuplicateEvents(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if _, err := ApproveGate(context.Background(), input); err != nil {
		t.Fatalf("first ApproveGate() error = %v", err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApproveGate(context.Background(), input)
	if !errors.Is(err, ErrInvalidGate) {
		t.Fatalf("reapproval error = %v, want ErrInvalidGate", err)
	}
	if result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("reapproval result = %+v, want no second transaction", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("reapproval changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot}); len(records) != 5 {
		t.Fatalf("reapproval audit records = %d, want no duplicates", len(records))
	}
}

func TestApproveGateIntegrationReadsFreshStateForSecondTransition(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	readCalls := 0
	result, err := approveGateWithOps(context.Background(), input, gateOps{
		readDocument: func(root *os.Root) (state.Document, error) {
			readCalls++
			return state.ReadDocument(root)
		},
	})
	if err != nil {
		t.Fatalf("approveGateWithOps() error = %v", err)
	}
	if !result.FinalTransitionComplete || readCalls < 2 {
		t.Fatalf("approveGateWithOps() result/read calls = %+v/%d, want final transition and at least two state reads", result, readCalls)
	}
}

func TestApproveGateIntegrationSerializesSameRecordCompetition(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), fixture.identity)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = guard.Release() }()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result, err := ApproveGate(ctx, input)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("competing ApproveGate() error = %v, want context deadline", err)
	}
	if result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("competing result = %+v, want no transaction", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("competing approval changed state:\n got %q\nwant %q", after, before)
	}
}

func TestApproveGateIntegrationLeavesFirstApprovalWhenNextStageUnsupported(t *testing.T) {
	fixture := newApproveIntegrationFixtureWithNextPhase(t, "construction")
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	result, err := ApproveGate(context.Background(), input)
	if !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("ApproveGate() error = %v, want ErrUnsupportedGate", err)
	}
	if !result.ApprovalSaved || result.FinalTransitionComplete {
		t.Fatalf("unsupported next result = %+v, want first approval only", result)
	}
	content, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "- [x] intent-capture — EXECUTE") || !strings.Contains(string(content), "- **Current Stage**: intent-capture") {
		t.Fatalf("unsupported next state = %s, want intermediate approval state", content)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 4 {
		t.Fatalf("unsupported next audit records = %d, want gate-open, human, and approval pair", len(records))
	}
}

func TestApproveGateIntegrationRejectsMissingArtifactWithoutMutation(t *testing.T) {
	fixture := newApproveIntegrationFixtureWithProduces(t, `["intent-statement"]`)
	artifactPath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "ideation", "intent-capture", "intent-statement.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApproveGate(context.Background(), input)
	if !errors.Is(err, ErrGateNotReady) {
		t.Fatalf("ApproveGate() error = %v, want ErrGateNotReady", err)
	}
	if result.ApprovalSaved {
		t.Fatalf("missing artifact result = %+v, want no approval", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("missing artifact changed state:\n got %q\nwant %q", after, before)
	}
	if records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot}); len(records) != 2 {
		t.Fatalf("missing artifact audit records = %d, want gate-open and human only", len(records))
	}
}

func TestApproveGateIntegrationRejectsUnknownScopeBeforeApprovalAudit(t *testing.T) {
	fixture := newApproveIntegrationFixture(t)
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	statePath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "aidlc-state.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	content = []byte(strings.Replace(string(content), "- **Scope**: classic", "- **Scope**: unknown", 1))
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ApproveGate(context.Background(), input)
	if !errors.Is(err, ErrStateCatalogMismatch) {
		t.Fatalf("ApproveGate() error = %v, want ErrStateCatalogMismatch", err)
	}
	if result.ApprovalSaved {
		t.Fatalf("unknown scope result = %+v, want no approval", result)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 2 {
		t.Fatalf("unknown scope audit records = %d, want gate-open and human only", len(records))
	}
}

func TestApproveGateIntegrationRejectsReaderBackstopBeforeApprovalAudit(t *testing.T) {
	fixture := newApproveIntegrationFixtureWithProduces(t, `["intent-statement"]`)
	artifactPath := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "ideation", "intent-capture", "intent-statement.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := approveInputForFixture(fixture, "Approve")
	if _, err := openApproveFixtureGate(t, fixture, input); err != nil {
		t.Fatal(err)
	}
	gateIntegrationAppendHumanTurn(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	auditDir := filepath.Join(fixture.projectDir, "aidlc", "spaces", "default", "intents", "build", "audit")
	manualTimestamp := time.Now().UTC().Truncate(time.Second).Add(time.Hour).Format(time.RFC3339)
	manual := fmt.Sprintf("# AI-DLC Audit Log\n\n## Artifact Updated\n**Timestamp**: %s\n**Event**: ARTIFACT_UPDATED\n**File**: /record/ideation/intent-capture/intent-statement.md\n\n---\n", manualTimestamp)
	if err := os.WriteFile(filepath.Join(auditDir, "manual.md"), []byte(manual), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApproveGate(context.Background(), input)
	if !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("ApproveGate() error = %v, want ErrUnsupportedGate from backstop", err)
	}
	if result.ApprovalSaved {
		t.Fatalf("backstop result = %+v, want no approval", result)
	}
	after, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("backstop changed state:\n got %q\nwant %q", after, before)
	}
	records := gateIntegrationReadRecords(t, gateIntegrationFixture{identity: fixture.identity, projectRoot: fixture.projectRoot, recordRoot: fixture.recordRoot})
	if len(records) != 3 {
		t.Fatalf("backstop audit records = %d, want gate-open, human, artifact write", len(records))
	}
}

func approveInputForFixture(fixture approveIntegrationFixture, choice string) ApproveInput {
	return ApproveInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog, Choice: choice,
	}
}

func openApproveFixtureGate(t *testing.T, fixture approveIntegrationFixture, input ApproveInput) (GateResult, error) {
	t.Helper()
	return OpenGate(context.Background(), GateInput{
		Identity: input.Identity, ProjectRoot: input.ProjectRoot, RecordRoot: input.RecordRoot,
		Current: input.Current, Catalog: input.Catalog,
	})
}

type approveIntegrationFixture struct {
	projectDir  string
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
	catalog     graph.Snapshot
	stage       graph.Stage
}

func newApproveIntegrationFixture(t *testing.T) approveIntegrationFixture {
	return newApproveIntegrationFixtureWithNextPhaseAndProduces(t, "ideation", "[]")
}

func newApproveIntegrationFixtureWithNextPhase(t *testing.T, nextPhase string) approveIntegrationFixture {
	return newApproveIntegrationFixtureWithNextPhaseAndProduces(t, nextPhase, "[]")
}

func newApproveIntegrationFixtureWithProduces(t *testing.T, produces string) approveIntegrationFixture {
	return newApproveIntegrationFixtureWithNextPhaseAndProduces(t, "ideation", produces)
}

func newApproveIntegrationFixtureWithNextPhaseAndProduces(t *testing.T, nextPhase, produces string) approveIntegrationFixture {
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
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":%s,"consumes":[],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"%s","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]}
]`, produces, nextPhase))},
		"scope-grid.json": {Data: []byte(`{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE","next-stage":"EXECUTE"}}}`)},
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
	if stage.Slug == "" {
		t.Fatal("fixture stage not found")
	}
	return approveIntegrationFixture{
		projectDir:  projectDir,
		identity:    identity,
		projectRoot: projectRoot,
		recordRoot:  recordRoot,
		catalog:     catalog,
		stage:       stage,
	}
}
