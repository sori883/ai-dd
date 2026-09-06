package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/sensor"
)

func TestIntentCaptureGateRequiresAllThreeArtifacts(t *testing.T) {
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement", "stakeholder-map", "intent-capture-questions"}}
	files := fstest.MapFS{
		"ideation/intent-capture/intent-statement.md": &fstest.MapFile{Mode: 0o600, Data: []byte("# Intent\n")},
	}
	present, err := intentCaptureArtifactsPresent(files, stage)
	if err != nil {
		t.Fatalf("intentCaptureArtifactsPresent(one artifact): %v", err)
	}
	if present {
		t.Fatal("intentCaptureArtifactsPresent(one artifact) = true, want all three outputs")
	}
}

func TestIntentCaptureGateCompletionRequiresAllThreeArtifactsAtApproval(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	stage.Produces = []string{"intent-statement", "stakeholder-map", "intent-capture-questions"}
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_STARTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1"}},
		{Event: "HUMAN_TURN", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(3 * time.Second), Shard: "one", Position: 3, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1", "Answer": "yes"}},
		{Event: "SUMMARY_CONFIRMATION_RECORDED", Timestamp: at.Add(4 * time.Second), Shard: "one", Position: 4, Fields: map[string]string{"Stage": stage.Slug, "Answer": "Looks correct", "Details": "Looks correct", "Checkpoint": "Consolidated Summary Confirmation", "Questions File": intentCaptureQuestionsFile, "Questions SHA-256": "questions-v1", "Hash Scope": "confirmed-content-v1"}},
		{Event: "REVIEW_REQUESTED", Timestamp: at.Add(5 * time.Second), Shard: "one", Position: 5, Fields: map[string]string{"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Artifact Path": "intent-statement.md", "Request Fingerprint": "request-v1"}},
		{Event: "REVIEW_COMPLETED", Timestamp: at.Add(6 * time.Second), Shard: "one", Position: 6, Fields: map[string]string{"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Verdict": "READY", "Request Fingerprint": "request-v1", "Artifact Path": "intent-statement.md", "Post Fingerprint": "artifact-v1", "Review Appendix Offset": "10"}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(7 * time.Second), Shard: "one", Position: 7, Fields: map[string]string{"Stage": stage.Slug, "Learning Question": "learning-v1", "Learning Selection": "none", "Details": "none"}},
	}
	decision := evaluateGateCompletion(stage, graph.Snapshot{}, fstest.MapFS{
		"ideation/intent-capture/intent-statement.md": &fstest.MapFile{Mode: 0o600, Data: []byte("# intent\n")},
	}, records)
	if decision.Ready || decision.Blocker != CompletionBlockerArtifact {
		t.Fatalf("evaluateGateCompletion(one artifact) = %#v, want artifact blocker", decision)
	}
}

func TestIntentCaptureGateValidatesReviewRequestCompletionAndCurrentFingerprint(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "REVIEW_REQUESTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Artifact Path": "intent-statement.md", "Request Fingerprint": "request-v1", "Artifact Fingerprint": "artifact-v1",
		}},
		{Event: "REVIEW_COMPLETED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Verdict": "READY", "Request Fingerprint": "request-v1", "Artifact Path": "intent-statement.md", "Post Fingerprint": "",
		}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, true)
	if evidence.ReviewCompleted {
		t.Fatal("review with missing current post fingerprint was accepted")
	}
}

func TestIntentCaptureGateAcceptsCanonicalReviewBinding(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	requestFingerprint := "sha256:" + strings.Repeat("1", 64)
	records := []audit.AuditRecord{
		{Event: "REVIEW_REQUESTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Artifact Fingerprint": requestFingerprint,
			"Review Appendix Artifact": "ideation/intent-capture/intent-statement.md", "Review Appendix Offset": "10",
			"Review Appendix Prior Digest": "none", "Review Appendix Prior Length": "0",
		}},
		{Event: "REVIEW_COMPLETED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Verdict": "READY", "Request Fingerprint": requestFingerprint,
			"Artifact Fingerprint": "sha256:" + strings.Repeat("2", 64), "Review Appendix Artifact": "ideation/intent-capture/intent-statement.md",
			"Review Appendix Offset": "10", "Review Appendix Prior Digest": "none", "Review Appendix Prior Length": "0",
		}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, true)
	if !evidence.ReviewCompleted {
		t.Fatal("canonical review request/completion was not accepted")
	}
}

func TestIntentCaptureGateAcceptsNotReadyReviewAsNonBlocking(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	requestFingerprint := "sha256:" + strings.Repeat("1", 64)
	records := []audit.AuditRecord{
		{Event: "REVIEW_REQUESTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Artifact Fingerprint": requestFingerprint,
			"Review Appendix Artifact": "ideation/intent-capture/intent-statement.md", "Review Appendix Offset": "10",
			"Review Appendix Prior Digest": "none", "Review Appendix Prior Length": "0",
		}},
		{Event: "REVIEW_COMPLETED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{
			"Stage": stage.Slug, "Reviewer": stage.Reviewer, "Iteration": "1", "Verdict": "NOT-READY", "Request Fingerprint": requestFingerprint,
			"Artifact Fingerprint": "sha256:" + strings.Repeat("2", 64), "Review Appendix Artifact": "ideation/intent-capture/intent-statement.md",
			"Review Appendix Offset": "10", "Review Appendix Prior Digest": "none", "Review Appendix Prior Length": "0",
		}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, true)
	if !evidence.ReviewCompleted {
		t.Fatal("canonical NOT-READY review completion blocked the gate")
	}
}

func TestIntentCaptureGateSkipsReviewWhenEffectiveClassNone(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	stage.ReviewClass = graph.ReviewClassNone
	evidence := deriveIntentCaptureGateEvidence(stage, nil, true)
	if !evidence.ReviewCompleted {
		t.Fatal("review class none still required review evidence")
	}
	if intentCaptureReviewRequired(stage) {
		t.Fatal("review class none still enabled current receipt validation")
	}
}

func TestIntentCaptureGateRejectsCrossShardAmbiguity(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "DECISION_RECORDED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1"}},
		{Event: "HUMAN_TURN", Timestamp: at, Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "QUESTION_ANSWERED", Timestamp: at, Shard: "two", Position: 0, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1", "Answer": "yes"}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if evidence.QuestionsAnswered {
		t.Fatal("cross-shard equal-second question pair was accepted")
	}
}

func TestIntentCaptureGateRejectsQuestionAnswerWithoutFreshHumanTurn(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_STARTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1"}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1", "Answer": "yes"}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if evidence.QuestionsAnswered {
		t.Fatal("question answer without a fresh HUMAN_TURN authorized the gate")
	}
}

func TestIntentCaptureGateRetainsStageEpochQuestionAfterRevision(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_STARTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1"}},
		{Event: "HUMAN_TURN", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(3 * time.Second), Shard: "one", Position: 3, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1", "Answer": "yes"}},
		{Event: "GATE_REJECTED", Timestamp: at.Add(4 * time.Second), Shard: "one", Position: 4, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "STAGE_REVISING", Timestamp: at.Add(5 * time.Second), Shard: "one", Position: 5, Fields: map[string]string{"Stage": stage.Slug}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if !evidence.QuestionsAnswered {
		t.Fatal("stage-epoch question answer became stale after rejection/revision")
	}
}

func TestIntentCaptureGateRequiresFreshPairForChangedRevisionQuestion(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_STARTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1"}},
		{Event: "HUMAN_TURN", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(3 * time.Second), Shard: "one", Position: 3, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v1", "Answer": "yes"}},
		{Event: "GATE_REJECTED", Timestamp: at.Add(4 * time.Second), Shard: "one", Position: 4, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "STAGE_REVISING", Timestamp: at.Add(5 * time.Second), Shard: "one", Position: 5, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(6 * time.Second), Shard: "one", Position: 6, Fields: map[string]string{"Stage": stage.Slug, "Decision": "q-1", "Question Fingerprint": "q-v2"}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if evidence.QuestionsAnswered {
		t.Fatal("changed revision question reused the old answer, want fresh pair")
	}
}

func TestIntentCaptureGateRetainsLearningAnswerAfterRevision(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_STARTED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "DECISION_RECORDED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Decision": "learning-q1", "Learning Question": "learning-q1", "Learning Generation": "1"}},
		{Event: "HUMAN_TURN", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "QUESTION_ANSWERED", Timestamp: at.Add(3 * time.Second), Shard: "one", Position: 3, Fields: map[string]string{"Stage": stage.Slug, "Learning Question": "learning-q1", "Learning Generation": "1", "Learning Selection": "none"}},
		{Event: "GATE_REJECTED", Timestamp: at.Add(4 * time.Second), Shard: "one", Position: 4, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "STAGE_REVISING", Timestamp: at.Add(5 * time.Second), Shard: "one", Position: 5, Fields: map[string]string{"Stage": stage.Slug}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if !evidence.LearningsAnswered {
		t.Fatal("learning answer became stale after rejection/revision")
	}
}

func TestIntentCaptureGateAcceptsNoLearningSelection(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "DECISION_RECORDED", Timestamp: at.Add(-2 * time.Second), Shard: "one", Position: 0, Fields: map[string]string{
			"Stage": stage.Slug, "Decision": "learning-q1", "Learning Question": "learning-q1", "Learning Generation": "1",
		}},
		{Event: "HUMAN_TURN", Timestamp: at.Add(-time.Second), Shard: "one", Position: 1, Fields: map[string]string{}},
		{Event: "QUESTION_ANSWERED", Timestamp: at, Shard: "one", Position: 2, Fields: map[string]string{
			"Stage": stage.Slug, "Question": "learning-q1", "Learning Selection": "none", "Details": "none",
			"Learning Question": "learning-q1", "Learning Generation": "1",
		}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, false)
	if !evidence.LearningsAnswered {
		t.Fatal("learning answer with no selection did not satisfy LearningsAnswered")
	}
}

func TestIntentCaptureGateRejectsForgedLearningAnswer(t *testing.T) {
	stage := canonicalIntentCaptureTestStage()
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "QUESTION_ANSWERED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{
			"Stage": stage.Slug, "Learning Question": "forged-learning", "Learning Generation": "1", "Learning Selection": "none", "Details": "none",
		}},
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, true)
	if evidence.LearningsAnswered {
		t.Fatal("learning answer without a matching current-attempt decision authorized the gate")
	}
}

func TestIntentCaptureGateAttemptsAllSensorsAndIgnoresExecutionOrRecordingFailures(t *testing.T) {
	checks := map[string]sensor.CheckFunc{
		sensor.ClaimSourcesID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("claim source failed")
		},
		sensor.RequiredSectionsID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("sections failed")
		},
		sensor.UpstreamCoverageID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("coverage failed")
		},
	}
	invocations := RunIntentCaptureAdvisorySensors(context.Background(), sensor.Input{Stage: "intent-capture"}, checks)
	if len(invocations) != 3 {
		t.Fatalf("RunIntentCaptureAdvisorySensors() returned %d invocations, want 3", len(invocations))
	}
	recorded := 0
	if err := recordAdvisorySensorInvocations(invocations, func(sensor.Invocation) error {
		recorded++
		return errors.New("audit recording failed")
	}); err != nil {
		t.Fatalf("recordAdvisorySensorInvocations() error = %v, want non-blocking nil", err)
	}
	if recorded != 3 {
		t.Fatalf("sensor recorder calls = %d, want all three attempts", recorded)
	}
}

func TestIntentCaptureRevisionRefiresAdvisorySensors(t *testing.T) {
	input := sensor.Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("# Intent\n")}
	first := RunIntentCaptureAdvisorySensors(context.Background(), input, nil)
	second := RunIntentCaptureAdvisorySensors(context.Background(), input, nil)
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("sensor invocation lengths = %d/%d, want 3/3", len(first), len(second))
	}
	for index := range first {
		if first[index].FireResult().FireID() == second[index].FireResult().FireID() {
			t.Fatalf("sensor %d reused Fire ID %q across gate attempts", index, first[index].FireResult().FireID())
		}
	}
}

func TestIntentCaptureSensorsSkipDuplicateGateAttempt(t *testing.T) {
	stage := "intent-capture"
	at := time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "REVIEW_COMPLETED", Timestamp: at, Shard: "one", Position: 0, Fields: map[string]string{"Stage": stage}},
		{Event: "SENSOR_FIRED", Timestamp: at.Add(time.Second), Shard: "one", Position: 1, Fields: map[string]string{"Stage slug": stage, "Sensor ID": sensor.ClaimSourcesID}},
		{Event: "SENSOR_FIRED", Timestamp: at.Add(2 * time.Second), Shard: "one", Position: 2, Fields: map[string]string{"Stage slug": stage, "Sensor ID": sensor.RequiredSectionsID}},
		{Event: "SENSOR_FIRED", Timestamp: at.Add(3 * time.Second), Shard: "one", Position: 3, Fields: map[string]string{"Stage slug": stage, "Sensor ID": sensor.UpstreamCoverageID}},
	}
	if intentCaptureSensorsNeedAttempt(records, stage) {
		t.Fatal("intentCaptureSensorsNeedAttempt() = true after all current review sensor fires")
	}
	records[3].Fields["Stage slug"] = "other-stage"
	if !intentCaptureSensorsNeedAttempt(records, stage) {
		t.Fatal("intentCaptureSensorsNeedAttempt() = false with a missing current-stage fire")
	}
}

func canonicalIntentCaptureTestStage() graph.Stage {
	return graph.Stage{
		Slug: "intent-capture", Phase: "ideation", Execution: "ALWAYS", Mode: "inline", Enabled: true,
		LeadAgent: "aidlc-product-agent", SupportAgents: []string{"aidlc-architect-agent"}, Scopes: []string{"enterprise", "feature", "mvp", "poc"},
		Reviewer: "aidlc-product-lead-agent", ReviewArtifact: "intent-statement", ReviewerMaxIterations: 2,
		ReviewClass: graph.ReviewClassAdvisory, SummaryConfirmation: "required", Sensors: []string{"claim-sources", "required-sections", "upstream-coverage"},
		Produces: []string{"intent-statement", "stakeholder-map", "intent-capture-questions"},
	}
}

func TestIntentCaptureOpenGateEvidenceOrder(t *testing.T) {
	evidence := IntentCaptureGateEvidence{}
	for _, want := range []IntentCaptureGateBlocker{
		IntentCaptureGateQuestions,
		IntentCaptureGateSummary,
		IntentCaptureGateArtifact,
		IntentCaptureGateReview,
		IntentCaptureGateLearnings,
		IntentCaptureGateNone,
	} {
		decision := EvaluateIntentCaptureGate(evidence)
		if decision.Blocker != want {
			t.Fatalf("EvaluateIntentCaptureGate(%#v) blocker = %q, want %q", evidence, decision.Blocker, want)
		}
		switch want {
		case IntentCaptureGateQuestions:
			evidence.QuestionsAnswered = true
		case IntentCaptureGateSummary:
			evidence.SummaryConfirmed = true
		case IntentCaptureGateArtifact:
			evidence.ArtifactPresent = true
		case IntentCaptureGateReview:
			evidence.ReviewCompleted = true
		case IntentCaptureGateLearnings:
			evidence.LearningsAnswered = true
		}
	}
	if !EvaluateIntentCaptureGate(evidence).Ready {
		t.Fatal("complete intent-capture evidence was not ready")
	}
	if err := validateGateCapabilities(canonicalIntentCaptureTestStage()); err != nil {
		t.Fatalf("validateGateCapabilities(canonical intent-capture): %v", err)
	}
}

func TestIntentCaptureCapabilityRequiresFixedGraphMetadata(t *testing.T) {
	canonical := canonicalIntentCaptureTestStage()
	canonical.Number = "1.1"
	canonical.Name = "Intent Capture & Framing"
	canonical.Execution = "ALWAYS"
	canonical.LeadAgent = "aidlc-product-agent"
	canonical.SupportAgents = []string{"aidlc-architect-agent"}
	canonical.Scopes = []string{"enterprise", "feature", "mvp", "poc"}
	canonical.ReviewerMaxIterations = 2
	canonical.Produces = []string{"intent-statement", "stakeholder-map", "intent-capture-questions"}
	canonical.OptionalProduces = nil
	canonical.Consumes = nil
	canonical.RequiresStages = nil
	if !isSupportedIntentCaptureStage(canonical) {
		t.Fatal("canonical fixed intent-capture metadata was not supported")
	}
	for name, mutate := range map[string]func(*graph.Stage){
		"wrong lead":      func(stage *graph.Stage) { stage.LeadAgent = "orchestrator" },
		"wrong support":   func(stage *graph.Stage) { stage.SupportAgents = []string{} },
		"wrong scopes":    func(stage *graph.Stage) { stage.Scopes = []string{"classic"} },
		"wrong produces":  func(stage *graph.Stage) { stage.Produces = []string{"intent-statement"} },
		"wrong max":       func(stage *graph.Stage) { stage.ReviewerMaxIterations = 1 },
		"optional output": func(stage *graph.Stage) { stage.OptionalProduces = []string{"memory"} },
		"consume":         func(stage *graph.Stage) { stage.Consumes = []graph.Consume{{Artifact: "input"}} },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := canonical
			mutate(&candidate)
			if isSupportedIntentCaptureStage(candidate) {
				t.Fatalf("mutated metadata was supported: %#v", candidate)
			}
		})
	}
}

func TestIntentCaptureGateRejectsMissingOrStaleEvidence(t *testing.T) {
	base := IntentCaptureGateEvidence{
		QuestionsAnswered: true, SummaryConfirmed: true, ArtifactPresent: true,
		ReviewCompleted: true, LearningsAnswered: true,
	}
	for name, evidence := range map[string]IntentCaptureGateEvidence{
		"missing question": {SummaryConfirmed: true, ArtifactPresent: true, ReviewCompleted: true, LearningsAnswered: true},
		"stale summary":    {QuestionsAnswered: true, ArtifactPresent: true, ReviewCompleted: true, LearningsAnswered: true, SummaryStale: true},
		"missing artifact": base,
		"stale review":     {QuestionsAnswered: true, SummaryConfirmed: true, ArtifactPresent: true, ReviewCompleted: true, ReviewStale: true, LearningsAnswered: true},
		"missing learning": {QuestionsAnswered: true, SummaryConfirmed: true, ArtifactPresent: true, ReviewCompleted: true},
	} {
		t.Run(name, func(t *testing.T) {
			if name == "missing artifact" {
				evidence.ArtifactPresent = false
			}
			decision := EvaluateIntentCaptureGate(evidence)
			if decision.Ready || decision.Blocker == IntentCaptureGateNone {
				t.Fatalf("decision = %#v, want fail closed", decision)
			}
		})
	}
}

func TestIntentCaptureSummaryUsesLatestReceipt(t *testing.T) {
	valid := map[string]string{
		"Stage":             "intent-capture",
		"Answer":            "Looks correct",
		"Details":           "Looks correct",
		"Checkpoint":        "Consolidated Summary Confirmation",
		"Questions File":    intentCaptureQuestionsFile,
		"Questions SHA-256": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"Hash Scope":        "confirmed-content-v1",
	}
	malformed := map[string]string{
		"Stage":      "intent-capture",
		"Answer":     "Looks correct",
		"Details":    "Looks correct",
		"Checkpoint": "Consolidated Summary Confirmation",
	}
	evidence := deriveIntentCaptureGateEvidence(canonicalIntentCaptureTestStage(), []audit.AuditRecord{
		{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: valid},
		{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: malformed},
	}, true)
	if evidence.SummaryConfirmed {
		t.Fatalf("summary evidence = %#v, want latest malformed receipt to invalidate an older valid receipt", evidence)
	}
}

func TestIntentCaptureRejectReviseRequiresFreshEvidence(t *testing.T) {
	if err := ValidateIntentCaptureRevision(IntentCaptureRevisionEvidence{Rejected: true, Revised: true, FreshEvidence: true}); err != nil {
		t.Fatalf("ValidateIntentCaptureRevision(fresh): %v", err)
	}
	for name, evidence := range map[string]IntentCaptureRevisionEvidence{
		"missing rejection": {Revised: true, FreshEvidence: true},
		"old evidence":      {Rejected: true, Revised: true, FreshEvidence: false},
		"not revised":       {Rejected: true, FreshEvidence: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateIntentCaptureRevision(evidence); !errors.Is(err, ErrIntentCaptureStaleEvidence) {
				t.Fatalf("ValidateIntentCaptureRevision() error = %v, want stale evidence", err)
			}
		})
	}
}

func TestIntentCaptureGateAttemptsAllAdvisorySensorsWithoutBlocking(t *testing.T) {
	checks := map[string]sensor.CheckFunc{
		"claim-sources": func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("claim source unavailable")
		},
		"required-sections": func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("section scanner failed")
		},
		"upstream-coverage": func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, errors.New("upstream unavailable")
		},
	}
	invocations := RunIntentCaptureAdvisorySensors(context.Background(), sensor.Input{Stage: "intent-capture"}, checks)
	if len(invocations) != 3 {
		t.Fatalf("RunIntentCaptureAdvisorySensors() invoked %d sensors, want 3", len(invocations))
	}
	for index, invocation := range invocations {
		if invocation.Error() == nil {
			t.Errorf("invocation %d error = nil, want collected failure", index)
		}
	}
	decision := EvaluateIntentCaptureGate(IntentCaptureGateEvidence{
		QuestionsAnswered: true, SummaryConfirmed: true, ArtifactPresent: true,
		ReviewCompleted: true, LearningsAnswered: true,
	})
	if !decision.Ready {
		t.Fatalf("sensor failures blocked gate: %#v", decision)
	}
}
