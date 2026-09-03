// Package orchestrator connects the internal APIs that start an AI-DLC intent.
package orchestrator

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

// StartInput contains caller-resolved values for starting one intent.
// DataFS and ScopesFS are borrowed and remain owned by the caller.
type StartInput struct {
	Root                      workspace.RootInput
	SpaceName                 string
	Label                     string
	Scope                     string
	Repos                     []string
	DataFS                    fs.FS
	ScopesFS                  fs.FS
	ProjectDescription        string
	ProjectDescriptionPreview string
	DepthOverride             string
	TestStrategyOverride      string
	ReviewOverride            string
}

// StartedIntent contains the committed intent and the results produced while
// its initial state was being built. Intent remains populated after the
// registry commit even if initialization is only partially complete.
type StartedIntent struct {
	Intent                 workspace.CreatedIntent
	Workspace              workspace.ScanResult
	Initial                state.Initial
	InitializationComplete bool
}

type startIntentOps struct {
	createIntent func(context.Context, workspace.RootInput, workspace.IntentCreateInput, workspace.IntentInitializer) (workspace.CreatedIntent, error)
	detect       func(*os.Root) workspace.ScanResult
	loadGraph    func(fs.FS) (graph.Snapshot, error)
	readScopes   func(fs.FS) ([]scope.Metadata, error)
	now          func() time.Time
	buildInitial func(state.Input) (state.Initial, error)
	writeInitial func(*os.Root, state.Initial) error
}

func systemStartIntentOps() startIntentOps {
	return startIntentOps{
		createIntent: workspace.CreateIntentWithInitializer,
		detect:       workspace.Detect,
		loadGraph:    graph.Load,
		readScopes:   scope.ReadAll,
		now:          time.Now,
		buildInitial: state.BuildInitial,
		writeInitial: state.WriteInitial,
	}
}

// StartIntent creates an intent and builds its initial state while the
// workspace lock and the project and record roots are held.
func StartIntent(ctx context.Context, input StartInput) (StartedIntent, error) {
	return startIntent(ctx, input, systemStartIntentOps())
}

func startIntent(
	ctx context.Context,
	input StartInput,
	ops startIntentOps,
) (started StartedIntent, err error) {
	scopeValue := input.Scope
	var scopePointer *string
	if scopeValue != "" {
		scopePointer = &scopeValue
	}
	created, err := ops.createIntent(
		ctx,
		input.Root,
		workspace.IntentCreateInput{
			SpaceName: input.SpaceName,
			Label:     input.Label,
			Scope:     scopePointer,
			Repos:     input.Repos,
		},
		func(projectRoot, recordRoot *os.Root, created workspace.CreatedIntent) error {
			return initializeIntent(
				input,
				workspace.ResolveRoot(input.Root),
				projectRoot,
				recordRoot,
				created,
				&started,
				ops,
			)
		},
	)
	started.Intent = created
	return started, err
}

func initializeIntent(
	input StartInput,
	projectPath string,
	projectRoot *os.Root,
	recordRoot *os.Root,
	created workspace.CreatedIntent,
	started *StartedIntent,
	ops startIntentOps,
) error {
	workspaceResult := ops.detect(projectRoot)
	started.Workspace = workspaceResult

	graphSnapshot, err := ops.loadGraph(input.DataFS)
	if err != nil {
		return fmt.Errorf("load stage graph: %w", err)
	}

	scopes, err := ops.readScopes(input.ScopesFS)
	if err != nil {
		return fmt.Errorf("read scope metadata: %w", err)
	}
	scopeMetadata, err := exactScope(scopes, input.Scope)
	if err != nil {
		return err
	}

	startDate := ops.now().UTC().Format(time.RFC3339)
	initial, err := ops.buildInitial(state.Input{
		Graph:         graphSnapshot,
		Scope:         input.Scope,
		ScopeMetadata: scopeMetadata,
		Workspace: state.WorkspaceInfo{
			ProjectType: workspaceResult.ProjectType,
			Languages:   workspaceResult.Languages,
			Frameworks:  workspaceResult.Frameworks,
			BuildSystem: workspaceResult.BuildSystem,
		},
		ProjectRoot:               projectPath,
		ProjectDescription:        input.ProjectDescription,
		ProjectDescriptionPreview: input.ProjectDescriptionPreview,
		StartDate:                 startDate,
		DepthOverride:             input.DepthOverride,
		TestStrategyOverride:      input.TestStrategyOverride,
		ReviewOverride:            input.ReviewOverride,
	})
	started.Initial = initial
	if err != nil {
		return fmt.Errorf("build initial state: %w", err)
	}

	if err := ops.writeInitial(recordRoot, initial); err != nil {
		return fmt.Errorf("write initial state: %w", err)
	}
	started.InitializationComplete = true
	return nil
}

func exactScope(metadata []scope.Metadata, name string) (scope.Metadata, error) {
	for _, item := range metadata {
		if item.Name == name {
			return item, nil
		}
	}
	return scope.Metadata{}, fmt.Errorf("start intent: unknown scope %q", name)
}
