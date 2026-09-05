package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/steering"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

// RunStageInput identifies the caller-owned roots used for one composition.
type RunStageInput struct {
	Identity    recordlock.Identity
	ProjectRoot *os.Root
	RecordRoot  *os.Root
	// IntentUUID optionally binds delivery freshness to the selected intent
	// registry identity when the caller has resolved one.
	IntentUUID     *string
	EnabledPlugins []string
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
	// Freshness binds the returned directive and rule bundle to the fresh inputs.
	Freshness steering.ContinuationFreshness
	// Claims contains the initial continuation claims when rules require chunks.
	Claims steering.ContinuationClaims
}

var (
	// ErrSelectionMismatch indicates that active workspace cursors do not
	// match the identity-bound record.
	ErrSelectionMismatch = errors.New("delivery: active selection mismatch")
	// ErrUnsupportedConsumeProvenance indicates that a missing required input
	// cannot be explained by an auditable conditional skip.
	ErrUnsupportedConsumeProvenance = errors.New("delivery: unsupported consume provenance")
	// ErrDirectiveTooLarge indicates that the complete directive exceeds the wire cap.
	ErrDirectiveTooLarge = errors.New("delivery: directive exceeds byte limit")
)

const maxRunStageDirectiveBytes = 28 * 1024

// ComposeRunStage reads the active selection and graph for one identity-bound
// run-stage composition. Caller-owned roots remain open and are never closed.
func ComposeRunStage(ctx context.Context, input RunStageInput) (RunStageComposition, error) {
	if err := validateRunStageInput(ctx, input); err != nil {
		return RunStageComposition{}, err
	}
	var result RunStageComposition
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		result, err = composeRunStageWithGuard(ctx, guard, input)
		return err
	})
	if err != nil {
		return RunStageComposition{}, err
	}
	return result, nil
}

// ComposeRunStageWithGuard composes a run-stage view while the caller-owned
// record lock remains held. It validates the guard and then performs all
// reads through that same transaction.
func ComposeRunStageWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput) (RunStageComposition, error) {
	if err := validateRunStageInput(ctx, input); err != nil {
		return RunStageComposition{}, err
	}
	return composeRunStageWithGuard(ctx, guard, input)
}

func validateRunStageInput(ctx context.Context, input RunStageInput) error {
	if ctx == nil {
		return fmt.Errorf("compose run-stage: context is nil: %w", orchestrator.ErrInvalidNext)
	}
	if input.ProjectRoot == nil || input.RecordRoot == nil {
		return fmt.Errorf("compose run-stage: project and record roots are required: %w", orchestrator.ErrInvalidNext)
	}
	return nil
}

func composeRunStageWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput) (RunStageComposition, error) {
	if err := audit.ValidateRecordBinding(ctx, input.Identity, guard, input.ProjectRoot, input.RecordRoot); err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: validate record binding: %w", err)
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

	next, err := orchestrator.NextWithGuard(ctx, guard, orchestrator.NextInput{
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
	depth, err := state.Depth(next.Content)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: read knowledge depth: %w", err)
	}
	frameworkFS, err := fs.Sub(projectFS, ".codex")
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: open framework knowledge: %w", err)
	}
	spaceKnowledgeFS, err := fs.Sub(projectFS, path.Join("aidlc", "spaces", input.Identity.Space(), "knowledge"))
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: open Space knowledge: %w", err)
	}
	roster, err := knowledge.BuildRoster(knowledge.RosterInput{
		Stage:        stage,
		Depth:        depth,
		Framework:    knowledge.Source{FS: frameworkFS, DisplayPrefix: ".codex"},
		FrameworkDir: filepath.Join(input.Identity.ProjectPath(), ".codex"),
		SpaceKnowledge: &knowledge.Source{
			FS:            spaceKnowledgeFS,
			DisplayPrefix: path.Join("aidlc", "spaces", input.Identity.Space(), "knowledge"),
		},
		EnabledPlugins: input.EnabledPlugins,
	})
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: build knowledge roster: %w", err)
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
	wire, err := buildRunStageWire(input.Identity, stage, next.State, catalog, input.ProjectRoot, input.RecordRoot, rules, roster)
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: build wire: %w", err)
	}
	if len(wire) > maxRunStageDirectiveBytes {
		return RunStageComposition{}, fmt.Errorf(
			"compose run-stage: directive is %d bytes, limit is %d: %w",
			len(wire),
			maxRunStageDirectiveBytes,
			ErrDirectiveTooLarge,
		)
	}
	routeHash, err := catalog.RouteHash(stage.Slug, next.State.Scope())
	if err != nil {
		return RunStageComposition{}, fmt.Errorf("compose run-stage: hash route: %w", err)
	}
	directiveDigest := sha256.Sum256(wire)
	directiveHash := hex.EncodeToString(directiveDigest[:])
	stateDigest := sha256.Sum256(next.Content)
	stateHash := hex.EncodeToString(stateDigest[:])
	freshnessStateHash := stateHash
	freshness := steering.ContinuationFreshness{
		Stage:         stage.Slug,
		Scope:         next.State.Scope(),
		Bundle:        bundle,
		DirectiveHash: directiveHash,
		RouteHash:     routeHash,
		StateHash:     &freshnessStateHash,
	}
	claims := steering.ContinuationClaims{}
	if len(chunks) != 0 {
		nextStage, err := nextStageName(next.State.NextStage(), catalog)
		if err != nil {
			return RunStageComposition{}, fmt.Errorf("compose run-stage: resolve continuation next stage: %w", err)
		}
		claimStateHash := stateHash
		claimNextStage := cloneRunStageString(nextStage)
		swarmSettled := false
		claims = steering.ContinuationClaims{
			Version:       1,
			Stage:         stage.Slug,
			Scope:         next.State.Scope(),
			NextPart:      1,
			Bundle:        bundle,
			DirectiveHash: directiveHash,
			RouteHash:     routeHash,
			StateAware:    true,
			Gate:          steering.GateTrue,
			NextStage: steering.OptionalNullableString{
				Present: true,
				Value:   claimNextStage,
			},
			SwarmSettled: &swarmSettled,
			StateHash:    &claimStateHash,
		}
	}
	return RunStageComposition{
		Directive: RunStageDirective{Kind: next.Kind(), Stage: stage},
		Wire:      wire,
		Rules:     rules,
		Chunks:    chunks,
		Bundle:    bundle,
		Freshness: freshness,
		Claims:    claims,
	}, nil
}

func cloneRunStageString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
