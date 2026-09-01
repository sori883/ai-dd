//go:build integration

package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateIntentIntegrationCreatesCoreArtifacts(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc/spaces/team/intents"},
		map[string]string{
			"aidlc/spaces/team/intents/intents.json": `[
  {"uuid":"existing","slug":"keep","status":"planning","future":{"nested":true}}
]
`,
			"aidlc/spaces/team/knowledge/keep.md": "protected",
			"aidlc/.aidlc-sessions/keep":          "session",
			".codex/config.toml":                  "config",
		},
	)
	before := snapshotSpaceTree(t, project)
	scope := "repository"
	startDate := time.Now().UTC().Format("060102")
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{
			SpaceName: "team",
			Label:     "Build Auth",
			Scope:     &scope,
			Repos:     []string{"api", "web"},
		},
	)
	endDate := time.Now().UTC().Format("060102")
	if err != nil {
		t.Fatal(err)
	}
	if created.Slug != "build-auth" || created.SpaceName != "team" {
		t.Errorf("CreateIntent() = %+v, want normalized slug and selected space", created)
	}
	wantPrefix := startDate + "-build-auth"
	if startDate != endDate {
		wantPrefix = endDate + "-build-auth"
	}
	if created.DirName != wantPrefix {
		t.Errorf("DirName = %q, want current UTC base %q", created.DirName, wantPrefix)
	}
	wantRecord := filepath.Join(project, "aidlc", "spaces", "team", "intents", created.DirName)
	if created.RecordDir != wantRecord {
		t.Errorf("RecordDir = %q, want %q", created.RecordDir, wantRecord)
	}
	compactUUID := strings.ReplaceAll(created.UUID, "-", "")
	if len(compactUUID) != 32 || compactUUID[12] != '7' || !strings.ContainsRune("89ab", rune(compactUUID[16])) {
		t.Errorf("UUID = %q, want UUIDv7 with RFC variant", created.UUID)
	}
	state, err := os.ReadFile(filepath.Join(created.RecordDir, "aidlc-state.md"))
	if err != nil || string(state) != intentStateStub {
		t.Errorf("state stub = (%q, %v), want exact header", state, err)
	}
	registryPath := filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json")
	registry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry) == 0 || registry[len(registry)-1] != '\n' || strings.HasSuffix(string(registry), "\n\n") {
		t.Errorf("registry ending = %q, want exactly one LF", registry)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(registry, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("registry rows = %s, want existing unknown field and appended row", registry)
	}
	var future map[string]bool
	if err := json.Unmarshal(rows[0]["future"], &future); err != nil || !future["nested"] {
		t.Fatalf("registry rows = %s, want existing unknown field: %v", registry, err)
	}
	newRow := rows[1]
	wantFields := map[string]string{
		"uuid": created.UUID, "slug": created.Slug, "dirName": created.DirName,
		"scope": scope, "status": "in-flight",
	}
	for field, want := range wantFields {
		var got string
		if err := json.Unmarshal(newRow[field], &got); err != nil || got != want {
			t.Errorf("registry %s = (%q, %v), want %q", field, got, err, want)
		}
	}
	var repos []string
	if err := json.Unmarshal(newRow["repos"], &repos); err != nil || len(repos) != 2 || repos[0] != "api" || repos[1] != "web" {
		t.Errorf("registry repos = (%q, %v), want input order", repos, err)
	}
	for path, want := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"):                               "team\n",
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "active-intent"): created.DirName + "\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Errorf("cursor %q = (%q, %v), want %q", path, data, err, want)
		}
	}
	after := snapshotSpaceTree(t, project)
	for _, mutable := range []string{
		"aidlc", "aidlc/active-space", "aidlc/spaces/team", "aidlc/spaces/team/intents",
		"aidlc/spaces/team/intents/active-intent", "aidlc/spaces/team/intents/intents.json",
		"aidlc/spaces/team/intents/" + created.DirName,
		"aidlc/spaces/team/intents/" + created.DirName + "/aidlc-state.md",
	} {
		delete(before, filepath.Join(project, filepath.FromSlash(mutable)))
		delete(after, filepath.Join(project, filepath.FromSlash(mutable)))
	}
	if !maps.Equal(before, after) {
		t.Error("CreateIntent changed protected session, knowledge, or config data")
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentIntegrationInvalidRegistryIsUnchanged(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc/spaces/team/intents"},
		map[string]string{
			"aidlc/spaces/team/intents/intents.json": `{"not":"an array"}`,
			"keep":                                   "unchanged",
		},
	)
	before := snapshotSpaceTree(t, project)
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if created != (CreatedIntent{}) || !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("CreateIntent() = (%+v, %v), want zero and fs.ErrInvalid", created, err)
	}
	if !maps.Equal(before, snapshotSpaceTree(t, project)) {
		t.Error("invalid registry changed the project")
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentIntegrationRequiresExistingSpaceAndHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		space string
		ctx   func() context.Context
	}{
		{name: "missing named space", space: "team", ctx: context.Background},
		{name: "synthetic default is not a directory", space: "default", ctx: context.Background},
		{name: "invalid space", space: "../team", ctx: context.Background},
		{name: "canceled before mutation", space: "team", ctx: func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			writeSpaceFixture(t, project, nil, map[string]string{"keep": "unchanged"})
			before := snapshotSpaceTree(t, project)
			created, err := CreateIntent(
				tt.ctx(),
				RootInput{ExplicitDir: project},
				IntentCreateInput{SpaceName: tt.space, Label: "Build Auth"},
			)
			if created != (CreatedIntent{}) || err == nil {
				t.Errorf("CreateIntent() = (%+v, %v), want zero and error", created, err)
			}
			if !maps.Equal(before, snapshotSpaceTree(t, project)) {
				t.Error("rejected input changed the project")
			}
			assertWorkspaceLockAbsent(t, project)
		})
	}
}

func TestCreateIntentIntegrationInitialProjectLink(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(t, project, []string{"aidlc/spaces/team/intents"}, nil)
	base := t.TempDir()
	link := filepath.Join(base, "project-link")
	createSpaceSymlink(t, project, link)
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: link},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if err != nil || created == (CreatedIntent{}) {
		t.Errorf("CreateIntent() = (%+v, %v), want initial project-link success", created, err)
	}
	if !strings.HasPrefix(created.RecordDir, link+string(filepath.Separator)) {
		t.Errorf("RecordDir = %q, want resolved input path through initial link %q", created.RecordDir, link)
	}
	if _, err := os.Stat(filepath.Join(project, "aidlc", "spaces", "team", "intents", created.DirName)); err != nil {
		t.Errorf("record missing at linked project target: %v", err)
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentIntegrationCollisionAndExistingActiveSpace(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc/spaces/team/intents"},
		map[string]string{"aidlc/active-space": "other\n"},
	)
	first, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.DirName != first.DirName+"-2" {
		t.Errorf("collision names = %q, %q, want base then -2", first.DirName, second.DirName)
	}
	if first.UUID == second.UUID {
		t.Errorf("collision creations reused UUID %q", first.UUID)
	}
	activeSpace, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
	if err != nil || string(activeSpace) != "other\n" {
		t.Errorf("active-space = (%q, %v), want existing value preserved", activeSpace, err)
	}
	activeIntent, err := os.ReadFile(
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "active-intent"),
	)
	if err != nil || string(activeIntent) != second.DirName+"\n" {
		t.Errorf("active-intent = (%q, %v), want second created intent", activeIntent, err)
	}
	registry, err := os.ReadFile(
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(registry, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("registry = (%s, %v), want two rows", registry, err)
	}
	for index, row := range rows {
		for _, field := range []string{"scope", "repos"} {
			if _, exists := row[field]; exists {
				t.Errorf("row %d persisted empty field %q", index, field)
			}
		}
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentIntegrationRejectsRegistryNonRegularFilesWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			outside := t.TempDir()
			intentsRoot := filepath.Join(project, "aidlc", "spaces", "team", "intents")
			writeSpaceFixture(t, project, []string{"aidlc/spaces/team/intents"}, map[string]string{"keep": "project"})
			writeSpaceFixture(t, outside, nil, map[string]string{"registry": `[]`, "keep": "outside"})
			registryPath := filepath.Join(intentsRoot, "intents.json")
			switch kind {
			case "symlink":
				createSpaceSymlink(t, filepath.Join(outside, "registry"), registryPath)
			case "directory":
				if err := os.Mkdir(registryPath, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotSpaceTree(t, project)
			outsideBefore := snapshotSpaceTree(t, outside)
			created, err := CreateIntent(
				context.Background(),
				RootInput{ExplicitDir: project},
				IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
			)
			if created != (CreatedIntent{}) || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("CreateIntent() = (%+v, %v), want zero and fs.ErrInvalid", created, err)
			}
			if !maps.Equal(before, snapshotSpaceTree(t, project)) {
				t.Error("rejected registry changed the project")
			}
			if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("rejected registry changed the outside target")
			}
			assertWorkspaceLockAbsent(t, project)
		})
	}
}

func TestCreateIntentIntegrationActiveIntentSymlinkReportsCommittedResult(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	outside := t.TempDir()
	intentsRoot := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	writeSpaceFixture(t, project, []string{"aidlc/spaces/team/intents"}, nil)
	writeSpaceFixture(t, outside, nil, map[string]string{"cursor": "outside", "keep": "outside"})
	createSpaceSymlink(t, filepath.Join(outside, "cursor"), filepath.Join(intentsRoot, "active-intent"))
	outsideBefore := snapshotSpaceTree(t, outside)
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if created == (CreatedIntent{}) || !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("CreateIntent() = (%+v, %v), want committed result and cursor fs.ErrInvalid", created, err)
	}
	if _, statErr := os.Stat(filepath.Join(created.RecordDir, "aidlc-state.md")); statErr != nil {
		t.Errorf("committed record missing: %v", statErr)
	}
	registry, readErr := os.ReadFile(filepath.Join(intentsRoot, "intents.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var rows []json.RawMessage
	if decodeErr := json.Unmarshal(registry, &rows); decodeErr != nil || len(rows) != 1 {
		t.Errorf("committed registry = (%s, %v), want one row", registry, decodeErr)
	}
	if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
		t.Error("active-intent symlink target changed")
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentIntegrationSpaceLinkBoundaries(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"inside relative", "outside relative", "absolute inside", "broken"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			outside := t.TempDir()
			writeSpaceFixture(t, project, []string{"aidlc/spaces", "storage/intents"}, map[string]string{"keep": "project"})
			writeSpaceFixture(t, outside, []string{"storage/intents"}, map[string]string{"keep": "outside"})
			link := filepath.Join(project, "aidlc", "spaces", "team")
			targetPath := filepath.Join(project, "storage")
			switch kind {
			case "outside relative":
				targetPath = filepath.Join(outside, "storage")
			case "broken":
				targetPath = filepath.Join(project, "missing")
			}
			target := targetPath
			if kind != "absolute inside" {
				var err error
				target, err = filepath.Rel(filepath.Dir(link), targetPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, target, link)
			before := snapshotSpaceTree(t, project)
			outsideBefore := snapshotSpaceTree(t, outside)
			created, err := CreateIntent(
				context.Background(),
				RootInput{ExplicitDir: project},
				IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
			)
			if kind == "inside relative" {
				if err != nil || created == (CreatedIntent{}) {
					t.Errorf("CreateIntent() = (%+v, %v), want inner-link success", created, err)
				}
				if _, err := os.Stat(filepath.Join(project, "storage", "intents", created.DirName, "aidlc-state.md")); err != nil {
					t.Errorf("inner-link record missing: %v", err)
				}
			} else {
				if created != (CreatedIntent{}) || err == nil {
					t.Errorf("CreateIntent() = (%+v, %v), want boundary failure", created, err)
				}
				if !maps.Equal(before, snapshotSpaceTree(t, project)) {
					t.Error("rejected space link changed the project")
				}
			}
			if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("space link operation changed outside data")
			}
			assertWorkspaceLockAbsent(t, project)
		})
	}
}

func TestCreateIntentIntegrationSerializesTwoProcesses(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	control := t.TempDir()
	writeSpaceFixture(t, project, []string{"aidlc/spaces/team/intents"}, nil)
	lockOps := systemWorkspaceLockOps()
	receipt, err := acquireWorkspaceLock(
		context.Background(),
		project,
		workspaceLockSettings{maxRetries: 0},
		lockOps,
	)
	if err != nil {
		t.Fatal(err)
	}
	lockHeld := true
	t.Cleanup(func() {
		if lockHeld {
			_ = releaseWorkspaceLock(receipt, lockOps)
		}
	})

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	type process struct {
		cmd        *exec.Cmd
		stdout     bytes.Buffer
		stderr     bytes.Buffer
		readyPath  string
		resultPath string
	}
	processes := make([]*process, 2)
	for index := range processes {
		child := &process{
			readyPath:  filepath.Join(control, "ready-"+string(rune('1'+index))),
			resultPath: filepath.Join(control, "result-"+string(rune('1'+index))),
		}
		child.cmd = exec.CommandContext(ctx, executable, "-test.run=^TestCreateIntentHelperProcess$")
		child.cmd.Env = append(
			os.Environ(),
			"AIDLC_INTENT_CREATE_HELPER=1",
			"AIDLC_INTENT_CREATE_PROJECT="+project,
			"AIDLC_INTENT_CREATE_READY="+child.readyPath,
			"AIDLC_INTENT_CREATE_RESULT="+child.resultPath,
		)
		child.cmd.Stdout = &child.stdout
		child.cmd.Stderr = &child.stderr
		if err := child.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		processes[index] = child
	}
	deadline := time.Now().Add(5 * time.Second)
	for _, child := range processes {
		for {
			if _, err := os.Stat(child.readyPath); err == nil {
				break
			} else if !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
			if time.Now().After(deadline) {
				t.Fatalf("helper did not reach lock wait; stderr=%q", child.stderr.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := releaseWorkspaceLock(receipt, lockOps); err != nil {
		t.Fatal(err)
	}
	lockHeld = false

	created := make([]CreatedIntent, len(processes))
	for index, child := range processes {
		if err := child.cmd.Wait(); err != nil {
			t.Fatalf(
				"helper %d failed: %v; stdout=%q stderr=%q",
				index,
				err,
				child.stdout.String(),
				child.stderr.String(),
			)
		}
		data, err := os.ReadFile(child.resultPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &created[index]); err != nil {
			t.Fatalf("decode helper %d result %q: %v", index, data, err)
		}
	}
	if created[0].UUID == created[1].UUID {
		t.Errorf("concurrent processes reused UUID %q", created[0].UUID)
	}
	dirNames := map[string]bool{created[0].DirName: true, created[1].DirName: true}
	base := strings.TrimSuffix(created[0].DirName, "-2")
	if !dirNames[base] || !dirNames[base+"-2"] {
		t.Errorf("concurrent directories = %v, want base and -2", dirNames)
	}
	registry, err := os.ReadFile(
		filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(registry, &rows); err != nil || len(rows) != 2 {
		t.Errorf("concurrent registry = (%s, %v), want two rows", registry, err)
	}
	assertWorkspaceLockAbsent(t, project)
}

func TestCreateIntentHelperProcess(t *testing.T) {
	if os.Getenv("AIDLC_INTENT_CREATE_HELPER") != "1" {
		return
	}
	project := os.Getenv("AIDLC_INTENT_CREATE_PROJECT")
	readyPath := os.Getenv("AIDLC_INTENT_CREATE_READY")
	resultPath := os.Getenv("AIDLC_INTENT_CREATE_RESULT")
	if project == "" || readyPath == "" || resultPath == "" {
		t.Fatal("intent create helper environment is incomplete")
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err := CreateIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Concurrent Build"},
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertWorkspaceLockAbsent(t *testing.T, project string) {
	t.Helper()

	path := workspaceLockPath(project, os.TempDir(), filepath.EvalSymlinks)
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("workspace lock remains at %q: %v", path, err)
	}
}
