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

	if isUnsupportedPerUnitStage(input.Current) {
		return completionBlocked(CompletionBlockerPerUnit, fmt.Sprintf("stage %q uses unsupported per-unit artifact placement", input.Current.Slug))
	}
	if input.Current.Slug == "reverse-engineering" {
		return completionBlocked(CompletionBlockerCodeKB, fmt.Sprintf("stage %q writes unsupported CodeKB artifacts", input.Current.Slug))
	}
	if input.Current.ProducesKinds != nil {
		return completionBlocked(CompletionBlockerPerUnit, fmt.Sprintf("stage %q declares unsupported per-kind artifact applicability", input.Current.Slug))
	}

	present, err := artifact.HasRequiredOutput(input.RecordFS, input.Current)
	if err != nil {
		return completionBlocked(CompletionBlockerArtifact, fmt.Sprintf("artifact evidence could not be checked: %v", err))
	}
	if !present {
		return completionBlocked(CompletionBlockerArtifact, fmt.Sprintf("stage %q has no required artifact", input.Current.Slug))
	}
	if input.Current.WorkspaceRequires {
		return completionBlocked(CompletionBlockerWorkspace, fmt.Sprintf("stage %q requires unsupported workspace source evidence", input.Current.Slug))
	}

	switch input.Current.SummaryConfirmation {
	case "":
		// This stage has no summary confirmation policy.
	case "required":
		if !input.Evidence.SummaryConfirmed {
			return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q requires confirmed summary evidence", input.Current.Slug))
		}
	case "if-present":
		if input.Evidence.SummaryPresent && !input.Evidence.SummaryConfirmed {
			return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q has an unconfirmed summary", input.Current.Slug))
		}
	default:
		return completionBlocked(CompletionBlockerSummary, fmt.Sprintf("stage %q has an invalid summary confirmation policy %q", input.Current.Slug, input.Current.SummaryConfirmation))
	}

	if input.Current.Mode == "pipeline" && !input.Evidence.PipelineLinked {
		return completionBlocked(CompletionBlockerPipeline, fmt.Sprintf("stage %q has no recorded pipeline handoff evidence", input.Current.Slug))
	}
	if input.Current.Reviewer != "" && !input.Evidence.ReviewReceipt {
		return completionBlocked(CompletionBlockerReview, fmt.Sprintf("stage %q has no reviewer receipt", input.Current.Slug))
	}
	if len(input.Current.Sensors) > 0 {
		if !input.Evidence.SensorsPassed {
			return completionBlocked(CompletionBlockerSensor, fmt.Sprintf("stage %q has unsatisfied sensor evidence", input.Current.Slug))
		}
		if !input.Evidence.BlockingSensorsClear {
			return completionBlocked(CompletionBlockerBlocking, fmt.Sprintf("stage %q has uncleared blocking sensor evidence", input.Current.Slug))
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
	if len(current.ProducesKinds) != len(selected.ProducesKinds) {
		return false
	}
	for name, kinds := range current.ProducesKinds {
		if !slices.Equal(kinds, selected.ProducesKinds[name]) {
			return false
		}
	}
	return true
}
