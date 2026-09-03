//go:build integration

package state

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestReadIntegrationSuccessKeepsRootOpenAndFilesystemUnchanged(t *testing.T) {
	t.Parallel()

	recordDir := t.TempDir()
	statePath := filepath.Join(recordDir, stateFile)
	content := []byte(canonicalStateContent())
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotReadFilesystem(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	got, err := Read(root)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.Scope() != "classic" || got.Version() != 8 {
		t.Errorf("Read() identity = version %d scope %q, want 8/classic", got.Version(), got.Scope())
	}
	if err := assertReadFilesystemUnchanged(recordDir, before); err != nil {
		t.Error(err)
	}
	gotBytes, err := root.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Root.ReadFile() after Read() error = %v", err)
	}
	if !bytes.Equal(gotBytes, content) {
		t.Errorf("state bytes changed: got %q, want %q", gotBytes, content)
	}
	if _, err := root.Stat(stateFile); err != nil {
		t.Errorf("Root remained unusable after Read(): %v", err)
	}
}

func TestReadIntegrationRejectsMissingFileAndKeepsRootOpen(t *testing.T) {
	t.Parallel()

	root := openReadTestRoot(t)
	got, err := Read(root)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Read() error = %v, want fs.ErrNotExist", err)
	}
	if got.Version() != 0 || got.Scope() != "" || len(got.Stages()) != 0 {
		t.Fatalf("Read() returned partial state %#v", got)
	}
	if _, err := root.Stat("."); err != nil {
		t.Errorf("Root remained unusable after missing-file Read(): %v", err)
	}
}

func TestReadIntegrationRejectsNilRootBeforeIO(t *testing.T) {
	t.Parallel()

	got, err := Read(nil)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Read(nil) error = %v, want fs.ErrInvalid", err)
	}
	if got.Version() != 0 || got.Scope() != "" || len(got.Stages()) != 0 {
		t.Fatalf("Read(nil) returned partial state %#v", got)
	}
}

func TestReadIntegrationRejectsNonRegularLeaf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setup        func(string) error
		needsSymlink bool
	}{
		{
			name: "directory",
			setup: func(recordDir string) error {
				return os.Mkdir(filepath.Join(recordDir, stateFile), 0o700)
			},
		},
		{
			name:         "symlink",
			needsSymlink: true,
			setup: func(recordDir string) error {
				return os.WriteFile(filepath.Join(recordDir, "target"), []byte(canonicalStateContent()), 0o600)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recordDir := t.TempDir()
			if err := tt.setup(recordDir); err != nil {
				t.Fatal(err)
			}
			if tt.needsSymlink {
				if err := os.Symlink("target", filepath.Join(recordDir, stateFile)); err != nil {
					if runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission) {
						t.Skipf("Windows symlink permissions: %v", err)
					}
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(recordDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("Root.Close() error = %v", err)
				}
			})

			got, err := Read(root)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Read() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || len(got.Stages()) != 0 {
				t.Fatalf("Read() returned partial state %#v", got)
			}
			if _, err := root.Stat("."); err != nil {
				t.Errorf("Root remained unusable after nonregular Read(): %v", err)
			}
		})
	}
}

func TestReadIntegrationRejectsInvalidContentWithoutMutation(t *testing.T) {
	t.Parallel()

	recordDir := t.TempDir()
	statePath := filepath.Join(recordDir, stateFile)
	content := []byte("# invalid state\n")
	if err := os.WriteFile(statePath, content, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotReadFilesystem(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	got, err := Read(root)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Read() error = %v, want fs.ErrInvalid", err)
	}
	if got.Version() != 0 || got.Scope() != "" || len(got.Stages()) != 0 {
		t.Fatalf("Read() returned partial state %#v", got)
	}
	if err := assertReadFilesystemUnchanged(recordDir, before); err != nil {
		t.Error(err)
	}
	gotBytes, err := root.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Root.ReadFile() after invalid Read() error = %v", err)
	}
	if !bytes.Equal(gotBytes, content) {
		t.Errorf("state bytes changed: got %q, want %q", gotBytes, content)
	}
}

type readFilesystemSnapshot struct {
	entries []string
	state   []byte
	mode    fs.FileMode
	modTime time.Time
}

func snapshotReadFilesystem(recordDir string) (readFilesystemSnapshot, error) {
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		return readFilesystemSnapshot{}, err
	}
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	sort.Strings(names)

	statePath := filepath.Join(recordDir, stateFile)
	info, err := os.Lstat(statePath)
	if err != nil {
		return readFilesystemSnapshot{}, err
	}
	state, err := os.ReadFile(statePath)
	if err != nil {
		return readFilesystemSnapshot{}, err
	}
	return readFilesystemSnapshot{
		entries: names,
		state:   state,
		mode:    info.Mode(),
		modTime: info.ModTime(),
	}, nil
}

func assertReadFilesystemUnchanged(recordDir string, before readFilesystemSnapshot) error {
	after, err := snapshotReadFilesystem(recordDir)
	if err != nil {
		return err
	}
	if !equalReadStringSlices(before.entries, after.entries) {
		return fmt.Errorf("directory entries changed: before=%v after=%v", before.entries, after.entries)
	}
	if !bytes.Equal(before.state, after.state) {
		return fmt.Errorf("state bytes changed: before=%q after=%q", before.state, after.state)
	}
	if before.mode != after.mode {
		return fmt.Errorf("state mode changed: before=%#o after=%#o", before.mode, after.mode)
	}
	if !before.modTime.Equal(after.modTime) {
		return fmt.Errorf("state mtime changed: before=%v after=%v", before.modTime, after.modTime)
	}
	return nil
}

func equalReadStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func openReadTestRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})
	return root
}
