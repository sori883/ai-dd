//go:build integration

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestNextIntegrationUsesStartIntentStateWithoutMutation(t *testing.T) {
	project := t.TempDir()
	intentsDir := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(intentsDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intentsDir, "intents.json"), []byte("[]\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	dataFS, scopesFS := startIntentIntegrationFS(t)
	started, err := StartIntent(context.Background(), StartInput{
		Root:               workspace.RootInput{ExplicitDir: project},
		SpaceName:          "team",
		Label:              "Build Auth",
		Scope:              "classic",
		DataFS:             dataFS,
		ScopesFS:           scopesFS,
		ProjectDescription: "Build authentication",
	})
	if err != nil {
		t.Fatalf("StartIntent() error = %v", err)
	}
	catalog, err := graph.Load(dataFS)
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := os.OpenRoot(started.Intent.RecordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(project, "team", started.Intent.DirName)
	if err != nil {
		t.Fatal(err)
	}

	stateBefore, err := os.ReadFile(filepath.Join(started.Intent.RecordDir, "aidlc-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	registryBefore, err := os.ReadFile(filepath.Join(intentsDir, "intents.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Next(context.Background(), NextInput{
		Identity:    identity,
		ProjectRoot: projectRoot,
		RecordRoot:  recordRoot,
		Catalog:     catalog,
	})
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if result.Kind() != DirectiveKindRunStage {
		t.Fatalf("Next().Kind() = %q, want %q", result.Kind(), DirectiveKindRunStage)
	}
	stage, ok := result.Stage()
	if !ok || stage.Slug != "intent-capture" {
		t.Fatalf("Next().Stage() = (%#v, %v), want intent-capture", stage, ok)
	}
	stateAfter, err := os.ReadFile(filepath.Join(started.Intent.RecordDir, "aidlc-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatalf("Next() changed state bytes:\n got %q\nwant %q", stateAfter, stateBefore)
	}
	registryAfter, err := os.ReadFile(filepath.Join(intentsDir, "intents.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(registryAfter) != string(registryBefore) {
		t.Fatalf("Next() changed registry bytes:\n got %q\nwant %q", registryAfter, registryBefore)
	}
	if _, err := projectRoot.Stat("."); err != nil {
		t.Fatalf("project root unusable after Next(): %v", err)
	}
	if _, err := recordRoot.Stat("."); err != nil {
		t.Fatalf("record root unusable after Next(): %v", err)
	}
}

func TestReportIntegrationOpensAndApprovesStartIntentGate(t *testing.T) {
	project := t.TempDir()
	intentsDir := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(intentsDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intentsDir, "intents.json"), []byte("[]\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	dataFS, scopesFS := startIntentIntegrationFS(t)
	started, err := StartIntent(context.Background(), StartInput{
		Root:               workspace.RootInput{ExplicitDir: project},
		SpaceName:          "team",
		Label:              "Build Auth",
		Scope:              "classic",
		DataFS:             dataFS,
		ScopesFS:           scopesFS,
		ProjectDescription: "Build authentication",
	})
	if err != nil {
		t.Fatalf("StartIntent() error = %v", err)
	}
	catalog, err := graph.Load(dataFS)
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := os.OpenRoot(started.Intent.RecordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(project, "team", started.Intent.DirName)
	if err != nil {
		t.Fatal(err)
	}
	stage := lifecycleStage(t, catalog, "intent-capture")

	stateBefore, err := recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if result, err := Report(context.Background(), ReportInput{
		Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot,
		Kind: ReportKindAwaitingApproval, Slug: stage.Slug, Current: stage, Catalog: catalog,
	}); err != nil || !result.Gate.Changed {
		t.Fatalf("Report(awaiting-approval) = (%+v, %v), want changed gate", result, err)
	}
	awaiting, err := Next(context.Background(), NextInput{
		Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot, Catalog: catalog,
	})
	if err != nil || awaiting.Kind() != DirectiveKindAwaitingApproval {
		t.Fatalf("Next() while awaiting = (%+v, %v), want awaiting-approval", awaiting, err)
	}
	if result, err := Report(context.Background(), ReportInput{
		Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot,
		Kind: ReportKindApproved, Slug: stage.Slug, Current: stage, Catalog: catalog, Choice: "Approve",
	}); !errors.Is(err, ErrStaleHumanTurn) || result.Approval.ApprovalSaved {
		t.Fatalf("Report(approved) without human = (%+v, %v), want stale and no save", result, err)
	}
	stateAfterStale, err := recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(stateAfterStale) == string(stateBefore) {
		t.Fatal("state did not record gate opening before stale approval check")
	}

	appendLifecycleHumanTurn(t, identity, projectRoot, recordRoot)
	result, err := Report(context.Background(), ReportInput{
		Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot,
		Kind: ReportKindApproved, Slug: stage.Slug, Current: stage, Catalog: catalog, Choice: "Approve",
	})
	if err != nil {
		t.Fatalf("Report(approved) with human error = %v", err)
	}
	if !result.Approval.ApprovalSaved || !result.Approval.FinalTransitionComplete {
		t.Fatalf("Report(approved) result = %+v, want completed approval", result.Approval)
	}
	if result.Approval.State.WorkflowStatus() != state.WorkflowStatusCompleted {
		t.Fatalf("Report(approved) status = %q, want Completed", result.Approval.State.WorkflowStatus())
	}
}

func TestReportIntegrationRejectsForeignSlugWithoutCurrent(t *testing.T) {
	tests := []struct {
		name string
		kind ReportKind
	}{
		{name: "awaiting approval", kind: ReportKindAwaitingApproval},
		{name: "rejected", kind: ReportKindRejected},
		{name: "revised", kind: ReportKindRevised},
		{name: "approved", kind: ReportKindApproved},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fixture := newLifecycleIntegrationFixture(t)
			stage := lifecycleStage(t, fixture.catalog, "ideation-first")
			prepareForeignReportFixture(t, fixture, stage, test.kind)
			stateBefore, err := fixture.recordRoot.ReadFile("aidlc-state.md")
			if err != nil {
				t.Fatal(err)
			}
			auditBefore := lifecycleAuditSnapshot(t, fixture)

			choice, feedback := "", ""
			if test.kind == ReportKindRejected {
				choice, feedback = "Request Changes", "Please revise this stage"
			}
			if test.kind == ReportKindApproved {
				choice = "Approve"
			}
			input := lifecycleReportInputWithChoice(fixture, stage, test.kind, choice, feedback)
			input.Current = graph.Stage{}
			input.Slug = "foreign-stage"
			result, err := Report(context.Background(), input)
			if !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("Report(%s) = (%+v, %v), want ErrInvalidReport", test.kind, result, err)
			}
			stateAfter, err := fixture.recordRoot.ReadFile("aidlc-state.md")
			if err != nil {
				t.Fatal(err)
			}
			if string(stateAfter) != string(stateBefore) {
				t.Fatalf("Report(%s) changed state for foreign slug", test.kind)
			}
			if auditAfter := lifecycleAuditSnapshot(t, fixture); !reflect.DeepEqual(auditAfter, auditBefore) {
				t.Fatalf("Report(%s) changed audit for foreign slug: before=%#v after=%#v", test.kind, auditBefore, auditAfter)
			}
		})
	}
}

func TestReportIntegrationRejectsCanonicalForeignCurrentWithoutMutation(t *testing.T) {
	tests := []struct {
		name string
		kind ReportKind
	}{
		{name: "awaiting approval", kind: ReportKindAwaitingApproval},
		{name: "rejected", kind: ReportKindRejected},
		{name: "revised", kind: ReportKindRevised},
		{name: "approved", kind: ReportKindApproved},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fixture := newLifecycleIntegrationFixture(t)
			stage := lifecycleStage(t, fixture.catalog, "ideation-first")
			prepareForeignReportFixture(t, fixture, stage, test.kind)
			stateBefore, err := fixture.recordRoot.ReadFile("aidlc-state.md")
			if err != nil {
				t.Fatal(err)
			}
			auditBefore := lifecycleAuditSnapshot(t, fixture)
			foreignStage := lifecycleStage(t, fixture.catalog, "ideation-second")

			choice, feedback := "", ""
			if test.kind == ReportKindRejected {
				choice, feedback = "Request Changes", "Please revise this stage"
			}
			if test.kind == ReportKindApproved {
				choice = "Approve"
			}
			input := lifecycleReportInputWithChoice(fixture, foreignStage, test.kind, choice, feedback)
			result, err := Report(context.Background(), input)
			if !errors.Is(err, ErrInvalidGate) {
				t.Fatalf("Report(%s) = (%+v, %v), want ErrInvalidGate", test.kind, result, err)
			}
			stateAfter, err := fixture.recordRoot.ReadFile("aidlc-state.md")
			if err != nil {
				t.Fatal(err)
			}
			if string(stateAfter) != string(stateBefore) {
				t.Fatalf("Report(%s) changed state for canonical foreign current", test.kind)
			}
			if auditAfter := lifecycleAuditSnapshot(t, fixture); !reflect.DeepEqual(auditAfter, auditBefore) {
				t.Fatalf("Report(%s) changed audit for canonical foreign current: before=%#v after=%#v", test.kind, auditBefore, auditAfter)
			}
		})
	}
}

func TestLifecycleIntegrationRunsStartIntentToTerminalNext(t *testing.T) {
	fixture := newLifecycleIntegrationFixture(t)
	first := lifecycleStage(t, fixture.catalog, "ideation-first")

	stateBefore, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Report(context.Background(), lifecycleReportInput(fixture, first, ReportKindAwaitingApproval)); !errors.Is(err, ErrGateNotReady) {
		t.Fatalf("missing artifact report error = %v, want ErrGateNotReady", err)
	}
	stateAfter, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("missing artifact report changed state")
	}
	if _, err := os.Stat(filepath.Join(fixture.recordDir, "audit")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing artifact report audit directory error = %v, want absent", err)
	}

	artifactPath := filepath.Join(fixture.recordDir, first.Phase, first.Slug, artifact.Filename(first.Produces[0]))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("intent evidence"), 0o600); err != nil {
		t.Fatal(err)
	}

	if result, err := Report(context.Background(), lifecycleReportInput(fixture, first, ReportKindAwaitingApproval)); err != nil || !result.Gate.Changed {
		t.Fatalf("first awaiting report = (%+v, %v), want changed gate", result, err)
	}
	awaiting, err := Next(context.Background(), NextInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: fixture.catalog,
	})
	if err != nil || awaiting.Kind() != DirectiveKindAwaitingApproval {
		t.Fatalf("Next() awaiting = (%+v, %v), want awaiting-approval", awaiting, err)
	}
	staleBefore, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, first, ReportKindApproved, "Approve", ""))
	if !errors.Is(err, ErrStaleHumanTurn) || stale.Approval.ApprovalSaved {
		t.Fatalf("first stale approval = (%+v, %v), want stale without save", stale, err)
	}
	staleAfter, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(staleAfter) != string(staleBefore) {
		t.Fatal("stale approval changed state")
	}

	appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
	approved, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, first, ReportKindApproved, "Approve", ""))
	if err != nil {
		t.Fatalf("first approval error = %v", err)
	}
	if !approved.Approval.ApprovalSaved || !approved.Approval.FinalTransitionComplete || approved.Approval.State.CurrentStage() != "ideation-second" {
		t.Fatalf("first approval result = %+v, want ideation-second transition", approved.Approval)
	}
	oldReportState, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	oldReportRecords := lifecycleReadAuditRecords(t, fixture)
	if _, err := Report(context.Background(), lifecycleReportInput(fixture, first, ReportKindAwaitingApproval)); !errors.Is(err, ErrInvalidGate) {
		t.Fatalf("old-stage report error = %v, want ErrInvalidGate", err)
	}
	oldReportStateAfter, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(oldReportStateAfter) != string(oldReportState) || len(lifecycleReadAuditRecords(t, fixture)) != len(oldReportRecords) {
		t.Fatal("old-stage report changed state or audit")
	}
	if next, err := Next(context.Background(), NextInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: fixture.catalog}); err != nil || next.Kind() != DirectiveKindRunStage {
		t.Fatalf("Next() after first approval = (%+v, %v), want run-stage", next, err)
	} else if stage, ok := next.Stage(); !ok || stage.Slug != "ideation-second" {
		t.Fatalf("Next() after first approval stage = (%#v, %v), want ideation-second", stage, ok)
	}

	second := lifecycleStage(t, fixture.catalog, "ideation-second")
	if result, err := Report(context.Background(), lifecycleReportInput(fixture, second, ReportKindAwaitingApproval)); err != nil || !result.Gate.Changed {
		t.Fatalf("second awaiting report = (%+v, %v), want changed gate", result, err)
	}
	if _, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, second, ReportKindApproved, "Approve", "")); !errors.Is(err, ErrStaleHumanTurn) {
		t.Fatalf("second old receipt approval error = %v, want ErrStaleHumanTurn", err)
	}
	appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
	if result, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, second, ReportKindRejected, "Request Changes", "Please revise this stage")); err != nil || !result.Gate.Changed {
		t.Fatalf("second rejection = (%+v, %v), want changed revision", result, err)
	}
	revising, err := Next(context.Background(), NextInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: fixture.catalog})
	if err != nil || revising.Kind() != DirectiveKindRevising {
		t.Fatalf("Next() revising = (%+v, %v), want revising", revising, err)
	}
	if result, err := Report(context.Background(), lifecycleReportInput(fixture, second, ReportKindRevised)); err != nil || !result.Gate.Changed {
		t.Fatalf("second revised report = (%+v, %v), want changed gate", result, err)
	}
	if _, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, second, ReportKindApproved, "Approve", "")); !errors.Is(err, ErrStaleHumanTurn) {
		t.Fatalf("second post-revise approval error = %v, want ErrStaleHumanTurn", err)
	}
	appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
	secondApproved, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, second, ReportKindApproved, "Approve", ""))
	if err != nil {
		t.Fatalf("second approval error = %v", err)
	}
	if secondApproved.Approval.State.CurrentStage() != "inception-first" || secondApproved.Approval.State.LifecyclePhase() != state.LifecyclePhaseInception {
		t.Fatalf("second approval state = %#v, want inception-first boundary", secondApproved.Approval.State)
	}
	revisionCount, err := state.RevisionCount(secondApproved.Approval.Content)
	if err != nil || revisionCount != 1 {
		t.Fatalf("Revision Count after reject/revise = (%d, %v), want 1", revisionCount, err)
	}

	for _, slug := range []string{"inception-first", "inception-second", "operation-final"} {
		stage := lifecycleStage(t, fixture.catalog, slug)
		if result, err := Report(context.Background(), lifecycleReportInput(fixture, stage, ReportKindAwaitingApproval)); err != nil || !result.Gate.Changed {
			t.Fatalf("%s awaiting report = (%+v, %v), want changed gate", slug, result, err)
		}
		appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
		result, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, stage, ReportKindApproved, "Approve", ""))
		if err != nil {
			t.Fatalf("%s approval error = %v", slug, err)
		}
		if !result.Approval.ApprovalSaved || !result.Approval.FinalTransitionComplete {
			t.Fatalf("%s approval result = %+v, want complete transition", slug, result.Approval)
		}
	}

	terminalDocument, err := state.ReadDocument(fixture.recordRoot)
	if err != nil {
		t.Fatal(err)
	}
	if terminalDocument.State.WorkflowStatus() != state.WorkflowStatusCompleted || terminalDocument.State.CurrentStage() != "operation-final" || terminalDocument.State.NextStage() != "none" || terminalDocument.State.Summary().InProgress != "none" {
		t.Fatalf("terminal state = %#v, want completed with retained current and no next", terminalDocument.State)
	}
	if terminalDocument.State.Summary().Completed != terminalDocument.State.Summary().TotalStages || terminalDocument.State.Summary().Completed != 6 {
		t.Fatalf("terminal summary = %#v, want six completed execute stages", terminalDocument.State.Summary())
	}
	if !strings.Contains(string(terminalDocument.Content), "## Caller Notes\n<!-- preserve lifecycle bytes -->\n") {
		t.Fatal("terminal state lost unknown caller bytes")
	}
	registry, err := os.ReadFile(fixture.registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(registry), `"status": "in-flight"`) {
		t.Fatalf("registry = %s, want unsynchronized in-flight status", registry)
	}
	records := lifecycleReadAuditRecords(t, fixture)
	wantEvents := []string{
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED", "STAGE_STARTED",
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_REJECTED", "STAGE_REVISING",
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED",
		"PHASE_COMPLETED", "PHASE_VERIFIED", "PHASE_STARTED", "STAGE_STARTED",
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED", "STAGE_STARTED",
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED",
		"PHASE_COMPLETED", "PHASE_VERIFIED", "PHASE_STARTED", "STAGE_STARTED",
		"STAGE_AWAITING_APPROVAL", "HUMAN_TURN", "GATE_APPROVED", "STAGE_COMPLETED",
		"PHASE_COMPLETED", "PHASE_VERIFIED", "WORKFLOW_COMPLETED",
	}
	if len(records) != len(wantEvents) {
		t.Fatalf("audit record count = %d, want %d", len(records), len(wantEvents))
	}
	for index, want := range wantEvents {
		if records[index].Event != want {
			t.Errorf("audit[%d].Event = %q, want %q", index, records[index].Event, want)
		}
	}
	if records[7].Fields["Stage"] != "ideation-second" || records[7].Fields["Feedback"] != "Please revise this stage" {
		t.Errorf("rejection audit fields = %#v, want stage and feedback", records[7].Fields)
	}
	if records[8].Fields["Revision count"] != "1" {
		t.Errorf("revision audit fields = %#v, want Revision count 1", records[8].Fields)
	}
	if records[13].Fields["From phase"] != "ideation" || records[13].Fields["To phase"] != "inception" {
		t.Errorf("first phase boundary fields = %#v", records[13].Fields)
	}
	if records[34].Fields["From phase"] != "operation" || records[36].Fields["Details"] == "" {
		t.Errorf("terminal audit fields = %#v / %#v", records[34].Fields, records[36].Fields)
	}
	for _, record := range records {
		if record.Fields["Stage"] == "ideation-skip" {
			t.Fatal("saved SKIP stage appeared in audit")
		}
	}
	for _, progress := range terminalDocument.State.Stages() {
		if progress.Slug == "ideation-skip" && (progress.PlanAction != state.PlanActionSkip || progress.CheckboxState != state.CheckboxStatePending) {
			t.Fatalf("saved SKIP progress = %#v, want pending SKIP", progress)
		}
	}

	terminalCatalog := fixture.catalog
	firstNext, err := Next(context.Background(), NextInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: terminalCatalog})
	if err != nil || firstNext.Kind() != DirectiveKindWorkflowComplete {
		t.Fatalf("terminal Next() = (%+v, %v), want workflow-complete", firstNext, err)
	}
	stateSnapshot, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	registrySnapshot := append([]byte(nil), registry...)
	secondNext, err := Next(context.Background(), NextInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: terminalCatalog})
	if err != nil || secondNext.Kind() != DirectiveKindWorkflowComplete {
		t.Fatalf("repeated terminal Next() = (%+v, %v), want workflow-complete", secondNext, err)
	}
	stateRepeated, err := fixture.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(stateRepeated) != string(stateSnapshot) {
		t.Fatal("repeated terminal Next() changed state bytes")
	}
	registryRepeated, err := os.ReadFile(fixture.registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(registryRepeated) != string(registrySnapshot) {
		t.Fatal("repeated terminal Next() changed registry bytes")
	}

	if err := os.RemoveAll(filepath.Join(fixture.recordDir, "audit")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}
	withoutAudit, err := Next(context.Background(), NextInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot, Catalog: graph.Snapshot{}})
	if err != nil || withoutAudit.Kind() != DirectiveKindWorkflowComplete {
		t.Fatalf("terminal Next() without audit/graph/artifact = (%+v, %v), want workflow-complete", withoutAudit, err)
	}
	if _, err := fixture.projectRoot.Stat("."); err != nil {
		t.Fatalf("project root unusable after terminal Next(): %v", err)
	}
	if _, err := fixture.recordRoot.Stat("."); err != nil {
		t.Fatalf("record root unusable after terminal Next(): %v", err)
	}
	unrelatedRecordAfter, err := os.ReadFile(fixture.unrelatedRecordPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(unrelatedRecordAfter) != string(fixture.unrelatedRecordBytes) {
		t.Fatalf("unrelated record changed: got %q, want %q", unrelatedRecordAfter, fixture.unrelatedRecordBytes)
	}
}

type lifecycleIntegrationFixture struct {
	projectDir           string
	recordDir            string
	registryPath         string
	unrelatedRecordPath  string
	unrelatedRecordBytes []byte
	identity             recordlock.Identity
	projectRoot          *os.Root
	recordRoot           *os.Root
	catalog              graph.Snapshot
}

func newLifecycleIntegrationFixture(t *testing.T) lifecycleIntegrationFixture {
	t.Helper()
	project := t.TempDir()
	intentsDir := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(intentsDir, 0o777); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(intentsDir, "intents.json")
	if err := os.WriteFile(registryPath, []byte("[]\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	dataFS, scopesFS := lifecycleIntegrationFS(t, false)
	started, err := StartIntent(context.Background(), StartInput{
		Root:                      workspace.RootInput{ExplicitDir: project},
		SpaceName:                 "team",
		Label:                     "Lifecycle Walk",
		Scope:                     "classic",
		DataFS:                    dataFS,
		ScopesFS:                  scopesFS,
		ProjectDescription:        "Lifecycle walk",
		ProjectDescriptionPreview: "Lifecycle walk",
	})
	if err != nil {
		t.Fatalf("StartIntent() error = %v", err)
	}
	unrelatedRecordPath := filepath.Join(intentsDir, "unrelated-record", "sentinel.md")
	unrelatedRecordBytes := []byte("unrelated record sentinel\n")
	if err := os.MkdirAll(filepath.Dir(unrelatedRecordPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelatedRecordPath, unrelatedRecordBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	routingDataFS, _ := lifecycleIntegrationFS(t, true)
	catalog, err := graph.Load(routingDataFS)
	if err != nil {
		t.Fatalf("graph.Load() error = %v", err)
	}
	recordDir := started.Intent.RecordDir
	statePath := filepath.Join(recordDir, "aidlc-state.md")
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n## Caller Notes\n<!-- preserve lifecycle bytes -->\n")...)
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	identity, err := recordlock.NewIdentity(project, "team", started.Intent.DirName)
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return lifecycleIntegrationFixture{
		projectDir: project, recordDir: recordDir, registryPath: registryPath,
		unrelatedRecordPath: unrelatedRecordPath, unrelatedRecordBytes: unrelatedRecordBytes,
		identity: identity, projectRoot: projectRoot, recordRoot: recordRoot, catalog: catalog,
	}
}

func lifecycleIntegrationFS(t *testing.T, skipAsExecute bool) (fs.FS, fs.FS) {
	t.Helper()
	skipAction := "SKIP"
	if skipAsExecute {
		skipAction = "EXECUTE"
	}
	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: []byte(`[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"aidlc-orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"ideation-first","number":"1.1","name":"Ideation First","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":["intent-statement"],"consumes":[],"requires_stage":[]},
  {"slug":"ideation-second","number":"1.2","name":"Ideation Second","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"ideation-skip","number":"1.3","name":"Ideation Skip","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"inception-first","number":"2.1","name":"Inception First","phase":"inception","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"inception-second","number":"2.2","name":"Inception Second","phase":"inception","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"operation-final","number":"4.1","name":"Operation Final","phase":"operation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]}
]`)},
		"scope-grid.json": {Data: []byte(fmt.Sprintf(`{"classic":{"stages":{"workspace-scaffold":"EXECUTE","ideation-first":"EXECUTE","ideation-second":"EXECUTE","ideation-skip":"%s","inception-first":"EXECUTE","inception-second":"EXECUTE","operation-final":"EXECUTE"}}}`, skipAction))},
	}
	scopesFS := fstest.MapFS{
		"classic.md": {Data: []byte("---\nname: classic\ndepth: Standard\ntestStrategy: Standard\n---\n")},
	}
	return dataFS, scopesFS
}

func lifecycleReportInput(fixture lifecycleIntegrationFixture, stage graph.Stage, kind ReportKind) ReportInput {
	return lifecycleReportInputWithChoice(fixture, stage, kind, "", "")
}

func lifecycleReportInputWithChoice(fixture lifecycleIntegrationFixture, stage graph.Stage, kind ReportKind, choice, feedback string) ReportInput {
	return ReportInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Kind: kind, Slug: stage.Slug, Current: stage, Catalog: fixture.catalog,
		Choice: choice, Feedback: feedback,
	}
}

func prepareForeignReportFixture(t *testing.T, fixture lifecycleIntegrationFixture, stage graph.Stage, kind ReportKind) {
	t.Helper()
	artifactPath := filepath.Join(fixture.recordDir, stage.Phase, stage.Slug, artifact.Filename(stage.Produces[0]))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("foreign report evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case ReportKindAwaitingApproval:
		return
	case ReportKindRejected:
		appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
	case ReportKindRevised:
		appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
		if _, err := Report(context.Background(), lifecycleReportInputWithChoice(fixture, stage, ReportKindRejected, "Request Changes", "Please revise this stage")); err != nil {
			t.Fatalf("prepare rejected state: %v", err)
		}
	case ReportKindApproved:
		if _, err := Report(context.Background(), lifecycleReportInput(fixture, stage, ReportKindAwaitingApproval)); err != nil {
			t.Fatalf("prepare awaiting state: %v", err)
		}
		appendLifecycleHumanTurn(t, fixture.identity, fixture.projectRoot, fixture.recordRoot)
	default:
		t.Fatalf("unsupported fixture report kind %q", kind)
	}
}

func lifecycleAuditSnapshot(t *testing.T, fixture lifecycleIntegrationFixture) map[string][]byte {
	t.Helper()
	auditDir := filepath.Join(fixture.recordDir, "audit")
	entries, err := os.ReadDir(auditDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("audit snapshot found directory %q", entry.Name())
		}
		content, err := os.ReadFile(filepath.Join(auditDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		snapshot[entry.Name()] = content
	}
	return snapshot
}

func lifecycleStage(t *testing.T, catalog graph.Snapshot, slug string) graph.Stage {
	t.Helper()
	for _, stage := range catalog.Stages() {
		if stage.Slug == slug {
			return stage
		}
	}
	t.Fatalf("stage %q not found", slug)
	return graph.Stage{}
}

func appendLifecycleHumanTurn(t *testing.T, identity recordlock.Identity, projectRoot, recordRoot *os.Root) {
	t.Helper()
	err := recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		return audit.Append(context.Background(), guard, projectRoot, recordRoot, []audit.Event{{
			Event:  "HUMAN_TURN",
			Fields: map[string]string{"Prompt": "lifecycle approval"},
		}})
	})
	if err != nil {
		t.Fatalf("append HUMAN_TURN: %v", err)
	}
}

func lifecycleReadAuditRecords(t *testing.T, fixture lifecycleIntegrationFixture) []audit.AuditRecord {
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
