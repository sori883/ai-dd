package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// SwitchSpace selects a listed space by updating the shared active-space cursor.
// The project must already exist; the synthetic default space need not exist.
// Raw empty/help/-h names are invalid. Other names use the creation slug rules,
// but listed names reserved for creation can be selected.
//
// The cursor is replaced within os.Root boundaries; existing symlink or
// nonregular cursors are rejected. Only existing permission bits are preserved.
// An error returns an empty name, but the cursor may already have changed;
// callers must not interpret failure as a rollback or an atomicity guarantee.
func SwitchSpace(input RootInput, rawName string) (string, error) {
	return switchSpace(
		input,
		rawName,
		os.OpenRoot,
		(*os.Root).Close,
		saveSpaceCursor,
	)
}

func switchSpace(
	input RootInput,
	rawName string,
	openProject func(string) (*os.Root, error),
	closeProject func(*os.Root) error,
	save func(*os.Root, string) error,
) (name string, err error) {
	switch rawName {
	case "", "help", "-h":
		return "", fmt.Errorf("invalid space name %q: %w", rawName, fs.ErrInvalid)
	}
	name = spaceSlug(rawName)
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return "", fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return "", fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeProject(projectRoot); closeErr != nil {
			name = ""
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()
	spaces := ListSpaces(projectRoot.FS(), &name)
	if !slices.ContainsFunc(spaces, func(space Space) bool { return space.Name == name }) {
		return "", fmt.Errorf("unknown space %q: not in the space list", name)
	}
	if err := save(projectRoot, name); err != nil {
		return "", err
	}
	return name, nil
}

func saveSpaceCursor(root *os.Root, name string) error {
	// Match the reference's mkdir default; the process umask controls permissions.
	if err := root.MkdirAll("aidlc", 0o777); err != nil {
		return fmt.Errorf("create cursor parent %q: %w", "aidlc", err)
	}
	return saveCursorInRoot(
		name,
		"active-space",
		".active-space-",
		"aidlc",
		func() (*os.Root, error) { return root.OpenRoot("aidlc") },
		(*os.Root).Close,
	)
}
