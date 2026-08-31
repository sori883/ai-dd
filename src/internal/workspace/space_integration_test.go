//go:build integration

package workspace

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
)

func TestSpaceReadersFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dirs           []string
		files          map[string]string
		expectedActive string
		expectedSpaces []Space
	}{
		{
			name:           "uninitialized project",
			expectedActive: "default",
			expectedSpaces: []Space{{Name: "default", Active: true}},
		},
		{
			name:           "empty cursor and spaces",
			dirs:           []string{"aidlc/spaces"},
			files:          map[string]string{"aidlc/active-space": ""},
			expectedActive: "default",
			expectedSpaces: []Space{{Name: "default", Active: true}},
		},
		{
			name: "normal cursor and immediate directories",
			dirs: []string{
				"aidlc/spaces/default",
				"aidlc/spaces/research/intents/nested",
				"aidlc/spaces/zeta",
				"aidlc/spaces/.hidden",
				"aidlc/spaces/Alpha",
			},
			files: map[string]string{
				"aidlc/active-space":     "\ufeff research\ufeff\r\n",
				"aidlc/spaces/readme.md": "not a space",
				"keep.txt":               "untouched",
			},
			expectedActive: "research",
			expectedSpaces: []Space{
				{Name: ".hidden"},
				{Name: "Alpha"},
				{Name: "default"},
				{Name: "research", Active: true},
				{Name: "zeta"},
			},
		},
		{
			name:           "missing cursor does not select the sole directory",
			dirs:           []string{"aidlc/spaces/research"},
			expectedActive: "default",
			expectedSpaces: []Space{{Name: "default", Active: true}, {Name: "research"}},
		},
		{
			name:           "cursor path is a directory",
			dirs:           []string{"aidlc/active-space", "aidlc/spaces/research"},
			expectedActive: "default",
			expectedSpaces: []Space{{Name: "default", Active: true}, {Name: "research"}},
		},
		{
			name: "spaces path is a regular file",
			files: map[string]string{
				"aidlc/active-space": "unknown",
				"aidlc/spaces":       "not a directory",
			},
			expectedActive: "unknown",
			expectedSpaces: []Space{{Name: "default"}},
		},
		{
			name: "regular files are not spaces",
			files: map[string]string{
				"aidlc/active-space":    "research",
				"aidlc/spaces/default":  "not a directory",
				"aidlc/spaces/research": "not a directory",
			},
			expectedActive: "research",
			expectedSpaces: []Space{{Name: "default"}},
		},
		{
			name:           "unknown path like cursor is preserved",
			dirs:           []string{"aidlc/spaces/research"},
			files:          map[string]string{"aidlc/active-space": "../outside"},
			expectedActive: "../outside",
			expectedSpaces: []Space{{Name: "default"}, {Name: "research"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeSpaceFixture(
				t,
				root,
				tt.dirs,
				tt.files,
			)
			assertReadOnlySpaceReaders(
				t,
				root,
				tt.expectedActive,
				tt.expectedSpaces,
			)
		})
	}
}

func TestSpaceReadersSymlinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		targetKind     string
		isOutside      bool
		expectedSpaces []Space
	}{
		{
			name:       "directory symlink is included",
			targetKind: "directory",
			expectedSpaces: []Space{
				{Name: "a-kept"},
				{Name: "default"},
				{Name: "m-link", Active: true},
				{Name: "z-unvisited"},
			},
		},
		{
			name:       "file symlink is excluded",
			targetKind: "file",
			expectedSpaces: []Space{
				{Name: "a-kept"},
				{Name: "default"},
				{Name: "z-unvisited"},
			},
		},
		{
			name:           "broken symlink stops enumeration",
			targetKind:     "missing",
			expectedSpaces: []Space{{Name: "a-kept"}, {Name: "default"}},
		},
		{
			name:       "dirfs follows directory symlinks outside the project",
			targetKind: "directory",
			isOutside:  true,
			expectedSpaces: []Space{
				{Name: "a-kept"},
				{Name: "default"},
				{Name: "m-link", Active: true},
				{Name: "z-unvisited"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeSpaceFixture(
				t,
				root,
				[]string{"aidlc/spaces/a-kept", "aidlc/spaces/z-unvisited"},
				map[string]string{"aidlc/active-space": "m-link"},
			)
			targetRoot := root
			if tt.isOutside {
				targetRoot = t.TempDir()
			}
			switch tt.targetKind {
			case "directory":
				writeSpaceFixture(
					t,
					targetRoot,
					[]string{"target"},
					map[string]string{"target/keep.txt": "untouched"},
				)
			case "file":
				writeSpaceFixture(
					t,
					targetRoot,
					[]string{},
					map[string]string{"target": "not a directory"},
				)
			case "missing":
				// Leave the target absent to exercise a real Stat failure.
			default:
				t.Fatalf("unknown target kind %q", tt.targetKind)
			}
			link := filepath.Join(root, filepath.FromSlash("aidlc/spaces/m-link"))
			createSpaceSymlink(t, filepath.Join(targetRoot, "target"), link)
			beforeTarget := snapshotSpaceTree(t, targetRoot)
			assertReadOnlySpaceReaders(
				t,
				root,
				"m-link",
				tt.expectedSpaces,
			)
			if after := snapshotSpaceTree(t, targetRoot); !maps.Equal(after, beforeTarget) {
				t.Errorf("target filesystem changed: before=%v, after=%v", beforeTarget, after)
			}
		})
	}
}

func writeSpaceFixture(t *testing.T, root string, dirs []string, files map[string]string) {
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

func createSpaceSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		// Windows ERROR_PRIVILEGE_NOT_HELD is not always fs.ErrPermission.
		isPermissionError := errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.Errno(1314))
		if runtime.GOOS == "windows" && isPermissionError {
			t.Skipf("Windows symlink permission unavailable (Developer Mode or privilege required): %v", err)
		}
		t.Fatal(err)
	}
}

func assertReadOnlySpaceReaders(t *testing.T, root, expectedActive string, expectedSpaces []Space) {
	t.Helper()

	before := snapshotSpaceTree(t, root)
	projectFS := os.DirFS(root)
	if got := ActiveSpace(projectFS); got != expectedActive {
		t.Errorf("ActiveSpace() = %q, want %q", got, expectedActive)
	}
	if got := ListSpaces(projectFS, nil); !slices.Equal(got, expectedSpaces) {
		t.Errorf("ListSpaces() = %v, want %v", got, expectedSpaces)
	}
	if after := snapshotSpaceTree(t, root); !maps.Equal(after, before) {
		t.Errorf("project filesystem changed: before=%v, after=%v", before, after)
	}
}

type spaceTreeEntry struct {
	mode       fs.FileMode
	modified   int64
	content    string
	linkTarget string
}

func snapshotSpaceTree(t *testing.T, root string) map[string]spaceTreeEntry {
	t.Helper()

	snapshot := map[string]spaceTreeEntry{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := spaceTreeEntry{mode: info.Mode(), modified: info.ModTime().UnixNano()}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			state.linkTarget = target
		case info.Mode().IsRegular():
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state.content = string(content)
		}
		snapshot[path] = state
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
