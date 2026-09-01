package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReadIntentsRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	got, err := readIntents(
		RootInput{WorkingDir: "relative"},
		func(string) (*os.Root, error) {
			t.Error("project open called for invalid root")
			return nil, fs.ErrPermission
		},
		(*os.Root).OpenRoot,
		(*os.Root).Close,
	)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("readIntents() error = %v, want fs.ErrInvalid", err)
	}
	assertIntentListing(t, got, IntentListing{})
}

func TestReadIntentsProjectOpenErrorUsesResolvedPriority(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	explicit := filepath.Join(base, "explicit")
	aidlc := filepath.Join(base, "aidlc")
	working := filepath.Join(base, "working")
	cause := errors.New("injected project open failure")
	opened := []string{}
	got, err := readIntents(
		RootInput{ExplicitDir: explicit, AIDLCProjectDir: aidlc, WorkingDir: working},
		func(path string) (*os.Root, error) {
			opened = append(opened, path)
			return nil, cause
		},
		func(*os.Root, string) (*os.Root, error) {
			t.Error("child open called after project open error")
			return nil, fs.ErrInvalid
		},
		func(*os.Root) error {
			t.Error("close called without an acquired root")
			return nil
		},
	)
	if !errors.Is(err, cause) {
		t.Errorf("readIntents() error = %v, want cause %v", err, cause)
	}
	if !slices.Equal(opened, []string{explicit}) {
		t.Errorf("opened paths = %q, want only %q", opened, explicit)
	}
	assertIntentListing(t, got, IntentListing{})
}

func TestReadIntentsMissingIntentRootReturnsMetadata(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "aidlc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "aidlc", "active-space"), []byte("beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	openedChildren := []string{}
	got, err := readIntents(
		RootInput{ExplicitDir: project},
		os.OpenRoot,
		func(root *os.Root, path string) (*os.Root, error) {
			openedChildren = append(openedChildren, path)
			return root.OpenRoot(path)
		},
		(*os.Root).Close,
	)
	if err != nil {
		t.Fatalf("readIntents() error = %v, want nil", err)
	}
	wantChild := filepath.Join("aidlc", "spaces", "beta", "intents")
	if !slices.Equal(openedChildren, []string{wantChild}) {
		t.Errorf("opened child paths = %q, want %q", openedChildren, wantChild)
	}
	assertIntentListing(t, got, IntentListing{
		ProjectRoot: project,
		SpaceName:   "beta",
		Intents:     []Intent{},
	})
}

func TestReadIntentsListsActiveSpace(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeIntentReadFixture(t, project, nil, map[string]string{
		"aidlc/active-space":                                         "beta\n",
		"aidlc/spaces/beta/intents/active-intent":                    "240901-build-auth\n",
		"aidlc/spaces/beta/intents/intents.json":                     `[{"uuid":"uuid","slug":"build-auth","status":"construction","dirName":"240901-build-auth"}]`,
		"aidlc/spaces/beta/intents/240901-build-auth/aidlc-state.md": "state",
	})
	dirName := "240901-build-auth"
	got, err := ReadIntents(RootInput{ExplicitDir: project})
	if err != nil {
		t.Fatalf("ReadIntents() error = %v, want nil", err)
	}
	assertIntentListing(t, got, IntentListing{
		ProjectRoot: project,
		SpaceName:   "beta",
		Intents: []Intent{{
			UUID: "uuid", Slug: "build-auth", Status: "construction", Repos: []string{},
			DirName: &dirName, Active: true,
		}},
	})
}

func TestReadIntentsInvalidSpaceDoesNotOpenChild(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeIntentReadFixture(t, project, nil, map[string]string{"aidlc/active-space": "../outside"})
	childOpened := false
	got, err := readIntents(
		RootInput{ExplicitDir: project},
		os.OpenRoot,
		func(*os.Root, string) (*os.Root, error) {
			childOpened = true
			return nil, fs.ErrPermission
		},
		(*os.Root).Close,
	)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("readIntents() error = %v, want fs.ErrInvalid", err)
	}
	if childOpened {
		t.Error("invalid space reached child open")
	}
	assertIntentListing(t, got, IntentListing{})
}

func TestReadIntentsJoinsQueryAndCloseErrorsInReverseOrder(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeIntentReadFixture(t, project, []string{"aidlc/spaces/default/intents"}, map[string]string{
		"aidlc/spaces/default/intents/intents.json": `[null]`,
	})
	childCloseErr := errors.New("injected child close failure")
	projectCloseErr := errors.New("injected project close failure")
	var projectRoot *os.Root
	var childRoot *os.Root
	closeOrder := []string{}
	got, err := readIntents(
		RootInput{ExplicitDir: project},
		func(path string) (*os.Root, error) {
			var openErr error
			projectRoot, openErr = os.OpenRoot(path)
			return projectRoot, openErr
		},
		func(root *os.Root, path string) (*os.Root, error) {
			var openErr error
			childRoot, openErr = root.OpenRoot(path)
			return childRoot, openErr
		},
		func(root *os.Root) error {
			closeErr := root.Close()
			switch root {
			case childRoot:
				closeOrder = append(closeOrder, "child")
				return errors.Join(closeErr, childCloseErr)
			case projectRoot:
				closeOrder = append(closeOrder, "project")
				return errors.Join(closeErr, projectCloseErr)
			default:
				t.Fatalf("closed unknown root %p", root)
				return closeErr
			}
		},
	)
	if !errors.Is(err, fs.ErrInvalid) || !errors.Is(err, childCloseErr) || !errors.Is(err, projectCloseErr) {
		t.Errorf("readIntents() error = %v, want query and both close causes", err)
	}
	if !slices.Equal(closeOrder, []string{"child", "project"}) {
		t.Errorf("close order = %q, want child then project", closeOrder)
	}
	assertIntentListing(t, got, IntentListing{})
}

func assertIntentListing(t *testing.T, got, want IntentListing) {
	t.Helper()

	if got.ProjectRoot != want.ProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", got.ProjectRoot, want.ProjectRoot)
	}
	if got.SpaceName != want.SpaceName {
		t.Errorf("SpaceName = %q, want %q", got.SpaceName, want.SpaceName)
	}
	assertIntents(t, got.Intents, want.Intents)
}

func writeIntentReadFixture(t *testing.T, root string, dirs []string, files map[string]string) {
	t.Helper()

	for _, name := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
