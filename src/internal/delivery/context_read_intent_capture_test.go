package delivery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadIntentCaptureContextOrder(t *testing.T) {
	fixture := newRunStageFixture(t)
	prefix := filepath.ToSlash(filepath.Join("aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent()))
	wire := contextRunStageWire{
		Kind:  string(ActiveDirectiveKindRunStage),
		Stage: "intent-capture",
		InlineContextPaths: []string{
			".codex/agents/aidlc-product-agent.md",
			".codex/aidlc-common/protocols/stage-protocol.md",
			".codex/aidlc-common/protocols/stage-protocol-reviewer.md",
			".codex/skills/aidlc/question-rendering.md",
			prefix + "/project-description.json",
		},
		StageFile: ".codex/aidlc-common/stages/ideation/intent-capture.md",
		Consumes:  []string{prefix + "/ideation/intent-capture/input.md"},
	}
	plan, err := buildContextReadPlan(
		RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot},
		ActiveDirectiveMarker{Revision: 1, ActiveAttempt: &ActiveDirectiveAttempt{ID: "generation"}},
		RunStageComposition{Wire: []byte(`{"kind":"run-stage"}`)},
		wire,
	)
	if err != nil {
		t.Fatalf("buildContextReadPlan() error = %v", err)
	}
	if len(plan.Targets) != 7 {
		t.Fatalf("context targets = %d, want 7", len(plan.Targets))
	}
	wantSlots := []ContextReadSlot{
		ContextReadSlotInline,
		ContextReadSlotProtocol,
		ContextReadSlotReviewer,
		ContextReadSlotQuestion,
		ContextReadSlotProjectDescription,
		ContextReadSlotStage,
		ContextReadSlotConsume,
	}
	for index, want := range wantSlots {
		if got := plan.Targets[index].Slot; got != want {
			t.Errorf("target %d slot = %q, want %q", index, got, want)
		}
	}
	if got := plan.Targets[4].Root; got != fixture.recordRoot {
		t.Errorf("project description root = %p, want record root %p", got, fixture.recordRoot)
	}
	if got, want := plan.Targets[4].RelativePath, "project-description.json"; got != want {
		t.Errorf("project description relative path = %q, want %q", got, want)
	}
}

func TestReadIntentCaptureContextRejectsUnsafeTarget(t *testing.T) {
	fixture := newRunStageFixture(t)
	_, err := buildContextReadPlan(
		RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot},
		ActiveDirectiveMarker{Revision: 1, ActiveAttempt: &ActiveDirectiveAttempt{ID: "generation"}},
		RunStageComposition{Wire: []byte(`{"kind":"run-stage"}`)},
		contextRunStageWire{
			Kind:               string(ActiveDirectiveKindRunStage),
			Stage:              "intent-capture",
			InlineContextPaths: []string{"../outside.md"},
			StageFile:          ".codex/aidlc-common/stages/ideation/intent-capture.md",
		},
	)
	if err == nil || !errors.Is(err, ErrContextReadUnsafePath) {
		t.Fatalf("buildContextReadPlan(unsafe) error = %v, want ErrContextReadUnsafePath", err)
	}
}

func TestContinueIntentCaptureContextDetectsChange(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if first.ReadContinueToken == "" {
		t.Fatal("ReadContext() token is empty, want continuation token")
	}
	leadPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "agents", "aidlc-product-agent.md")
	if err := os.WriteFile(leadPath, []byte(strings.Repeat("changed\n", 4)), 0o600); err != nil {
		t.Fatalf("WriteFile(changed context): %v", err)
	}
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
		t.Fatalf("ContinueContext(changed) error = %v, want ErrContextReadFileChanged", err)
	}
}
