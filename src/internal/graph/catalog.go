package graph

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

const (
	graphDataPath   = ".codex/tools/data"
	maxCatalogBytes = 8 * 1024 * 1024
)

var (
	// ErrInvalidCatalog identifies a catalog leaf that is not a stable regular
	// file beneath the caller-owned project Root.
	ErrInvalidCatalog = errors.New("graph: invalid catalog leaf")
	// ErrCatalogTooLarge identifies a catalog that exceeds its bounded read.
	ErrCatalogTooLarge = errors.New("graph: catalog exceeds read limit")
)

// LoadFromRoot loads the project catalog through descriptor- and path-bound
// regular-file reads. The general Load(fs.FS) API remains available for
// in-memory and caller-controlled filesystems.
func LoadFromRoot(projectRoot *os.Root) (Snapshot, error) {
	if projectRoot == nil {
		return Snapshot{}, fmt.Errorf("load graph from root: project root is nil: %w", ErrInvalidCatalog)
	}
	stagePath := path.Join(graphDataPath, "stage-graph.json")
	stageData, err := readCatalogLeaf(projectRoot, stagePath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load stage graph: read stage-graph.json: %w", err)
	}
	scopePath := path.Join(graphDataPath, "scope-grid.json")
	scopeData, scopeErr := readCatalogLeaf(projectRoot, scopePath)
	if scopeErr != nil && !errors.Is(scopeErr, fs.ErrNotExist) {
		return Snapshot{}, fmt.Errorf("load scope grid: read scope-grid.json: %w", scopeErr)
	}
	return loadSnapshot(stageData, scopeData, scopeErr)
}

func readCatalogLeaf(projectRoot *os.Root, name string) (content []byte, err error) {
	pathInfo, err := projectRoot.Lstat(name)
	if err != nil {
		return nil, err
	}
	if pathInfo == nil || pathInfo.Mode()&fs.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("catalog %q must be a regular non-symlink file: %w", name, ErrInvalidCatalog)
	}
	file, err := openCatalogLeaf(projectRoot, name)
	if err != nil {
		return nil, fmt.Errorf("open catalog %q: %w", name, err)
	}
	if file == nil {
		return nil, fmt.Errorf("open catalog %q returned nil file: %w", name, ErrInvalidCatalog)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close catalog %q: %w", name, closeErr))
		}
	}()

	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened catalog %q: %w", name, err)
	}
	if !sameCatalogIdentity(pathInfo, opened) {
		return nil, fmt.Errorf("catalog %q changed identity before read: %w", name, ErrInvalidCatalog)
	}
	content, err = io.ReadAll(io.LimitReader(file, maxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read catalog %q: %w", name, err)
	}
	if len(content) > maxCatalogBytes {
		return nil, fmt.Errorf("read catalog %q: %w", name, ErrCatalogTooLarge)
	}
	final, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat catalog %q after read: %w", name, err)
	}
	current, err := projectRoot.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("inspect catalog %q after read: %w", name, err)
	}
	if !sameCatalogIdentity(pathInfo, final) || !sameCatalogIdentity(pathInfo, current) {
		return nil, fmt.Errorf("catalog %q changed identity during read: %w", name, ErrInvalidCatalog)
	}
	return content, nil
}

func sameCatalogIdentity(expected, actual fs.FileInfo) bool {
	return expected != nil && actual != nil && expected.Mode().IsRegular() && actual.Mode().IsRegular() && os.SameFile(expected, actual)
}
