package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadSpaces reads the shared-cursor space listing within an existing project.
// It owns the project Root and returns nil on root resolution, open, or close errors.
// Reader fallbacks are unchanged and do not distinguish missing data from read errors.
func ReadSpaces(input RootInput) ([]Space, error) {
	return readSpaces(input, os.OpenRoot, (*os.Root).Close)
}

func readSpaces(
	input RootInput,
	openProject func(string) (*os.Root, error),
	closeProject func(*os.Root) error,
) (spaces []Space, err error) {
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return nil, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeProject(projectRoot); closeErr != nil {
			spaces = nil
			err = fmt.Errorf("close project root %q: %w", projectPath, closeErr)
		}
	}()
	return ListSpaces(projectRoot.FS(), nil), nil
}
