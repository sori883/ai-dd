package orchestrator

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"time"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/sensor"
)

type IntentCaptureGateBlocker string

const (
	IntentCaptureGateNone      IntentCaptureGateBlocker = ""
	IntentCaptureGateQuestions IntentCaptureGateBlocker = "questions"
	IntentCaptureGateSummary   IntentCaptureGateBlocker = "summary"
	IntentCaptureGateArtifact  IntentCaptureGateBlocker = "artifact"
	IntentCaptureGateReview    IntentCaptureGateBlocker = "review"
	IntentCaptureGateLearnings IntentCaptureGateBlocker = "learnings"
)

type IntentCaptureGateEvidence struct {
	QuestionsAnswered bool
	SummaryConfirmed  bool
	SummaryStale      bool
	ArtifactPresent   bool
	ReviewCompleted   bool
	ReviewStale       bool
	LearningsAnswered bool
}

type IntentCaptureGateDecision struct {
	Ready   bool
	Blocker IntentCaptureGateBlocker
	Reason  string
}

type IntentCaptureRevisionEvidence struct {
	Rejected      bool
	Revised       bool
	FreshEvidence bool
}

var ErrIntentCaptureStaleEvidence = errors.New("orchestrator: stale intent-capture evidence")

// intentCaptureArtifactsPresent is the intent-capture-specific presence seam.
// Unlike ordinary stages, intent-capture requires every declared deliverable;
// the all-artifact check is completed in the gate read-model slice.
func intentCaptureArtifactsPresent(recordFS fs.FS, stage graph.Stage) (bool, error) {
	if recordFS == nil {
		return false, artifact.ErrInvalidFilesystem
	}
	if len(stage.Produces) == 0 {
		return false, nil
	}
	want := []string{"intent-capture-questions", "intent-statement", "stakeholder-map"}
	for _, name := range want {
		if !slices.Contains(stage.Produces, name) {
			return false, nil
		}
		candidate := path.Join(stage.Phase, stage.Slug, artifact.Filename(name))
		info, err := fs.Stat(recordFS, candidate)
		if err != nil || info == nil || !info.Mode().IsRegular() {
			return false, nil
		}
	}
	return true, nil
}

func EvaluateIntentCaptureGate(evidence IntentCaptureGateEvidence) IntentCaptureGateDecision {
	if !evidence.QuestionsAnswered {
		return intentCaptureGateBlocked(IntentCaptureGateQuestions, "required question/answer evidence is missing")
	}
	if !evidence.SummaryConfirmed || evidence.SummaryStale {
		return intentCaptureGateBlocked(IntentCaptureGateSummary, "consolidated summary evidence is missing or stale")
	}
	if !evidence.ArtifactPresent {
		return intentCaptureGateBlocked(IntentCaptureGateArtifact, "required intent-capture artifact is missing")
	}
	if !evidence.ReviewCompleted || evidence.ReviewStale {
		return intentCaptureGateBlocked(IntentCaptureGateReview, "product-lead review evidence is missing or stale")
	}
	if !evidence.LearningsAnswered {
		return intentCaptureGateBlocked(IntentCaptureGateLearnings, "mandatory learnings question is missing")
	}
	return IntentCaptureGateDecision{Ready: true, Blocker: IntentCaptureGateNone, Reason: "required intent-capture evidence is fresh"}
}

func ValidateIntentCaptureRevision(evidence IntentCaptureRevisionEvidence) error {
	if !evidence.Rejected || !evidence.Revised || !evidence.FreshEvidence {
		return ErrIntentCaptureStaleEvidence
	}
	return nil
}

func RunIntentCaptureAdvisorySensors(ctx context.Context, input sensor.Input, checks map[string]sensor.CheckFunc) []sensor.Invocation {
	return sensor.RunAll(ctx, input, checks)
}

func intentCaptureGateBlocked(blocker IntentCaptureGateBlocker, reason string) IntentCaptureGateDecision {
	return IntentCaptureGateDecision{Blocker: blocker, Reason: reason}
}

func isSupportedIntentCaptureStage(stage graph.Stage) bool {
	return stage.Enabled && stage.Slug == "intent-capture" && stage.Phase == "ideation" && stage.Execution == "ALWAYS" && stage.Mode == "inline" &&
		stage.LeadAgent == "aidlc-product-agent" && slices.Equal(stage.SupportAgents, []string{"aidlc-architect-agent"}) &&
		slices.Equal(stage.Scopes, []string{"enterprise", "feature", "mvp", "poc"}) && stage.Reviewer == "aidlc-product-lead-agent" &&
		stage.ReviewArtifact == "intent-statement" && stage.ReviewerMaxIterations == 2 && stage.ReviewClass == graph.ReviewClassAdvisory &&
		stage.SummaryConfirmation == "required" && slices.Equal(stage.Sensors, []string{"claim-sources", "required-sections", "upstream-coverage"}) &&
		slices.Equal(stage.Produces, []string{"intent-statement", "stakeholder-map", "intent-capture-questions"}) &&
		len(stage.OptionalProduces) == 0 && len(stage.Consumes) == 0 && len(stage.RequiresStages) == 0 && stage.ProducesKinds == nil
}

func evaluateIntentCaptureGateCurrent(stage graph.Stage, recordRoot *os.Root, records []audit.AuditRecord) CompletionDecision {
	if recordRoot == nil {
		return completionBlocked(CompletionBlockerArtifact, "intent-capture record root is unavailable")
	}
	present, err := intentCaptureArtifactsPresentRoot(recordRoot, stage)
	if err != nil {
		return completionBlocked(CompletionBlockerArtifact, err.Error())
	}
	evidence := deriveIntentCaptureGateEvidence(stage, records, present)
	if evidence.SummaryConfirmed {
		if record, ok := latestIntentCaptureSummaryRecord(records, stage.Slug); !ok || !intentCaptureSummaryReceiptShape(record) {
			evidence.SummaryConfirmed = false
			evidence.SummaryStale = true
		} else if err := audit.ValidateSummaryConfirmationCurrent(recordRoot, record); err != nil {
			evidence.SummaryConfirmed = false
			evidence.SummaryStale = true
		}
	}
	if evidence.ReviewCompleted && intentCaptureReviewRequired(stage) {
		if err := audit.ValidateReviewReceiptCurrent(recordRoot, records, stage.Slug, stage.Reviewer); err != nil {
			evidence.ReviewCompleted = false
			evidence.ReviewStale = true
		}
	}
	return evaluateIntentCaptureCompletion(evidence)
}

func intentCaptureReviewRequired(stage graph.Stage) bool {
	return stage.ReviewClass != graph.ReviewClassNone
}

func latestIntentCaptureSummaryRecord(records []audit.AuditRecord, stage string) (audit.AuditRecord, bool) {
	ordered := append([]audit.AuditRecord(nil), records...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Timestamp.Equal(ordered[right].Timestamp) {
			if ordered[left].Shard == ordered[right].Shard {
				return ordered[left].Position < ordered[right].Position
			}
			return ordered[left].Shard < ordered[right].Shard
		}
		return ordered[left].Timestamp.Before(ordered[right].Timestamp)
	})
	anchor := latestIntentCaptureStageEpochAnchor(ordered, stage)
	for index := len(ordered) - 1; index > anchor; index-- {
		record := ordered[index]
		if record.Event == "SUMMARY_CONFIRMATION_RECORDED" && record.Fields["Stage"] == stage {
			return record, true
		}
	}
	return audit.AuditRecord{}, false
}

func intentCaptureSummaryReceiptShape(record audit.AuditRecord) bool {
	return record.Event == "SUMMARY_CONFIRMATION_RECORDED" &&
		record.Fields["Details"] == "Looks correct" &&
		record.Fields["Checkpoint"] == "Consolidated Summary Confirmation" &&
		record.Fields["Questions File"] == intentCaptureQuestionsFile &&
		record.Fields["Questions SHA-256"] != "" &&
		record.Fields["Hash Scope"] == "confirmed-content-v1"
}

func intentCaptureArtifactsPresentRoot(recordRoot *os.Root, stage graph.Stage) (bool, error) {
	if recordRoot == nil {
		return false, artifact.ErrInvalidFilesystem
	}
	want := []string{"intent-capture-questions", "intent-statement", "stakeholder-map"}
	for _, name := range want {
		if !slices.Contains(stage.Produces, name) {
			return false, nil
		}
		candidate := path.Join(stage.Phase, stage.Slug, artifact.Filename(name))
		info, err := recordRoot.Lstat(candidate)
		if err != nil || info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, nil
		}
	}
	return true, nil
}

func deriveIntentCaptureGateEvidence(stage graph.Stage, records []audit.AuditRecord, artifactPresent bool) IntentCaptureGateEvidence {
	ordered := append([]audit.AuditRecord(nil), records...)
	for left := 0; left < len(ordered); left++ {
		for right := left + 1; right < len(ordered); right++ {
			if ordered[left].Timestamp.Equal(ordered[right].Timestamp) && ordered[left].Shard != ordered[right].Shard {
				return IntentCaptureGateEvidence{ArtifactPresent: artifactPresent, SummaryStale: true, ReviewStale: true}
			}
		}
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Timestamp.Equal(ordered[right].Timestamp) {
			if ordered[left].Shard == ordered[right].Shard {
				return ordered[left].Position < ordered[right].Position
			}
			return ordered[left].Shard < ordered[right].Shard
		}
		return ordered[left].Timestamp.Before(ordered[right].Timestamp)
	})
	ordinaryAnchor := latestIntentCaptureStageEpochAnchor(ordered, stage.Slug)
	reviewAnchor := latestIntentCaptureAttemptAnchor(ordered, stage.Slug)
	type questionPair struct {
		decisionIndex int
		humanTurns    int
		answered      bool
		invalid       bool
	}
	questionPairs := make(map[string]*questionPair)
	reviewRequests := make(map[string]audit.AuditRecord)
	type learningPair struct {
		decisionIndex int
		humanTurns    int
		answered      bool
		invalid       bool
	}
	learningPairs := make(map[string]*learningPair)
	evidence := IntentCaptureGateEvidence{
		ArtifactPresent: artifactPresent,
		ReviewCompleted: stage.ReviewClass == graph.ReviewClassNone,
	}
	for index, record := range ordered {
		if record.Event == "REVIEW_REQUESTED" || record.Event == "REVIEW_COMPLETED" {
			if index <= reviewAnchor {
				continue
			}
		} else if index <= ordinaryAnchor || (record.Event != "HUMAN_TURN" && record.Fields["Stage"] != stage.Slug) {
			continue
		}
		switch record.Event {
		case "DECISION_RECORDED":
			if learningID := record.Fields["Learning Question"]; learningID != "" {
				if existing, ok := learningPairs[learningID]; ok {
					existing.invalid = true
				} else {
					learningPairs[learningID] = &learningPair{decisionIndex: index}
				}
				continue
			}
			if decision := record.Fields["Decision"]; decision != "" && record.Fields["Checkpoint"] == "" {
				key := intentCaptureDecisionPairKey(decision, record.Fields["Question Fingerprint"])
				if existing, ok := questionPairs[key]; ok {
					existing.invalid = true
				} else {
					questionPairs[key] = &questionPair{decisionIndex: index}
				}
			}
		case "HUMAN_TURN":
			for _, pair := range questionPairs {
				if !pair.answered && index > pair.decisionIndex {
					pair.humanTurns++
				}
			}
			for _, pair := range learningPairs {
				if !pair.answered && index > pair.decisionIndex {
					pair.humanTurns++
				}
			}
		case "QUESTION_ANSWERED":
			if learningID := record.Fields["Learning Question"]; learningID != "" {
				if pair, ok := learningPairs[learningID]; ok && !pair.invalid && !pair.answered && pair.humanTurns == 1 && record.Fields["Learning Generation"] != "" {
					decisionGeneration := ""
					for _, decisionRecord := range ordered[pair.decisionIndex : pair.decisionIndex+1] {
						decisionGeneration = decisionRecord.Fields["Learning Generation"]
					}
					if record.Fields["Learning Generation"] == decisionGeneration {
						pair.answered = true
					}
				}
				continue
			}
			if decision := record.Fields["Decision"]; decision != "" {
				key := intentCaptureDecisionPairKey(decision, record.Fields["Question Fingerprint"])
				if pair, ok := questionPairs[key]; ok {
					if pair.answered || pair.humanTurns != 1 {
						pair.invalid = true
					} else {
						pair.answered = true
					}
				}
			}
		case "SUMMARY_CONFIRMATION_RECORDED":
			evidence.SummaryConfirmed = record.Fields["Details"] == "Looks correct" && record.Fields["Checkpoint"] == "Consolidated Summary Confirmation" && record.Fields["Questions File"] == intentCaptureQuestionsFile && record.Fields["Questions SHA-256"] != "" && record.Fields["Hash Scope"] == "confirmed-content-v1"
			evidence.SummaryStale = !evidence.SummaryConfirmed
		case "REVIEW_REQUESTED":
			if record.Fields["Artifact Fingerprint"] != "" && record.Fields["Review Appendix Artifact"] != "" &&
				record.Fields["Review Appendix Offset"] != "" && record.Fields["Review Appendix Prior Digest"] != "" && record.Fields["Review Appendix Prior Length"] != "" {
				reviewRequests[record.Fields["Artifact Fingerprint"]] = record
			}
		case "REVIEW_COMPLETED":
			request, ok := reviewRequests[record.Fields["Request Fingerprint"]]
			verdict := record.Fields["Verdict"]
			validVerdict := verdict == "READY" || verdict == "NOT-READY"
			if ok && record.Fields["Reviewer"] == stage.Reviewer && record.Fields["Reviewer"] == request.Fields["Reviewer"] && record.Fields["Iteration"] == request.Fields["Iteration"] && validVerdict && record.Fields["Review Appendix Artifact"] == request.Fields["Review Appendix Artifact"] && record.Fields["Review Appendix Offset"] == request.Fields["Review Appendix Offset"] && record.Fields["Review Appendix Prior Digest"] == request.Fields["Review Appendix Prior Digest"] && record.Fields["Review Appendix Prior Length"] == request.Fields["Review Appendix Prior Length"] && record.Fields["Review Challenge"] == request.Fields["Review Challenge"] && record.Fields["Artifact Fingerprint"] != "" {
				evidence.ReviewCompleted = true
			}
		}
	}
	evidence.QuestionsAnswered = len(questionPairs) > 0
	for _, pair := range questionPairs {
		if pair.invalid || !pair.answered {
			evidence.QuestionsAnswered = false
			break
		}
	}
	for _, pair := range learningPairs {
		if pair.answered && !pair.invalid {
			evidence.LearningsAnswered = true
			break
		}
	}
	return evidence
}

func intentCaptureDecisionPairKey(decision, fingerprint string) string {
	return decision + "\x00" + fingerprint
}

const intentCaptureQuestionsFile = "ideation/intent-capture/intent-capture-questions.md"

func evaluateGateCompletion(stage graph.Stage, catalog graph.Snapshot, recordFS fs.FS, records []audit.AuditRecord) CompletionDecision {
	if isSupportedIntentCaptureStage(stage) {
		present, err := intentCaptureArtifactsPresent(recordFS, stage)
		if err != nil {
			return completionBlocked(CompletionBlockerArtifact, err.Error())
		}
		return evaluateIntentCaptureCompletion(deriveIntentCaptureGateEvidence(stage, records, present))
	}
	return EvaluateStageCompletion(CompletionInput{Current: stage, Catalog: catalog, RecordFS: recordFS})
}

func sensorInputForIntentCapture(projectRoot, recordRoot *os.Root, stage graph.Stage) sensor.Input {
	input, err := sensor.BuildIntentCaptureInput(projectRoot, recordRoot, stage)
	if err != nil {
		return sensor.Input{
			Stage:        stage.Slug,
			ArtifactPath: path.Join(stage.Phase, stage.Slug, artifact.Filename(stage.ReviewArtifact)),
			AuthorityFindings: []sensor.Finding{{
				Path:    stage.Slug,
				Message: err.Error(),
			}},
		}
	}
	return input
}

func recordIntentCaptureAdvisorySensors(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, input sensor.Input) error {
	invocations := RunIntentCaptureAdvisorySensors(ctx, input, nil)
	return recordAdvisorySensorInvocations(invocations, func(invocation sensor.Invocation) error {
		return audit.RecordSensorInvocationWithGuard(ctx, identity, guard, projectRoot, recordRoot, invocation)
	})
}

func intentCaptureSensorsNeedAttempt(records []audit.AuditRecord, stage string) bool {
	ordered := append([]audit.AuditRecord(nil), records...)
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].Timestamp.Equal(ordered[right].Timestamp) {
			if ordered[left].Shard == ordered[right].Shard {
				return ordered[left].Position < ordered[right].Position
			}
			return ordered[left].Shard < ordered[right].Shard
		}
		return ordered[left].Timestamp.Before(ordered[right].Timestamp)
	})
	floor := -1
	for index, record := range ordered {
		if record.Fields["Stage"] != stage {
			continue
		}
		switch record.Event {
		case "STAGE_STARTED", "STAGE_REVISING", "GATE_REJECTED", "REVIEW_COMPLETED":
			floor = index
		}
	}
	want := map[string]bool{sensor.ClaimSourcesID: false, sensor.RequiredSectionsID: false, sensor.UpstreamCoverageID: false}
	for index := floor + 1; index < len(ordered); index++ {
		record := ordered[index]
		if record.Event != "SENSOR_FIRED" || record.Fields["Stage slug"] != stage {
			continue
		}
		id := record.Fields["Sensor ID"]
		if id == "" {
			id = record.Fields["Sensor"]
		}
		if _, ok := want[id]; ok {
			want[id] = true
		}
	}
	for _, attempted := range want {
		if !attempted {
			return true
		}
	}
	return false
}

// recordAdvisorySensorInvocations deliberately discards recorder errors: the
// three sensor attempts and any observations are advisory, so execution or
// ledger-recording failure cannot block an otherwise valid gate.
func recordAdvisorySensorInvocations(invocations []sensor.Invocation, record func(sensor.Invocation) error) error {
	if record == nil {
		return nil
	}
	for _, invocation := range invocations {
		// The observation remains useful when recording fails, but neither the
		// execution error nor this persistence error is gate authority.
		_ = record(invocation)
	}
	return nil
}

func latestIntentCaptureAttemptAnchor(records []audit.AuditRecord, stage string) int {
	anchor := -1
	for index, record := range records {
		if record.Fields["Stage"] != stage {
			continue
		}
		switch record.Event {
		case "STAGE_STARTED", "STAGE_REVISING", "GATE_REJECTED":
			anchor = index
		}
	}
	return anchor
}

func latestIntentCaptureStageEpochAnchor(records []audit.AuditRecord, stage string) int {
	anchor := -1
	for index, record := range records {
		if record.Fields["Stage"] == stage && record.Event == "STAGE_STARTED" {
			anchor = index
		}
	}
	return anchor
}

func intentCaptureRecordAfter(record audit.AuditRecord, anchor time.Time) bool {
	return anchor.IsZero() || record.Timestamp.After(anchor)
}
