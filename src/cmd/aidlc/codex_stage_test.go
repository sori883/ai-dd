package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/audit"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/review"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/sensor"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestDefaultCodexStageDispatchRunsPurposeSpecificActions(t *testing.T) {
	for _, action := range []string{"decision", "answer", "review-request", "review-complete", "run-sensors", "learnings-surface", "learnings-persist"} {
		_, err := defaultCodexStageDispatch(context.Background(), action, deliverypkg.RunStageInput{})
		if err == nil || strings.Contains(err.Error(), "not available") {
			t.Errorf("defaultCodexStageDispatch(%q) error = %v, want action-specific dispatch error", action, err)
		}
	}
}

func TestCodexStageDispatchSerializesWorkspaceSelectionSwitches(t *testing.T) {
	for _, target := range []string{"intent", "space"} {
		t.Run(target, func(t *testing.T) {
			fixture := newCodexIntentCaptureRoots(t)
			revisedPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "revised")
			if err := os.MkdirAll(revisedPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(revisedPath, "aidlc-state.md"), []byte("state\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			intentsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "intents.json")
			if err := os.WriteFile(intentsPath, []byte(`[{"uuid":"codex-intent","slug":"codex-intent","status":"planning","dirName":"build"},{"uuid":"revised","slug":"revised","status":"planning","dirName":"revised"}]`), 0o600); err != nil {
				t.Fatal(err)
			}
			if target == "space" {
				otherIntent := filepath.Join(fixture.project, "aidlc", "spaces", "other", "intents", "other")
				if err := os.MkdirAll(otherIntent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(fixture.project, "aidlc", "spaces", "other", "intents", "intents.json"), []byte(`[{"uuid":"other","slug":"other","status":"planning","dirName":"other"}]`), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			previous := codexStageAfterSelectionValidation
			t.Cleanup(func() { codexStageAfterSelectionValidation = previous })
			contention := make(chan string, 1)
			restoreContention := workspace.SetWorkspaceLockContentionObserverForTest(func(path string) {
				select {
				case contention <- path:
				default:
				}
			})
			t.Cleanup(restoreContention)
			selectionValidated := make(chan struct{})
			continueDispatch := make(chan struct{})
			codexStageAfterSelectionValidation = func() {
				close(selectionValidated)
				<-continueDispatch
			}
			input := deliverypkg.RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
			dispatchDone := make(chan error, 1)
			go func() {
				_, err := defaultCodexStageDispatchWithPayload(context.Background(), "learnings-surface", input, []byte(`{}`))
				dispatchDone <- err
			}()
			select {
			case <-selectionValidated:
			case <-time.After(2 * time.Second):
				t.Fatal("hidden stage dispatch did not reach selection validation")
			}

			switchStarted := make(chan struct{})
			switchDone := make(chan error, 1)
			go func() {
				close(switchStarted)
				if target == "intent" {
					_, err := workspace.SwitchIntent(workspace.RootInput{ExplicitDir: fixture.project}, "revised")
					switchDone <- err
					return
				}
				_, err := workspace.SwitchSpace(workspace.RootInput{ExplicitDir: fixture.project}, "other")
				switchDone <- err
			}()
			<-switchStarted
			select {
			case <-contention:
			case err := <-switchDone:
				t.Fatalf("workspace switch completed without contending on hidden action lock: %v", err)
			case <-time.After(2 * time.Second):
				t.Fatal("workspace switch did not reach the hidden action workspace lock")
			}
			select {
			case err := <-switchDone:
				t.Fatalf("workspace switch completed before hidden action released lock: %v", err)
			default:
				runtime.Gosched()
			}
			if active, err := os.ReadFile(filepath.Join(fixture.project, "aidlc", "active-space")); err != nil || string(active) != "team\n" {
				t.Fatalf("active-space while hidden action held lock = (%q, %v), want team", active, err)
			}
			close(continueDispatch)
			select {
			case err := <-dispatchDone:
				if err != nil {
					t.Fatalf("hidden stage dispatch error = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("hidden stage dispatch did not finish after release")
			}
			select {
			case err := <-switchDone:
				if err != nil {
					t.Fatalf("workspace switch error = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("workspace switch did not finish after hidden action")
			}
		})
	}
}

func TestCodexReviewActionsRejectWhenEffectiveReviewIsNone(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	scopePath := filepath.Join(fixture.project, ".codex", "scopes", "feature.md")
	if err := os.MkdirAll(filepath.Dir(scopePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(scope): %v", err)
	}
	if err := os.WriteFile(scopePath, []byte("---\nname: feature\nreview_cap: none\n---\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(scope): %v", err)
	}
	input := deliverypkg.RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	for _, action := range []string{"review-request", "review-complete"} {
		t.Run(action, func(t *testing.T) {
			payload := []byte(`{}`)
			if action == "review-complete" {
				payload = []byte(`{"verdict":"READY"}`)
			}
			_, err := defaultCodexStageDispatchWithPayload(context.Background(), action, input, payload)
			if err == nil || !strings.Contains(err.Error(), "review is disabled") {
				t.Fatalf("%s dispatch error = %v, want effective review disabled", action, err)
			}
		})
	}
}

func TestCodexReviewRequestRequiresCurrentSummaryConfirmation(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	artifactDir := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(artifacts): %v", err)
	}
	for _, name := range []string{"intent-capture-questions", "intent-statement", "stakeholder-map"} {
		if err := os.WriteFile(filepath.Join(artifactDir, artifact.Filename(name)), []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	input := deliverypkg.RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	_, err := defaultCodexStageDispatchWithPayload(context.Background(), "review-request", input, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "summary confirmation") {
		t.Fatalf("review-request without summary error = %v, want current summary prerequisite", err)
	}
}

func TestLearningSurfaceRecordsDecision(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	input := deliverypkg.RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	wire, err := defaultCodexStageDispatchWithPayload(context.Background(), "learnings-surface", input, []byte(`{}`))
	if err != nil {
		t.Fatalf("learnings-surface dispatch: %v", err)
	}
	if !strings.Contains(string(wire), `"kind":"learnings-question"`) {
		t.Fatalf("learnings-surface wire = %s, want learning question", wire)
	}
	var records []audit.AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = audit.ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 1 || records[0].Event != "DECISION_RECORDED" || records[0].Fields["Learning Question"] == "" {
		t.Fatalf("learning surface records = %#v, want one learning DECISION_RECORDED", records)
	}
}

func TestCodexStageRejectsCallerAuthorityFields(t *testing.T) {
	for _, field := range []string{"timestamp", "digest", "fire_id", "receipt", "artifact_snapshot", "reviewer", "iteration"} {
		t.Run(field, func(t *testing.T) {
			_, err := decodeCodexStagePayload("decision", []byte(`{"`+field+`":"caller"}`))
			if err == nil {
				t.Fatalf("decodeCodexStagePayload(%s) error = nil, want caller authority field rejection", field)
			}
		})
	}
}

func TestReviewIterationDerivesRecoveryOrdinalAndRejectsPendingOrSkipped(t *testing.T) {
	stage := graph.Stage{Slug: "intent-capture", Reviewer: "aidlc-product-lead-agent", ReviewerMaxIterations: 2}
	at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	record := func(event string, offset int, fields map[string]string) audit.AuditRecord {
		return audit.AuditRecord{Event: event, Timestamp: at.Add(time.Duration(offset) * time.Second), Shard: "audit/a", Position: offset, Fields: fields}
	}
	firstRequest := map[string]string{"Stage": stage.Slug, "Iteration": "1", "Artifact Fingerprint": "sha256:" + strings.Repeat("1", 64)}
	firstCompletion := map[string]string{"Stage": stage.Slug, "Iteration": "1", "Request Fingerprint": firstRequest["Artifact Fingerprint"]}
	base := []audit.AuditRecord{record("STAGE_STARTED", 0, map[string]string{"Stage": stage.Slug}), record("REVIEW_REQUESTED", 1, firstRequest), record("REVIEW_COMPLETED", 2, firstCompletion)}
	if _, err := deriveNextReviewIteration(stage, base); err == nil {
		t.Fatal("deriveNextReviewIteration(initial complete) error = nil, want duplicate-flow rejection")
	}
	recovery := append(append([]audit.AuditRecord(nil), base...), record("GATE_REJECTED", 3, map[string]string{"Stage": stage.Slug}), record("STAGE_REVISING", 4, map[string]string{"Stage": stage.Slug}))
	if got, err := deriveNextReviewIteration(stage, recovery); err != nil || got != 2 {
		t.Fatalf("deriveNextReviewIteration(recovery) = %d, %v; want iteration 2", got, err)
	}
	pending := append([]audit.AuditRecord(nil), recovery[:1]...)
	pending = append(pending, record("REVIEW_REQUESTED", 1, firstRequest))
	if _, err := deriveNextReviewIteration(stage, pending); err == nil {
		t.Fatal("deriveNextReviewIteration(pending) error = nil, want pending request rejection")
	}
	skipped := append([]audit.AuditRecord(nil), recovery...)
	skipped = append(skipped, record("REVIEW_REQUESTED", 5, map[string]string{"Stage": stage.Slug, "Iteration": "3", "Artifact Fingerprint": "sha256:" + strings.Repeat("2", 64)}))
	if _, err := deriveNextReviewIteration(stage, skipped); err == nil {
		t.Fatal("deriveNextReviewIteration(skipped ordinal) error = nil, want fail closed")
	}
}

func TestReviewCompletionRejectsDuplicatePendingRequests(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	artifactDir := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(artifact directory): %v", err)
	}
	artifacts := map[string][]byte{
		"intent-capture-questions.md": []byte("# Questions\n"),
		"intent-statement.md":         []byte("# Intent\n"),
		"stakeholder-map.md":          []byte("# Stakeholders\n"),
	}
	for name, content := range artifacts {
		if err := os.WriteFile(filepath.Join(artifactDir, name), content, 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Reviewer: "aidlc-product-lead-agent", ReviewArtifact: "intent-statement", ReviewerMaxIterations: 2}
	artifactPath := "ideation/intent-capture/intent-statement.md"
	snapshots := map[string][]byte{
		"ideation/intent-capture/intent-capture-questions.md": artifacts["intent-capture-questions.md"],
		artifactPath: artifacts["intent-statement.md"],
		"ideation/intent-capture/stakeholder-map.md": artifacts["stakeholder-map.md"],
	}
	seed, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: stage.Reviewer, Iteration: 1,
		ArtifactPath: artifactPath, ArtifactSnapshot: snapshots[artifactPath], ArtifactSnapshots: snapshots,
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	requestFields := map[string]string{
		"Stage":                        stage.Slug,
		"Reviewer":                     stage.Reviewer,
		"Iteration":                    "1",
		"Artifact Fingerprint":         seed.ArtifactFingerprint,
		"Review Appendix Artifact":     artifactPath,
		"Review Appendix Offset":       fmt.Sprint(seed.AppendixOffset),
		"Review Appendix Prior Digest": "none",
		"Review Appendix Prior Length": "0",
	}
	record := func(event string, offset int) audit.AuditRecord {
		fields := map[string]string{"Stage": stage.Slug}
		for key, value := range requestFields {
			fields[key] = value
		}
		return audit.AuditRecord{Event: event, Timestamp: at.Add(time.Duration(offset) * time.Second), Shard: "audit/a", Position: offset, Fields: fields}
	}
	records := []audit.AuditRecord{record("STAGE_STARTED", 0), record("REVIEW_REQUESTED", 1), record("REVIEW_REQUESTED", 2)}
	if _, err := latestCodexReviewRequest(fixture.recordRoot, records, stage); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("latestCodexReviewRequest(duplicate pending) error = %v, want pending-history rejection", err)
	}
}

func TestLearningPersistencePayloadUsesSelectionsOnly(t *testing.T) {
	if _, err := decodeCodexStagePayload("learnings-persist", []byte(`{"rule":"forged shortcut"}`)); err == nil {
		t.Fatal("decodeCodexStagePayload(rule) error = nil, want fixed selections-only grammar")
	}
	values, err := decodeCodexStagePayload("learnings-persist", []byte(`{"selections":[]}`))
	if err != nil {
		t.Fatalf("decodeCodexStagePayload(selections): %v", err)
	}
	if _, err := codexLearningSelections(values); err != nil {
		t.Fatalf("codexLearningSelections(empty): %v", err)
	}
	for _, payload := range []string{
		`{"selections":[{"candidate_id":"c1","type":"learning","scope":"project","heading":"Corrections","text":"x","source":"orchestrator","digest":"forged"}]}`,
		`{"selections":[{"candidate_id":"c1","type":"sensor","origin_stage":"intent-capture","source":"orchestrator","manifest_fields":{"id":"s","kind":"deterministic","command":"x","default_severity":"advisory","description":"x","matches":"**/*","fire_id":"forged"}}]}`,
	} {
		values, err := decodeCodexStagePayload("learnings-persist", []byte(payload))
		if err != nil {
			continue
		}
		if _, err := codexLearningSelections(values); err == nil {
			t.Fatalf("codexLearningSelections(%s) error = nil, want authority rejection", payload)
		}
	}
}

func TestLearningPersistenceAcceptsFixedSensorManifestShape(t *testing.T) {
	values, err := decodeCodexStagePayload("learnings-persist", []byte(`{"selections":[{"candidate_id":"c1","type":"sensor","origin_stage":"intent-capture","manifest_fields":{"id":"deterministic-check","kind":"deterministic","command":"check","default_severity":"advisory","description":"a check","matches":"**/*.md","timeout_seconds":30,"category":"document-shape"}}]}`))
	if err != nil {
		t.Fatalf("decodeCodexStagePayload(fixed sensor): %v", err)
	}
	selections, err := codexLearningSelections(values)
	if err != nil {
		t.Fatalf("codexLearningSelections(fixed sensor): %v", err)
	}
	if len(selections) != 1 || selections[0].ManifestFields["timeout_seconds"] != "30" || selections[0].Source != "orchestrator" {
		t.Fatalf("selections = %#v, want normalized timeout and default source", selections)
	}
}

func TestCodexSensorsDeriveAuthoritativeDescriptionScopeAndMemory(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	memoryDir := filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory")
	if err := os.MkdirAll(memoryDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	for name, body := range map[string]string{
		"org.md":     "## Rules\n\n- organization rule\n",
		"team.md":    "## Rules\n\n- keep the source register visible\n",
		"project.md": "## Rules\n\n- project rule\n",
	} {
		if err := os.WriteFile(filepath.Join(memoryDir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	stage := graph.Stage{
		Slug: "intent-capture", Phase: "ideation", ReviewArtifact: "intent-statement",
		Produces: []string{"intent-statement", "stakeholder-map", "intent-capture-questions", "intent-capture-timestamp", "memory"},
		Consumes: nil,
	}
	for name, body := range map[string]string{
		"intent-statement":         "## Initial Scope Signal\n\n- workflow-selected [scope]\n## Assumptions & Open Questions\n\n- None.\n",
		"stakeholder-map":          "## Stakeholders\n\n- team\n## Notes\n\n- none\n",
		"intent-capture-questions": "## Sources\n\n- [desc] Initial description: \"codex journey\"\n- [scope] Workflow-selected scope: `feature`.\n- [memory:rules] `aidlc/spaces/team/memory/team.md#Rules`: \"keep the source register visible\"\n",
		"intent-capture-timestamp": "generated\n",
		"memory":                   "scaffolding\n",
	} {
		artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", artifact.Filename(name))
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s): %v", name, err)
		}
		if err := os.WriteFile(artifactPath, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	input, err := sensor.BuildIntentCaptureInput(fixture.projectRoot, fixture.recordRoot, stage)
	if err != nil {
		t.Fatalf("BuildIntentCaptureInput(): %v", err)
	}
	if input.ProjectDescription != "codex journey" || input.Scope != "feature" {
		t.Fatalf("derived authority = description %q, scope %q", input.ProjectDescription, input.Scope)
	}
	if input.ActiveSpace != "team" {
		t.Fatalf("ActiveSpace = %q, want team", input.ActiveSpace)
	}
	if len(input.MemorySources) == 0 || len(input.MemorySources["aidlc/spaces/team/memory/team.md#Rules"]) != 1 {
		t.Fatalf("MemorySources = %#v, want visible team rule", input.MemorySources)
	}
	if len(input.OutputFiles) != 2 {
		t.Fatalf("OutputFiles = %#v, want only two prose deliverables", input.OutputFiles)
	}
	for _, output := range input.OutputFiles {
		if strings.HasSuffix(output.Path, "-questions.md") || strings.HasSuffix(output.Path, "-timestamp.md") || filepath.Base(output.Path) == "memory.md" {
			t.Fatalf("scaffolding leaked into OutputFiles: %#v", input.OutputFiles)
		}
	}
	if string(input.Questions) == "" || !strings.Contains(string(input.Questions), "## Sources") {
		t.Fatalf("Questions = %q, want separately retained questions file", input.Questions)
	}
}

func TestCodexSensorsExposeRecordedFailureDetailPath(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	stage := graph.Stage{
		Slug: "intent-capture", Phase: "ideation",
		Produces: []string{"intent-statement"},
	}
	wire, err := dispatchCodexSensors(context.Background(), deliverypkg.RunStageInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
	}, stage)
	if err != nil {
		t.Fatalf("dispatchCodexSensors(): %v", err)
	}
	var decoded struct {
		Results []codexSensorResult `json:"results"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(sensor wire): %v", err)
	}
	for _, result := range decoded.Results {
		if result.Status != sensor.StatusFailed {
			continue
		}
		if result.DetailPath == "" {
			t.Fatalf("failed sensor result = %#v, want detail_path", result)
		}
		if _, err := fixture.recordRoot.Stat(result.DetailPath); err != nil {
			t.Fatalf("recordRoot.Stat(%q): %v, want recorded detail path", result.DetailPath, err)
		}
		return
	}
	t.Fatalf("sensor results = %#v, want at least one failed result with detail_path", decoded.Results)
}

func TestCodexSensorsDoNotAdvertiseMissingFailureDetail(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	if err := os.WriteFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", ".aidlc-sensors"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(detail blocker): %v", err)
	}
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}}
	wire, err := dispatchCodexSensors(context.Background(), deliverypkg.RunStageInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
	}, stage)
	if err != nil {
		t.Fatalf("dispatchCodexSensors(): %v", err)
	}
	var decoded struct {
		Results []codexSensorResult `json:"results"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(sensor wire): %v", err)
	}
	for _, result := range decoded.Results {
		if result.Status != sensor.StatusFailed {
			continue
		}
		if result.DetailPath == "" {
			t.Fatalf("failed sensor result = %#v, want a path that exists", result)
		}
		if _, statErr := fixture.recordRoot.Stat(result.DetailPath); statErr != nil {
			t.Fatalf("failed sensor result = %#v, detail path is missing: %v", result, statErr)
		}
	}
}

func TestDefaultCodexStageDispatchPersistsQuestionPair(t *testing.T) {
	fixture := newCodexIntentCaptureRoots(t)
	input := deliverypkg.RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	if err := os.WriteFile(questionsPath, []byte("# Questions\n\n## Q1\n\n[Answer]:\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(questions): %v", err)
	}
	decisionWire, err := defaultCodexStageDispatchWithPayload(context.Background(), "decision", input, []byte(`{"decision":"q1","options":"A. Yes,B. No"}`))
	if err != nil {
		t.Fatalf("decision dispatch: %v", err)
	}
	if string(decisionWire) != `{"kind":"decision","stage":"intent-capture","decision":"q1"}` {
		t.Fatalf("decision wire = %s", decisionWire)
	}
	if err := audit.RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	answerWire, err := defaultCodexStageDispatchWithPayload(context.Background(), "answer", input, []byte(`{"decision":"q1","answer":"A. Yes"}`))
	if err != nil {
		t.Fatalf("answer dispatch: %v", err)
	}
	if string(answerWire) != `{"kind":"answer","stage":"intent-capture","decision":"q1"}` {
		t.Fatalf("answer wire = %s", answerWire)
	}
	var records []audit.AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = audit.ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 || records[0].Event != "DECISION_RECORDED" || records[1].Event != "HUMAN_TURN" || records[2].Event != "QUESTION_ANSWERED" {
		t.Fatalf("records = %#v, want decision/human/answer", records)
	}
	if records[0].Fields["Options"] != "A. Yes,B. No" {
		t.Fatalf("decision options = %q, want rendered options", records[0].Fields["Options"])
	}
}

type codexIntentCaptureRoots struct {
	project     string
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
}

func newCodexIntentCaptureRoots(t *testing.T) codexIntentCaptureRoots {
	t.Helper()
	project := t.TempDir()
	dataDir := filepath.Join(project, ".codex", "tools", "data")
	recordPath := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	for name, content := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"):                               "team\n",
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "active-intent"): "build\n",
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"):  `[{"uuid":"codex-intent","slug":"codex-intent","status":"planning","dirName":"build"}]`,
		filepath.Join(dataDir, "stage-graph.json"):                                    `[{"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["enterprise","feature","mvp","poc"],"enabled":true,"summary_confirmation":"required","reviewer":"aidlc-product-lead-agent","review_artifact":"intent-statement","reviewer_max_iterations":2,"review_class":"advisory","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"requires_stage":[]}]`,
		filepath.Join(dataDir, "scope-grid.json"):                                     `{"feature":{"stages":{"intent-capture":"EXECUTE"}}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", name, err)
		}
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}
	catalog, err := graph.Load(fstest.MapFS{
		"stage-graph.json": &fstest.MapFile{Data: []byte(`[{"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["enterprise","feature","mvp","poc"],"enabled":true,"summary_confirmation":"required","reviewer":"aidlc-product-lead-agent","review_artifact":"intent-statement","reviewer_max_iterations":2,"review_class":"advisory","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"requires_stage":[]}]`)},
		"scope-grid.json":  &fstest.MapFile{Data: []byte(`{"feature":{"stages":{"intent-capture":"EXECUTE"}}}`)},
	})
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph: catalog, Scope: "feature", ScopeMetadata: scope.Metadata{Name: "feature", Depth: "Standard", TestStrategy: "Standard"},
		Workspace: state.WorkspaceInfo{ProjectType: "Brownfield"}, ProjectRoot: project, ProjectDescription: "codex journey", ProjectDescriptionPreview: "codex journey", StartDate: "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte(initial.StateContent), 0o600); err != nil {
		t.Fatalf("WriteFile(state): %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordPath, "project-description.json"), []byte(initial.ProjectDescriptionJSON), 0o600); err != nil {
		t.Fatalf("WriteFile(project description): %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build")))
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(project, "team", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return codexIntentCaptureRoots{project: project, identity: identity, projectRoot: projectRoot, recordRoot: recordRoot}
}

func TestIntentCaptureCommandAdapter(t *testing.T) {
	previousResolver := deliveryInputResolver
	previousCloser := deliveryRootCloser
	t.Cleanup(func() {
		deliveryInputResolver = previousResolver
		deliveryRootCloser = previousCloser
	})
	projectRoot, recordRoot, identity := newCodexStageTestRoots(t)
	resolverCalls := 0
	deliveryInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		resolverCalls++
		return deliverypkg.RunStageInput{Identity: identity, ProjectRoot: projectRoot, RecordRoot: recordRoot}, projectRoot, recordRoot, nil
	}
	closeCalls := 0
	deliveryRootCloser = func(*os.Root) error { closeCalls++; return nil }
	dispatchCalls := 0
	adapter := codexStageAdapter(nil, nil, func(ctx context.Context, action string, input deliverypkg.RunStageInput) ([]byte, error) {
		dispatchCalls++
		if ctx == nil || action != "decision" || input.Identity != identity {
			t.Fatalf("dispatch(ctx, action, identity) = (%v, %q, %v)", ctx, action, input.Identity)
		}
		return []byte(`{"kind":"decision","stage":"intent-capture"}`), nil
	})
	wire, err := adapter("decision", "/tmp/project")
	if err != nil || string(wire) != `{"kind":"decision","stage":"intent-capture"}` {
		t.Fatalf("codexStageAdapter() = (%q, %v), want decision wire", wire, err)
	}
	if resolverCalls != 1 || dispatchCalls != 1 || closeCalls != 2 {
		t.Fatalf("resolver/dispatch/close calls = %d/%d/%d, want 1/1/2", resolverCalls, dispatchCalls, closeCalls)
	}
}

func TestCodexStageAdapterRejectsDispatchOrCleanupFailure(t *testing.T) {
	projectRoot, recordRoot, _ := newCodexStageTestRoots(t)
	previousResolver := deliveryInputResolver
	previousCloser := deliveryRootCloser
	t.Cleanup(func() {
		deliveryInputResolver = previousResolver
		deliveryRootCloser = previousCloser
	})
	deliveryInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliverypkg.RunStageInput{ProjectRoot: projectRoot, RecordRoot: recordRoot}, projectRoot, recordRoot, nil
	}
	cause := errors.New("dispatch failed")
	cleanupCause := errors.New("cleanup failed")
	deliveryRootCloser = func(*os.Root) error { return cleanupCause }
	wire, err := codexStageAdapter(nil, nil, func(context.Context, string, deliverypkg.RunStageInput) ([]byte, error) { return nil, cause })("answer", "")
	if wire != nil || !errors.Is(err, cause) || !errors.Is(err, cleanupCause) {
		t.Fatalf("adapter failure = (%q, %v), want dispatch and cleanup causes", wire, err)
	}
}

func newCodexStageTestRoots(t *testing.T) (*os.Root, *os.Root, recordlock.Identity) {
	t.Helper()
	projectPath := t.TempDir()
	recordPath := t.TempDir()
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(projectPath, "space", "intent")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return projectRoot, recordRoot, identity
}
