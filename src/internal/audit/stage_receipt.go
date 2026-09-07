package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

// RecordStageQuestionDecision derives an ordinary question's fingerprint from
// the current Ideation stage's mandatory questions artifact under the record lock.
// Options are presentation evidence; callers cannot choose paths or fingerprints.
func RecordStageQuestionDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, options string) error {
	if decisionID == "summary" {
		return fmt.Errorf("summary requires its own decision API: %w", ErrIntentCaptureAmbiguous)
	}
	return recordStageQuestionDecision(ctx, identity, projectRoot, recordRoot, stage, decisionID, options, loadStageSummaryContract)
}

// RecordStageQuestionAnswer resolves the current decision from the ledger and
// requires exactly one subsequent human turn before recording its only answer.
func RecordStageQuestionAnswer(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, answer string) error {
	if decisionID == "summary" {
		return fmt.Errorf("summary requires its own confirmation API: %w", ErrIntentCaptureAmbiguous)
	}
	return recordStageQuestionAnswer(ctx, identity, projectRoot, recordRoot, stage, decisionID, answer, loadStageSummaryContract)
}

type stageContractResolver func(*os.Root, string) (stageSummaryContract, error)

// Legacy entry points retain their fixed placement and do not acquire a new
// catalog prerequisite. Generic entry points always load fresh metadata.
func legacyIntentSummaryContract(_ *os.Root, _ string) (stageSummaryContract, error) {
	return stageSummaryContract{questionsFile: intentCaptureQuestionsFile}, nil
}

func recordStageAnswer(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision, answer string) error {
	if err := validateIntentCaptureDecision(decision); err != nil {
		return err
	}
	if answer == "" {
		return fmt.Errorf("record intent-capture answer: empty answer: %w", ErrIntentCaptureAmbiguous)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture answer: read state: %w", err)
		}
		if document.State.CurrentStage() != decision.Stage {
			return fmt.Errorf("record intent-capture answer: current stage changed: %w", ErrIntentCaptureStale)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture answer: read audit: %w", err)
		}
		ordered, err := orderOpenStageReceiptRecords(records, decision.Stage)
		if err != nil {
			return err
		}
		if err := validateIntentCaptureDecisionAnswer(ordered, decision); err != nil {
			return err
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event: "QUESTION_ANSWERED",
			Fields: map[string]string{
				"Stage":                decision.Stage,
				"Decision":             decision.DecisionID,
				"Answer":               answer,
				"Details":              answer,
				"Question Fingerprint": decision.Fingerprint,
			},
		}})
	})
}

// RecordStageSummaryDecision records the canonical summary checkpoint for the
// current stage, deriving its questions path from fresh project metadata.
func RecordStageSummaryDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage string) error {
	return recordStageDecision(ctx, identity, projectRoot, recordRoot, IntentCaptureDecision{Stage: stage, DecisionID: "summary", Fingerprint: "summary"}, loadStageSummaryContract)
}

// RecordStageSummaryConfirmation records an exact Looks correct response after
// a matching decision and fresh human turn. The backend derives the semantic hash.
func RecordStageSummaryConfirmation(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, answer string) error {
	return recordStageSummaryConfirmation(ctx, identity, projectRoot, recordRoot, IntentCaptureDecision{Stage: stage, DecisionID: "summary", Fingerprint: "summary"}, answer, loadStageSummaryContract)
}

// ValidateStageSummaryConfirmationCurrent checks the current epoch's latest
// summary receipt and its decision, human turn, canonical fields, and content.
// It reads the ledger itself so callers cannot provide authority positions.
func ValidateStageSummaryConfirmationCurrent(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage string) error {
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("validate summary state: %w", err)
		}
		if document.State.CurrentStage() != stage {
			return fmt.Errorf("summary stage changed: %w", ErrIntentCaptureStale)
		}
		contract, err := loadStageSummaryContract(projectRoot, stage)
		if err != nil {
			return err
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return err
		}
		ordered, err := orderOpenStageReceiptRecords(records, stage)
		if err != nil {
			return err
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, stage)
		receiptIndex := -1
		for index, record := range ordered {
			if index <= anchor || record.Fields["Stage"] != stage {
				continue
			}
			if record.Event == "DECISION_RECORDED" && record.Fields["Decision"] == "summary" {
				receiptIndex = -1
			}
			if record.Event == "SUMMARY_CONFIRMATION_RECORDED" {
				receiptIndex = index
			}
		}
		if receiptIndex < 0 {
			return fmt.Errorf("current summary receipt is missing: %w", ErrIntentCaptureStale)
		}
		decision := IntentCaptureDecision{Stage: stage, DecisionID: "summary", Fingerprint: "summary"}
		if err := validateStageSummaryDecisionAnswer(ordered[:receiptIndex], decision, contract); err != nil {
			return err
		}
		return validateStageSummaryContent(recordRoot, ordered[receiptIndex], contract)
	})
}

func validateStageSummaryDecisionAnswer(records []AuditRecord, decision IntentCaptureDecision, contract stageSummaryContract) error {
	if err := validateIntentCaptureDecisionAnswer(records, decision); err != nil {
		return err
	}
	ordered, err := orderOpenStageReceiptRecords(records, decision.Stage)
	if err != nil {
		return err
	}
	for index := len(ordered) - 1; index >= 0; index-- {
		record := ordered[index]
		if record.Event != "DECISION_RECORDED" || record.Fields["Stage"] != decision.Stage || record.Fields["Decision"] != "summary" {
			continue
		}
		if record.Fields["Checkpoint"] != "Consolidated Summary Confirmation" || record.Fields["Questions File"] != contract.questionsFile || record.Fields["Options"] != intentCaptureSummaryOptions {
			return fmt.Errorf("summary decision fields changed: %w", ErrIntentCaptureStale)
		}
		return nil
	}
	return fmt.Errorf("summary decision is missing: %w", ErrIntentCaptureStale)
}

func validateStageSummaryContent(recordRoot *os.Root, record AuditRecord, contract stageSummaryContract) error {
	if record.Event != "SUMMARY_CONFIRMATION_RECORDED" || record.Fields["Details"] != "Looks correct" || record.Fields["Checkpoint"] != "Consolidated Summary Confirmation" || record.Fields["Questions File"] != contract.questionsFile {
		return fmt.Errorf("summary confirmation fields are incomplete: %w", ErrIntentCaptureStale)
	}
	questions, _, err := readStageQuestions(recordRoot, contract.questionsFile)
	if err != nil {
		return fmt.Errorf("validate summary confirmation questions: %w", err)
	}
	if record.Fields["Hash Scope"] != intentCaptureHashScope {
		return fmt.Errorf("summary confirmation hash scope is unsupported: %w", ErrIntentCaptureStale)
	}
	digest, err := summaryConfirmationContentHash(questions)
	if err != nil {
		return fmt.Errorf("validate summary confirmation questions: %w", err)
	}
	if digest != record.Fields["Questions SHA-256"] {
		return fmt.Errorf("summary confirmation questions digest changed: %w", ErrIntentCaptureStale)
	}
	return nil
}

func recordStageDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision, resolve stageContractResolver) error {
	if err := validateIntentCaptureDecision(decision); err != nil {
		return err
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: read state: %w", err)
		}
		if document.State.CurrentStage() != decision.Stage {
			return fmt.Errorf("record intent-capture decision: current stage changed: %w", ErrIntentCaptureStale)
		}
		contract, err := resolve(projectRoot, decision.Stage)
		if err != nil {
			return err
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: read audit: %w", err)
		}
		ordered, err := orderOpenStageReceiptRecords(records, decision.Stage)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: order audit: %w", err)
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, decision.Stage)
		currentSummaryDigest := ""
		if decision.DecisionID == "summary" {
			questions, _, readErr := readStageQuestions(recordRoot, contract.questionsFile)
			if readErr != nil {
				return fmt.Errorf("record intent-capture summary decision: read questions: %w", readErr)
			}
			if _, err := summaryConfirmationCanonicalContent(questions, false); err != nil {
				return fmt.Errorf("record intent-capture summary decision: validate questions: %w", err)
			}
			if digest, digestErr := summaryConfirmationContentHash(questions); digestErr == nil {
				currentSummaryDigest = digest
			}
		}
		for index, record := range ordered {
			if index <= anchor {
				continue
			}
			if record.Event == "DECISION_RECORDED" && record.Fields["Stage"] == decision.Stage && record.Fields["Decision"] == decision.DecisionID {
				if decision.DecisionID == "summary" && currentSummaryDigest != "" && !summaryConfirmationMatchesAfter(ordered, index, currentSummaryDigest) {
					continue
				}
				return fmt.Errorf("record intent-capture decision: duplicate decision %q: %w", decision.DecisionID, ErrIntentCaptureStale)
			}
		}
		fields := map[string]string{
			"Stage":    decision.Stage,
			"Decision": decision.DecisionID,
		}
		if decision.Options != "" {
			fields["Options"] = decision.Options
		}
		if decision.DecisionID == "summary" {
			questions, _, readErr := readStageQuestions(recordRoot, contract.questionsFile)
			if readErr != nil {
				return fmt.Errorf("record intent-capture summary decision: read questions: %w", readErr)
			}
			if _, err := summaryConfirmationCanonicalContent(questions, false); err != nil {
				return fmt.Errorf("record intent-capture summary decision: validate questions: %w", err)
			}
			fields["Checkpoint"] = "Consolidated Summary Confirmation"
			fields["Questions File"] = contract.questionsFile
			fields["Options"] = intentCaptureSummaryOptions
		} else {
			fields["Question Fingerprint"] = decision.Fingerprint
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event:  "DECISION_RECORDED",
			Fields: fields,
		}})
	})
}

func recordStageSummaryConfirmation(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision, answer string, resolve stageContractResolver) error {
	if err := validateIntentCaptureDecision(decision); err != nil {
		return err
	}
	if answer != "Looks correct" {
		return fmt.Errorf("summary confirmation answer %q is not exact: %w", answer, ErrIntentCaptureAmbiguous)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record summary confirmation: read state: %w", err)
		}
		if document.State.CurrentStage() != decision.Stage {
			return fmt.Errorf("record summary confirmation: current stage changed: %w", ErrIntentCaptureStale)
		}
		contract, err := resolve(projectRoot, decision.Stage)
		if err != nil {
			return err
		}
		questions, _, err := readStageQuestions(recordRoot, contract.questionsFile)
		if err != nil {
			return fmt.Errorf("record summary confirmation: read questions: %w", err)
		}
		questionsDigest, err := summaryConfirmationContentHash(questions)
		if err != nil {
			return fmt.Errorf("record summary confirmation: validate questions: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record summary confirmation: read audit: %w", err)
		}
		resolved := decision
		resolved.Fingerprint = "summary"
		if err := validateStageSummaryDecisionAnswer(records, resolved, contract); err != nil {
			return err
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event: "SUMMARY_CONFIRMATION_RECORDED",
			Fields: map[string]string{
				"Stage":             decision.Stage,
				"Details":           "Looks correct",
				"Checkpoint":        "Consolidated Summary Confirmation",
				"Questions File":    contract.questionsFile,
				"Questions SHA-256": questionsDigest,
				"Hash Scope":        intentCaptureHashScope,
			},
		}})
	})
}

func loadStageSummaryContract(projectRoot *os.Root, slug string) (stageSummaryContract, error) {
	catalog, err := graph.LoadFromRoot(projectRoot)
	if err != nil {
		return stageSummaryContract{}, fmt.Errorf("load summary catalog: %w", err)
	}
	for _, stage := range catalog.Stages() {
		if stage.Slug == slug {
			return resolveStageSummaryContract(stage, catalog)
		}
	}
	return stageSummaryContract{}, fmt.Errorf("summary stage %q is absent from catalog: %w", slug, ErrIntentCaptureAmbiguous)
}

func recordStageQuestionDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, options string, resolve stageContractResolver) error {
	if !validIntentCaptureToken(stage) || !validIntentCaptureToken(decisionID) {
		return fmt.Errorf("record intent-capture decision: invalid stage or decision id: %w", ErrIntentCaptureAmbiguous)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: read state: %w", err)
		}
		if document.State.CurrentStage() != stage {
			return fmt.Errorf("record intent-capture decision: current stage changed: %w", ErrIntentCaptureStale)
		}
		contract, err := resolve(projectRoot, stage)
		if err != nil {
			return err
		}
		_, fingerprint, err := readStageQuestions(recordRoot, contract.questionsFile)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: read questions: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: read audit: %w", err)
		}
		ordered, err := orderOpenStageReceiptRecords(records, stage)
		if err != nil {
			return fmt.Errorf("record intent-capture decision: order audit: %w", err)
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, stage)
		for index, record := range ordered {
			if index > anchor && record.Event == "DECISION_RECORDED" && record.Fields["Stage"] == stage && record.Fields["Decision"] == decisionID {
				return fmt.Errorf("record intent-capture decision: duplicate decision %q: %w", decisionID, ErrIntentCaptureStale)
			}
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event: "DECISION_RECORDED",
			Fields: map[string]string{
				"Stage": stage, "Decision": decisionID, "Question Fingerprint": fingerprint, "Options": options,
			},
		}})
	})
}

func recordStageQuestionAnswer(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, answer string, resolve stageContractResolver) error {
	if !validIntentCaptureToken(stage) || !validIntentCaptureToken(decisionID) {
		return fmt.Errorf("record intent-capture answer: invalid stage or decision id: %w", ErrIntentCaptureAmbiguous)
	}
	if answer == "" {
		return fmt.Errorf("record intent-capture answer: empty answer: %w", ErrIntentCaptureAmbiguous)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture answer: read state: %w", err)
		}
		if document.State.CurrentStage() != stage {
			return fmt.Errorf("record intent-capture answer: current stage changed: %w", ErrIntentCaptureStale)
		}
		if _, err := resolve(projectRoot, stage); err != nil {
			return err
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record intent-capture answer: read audit: %w", err)
		}
		ordered, err := orderOpenStageReceiptRecords(records, stage)
		if err != nil {
			return fmt.Errorf("record intent-capture answer: order audit: %w", err)
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, stage)
		decisionIndex := -1
		fingerprint := ""
		for index, record := range ordered {
			if index <= anchor || record.Event != "DECISION_RECORDED" || record.Fields["Stage"] != stage || record.Fields["Decision"] != decisionID || record.Fields["Checkpoint"] != "" {
				continue
			}
			decisionIndex = index
			fingerprint = record.Fields["Question Fingerprint"]
		}
		if decisionIndex < 0 || fingerprint == "" {
			return fmt.Errorf("record intent-capture answer: decision %q is missing: %w", decisionID, ErrIntentCaptureStale)
		}
		decision := IntentCaptureDecision{Stage: stage, DecisionID: decisionID, Fingerprint: fingerprint}
		if err := validateIntentCaptureDecisionAnswer(ordered, decision); err != nil {
			return err
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event: "QUESTION_ANSWERED",
			Fields: map[string]string{
				"Stage": stage, "Decision": decisionID, "Answer": answer, "Details": answer, "Question Fingerprint": fingerprint,
			},
		}})
	})
}

type stageQuestionsReadOps struct {
	lstat func(string) (fs.FileInfo, error)
	open  func(*os.Root, string) (*os.File, error)
}

func readStageQuestions(root *os.Root, name string) ([]byte, string, error) {
	if root == nil {
		return nil, "", fmt.Errorf("questions root is nil: %w", ErrInvalidRoot)
	}
	return readStageQuestionsWithOps(root, name, stageQuestionsReadOps{lstat: root.Lstat, open: openAuditLeaf})
}

func readStageQuestionsWithOps(recordRoot *os.Root, name string, ops stageQuestionsReadOps) ([]byte, string, error) {
	if recordRoot == nil {
		return nil, "", fmt.Errorf("questions root is nil: %w", ErrInvalidRoot)
	}
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, `\`) {
		return nil, "", fmt.Errorf("invalid questions path %q: %w", name, ErrIntentCaptureAmbiguous)
	}
	pathInfo, err := ops.lstat(name)
	if err != nil {
		return nil, "", err
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, "", fmt.Errorf("questions file must be a regular non-symlink file: %w", ErrIntentCaptureAmbiguous)
	}
	if err := auditLeafAfterLstat(recordRoot, name); err != nil {
		return nil, "", fmt.Errorf("questions pre-open validation: %w", err)
	}
	file, err := ops.open(recordRoot, name)
	if err != nil {
		return nil, "", err
	}
	if file == nil {
		return nil, "", fmt.Errorf("questions file open returned nil: %w", ErrIntentCaptureAmbiguous)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	if opened == nil || !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return nil, "", fmt.Errorf("questions file changed identity before read: %w", ErrIntentCaptureStale)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxIntentCaptureQuestions+1))
	if err != nil {
		return nil, "", err
	}
	if len(content) > maxIntentCaptureQuestions {
		return nil, "", fmt.Errorf("questions file exceeds %d bytes: %w", maxIntentCaptureQuestions, ErrIntentCaptureAmbiguous)
	}
	if !utf8.Valid(content) {
		return nil, "", fmt.Errorf("questions file is not valid UTF-8: %w", ErrIntentCaptureAmbiguous)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	current, err := ops.lstat(name)
	if err != nil {
		return nil, "", err
	}
	if final == nil || current == nil || !final.Mode().IsRegular() || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(pathInfo, final) || !os.SameFile(pathInfo, current) {
		return nil, "", fmt.Errorf("questions file changed identity during read: %w", ErrIntentCaptureStale)
	}
	digest := sha256.Sum256(content)
	return content, hex.EncodeToString(digest[:]), nil
}

type stageSummaryContract struct{ questionsFile string }

func resolveStageSummaryContract(stage graph.Stage, catalog graph.Snapshot) (stageSummaryContract, error) {
	isInlineIdeation := stage.Phase == "ideation" && stage.Mode == "inline"
	if !isInlineIdeation || stage.ForEach != "" || stage.SummaryConfirmation != "required" {
		return stageSummaryContract{}, fmt.Errorf("unsupported summary stage %q: %w", stage.Slug, ErrIntentCaptureAmbiguous)
	}
	paths, err := artifact.ResolvePaths(stage, catalog, "")
	if err != nil {
		return stageSummaryContract{}, fmt.Errorf("resolve summary paths: %w: %w", ErrIntentCaptureAmbiguous, err)
	}
	count := 0
	questionsFile := ""
	for index, output := range stage.Produces {
		if output == stage.Slug+"-questions" {
			count++
			questionsFile = paths.Produces[index]
		}
	}
	if count != 1 {
		return stageSummaryContract{}, fmt.Errorf("stage %q has %d mandatory questions artifacts: %w", stage.Slug, count, ErrIntentCaptureAmbiguous)
	}
	return stageSummaryContract{questionsFile: questionsFile}, nil
}

// A completion closes receipt authority until a later start opens a new epoch.
// Order first so a tie across shards cannot reopen an ambiguously closed epoch.
func orderOpenStageReceiptRecords(records []AuditRecord, stage string) ([]AuditRecord, error) {
	ordered, err := orderIntentCaptureRecords(records)
	if err != nil {
		return nil, err
	}
	for index := len(ordered) - 1; index >= 0; index-- {
		record := ordered[index]
		if record.Fields["Stage"] != stage {
			continue
		}
		switch record.Event {
		case "STAGE_STARTED":
			return ordered, nil
		case "STAGE_COMPLETED":
			return nil, fmt.Errorf("stage %q receipt epoch is completed: %w", stage, ErrIntentCaptureStale)
		}
	}
	return ordered, nil
}
