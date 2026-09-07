package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/learnings"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/review"
	"github.com/sori883/ai-dd/src/internal/sensor"
	"github.com/sori883/ai-dd/src/internal/state"
)

// RecordReviewRequested records the immutable request-time review snapshot.
// The request is checked while the identity-bound record lock is held so a
// later gate can distinguish a fresh receipt from a stale or duplicated one.
func RecordReviewRequested(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, request review.Request) error {
	if err := review.ValidateRequest(request); err != nil {
		return err
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record review request: read state: %w", err)
		}
		if document.State.CurrentStage() != request.Stage {
			return fmt.Errorf("record review request: current stage changed: %w", ErrIntentCaptureStale)
		}
		artifact, err := readReviewArtifact(recordRoot, request.ArtifactPath)
		if err != nil {
			return fmt.Errorf("record review request: read artifact %q: %w", request.ArtifactPath, err)
		}
		if !bytes.Equal(artifact, request.ArtifactSnapshot) {
			return fmt.Errorf("record review request: artifact changed: %w", review.ErrArtifactChanged)
		}
		if err := validateReviewArtifactSet(recordRoot, request.ArtifactSnapshots); err != nil {
			return fmt.Errorf("record review request: declared artifact changed: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record review request: read audit: %w", err)
		}
		reviewAttemptAnchor := -1
		if request.Stage == "intent-capture" {
			for index, record := range records {
				if record.Fields["Stage"] != request.Stage {
					continue
				}
				switch record.Event {
				case "STAGE_STARTED", "GATE_REJECTED", "STAGE_REVISING":
					reviewAttemptAnchor = index
				}
			}
		}
		for index, record := range records {
			if index <= reviewAttemptAnchor {
				continue
			}
			if record.Event == "REVIEW_REQUESTED" && record.Fields["Artifact Fingerprint"] == request.ArtifactFingerprint &&
				record.Fields["Review Appendix Artifact"] == request.ArtifactPath &&
				record.Fields["Review Appendix Offset"] == fmt.Sprint(request.AppendixOffset) {
				return fmt.Errorf("record review request: duplicate request: %w", ErrIntentCaptureStale)
			}
		}
		fields := map[string]string{
			"Stage":                        request.Stage,
			"Reviewer":                     request.Reviewer,
			"Iteration":                    fmt.Sprint(request.Iteration),
			"Artifact Fingerprint":         request.ArtifactFingerprint,
			"Review Appendix Artifact":     request.ArtifactPath,
			"Review Appendix Offset":       fmt.Sprint(request.AppendixOffset),
			"Review Appendix Prior Digest": request.PriorDigest,
			"Review Appendix Prior Length": fmt.Sprint(request.PriorLength),
		}
		if request.ReviewChallenge != "" {
			fields["Review Challenge"] = request.ReviewChallenge
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event:  "REVIEW_REQUESTED",
			Fields: fields,
		}})
	})
}

// RecordReviewCompleted validates and records the terminal appendix emitted
// for one previously recorded request. NOT-READY is a valid review verdict;
// gate policy decides whether another iteration is allowed.
func RecordReviewCompleted(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, request review.Request, completion review.Completion) error {
	if err := review.ValidateCompletion(request, completion); err != nil {
		return err
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record review completion: read state: %w", err)
		}
		if document.State.CurrentStage() != request.Stage {
			return fmt.Errorf("record review completion: current stage changed: %w", ErrIntentCaptureStale)
		}
		artifact, err := readReviewArtifact(recordRoot, request.ArtifactPath)
		if err != nil {
			return fmt.Errorf("record review completion: read artifact %q: %w", request.ArtifactPath, err)
		}
		if !bytes.Equal(artifact, completion.ArtifactAfter) {
			return fmt.Errorf("record review completion: artifact changed after validation: %w", review.ErrArtifactChanged)
		}
		currentArtifacts := cloneAuditArtifactSnapshots(request.ArtifactSnapshots)
		if len(currentArtifacts) == 0 {
			currentArtifacts = map[string][]byte{request.ArtifactPath: append([]byte(nil), request.ArtifactSnapshot...)}
		}
		currentArtifacts[request.ArtifactPath] = append([]byte(nil), completion.ArtifactAfter...)
		if err := validateReviewArtifactSet(recordRoot, currentArtifacts); err != nil {
			return fmt.Errorf("record review completion: declared artifact changed: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record review completion: read audit: %w", err)
		}
		if err := validateReviewRequestReceipt(records, request); err != nil {
			return err
		}
		fields := map[string]string{
			"Stage":                        request.Stage,
			"Reviewer":                     request.Reviewer,
			"Iteration":                    fmt.Sprint(request.Iteration),
			"Verdict":                      completion.Verdict,
			"Request Fingerprint":          request.ArtifactFingerprint,
			"Artifact Fingerprint":         completion.PostFingerprint,
			"Review Appendix Artifact":     request.ArtifactPath,
			"Review Appendix Offset":       fmt.Sprint(completion.AppendixOffset),
			"Review Appendix Prior Digest": request.PriorDigest,
			"Review Appendix Prior Length": fmt.Sprint(request.PriorLength),
		}
		if request.ReviewChallenge != "" {
			fields["Review Challenge"] = request.ReviewChallenge
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event:  "REVIEW_COMPLETED",
			Fields: fields,
		}})
	})
}

func validateReviewRequestReceipt(records []AuditRecord, request review.Request) error {
	requested := false
	completed := false
	for _, record := range records {
		if record.Event != "REVIEW_REQUESTED" && record.Event != "REVIEW_COMPLETED" {
			continue
		}
		if record.Fields["Review Appendix Artifact"] != request.ArtifactPath ||
			record.Fields["Review Appendix Offset"] != fmt.Sprint(request.AppendixOffset) ||
			record.Fields["Review Appendix Prior Digest"] != request.PriorDigest ||
			record.Fields["Review Appendix Prior Length"] != fmt.Sprint(request.PriorLength) ||
			record.Fields["Stage"] != request.Stage || record.Fields["Reviewer"] != request.Reviewer || record.Fields["Iteration"] != fmt.Sprint(request.Iteration) {
			continue
		}
		if request.ReviewChallenge != "" && record.Fields["Review Challenge"] != request.ReviewChallenge {
			continue
		}
		switch record.Event {
		case "REVIEW_REQUESTED":
			if record.Fields["Artifact Fingerprint"] == request.ArtifactFingerprint && record.Fields["Request Fingerprint"] == "" {
				requested = true
			}
		case "REVIEW_COMPLETED":
			if record.Fields["Request Fingerprint"] == request.ArtifactFingerprint && record.Fields["Artifact Fingerprint"] != "" {
				completed = true
			}
		}
	}
	if !requested {
		return fmt.Errorf("record review completion: request receipt is missing: %w", ErrIntentCaptureStale)
	}
	if completed {
		return fmt.Errorf("record review completion: request is already complete: %w", ErrIntentCaptureStale)
	}
	return nil
}

const maxReviewArtifactBytes = 8 << 20

// auditLeafAfterLstat is a package-private replacement seam used by the Unix
// FIFO regression tests. Production leaves it as a no-op; the readers still
// prove the opened descriptor and path identity after this boundary.
var auditLeafAfterLstat = func(*os.Root, string) error { return nil }

func setAuditLeafAfterLstat(fn func(*os.Root, string) error) func() {
	previous := auditLeafAfterLstat
	auditLeafAfterLstat = fn
	return func() { auditLeafAfterLstat = previous }
}

func validateReviewArtifactSet(recordRoot *os.Root, expected map[string][]byte) error {
	if len(expected) == 0 {
		return nil
	}
	for name, want := range expected {
		got, err := readReviewArtifact(recordRoot, name)
		if err != nil {
			return fmt.Errorf("read declared artifact %q: %w", name, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("declared artifact %q changed: %w", name, review.ErrArtifactChanged)
		}
	}
	return nil
}

func cloneAuditArtifactSnapshots(snapshots map[string][]byte) map[string][]byte {
	if len(snapshots) == 0 {
		return nil
	}
	clone := make(map[string][]byte, len(snapshots))
	for name, snapshot := range snapshots {
		clone[name] = append([]byte(nil), snapshot...)
	}
	return clone
}

func encodeArtifactSet(snapshots map[string][]byte) string {
	if len(snapshots) <= 1 {
		return ""
	}
	names := make([]string, 0, len(snapshots))
	for name := range snapshots {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func decodeArtifactSet(fields map[string]string) []string {
	if value := fields["Artifact Set"]; value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			if part != "" {
				result = append(result, part)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if value := fields["Artifact Path"]; value != "" {
		return []string{value}
	}
	return nil
}

func readReviewArtifact(recordRoot *os.Root, name string) ([]byte, error) {
	if recordRoot == nil || name == "" || !fs.ValidPath(name) || path.IsAbs(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("review artifact path is unsafe: %w", fs.ErrInvalid)
	}
	pathInfo, err := recordRoot.Lstat(name)
	if err != nil {
		return nil, err
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("review artifact is not a regular file: %w", fs.ErrInvalid)
	}
	if err := auditLeafAfterLstat(recordRoot, name); err != nil {
		return nil, fmt.Errorf("review artifact pre-open validation: %w", err)
	}
	file, err := openAuditLeaf(recordRoot, name)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("review artifact open returned nil file: %w", fs.ErrInvalid)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if opened == nil || !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return nil, fmt.Errorf("review artifact changed identity: %w", fs.ErrInvalid)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxReviewArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxReviewArtifactBytes {
		return nil, fmt.Errorf("review artifact exceeds %d bytes: %w", maxReviewArtifactBytes, fs.ErrInvalid)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current, err := recordRoot.Lstat(name)
	if err != nil {
		return nil, err
	}
	if final == nil || current == nil || !final.Mode().IsRegular() || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(pathInfo, final) || !os.SameFile(pathInfo, current) {
		return nil, fmt.Errorf("review artifact changed identity: %w", fs.ErrInvalid)
	}
	return content, nil
}

// RecordSensorInvocation records advisory sensor observations without
// allowing them to authorize or block a workflow gate. A missing terminal is
// intentionally represented by only the FIRED event.
func RecordSensorInvocation(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, invocation sensor.Invocation) error {
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		return RecordSensorInvocationWithGuard(ctx, identity, guard, projectRoot, recordRoot, invocation)
	})
}

// RecordSensorInvocationWithGuard is the non-reentrant form for an existing
// record transaction, such as the gate path. The caller keeps guard held for
// the complete selection and append transaction.
func RecordSensorInvocationWithGuard(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, invocation sensor.Invocation) error {
	fired := invocation.FireResult()
	if err := validateSensorFire(fired); err != nil {
		return err
	}
	terminal := invocation.TerminalResult()
	if invocation.HasTerminal() && (terminal.Stage != fired.Stage || terminal.Sensor != fired.Sensor || terminal.FireID() != fired.FireID() || !terminal.Terminal || terminal.Status == "") {
		return fmt.Errorf("record sensor invocation: terminal identity is invalid: %w", ErrIntentCaptureAmbiguous)
	}
	document, err := state.ReadDocument(recordRoot)
	if err != nil {
		return fmt.Errorf("record sensor invocation: read state: %w", err)
	}
	if fired.Stage == "" || document.State.CurrentStage() != fired.Stage {
		return fmt.Errorf("record sensor invocation: current stage changed: %w", ErrIntentCaptureStale)
	}
	records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
	if err != nil {
		return fmt.Errorf("record sensor invocation: read audit: %w", err)
	}
	for _, record := range records {
		if record.Event == "SENSOR_FIRED" && (record.Fields["Fire id"] == fired.FireID() || record.Fields["Fire ID"] == fired.FireID()) {
			return fmt.Errorf("record sensor invocation: duplicate fire id: %w", ErrIntentCaptureStale)
		}
	}
	outputPath := fired.OutputPath
	if outputPath == "" {
		outputPath = terminal.OutputPath
	}
	events := []Event{{
		Event: "SENSOR_FIRED",
		Fields: map[string]string{
			"Fire id":     fired.FireID(),
			"Sensor ID":   fired.Sensor,
			"Stage slug":  document.State.CurrentStage(),
			"Output path": outputPath,
		},
	}}
	if invocation.HasTerminal() {
		findingsCount := terminal.FindingsCount
		if findingsCount == 0 && len(terminal.Findings) != 0 {
			findingsCount = len(terminal.Findings)
		}
		terminalFields := map[string]string{
			"Fire id":     terminal.FireID(),
			"Sensor ID":   terminal.Sensor,
			"Stage slug":  document.State.CurrentStage(),
			"Output path": outputPath,
		}
		terminalEvent := sensorTerminalEvent(terminal.Status)
		switch terminalEvent {
		case "SENSOR_PASSED":
			terminalFields["Duration ms"] = strconv.FormatInt(terminal.DurationMS, 10)
			if terminal.Note != "" {
				terminalFields["Note"] = terminal.Note
			}
		case "SENSOR_FAILED":
			terminalFields["Findings count"] = strconv.Itoa(findingsCount)
			if detailPath, detailErr := writeSensorDetail(recordRoot, terminal, document.State.CurrentStage()); detailErr == nil {
				terminalFields["Detail path"] = detailPath
			} else {
				// A failed advisory script whose diagnostic cannot be safely
				// persisted is normalized to the fixed pass terminal with an
				// observable script-error note. It remains non-authoritative.
				terminalEvent = "SENSOR_PASSED"
				delete(terminalFields, "Findings count")
				terminalFields["Duration ms"] = strconv.FormatInt(terminal.DurationMS, 10)
				terminalFields["Note"] = "script-error: detail-write-failed: " + detailErr.Error()
			}
		case "SENSOR_BUDGET_OVERRIDE":
			terminalFields["Cap layer"] = "registry"
			terminalFields["Cap value"] = "0"
			terminalFields["Observed value"] = strconv.FormatInt(terminal.DurationMS, 10)
		}
		events = append(events, Event{Event: terminalEvent, Fields: terminalFields})
	}
	return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, events)
}

func writeSensorDetail(recordRoot *os.Root, terminal sensor.Result, stage string) (string, error) {
	if recordRoot == nil || !validSensorDetailComponent(stage) || !validSensorDetailComponent(terminal.Sensor) {
		return "", fmt.Errorf("sensor detail path is invalid: %w", ErrIntentCaptureAmbiguous)
	}
	relativeDir := path.Join(".aidlc-sensors", stage)
	if err := recordRoot.MkdirAll(relativeDir, 0o700); err != nil {
		return "", err
	}
	relative := path.Join(relativeDir, terminal.Sensor+"-"+terminal.FireID()+".md")
	if _, err := recordRoot.Lstat(relative); err == nil {
		return "", fmt.Errorf("sensor detail %q already exists: %w", relative, fs.ErrExist)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	body := "# Advisory sensor findings\n\n"
	if terminal.Detail != "" {
		body += terminal.Detail + "\n\n"
	}
	if findings := formatSensorFindings(terminal.Findings); findings != "" {
		body += "## Findings\n\n" + findings + "\n"
	}
	if err := writeProjectArtifact(recordRoot, relative, []byte(body)); err != nil {
		return "", err
	}
	return relative, nil
}

func validSensorDetailComponent(value string) bool {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func sensorTerminalEvent(status sensor.Status) string {
	switch status {
	case sensor.StatusCompleted:
		return "SENSOR_PASSED"
	case sensor.StatusUnavailable:
		return "SENSOR_BUDGET_OVERRIDE"
	case sensor.StatusFailed:
		return "SENSOR_FAILED"
	default:
		return "SENSOR_FAILED"
	}
}

func validateSensorFire(result sensor.Result) error {
	if result.Sensor == "" || result.Status != sensor.StatusFired {
		return fmt.Errorf("record sensor invocation: invalid FIRED result: %w", ErrIntentCaptureAmbiguous)
	}
	fireID := result.FireID()
	if len(fireID) != 8 {
		return fmt.Errorf("record sensor invocation: invalid fire id: %w", ErrIntentCaptureAmbiguous)
	}
	for _, char := range fireID {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return fmt.Errorf("record sensor invocation: invalid fire id: %w", ErrIntentCaptureAmbiguous)
		}
	}
	return nil
}

func formatSensorFindings(findings []sensor.Finding) string {
	values := make([]string, 0, len(findings))
	for _, finding := range findings {
		value := strings.TrimSpace(finding.Message)
		if finding.Path != "" {
			value = finding.Path + ": " + value
		}
		if value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, "; ")
}

// RecordIntentCaptureLearningDecision records the mandatory learnings question
// for the current stage attempt and returns the question derived by the
// backend. Sensor receipts are observations only; their presence or recording
// success is deliberately not part of this authority boundary.
func RecordIntentCaptureLearningDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage string) (learnings.Question, error) {
	if !validIntentCaptureToken(stage) {
		return learnings.Question{}, fmt.Errorf("record learning decision: invalid stage: %w", ErrIntentCaptureAmbiguous)
	}
	var question learnings.Question
	err := recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record learning decision: read state: %w", err)
		}
		if document.State.CurrentStage() != stage {
			return fmt.Errorf("record learning decision: current stage changed: %w", ErrIntentCaptureStale)
		}
		surface, err := readIntentCaptureLearningSurface(recordRoot, identity, stage)
		if err != nil {
			return fmt.Errorf("record learning decision: read surface: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record learning decision: read audit: %w", err)
		}
		ordered, err := orderIntentCaptureRecords(records)
		if err != nil {
			return fmt.Errorf("record learning decision: order audit: %w", err)
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, stage)
		generation := intentCaptureLearningGeneration(ordered, stage)
		question, err = newIntentCaptureLearningQuestionWithSurface(identity, stage, generation, surface)
		if err != nil {
			return err
		}
		for index, record := range ordered {
			if index > anchor && record.Event == "DECISION_RECORDED" && record.Fields["Stage"] == stage && record.Fields["Learning Question"] != "" {
				return fmt.Errorf("record learning decision: current attempt already has a learning decision: %w", ErrIntentCaptureStale)
			}
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, []Event{{
			Event: "DECISION_RECORDED",
			Fields: map[string]string{
				"Stage": stage, "Decision": question.ID, "Learning Question": question.ID,
				"Learning Generation": fmt.Sprint(question.Generation), "Learning Surface": surface.Fingerprint, "Options": "none",
			},
		}})
	})
	if err != nil {
		return learnings.Question{}, err
	}
	return question, nil
}

// ResolveIntentCaptureLearningDecision returns the current attempt's backend-
// recorded learning question without creating another decision receipt.
func ResolveIntentCaptureLearningDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage string) (learnings.Question, error) {
	if !validIntentCaptureToken(stage) {
		return learnings.Question{}, fmt.Errorf("resolve learning decision: invalid stage: %w", ErrIntentCaptureAmbiguous)
	}
	var question learnings.Question
	err := recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("resolve learning decision: read state: %w", err)
		}
		if document.State.CurrentStage() != stage {
			return fmt.Errorf("resolve learning decision: current stage changed: %w", ErrIntentCaptureStale)
		}
		surface, err := readIntentCaptureLearningSurface(recordRoot, identity, stage)
		if err != nil {
			return fmt.Errorf("resolve learning decision: read surface: %w", err)
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("resolve learning decision: read audit: %w", err)
		}
		ordered, err := orderIntentCaptureRecords(records)
		if err != nil {
			return fmt.Errorf("resolve learning decision: order audit: %w", err)
		}
		anchor := latestIntentCaptureStageEpochIndex(ordered, stage)
		for index := len(ordered) - 1; index > anchor; index-- {
			record := ordered[index]
			if record.Event != "DECISION_RECORDED" || record.Fields["Stage"] != stage || record.Fields["Learning Question"] == "" {
				continue
			}
			generation, err := strconv.ParseUint(record.Fields["Learning Generation"], 10, 64)
			if err != nil || generation == 0 {
				return fmt.Errorf("resolve learning decision: invalid generation: %w", ErrIntentCaptureAmbiguous)
			}
			question, err = newIntentCaptureLearningQuestionWithSurface(identity, stage, generation, surface)
			if err != nil {
				return err
			}
			if record.Fields["Learning Question"] != question.ID || record.Fields["Decision"] != question.ID {
				return fmt.Errorf("resolve learning decision: question identity changed: %w", ErrIntentCaptureStale)
			}
			return nil
		}
		return fmt.Errorf("resolve learning decision: current attempt has no learning decision: %w", ErrIntentCaptureStale)
	})
	if err != nil {
		return learnings.Question{}, err
	}
	return question, nil
}

func newIntentCaptureLearningQuestion(identity recordlock.Identity, stage string, generation uint64) (learnings.Question, error) {
	return newIntentCaptureLearningQuestionWithSurface(identity, stage, generation, learnings.Surface{})
}

func newIntentCaptureLearningQuestionWithSurface(identity recordlock.Identity, stage string, generation uint64, surface learnings.Surface) (learnings.Question, error) {
	return learnings.NewQuestion(learnings.QuestionInput{
		Stage: stage, Identity: identity.String(), Generation: generation,
		SensorIDs: []string{sensor.ClaimSourcesID, sensor.RequiredSectionsID, sensor.UpstreamCoverageID},
		Surface:   surface,
	})
}

// ReadIntentCaptureLearningSurface returns the current visible memory surface
// without minting an audit event. The hidden receiver uses this only for
// presentation; persistence re-reads it under the identity-bound lock.
func ReadIntentCaptureLearningSurface(recordRoot *os.Root, identity recordlock.Identity, stage string) (learnings.Surface, error) {
	return readIntentCaptureLearningSurface(recordRoot, identity, stage)
}

func readIntentCaptureLearningSurface(recordRoot *os.Root, identity recordlock.Identity, stage string) (learnings.Surface, error) {
	content, err := readReviewArtifact(recordRoot, "ideation/intent-capture/memory.md")
	if errors.Is(err, os.ErrNotExist) {
		content = nil
	} else if err != nil {
		return learnings.Surface{}, err
	}
	if !utf8.Valid(content) {
		return learnings.Surface{}, fmt.Errorf("learning memory is not UTF-8: %w", learnings.ErrInvalid)
	}
	return learnings.ParseSurface(learnings.SurfaceInput{
		Stage: stage, Identity: identity.String(), Space: identity.Space(), Intent: identity.Intent(), Content: content,
	})
}

func intentCaptureLearningGeneration(records []AuditRecord, stage string) uint64 {
	generation := uint64(0)
	for _, record := range records {
		if record.Fields["Stage"] != stage {
			continue
		}
		if record.Event == "STAGE_STARTED" {
			generation++
		}
	}
	if generation == 0 {
		generation = 1
	}
	return generation
}

// RecordIntentCaptureLearning records the mandatory learning answer receipt
// separately from optional RULE_LEARNED/SENSOR_PROPOSED selections. The
// caller's FreshTurn bit is deliberately ignored; freshness is derived from
// the locked audit sequence after the matching learning decision.
func RecordIntentCaptureLearning(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, question learnings.Question, answer learnings.Answer) error {
	if question.Identity != identity.String() {
		return fmt.Errorf("record learning: identity changed: %w", learnings.ErrStale)
	}
	return recordlock.With(ctx, identity, func(guard *recordlock.Guard) error {
		document, err := state.ReadDocument(recordRoot)
		if err != nil {
			return fmt.Errorf("record learning: read state: %w", err)
		}
		if document.State.CurrentStage() != question.Stage {
			return fmt.Errorf("record learning: current stage changed: %w", learnings.ErrStale)
		}
		surface, err := readIntentCaptureLearningSurface(recordRoot, identity, question.Stage)
		if err != nil {
			return fmt.Errorf("record learning: read surface: %w", err)
		}
		backendQuestion, err := newIntentCaptureLearningQuestionWithSurface(identity, question.Stage, question.Generation, surface)
		if err != nil {
			return err
		}
		if surface.Fingerprint != "" && backendQuestion.ID != question.ID {
			return fmt.Errorf("record learning: surface changed: %w", learnings.ErrStale)
		}
		effectiveQuestion := backendQuestion
		// Empty memory has no surface bytes to bind. Keep the source-compatible
		// direct audit seam for that empty case; production surface/persist
		// calls always carry a non-empty backend fingerprint and use the
		// re-derived question above.
		if surface.Fingerprint == "" {
			effectiveQuestion = question
		}
		records, err := ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
		if err != nil {
			return fmt.Errorf("record learning: read audit: %w", err)
		}
		ordered, err := orderIntentCaptureRecords(records)
		if err != nil {
			return fmt.Errorf("record learning: order audit: %w", err)
		}
		existingAnswer, canRetry := retryableCurrentLearningAnswer(ordered, effectiveQuestion)
		if err := validateCurrentLearningPair(ordered, effectiveQuestion); err != nil && !canRetry {
			return err
		}
		validatedAnswer := answer
		validatedAnswer.FreshTurn = true
		learningEvents, err := learnings.Persist(effectiveQuestion, validatedAnswer)
		if err != nil {
			return err
		}
		if err := persistLearningSelections(projectRoot, identity, effectiveQuestion, surface, answer.Selections); err != nil {
			return err
		}
		selection := "none"
		hasLearning, hasSensor := false, false
		for _, selected := range answer.Selections {
			hasLearning = hasLearning || selected.Type == learnings.SelectionTypeLearning
			hasSensor = hasSensor || selected.Type == learnings.SelectionTypeSensor
		}
		if hasLearning && hasSensor {
			selection = "learning,sensor"
		} else if hasLearning {
			selection = "learning"
		} else if hasSensor {
			selection = "sensor"
		} else if strings.TrimSpace(answer.Rule) != "" && strings.TrimSpace(answer.ProposedSensor) != "" {
			selection = "rule,sensor"
		} else if strings.TrimSpace(answer.Rule) != "" {
			selection = "rule"
		} else if strings.TrimSpace(answer.ProposedSensor) != "" {
			selection = "sensor"
		}
		if canRetry && existingAnswer.Fields["Learning Selection"] != selection {
			return fmt.Errorf("record learning: retry selection differs from recorded answer: %w", learnings.ErrConflict)
		}
		events := make([]Event, 0, len(learningEvents)+1)
		if !canRetry {
			events = append(events, Event{Event: "QUESTION_ANSWERED", Fields: map[string]string{
				"Stage":               effectiveQuestion.Stage,
				"Learning Question":   effectiveQuestion.ID,
				"Learning Generation": fmt.Sprint(effectiveQuestion.Generation),
				"Learning Selection":  selection,
				"Details":             selection,
			}})
		}
		for _, learningEvent := range learningEvents {
			fields := cloneStringMap(learningEvent.Fields)
			if learningEvent.Type == learnings.EventRuleLearned {
				candidateID := fields["Candidate-ID"]
				for _, selection := range answer.Selections {
					if selection.CandidateID == candidateID {
						fields["Destination"] = path.Join("aidlc", "spaces", identity.Space(), "memory", selection.Scope+".md")
						break
					}
				}
			} else if learningEvent.Type == learnings.EventSensorProposed {
				if sensorID := fields["Sensor ID"]; sensorID != "" {
					fields["Manifest path"] = path.Join(".codex", "sensors", "aidlc-"+sensorID+".md")
				}
			}
			event := Event{Event: learningEvent.Type, Fields: fields}
			if !learningEventAlreadyRecorded(ordered, event) {
				events = append(events, event)
			}
		}
		if len(events) == 0 {
			return nil
		}
		return appendIntentCaptureForIdentity(ctx, identity, guard, projectRoot, recordRoot, events)
	})
}

func persistLearningSelections(projectRoot *os.Root, identity recordlock.Identity, question learnings.Question, surface learnings.Surface, selections []learnings.Selection) error {
	if len(selections) == 0 {
		return nil
	}
	for _, selection := range selections {
		candidate, ok := learningCandidate(question, selection.CandidateID)
		if !ok && selection.Source == "user_addition" {
			candidate = learnings.Candidate{ID: selection.CandidateID, Heading: selection.Heading, Text: selection.Text, Summary: selection.Text, Scope: selection.Scope, Source: selection.Source}
			ok = true
		}
		if !ok {
			return fmt.Errorf("persist learning selection %q: current candidate is missing: %w", selection.CandidateID, learnings.ErrStale)
		}
		switch selection.Type {
		case learnings.SelectionTypeLearning:
			if err := writePracticeSelection(projectRoot, identity, question.Stage, candidate, selection); err != nil {
				return err
			}
		case learnings.SelectionTypeSensor:
			if err := writeSensorSelection(projectRoot, identity, question.Stage, selection); err != nil {
				return err
			}
		default:
			return fmt.Errorf("persist learning selection type %q: %w", selection.Type, learnings.ErrInvalid)
		}
	}
	_ = surface
	return nil
}

func learningCandidate(question learnings.Question, id string) (learnings.Candidate, bool) {
	for _, candidate := range question.Candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return learnings.Candidate{}, false
}

func writePracticeSelection(projectRoot *os.Root, identity recordlock.Identity, stage string, candidate learnings.Candidate, selection learnings.Selection) error {
	digest := sha256.Sum256([]byte(selection.Text))
	contentHash := hex.EncodeToString(digest[:])
	marker := fmt.Sprintf("<!-- cid:%s:%s:%s -->", identity.Intent(), stage, contentHash)
	name := path.Join("aidlc", "spaces", identity.Space(), "memory", selection.Scope+".md")
	content, err := readOptionalProjectArtifact(projectRoot, name)
	if err != nil {
		return fmt.Errorf("read learning destination %q: %w", name, err)
	}
	if strings.Contains(string(content), marker) {
		return nil
	}
	heading := strings.TrimSpace(selection.Heading)
	if heading == "" {
		heading = "Corrections"
	}
	if strings.ContainsAny(heading, "\r\n") || strings.HasPrefix(heading, "#") {
		return fmt.Errorf("learning heading is unsafe: %w", learnings.ErrInvalid)
	}
	line := fmt.Sprintf("- %s (learned %s) %s\n", selection.Text, time.Now().UTC().Format("2006-01-02"), marker)
	if len(bytes.TrimSpace(content)) == 0 {
		if selection.Scope == "team" {
			content = []byte("# Team-Level Rules\n\n")
		} else {
			content = []byte("# Project-Level Rules\n\n")
		}
	}
	updated := appendLearningLine(content, heading, line)
	return writeProjectArtifact(projectRoot, name, updated)
}

func appendLearningLine(content []byte, heading, line string) []byte {
	text := strings.ReplaceAll(strings.ReplaceAll(string(content), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	for index, raw := range lines {
		if strings.TrimSpace(raw) != "## "+heading {
			continue
		}
		insert := index + 1
		for insert < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[insert]), "## ") {
			insert++
		}
		lines = append(lines, "")
		copy(lines[insert+1:], lines[insert:])
		lines[insert] = strings.TrimSuffix(line, "\n")
		return []byte(strings.Join(lines, "\n"))
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + "\n## " + heading + "\n" + line)
}

func writeSensorSelection(projectRoot *os.Root, identity recordlock.Identity, stage string, selection learnings.Selection) error {
	fields := selection.ManifestFields
	sensorID := fields["id"]
	name := path.Join(".codex", "sensors", "aidlc-"+sensorID+".md")
	stagePath := path.Join(".codex", "aidlc-common", "stages", "ideation", stage+".md")
	stageContent, err := readOptionalProjectArtifact(projectRoot, stagePath)
	if err != nil || len(stageContent) == 0 {
		if err == nil {
			err = os.ErrNotExist
		}
		return fmt.Errorf("read originating stage %q: %w", stagePath, err)
	}
	updated, err := appendStageSensorBinding(stageContent, sensorID)
	if err != nil {
		return err
	}
	manifest := "---\n" +
		"id: " + sensorID + "\n" +
		"kind: " + fields["kind"] + "\n" +
		"command: " + quoteManifestValue(fields["command"]) + "\n" +
		"default_severity: " + fields["default_severity"] + "\n" +
		"description: " + quoteManifestValue(fields["description"]) + "\n" +
		"matches: " + quoteManifestValue(fields["matches"]) + "\n"
	if value := fields["timeout_seconds"]; value != "" {
		manifest += "timeout_seconds: " + value + "\n"
	}
	if value := fields["category"]; value != "" {
		manifest += "category: " + quoteManifestValue(value) + "\n"
	}
	manifest += "---\n\n# " + sensorID + " sensor\n\n" +
		fields["description"] + "\n\n" +
		"Scaffolded by the §13 learning gate (project-tier).\n"
	if existing, err := readOptionalProjectArtifact(projectRoot, name); err != nil {
		return fmt.Errorf("read sensor manifest %q: %w", name, err)
	} else if len(existing) != 0 && string(existing) != manifest {
		return fmt.Errorf("sensor manifest %q conflicts with existing content: %w", name, learnings.ErrConflict)
	} else if len(existing) == 0 {
		if err := writeProjectArtifact(projectRoot, name, []byte(manifest)); err != nil {
			return err
		}
	}
	if !bytes.Equal(updated, stageContent) {
		if err := writeProjectArtifact(projectRoot, stagePath, updated); err != nil {
			return err
		}
	}
	return nil
}

func quoteManifestValue(value string) string {
	return strconv.Quote(value)
}

func appendStageSensorBinding(content []byte, sensorID string) ([]byte, error) {
	text := strings.ReplaceAll(strings.ReplaceAll(string(content), "\r\n", "\n"), "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("originating stage frontmatter is missing: %w", learnings.ErrInvalid)
	}
	lines := strings.Split(text, "\n")
	frontEnd := -1
	for index := 1; index < len(lines); index++ {
		if lines[index] == "---" {
			frontEnd = index
			break
		}
	}
	if frontEnd < 0 {
		return nil, fmt.Errorf("originating stage frontmatter is malformed: %w", learnings.ErrInvalid)
	}
	slugCount, phaseCount, sensorsCount := 0, 0, 0
	sensorsLine := -1
	for index := 1; index < frontEnd; index++ {
		line := strings.TrimSpace(lines[index])
		switch {
		case strings.HasPrefix(line, "slug:"):
			slugCount++
			if line != "slug: intent-capture" {
				return nil, fmt.Errorf("originating stage identity is invalid: %w", learnings.ErrInvalid)
			}
		case strings.HasPrefix(line, "phase:"):
			phaseCount++
			if line != "phase: ideation" {
				return nil, fmt.Errorf("originating stage identity is invalid: %w", learnings.ErrInvalid)
			}
		case line == "sensors:":
			sensorsCount++
			sensorsLine = index
		case strings.HasPrefix(line, "sensors:"):
			return nil, fmt.Errorf("originating stage sensors binding is malformed: %w", learnings.ErrInvalid)
		}
	}
	if slugCount != 1 || phaseCount != 1 || sensorsCount != 1 || sensorsLine < 0 {
		return nil, fmt.Errorf("originating stage frontmatter identity is ambiguous: %w", learnings.ErrInvalid)
	}
	insert := sensorsLine + 1
	for insert < frontEnd && strings.HasPrefix(strings.TrimSpace(lines[insert]), "-") {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[insert]), "-")) == sensorID {
			return []byte(text), nil
		}
		insert++
	}
	lines = append(lines, "")
	copy(lines[insert+1:], lines[insert:])
	lines[insert] = "  - " + sensorID
	return []byte(strings.Join(lines, "\n")), nil
}

func readOptionalProjectArtifact(root *os.Root, name string) ([]byte, error) {
	content, err := readReviewArtifact(root, name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("project artifact %q is not UTF-8: %w", name, learnings.ErrInvalid)
	}
	return content, nil
}

func writeProjectArtifact(root *os.Root, name string, content []byte) error {
	return writeProjectArtifactWithOps(root, name, content, defaultProjectArtifactOps())
}

type projectArtifactOps struct {
	openTemp func(*os.Root, string) (*os.File, error)
	write    func(*os.File, []byte) (int, error)
	close    func(*os.File) error
	rename   func(*os.Root, string, string) error
	remove   func(*os.Root, string) error
}

var projectArtifactTempSequence atomic.Uint64

func defaultProjectArtifactOps() projectArtifactOps {
	return projectArtifactOps{
		openTemp: func(root *os.Root, name string) (*os.File, error) {
			return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		},
		write:  func(file *os.File, content []byte) (int, error) { return file.Write(content) },
		close:  func(file *os.File) error { return file.Close() },
		rename: func(root *os.Root, oldName, newName string) error { return root.Rename(oldName, newName) },
		remove: func(root *os.Root, name string) error { return root.Remove(name) },
	}
}

func writeProjectArtifactWithOps(root *os.Root, name string, content []byte, ops projectArtifactOps) error {
	if root == nil || name == "" || !fs.ValidPath(name) || path.IsAbs(name) || strings.Contains(name, "\\") {
		return fmt.Errorf("project artifact path %q is unsafe: %w", name, learnings.ErrInvalid)
	}
	if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
		return err
	}
	info, err := root.Lstat(name)
	existed := err == nil
	if existed && (info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("project artifact %q is not a regular file: %w", name, learnings.ErrInvalid)
	}
	if !existed && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if ops.openTemp == nil || ops.write == nil || ops.close == nil || ops.rename == nil || ops.remove == nil {
		return fmt.Errorf("project artifact operations are incomplete: %w", learnings.ErrInvalid)
	}
	tempName := ""
	var file *os.File
	for attempt := 0; attempt < 100; attempt++ {
		sequence := projectArtifactTempSequence.Add(1)
		tempName = path.Join(path.Dir(name), fmt.Sprintf(".%s.tmp-%d", path.Base(name), sequence))
		file, err = ops.openTemp(root, tempName)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	if file == nil {
		return fmt.Errorf("project artifact temporary name exhausted: %w", learnings.ErrConflict)
	}
	committed := false
	defer func() {
		if !committed {
			_ = ops.remove(root, tempName)
		}
	}()
	written, writeErr := ops.write(file, content)
	closeErr := ops.close(file)
	if writeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if written != len(content) {
		return errors.Join(io.ErrShortWrite, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	current, currentErr := root.Lstat(name)
	if existed {
		if currentErr != nil || current == nil || current.Mode()&fs.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(info, current) {
			return fmt.Errorf("project artifact %q changed identity before replace: %w", name, learnings.ErrStale)
		}
	} else if currentErr == nil {
		return fmt.Errorf("project artifact %q appeared before create: %w", name, learnings.ErrStale)
	} else if !errors.Is(currentErr, os.ErrNotExist) {
		return currentErr
	}
	if err := ops.rename(root, tempName, name); err != nil {
		return err
	}
	committed = true
	return nil
}

func validateCurrentLearningPair(records []AuditRecord, question learnings.Question) error {
	anchor := latestIntentCaptureStageEpochIndex(records, question.Stage)
	decisionIndex := -1
	for index, record := range records {
		if index <= anchor || record.Event != "DECISION_RECORDED" || record.Fields["Stage"] != question.Stage || record.Fields["Learning Question"] != question.ID {
			continue
		}
		if record.Fields["Learning Generation"] != fmt.Sprint(question.Generation) || record.Fields["Decision"] != question.ID {
			return fmt.Errorf("record learning: matching decision identity changed: %w", learnings.ErrStale)
		}
		if decisionIndex >= 0 {
			return fmt.Errorf("record learning: duplicate current learning decision: %w", learnings.ErrStale)
		}
		decisionIndex = index
	}
	if decisionIndex < 0 {
		return fmt.Errorf("record learning: matching current learning decision is missing: %w", learnings.ErrStale)
	}
	humanTurns := 0
	for _, record := range records[decisionIndex+1:] {
		switch record.Event {
		case "HUMAN_TURN":
			humanTurns++
		case "QUESTION_ANSWERED":
			if record.Fields["Learning Question"] == question.ID {
				return fmt.Errorf("record learning: question is already answered: %w", learnings.ErrStale)
			}
		}
	}
	if humanTurns != 1 {
		return fmt.Errorf("record learning: current learning decision has %d HUMAN_TURN receipts; want exactly one: %w", humanTurns, learnings.ErrStale)
	}
	return nil
}

// retryableCurrentLearningAnswer identifies the narrow recovery case where a
// prior attempt durably wrote QUESTION_ANSWERED but failed before its owned
// learning rows were appended. It deliberately requires the same current
// decision, one HUMAN_TURN, and a complete answer binding; arbitrary or
// legacy-looking rows cannot authorize a retry.
func retryableCurrentLearningAnswer(records []AuditRecord, question learnings.Question) (AuditRecord, bool) {
	anchor := latestIntentCaptureStageEpochIndex(records, question.Stage)
	decisionIndex := -1
	for index, record := range records {
		if index <= anchor || record.Event != "DECISION_RECORDED" || record.Fields["Stage"] != question.Stage || record.Fields["Learning Question"] != question.ID {
			continue
		}
		if record.Fields["Learning Generation"] != fmt.Sprint(question.Generation) || record.Fields["Decision"] != question.ID || decisionIndex >= 0 {
			return AuditRecord{}, false
		}
		decisionIndex = index
	}
	if decisionIndex < 0 {
		return AuditRecord{}, false
	}
	humanTurns := 0
	var answer AuditRecord
	answerCount := 0
	for _, record := range records[decisionIndex+1:] {
		switch record.Event {
		case "HUMAN_TURN":
			humanTurns++
		case "QUESTION_ANSWERED":
			if record.Fields["Learning Question"] != question.ID {
				continue
			}
			answer = record
			answerCount++
		}
	}
	if humanTurns != 1 || answerCount != 1 || answer.Fields["Learning Generation"] != fmt.Sprint(question.Generation) || strings.TrimSpace(answer.Fields["Learning Selection"]) == "" {
		return AuditRecord{}, false
	}
	return answer, true
}

func learningEventAlreadyRecorded(records []AuditRecord, event Event) bool {
	for _, record := range records {
		if record.Event != event.Event || record.Fields["Stage"] != event.Fields["Stage"] {
			continue
		}
		switch event.Event {
		case learnings.EventRuleLearned:
			if record.Fields["Content-Hash"] == event.Fields["Content-Hash"] {
				return true
			}
		case learnings.EventSensorProposed:
			if record.Fields["Sensor ID"] == event.Fields["Sensor ID"] {
				return true
			}
		}
	}
	return false
}

func cloneStringMap(fields map[string]string) map[string]string {
	if fields == nil {
		return nil
	}
	clone := make(map[string]string, len(fields))
	for key, value := range fields {
		clone[key] = value
	}
	return clone
}

var (
	ErrIntentCaptureAmbiguous = errors.New("audit: intent-capture evidence is ambiguous")
	ErrIntentCaptureStale     = errors.New("audit: intent-capture evidence is stale")
)

// IntentCaptureDecision identifies one purpose-specific question or summary
// decision. The fingerprint is generated by the reader from the question
// bytes; callers do not provide timestamps or audit positions.
type IntentCaptureDecision struct {
	Stage       string
	DecisionID  string
	Fingerprint string
	// Options is the rendered choice set for a normal question. It is
	// presentation evidence only; the decision identity and fingerprint are
	// still derived/validated by the owning API.
	Options string
}

// RecordIntentCaptureDecision records the decision boundary before its human
// response. It deliberately does not mint a HUMAN_TURN event.
func RecordIntentCaptureDecision(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision) error {
	return recordStageDecision(ctx, identity, projectRoot, recordRoot, decision, legacyIntentSummaryContract)
}

func summaryConfirmationMatchesAfter(records []AuditRecord, decisionIndex int, digest string) bool {
	for _, record := range records[decisionIndex+1:] {
		if record.Event == "SUMMARY_CONFIRMATION_RECORDED" && record.Fields["Questions SHA-256"] == digest && record.Fields["Hash Scope"] == intentCaptureHashScope {
			return true
		}
	}
	return false
}

// RecordIntentCaptureDecisionFromQuestions is the hidden-bridge entry point
// for an ordinary question. It derives the question fingerprint while the
// record lock is held; the caller supplies only the decision identifier.
func RecordIntentCaptureDecisionFromQuestions(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID string) error {
	return RecordIntentCaptureDecisionFromQuestionsWithOptions(ctx, identity, projectRoot, recordRoot, stage, decisionID, "")
}

// RecordIntentCaptureDecisionFromQuestionsWithOptions is the hidden bridge
// entry point for a normal question. The question file fingerprint is
// derived while the record lock is held; the caller may provide only the
// rendered options, never an authority fingerprint or audit position.
func RecordIntentCaptureDecisionFromQuestionsWithOptions(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, options string) error {
	return recordStageQuestionDecision(ctx, identity, projectRoot, recordRoot, stage, decisionID, options, legacyIntentSummaryContract)
}

// RecordIntentCaptureAnswer records a question answer only after exactly one
// fresh HUMAN_TURN follows the matching decision. A second turn, an already
// answered decision, or ambiguous cross-shard ordering fails closed.
func RecordIntentCaptureAnswer(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision, answer string) error {
	return recordStageAnswer(ctx, identity, projectRoot, recordRoot, decision, answer)
}

// RecordIntentCaptureAnswerByID is the hidden-bridge answer entry point. The
// backend resolves the latest current-attempt decision and its fingerprint
// from the locked ledger, so callers cannot supply a digest or stale receipt.
func RecordIntentCaptureAnswerByID(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, stage, decisionID, answer string) error {
	return recordStageQuestionAnswer(ctx, identity, projectRoot, recordRoot, stage, decisionID, answer, legacyIntentSummaryContract)
}

// RecordSummaryConfirmation records the exact consolidated-summary response
// after its own decision and fresh human turn. The hash scope is a fixed
// protocol token, not a caller-selected authority mode.
func RecordSummaryConfirmation(ctx context.Context, identity recordlock.Identity, projectRoot, recordRoot *os.Root, decision IntentCaptureDecision, answer, contentFingerprint string) error {
	_ = contentFingerprint // retained for source compatibility; never authority
	return recordStageSummaryConfirmation(ctx, identity, projectRoot, recordRoot, decision, answer, legacyIntentSummaryContract)
}

const (
	// Artifact paths in owned audit fields are relative to the identity-bound
	// record root. This is the fixed engine placement from the stage source.
	intentCaptureQuestionsFile  = "ideation/intent-capture/intent-capture-questions.md"
	intentCaptureHashScope      = "confirmed-content-v1"
	maxIntentCaptureQuestions   = 8 << 20
	intentCaptureSummaryOptions = "Looks correct,Request changes"
)

var (
	summaryATXHeadingPattern = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+|$)(.*)$`)
	summarySetextPattern     = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	summaryRawHeadingPattern = regexp.MustCompile(`(?i)<h([1-6])\b`)
	summaryQuestionPattern   = regexp.MustCompile(`^Q([1-9][0-9]*)(?:[.:](?:[ \t]+.*)?)?$`)
)

type summaryHeading struct {
	line  int
	title string
	level int
	style string
}

// summaryConfirmationCanonicalContent implements the fixed
// confirmed-content-v1 boundary. It deliberately uses the sensor's
// line-preserving visibility helper, then applies a stricter top-level heading
// grammar because a summary receipt must never be minted from code, comments,
// or a nested Markdown container.
func summaryConfirmationCanonicalContent(content []byte, requireAnswer bool) ([]byte, error) {
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("questions file is not valid UTF-8: %w", ErrIntentCaptureAmbiguous)
	}
	normalized := strings.ReplaceAll(strings.ReplaceAll(string(content), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")
	visible := sensor.VisibleMarkdownLines(normalized)
	for index := range visible {
		// A line containing a comment marker is ambiguous even when it has
		// visible text on either side. Treating the whole line as invisible is
		// conservative and prevents `##<!--...-->` from becoming authority.
		raw := lines[index]
		if strings.Contains(raw, "<!--") || strings.Contains(raw, "-->") || strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, "    ") {
			visible[index] = ""
		}
	}
	headings := summaryHeadings(lines, visible)
	if len(headings) == 0 {
		return nil, fmt.Errorf("summary confirmation checkpoint is missing: %w", ErrIntentCaptureAmbiguous)
	}
	summaryLine := -1
	seenQuestion := make(map[string]bool)
	seenH2 := make(map[string]bool)
	seenFollowUp := make(map[string]bool)
	postAssumptionLine := -1
	postAssumptionEnd := len(lines)
	postAssumptionSeen := false
	for _, heading := range headings {
		if heading.style == "atx" && heading.level == 2 && heading.title == "Consolidated Summary Confirmation" {
			if summaryLine >= 0 {
				return nil, fmt.Errorf("summary confirmation checkpoint is duplicated: %w", ErrIntentCaptureAmbiguous)
			}
			summaryLine = heading.line
			continue
		}
		if summaryLine < 0 || heading.line < summaryLine {
			if heading.style == "atx" && heading.level == 2 {
				if heading.title == "Assumption Confirmation" || heading.title == "Requested Changes Feedback" {
					if seenFollowUp[heading.title] {
						return nil, fmt.Errorf("section %q is duplicated: %w", heading.title, ErrIntentCaptureAmbiguous)
					}
					seenFollowUp[heading.title] = true
					continue
				}
				if questionID := summaryQuestionPattern.FindStringSubmatch(heading.title); questionID != nil {
					key := "Q" + questionID[1]
					if seenQuestion[key] {
						return nil, fmt.Errorf("question section %q is duplicated: %w", key, ErrIntentCaptureAmbiguous)
					}
					seenQuestion[key] = true
				}
				if heading.title != "Assumption Confirmation" && heading.title != "Requested Changes Feedback" {
					if seenH2[heading.title] {
						return nil, fmt.Errorf("H2 section %q is duplicated: %w", heading.title, ErrIntentCaptureAmbiguous)
					}
					seenH2[heading.title] = true
				}
			}
			continue
		}

		// Once the checkpoint is found, only the exact follow-up sections
		// allowed by the fixed source may follow it. Any other visible
		// Markdown, setext, or raw-HTML heading fails closed.
		if heading.style != "atx" || heading.level != 2 {
			return nil, fmt.Errorf("unsupported heading %q after summary confirmation: %w", heading.title, ErrIntentCaptureAmbiguous)
		}
		switch {
		case heading.title == "Assumption Confirmation":
			if postAssumptionSeen || seenFollowUp[heading.title] {
				return nil, fmt.Errorf("Assumption Confirmation section is duplicated: %w", ErrIntentCaptureAmbiguous)
			}
			seenFollowUp[heading.title] = true
			postAssumptionSeen = true
			postAssumptionLine = heading.line
		case heading.title == "Requested Changes Feedback":
			if seenFollowUp[heading.title] {
				return nil, fmt.Errorf("Requested Changes Feedback section is duplicated: %w", ErrIntentCaptureAmbiguous)
			}
			seenFollowUp[heading.title] = true
			if postAssumptionSeen && postAssumptionEnd == len(lines) {
				postAssumptionEnd = heading.line
			}
		case summaryQuestionPattern.MatchString(heading.title):
			questionMatch := summaryQuestionPattern.FindStringSubmatch(heading.title)
			questionID := "Q" + questionMatch[1]
			if seenQuestion[questionID] {
				return nil, fmt.Errorf("question section %q is duplicated: %w", questionID, ErrIntentCaptureAmbiguous)
			}
			seenQuestion[questionID] = true
			if postAssumptionSeen && postAssumptionEnd == len(lines) {
				postAssumptionEnd = heading.line
			}
		default:
			return nil, fmt.Errorf("unsupported heading %q after summary confirmation: %w", heading.title, ErrIntentCaptureAmbiguous)
		}
	}
	if summaryLine < 0 {
		return nil, fmt.Errorf("summary confirmation checkpoint is missing: %w", ErrIntentCaptureAmbiguous)
	}
	// The exact answer must be visible in the summary section, not in a
	// nested list, code span, comment, or a later excluded section.
	summaryEnd := len(lines)
	for _, heading := range headings {
		if heading.line > summaryLine && heading.line < summaryEnd {
			summaryEnd = heading.line
		}
	}
	answerCount := 0
	answerExact := true
	for index := summaryLine + 1; index < summaryEnd; index++ {
		line := strings.TrimSpace(strings.TrimRight(visible[index], " \t"))
		if !strings.HasPrefix(line, "[Answer]:") {
			continue
		}
		answerCount++
		if line != "[Answer]: Looks correct" {
			answerExact = false
		}
	}
	if requireAnswer && (answerCount != 1 || !answerExact) {
		return nil, fmt.Errorf("summary confirmation requires exactly one visible [Answer]: Looks correct: %w", ErrIntentCaptureAmbiguous)
	}

	canonical := make([]string, 0, len(lines))
	for index, line := range lines {
		if postAssumptionLine >= 0 && index >= postAssumptionLine && index < postAssumptionEnd {
			continue
		}
		canonical = append(canonical, strings.TrimRight(line, " \t"))
	}
	return []byte(strings.TrimRight(strings.Join(canonical, "\n"), "\n \t")), nil
}

func summaryConfirmationContentHash(content []byte) (string, error) {
	canonical, err := summaryConfirmationCanonicalContent(content, true)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

func summaryHeadings(lines, visible []string) []summaryHeading {
	headings := make([]summaryHeading, 0)
	for index, line := range visible {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if match := summaryATXHeadingPattern.FindStringSubmatch(line); match != nil {
			title := strings.TrimSpace(strings.TrimRight(match[2], "#"))
			headings = append(headings, summaryHeading{line: index, title: title, level: len(match[1]), style: "atx"})
			continue
		}
		if summaryRawHeadingPattern.MatchString(line) {
			match := summaryRawHeadingPattern.FindStringSubmatch(line)
			level, _ := strconv.Atoi(match[1])
			headings = append(headings, summaryHeading{line: index, title: "<h" + match[1] + ">", level: level, style: "html"})
			continue
		}
		if summarySetextPattern.MatchString(line) && index > 0 {
			previous := strings.TrimSpace(visible[index-1])
			if previous == "" || summaryATXHeadingPattern.MatchString(previous) {
				continue
			}
			level := 2
			if line[0] == '=' {
				level = 1
			}
			headings = append(headings, summaryHeading{line: index - 1, title: previous, level: level, style: "setext"})
		}
	}
	return headings
}

func readIntentCaptureQuestions(recordRoot *os.Root) ([]byte, string, error) {
	return readStageQuestions(recordRoot, intentCaptureQuestionsFile)
}

// ValidateSummaryConfirmationCurrent verifies the fixed questions leaf and
// canonical confirmation fields against a previously recorded receipt.
func ValidateSummaryConfirmationCurrent(recordRoot *os.Root, record AuditRecord) error {
	return validateStageSummaryContent(recordRoot, record, stageSummaryContract{questionsFile: intentCaptureQuestionsFile})
}

// ValidateReviewReceiptCurrent verifies the latest request/completion pair
// against the current artifact bytes. It is intentionally a read-side check;
// the append APIs remain responsible for writing the two owned events.
func ValidateReviewReceiptCurrent(recordRoot *os.Root, records []AuditRecord, stage, reviewer string) error {
	requests := make(map[string]AuditRecord)
	for _, record := range records {
		if record.Event != "REVIEW_REQUESTED" || record.Fields["Stage"] != stage || record.Fields["Reviewer"] != reviewer {
			continue
		}
		if hasLegacyReviewReceiptField(record.Fields) || !validCanonicalReviewReceiptFields(record.Fields, false) {
			continue
		}
		if fingerprint := record.Fields["Artifact Fingerprint"]; fingerprint != "" {
			requests[fingerprint] = record
		}
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		if record.Event != "REVIEW_COMPLETED" || record.Fields["Stage"] != stage || record.Fields["Reviewer"] != reviewer {
			continue
		}
		if hasLegacyReviewReceiptField(record.Fields) || !validCanonicalReviewReceiptFields(record.Fields, true) {
			return fmt.Errorf("review request/completion binding is stale: %w", ErrIntentCaptureStale)
		}
		request, ok := requests[record.Fields["Request Fingerprint"]]
		if !ok || record.Fields["Iteration"] == "" || record.Fields["Iteration"] != request.Fields["Iteration"] || record.Fields["Review Appendix Artifact"] != request.Fields["Review Appendix Artifact"] || record.Fields["Review Appendix Offset"] != request.Fields["Review Appendix Offset"] || record.Fields["Review Appendix Prior Digest"] != request.Fields["Review Appendix Prior Digest"] || record.Fields["Review Appendix Prior Length"] != request.Fields["Review Appendix Prior Length"] || (record.Fields["Review Challenge"] != request.Fields["Review Challenge"]) {
			return fmt.Errorf("review request/completion binding is stale: %w", ErrIntentCaptureStale)
		}
		if record.Fields["Request Fingerprint"] != request.Fields["Artifact Fingerprint"] {
			return fmt.Errorf("review request fingerprint binding is stale: %w", ErrIntentCaptureStale)
		}
		paths := reviewReceiptArtifactPaths(request.Fields["Review Appendix Artifact"], stage)
		current := make(map[string][]byte, len(paths))
		for _, artifactPath := range paths {
			content, err := readReviewArtifact(recordRoot, artifactPath)
			if err != nil {
				return fmt.Errorf("read current review artifact %q: %w", artifactPath, err)
			}
			current[artifactPath] = content
		}
		if review.ArtifactSnapshotsFingerprint(current) != record.Fields["Artifact Fingerprint"] {
			return fmt.Errorf("current review artifact fingerprint changed: %w", ErrIntentCaptureStale)
		}
		appendixOffset, err := strconv.ParseInt(request.Fields["Review Appendix Offset"], 10, 64)
		if err != nil || appendixOffset < 0 || appendixOffset > int64(len(current[request.Fields["Review Appendix Artifact"]])) {
			return fmt.Errorf("review appendix offset binding is stale: %w", ErrIntentCaptureStale)
		}
		if review.ArtifactSnapshotsRequestFingerprint(current, request.Fields["Review Appendix Artifact"], appendixOffset) != request.Fields["Artifact Fingerprint"] {
			return fmt.Errorf("current review artifact fingerprint changed: %w", ErrIntentCaptureStale)
		}
		return nil
	}
	return fmt.Errorf("review completion is missing: %w", ErrIntentCaptureStale)
}

var reviewReceiptChallengePattern = regexp.MustCompile(`^review:[0-9a-f]{32}$`)
var reviewReceiptDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func hasLegacyReviewReceiptField(fields map[string]string) bool {
	for _, key := range []string{"Artifact Path", "Request Source", "Prior Digest", "Prior Length", "Post Fingerprint", "Artifact Set"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func validCanonicalReviewReceiptFields(fields map[string]string, completion bool) bool {
	allowed := map[string]struct{}{
		"Stage": {}, "Reviewer": {}, "Iteration": {}, "Artifact Fingerprint": {},
		"Review Appendix Artifact": {}, "Review Appendix Offset": {},
		"Review Appendix Prior Digest": {}, "Review Appendix Prior Length": {},
		"Review Challenge": {},
	}
	if completion {
		allowed["Verdict"] = struct{}{}
		allowed["Request Fingerprint"] = struct{}{}
	}
	for key := range fields {
		if _, ok := allowed[key]; !ok {
			return false
		}
	}
	for _, key := range []string{"Stage", "Reviewer", "Iteration", "Artifact Fingerprint", "Review Appendix Artifact", "Review Appendix Offset", "Review Appendix Prior Digest", "Review Appendix Prior Length"} {
		if strings.TrimSpace(fields[key]) == "" {
			return false
		}
	}
	if !reviewReceiptDigestPattern.MatchString(fields["Artifact Fingerprint"]) {
		return false
	}
	priorLength, err := strconv.ParseInt(fields["Review Appendix Prior Length"], 10, 64)
	if err != nil || priorLength < 0 {
		return false
	}
	priorDigest := fields["Review Appendix Prior Digest"]
	challenge, hasChallenge := fields["Review Challenge"]
	if priorLength == 0 {
		if priorDigest != "none" || hasChallenge {
			return false
		}
	} else {
		if !reviewReceiptDigestPattern.MatchString(priorDigest) || !hasChallenge || !reviewReceiptChallengePattern.MatchString(challenge) {
			return false
		}
	}
	if offset, err := strconv.ParseInt(fields["Review Appendix Offset"], 10, 64); err != nil || offset < 0 {
		return false
	}
	if completion {
		for _, key := range []string{"Verdict", "Request Fingerprint"} {
			if strings.TrimSpace(fields[key]) == "" {
				return false
			}
		}
		if !reviewReceiptDigestPattern.MatchString(fields["Request Fingerprint"]) || fields["Verdict"] != "READY" && fields["Verdict"] != "NOT-READY" {
			return false
		}
	}
	return true
}

func reviewReceiptArtifactPaths(appendixArtifact, stage string) []string {
	if stage != "intent-capture" {
		return []string{appendixArtifact}
	}
	dir := path.Dir(appendixArtifact)
	return []string{
		path.Join(dir, "intent-capture-questions.md"),
		path.Join(dir, "intent-statement.md"),
		path.Join(dir, "stakeholder-map.md"),
	}
}

func latestIntentCaptureAttemptIndex(records []AuditRecord, stage string) int {
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

// latestIntentCaptureStageEpochIndex is the floor for ordinary questions,
// summary confirmation, and the mandatory learnings pair. A gate rejection or
// revision does not invalidate already valid answers in the same stage epoch.
func latestIntentCaptureStageEpochIndex(records []AuditRecord, stage string) int {
	anchor := -1
	for index, record := range records {
		if record.Fields["Stage"] == stage && record.Event == "STAGE_STARTED" {
			anchor = index
		}
	}
	return anchor
}

func validateIntentCaptureDecision(decision IntentCaptureDecision) error {
	if !validIntentCaptureToken(decision.Stage) || !validIntentCaptureToken(decision.DecisionID) || decision.Fingerprint == "" {
		return fmt.Errorf("intent-capture decision fields are incomplete: %w", ErrIntentCaptureAmbiguous)
	}
	return nil
}

func validIntentCaptureToken(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if char == '/' || char == '\\' || char < 0x20 {
			return false
		}
	}
	return true
}

func validateIntentCaptureSummaryConfirmation(answer, expectedFingerprint, actualFingerprint string) error {
	if answer != "Looks correct" {
		return fmt.Errorf("summary confirmation answer %q is not exact: %w", answer, ErrIntentCaptureAmbiguous)
	}
	if expectedFingerprint == "" || actualFingerprint == "" || expectedFingerprint != actualFingerprint {
		return fmt.Errorf("summary confirmation fingerprint changed or is missing: %w", ErrIntentCaptureAmbiguous)
	}
	return nil
}

func validateIntentCaptureDecisionAnswer(records []AuditRecord, decision IntentCaptureDecision) error {
	ordered, err := orderIntentCaptureRecords(records)
	if err != nil {
		return err
	}
	decisionIndex := -1
	anchor := latestIntentCaptureStageEpochIndex(ordered, decision.Stage)
	for index, record := range ordered {
		if index <= anchor {
			continue
		}
		if record.Event == "DECISION_RECORDED" && record.Fields["Stage"] == decision.Stage && record.Fields["Decision"] == decision.DecisionID {
			if decision.DecisionID != "summary" && record.Fields["Question Fingerprint"] != decision.Fingerprint {
				return fmt.Errorf("intent-capture decision fingerprint changed: %w", ErrIntentCaptureStale)
			}
			decisionIndex = index
		}
	}
	if decisionIndex < 0 {
		return fmt.Errorf("intent-capture decision %q is missing: %w", decision.DecisionID, ErrIntentCaptureStale)
	}
	humanTurns := 0
	for _, record := range ordered[decisionIndex+1:] {
		switch record.Event {
		case "HUMAN_TURN":
			humanTurns++
		case "QUESTION_ANSWERED", "SUMMARY_CONFIRMATION_RECORDED":
			return fmt.Errorf("intent-capture decision %q is already answered: %w", decision.DecisionID, ErrIntentCaptureStale)
		}
	}
	if humanTurns != 1 {
		return fmt.Errorf("intent-capture decision %q has %d human turns; want exactly one: %w", decision.DecisionID, humanTurns, ErrIntentCaptureStale)
	}
	return nil
}

func orderIntentCaptureRecords(records []AuditRecord) ([]AuditRecord, error) {
	ordered := append([]AuditRecord(nil), records...)
	for left := 0; left < len(ordered); left++ {
		for right := left + 1; right < len(ordered); right++ {
			if ordered[left].Timestamp.Equal(ordered[right].Timestamp) && ordered[left].Shard != ordered[right].Shard {
				return nil, fmt.Errorf("intent-capture audit ordering is ambiguous at %s: %w", ordered[left].Timestamp.Format(time.RFC3339), ErrIntentCaptureAmbiguous)
			}
		}
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		if !ordered[left].Timestamp.Equal(ordered[right].Timestamp) {
			return ordered[left].Timestamp.Before(ordered[right].Timestamp)
		}
		if ordered[left].Shard != ordered[right].Shard {
			return ordered[left].Shard < ordered[right].Shard
		}
		return ordered[left].Position < ordered[right].Position
	})
	return ordered, nil
}
