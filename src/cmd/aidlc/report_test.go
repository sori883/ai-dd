package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
)

const reportAdapterGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const reportAdapterScopeGridJSON = `{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE"}}}`

func TestReportAdapterLoadsFreshStateAndCatalogForOrchestrator(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}

	var captured orchestrator.ReportInput
	callback := reportAdapter(nil, nil, func(_ context.Context, input orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		captured = input
		return orchestrator.ReportResult{Kind: input.Kind, Slug: input.Slug}, nil
	})
	wire, err := callback("intent-capture", "rejected", "Request Changes", "please revise", "")
	if err != nil {
		t.Fatalf("reportAdapter() error = %v", err)
	}
	if string(wire) != `{"kind":"print","message":"Recorded rejected for \"intent-capture\"."}` {
		t.Fatalf("reportAdapter() wire = %q, want canonical rejected print", wire)
	}
	if captured.Identity != fixture.input.Identity {
		t.Errorf("ReportInput identity = %q, want %q", captured.Identity, fixture.input.Identity)
	}
	if captured.Slug != "intent-capture" || captured.Kind != orchestrator.ReportKindRejected {
		t.Errorf("ReportInput stage = (%q, %q), want intent-capture/rejected", captured.Slug, captured.Kind)
	}
	if captured.Current.Slug != "intent-capture" {
		t.Errorf("ReportInput current slug = %q, want intent-capture", captured.Current.Slug)
	}
	if captured.Choice != "Request Changes" || captured.Feedback != "please revise" {
		t.Errorf("ReportInput choice/feedback = %q/%q, want exact values", captured.Choice, captured.Feedback)
	}
	if len(captured.Catalog.Stages()) != 2 || captured.Catalog.Stages()[1].Slug != "intent-capture" {
		t.Errorf("ReportInput catalog = %#v, want fresh graph stages", captured.Catalog.Stages())
	}
}

func TestReportAdapterFormatsCanonicalResultWires(t *testing.T) {
	tests := []struct {
		name   string
		result string
		value  orchestrator.ReportResult
		want   string
	}{
		{
			name:   "awaiting approval",
			result: "awaiting-approval",
			value:  orchestrator.ReportResult{Kind: orchestrator.ReportKindAwaitingApproval, Slug: "intent-capture"},
			want:   `{"kind":"print","message":"Recorded awaiting-approval for \"intent-capture\"."}`,
		},
		{
			name:   "already awaiting approval",
			result: "awaiting-approval",
			value:  orchestrator.ReportResult{Kind: orchestrator.ReportKindAwaitingApproval, Slug: "intent-capture", Gate: orchestrator.GateResult{AlreadyAwaiting: true}},
			want:   `{"kind":"print","message":"Stage \"intent-capture\" is already awaiting approval; gate evidence revalidated."}`,
		},
		{
			name:   "rejected",
			result: "rejected",
			value:  orchestrator.ReportResult{Kind: orchestrator.ReportKindRejected, Slug: "intent-capture"},
			want:   `{"kind":"print","message":"Recorded rejected for \"intent-capture\"."}`,
		},
		{
			name:   "revised",
			result: "revised",
			value:  orchestrator.ReportResult{Kind: orchestrator.ReportKindRevised, Slug: "intent-capture"},
			want:   `{"kind":"print","message":"Recorded revised for \"intent-capture\"."}`,
		},
		{
			name:   "approved",
			result: "approved",
			value:  orchestrator.ReportResult{Kind: orchestrator.ReportKindApproved, Slug: "intent-capture"},
			want:   `{"kind":"done","reason":"Committed approve for \"intent-capture\". State advanced; run next to continue."}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newReportAdapterFixture(t)
			previousResolver := reportInputResolver
			t.Cleanup(func() { reportInputResolver = previousResolver })
			reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
				return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
			}
			callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
				return tt.value, nil
			})
			wire, err := callback("intent-capture", tt.result, "", "", "")
			if err != nil {
				t.Fatalf("reportAdapter() error = %v", err)
			}
			if string(wire) != tt.want {
				t.Errorf("reportAdapter() wire = %q, want %q", wire, tt.want)
			}
		})
	}
}

func TestReportAdapterWorkflowRejectionAndInternalFailureClassification(t *testing.T) {
	tests := []struct {
		name          string
		callbackError error
		wantWorkflow  bool
	}{
		{
			name:          "workflow rejection",
			callbackError: orchestrator.NewWorkflowError("stage is not ready", orchestrator.ErrGateNotReady),
			wantWorkflow:  true,
		},
		{
			name:          "ordinary I/O failure",
			callbackError: errors.New("record read failed"),
			wantWorkflow:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newReportAdapterFixture(t)
			previousResolver := reportInputResolver
			t.Cleanup(func() { reportInputResolver = previousResolver })
			reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
				return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
			}
			callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
				return orchestrator.ReportResult{}, tt.callbackError
			})
			wire, err := callback("intent-capture", "awaiting-approval", "", "", "")
			if wire != nil {
				t.Errorf("reportAdapter() wire = %q, want nil on callback failure", wire)
			}
			if err == nil {
				t.Fatal("reportAdapter() error = nil, want callback error")
			}
			if got := orchestrator.IsWorkflowError(err); got != tt.wantWorkflow {
				t.Errorf("IsWorkflowError(%v) = %v, want %v", err, got, tt.wantWorkflow)
			}
		})
	}
}

func TestReportAdapterRootCloseFailureIsInternal(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	previousCloser := reportRootCloser
	t.Cleanup(func() {
		reportInputResolver = previousResolver
		reportRootCloser = previousCloser
	})
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	closeErr := errors.New("report root close failed")
	reportRootCloser = func(root *os.Root) error {
		if root == fixture.recordRoot {
			return closeErr
		}
		return nil
	}
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		return orchestrator.ReportResult{Kind: orchestrator.ReportKindAwaitingApproval, Slug: "intent-capture"}, nil
	})
	wire, err := callback("intent-capture", "awaiting-approval", "", "", "")
	if wire != nil {
		t.Errorf("reportAdapter() wire = %q, want nil after cleanup failure", wire)
	}
	if err == nil || !errors.Is(err, closeErr) {
		t.Fatalf("reportAdapter() error = %v, want close failure", err)
	}
	if orchestrator.IsWorkflowError(err) {
		t.Errorf("reportAdapter() error = %v, must not remain workflow error", err)
	}
}

func TestReportAdapterResolverFailureIsInternal(t *testing.T) {
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	resolverErr := errors.New("active intent is not a safe component")
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return deliverypkg.RunStageInput{}, nil, nil, resolverErr
	}
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		t.Fatal("orchestrator callback invoked after resolver failure")
		return orchestrator.ReportResult{}, nil
	})
	wire, err := callback("intent-capture", "awaiting-approval", "", "", "")
	if wire != nil || !errors.Is(err, resolverErr) {
		t.Fatalf("reportAdapter(resolver failure) = (%q, %v), want resolver error", wire, err)
	}
	if orchestrator.IsWorkflowError(err) {
		t.Errorf("resolver failure = %v, want internal classification", err)
	}
}

func TestReportAdapterRejectsStageMismatchAsWorkflowError(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		t.Fatal("orchestrator callback invoked for stale stage")
		return orchestrator.ReportResult{}, nil
	})
	wire, err := callback("next-stage", "awaiting-approval", "", "", "")
	if wire != nil || err == nil {
		t.Fatalf("reportAdapter(stage mismatch) = (%q, %v), want workflow error", wire, err)
	}
	if !orchestrator.IsWorkflowError(err) || !errors.Is(err, orchestrator.ErrInvalidReport) {
		t.Errorf("stage mismatch error = %v, want workflow ErrInvalidReport", err)
	}
}

func TestReportAdapterRejectsMissingStateAndCatalogAsInternal(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	if err := fixture.recordRoot.Remove("aidlc-state.md"); err != nil {
		t.Fatalf("Remove(state): %v", err)
	}
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		t.Fatal("orchestrator callback invoked with missing state")
		return orchestrator.ReportResult{}, nil
	})
	wire, err := callback("intent-capture", "awaiting-approval", "", "", "")
	if wire != nil || err == nil {
		t.Fatalf("reportAdapter(missing state) = (%q, %v), want internal error", wire, err)
	}
	if orchestrator.IsWorkflowError(err) {
		t.Errorf("missing state error = %v, want internal classification", err)
	}
}

func TestReportAdapterRejectsMissingCatalogAsInternal(t *testing.T) {
	fixture := newReportAdapterFixture(t)
	previousResolver := reportInputResolver
	t.Cleanup(func() { reportInputResolver = previousResolver })
	reportInputResolver = func(func() (string, error), func(string) string, string) (deliverypkg.RunStageInput, *os.Root, *os.Root, error) {
		return fixture.input, fixture.projectRoot, fixture.recordRoot, nil
	}
	if err := fixture.projectRoot.Remove(".codex/tools/data/stage-graph.json"); err != nil {
		t.Fatalf("Remove(stage graph): %v", err)
	}
	callback := reportAdapter(nil, nil, func(context.Context, orchestrator.ReportInput) (orchestrator.ReportResult, error) {
		t.Fatal("orchestrator callback invoked with missing catalog")
		return orchestrator.ReportResult{}, nil
	})
	wire, err := callback("intent-capture", "awaiting-approval", "", "", "")
	if wire != nil || err == nil {
		t.Fatalf("reportAdapter(missing catalog) = (%q, %v), want internal error", wire, err)
	}
	if orchestrator.IsWorkflowError(err) {
		t.Errorf("missing catalog error = %v, want internal classification", err)
	}
}

type reportAdapterFixture struct {
	input       deliverypkg.RunStageInput
	projectRoot *os.Root
	recordRoot  *os.Root
}

func newReportAdapterFixture(t *testing.T) reportAdapterFixture {
	t.Helper()
	projectPath := t.TempDir()
	recordPath := filepath.Join(projectPath, "aidlc", "spaces", "team", "intents", "build")
	dataPath := filepath.Join(projectPath, ".codex", "tools", "data")
	if err := os.MkdirAll(recordPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	writeReportAdapterFile(t, filepath.Join(dataPath, "stage-graph.json"), reportAdapterGraphJSON)
	writeReportAdapterFile(t, filepath.Join(dataPath, "scope-grid.json"), reportAdapterScopeGridJSON)
	catalog, err := graph.Load(os.DirFS(dataPath))
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     catalog,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               projectPath,
		ProjectDescription:        "report adapter",
		ProjectDescriptionPreview: "report adapter",
		StartDate:                 "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	writeReportAdapterFile(t, filepath.Join(recordPath, "aidlc-state.md"), initial.StateContent)
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build")))
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(projectPath, "team", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return reportAdapterFixture{
		input: deliverypkg.RunStageInput{
			Identity:    identity,
			ProjectRoot: projectRoot,
			RecordRoot:  recordRoot,
		},
		projectRoot: projectRoot,
		recordRoot:  recordRoot,
	}
}

func writeReportAdapterFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
