package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

var (
	ErrInvalidReport  = errors.New("orchestrator: invalid report")
	ErrReportWorkflow = errors.New("orchestrator: report workflow rejected")
)

// WorkflowError marks a syntactically valid report whose requested lifecycle
// transition was rejected by the current workflow state. Public adapters may
// render this error as a terminal directive while preserving its cause for
// diagnostics and tests.
type WorkflowError struct {
	Message string
	Cause   error
}

func (err *WorkflowError) Error() string { return err.Message }
func (err *WorkflowError) Unwrap() error { return err.Cause }

// IsWorkflowError reports whether err is a typed workflow rejection.
func IsWorkflowError(err error) bool {
	if recordlock.IsReleaseError(err) || hasCompositeCauseOutsideWorkflow(err) {
		return false
	}
	var workflowErr *WorkflowError
	return errors.As(err, &workflowErr)
}

func hasCompositeCauseOutsideWorkflow(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*WorkflowError); ok {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) > 1 {
			return true
		}
		if len(causes) == 1 {
			return hasCompositeCauseOutsideWorkflow(causes[0])
		}
		return false
	}
	wrapped, ok := err.(interface{ Unwrap() error })
	return ok && hasCompositeCauseOutsideWorkflow(wrapped.Unwrap())
}

// NewWorkflowError wraps a workflow rejection without converting internal I/O
// failures into a user-visible terminal directive.
func NewWorkflowError(message string, cause error) error {
	if cause == nil {
		cause = ErrReportWorkflow
	} else if !errors.Is(cause, ErrReportWorkflow) {
		cause = errors.Join(ErrReportWorkflow, cause)
	}
	return &WorkflowError{Message: message, Cause: cause}
}

// ReportKind identifies one explicit lifecycle result. No free-form result or
// completion alias is accepted by Report.
type ReportKind string

const (
	ReportKindAwaitingApproval ReportKind = "awaiting-approval"
	ReportKindRejected         ReportKind = "rejected"
	ReportKindRevised          ReportKind = "revised"
	ReportKindApproved         ReportKind = "approved"
)

// ReportInput identifies the stage result and the identity-bound roots used by
// the delegated gate operation. Slug and Current.Slug are both required and
// must match, so an old or incomplete report cannot silently target a
// different stage.
type ReportInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
	Kind        ReportKind
	Slug        string
	Current     graph.Stage
	Catalog     graph.Snapshot
	Choice      string
	Feedback    string
}

// ReportResult preserves the delegated operation and its durable result. For
// an approval, Approval retains PR6's partial-transaction flags and state;
// Gate is populated for the other three report kinds.
type ReportResult struct {
	Kind     ReportKind
	Slug     string
	Gate     GateResult
	Approval ApproveResult
}

type reportOps struct {
	openGate    func(context.Context, GateInput) (GateResult, error)
	rejectGate  func(context.Context, GateInput) (GateResult, error)
	reviseGate  func(context.Context, GateInput) (GateResult, error)
	approveGate func(context.Context, ApproveInput) (ApproveResult, error)
}

func systemReportOps() reportOps {
	return reportOps{
		openGate:    OpenGate,
		rejectGate:  RejectGate,
		reviseGate:  ReviseGate,
		approveGate: ApproveGate,
	}
}

// Report dispatches one explicit stage result to the corresponding gate
// transaction. The delegated operation owns the record lock; Report does not
// wrap it in another lock or attempt a later advance after approval.
func Report(ctx context.Context, input ReportInput) (ReportResult, error) {
	return reportWithOps(ctx, input, systemReportOps())
}

func reportWithOps(ctx context.Context, input ReportInput, ops reportOps) (result ReportResult, err error) {
	if err := validateReportInput(ctx, input); err != nil {
		return ReportResult{}, err
	}
	result = ReportResult{Kind: input.Kind, Slug: input.Slug}
	gateInput := GateInput{
		Identity:    input.Identity,
		ProjectRoot: input.ProjectRoot,
		RecordRoot:  input.RecordRoot,
		Current:     input.Current,
		Catalog:     input.Catalog,
		Choice:      input.Choice,
		Feedback:    input.Feedback,
	}
	switch input.Kind {
	case ReportKindAwaitingApproval:
		result.Gate, err = ops.openGate(ctx, gateInput)
	case ReportKindRejected:
		result.Gate, err = ops.rejectGate(ctx, gateInput)
	case ReportKindRevised:
		result.Gate, err = ops.reviseGate(ctx, gateInput)
	case ReportKindApproved:
		result.Approval, err = ops.approveGate(ctx, ApproveInput{
			Identity:    input.Identity,
			ProjectRoot: input.ProjectRoot,
			RecordRoot:  input.RecordRoot,
			Current:     input.Current,
			Catalog:     input.Catalog,
			Choice:      input.Choice,
		})
	default:
		// validateReportInput handles this branch; keep the switch closed if a
		// new kind is added without a corresponding transaction.
		return ReportResult{}, fmt.Errorf("report kind %q is unsupported: %w", input.Kind, ErrInvalidReport)
	}
	return result, err
}

func validateReportInput(ctx context.Context, input ReportInput) error {
	if ctx == nil {
		return fmt.Errorf("report: nil context: %w", ErrInvalidReport)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("report: project and record roots are required: %w", ErrInvalidReport)
	}
	if !validReportSlug(input.Slug) {
		return fmt.Errorf("report: stage slug is required and must be a single canonical token: %w", ErrInvalidReport)
	}
	if input.Current.Slug == "" || input.Current.Slug != input.Slug {
		return fmt.Errorf("report: requested slug %q must exactly match non-empty current stage %q: %w", input.Slug, input.Current.Slug, ErrInvalidReport)
	}
	switch input.Kind {
	case ReportKindAwaitingApproval, ReportKindRejected, ReportKindRevised, ReportKindApproved:
		return nil
	default:
		return fmt.Errorf("report kind %q is unsupported: %w", input.Kind, ErrInvalidReport)
	}
}

func validReportSlug(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}
