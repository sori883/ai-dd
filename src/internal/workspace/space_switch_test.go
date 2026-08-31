package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func TestSwitchSpaceInvalidNameBeforeOpen(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "help", "-h"} {
		t.Run("raw "+raw, func(t *testing.T) {
			t.Parallel()

			name, err := switchSpace(
				RootInput{},
				raw,
				func(string) (*os.Root, error) {
					t.Error("invalid name reached project open")
					return nil, fs.ErrPermission
				},
				(*os.Root).Close,
				saveSpaceCursor,
			)
			if name != "" || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("switchSpace() = (%q, %v), want empty name and fs.ErrInvalid", name, err)
			}
		})
	}
}

func TestSwitchSpaceRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input RootInput
	}{
		{name: "empty"},
		{name: "relative cwd", input: RootInput{WorkingDir: "relative"}},
		{name: "relative explicit", input: RootInput{ExplicitDir: "project"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, err := SwitchSpace(tt.input, "default")
			if name != "" || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("SwitchSpace() = (%q, %v), want empty name and fs.ErrInvalid", name, err)
			}
		})
	}
}

func TestSwitchSpaceProjectOpenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cause error
	}{
		{name: "missing", cause: fs.ErrNotExist},
		{name: "permission", cause: fs.ErrPermission},
		{name: "other", cause: errors.New("open failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			working := t.TempDir()
			project := filepath.Join(working, "selected")
			opened := []string{}
			name, err := switchSpace(
				RootInput{ExplicitDir: project, WorkingDir: working},
				"default",
				func(path string) (*os.Root, error) {
					opened = append(opened, path)
					return nil, tt.cause
				},
				func(*os.Root) error {
					t.Error("close after unsuccessful open")
					return nil
				},
				func(*os.Root, string) error {
					t.Error("save after unsuccessful open")
					return nil
				},
			)
			if name != "" || !errors.Is(err, tt.cause) {
				t.Errorf(
					"switchSpace() = (%q, %v), want empty name and cause %v",
					name,
					err,
					tt.cause,
				)
			}
			if !slices.Equal(opened, []string{project}) {
				t.Errorf("opened %q, want only %q without fallback", opened, project)
			}
		})
	}
}

func TestSwitchSpaceRootPrecedence(t *testing.T) {
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
			name:  "explicit wins",
			input: RootInput{ExplicitDir: explicit, AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working},
			want:  explicit,
		},
		{
			name: "aidlc wins", input: RootInput{AIDLCProjectDir: aidlc, ClaudeProjectDir: claude, WorkingDir: working},
			want: aidlc,
		},
		{name: "claude wins", input: RootInput{ClaudeProjectDir: claude, WorkingDir: working}, want: claude},
		{name: "cwd fallback", input: RootInput{WorkingDir: working}, want: working},
		{name: "relative is resolved", input: RootInput{ExplicitDir: "../explicit", WorkingDir: working}, want: explicit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opened := []string{}
			name, err := switchSpace(
				tt.input,
				"default",
				func(path string) (*os.Root, error) {
					opened = append(opened, path)
					return nil, fs.ErrPermission
				},
				(*os.Root).Close,
				saveSpaceCursor,
			)
			if name != "" || !errors.Is(err, fs.ErrPermission) {
				t.Errorf("switchSpace() = (%q, %v), want empty name and open cause", name, err)
			}
			if !slices.Equal(opened, []string{tt.want}) {
				t.Errorf("opened paths=%q, want only %q", opened, tt.want)
			}
		})
	}
}

func TestSaveCursorInRootOpenFailure(t *testing.T) {
	t.Parallel()

	cause := fs.ErrPermission
	err := saveCursorInRoot(
		"default",
		func() (*os.Root, error) { return nil, cause },
		func(*os.Root) error {
			t.Error("close called without acquired aidlc root")
			return nil
		},
	)
	if !errors.Is(err, cause) {
		t.Errorf("saveCursorInRoot() error = %v, want cause %v", err, cause)
	}
}

func TestReplaceSpaceCursorNonRegular(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode fs.FileMode
	}{
		{name: "directory", mode: fs.ModeDir},
		{name: "link", mode: fs.ModeSymlink},
		{name: "named pipe", mode: fs.ModeNamedPipe},
		{name: "device", mode: fs.ModeDevice},
		{name: "socket", mode: fs.ModeSocket},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, err := fs.Stat(fstest.MapFS{"active-space": &fstest.MapFile{Mode: tt.mode}}, "active-space")
			if err != nil {
				t.Fatal(err)
			}
			ops := successfulCursorOps()
			ops.lstat = func(string) (fs.FileInfo, error) { return info, nil }
			ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
				t.Error("nonregular cursor reached temporary file creation")
				return nil, fs.ErrPermission
			}
			if err := replaceSpaceCursor("default", ops); !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("replaceSpaceCursor() error=%v, want fs.ErrInvalid", err)
			}
		})
	}
}

func TestReplaceSpaceCursorPreservesPermissions(t *testing.T) {
	t.Parallel()

	info, err := fs.Stat(fstest.MapFS{
		"active-space": &fstest.MapFile{Mode: fs.ModeSetuid | 0o640},
	}, "active-space")
	if err != nil {
		t.Fatal(err)
	}
	ops := successfulCursorOps()
	steps := []string{}
	ops.lstat = func(string) (fs.FileInfo, error) { return info, nil }
	ops.openFile = func(_ string, _ int, mode fs.FileMode) (*os.File, error) {
		if mode != 0o600 {
			t.Errorf("replacement temp mode = %o, want 600 before writing", mode)
		}
		return nil, nil
	}
	ops.write = func(_ *os.File, content string) (int, error) {
		steps = append(steps, "write")
		return len(content), nil
	}
	ops.chmod = func(_ *os.File, mode fs.FileMode) error {
		steps = append(steps, "chmod")
		if mode != 0o640 {
			t.Errorf("restored mode = %o, want only permission bits 640", mode)
		}
		return nil
	}
	ops.close = func(*os.File) error {
		steps = append(steps, "close")
		return nil
	}
	ops.rename = func(string, string) error {
		steps = append(steps, "rename")
		return nil
	}
	if err := replaceSpaceCursor("default", ops); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(steps, []string{"write", "chmod", "close", "rename"}) {
		t.Errorf("replacement steps = %q, want write, chmod, close, rename", steps)
	}
}

func TestReplaceSpaceCursorShortWrite(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	ops.write = func(_ *os.File, content string) (int, error) { return len(content) - 1, nil }
	ops.rename = func(string, string) error {
		t.Error("short write reached rename")
		return nil
	}
	if err := replaceSpaceCursor("default", ops); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("replaceSpaceCursor() error = %v, want io.ErrShortWrite", err)
	}
}

func TestReplaceSpaceCursorRetriesCollision(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	attempts := 0
	ops.tempName = func() string {
		attempts++
		return fmt.Sprintf(".active-space-%d", attempts)
	}
	ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
		if flags != os.O_WRONLY|os.O_CREATE|os.O_EXCL || mode != 0o666 {
			t.Errorf("open flags/mode = (%d, %o), want write/create/exclusive and 666", flags, mode)
		}
		if name == ".active-space-1" {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
		}
		return nil, nil
	}
	ops.rename = func(from, to string) error {
		if from != ".active-space-2" || to != "active-space" {
			t.Errorf("rename(%q, %q), want second owned temp to cursor", from, to)
		}
		return nil
	}
	ops.remove = func(name string) error {
		t.Errorf("removed unowned or published temp %q", name)
		return nil
	}
	if err := replaceSpaceCursor("default", ops); err != nil {
		t.Errorf("replaceSpaceCursor() error = %v, want collision retry success", err)
	}
	if attempts != 2 {
		t.Errorf("temp attempts = %d, want 2", attempts)
	}
}

func TestReplaceSpaceCursorCollisionLimit(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	attempts := 0
	ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
		attempts++
		return nil, fmt.Errorf("collision: %w", fs.ErrExist)
	}
	ops.remove = func(name string) error {
		t.Errorf("removed unowned collision %q", name)
		return nil
	}
	if err := replaceSpaceCursor("default", ops); !errors.Is(err, fs.ErrExist) {
		t.Errorf("replaceSpaceCursor() error = %v, want fs.ErrExist", err)
	}
	if attempts != cursorTempAttempts {
		t.Errorf("attempts = %d, want bounded limit %d", attempts, cursorTempAttempts)
	}
}

func TestReplaceSpaceCursorFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		failures []string
		steps    []string
	}{
		{name: "inspect", failures: []string{"inspect"}, steps: []string{"inspect"}},
		{name: "create", failures: []string{"open"}, steps: []string{"inspect", "open"}},
		{
			name: "write", failures: []string{"write"},
			steps: []string{"inspect", "open", "write", "close", "remove"},
		},
		{
			name: "chmod", failures: []string{"chmod"},
			steps: []string{"inspect", "open", "write", "chmod", "close", "remove"},
		},
		{
			name: "close", failures: []string{"close"},
			steps: []string{"inspect", "open", "write", "chmod", "close", "remove"},
		},
		{
			name: "rename", failures: []string{"rename"},
			steps: []string{"inspect", "open", "write", "chmod", "close", "rename", "remove"},
		},
		{
			name: "write close cleanup joined", failures: []string{"write", "close", "remove"},
			steps: []string{"inspect", "open", "write", "close", "remove"},
		},
		{
			name: "chmod close cleanup joined", failures: []string{"chmod", "close", "remove"},
			steps: []string{"inspect", "open", "write", "chmod", "close", "remove"},
		},
		{
			name: "rename cleanup joined", failures: []string{"rename", "remove"},
			steps: []string{"inspect", "open", "write", "chmod", "close", "rename", "remove"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			failures := map[string]error{}
			for _, failure := range tt.failures {
				failures[failure] = fmt.Errorf("injected %s failure", failure)
			}
			steps := []string{}
			step := func(name string) error {
				steps = append(steps, name)
				return failures[name]
			}
			info, err := fs.Stat(fstest.MapFS{"active-space": &fstest.MapFile{Mode: 0o640}}, "active-space")
			if err != nil {
				t.Fatal(err)
			}
			ops := successfulCursorOps()
			ops.lstat = func(name string) (fs.FileInfo, error) {
				if name != "active-space" {
					t.Errorf("inspected %q, want active-space", name)
				}
				return info, step("inspect")
			}
			ops.openFile = func(string, int, fs.FileMode) (*os.File, error) { return nil, step("open") }
			ops.write = func(_ *os.File, content string) (int, error) { return len(content), step("write") }
			ops.chmod = func(*os.File, fs.FileMode) error { return step("chmod") }
			ops.close = func(*os.File) error { return step("close") }
			ops.rename = func(string, string) error { return step("rename") }
			ops.remove = func(name string) error {
				if name != ".active-space-test" {
					t.Errorf("removed %q, want only owned temporary cursor", name)
				}
				return step("remove")
			}
			err = replaceSpaceCursor("default", ops)
			for _, cause := range failures {
				if !errors.Is(err, cause) {
					t.Errorf("error %v lost cause %v", err, cause)
				}
			}
			if !slices.Equal(steps, tt.steps) {
				t.Errorf("steps = %q, want %q", steps, tt.steps)
			}
		})
	}
}

func successfulCursorOps() cursorOps {
	return cursorOps{
		lstat:    func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist },
		tempName: func() string { return ".active-space-test" },
		openFile: func(string, int, fs.FileMode) (*os.File, error) { return nil, nil },
		write:    func(_ *os.File, content string) (int, error) { return len(content), nil },
		chmod:    func(*os.File, fs.FileMode) error { return nil },
		close:    func(*os.File) error { return nil },
		rename:   func(string, string) error { return nil },
		remove:   func(string) error { return nil },
	}
}
