package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReadSpacesRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RootInput
	}{
		{name: "empty input"},
		{name: "relative working directory", input: RootInput{WorkingDir: "relative"}},
		{name: "relative explicit directory", input: RootInput{ExplicitDir: "project", WorkingDir: "relative"}},
		{name: "relative environment directory", input: RootInput{AIDLCProjectDir: "project"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ReadSpaces(tt.input)
			if got != nil || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadSpaces() = (%v, %v), want nil and fs.ErrInvalid", got, err)
			}
		})
	}
}

func TestReadSpacesProjectOpenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cause error
	}{
		{name: "missing project", cause: fs.ErrNotExist},
		{name: "permission denied", cause: fs.ErrPermission},
		{name: "other failure", cause: errors.New("injected open failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			working := t.TempDir()
			project := filepath.Join(working, "selected")
			openedPaths := []string{}
			got, err := readSpaces(
				RootInput{ExplicitDir: project, WorkingDir: working},
				func(path string) (*os.Root, error) {
					openedPaths = append(openedPaths, path)
					return nil, tt.cause
				},
				func(*os.Root) error {
					t.Error("close called after project open failure")
					return nil
				},
			)
			if got != nil || !errors.Is(err, tt.cause) {
				t.Errorf(
					"readSpaces() = (%v, %v), want nil and cause %v",
					got,
					err,
					tt.cause,
				)
			}
			if !slices.Equal(openedPaths, []string{project}) {
				t.Errorf("opened paths = %q, want only %q without fallback", openedPaths, project)
			}
		})
	}
}

func TestReadSpacesRootPrecedence(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	explicit := filepath.Join(base, "explicit")
	aidlc := filepath.Join(base, "aidlc-env")
	claude := filepath.Join(base, "claude-env")
	working := filepath.Join(base, "working")
	tests := []struct {
		name  string
		input RootInput
		want  string
	}{
		{
			name: "explicit wins",
			input: RootInput{
				ExplicitDir: explicit, AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working,
			},
			want: explicit,
		},
		{
			name:  "aidlc precedes claude",
			input: RootInput{AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working},
			want:  aidlc,
		},
		{
			name:  "claude precedes working directory",
			input: RootInput{ClaudeProjectDir: claude, WorkingDir: working},
			want:  claude,
		},
		{name: "working directory fallback", input: RootInput{WorkingDir: working}, want: working},
		{
			name:  "relative candidate is resolved and cleaned",
			input: RootInput{ExplicitDir: "../explicit/nested/..", WorkingDir: working},
			want:  explicit,
		},
		{
			name:  "absolute candidate ignores relative working directory",
			input: RootInput{ExplicitDir: explicit, WorkingDir: "relative"},
			want:  explicit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			openedPaths := []string{}
			got, err := readSpaces(
				tt.input,
				func(path string) (*os.Root, error) {
					openedPaths = append(openedPaths, path)
					return nil, fs.ErrPermission
				},
				(*os.Root).Close,
			)
			if got != nil || !errors.Is(err, fs.ErrPermission) {
				t.Errorf("readSpaces() = (%v, %v), want nil and open failure", got, err)
			}
			if !slices.Equal(openedPaths, []string{tt.want}) {
				t.Errorf("opened paths = %q, want only %q", openedPaths, tt.want)
			}
		})
	}
}

func TestReadSpacesInvalidRootDoesNotOpen(t *testing.T) {
	t.Parallel()

	got, err := readSpaces(
		RootInput{},
		func(string) (*os.Root, error) {
			t.Error("invalid root reached project open")
			return nil, fs.ErrPermission
		},
		func(*os.Root) error {
			t.Error("invalid root reached project close")
			return nil
		},
	)
	if got != nil || !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("readSpaces() = (%v, %v), want nil and fs.ErrInvalid", got, err)
	}
}
