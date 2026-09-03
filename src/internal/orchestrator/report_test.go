package orchestrator

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestReportDelegatesEachExplicitKindOnce(t *testing.T) {
	stage := graph.Stage{Slug: "intent-capture"}
	inputBase := ReportInput{
		Identity:    recordlock.Identity{},
		ProjectRoot: new(os.Root),
		RecordRoot:  new(os.Root),
		Slug:        stage.Slug,
		Current:     stage,
		Catalog:     graph.Snapshot{},
		Choice:      "Request Changes",
		Feedback:    "Please update the artifact",
	}

	for _, test := range []struct {
		name string
		kind ReportKind
	}{
		{name: "awaiting approval", kind: ReportKindAwaitingApproval},
		{name: "rejected", kind: ReportKindRejected},
		{name: "revised", kind: ReportKindRevised},
		{name: "approved", kind: ReportKindApproved},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := map[string]int{}
			ops := reportOps{
				openGate: func(context.Context, GateInput) (GateResult, error) {
					calls["open"]++
					return GateResult{Changed: true}, nil
				},
				rejectGate: func(context.Context, GateInput) (GateResult, error) {
					calls["reject"]++
					return GateResult{Changed: true}, nil
				},
				reviseGate: func(context.Context, GateInput) (GateResult, error) {
					calls["revise"]++
					return GateResult{Changed: true}, nil
				},
				approveGate: func(context.Context, ApproveInput) (ApproveResult, error) {
					calls["approve"]++
					return ApproveResult{ApprovalSaved: true}, nil
				},
			}

			input := inputBase
			input.Kind = test.kind
			result, err := reportWithOps(context.Background(), input, ops)
			if err != nil {
				t.Fatalf("reportWithOps() error = %v", err)
			}
			if result.Kind != test.kind || result.Slug != stage.Slug {
				t.Fatalf("ReportResult = %#v, want kind %q and slug %q", result, test.kind, stage.Slug)
			}

			for name, count := range calls {
				want := 0
				if (test.kind == ReportKindAwaitingApproval && name == "open") ||
					(test.kind == ReportKindRejected && name == "reject") ||
					(test.kind == ReportKindRevised && name == "revise") ||
					(test.kind == ReportKindApproved && name == "approve") {
					want = 1
				}
				if count != want {
					t.Errorf("%s delegate count = %d, want %d", name, count, want)
				}
			}
		})
	}
}

func TestReportRejectsInvalidKindOrSlugBeforeDelegation(t *testing.T) {
	base := ReportInput{
		ProjectRoot: new(os.Root),
		RecordRoot:  new(os.Root),
		Kind:        ReportKindApproved,
		Slug:        "intent-capture",
		Current:     graph.Stage{Slug: "intent-capture"},
	}
	for _, test := range []struct {
		name   string
		mutate func(*ReportInput)
	}{
		{name: "unknown kind", mutate: func(input *ReportInput) { input.Kind = ReportKind("done") }},
		{name: "empty slug", mutate: func(input *ReportInput) { input.Slug = "" }},
		{name: "slug with whitespace", mutate: func(input *ReportInput) { input.Slug = " intent-capture" }},
		{name: "current mismatch", mutate: func(input *ReportInput) { input.Current = graph.Stage{Slug: "other-stage"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			result, err := reportWithOps(context.Background(), input, reportOps{})
			if !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("reportWithOps() error = %v, want ErrInvalidReport", err)
			}
			if !reflect.DeepEqual(result, ReportResult{}) {
				t.Fatalf("reportWithOps() result = %#v, want zero result", result)
			}
		})
	}
}

func TestReportPreservesPartialApprovalResultAndError(t *testing.T) {
	partial := ApproveResult{
		ApprovalSaved:           true,
		FinalTransitionComplete: false,
		Changed:                 true,
	}
	wantErr := errors.New("second transition failed")
	input := ReportInput{
		ProjectRoot: new(os.Root),
		RecordRoot:  new(os.Root),
		Kind:        ReportKindApproved,
		Slug:        "intent-capture",
		Current:     graph.Stage{Slug: "intent-capture"},
		Choice:      "Approve",
	}
	result, err := reportWithOps(context.Background(), input, reportOps{
		approveGate: func(context.Context, ApproveInput) (ApproveResult, error) {
			return partial, wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("reportWithOps() error = %v, want injected error", err)
	}
	if !reflect.DeepEqual(result.Approval, partial) {
		t.Fatalf("ReportResult.Approval = %#v, want partial result %#v", result.Approval, partial)
	}
}
