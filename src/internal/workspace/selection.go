package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Selection contains the metadata read for one project's current space.
// ProjectRoot is the resolved input path, not a symlink-expanded canonical path.
// IntentDirs is non-nil on success. HasActiveIntent reports selection only, not
// state validity or a guarantee that the intent will still exist after return.
type Selection struct {
	ProjectRoot     string
	SpaceName       string
	IntentDirs      []string
	ActiveIntent    string
	HasActiveIntent bool
}

// ReadSelection connects the workspace readers without modifying workspace state.
// It uses only input for root selection; it does not read process cwd or environment.
// The resolved project path must be absolute. The active-space name must be one
// non-dot [fs.ValidPath] component accepted by [filepath.Localize].
//
// The initial project path may follow symlinks. Child traversal is confined to
// that project root, and subsequent intent reads to the opened intents root.
// All acquired roots are closed internally, joining close failures with earlier
// errors. Any returned error is accompanied by the zero value of Selection.
// Child absence yields an empty selection; existing readers still absorb their
// own read errors, so not every I/O failure is exposed by this function.
func ReadSelection(input RootInput) (Selection, error) {
	return readSelection(
		input,
		os.OpenRoot,
		(*os.Root).OpenRoot,
		(*os.Root).Close,
	)
}

// readSelection injects failure points locally while retaining concrete Root ownership.
func readSelection(
	input RootInput,
	openProject func(string) (*os.Root, error),
	openChild func(*os.Root, string) (*os.Root, error),
	closeRoot func(*os.Root) error,
) (selection Selection, err error) {
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return Selection{}, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return Selection{}, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeRoot(projectRoot); closeErr != nil {
			selection = Selection{}
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()
	spaceName := ActiveSpace(projectRoot.FS())
	spacePath, err := localizeSpace(spaceName)
	if err != nil {
		return Selection{}, err
	}
	selection = Selection{ProjectRoot: projectPath, SpaceName: spaceName, IntentDirs: []string{}}
	childPath := filepath.Join(
		"aidlc",
		"spaces",
		spacePath,
		"intents",
	)
	intentsRoot, err := openChild(projectRoot, childPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return selection, nil
		}
		return Selection{}, fmt.Errorf("open intents root %q: %w", childPath, err)
	}
	defer func() {
		if closeErr := closeRoot(intentsRoot); closeErr != nil {
			selection = Selection{}
			err = errors.Join(err, fmt.Errorf("close intents root %q: %w", childPath, closeErr))
		}
	}()
	intentsFS := intentsRoot.FS()
	selection.IntentDirs = ListIntentDirs(intentsFS)
	selection.ActiveIntent, selection.HasActiveIntent = ActiveIntent(intentsFS, "")
	return selection, nil
}

// localizeSpace validates before conversion or Join can normalize a name.
func localizeSpace(name string) (string, error) {
	isComponent := name != "." && !strings.Contains(name, "/")
	if !fs.ValidPath(name) || !isComponent {
		return "", fmt.Errorf("validate space %q: %w", name, fs.ErrInvalid)
	}
	localized, err := filepath.Localize(name)
	if err != nil {
		return "", fmt.Errorf(
			"localize space %q: %w: %w",
			name,
			fs.ErrInvalid,
			err,
		)
	}
	return localized, nil
}
