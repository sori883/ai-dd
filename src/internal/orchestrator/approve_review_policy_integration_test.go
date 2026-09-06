//go:build integration

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/learnings"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/sensor"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestApproveGateIntegrationUsesEffectiveReviewPolicy(t *testing.T) {
	tests := map[string]struct {
		reviewCap string
		override  string
	}{
		"scope cap none":      {reviewCap: "none"},
		"state override none": {reviewCap: "adversarial", override: "none"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newApproveReviewPolicyFixture(t, tt.reviewCap, tt.override)
			input := ApproveInput{
				Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
				Current: fixture.stage, Catalog: fixture.catalog, Choice: "Approve",
			}
			if _, err := OpenGate(context.Background(), GateInput{
				Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
				Current: fixture.stage, Catalog: fixture.catalog,
			}); err != nil {
				t.Fatalf("OpenGate() error = %v", err)
			}
			if err := audit.RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
				t.Fatalf("RecordHumanTurn(): %v", err)
			}
			if _, err := ApproveGate(context.Background(), input); err != nil {
				t.Fatalf("ApproveGate() error = %v, want success with effective review none", err)
			}
		})
	}
}

func TestOpenGateIntegrationRequiresReviewForEffectiveAdvisory(t *testing.T) {
	fixture := newApproveReviewPolicyFixture(t, "advisory", "")
	_, err := OpenGate(context.Background(), GateInput{
		Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot,
		Current: fixture.stage, Catalog: fixture.catalog,
	})
	if !errors.Is(err, ErrGateNotReady) {
		t.Fatalf("OpenGate() error = %v, want ErrGateNotReady for missing advisory review", err)
	}
}

type approveReviewPolicyFixture struct {
	projectDir  string
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
	catalog     graph.Snapshot
	stage       graph.Stage
}

func newApproveReviewPolicyFixture(t *testing.T, reviewCap, override string) approveReviewPolicyFixture {
	t.Helper()
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "team", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".codex", "scopes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "aidlc", ".aidlc-clone-id"), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".codex", "scopes", "feature.md"), []byte(fmt.Sprintf("---\nname: feature\nreview_cap: %s\n---\n", reviewCap)), 0o600); err != nil {
		t.Fatal(err)
	}

	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: []byte(`[
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["enterprise","feature","mvp","poc"],"enabled":true,"summary_confirmation":"required","reviewer":"aidlc-product-lead-agent","reviewer_max_iterations":2,"review_class":"advisory","review_artifact":"intent-statement","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"requires_stage":[]},
  {"slug":"market-research","number":"1.2","name":"Market Research","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["feature"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`)},
		"scope-grid.json": {Data: []byte(`{"feature":{"stages":{"intent-capture":"EXECUTE","market-research":"EXECUTE"}}}`)},
	}
	catalog, err := graph.Load(dataFS)
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph: catalog, Scope: "feature", ScopeMetadata: scope.Metadata{Name: "feature", Depth: "Standard", ReviewCap: scope.ReviewCap(reviewCap)},
		ReviewOverride: override, Workspace: state.WorkspaceInfo{ProjectType: "Brownfield"}, ProjectRoot: projectDir,
		ProjectDescription: "approval review policy fixture", ProjectDescriptionPreview: "approval review policy fixture", StartDate: "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, "aidlc-state.md"), []byte(initial.StateContent), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := recordlock.NewIdentity(projectDir, "team", "build")
	if err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	var stage graph.Stage
	for _, candidate := range catalog.Stages() {
		if candidate.Slug == "intent-capture" {
			stage = candidate
			break
		}
	}
	if stage.Slug == "" {
		t.Fatal("intent-capture stage missing")
	}
	writeApproveReviewPolicyEvidence(t, identity, projectRoot, recordRoot)
	return approveReviewPolicyFixture{projectDir: projectDir, identity: identity, projectRoot: projectRoot, recordRoot: recordRoot, catalog: catalog, stage: stage}
}

func writeApproveReviewPolicyEvidence(t *testing.T, identity recordlock.Identity, projectRoot, recordRoot *os.Root) {
	t.Helper()
	questionsPath := filepath.Join("ideation", "intent-capture", "intent-capture-questions.md")
	questions := "# Intent Capture Questions\n\n## Q1\nWhat is the goal?\n[Answer]: yes\n\n## Consolidated Summary Confirmation\n- Looks correct\n- Request changes\n[Answer]: Looks correct\n\n## Assumption Confirmation\n[Answer]: none\n"
	for name, content := range map[string]string{
		questionsPath: questions,
		"ideation/intent-capture/intent-statement.md": "# Intent Statement\n\n## Problem\n- [desc] A traceable goal.\n",
		"ideation/intent-capture/stakeholder-map.md":  "# Stakeholder Map\n\n## Stakeholders\n- [desc] The product team.\n",
	} {
		if err := recordRoot.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s): %v", name, err)
		}
		if err := recordRoot.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	if err := recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		return audit.Append(context.Background(), guard, projectRoot, recordRoot, []audit.Event{{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": "intent-capture"}}})
	}); err != nil {
		t.Fatalf("append STAGE_STARTED: %v", err)
	}
	if err := audit.RecordIntentCaptureDecisionFromQuestions(context.Background(), identity, projectRoot, recordRoot, "intent-capture", "q1"); err != nil {
		t.Fatalf("record q1 decision: %v", err)
	}
	if err := audit.RecordHumanTurn(context.Background(), identity, projectRoot, recordRoot); err != nil {
		t.Fatalf("record q1 turn: %v", err)
	}
	if err := audit.RecordIntentCaptureAnswerByID(context.Background(), identity, projectRoot, recordRoot, "intent-capture", "q1", "yes"); err != nil {
		t.Fatalf("record q1 answer: %v", err)
	}
	if err := audit.RecordIntentCaptureDecision(context.Background(), identity, projectRoot, recordRoot, audit.IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "backend"}); err != nil {
		t.Fatalf("record summary decision: %v", err)
	}
	if err := audit.RecordHumanTurn(context.Background(), identity, projectRoot, recordRoot); err != nil {
		t.Fatalf("record summary turn: %v", err)
	}
	if err := recordRoot.WriteFile(questionsPath, []byte(questions), 0o600); err != nil {
		t.Fatalf("rewrite answered questions: %v", err)
	}
	if err := audit.RecordSummaryConfirmation(context.Background(), identity, projectRoot, recordRoot, audit.IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "backend"}, "Looks correct", "ignored"); err != nil {
		t.Fatalf("record summary confirmation: %v", err)
	}
	for _, invocation := range sensor.RunAll(context.Background(), sensor.Input{Stage: "intent-capture", ArtifactPath: "ideation/intent-capture/intent-statement.md", Questions: []byte(questions)}, nil) {
		if err := audit.RecordSensorInvocation(context.Background(), identity, projectRoot, recordRoot, invocation); err != nil {
			t.Fatalf("record sensor invocation: %v", err)
		}
	}
	question, err := audit.RecordIntentCaptureLearningDecision(context.Background(), identity, projectRoot, recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("record learning decision: %v", err)
	}
	if err := audit.RecordHumanTurn(context.Background(), identity, projectRoot, recordRoot); err != nil {
		t.Fatalf("record learning turn: %v", err)
	}
	if err := audit.RecordIntentCaptureLearning(context.Background(), identity, projectRoot, recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation}); err != nil {
		t.Fatalf("record learning answer: %v", err)
	}
}
