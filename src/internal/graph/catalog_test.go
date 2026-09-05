package graph

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCatalogLeafRejectsOversizedRegularFile(t *testing.T) {
	projectPath := t.TempDir()
	dataPath := filepath.Join(projectPath, ".codex", "tools", "data")
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	catalogPath := filepath.Join(dataPath, "stage-graph.json")
	if err := os.WriteFile(catalogPath, make([]byte, maxCatalogBytes+1), 0o600); err != nil {
		t.Fatalf("WriteFile(catalog): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })

	_, err = readCatalogLeaf(projectRoot, filepath.ToSlash(filepath.Join(".codex", "tools", "data", "stage-graph.json")))
	if !errors.Is(err, ErrCatalogTooLarge) {
		t.Fatalf("readCatalogLeaf() error = %v, want ErrCatalogTooLarge", err)
	}
}

func TestReadCatalogLeafRejectsIdentityReplacementBeforeOpen(t *testing.T) {
	projectPath := t.TempDir()
	dataPath := filepath.Join(projectPath, ".codex", "tools", "data")
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	catalogPath := filepath.Join(dataPath, "stage-graph.json")
	if err := os.WriteFile(catalogPath, []byte(`{"stages":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile(catalog): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })

	name := filepath.ToSlash(filepath.Join(".codex", "tools", "data", "stage-graph.json"))
	replacementPath := catalogPath + ".old"
	openCalls := 0
	var injectionErr error
	ops := catalogLeafReadOps{
		lstat: projectRoot.Lstat,
		open: func(root *os.Root, path string) (*os.File, error) {
			openCalls++
			if err := os.Rename(catalogPath, replacementPath); err != nil {
				injectionErr = err
				return nil, err
			}
			if err := os.WriteFile(catalogPath, []byte(`{"replacement":true}`), 0o600); err != nil {
				injectionErr = err
				return nil, err
			}
			return openCatalogLeaf(root, path)
		},
	}
	_, err = readCatalogLeafWithOps(projectRoot, name, ops)
	if injectionErr != nil {
		t.Fatalf("inject catalog replacement: %v", injectionErr)
	}
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("readCatalogLeafWithOps() error = %v, want ErrInvalidCatalog", err)
	}
	if openCalls != 1 {
		t.Fatalf("catalog open calls = %d, want one after initial Lstat", openCalls)
	}
}
