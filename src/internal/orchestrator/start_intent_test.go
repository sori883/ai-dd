package orchestrator

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestStartIntent(t *testing.T) {
	t.Parallel()

	projectRoot := new(os.Root)
	recordRoot := new(os.Root)
	created := workspace.CreatedIntent{
		UUID:      "0199-aaaa",
		Slug:      "build-auth",
		DirName:   "260903-build-auth",
		RecordDir: "/project/aidlc/spaces/team/intents/260903-build-auth",
		SpaceName: "team",
	}
	now := time.Date(2026, time.September, 3, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60))
	steps := []string{}
	wantWorkspace := workspace.ScanResult{
		ProjectType: "Brownfield",
		Languages:   "Go",
		Frameworks:  "Unknown",
		BuildSystem: "Go modules",
		NestedRoot:  "service",
		Submodules:  []workspace.Submodule{{Name: "shared", Path: "shared", URL: "https://example.invalid/shared", Initialized: true}},
	}
	wantGraph := graph.Snapshot{}
	wantMetadata := scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"}
	wantInitial := state.Initial{StateContent: "state", ProjectDescriptionJSON: "description"}
	ops := startIntentTestOps()
	ops.createIntent = func(
		ctx context.Context,
		root workspace.RootInput,
		input workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		steps = append(steps, "create")
		if ctx == nil {
			t.Error("create intent received nil context")
		}
		if root.ExplicitDir != "/project" {
			t.Errorf("create root = %+v, want explicit project root", root)
		}
		wantInput := workspace.IntentCreateInput{
			SpaceName: "team",
			Label:     "Build Auth",
			Scope:     stringPointer("classic"),
			Repos:     []string{"api", "web"},
		}
		if !reflect.DeepEqual(input, wantInput) {
			t.Errorf("create input = %+v, want %+v", input, wantInput)
		}
		err := initialize(projectRoot, recordRoot, created)
		return created, err
	}
	ops.detect = func(root *os.Root) workspace.ScanResult {
		steps = append(steps, "detect")
		if root != projectRoot {
			t.Error("Detect received a root other than the project root")
		}
		return wantWorkspace
	}
	ops.loadGraph = func(dataFS fs.FS) (graph.Snapshot, error) {
		steps = append(steps, "graph")
		if dataFS == nil {
			t.Error("graph loader received nil data FS")
		}
		return wantGraph, nil
	}
	ops.readScopes = func(scopesFS fs.FS) ([]scope.Metadata, error) {
		steps = append(steps, "scopes")
		if scopesFS == nil {
			t.Error("scope reader received nil scopes FS")
		}
		return []scope.Metadata{wantMetadata}, nil
	}
	ops.now = func() time.Time {
		steps = append(steps, "now")
		return now
	}
	ops.buildInitial = func(input state.Input) (state.Initial, error) {
		steps = append(steps, "build")
		if !reflect.DeepEqual(input.Graph, wantGraph) || input.Scope != "classic" ||
			!reflect.DeepEqual(input.ScopeMetadata, wantMetadata) {
			t.Errorf("initial input graph/scope/metadata = (%+v, %q, %+v)", input.Graph, input.Scope, input.ScopeMetadata)
		}
		if input.Workspace.ProjectType != wantWorkspace.ProjectType || input.ProjectRoot != "/project" {
			t.Errorf("initial input workspace/root = (%+v, %q)", input.Workspace, input.ProjectRoot)
		}
		if input.StartDate != now.UTC().Format(time.RFC3339) {
			t.Errorf("initial StartDate = %q, want %q", input.StartDate, now.UTC().Format(time.RFC3339))
		}
		return wantInitial, nil
	}
	ops.writeInitial = func(root *os.Root, initial state.Initial) error {
		steps = append(steps, "write")
		if root != recordRoot || !reflect.DeepEqual(initial, wantInitial) {
			t.Errorf("WriteInitial arguments = (%p, %+v), want (%p, %+v)", root, initial, recordRoot, wantInitial)
		}
		return nil
	}
	input := StartInput{
		Root:                      workspace.RootInput{ExplicitDir: "/project"},
		SpaceName:                 "team",
		Label:                     "Build Auth",
		Scope:                     "classic",
		Repos:                     []string{"api", "web"},
		DataFS:                    fstestMapFS(),
		ScopesFS:                  fstestMapFS(),
		ProjectDescription:        "Build authentication",
		ProjectDescriptionPreview: "Build authentication",
		DepthOverride:             "Standard",
		TestStrategyOverride:      "Comprehensive",
		ReviewOverride:            "advisory",
	}
	got, err := startIntent(context.Background(), input, ops)
	if err != nil {
		t.Fatal(err)
	}
	if got.Intent != created || !reflect.DeepEqual(got.Workspace, wantWorkspace) ||
		!reflect.DeepEqual(got.Initial, wantInitial) || !got.InitializationComplete {
		t.Errorf("startIntent() = %+v, want created workspace initial and complete", got)
	}
	wantSteps := []string{"create", "detect", "graph", "scopes", "now", "build", "write"}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want %q", steps, wantSteps)
	}
}

func TestStartIntentRetainsCommittedIntentAndBuiltInitialOnWriteFailure(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	created := workspace.CreatedIntent{
		UUID:      "0199-aaaa",
		Slug:      "build-auth",
		DirName:   "260903-build-auth",
		RecordDir: project + "/record",
		SpaceName: "team",
	}
	wantWorkspace := workspace.ScanResult{ProjectType: "Brownfield", Languages: "Go"}
	wantInitial := state.Initial{
		StateContent:           "state built before write",
		ProjectDescriptionJSON: "description",
	}
	writeCause := errors.New("write initial failed")
	now := time.Date(2026, time.September, 3, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60))
	nowCalls := 0
	ops := startIntentTestOps()
	ops.createIntent = func(
		ctx context.Context,
		root workspace.RootInput,
		input workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		if err := initialize(new(os.Root), new(os.Root), created); err != nil {
			return created, err
		}
		return created, nil
	}
	ops.detect = func(*os.Root) workspace.ScanResult { return wantWorkspace }
	ops.loadGraph = func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, nil }
	ops.readScopes = func(fs.FS) ([]scope.Metadata, error) {
		return []scope.Metadata{{Name: "classic"}}, nil
	}
	ops.now = func() time.Time {
		nowCalls++
		return now
	}
	ops.buildInitial = func(input state.Input) (state.Initial, error) {
		if input.StartDate != now.UTC().Format(time.RFC3339) {
			t.Errorf("BuildInitial StartDate = %q, want %q", input.StartDate, now.UTC().Format(time.RFC3339))
		}
		return wantInitial, nil
	}
	ops.writeInitial = func(*os.Root, state.Initial) error { return writeCause }

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:      workspace.RootInput{ExplicitDir: project},
			SpaceName: "team",
			Label:     "Build Auth",
			Scope:     "classic",
			DataFS:    fstestMapFS(),
			ScopesFS:  fstestMapFS(),
		},
		ops,
	)
	if got.Intent != created {
		t.Errorf("started intent = %+v, want committed intent %+v", got.Intent, created)
	}
	if !reflect.DeepEqual(got.Workspace, wantWorkspace) {
		t.Errorf("workspace = %+v, want %+v", got.Workspace, wantWorkspace)
	}
	if !reflect.DeepEqual(got.Initial, wantInitial) {
		t.Errorf("initial = %+v, want build result %+v", got.Initial, wantInitial)
	}
	if got.InitializationComplete {
		t.Error("InitializationComplete = true, want false after WriteInitial failure")
	}
	if !errors.Is(err, writeCause) {
		t.Errorf("StartIntent() error = %v, want write cause", err)
	}
	if nowCalls != 1 {
		t.Errorf("clock calls = %d, want exactly one", nowCalls)
	}
}

func TestStartIntentRetainsCommittedIntentWhenGraphFails(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	graphCause := errors.New("graph unavailable")
	initialized := false
	ops := startIntentTestOps()
	ops.createIntent = func(
		_ context.Context,
		_ workspace.RootInput,
		_ workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		initialized = true
		return created, initialize(new(os.Root), new(os.Root), created)
	}
	ops.detect = func(*os.Root) workspace.ScanResult {
		return workspace.ScanResult{ProjectType: "Brownfield"}
	}
	ops.loadGraph = func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, graphCause }

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:      workspace.RootInput{ExplicitDir: project},
			SpaceName: "team",
			Label:     "Build Auth",
			Scope:     "classic",
			DataFS:    fstestMapFS(),
			ScopesFS:  fstestMapFS(),
		},
		ops,
	)
	if !initialized {
		t.Error("initializer was not called")
	}
	if got.Intent != created {
		t.Errorf("started intent = %+v, want committed intent %+v", got.Intent, created)
	}
	if got.Workspace.ProjectType != "Brownfield" {
		t.Errorf("workspace = %+v, want Detect result retained on graph failure", got.Workspace)
	}
	if got.InitializationComplete {
		t.Error("InitializationComplete = true after graph failure")
	}
	if !errors.Is(err, graphCause) {
		t.Errorf("StartIntent() error = %v, want graph cause", err)
	}
}

func TestStartIntentRetainsCommittedIntentWhenScopeFails(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	scopeCause := errors.New("scope metadata unavailable")
	ops := startIntentTestOps()
	ops.createIntent = func(
		_ context.Context,
		_ workspace.RootInput,
		_ workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		return created, initialize(new(os.Root), new(os.Root), created)
	}
	ops.detect = func(*os.Root) workspace.ScanResult {
		return workspace.ScanResult{ProjectType: "Greenfield"}
	}
	ops.loadGraph = func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, nil }
	ops.readScopes = func(fs.FS) ([]scope.Metadata, error) { return nil, scopeCause }

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:     workspace.RootInput{ExplicitDir: project},
			Scope:    "classic",
			DataFS:   fstestMapFS(),
			ScopesFS: fstestMapFS(),
		},
		ops,
	)
	if got.Intent != created {
		t.Errorf("started intent = %+v, want committed intent %+v", got.Intent, created)
	}
	if got.Workspace.ProjectType != "Greenfield" {
		t.Errorf("workspace = %+v, want Detect result retained on scope failure", got.Workspace)
	}
	if got.InitializationComplete {
		t.Error("InitializationComplete = true after scope failure")
	}
	if !errors.Is(err, scopeCause) {
		t.Errorf("StartIntent() error = %v, want scope cause", err)
	}
}

func TestStartIntentRetainsBuildResultWhenBuildFails(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	buildCause := errors.New("initial build failed")
	wantInitial := state.Initial{StateContent: "partial build"}
	ops := startIntentTestOps()
	ops.createIntent = func(
		_ context.Context,
		_ workspace.RootInput,
		_ workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		return created, initialize(new(os.Root), new(os.Root), created)
	}
	ops.detect = func(*os.Root) workspace.ScanResult {
		return workspace.ScanResult{ProjectType: "Brownfield"}
	}
	ops.loadGraph = func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, nil }
	ops.readScopes = func(fs.FS) ([]scope.Metadata, error) {
		return []scope.Metadata{{Name: "classic"}}, nil
	}
	ops.buildInitial = func(state.Input) (state.Initial, error) { return wantInitial, buildCause }
	ops.writeInitial = func(*os.Root, state.Initial) error {
		t.Fatal("WriteInitial ran after BuildInitial failure")
		return nil
	}

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:     workspace.RootInput{ExplicitDir: project},
			Scope:    "classic",
			DataFS:   fstestMapFS(),
			ScopesFS: fstestMapFS(),
		},
		ops,
	)
	if got.Intent != created {
		t.Errorf("started intent = %+v, want committed intent %+v", got.Intent, created)
	}
	if !reflect.DeepEqual(got.Initial, wantInitial) {
		t.Errorf("initial = %+v, want BuildInitial result retained with error", got.Initial)
	}
	if got.InitializationComplete {
		t.Error("InitializationComplete = true after build failure")
	}
	if !errors.Is(err, buildCause) {
		t.Errorf("StartIntent() error = %v, want build cause", err)
	}
}

func TestStartIntentRejectsUnknownExactScope(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	initialized := false
	ops := startIntentTestOps()
	ops.createIntent = func(
		_ context.Context,
		_ workspace.RootInput,
		_ workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		initialized = true
		return created, initialize(new(os.Root), new(os.Root), created)
	}
	ops.loadGraph = func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, nil }
	ops.readScopes = func(fs.FS) ([]scope.Metadata, error) {
		return []scope.Metadata{{Name: "Classic"}}, nil
	}
	ops.buildInitial = func(state.Input) (state.Initial, error) {
		t.Fatal("BuildInitial ran for a missing exact scope")
		return state.Initial{}, nil
	}

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:     workspace.RootInput{ExplicitDir: project},
			Scope:    "classic",
			DataFS:   fstestMapFS(),
			ScopesFS: fstestMapFS(),
		},
		ops,
	)
	if !initialized || got.Intent != created {
		t.Errorf("startIntent() = %+v, %v, want committed intent after exact-scope failure", got, err)
	}
	if got.InitializationComplete {
		t.Error("InitializationComplete = true after unknown scope")
	}
	if err == nil || !strings.Contains(err.Error(), `unknown scope "classic"`) {
		t.Errorf("StartIntent() error = %v, want exact scope error", err)
	}
}

func TestStartIntentSelectsExactScopeAndBorrowsDataFilesystems(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	dataFS := &trackingFS{}
	scopesFS := &trackingFS{}
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	metadata := []scope.Metadata{
		{Name: "Classic", Depth: "Comprehensive"},
		{Name: "classic", Depth: "Standard"},
	}
	steps := []string{}
	ops := startIntentTestOps()
	ops.createIntent = func(
		ctx context.Context,
		root workspace.RootInput,
		input workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		steps = append(steps, "create")
		return created, initialize(new(os.Root), new(os.Root), created)
	}
	ops.detect = func(*os.Root) workspace.ScanResult {
		steps = append(steps, "detect")
		return workspace.ScanResult{ProjectType: "Brownfield"}
	}
	ops.loadGraph = func(got fs.FS) (graph.Snapshot, error) {
		steps = append(steps, "graph")
		if got != dataFS {
			t.Errorf("graph FS = %T, want caller data FS", got)
		}
		return graph.Snapshot{}, nil
	}
	ops.readScopes = func(got fs.FS) ([]scope.Metadata, error) {
		steps = append(steps, "scopes")
		if got != scopesFS {
			t.Errorf("scopes FS = %T, want caller scopes FS", got)
		}
		return metadata, nil
	}
	ops.now = func() time.Time {
		steps = append(steps, "now")
		return time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC)
	}
	ops.buildInitial = func(input state.Input) (state.Initial, error) {
		steps = append(steps, "build")
		if input.ScopeMetadata.Name != "classic" || input.ScopeMetadata.Depth != "Standard" {
			t.Errorf("scope metadata = %+v, want exact case-sensitive match", input.ScopeMetadata)
		}
		return state.Initial{}, nil
	}
	ops.writeInitial = func(*os.Root, state.Initial) error {
		steps = append(steps, "write")
		return nil
	}

	got, err := startIntent(
		context.Background(),
		StartInput{
			Root:      workspace.RootInput{ExplicitDir: project},
			SpaceName: "team",
			Label:     "Build Auth",
			Scope:     "classic",
			DataFS:    dataFS,
			ScopesFS:  scopesFS,
		},
		ops,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !got.InitializationComplete {
		t.Error("InitializationComplete = false, want true")
	}
	if dataFS.closed || scopesFS.closed {
		t.Error("StartIntent closed caller-owned data filesystem")
	}
	wantSteps := []string{"create", "detect", "graph", "scopes", "now", "build", "write"}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want %q", steps, wantSteps)
	}
}

func TestStartIntentRetainsCompletionAfterCreateCleanupError(t *testing.T) {
	t.Parallel()

	cleanupCause := errors.New("workspace cleanup failed")
	created := workspace.CreatedIntent{DirName: "260903-build-auth", SpaceName: "team"}
	ops := startIntentTestOps()
	ops.readScopes = func(fs.FS) ([]scope.Metadata, error) {
		return []scope.Metadata{{Name: "classic"}}, nil
	}
	ops.createIntent = func(
		_ context.Context,
		_ workspace.RootInput,
		_ workspace.IntentCreateInput,
		initialize workspace.IntentInitializer,
	) (workspace.CreatedIntent, error) {
		if err := initialize(new(os.Root), new(os.Root), created); err != nil {
			return created, err
		}
		return created, cleanupCause
	}

	got, err := startIntent(
		context.Background(),
		StartInput{Scope: "classic", DataFS: fstestMapFS(), ScopesFS: fstestMapFS()},
		ops,
	)
	if !got.InitializationComplete {
		t.Error("InitializationComplete = false, want true after successful write")
	}
	if !errors.Is(err, cleanupCause) {
		t.Errorf("StartIntent() error = %v, want cleanup cause", err)
	}
}

func startIntentTestOps() startIntentOps {
	return startIntentOps{
		createIntent: func(
			context.Context,
			workspace.RootInput,
			workspace.IntentCreateInput,
			workspace.IntentInitializer,
		) (workspace.CreatedIntent, error) {
			return workspace.CreatedIntent{}, nil
		},
		detect:     func(*os.Root) workspace.ScanResult { return workspace.ScanResult{} },
		loadGraph:  func(fs.FS) (graph.Snapshot, error) { return graph.Snapshot{}, nil },
		readScopes: func(fs.FS) ([]scope.Metadata, error) { return []scope.Metadata{}, nil },
		now:        time.Now,
		buildInitial: func(state.Input) (state.Initial, error) {
			return state.Initial{}, nil
		},
		writeInitial: func(*os.Root, state.Initial) error { return nil },
	}
}

func fstestMapFS() fs.FS { return fstest.MapFS{} }

type trackingFS struct{ closed bool }

func (*trackingFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

func (f *trackingFS) Close() error {
	f.closed = true
	return nil
}

func stringPointer(value string) *string { return &value }
