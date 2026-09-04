package delivery

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/steering"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

// RunStageInput identifies the caller-owned roots used for one composition.
type RunStageInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
}

// RunStageDirective is the minimal validated routing result exposed by a
// composed run-stage directive.
type RunStageDirective struct {
	Kind  orchestrator.DirectiveKind
	Stage graph.Stage
}

// RunStageComposition is the result of composing one fresh run-stage view.
type RunStageComposition struct {
	Directive RunStageDirective
	// Wire is reserved for the canonical run-stage JSON representation.
	Wire []byte
	// Rules, Chunks, and Bundle are reserved for the resolved rule bundle.
	Rules  []steering.RuleContent
	Chunks [][]steering.RuleContent
	Bundle string
}

var (
	// ErrSelectionMismatch indicates that active workspace cursors do not
	// match the identity-bound record.
	ErrSelectionMismatch = errors.New("delivery: active selection mismatch")
	// ErrUnsupportedConsumeProvenance indicates that a missing required input
	// cannot be explained by an auditable conditional skip.
	ErrUnsupportedConsumeProvenance = errors.New("delivery: unsupported consume provenance")
)

// ComposeRunStage reads the active selection and graph for one identity-bound
// run-stage composition. Caller-owned roots remain open and are never closed.
func ComposeRunStage(ctx context.Context, input RunStageInput) (RunStageComposition, error) {
	if ctx == nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: context is nil: %w", orchestrator.ErrInvalidNext)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: project and record roots are required: %w", orchestrator.ErrInvalidNext)
	}

	projectFS := input.ProjectRoot.FS()
	activeSpace := workspace.ActiveSpace(projectFS)
	if activeSpace != input.Identity.Space() {
		return RunStageComposition{}, fmt.Errorf(
			"compose run-stage: active space %q does not match identity %q: %w",
			activeSpace,
			input.Identity.Space(),
			ErrSelectionMismatch,
		)
	}

	intentsFS, err := fs.Sub(projectFS, path.Join("aidlc", "spaces", activeSpace, "intents"))
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: open active intents: %w", err)
	}
	activeIntent, found := workspace.ActiveIntent(intentsFS, "")
	if !found || activeIntent != input.Identity.Intent() {
		return RunStageComposition{}, fmt.Errorf(
			"compose run-stage: active intent %q does not match identity %q: %w",
			activeIntent,
			input.Identity.Intent(),
			ErrSelectionMismatch,
		)
	}

	dataFS, err := fs.Sub(projectFS, path.Join(".codex", "tools", "data"))
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: open graph data: %w", err)
	}
	catalog, err := graph.Load(dataFS)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: load graph: %w", err)
	}

	next, err := orchestrator.Next(ctx, orchestrator.NextInput{
		Identity:    input.Identity,
		ProjectRoot: input.ProjectRoot,
		RecordRoot:  input.RecordRoot,
		Catalog:     catalog,
	})
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: classify next: %w", err)
	}
	if next.Kind() != orchestrator.DirectiveKindRunStage {
		return RunStageComposition{}, fmt.Errorf(
			"compose run-stage: next kind %q is not run-stage: %w",
			next.Kind(),
			orchestrator.ErrInvalidNext,
		)
	}
	stage, ok := next.Stage()
	if !ok {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: next run-stage has no stage: %w", orchestrator.ErrInvalidNext)
	}
	resolvedRules, err := steering.ResolveRulePaths(
		input.Identity.ProjectPath(),
		input.Identity.Space(),
		"",
		stage.RulesInContext,
	)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: resolve rules: %w", err)
	}
	memoryFS, err := fs.Sub(projectFS, path.Join("aidlc", "spaces", input.Identity.Space(), "memory"))
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: open rules memory: %w", err)
	}
	rules, err := steering.ReadResolvedRules(projectFS, memoryFS, resolvedRules.Entries)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: read rules: %w", err)
	}
	bundle, err := steering.BundleDigest(rules)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: digest rules: %w", err)
	}
	chunks := steering.ChunkRules(rules)
	wire, err := buildRunStageWire(input.Identity, stage, next.State, catalog, input.RecordRoot, rules)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: build wire: %w", err)
	}
	return RunStageComposition{
		Directive: RunStageDirective{Kind: next.Kind(), Stage: stage},
		Wire:      wire,
		Rules:     rules,
		Chunks:    chunks,
		Bundle:    bundle,
	}, nil
}
