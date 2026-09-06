package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/audit"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/learnings"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/review"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/sensor"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

type codexStageDispatch func(context.Context, string, deliverypkg.RunStageInput) ([]byte, error)

type codexStageDispatchWithPayload func(context.Context, string, deliverypkg.RunStageInput, []byte) ([]byte, error)

// decodeCodexStagePayload decodes only ordinary action data. Authority fields
// are derived by the backend from fresh roots and audit state; accepting them
// at this bridge would let a caller manufacture a receipt.
func decodeCodexStagePayload(action string, payload []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("codex stage %q payload is invalid JSON: %w", action, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("codex stage %q payload has trailing data: %w", action, err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("codex stage %q payload must be a JSON object", action)
	}
	allowed := map[string]struct{}{}
	switch action {
	case "decision":
		allowed = map[string]struct{}{"decision": {}, "options": {}, "checkpoint": {}}
	case "summary":
		allowed = map[string]struct{}{}
	case "answer":
		allowed = map[string]struct{}{"decision": {}, "answer": {}}
	case "review-complete":
		allowed = map[string]struct{}{"verdict": {}}
	case "learnings-persist":
		allowed = map[string]struct{}{"selections": {}}
	case "review-request", "run-sensors", "learnings-surface":
	default:
		return nil, fmt.Errorf("codex stage action %q is unsupported", action)
	}
	for key := range object {
		lower := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
		if _, forbidden := codexAuthorityPayloadFields[lower]; forbidden {
			return nil, fmt.Errorf("codex stage payload field %q is caller authority: %w", key, errCodexStageAuthority)
		}
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("codex stage %q payload field %q is unsupported", action, key)
		}
	}
	return object, nil
}

func codexLearningSelections(values map[string]any) ([]learnings.Selection, error) {
	raw, ok := values["selections"]
	if !ok {
		return nil, fmt.Errorf("learnings-persist payload requires %q", "selections")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("learnings-persist payload %q must be an array", "selections")
	}
	selections := make([]learnings.Selection, 0, len(items))
	for index, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("learning selection %d must be an object", index)
		}
		selection, err := decodeCodexLearningSelection(object)
		if err != nil {
			return nil, fmt.Errorf("learning selection %d: %w", index, err)
		}
		selections = append(selections, selection)
	}
	if err := learnings.ValidateSelections(selections); err != nil {
		return nil, err
	}
	return selections, nil
}

func decodeCodexLearningSelection(object map[string]any) (learnings.Selection, error) {
	for key := range object {
		lower := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
		if _, forbidden := codexAuthorityPayloadFields[lower]; forbidden {
			return learnings.Selection{}, fmt.Errorf("selection field %q is caller authority: %w", key, errCodexStageAuthority)
		}
	}
	selection := learnings.Selection{}
	for key, value := range object {
		readString := func() (string, error) {
			text, ok := value.(string)
			if !ok {
				return "", fmt.Errorf("field %q must be a string", key)
			}
			return text, nil
		}
		switch key {
		case "candidate_id":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.CandidateID = text
		case "type":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.Type = text
		case "scope":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.Scope = text
		case "heading":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.Heading = text
		case "text":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.Text = text
		case "source":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.Source = text
		case "origin_stage":
			text, err := readString()
			if err != nil {
				return learnings.Selection{}, err
			}
			selection.OriginStage = text
		case "manifest_fields":
			manifest, ok := value.(map[string]any)
			if !ok {
				return learnings.Selection{}, fmt.Errorf("field %q must be an object", key)
			}
			selection.ManifestFields = make(map[string]string, len(manifest))
			for manifestKey, manifestValue := range manifest {
				if manifestKey == "timeout_seconds" {
					number, ok := manifestValue.(json.Number)
					if !ok {
						return learnings.Selection{}, fmt.Errorf("manifest field %q must be an integer", manifestKey)
					}
					value, parseErr := strconv.ParseInt(string(number), 10, 64)
					if parseErr != nil || value < 0 {
						return learnings.Selection{}, fmt.Errorf("manifest field %q must be a nonnegative integer", manifestKey)
					}
					selection.ManifestFields[manifestKey] = strconv.FormatInt(value, 10)
					continue
				}
				text, ok := manifestValue.(string)
				if !ok {
					return learnings.Selection{}, fmt.Errorf("manifest field %q must be a string", manifestKey)
				}
				selection.ManifestFields[manifestKey] = text
			}
		default:
			return learnings.Selection{}, fmt.Errorf("selection field %q is unsupported", key)
		}
	}
	if selection.Source == "" {
		selection.Source = "orchestrator"
	}
	return selection, nil
}

var errCodexStageAuthority = errors.New("codex stage: caller authority field")

var codexAuthorityPayloadFields = map[string]struct{}{
	"timestamp": {}, "digest": {}, "fire_id": {}, "receipt": {},
	"artifact_snapshot": {}, "reviewer": {}, "iteration": {},
	"artifact_fingerprint": {}, "request_fingerprint": {}, "appendix_offset": {},
	"prior_digest": {}, "prior_length": {}, "post_fingerprint": {},
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

// codexStageAdapter is the hidden receiver bridge. It resolves the active
// identity and roots afresh for every invocation, then delegates a purpose-
// specific action without exposing a public CLI command or caller authority
// fields.
func codexStageAdapter(getwd func() (string, error), getenv func(string) string, dispatch codexStageDispatch) func(string, string) ([]byte, error) {
	return func(action, explicitDir string) ([]byte, error) {
		return codexStageAdapterCall(getwd, getenv, action, explicitDir, nil, func(ctx context.Context, action string, input deliverypkg.RunStageInput, _ []byte) ([]byte, error) {
			if dispatch == nil {
				return nil, errors.New("codex stage dispatch is unavailable")
			}
			return dispatch(ctx, action, input)
		})
	}
}

func codexStageAdapterWithPayload(getwd func() (string, error), getenv func(string) string, dispatch codexStageDispatchWithPayload) func(string, string, []byte) ([]byte, error) {
	return func(action, explicitDir string, payload []byte) (wire []byte, err error) {
		return codexStageAdapterCall(getwd, getenv, action, explicitDir, payload, dispatch)
	}
}

func codexStageAdapterCall(getwd func() (string, error), getenv func(string) string, action, explicitDir string, payload []byte, dispatch codexStageDispatchWithPayload) (wire []byte, err error) {
	input, projectRoot, recordRoot, resolveErr := deliveryInputResolver(getwd, getenv, explicitDir)
	if resolveErr != nil {
		return nil, resolveErr
	}
	defer func() {
		closeErr := errors.Join(
			wrapDeliveryRootCloseError("record", deliveryRootCloser(recordRoot)),
			wrapDeliveryRootCloseError("project", deliveryRootCloser(projectRoot)),
		)
		if closeErr == nil {
			return
		}
		wire = nil
		if err == nil {
			err = closeErr
			return
		}
		err = errors.Join(err, closeErr)
	}()
	if dispatch == nil {
		return nil, errors.New("codex stage dispatch is unavailable")
	}
	wire, err = dispatch(context.Background(), action, input, payload)
	if err != nil {
		return nil, err
	}
	if len(wire) == 0 || !json.Valid(wire) {
		return nil, fmt.Errorf("codex stage %q returned invalid JSON", action)
	}
	return append([]byte(nil), wire...), nil
}

func defaultCodexStageDispatch(ctx context.Context, action string, input deliverypkg.RunStageInput) ([]byte, error) {
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return nil, fmt.Errorf("codex stage action %q requires a resolved project", action)
	}
	return dispatchCodexStageAction(ctx, action, input, nil)
}

func defaultCodexStageDispatchWithPayload(ctx context.Context, action string, input deliverypkg.RunStageInput, payload []byte) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("codex stage context is nil")
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return nil, fmt.Errorf("codex stage action %q requires project and record roots", action)
	}
	values, err := decodeCodexStagePayload(action, payload)
	if err != nil {
		return nil, err
	}
	return dispatchCodexStageAction(ctx, action, input, values)
}

// dispatchCodexStageAction is the action-specific backend for the hidden
// receiver bridge. It resolves every authority-bearing value from the active
// state, graph, audit ledger, and fixed stage paths. Payload values are
// limited to ordinary question answers, review verdicts, and learning text.
func dispatchCodexStageAction(ctx context.Context, action string, input deliverypkg.RunStageInput, values map[string]any) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("codex stage context is nil")
	}
	stage, err := resolveCodexStage(ctx, input)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = map[string]any{}
	}
	if action == "review-request" || action == "review-complete" {
		effective, _, err := resolveCodexReviewPolicy(input, stage)
		if err != nil {
			return nil, err
		}
		if effective == graph.ReviewClassNone {
			return nil, errors.New("codex stage review is disabled by the effective review policy")
		}
	}
	switch action {
	case "decision":
		decisionID, err := codexPayloadString(values, "decision", true)
		if err != nil {
			return nil, err
		}
		checkpoint, err := codexPayloadString(values, "checkpoint", false)
		if err != nil {
			return nil, err
		}
		options, err := codexPayloadString(values, "options", false)
		if err != nil {
			return nil, err
		}
		if checkpoint == "summary-confirmation" {
			if decisionID != "summary" {
				return nil, fmt.Errorf("codex stage decision: summary checkpoint requires decision %q", "summary")
			}
			if err := audit.RecordIntentCaptureDecision(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, audit.IntentCaptureDecision{Stage: stage.Slug, DecisionID: "summary", Fingerprint: "derived", Options: options}); err != nil {
				return nil, err
			}
		} else if checkpoint != "" {
			return nil, fmt.Errorf("codex stage decision: unsupported checkpoint %q", checkpoint)
		} else if err := audit.RecordIntentCaptureDecisionFromQuestionsWithOptions(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, stage.Slug, decisionID, options); err != nil {
			return nil, err
		}
		return marshalCodexDecisionWire(stage.Slug, decisionID), nil
	case "summary":
		if err := audit.RecordIntentCaptureDecision(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, audit.IntentCaptureDecision{Stage: stage.Slug, DecisionID: "summary", Fingerprint: "derived"}); err != nil {
			return nil, err
		}
		return marshalCodexDecisionWire(stage.Slug, "summary"), nil
	case "answer":
		decisionID, err := codexPayloadString(values, "decision", true)
		if err != nil {
			return nil, err
		}
		answer, err := codexPayloadString(values, "answer", true)
		if err != nil {
			return nil, err
		}
		if decisionID == "summary" {
			err = audit.RecordSummaryConfirmation(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, audit.IntentCaptureDecision{Stage: stage.Slug, DecisionID: "summary", Fingerprint: "derived"}, answer, "")
		} else {
			err = audit.RecordIntentCaptureAnswerByID(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, stage.Slug, decisionID, answer)
		}
		if err != nil {
			return nil, err
		}
		return marshalCodexAnswerWire(stage.Slug, decisionID), nil
	case "review-request":
		return dispatchCodexReviewRequest(ctx, input, stage)
	case "review-complete":
		verdict, err := codexPayloadString(values, "verdict", true)
		if err != nil {
			return nil, err
		}
		return dispatchCodexReviewCompletion(ctx, input, stage, verdict)
	case "run-sensors":
		return dispatchCodexSensors(ctx, input, stage)
	case "learnings-surface":
		question, err := deriveCodexLearningQuestion(ctx, input, stage)
		if err != nil {
			return nil, err
		}
		return json.Marshal(struct {
			Kind       string                `json:"kind"`
			Stage      string                `json:"stage"`
			Space      string                `json:"space"`
			Intent     string                `json:"intent"`
			ID         string                `json:"id"`
			Generation uint64                `json:"generation"`
			Sensors    []string              `json:"sensors"`
			Prompt     string                `json:"prompt"`
			Candidates []learnings.Candidate `json:"candidates,omitempty"`
			Parked     []string              `json:"parked,omitempty"`
		}{Kind: "learnings-question", Stage: question.Stage, Space: question.Space, Intent: question.Intent, ID: question.ID, Generation: question.Generation, Sensors: question.SensorIDs, Prompt: question.Prompt, Candidates: question.Candidates, Parked: question.Parked})
	case "learnings-persist":
		return dispatchCodexLearningPersistence(ctx, input, stage, values)
	default:
		return nil, fmt.Errorf("codex stage action %q is unsupported", action)
	}
}

func resolveCodexReviewPolicy(input deliverypkg.RunStageInput, stage graph.Stage) (graph.ReviewClass, int, error) {
	document, err := state.ReadDocument(input.RecordRoot)
	if err != nil {
		return graph.ReviewClassNone, 0, fmt.Errorf("codex stage: read review policy state: %w", err)
	}
	declared := stage.ReviewClass
	if declared == "" {
		declared = graph.ReviewClassAdversarial
	}
	cap := scope.ReviewCapAdversarial
	scopesFS, err := fs.Sub(input.ProjectRoot.FS(), path.Join(".codex", "scopes"))
	if err == nil {
		metadata, readErr := scope.ReadAll(scopesFS)
		if readErr != nil {
			return graph.ReviewClassNone, 0, fmt.Errorf("codex stage: read review scope metadata: %w", readErr)
		}
		for _, item := range metadata {
			if item.Name == document.State.Scope() && item.ReviewCap != "" {
				cap = item.ReviewCap
				break
			}
		}
	}
	effective, maxIterations := scope.ResolveReviewPolicy(declared, cap, document.State.ReviewOverride(), stage.ReviewerMaxIterations)
	return effective, maxIterations, nil
}

func resolveCodexStage(ctx context.Context, input deliverypkg.RunStageInput) (graph.Stage, error) {
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return graph.Stage{}, fmt.Errorf("codex stage action requires project and record roots")
	}
	if err := validateCodexActiveSelection(input); err != nil {
		return graph.Stage{}, err
	}
	document, err := state.ReadDocument(input.RecordRoot)
	if err != nil {
		return graph.Stage{}, fmt.Errorf("codex stage: read state: %w", err)
	}
	catalog, err := graph.LoadFromRoot(input.ProjectRoot)
	if err != nil {
		return graph.Stage{}, fmt.Errorf("codex stage: load graph: %w", err)
	}
	stage, err := reportCurrentStage(document.State, catalog)
	if err != nil {
		return graph.Stage{}, err
	}
	if stage.Slug != "intent-capture" || stage.Phase != "ideation" || stage.Mode != "inline" {
		return graph.Stage{}, fmt.Errorf("codex stage: current stage %q is not supported by the inline intent-capture bridge", stage.Slug)
	}
	if document.State.WorkflowStatus() != state.WorkflowStatusRunning {
		return graph.Stage{}, fmt.Errorf("codex stage: workflow is not running")
	}
	return stage, nil
}

func validateCodexActiveSelection(input deliverypkg.RunStageInput) error {
	activeSpace := workspace.ActiveSpace(input.ProjectRoot.FS())
	if activeSpace != input.Identity.Space() {
		return fmt.Errorf("codex stage: active space %q does not match identity %q", activeSpace, input.Identity.Space())
	}
	intentsFS, err := fs.Sub(input.ProjectRoot.FS(), path.Join("aidlc", "spaces", activeSpace, "intents"))
	if err != nil {
		return fmt.Errorf("codex stage: open active intents: %w", err)
	}
	activeIntent, found := workspace.ActiveIntent(intentsFS, "")
	if !found || activeIntent != input.Identity.Intent() {
		return fmt.Errorf("codex stage: active intent %q does not match identity %q", activeIntent, input.Identity.Intent())
	}
	return nil
}

func codexPayloadString(values map[string]any, key string, required bool) (string, error) {
	value, present := values[key]
	if !present {
		if required {
			return "", fmt.Errorf("codex stage payload requires %q", key)
		}
		return "", nil
	}
	text, ok := value.(string)
	if !ok || (required && strings.TrimSpace(text) == "") {
		return "", fmt.Errorf("codex stage payload %q must be a nonempty string", key)
	}
	return text, nil
}

func marshalCodexDecisionWire(stage, decision string) []byte {
	wire, _ := json.Marshal(struct {
		Kind     string `json:"kind"`
		Stage    string `json:"stage"`
		Decision string `json:"decision"`
	}{Kind: "decision", Stage: stage, Decision: decision})
	return wire
}

func marshalCodexAnswerWire(stage, decision string) []byte {
	wire, _ := json.Marshal(struct {
		Kind     string `json:"kind"`
		Stage    string `json:"stage"`
		Decision string `json:"decision"`
	}{Kind: "answer", Stage: stage, Decision: decision})
	return wire
}

const maxCodexStageArtifactBytes = 8 << 20

func codexStageArtifactPath(stage graph.Stage, name string) string {
	return path.Join(stage.Phase, stage.Slug, artifact.Filename(name))
}

func readCodexStageArtifact(recordRoot *os.Root, name string) ([]byte, error) {
	if recordRoot == nil || name == "" || !fs.ValidPath(name) || path.IsAbs(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("codex stage artifact path is unsafe: %w", fs.ErrInvalid)
	}
	pathInfo, err := recordRoot.Lstat(name)
	if err != nil {
		return nil, err
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("codex stage artifact %q is not a regular file: %w", name, fs.ErrInvalid)
	}
	file, err := openCodexStageLeaf(recordRoot, name)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("codex stage artifact %q opened nil: %w", name, fs.ErrInvalid)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if opened == nil || !opened.Mode().IsRegular() || !os.SameFile(pathInfo, opened) {
		return nil, fmt.Errorf("codex stage artifact %q changed identity before read: %w", name, fs.ErrInvalid)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxCodexStageArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxCodexStageArtifactBytes || !utf8.Valid(content) {
		return nil, fmt.Errorf("codex stage artifact %q is oversized or invalid UTF-8: %w", name, fs.ErrInvalid)
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
		return nil, fmt.Errorf("codex stage artifact %q changed identity during read: %w", name, fs.ErrInvalid)
	}
	return content, nil
}

func readCodexAudit(ctx context.Context, input deliverypkg.RunStageInput) ([]audit.AuditRecord, error) {
	var records []audit.AuditRecord
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = audit.ReadEvents(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot)
		return err
	})
	return records, err
}

func dispatchCodexReviewRequest(ctx context.Context, input deliverypkg.RunStageInput, stage graph.Stage) ([]byte, error) {
	records, err := readCodexAudit(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("codex stage review request: read audit: %w", err)
	}
	if err := validateCodexReviewSummaryPrerequisite(input.RecordRoot, records, stage); err != nil {
		return nil, err
	}
	iteration, err := deriveNextReviewIteration(stage, records)
	if err != nil {
		return nil, err
	}
	artifactPath := codexStageArtifactPath(stage, stage.ReviewArtifact)
	content, err := readCodexStageArtifact(input.RecordRoot, artifactPath)
	if err != nil {
		return nil, fmt.Errorf("codex stage review request: read artifact: %w", err)
	}
	artifactSnapshots := map[string][]byte{artifactPath: content}
	if stage.Slug == "intent-capture" {
		for _, artifact := range []string{"intent-capture-questions", "intent-statement", "stakeholder-map"} {
			artifactName := codexStageArtifactPath(stage, artifact)
			if _, ok := artifactSnapshots[artifactName]; ok {
				continue
			}
			value, readErr := readCodexStageArtifact(input.RecordRoot, artifactName)
			if readErr != nil {
				return nil, fmt.Errorf("codex stage review request: read declared artifact %q: %w", artifactName, readErr)
			}
			artifactSnapshots[artifactName] = value
		}
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: stage.Slug, Reviewer: stage.Reviewer, Iteration: iteration, ArtifactPath: artifactPath,
		ArtifactSnapshot: content, ArtifactSnapshots: artifactSnapshots, RequestSource: "codex-inline-product-lead",
	})
	if err != nil {
		return nil, err
	}
	if err := audit.RecordReviewRequested(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, request); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Kind            string `json:"kind"`
		Stage           string `json:"stage"`
		Review          string `json:"review"`
		Artifact        string `json:"artifact"`
		AppendixOffset  int64  `json:"appendix_offset"`
		Iteration       int    `json:"iteration"`
		ReviewChallenge string `json:"review_challenge,omitempty"`
	}{Kind: "review-requested", Stage: stage.Slug, Review: "aidlc-product-lead-agent", Artifact: request.ArtifactPath, AppendixOffset: request.AppendixOffset, Iteration: request.Iteration, ReviewChallenge: request.ReviewChallenge})
}

func validateCodexReviewSummaryPrerequisite(recordRoot *os.Root, records []audit.AuditRecord, stage graph.Stage) error {
	if stage.Slug != "intent-capture" {
		return nil
	}
	floor := -1
	for index, record := range records {
		if record.Event == "STAGE_STARTED" && record.Fields["Stage"] == stage.Slug {
			floor = index
		}
	}
	for index := len(records) - 1; index > floor; index-- {
		record := records[index]
		if record.Event != "SUMMARY_CONFIRMATION_RECORDED" || record.Fields["Stage"] != stage.Slug {
			continue
		}
		if record.Fields["Details"] != "Looks correct" ||
			record.Fields["Checkpoint"] != "Consolidated Summary Confirmation" ||
			record.Fields["Questions File"] != "ideation/intent-capture/intent-capture-questions.md" ||
			record.Fields["Hash Scope"] != "confirmed-content-v1" || record.Fields["Questions SHA-256"] == "" {
			return fmt.Errorf("codex stage review request: summary confirmation receipt is malformed: %w", audit.ErrIntentCaptureStale)
		}
		if err := audit.ValidateSummaryConfirmationCurrent(recordRoot, record); err != nil {
			return fmt.Errorf("codex stage review request: summary confirmation is stale: %w", err)
		}
		return nil
	}
	return fmt.Errorf("codex stage review request: current summary confirmation is required: %w", audit.ErrIntentCaptureStale)
}

type codexReviewHistory struct {
	pending       *audit.AuditRecord
	nextIteration int
	recovery      bool
}

// reviewHistoryForStage validates one stage epoch and preserves its one
// canonical pending request, if any. Both review request and completion paths
// use this read model so completion cannot silently select the last of several
// pending or out-of-order receipts.
func reviewHistoryForStage(stage graph.Stage, records []audit.AuditRecord) (codexReviewHistory, error) {
	ordered := append([]audit.AuditRecord(nil), records...)
	for left := 0; left < len(ordered); left++ {
		for right := left + 1; right < len(ordered); right++ {
			if ordered[left].Timestamp.Equal(ordered[right].Timestamp) && ordered[left].Shard != ordered[right].Shard {
				return codexReviewHistory{}, fmt.Errorf("review iteration ordering is ambiguous: %w", audit.ErrIntentCaptureAmbiguous)
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
	epoch := -1
	for index, record := range ordered {
		if record.Event == "STAGE_STARTED" && record.Fields["Stage"] == stage.Slug {
			epoch = index
		}
	}
	next := 1
	recovery := false
	var pending *audit.AuditRecord
	max := stage.ReviewerMaxIterations
	if max <= 0 {
		max = 1
	}
	for index := epoch + 1; index < len(ordered); index++ {
		record := ordered[index]
		if record.Fields["Stage"] != stage.Slug {
			continue
		}
		switch record.Event {
		case "GATE_REJECTED", "STAGE_REVISING":
			if pending == nil && next > 1 {
				recovery = true
			}
		case "REVIEW_REQUESTED":
			if pending != nil {
				return codexReviewHistory{}, fmt.Errorf("review history has multiple pending requests: %w", audit.ErrIntentCaptureStale)
			}
			iteration, err := strconv.Atoi(record.Fields["Iteration"])
			if err != nil || iteration != next {
				return codexReviewHistory{}, fmt.Errorf("review iteration %q is not the next ordinal: %w", record.Fields["Iteration"], audit.ErrIntentCaptureStale)
			}
			if iteration > max {
				return codexReviewHistory{}, fmt.Errorf("review iteration %d exceeds stage maximum %d: %w", iteration, max, audit.ErrIntentCaptureStale)
			}
			if iteration > 1 && !recovery {
				return codexReviewHistory{}, fmt.Errorf("review iteration %d has no revision recovery boundary: %w", iteration, audit.ErrIntentCaptureStale)
			}
			copy := record
			pending = &copy
		case "REVIEW_COMPLETED":
			if pending == nil {
				return codexReviewHistory{}, fmt.Errorf("review completion has no pending request: %w", audit.ErrIntentCaptureStale)
			}
			if record.Fields["Iteration"] != pending.Fields["Iteration"] || record.Fields["Request Fingerprint"] == "" || record.Fields["Request Fingerprint"] != pending.Fields["Artifact Fingerprint"] {
				return codexReviewHistory{}, fmt.Errorf("review completion does not bind its request: %w", audit.ErrIntentCaptureStale)
			}
			pending = nil
			next++
		}
	}
	return codexReviewHistory{pending: pending, nextIteration: next, recovery: recovery}, nil
}

// deriveNextReviewIteration validates the current stage epoch's review history
// and derives the next backend-owned ordinal. A completed review cannot be
// requested again until a gate rejection/revision opens a bounded recovery
// attempt; a pending, duplicated, skipped, or over-budget ordinal fails
// closed.
func deriveNextReviewIteration(stage graph.Stage, records []audit.AuditRecord) (int, error) {
	history, err := reviewHistoryForStage(stage, records)
	if err != nil {
		return 0, err
	}
	if history.pending != nil {
		return 0, fmt.Errorf("review iteration has an incomplete request: %w", audit.ErrIntentCaptureStale)
	}
	next := history.nextIteration
	if next == 1 {
		return 1, nil
	}
	if !history.recovery {
		return 0, fmt.Errorf("review iteration is already complete without revision recovery: %w", audit.ErrIntentCaptureStale)
	}
	max := stage.ReviewerMaxIterations
	if max <= 0 {
		max = 1
	}
	if next > max {
		return 0, fmt.Errorf("review iteration %d exceeds stage maximum %d: %w", next, max, audit.ErrIntentCaptureStale)
	}
	return next, nil
}

func dispatchCodexReviewCompletion(ctx context.Context, input deliverypkg.RunStageInput, stage graph.Stage, verdict string) ([]byte, error) {
	records, err := readCodexAudit(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("codex stage review completion: read audit: %w", err)
	}
	request, err := latestCodexReviewRequest(input.RecordRoot, records, stage)
	if err != nil {
		return nil, err
	}
	content, err := readCodexStageArtifact(input.RecordRoot, request.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("codex stage review completion: read artifact: %w", err)
	}
	completion, err := review.NewCompletion(review.CompletionInput{Request: request, Verdict: verdict, ArtifactAfter: content})
	if err != nil {
		return nil, err
	}
	if err := audit.RecordReviewCompleted(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, request, completion); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Kind    string `json:"kind"`
		Stage   string `json:"stage"`
		Verdict string `json:"verdict"`
	}{Kind: "review-completed", Stage: stage.Slug, Verdict: verdict})
}

func latestCodexReviewRequest(recordRoot *os.Root, records []audit.AuditRecord, stage graph.Stage) (review.Request, error) {
	history, err := reviewHistoryForStage(stage, records)
	if err != nil {
		return review.Request{}, fmt.Errorf("codex stage review completion: validate request history: %w", err)
	}
	if history.pending == nil {
		return review.Request{}, fmt.Errorf("codex stage review completion: request receipt is missing: %w", audit.ErrIntentCaptureStale)
	}
	record := *history.pending
	if record.Fields["Reviewer"] != "" && record.Fields["Reviewer"] != stage.Reviewer {
		return review.Request{}, fmt.Errorf("codex stage review request reviewer is stale: %w", review.ErrInvalidRequest)
	}
	for _, legacy := range []string{"Artifact Path", "Request Source", "Prior Digest", "Prior Length", "Post Fingerprint", "Artifact Set"} {
		if _, ok := record.Fields[legacy]; ok {
			return review.Request{}, fmt.Errorf("codex stage review request uses legacy field %q: %w", legacy, review.ErrInvalidRequest)
		}
	}
	iteration, err := strconv.Atoi(record.Fields["Iteration"])
	if err != nil || iteration < 1 {
		return review.Request{}, fmt.Errorf("codex stage review request iteration is invalid: %w", review.ErrInvalidRequest)
	}
	priorLength, err := strconv.ParseInt(record.Fields["Review Appendix Prior Length"], 10, 64)
	if err != nil || priorLength < 0 {
		return review.Request{}, fmt.Errorf("codex stage review request prior length is invalid: %w", review.ErrInvalidRequest)
	}
	appendixOffset, err := strconv.ParseInt(record.Fields["Review Appendix Offset"], 10, 64)
	if err != nil || appendixOffset < 0 {
		return review.Request{}, fmt.Errorf("codex stage review request appendix offset is invalid: %w", review.ErrInvalidRequest)
	}
	artifactPath := record.Fields["Review Appendix Artifact"]
	if artifactPath == "" || record.Fields["Artifact Fingerprint"] == "" {
		return review.Request{}, fmt.Errorf("codex stage review request canonical binding is incomplete: %w", review.ErrInvalidRequest)
	}
	content, err := readCodexStageArtifact(recordRoot, artifactPath)
	if err != nil {
		return review.Request{}, fmt.Errorf("codex stage review request artifact is unavailable: %w", err)
	}
	artifactSnapshots := map[string][]byte{artifactPath: content}
	if stage.Slug == "intent-capture" {
		for _, artifact := range []string{"intent-capture-questions", "intent-statement", "stakeholder-map"} {
			artifactName := codexStageArtifactPath(stage, artifact)
			if artifactName == artifactPath {
				continue
			}
			value, readErr := readCodexStageArtifact(recordRoot, artifactName)
			if readErr != nil {
				return review.Request{}, fmt.Errorf("codex stage review request declared artifact is unavailable: %w", readErr)
			}
			artifactSnapshots[artifactName] = value
		}
	}
	priorDigest := record.Fields["Review Appendix Prior Digest"]
	challenge := record.Fields["Review Challenge"]
	var request review.Request
	if priorLength > 0 {
		// The conductor is allowed to remove the old terminal appendix after
		// this request receipt is written. Rehydrate the request from the
		// canonical binding and let review.ValidateRequest enforce the
		// prefix/fingerprint/challenge contract.
		request = review.Request{
			Stage: stage.Slug, Reviewer: stage.Reviewer, Iteration: iteration,
			ArtifactPath: artifactPath, ArtifactSnapshot: content,
			ArtifactSnapshots: artifactSnapshots, ArtifactFingerprint: record.Fields["Artifact Fingerprint"],
			PriorDigest: priorDigest, PriorLength: priorLength, ReviewChallenge: challenge,
			AppendixOffset: appendixOffset, PriorAppendixRemoved: true,
		}
		request.RequestFingerprint = request.ArtifactFingerprint
	} else {
		var newErr error
		requestSnapshot := content
		if appendixOffset < 0 || appendixOffset > int64(len(requestSnapshot)) {
			return review.Request{}, fmt.Errorf("codex stage review request appendix offset is outside artifact: %w", review.ErrArtifactChanged)
		}
		// The initial request receipt binds the artifact prefix before the
		// reviewer appends its terminal appendix. Reconstruct that exact
		// request-time snapshot instead of letting NewRequest interpret the
		// newly appended terminal as a revision request.
		requestSnapshot = append([]byte(nil), requestSnapshot[:appendixOffset]...)
		artifactSnapshots[artifactPath] = requestSnapshot
		request, newErr = review.NewRequest(review.RequestInput{
			Stage: stage.Slug, Reviewer: stage.Reviewer, Iteration: iteration,
			ArtifactPath: artifactPath, ArtifactSnapshot: requestSnapshot,
			ArtifactSnapshots: artifactSnapshots,
		})
		if newErr != nil {
			return review.Request{}, newErr
		}
		if request.AppendixOffset != appendixOffset || request.PriorDigest != priorDigest || request.PriorLength != priorLength || request.ArtifactFingerprint != record.Fields["Artifact Fingerprint"] || challenge != "" {
			return review.Request{}, fmt.Errorf("codex stage review request fingerprint is stale: %w", review.ErrArtifactChanged)
		}
	}
	if err := review.ValidateRequest(request); err != nil {
		return review.Request{}, fmt.Errorf("codex stage review request binding is invalid: %w", err)
	}
	return request, nil
}

func dispatchCodexSensors(ctx context.Context, input deliverypkg.RunStageInput, stage graph.Stage) ([]byte, error) {
	inputData, err := sensor.BuildIntentCaptureInput(input.ProjectRoot, input.RecordRoot, stage)
	if err != nil {
		return nil, err
	}
	invocations := sensor.RunAll(ctx, inputData, nil)
	results := make([]codexSensorResult, 0, len(invocations))
	for _, invocation := range invocations {
		// Advisory observations must not turn a local sensor or ledger problem
		// into a workflow failure. Each invocation is attempted independently.
		_ = audit.RecordSensorInvocation(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, invocation)
		fired := invocation.FireResult()
		terminal := invocation.TerminalResult()
		result := codexSensorResult{
			Sensor: fired.Sensor, FireID: fired.FireID(), Status: fired.Status,
			OutputPath: fired.OutputPath, Terminal: invocation.HasTerminal(),
		}
		if invocation.HasTerminal() {
			result.Status = terminal.Status
			result.Detail = terminal.Detail
			result.Note = terminal.Note
			result.DurationMS = terminal.DurationMS
			result.DetailPath = terminal.DetailPath
			result.FindingsCount = terminal.FindingsCount
			result.Findings = terminal.Findings
			if result.Status == sensor.StatusFailed && result.DetailPath != "" {
				if _, statErr := input.RecordRoot.Stat(result.DetailPath); statErr != nil {
					// The audit writer normalizes an advisory failure whose detail
					// file could not be persisted. Keep the wire from advertising a
					// nonexistent diagnostic path as if it were durable evidence.
					result.Status = sensor.StatusCompleted
					result.Note = "script-error: detail-write-failed: " + statErr.Error()
					result.Detail = ""
					result.DetailPath = ""
					result.FindingsCount = 0
					result.Findings = nil
				}
			}
		}
		results = append(results, result)
	}
	return json.Marshal(struct {
		Kind    string              `json:"kind"`
		Stage   string              `json:"stage"`
		Results []codexSensorResult `json:"results"`
	}{Kind: "sensor-results", Stage: stage.Slug, Results: results})
}

func codexSensorConsumes(consumes []graph.Consume) []string {
	values := make([]string, 0, len(consumes))
	for _, consume := range consumes {
		if artifact := strings.TrimSpace(consume.Artifact); artifact != "" {
			values = append(values, artifact)
		}
	}
	return values
}

type codexSensorResult struct {
	Sensor        string           `json:"sensor"`
	FireID        string           `json:"fire_id"`
	Status        sensor.Status    `json:"status"`
	OutputPath    string           `json:"output_path,omitempty"`
	Detail        string           `json:"detail,omitempty"`
	Note          string           `json:"note,omitempty"`
	DurationMS    int64            `json:"duration_ms,omitempty"`
	DetailPath    string           `json:"detail_path,omitempty"`
	FindingsCount int              `json:"findings_count,omitempty"`
	Findings      []sensor.Finding `json:"findings,omitempty"`
	Terminal      bool             `json:"terminal"`
}

func deriveCodexLearningQuestion(ctx context.Context, input deliverypkg.RunStageInput, stage graph.Stage) (learnings.Question, error) {
	return audit.RecordIntentCaptureLearningDecision(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, stage.Slug)
}

func dispatchCodexLearningPersistence(ctx context.Context, input deliverypkg.RunStageInput, stage graph.Stage, values map[string]any) ([]byte, error) {
	question, err := audit.ResolveIntentCaptureLearningDecision(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, stage.Slug)
	if err != nil {
		return nil, err
	}
	selections, err := codexLearningSelections(values)
	if err != nil {
		return nil, err
	}
	answer := learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true, Selections: selections}
	if err := audit.RecordIntentCaptureLearning(ctx, input.Identity, input.ProjectRoot, input.RecordRoot, question, answer); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Kind       string `json:"kind"`
		Stage      string `json:"stage"`
		Generation uint64 `json:"generation"`
		Selection  string `json:"selection"`
	}{Kind: "learning-recorded", Stage: stage.Slug, Generation: question.Generation, Selection: learningSelectionsSummary(selections)})
}

func learningSelectionsSummary(selections []learnings.Selection) string {
	if len(selections) == 0 {
		return "none"
	}
	hasLearning, hasSensor := false, false
	for _, selection := range selections {
		hasLearning = hasLearning || selection.Type == learnings.SelectionTypeLearning
		hasSensor = hasSensor || selection.Type == learnings.SelectionTypeSensor
	}
	switch {
	case hasLearning && hasSensor:
		return "learning,sensor"
	case hasLearning:
		return "learning"
	default:
		return "sensor"
	}
}

func learningSelection(rule, proposedSensor string) string {
	switch {
	case strings.TrimSpace(rule) != "" && strings.TrimSpace(proposedSensor) != "":
		return "rule,sensor"
	case strings.TrimSpace(rule) != "":
		return "rule"
	case strings.TrimSpace(proposedSensor) != "":
		return "sensor"
	default:
		return "none"
	}
}
