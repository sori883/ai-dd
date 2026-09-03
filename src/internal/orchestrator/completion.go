package orchestrator

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
)

// CompletionBlocker identifies the first completion condition that is not
// satisfied. The empty value is reserved for a successful decision.
type CompletionBlocker string

const (
	CompletionBlockerNone          CompletionBlocker = ""
	CompletionBlockerInvalidInput  CompletionBlocker = "invalid-input"
	CompletionBlockerStageMismatch CompletionBlocker = "stage-mismatch"
	CompletionBlockerMode          CompletionBlocker = "mode"
	CompletionBlockerArtifact      CompletionBlocker = "artifact"
	CompletionBlockerSummary       CompletionBlocker = "summary"
	CompletionBlockerPipeline      CompletionBlocker = "pipeline"
	CompletionBlockerReview        CompletionBlocker = "review"
	CompletionBlockerSensor        CompletionBlocker = "sensor"
	CompletionBlockerBlocking      CompletionBlocker = "blocking"
	CompletionBlockerPerUnit       CompletionBlocker = "per-unit"
	CompletionBlockerCodeKB        CompletionBlocker = "codekb"
	CompletionBlockerWorkspace     CompletionBlocker = "workspace"
)

// CompletionEvidence contains caller-owned evidence for the completion gates.
// Evidence is never written or enriched by the evaluator.
type CompletionEvidence struct {
	SummaryPresent       bool
	SummaryConfirmed     bool
	PipelineLinked       bool
	ReviewReceipt        bool
	SensorsPassed        bool
	BlockingSensorsClear bool
}

// CompletionInput is the read-only input to EvaluateStageCompletion. Current
// is the stage selected by the caller, while Catalog is the graph snapshot
// that authorizes its completion metadata. RecordFS is borrowed and remains
// owned by the caller.
type CompletionInput struct {
	Current  graph.Stage
	Catalog  graph.Snapshot
	RecordFS fs.FS
	Evidence CompletionEvidence
}

// CompletionDecision is the read-only result of one completion evaluation.
// Its zero value is deliberately not ready.
type CompletionDecision struct {
	Ready   bool
	Blocker CompletionBlocker
	Reason  string
}

// EvaluateStageCompletion checks the first unmet completion condition in the
// fixed order artifact, summary, pipeline, review, sensor, and blocking. It
// performs no filesystem or state mutation; a failed read-only check is
// represented as a non-ready decision with a reason.
func EvaluateStageCompletion(input CompletionInput) CompletionDecision {
	if input.Current.Slug == "" || input.Current.Phase == "" {
		return completionBlocked(CompletionBlockerInvalidInput, "current stage requires a slug and phase")
	}

	selected, found := completionCatalogStage(input.Catalog, input.Current.Slug)
	if !found {
		return completionBlocked(CompletionBlockerStageMismatch, fmt.Sprintf("current stage %q is absent from the enabled graph", input.Current.Slug))
	}
	if !sameCompletionStage(input.Current, selected) {
		return completionBlocked(CompletionBlockerStageMismatch, fmt.Sprintf("current stage %q does not match graph metadata", input.Current.Slug))
	}

	stage := selected
	if stage.Mode == "agent-team" {
		return completionBlocked(CompletionBlockerMode, fmt.Sprintf("stage %q requires unsupported agent-team dispatcher", stage.Slug))
	}
	if isUnsupportedPerUnitStage(stage) {
		return completionBlocked(CompletionBlockerPerUnit, fmt.Sprintf("stage %q uses unsupported per-unit artifact placement", stage.Slug))
	}
	if stage.Slug == "reverse-engineering" {
		return completionBlocked(CompletionBlockerCodeKB, fmt.Sprintf("stage %q writes unsupported CodeKB artifacts", stage.Slug))
	}
	if stage.ProducesKinds != nil {
		return completionBlocked(CompletionBlockerPerUnit, fmt.Sprintf("stage %q declares unsupported per-kind artifact applicability", stage.Slug))
	}

	present, err := artifact.HasRequiredOutput(input.RecordFS, stage)
	if err != nil {
		return completionBlocked(CompletionBlockerArtifact, fmt.Sprintf("artifact evidence for %q could not be checked: %v", stage.Slug, err))
	}
	if !present {
		return completionBlocked(CompletionBlockerArtifact, fmt.Sprintf("stage %q has no required artifact", stage.Slug))
	}
	if stage.WorkspaceRequires {
		return completionBlocked(CompletionBlockerWorkspace, fmt.Sprintf("stage %q requires unsupported workspace source evidence", stage.Slug))
	}

	switch stage.SummaryConfirmation {
	case "":
		// This stage has no summary confirmation policy.
	case "required":
		if !input.Evidence.SummaryConfirmed {
			return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q requires confirmed summary evidence", stage.Slug))
		}
	case "if-present":
		if input.Evidence.SummaryPresent && !input.Evidence.SummaryConfirmed {
			return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q has an unconfirmed summary", stage.Slug))
		}
	default:
		return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q has an invalid summary confirmation policy %q", stage.Slug, stage.SummaryConfirmation))
	}

	if stage.Mode == "pipeline" && !input.Evidence.PipelineLinked {
		return completionBlocked(CompletionBlockerPipeline, fmt.Sprintf("stage %q has no recorded pipeline handoff evidence", stage.Slug))
	}
	if stage.Reviewer != "" && !input.Evidence.ReviewReceipt {
		return completionBlocked(CompletionBlockerReview, fmt.Sprintf("stage %q has no reviewer receipt", stage.Slug))
	}
	if len(stage.Sensors) > 0 {
		if !input.Evidence.SensorsPassed {
			return completionBlocked(CompletionBlockerSensor, fmt.Sprintf("stage %q has unsatisfied sensor evidence", stage.Slug))
		}
		if !input.Evidence.BlockingSensorsClear {
			return completionBlocked(CompletionBlockerBlocking, fmt.Sprintf("stage %q has uncleared blocking sensor evidence", stage.Slug))
		}
	}

	return CompletionDecision{
		Ready:   true,
		Blocker: CompletionBlockerNone,
		Reason:  "all completion conditions are satisfied",
	}
}

func isUnsupportedPerUnitStage(stage graph.Stage) bool {
	if stage.ForEach != "" {
		return true
	}
	switch stage.Slug {
	case "nfr-requirements", "nfr-design", "functional-design", "infrastructure-design", "code-generation":
		return true
	default:
		return false
	}
}

func completionBlocked(blocker CompletionBlocker, reason string) CompletionDecision {
	return CompletionDecision{Blocker: blocker, Reason: reason}
}

func completionCatalogStage(catalog graph.Snapshot, slug string) (graph.Stage, bool) {
	var selected graph.Stage
	found := false
	for _, stage := range catalog.Stages() {
		if stage.Slug != slug {
			continue
		}
		if found {
			return graph.Stage{}, false
		}
		selected = stage
		found = true
	}
	return selected, found
}

func sameCompletionStage(current, selected graph.Stage) bool {
	if current.Slug != selected.Slug ||
		current.Number != selected.Number ||
		current.Name != selected.Name ||
		current.Phase != selected.Phase ||
		current.Execution != selected.Execution ||
		current.LeadAgent != selected.LeadAgent ||
		current.Mode != selected.Mode ||
		current.Enabled != selected.Enabled ||
		current.ForEach != selected.ForEach ||
		current.WorkspaceRequires != selected.WorkspaceRequires ||
		current.Reviewer != selected.Reviewer ||
		current.SummaryConfirmation != selected.SummaryConfirmation {
		return false
	}
	if !slices.Equal(current.SupportAgents, selected.SupportAgents) ||
		!slices.Equal(current.Scopes, selected.Scopes) ||
		!slices.Equal(current.Sensors, selected.Sensors) ||
		!slices.Equal(current.Produces, selected.Produces) ||
		!slices.Equal(current.OptionalProduces, selected.OptionalProduces) ||
		!slices.Equal(current.RequiresStages, selected.RequiresStages) {
		return false
	}
	if len(current.Consumes) != len(selected.Consumes) {
		return false
	}
	for index := range current.Consumes {
		if current.Consumes[index] != selected.Consumes[index] {
			return false
		}
	}
	if (current.ProducesKinds == nil) != (selected.ProducesKinds == nil) {
		return false
	}
	if len(current.ProducesKinds) != len(selected.ProducesKinds) {
		return false
	}
	for name, kinds := range current.ProducesKinds {
		selectedKinds, ok := selected.ProducesKinds[name]
		if !ok || !strictStringSlicesEqual(kinds, selectedKinds) {
			return false
		}
	}
	return true
}

func strictStringSlicesEqual(left, right []string) bool {
	return (left == nil) == (right == nil) && slices.Equal(left, right)
}
