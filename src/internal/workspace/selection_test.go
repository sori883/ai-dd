package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestReadSelectionRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RootInput
	}{
		{name: "empty input"},
		{name: "relative working directory", input: RootInput{WorkingDir: "relative"}},
		{name: "relative explicit without working directory", input: RootInput{ExplicitDir: "project"}},
		{
			name: "relative candidate and working directory",
			input: RootInput{
				AIDLCProjectDir: "project",
				WorkingDir:      "relative",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ReadSelection(tt.input)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadSelection() error = %v, want fs.ErrInvalid", err)
			}
			assertSelection(t, got, Selection{})
		})
	}
}

func TestReadSelectionProjectOpenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cause error
	}{
		{name: "missing project", cause: fs.ErrNotExist},
		{name: "permission denied", cause: fs.ErrPermission},
		{name: "other failure", cause: errors.New("injected project open failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			projectPath := filepath.Join(base, "explicit")
			openedPaths := []string{}
			got, err := readSelection(
				RootInput{ExplicitDir: projectPath, AIDLCProjectDir: base, WorkingDir: base},
				func(name string) (*os.Root, error) {
					openedPaths = append(openedPaths, name)
					return nil, tt.cause
				},
				func(*os.Root, string) (*os.Root, error) {
					t.Error("child open called after project open failed")
					return nil, fs.ErrInvalid
				},
				func(*os.Root) error {
					t.Error("close called without an acquired root")
					return nil
				},
			)
			if !errors.Is(err, tt.cause) {
				t.Errorf("readSelection() error = %v, want cause %v", err, tt.cause)
			}
			if !slices.Equal(openedPaths, []string{projectPath}) {
				t.Errorf("opened project paths = %q, want only %q", openedPaths, projectPath)
			}
			assertSelection(t, got, Selection{})
		})
	}
}

func TestReadSelectionRootPrecedence(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	explicit := filepath.Join(base, "explicit")
	aidlc := filepath.Join(base, "aidlc-env")
	claude := filepath.Join(base, "claude-env")
	working := filepath.Join(base, "working")
	tests := []struct {
		name     string
		input    RootInput
		expected string
	}{
		{
			name: "explicit wins",
			input: RootInput{
				ExplicitDir: explicit, AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working,
			},
			expected: explicit,
		},
		{
			name:     "aidlc precedes claude",
			input:    RootInput{AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working},
			expected: aidlc,
		},
		{
			name:     "claude precedes working directory",
			input:    RootInput{ClaudeProjectDir: claude, WorkingDir: working},
			expected: claude,
		},
		{
			name:     "working directory fallback",
			input:    RootInput{WorkingDir: working},
			expected: working,
		},
		{
			name:     "relative candidate is resolved and cleaned",
			input:    RootInput{ExplicitDir: "../explicit/nested/..", WorkingDir: working},
			expected: explicit,
		},
		{
			name:     "absolute candidate does not need an absolute working directory",
			input:    RootInput{ExplicitDir: explicit, WorkingDir: "relative"},
			expected: explicit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			openedPaths := []string{}
			got, err := readSelection(
				tt.input,
				func(name string) (*os.Root, error) {
					openedPaths = append(openedPaths, name)
					return nil, fs.ErrPermission
				},
				(*os.Root).OpenRoot,
				(*os.Root).Close,
			)
			if !errors.Is(err, fs.ErrPermission) {
				t.Errorf("readSelection() error = %v, want fs.ErrPermission", err)
			}
			if !slices.Equal(openedPaths, []string{tt.expected}) {
				t.Errorf("opened project paths = %q, want only %q", openedPaths, tt.expected)
			}
			assertSelection(t, got, Selection{})
		})
	}
}

func TestReadSelectionInvalidRootDoesNotOpen(t *testing.T) {
	t.Parallel()

	got, err := readSelection(
		RootInput{},
		func(string) (*os.Root, error) {
			t.Error("project open called for an invalid root")
			return nil, fs.ErrInvalid
		},
		(*os.Root).OpenRoot,
		(*os.Root).Close,
	)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("readSelection() error = %v, want fs.ErrInvalid", err)
	}
	assertSelection(t, got, Selection{})
}

func TestLocalizeSpaceOSNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		space     string
		isInvalid bool
	}{
		{name: "uppercase unicode and underscore", space: "Research_東京"},
		{name: "embedded space", space: "team notes"},
		{name: "no additional trim", space: "\u0085Team\u0085"},
		{name: "null byte", space: "a\x00b", isInvalid: true},
		{name: "backslash", space: "a\\b", isInvalid: runtime.GOOS == "windows"},
		{name: "colon", space: "C:record", isInvalid: runtime.GOOS == "windows"},
		{name: "reserved device", space: "CON", isInvalid: runtime.GOOS == "windows"},
		{name: "reserved device with extension", space: "NUL.txt", isInvalid: runtime.GOOS == "windows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := localizeSpace(tt.space)
			if tt.isInvalid {
				if !errors.Is(err, fs.ErrInvalid) || got != "" {
					t.Errorf("localizeSpace() = (%q, %v), want empty path and fs.ErrInvalid", got, err)
				}
				return
			}
			if err != nil || got != tt.space {
				t.Errorf(
					"localizeSpace() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.space,
				)
			}
		})
	}
}

func TestLocalizeSpacePathRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		space string
	}{
		{name: "empty"},
		{name: "dot", space: "."},
		{name: "parent", space: ".."},
		{name: "traversal", space: "a/../b"},
		{name: "nested", space: "nested/name"},
		{name: "absolute", space: "/other"},
		{name: "invalid utf8", space: "\xff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := localizeSpace(tt.space)
			if !errors.Is(err, fs.ErrInvalid) || got != "" {
				t.Errorf("localizeSpace() = (%q, %v), want empty path and fs.ErrInvalid", got, err)
			}
		})
	}
}

func assertSelection(t *testing.T, got, expected Selection) {
	t.Helper()

	if got.ProjectRoot != expected.ProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", got.ProjectRoot, expected.ProjectRoot)
	}
	if got.SpaceName != expected.SpaceName {
		t.Errorf("SpaceName = %q, want %q", got.SpaceName, expected.SpaceName)
	}
	if (got.IntentDirs == nil) != (expected.IntentDirs == nil) || !slices.Equal(got.IntentDirs, expected.IntentDirs) {
		t.Errorf("IntentDirs = %#v, want %#v", got.IntentDirs, expected.IntentDirs)
	}
	if got.ActiveIntent != expected.ActiveIntent {
		t.Errorf("ActiveIntent = %q, want %q", got.ActiveIntent, expected.ActiveIntent)
	}
	if got.HasActiveIntent != expected.HasActiveIntent {
		t.Errorf("HasActiveIntent = %t, want %t", got.HasActiveIntent, expected.HasActiveIntent)
	}
}
