package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/state"
)

var (
	reportInputResolver = func(getwd func() (string, error), getenv func(string) string, explicitDir string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliveryInputResolver(getwd, getenv, explicitDir)
	}
	reportRootCloser = func(root *os.Root) error { return deliveryRootCloser(root) }
)

// reportAdapter is the callback boundary used by the public CLI. It resolves
// the active identity and roots for every invocation, then reads the graph and
// state immediately before handing one immutable selection to orchestrator.
func reportAdapter(
	getwd func() (string, error),
	getenv func(string) string,
	report func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error),
) func(stage, result, userInput, reason, explicitDir string) ([]byte, error) {
	return func(stage, result, userInput, reason, explicitDir string) (wire []byte, err error) {
		if reportInputResolver == nil {
			return nil, errors.New("report input resolver is unavailable")
		}
		input, projectRoot, recordRoot, resolveErr := reportInputResolver(getwd, getenv, explicitDir)
		if resolveErr != nil {
			return nil, resolveErr
		}
		defer func() {
			recordCloseErr := closeReportRoot("record", recordRoot)
			projectCloseErr := closeReportRoot("project", projectRoot)
			closeErr := errors.Join(recordCloseErr, projectCloseErr)
			if closeErr == nil {
				return
			}
			wire = nil
			if err == nil {
				err = closeErr
				return
			}
			err = fmt.Errorf("report operation failed during root cleanup: %v; %w", err, closeErr)
		}()

		if projectRoot == nil || recordRoot == nil {
			return nil, errors.New("report: project and record roots are required")
		}
		if report == nil {
			return nil, errors.New("report operation is unavailable")
		}
		kind, kindErr := parseReportKind(result)
		if kindErr != nil {
			return nil, kindErr
		}
		document, readErr := state.ReadDocument(recordRoot)
		if readErr != nil {
			return nil, fmt.Errorf("report: read state: %w", readErr)
		}
		dataPath := filepath.ToSlash(filepath.Join(".codex", "tools", "data"))
		dataFS, subErr := fs.Sub(projectRoot.FS(), dataPath)
		if subErr != nil {
			return nil, fmt.Errorf("report: open graph data %q: %w", dataPath, subErr)
		}
		catalog, loadErr := graph.Load(dataFS)
		if loadErr != nil {
			return nil, fmt.Errorf("report: load graph: %w", loadErr)
		}
		current, currentErr := reportCurrentStage(document.State, catalog)
		if currentErr != nil {
			return nil, currentErr
		}
		if stage != document.State.CurrentStage() {
			return nil, orchestrator.NewWorkflowError(
				fmt.Sprintf("requested stage %q does not match current stage %q", stage, document.State.CurrentStage()),
				orchestrator.ErrInvalidReport,
			)
		}
		resultState, reportErr := report(context.Background(), orchestrator.ReportInput{
			Identity:    input.Identity,
			ProjectRoot: projectRoot,
			RecordRoot:  recordRoot,
			Kind:        kind,
			Slug:        stage,
			Current:     current,
			Catalog:     catalog,
			Choice:      userInput,
			Feedback:    reason,
		})
		if reportErr != nil {
			return nil, classifyReportAdapterError(reportErr)
		}
		if resultState.Kind != kind || resultState.Slug != stage {
			return nil, fmt.Errorf("report callback returned mismatched result (%q, %q)", resultState.Kind, resultState.Slug)
		}
		return marshalReportResult(resultState)
	}
}

func closeReportRoot(name string, root *os.Root) error {
	if root == nil {
		return nil
	}
	if reportRootCloser == nil {
		return fmt.Errorf("close %s root: closer is unavailable", name)
	}
	return wrapDeliveryRootCloseError(name, reportRootCloser(root))
}

func parseReportKind(value string) (orchestrator.ReportKind, error) {
	switch value {
	case string(orchestrator.ReportKindAwaitingApproval):
		return orchestrator.ReportKindAwaitingApproval, nil
	case string(orchestrator.ReportKindRejected):
		return orchestrator.ReportKindRejected, nil
	case string(orchestrator.ReportKindRevised):
		return orchestrator.ReportKindRevised, nil
	case string(orchestrator.ReportKindApproved):
		return orchestrator.ReportKindApproved, nil
	default:
		return "", fmt.Errorf("report result %q is unsupported", value)
	}
}

func reportCurrentStage(current state.State, catalog graph.Snapshot) (graph.Stage, error) {
	var selected graph.Stage
	found := false
	for _, candidate := range catalog.Stages() {
		if candidate.Slug != current.CurrentStage() {
			continue
		}
		if found {
			return graph.Stage{}, orchestrator.NewWorkflowError(
				fmt.Sprintf("current stage %q is duplicated in the graph", current.CurrentStage()),
				orchestrator.ErrStateCatalogMismatch,
			)
		}
		selected = candidate
		found = true
	}
	if !found {
		return graph.Stage{}, orchestrator.NewWorkflowError(
			fmt.Sprintf("current stage %q is absent from the graph", current.CurrentStage()),
			orchestrator.ErrStateCatalogMismatch,
		)
	}
	return selected, nil
}

func classifyReportAdapterError(err error) error {
	if err == nil || orchestrator.IsWorkflowError(err) {
		return err
	}
	for _, cause := range []error{
		orchestrator.ErrInvalidReport,
		orchestrator.ErrInvalidState,
		orchestrator.ErrUnsupportedState,
		orchestrator.ErrStateCatalogMismatch,
		orchestrator.ErrInvalidGate,
		orchestrator.ErrUnsupportedGate,
		orchestrator.ErrGateNotReady,
		orchestrator.ErrStaleHumanTurn,
		orchestrator.ErrInvalidDecision,
		orchestrator.ErrInvalidFeedback,
		orchestrator.ErrSelfAttributedDecision,
	} {
		if errors.Is(err, cause) {
			return orchestrator.NewWorkflowError(err.Error(), err)
		}
	}
	return err
}

func marshalReportResult(result orchestrator.ReportResult) ([]byte, error) {
	switch result.Kind {
	case orchestrator.ReportKindAwaitingApproval, orchestrator.ReportKindRejected, orchestrator.ReportKindRevised:
		message := fmt.Sprintf("Recorded %s for %q.", result.Kind, result.Slug)
		if result.Kind == orchestrator.ReportKindAwaitingApproval && result.Gate.AlreadyAwaiting {
			message = fmt.Sprintf("Stage %q is already awaiting approval; gate evidence revalidated.", result.Slug)
		}
		return json.Marshal(struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}{Kind: "print", Message: message})
	case orchestrator.ReportKindApproved:
		return json.Marshal(struct {
			Kind   string `json:"kind"`
			Reason string `json:"reason"`
		}{
			Kind:   "done",
			Reason: fmt.Sprintf("Committed approve for %q. State advanced; run next to continue.", result.Slug),
		})
	default:
		return nil, fmt.Errorf("report callback returned unsupported result kind %q", result.Kind)
	}
}
