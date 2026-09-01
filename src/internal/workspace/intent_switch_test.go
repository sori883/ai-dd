package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSwitchIntentSelectsExactDirectory(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	intentsPath := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	dirName := "240901-build-auth"
	if err := os.MkdirAll(filepath.Join(intentsPath, dirName), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"): " \tteam \n",
		filepath.Join(intentsPath, "intents.json"): `[
            {"uuid":"one","slug":"build-auth","status":"planning","dirName":"240901-build-auth"}
        ]`,
		filepath.Join(intentsPath, dirName, "aidlc-state.md"): "state",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	saved := ""
	selection, err := switchIntent(
		RootInput{ExplicitDir: project},
		dirName,
		intentSwitchOps{
			openProject: os.OpenRoot,
			openChild:   (*os.Root).OpenRoot,
			closeRoot:   (*os.Root).Close,
			listIntents: ListIntents,
			completeActiveSpace: func(*os.Root, string) error {
				return nil
			},
			saveActiveIntent: func(_ *os.Root, name string) error {
				saved = name
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("switchIntent() error = %v, want nil", err)
	}
	want := IntentSelection{SpaceName: "team", DirName: dirName}
	if selection != want {
		t.Errorf("switchIntent() selection = %+v, want %+v", selection, want)
	}
	if saved != dirName {
		t.Errorf("saved intent = %q, want %q", saved, dirName)
	}
}

func TestResolveIntentTargetSelectsUniqueSlug(t *testing.T) {
	t.Parallel()

	dirName := "240901-build-auth"
	got, err := resolveIntentTarget(
		[]Intent{{Slug: "build-auth", DirName: &dirName}},
		"build-auth",
	)
	if err != nil || got != dirName {
		t.Errorf("resolveIntentTarget() = (%q, %v), want (%q, nil)", got, err, dirName)
	}
}

func TestResolveIntentTargetRejectsAmbiguousSlug(t *testing.T) {
	t.Parallel()

	first := "240901-build-auth"
	second := "240902-build-auth"
	got, err := resolveIntentTarget(
		[]Intent{
			{Slug: "build-auth", DirName: &first},
			{Slug: "build-auth", DirName: &second},
		},
		"build-auth",
	)
	if got != "" || err == nil {
		t.Fatalf("resolveIntentTarget() = (%q, %v), want empty and ambiguous error", got, err)
	}
	for _, candidate := range []string{first, second} {
		if !strings.Contains(err.Error(), candidate) {
			t.Errorf("ambiguous error %q is missing candidate %q", err, candidate)
		}
	}
}

func TestResolveIntentTargetGuards(t *testing.T) {
	t.Parallel()

	exact := "build-auth"
	other := "240901-build-auth"
	orphan := "240902-orphan"
	tests := []struct {
		name      string
		intents   []Intent
		target    string
		want      string
		wantError string
	}{
		{
			name: "exact directory wins over duplicate slug",
			intents: []Intent{
				{Slug: "build-auth", DirName: &other},
				{Slug: "build-auth", DirName: &exact},
			},
			target: exact,
			want:   exact,
		},
		{
			name:      "unknown",
			intents:   []Intent{{Slug: "build-auth", DirName: &other}},
			target:    "missing",
			wantError: "unknown intent",
		},
		{
			name:      "registry only",
			intents:   []Intent{{Slug: "registry-only"}},
			target:    "registry-only",
			wantError: "unknown intent",
		},
		{
			name:    "orphan slug",
			intents: []Intent{{Slug: "orphan", DirName: &orphan}},
			target:  "orphan",
			want:    orphan,
		},
		{
			name:      "case sensitive",
			intents:   []Intent{{Slug: "Build-Auth", DirName: &other}},
			target:    "build-auth",
			wantError: "unknown intent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveIntentTarget(tt.intents, tt.target)
			if got != tt.want {
				t.Errorf("resolveIntentTarget() name = %q, want %q", got, tt.want)
			}
			if tt.wantError == "" && err != nil {
				t.Errorf("resolveIntentTarget() error = %v, want nil", err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Errorf("resolveIntentTarget() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestSwitchIntentSavesActiveIntent(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	dirName := "240901-build-auth"
	intentsPath := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(filepath.Join(intentsPath, dirName), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"):       " \tteam \n",
		filepath.Join(intentsPath, dirName, "aidlc-state.md"): "state",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
	if err != nil {
		t.Fatalf("SwitchIntent() error = %v, want nil", err)
	}
	if selection != (IntentSelection{SpaceName: "team", DirName: dirName}) {
		t.Errorf("SwitchIntent() selection = %+v, want team and %q", selection, dirName)
	}
	data, err := os.ReadFile(filepath.Join(intentsPath, "active-intent"))
	if err != nil || string(data) != dirName+"\n" {
		t.Errorf("active-intent = (%q, %v), want %q", data, err, dirName+"\n")
	}
	spaceData, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
	if err != nil || string(spaceData) != " \tteam \n" {
		t.Errorf("existing active-space = (%q, %v), want original bytes", spaceData, err)
	}
}

func TestSwitchIntentResavesSameTargetWithLFAndPreservesMode(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	dirName := "240901-build-auth"
	intentsPath := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(filepath.Join(intentsPath, dirName), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"):       "team\n",
		filepath.Join(intentsPath, "active-intent"):           dirName,
		filepath.Join(intentsPath, dirName, "aidlc-state.md"): "state",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cursorPath := filepath.Join(intentsPath, "active-intent")
	if err := os.Chmod(cursorPath, 0o640); err != nil {
		t.Fatal(err)
	}

	selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
	if err != nil {
		t.Fatalf("SwitchIntent() error = %v, want nil", err)
	}
	wantSelection := IntentSelection{SpaceName: "team", DirName: dirName}
	if selection != wantSelection {
		t.Errorf("SwitchIntent() selection = %+v, want %+v", selection, wantSelection)
	}
	data, err := os.ReadFile(cursorPath)
	if err != nil || string(data) != dirName+"\n" {
		t.Errorf("active-intent = (%q, %v), want %q", data, err, dirName+"\n")
	}
	info, err := os.Stat(cursorPath)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("active-intent mode = (%v, %v), want 0640", info, err)
	}
}

func TestSwitchIntentCompletesMissingActiveSpace(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	dirName := "240901-build-auth"
	intentsPath := filepath.Join(project, "aidlc", "spaces", "default", "intents")
	if err := os.MkdirAll(filepath.Join(intentsPath, dirName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(intentsPath, dirName, "aidlc-state.md"),
		[]byte("state"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	selection, err := SwitchIntent(RootInput{ExplicitDir: project}, dirName)
	if err != nil {
		t.Fatalf("SwitchIntent() error = %v, want nil", err)
	}
	if selection.SpaceName != "default" {
		t.Errorf("selected space = %q, want default", selection.SpaceName)
	}
	data, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
	if err != nil || string(data) != "default\n" {
		t.Errorf("active-space = (%q, %v), want default and LF", data, err)
	}
}

func TestSwitchIntentUsesSharedSpaceAndBestEffortCompletion(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	intentsPath := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(intentsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "aidlc", "active-space"), []byte("team\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirName := "240901-build-auth"
	completionCause := errors.New("injected active-space completion failure")
	var override *string
	steps := []string{}
	selection, err := switchIntent(
		RootInput{ExplicitDir: project},
		dirName,
		intentSwitchOps{
			openProject: os.OpenRoot,
			openChild:   (*os.Root).OpenRoot,
			closeRoot:   (*os.Root).Close,
			listIntents: func(_ fs.FS, activeOverride *string) ([]Intent, error) {
				steps = append(steps, "list")
				override = activeOverride
				return []Intent{{Slug: "build-auth", DirName: &dirName}}, nil
			},
			completeActiveSpace: func(_ *os.Root, space string) error {
				steps = append(steps, "complete "+space)
				return completionCause
			},
			saveActiveIntent: func(_ *os.Root, name string) error {
				steps = append(steps, "save "+name)
				return nil
			},
		},
	)
	if err != nil || selection != (IntentSelection{SpaceName: "team", DirName: dirName}) {
		t.Errorf("switchIntent() = (%+v, %v), want selected intent despite completion failure", selection, err)
	}
	if override == nil || *override != "" {
		t.Errorf("active override = %v, want non-nil empty value", override)
	}
	wantSteps := []string{"list", "complete team", "save " + dirName}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want %q", steps, wantSteps)
	}
}

func TestSwitchIntentRejectsBeforeCursorWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(*testing.T, string)
		input      func(string) RootInput
		target     string
		wantCause  error
		wantPhrase string
	}{
		{
			name:      "relative root",
			input:     func(string) RootInput { return RootInput{} },
			target:    "target",
			wantCause: fs.ErrInvalid,
		},
		{
			name: "missing project",
			input: func(project string) RootInput {
				return RootInput{ExplicitDir: filepath.Join(project, "missing")}
			},
			target:    "target",
			wantCause: fs.ErrNotExist,
		},
		{
			name: "invalid active space",
			setup: func(t *testing.T, project string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(project, "aidlc"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(project, "aidlc", "active-space"), []byte("../bad\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			input:     func(project string) RootInput { return RootInput{ExplicitDir: project} },
			target:    "target",
			wantCause: fs.ErrInvalid,
		},
		{
			name: "missing intents root",
			setup: func(t *testing.T, project string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(project, "aidlc"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
			input:     func(project string) RootInput { return RootInput{ExplicitDir: project} },
			target:    "target",
			wantCause: fs.ErrNotExist,
		},
		{
			name: "invalid registry",
			setup: func(t *testing.T, project string) {
				t.Helper()
				intents := filepath.Join(project, "aidlc", "spaces", "default", "intents")
				if err := os.MkdirAll(intents, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(
					filepath.Join(intents, "intents.json"),
					[]byte(`[{"uuid":1,"slug":"bad","status":"planning"}]`),
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			},
			input:     func(project string) RootInput { return RootInput{ExplicitDir: project} },
			target:    "bad",
			wantCause: fs.ErrInvalid,
		},
		{
			name: "unknown target",
			setup: func(t *testing.T, project string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(project, "aidlc", "spaces", "default", "intents"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
			input:      func(project string) RootInput { return RootInput{ExplicitDir: project} },
			target:     "missing",
			wantPhrase: "unknown intent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, project)
			}
			selection, err := SwitchIntent(tt.input(project), tt.target)
			if selection != (IntentSelection{}) || err == nil {
				t.Fatalf("SwitchIntent() = (%+v, %v), want zero selection and error", selection, err)
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Errorf("error %v lost cause %v", err, tt.wantCause)
			}
			if tt.wantPhrase != "" && !strings.Contains(err.Error(), tt.wantPhrase) {
				t.Errorf("error %q is missing %q", err, tt.wantPhrase)
			}
			if _, statErr := os.Stat(filepath.Join(project, "aidlc", "active-space")); statErr == nil {
				if tt.name != "invalid active space" {
					t.Error("rejected switch created an active-space cursor")
				}
			}
			if matches, globErr := filepath.Glob(filepath.Join(project, "aidlc", "spaces", "*", "intents", "active-intent")); globErr != nil || len(matches) != 0 {
				t.Errorf("rejected switch active-intent files = %q, error = %v, want none", matches, globErr)
			}
		})
	}
}

func TestSwitchIntentJoinsRootCloseFailures(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	intentsPath := filepath.Join(project, "aidlc", "spaces", "default", "intents")
	if err := os.MkdirAll(intentsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	dirName := "only"
	intentsCloseCause := errors.New("injected intents root close failure")
	projectCloseCause := errors.New("injected project root close failure")
	steps := []string{}
	closeCalls := 0
	selection, err := switchIntent(
		RootInput{ExplicitDir: project},
		dirName,
		intentSwitchOps{
			openProject: os.OpenRoot,
			openChild:   (*os.Root).OpenRoot,
			closeRoot: func(root *os.Root) error {
				closeCalls++
				if closeCalls == 1 {
					steps = append(steps, "intents")
					return errors.Join(root.Close(), intentsCloseCause)
				}
				steps = append(steps, "project")
				return errors.Join(root.Close(), projectCloseCause)
			},
			listIntents: func(fs.FS, *string) ([]Intent, error) {
				return []Intent{{Slug: dirName, DirName: &dirName}}, nil
			},
			completeActiveSpace: func(*os.Root, string) error { return nil },
			saveActiveIntent:    func(*os.Root, string) error { return nil },
		},
	)
	if selection != (IntentSelection{}) {
		t.Errorf("selection = %+v, want zero after close error", selection)
	}
	for _, cause := range []error{intentsCloseCause, projectCloseCause} {
		if !errors.Is(err, cause) {
			t.Errorf("error %v lost close cause %v", err, cause)
		}
	}
	if !slices.Equal(steps, []string{"intents", "project"}) {
		t.Errorf("close order = %q, want intents then project", steps)
	}
}

func TestSwitchIntentSaveErrorIsReturnedAfterResolution(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "aidlc", "spaces", "default", "intents"), 0o700); err != nil {
		t.Fatal(err)
	}
	dirName := "only"
	cause := errors.New("injected active-intent save failure")
	selection, err := switchIntent(
		RootInput{ExplicitDir: project},
		dirName,
		intentSwitchOps{
			openProject: os.OpenRoot,
			openChild:   (*os.Root).OpenRoot,
			closeRoot:   (*os.Root).Close,
			listIntents: func(fs.FS, *string) ([]Intent, error) {
				return []Intent{{Slug: dirName, DirName: &dirName}}, nil
			},
			completeActiveSpace: func(*os.Root, string) error { return nil },
			saveActiveIntent:    func(*os.Root, string) error { return cause },
		},
	)
	if selection != (IntentSelection{}) || !errors.Is(err, cause) {
		t.Errorf("switchIntent() = (%+v, %v), want zero and save cause", selection, err)
	}
}
