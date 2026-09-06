package audit

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/pathnorm"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestRecordHumanTurnIfCurrentHoldsWorkspaceLockAcrossAppend(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	observation, err := ObserveHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot)
	if err != nil {
		t.Fatalf("ObserveHumanTurn() error = %v", err)
	}

	lockPath := humanTurnWorkspaceLockPath(fixture.project)
	selectionChecked := make(chan struct{})
	continueAppend := make(chan struct{})
	previous := humanTurnAfterSelectionValidation
	t.Cleanup(func() { humanTurnAfterSelectionValidation = previous })
	humanTurnAfterSelectionValidation = func() {
		if _, statErr := os.Stat(lockPath); statErr != nil {
			t.Errorf("workspace lock during selection revalidation: %v", statErr)
		}
		close(selectionChecked)
		<-continueAppend
	}

	recordDone := make(chan error, 1)
	go func() {
		recordDone <- RecordHumanTurnIfCurrent(
			context.Background(),
			fixture.identity,
			fixture.projectRoot,
			fixture.recordRoot,
			observation,
		)
	}()
	select {
	case <-selectionChecked:
	case <-time.After(2 * time.Second):
		t.Fatal("HUMAN_TURN append did not reach selection revalidation")
	}

	switchDone := make(chan error, 1)
	go func() {
		_, switchErr := workspace.SwitchIntent(
			workspace.RootInput{ExplicitDir: fixture.project},
			"revised",
		)
		switchDone <- switchErr
	}()
	close(continueAppend)

	select {
	case err := <-recordDone:
		if err != nil {
			t.Fatalf("RecordHumanTurnIfCurrent() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HUMAN_TURN append did not finish after releasing workspace lock")
	}
	select {
	case err := <-switchDone:
		if err != nil {
			t.Fatalf("SwitchIntent() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SwitchIntent() did not finish after HUMAN_TURN append")
	}

	activeIntent, err := os.ReadFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "active-intent"))
	if err != nil || string(activeIntent) != "revised\n" {
		t.Errorf("active-intent = (%q, %v), want revised", activeIntent, err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(records) != 1 || records[0].Event != "HUMAN_TURN" {
		t.Fatalf("records = %#v, want one HUMAN_TURN", records)
	}
}

func TestRecordHumanTurnIfCurrentRejectsCompletedSelectionSwitch(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	observation, err := ObserveHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot)
	if err != nil {
		t.Fatalf("ObserveHumanTurn() error = %v", err)
	}
	if _, err := workspace.SwitchIntent(workspace.RootInput{ExplicitDir: fixture.project}, "revised"); err != nil {
		t.Fatalf("SwitchIntent() error = %v", err)
	}
	if err := RecordHumanTurnIfCurrent(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, observation); !errors.Is(err, ErrHumanTurnObservationStale) {
		t.Fatalf("RecordHumanTurnIfCurrent() error = %v, want ErrHumanTurnObservationStale", err)
	}
	assertHumanTurnAuditAbsent(t, fixture.recordRoot)
}

func assertHumanTurnAuditAbsent(t *testing.T, recordRoot *os.Root) {
	t.Helper()
	if _, err := recordRoot.Lstat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("HUMAN_TURN audit after stale selection = %v, want absent", err)
	}
}

type humanTurnWorkspaceFixture struct {
	project     string
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
}

func newHumanTurnWorkspaceFixture(t *testing.T) humanTurnWorkspaceFixture {
	t.Helper()
	project := t.TempDir()
	graphSnapshot, err := graph.Load(fstest.MapFS{
		"stage-graph.json": &fstest.MapFile{Data: []byte(`[
            {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
	        ]`)},
		"scope-grid.json": &fstest.MapFile{Data: []byte(`{"classic":{"stages":{"intent-capture":"EXECUTE"}}}`)},
	})
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:                     graphSnapshot,
		Scope:                     "classic",
		ScopeMetadata:             scope.Metadata{Name: "classic", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               project,
		ProjectDescription:        "human turn",
		ProjectDescriptionPreview: "human turn",
		StartDate:                 "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	oldRecordPath := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	revisedRecordPath := filepath.Join(project, "aidlc", "spaces", "team", "intents", "revised")
	if err := os.MkdirAll(oldRecordPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(old record): %v", err)
	}
	if err := os.MkdirAll(revisedRecordPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(revised record): %v", err)
	}
	for name, content := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"):                               "team\n",
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "active-intent"): "build\n",
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"):  `[{"uuid":"build","slug":"build","status":"planning","dirName":"build"},{"uuid":"revised","slug":"revised","status":"planning","dirName":"revised"}]`,
		filepath.Join(oldRecordPath, "aidlc-state.md"):                                initial.StateContent,
		filepath.Join(revisedRecordPath, "aidlc-state.md"):                            initial.StateContent,
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", name, err)
		}
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build")))
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("OpenRoot(record): %v", err)
	}
	identity, err := recordlock.NewIdentity(project, "team", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatalf("NewIdentity(): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	return humanTurnWorkspaceFixture{project: project, identity: identity, projectRoot: projectRoot, recordRoot: recordRoot}
}

func humanTurnWorkspaceLockPath(project string) string {
	canonical, err := filepath.EvalSymlinks(project)
	if err != nil {
		canonical = filepath.Clean(project)
	}
	identity := pathnorm.NormalizeForPlatform(canonical, runtime.GOOS) + "\x00__workspace__"
	digest := md5.Sum([]byte(identity))
	return filepath.Join(os.TempDir(), ".aidlc-audit-"+hex.EncodeToString(digest[:4])+".lock")
}

func TestRecordHumanTurnUsesIdentityBoundAppendWithoutPromptFields(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "aidlc", ".aidlc-clone-id"), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(clone id): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "default", "intents", "build")))
	if err != nil {
		t.Fatalf("OpenRoot(record): %v", err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity(): %v", err)
	}

	if err := RecordHumanTurn(context.Background(), identity, projectRoot, recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn() error = %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one HUMAN_TURN", records)
	}
	if records[0].Event != "HUMAN_TURN" {
		t.Errorf("event = %q, want HUMAN_TURN", records[0].Event)
	}
	if len(records[0].Fields) != 0 {
		t.Errorf("HUMAN_TURN fields = %#v, want no prompt or choice fields", records[0].Fields)
	}
}

func TestRecordHumanTurnRejectsNilContextBeforeMutation(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "default", "intents", "build")))
	if err != nil {
		t.Fatalf("OpenRoot(record): %v", err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity(): %v", err)
	}
	if err := RecordHumanTurn(nil, identity, projectRoot, recordRoot); err == nil {
		t.Fatal("RecordHumanTurn(nil context) error = nil, want error")
	}
	if _, err := recordRoot.Lstat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("audit after nil context = %v, want absent", err)
	}
}
