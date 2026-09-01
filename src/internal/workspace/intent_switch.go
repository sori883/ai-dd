package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// IntentSelection identifies the shared space and intent selected by a switch.
type IntentSelection struct {
	SpaceName string
	DirName   string
}

type intentSwitchOps struct {
	openProject         func(string) (*os.Root, error)
	openChild           func(*os.Root, string) (*os.Root, error)
	closeRoot           func(*os.Root) error
	listIntents         func(fs.FS, *string) ([]Intent, error)
	completeActiveSpace func(*os.Root, string) error
	saveActiveIntent    func(*os.Root, string) error
}

// SwitchIntent selects an existing intent in the shared active space.
// It prefers an exact directory name, then accepts a unique slug match. The
// project and intents roots are owned and closed within this call.
//
// A missing shared active-space cursor is completed without replacing a
// concurrent value. The active-intent cursor is safely replaced inside the
// intents root. Any error is returned with a zero selection, but a cursor may
// already have changed; callers must not interpret failure as a rollback.
func SwitchIntent(input RootInput, target string) (IntentSelection, error) {
	return switchIntent(input, target, intentSwitchOps{
		openProject:         os.OpenRoot,
		openChild:           (*os.Root).OpenRoot,
		closeRoot:           (*os.Root).Close,
		listIntents:         ListIntents,
		completeActiveSpace: completeActiveSpaceCursor,
		saveActiveIntent:    saveIntentCursor,
	})
}

func completeActiveSpaceCursor(projectRoot *os.Root, spaceName string) (err error) {
	const cursorPath = "aidlc/active-space"
	if _, err := projectRoot.Lstat(cursorPath); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect active-space cursor: %w", err)
	}
	aidlcRoot, err := projectRoot.OpenRoot("aidlc")
	if err != nil {
		return fmt.Errorf("open active-space cursor parent: %w", err)
	}
	defer func() {
		if closeErr := aidlcRoot.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close active-space cursor parent: %w", closeErr))
		}
	}()
	return completeCursorNoReplace(
		"active-space",
		spaceName,
		cursorOperations(aidlcRoot, ".active-space-"),
	)
}

func saveIntentCursor(root *os.Root, name string) error {
	return replaceCursor(
		"active-intent",
		name,
		cursorOperations(root, ".active-intent-"),
	)
}

func switchIntent(
	input RootInput,
	target string,
	ops intentSwitchOps,
) (selection IntentSelection, err error) {
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return IntentSelection{}, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := ops.openProject(projectPath)
	if err != nil {
		return IntentSelection{}, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := ops.closeRoot(projectRoot); closeErr != nil {
			selection = IntentSelection{}
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()

	spaceName := ActiveSpace(projectRoot.FS())
	spacePath, err := localizeSpace(spaceName)
	if err != nil {
		return IntentSelection{}, err
	}
	childPath := filepath.Join("aidlc", "spaces", spacePath, "intents")
	intentsRoot, err := ops.openChild(projectRoot, childPath)
	if err != nil {
		return IntentSelection{}, fmt.Errorf("open intents root %q: %w", childPath, err)
	}
	defer func() {
		if closeErr := ops.closeRoot(intentsRoot); closeErr != nil {
			selection = IntentSelection{}
			err = errors.Join(err, fmt.Errorf("close intents root %q: %w", childPath, closeErr))
		}
	}()

	emptyOverride := ""
	intents, err := ops.listIntents(intentsRoot.FS(), &emptyOverride)
	if err != nil {
		return IntentSelection{}, fmt.Errorf("list intents: %w", err)
	}
	dirName, err := resolveIntentTarget(intents, target)
	if err != nil {
		return IntentSelection{}, err
	}
	if ops.completeActiveSpace != nil {
		_ = ops.completeActiveSpace(projectRoot, spaceName)
	}
	if err := ops.saveActiveIntent(intentsRoot, dirName); err != nil {
		return IntentSelection{}, err
	}
	return IntentSelection{SpaceName: spaceName, DirName: dirName}, nil
}

func resolveIntentTarget(intents []Intent, target string) (string, error) {
	for _, intent := range intents {
		if intent.DirName != nil && *intent.DirName == target {
			return *intent.DirName, nil
		}
	}
	matches := []string{}
	for _, intent := range intents {
		if intent.DirName != nil && intent.Slug == target {
			matches = append(matches, *intent.DirName)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous intent %q: matches directories %q", target, matches)
	}
	return "", fmt.Errorf("unknown intent %q: not in the intent list", target)
}
