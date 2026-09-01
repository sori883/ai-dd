package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadIntents reads the active space's registry and record-directory listing.
func ReadIntents(input RootInput) (IntentListing, error) {
	return readIntents(input, os.OpenRoot, (*os.Root).OpenRoot, (*os.Root).Close)
}

func readIntents(
	input RootInput,
	openProject func(string) (*os.Root, error),
	openChild func(*os.Root, string) (*os.Root, error),
	closeRoot func(*os.Root) error,
) (listing IntentListing, err error) {
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return IntentListing{}, fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return IntentListing{}, fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeRoot(projectRoot); closeErr != nil {
			listing = IntentListing{}
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()

	spaceName := ActiveSpace(projectRoot.FS())
	spacePath, err := localizeSpace(spaceName)
	if err != nil {
		return IntentListing{}, err
	}
	listing = IntentListing{ProjectRoot: projectPath, SpaceName: spaceName, Intents: []Intent{}}
	childPath := filepath.Join("aidlc", "spaces", spacePath, "intents")
	intentsRoot, err := openChild(projectRoot, childPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return listing, nil
		}
		return IntentListing{}, fmt.Errorf("open intents root %q: %w", childPath, err)
	}
	defer func() {
		if closeErr := closeRoot(intentsRoot); closeErr != nil {
			listing = IntentListing{}
			err = errors.Join(err, fmt.Errorf("close intents root %q: %w", childPath, closeErr))
		}
	}()

	listing.Intents, err = ListIntents(intentsRoot.FS(), nil)
	if err != nil {
		return IntentListing{}, fmt.Errorf("list intents: %w", err)
	}
	return listing, nil
}
