package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"testing"
)

func TestReplaceCursorUsesConfiguredCursorName(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	var inspected, opened, renamedFrom, renamedTo, content string
	ops.lstat = func(name string) (fs.FileInfo, error) {
		inspected = name
		return nil, fs.ErrNotExist
	}
	ops.tempName = func() string { return ".active-intent-test" }
	ops.openFile = func(name string, _ int, _ fs.FileMode) (*os.File, error) {
		opened = name
		return nil, nil
	}
	ops.write = func(_ *os.File, value string) (int, error) {
		content = value
		return len(value), nil
	}
	ops.rename = func(from, to string) error {
		renamedFrom, renamedTo = from, to
		return nil
	}
	if err := replaceCursor("active-intent", "240901-build-auth", ops); err != nil {
		t.Fatal(err)
	}
	if inspected != "active-intent" || opened != ".active-intent-test" {
		t.Errorf("inspect/open = %q/%q, want configured cursor/temp names", inspected, opened)
	}
	if content != "240901-build-auth\n" {
		t.Errorf("written content = %q, want selected directory and LF", content)
	}
	if renamedFrom != ".active-intent-test" || renamedTo != "active-intent" {
		t.Errorf("rename = %q -> %q, want configured temp -> cursor", renamedFrom, renamedTo)
	}
}

func TestCompleteCursorNoReplacePublishesAfterClose(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	steps := []string{}
	ops.write = func(_ *os.File, value string) (int, error) {
		steps = append(steps, "write "+value)
		return len(value), nil
	}
	ops.close = func(*os.File) error {
		steps = append(steps, "close")
		return nil
	}
	ops.link = func(from, to string) error {
		steps = append(steps, "link "+from+" "+to)
		return nil
	}
	ops.remove = func(name string) error {
		steps = append(steps, "remove "+name)
		return nil
	}
	if err := completeCursorNoReplace("active-space", "team", ops); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"write team\n",
		"close",
		"link .active-space-test active-space",
		"remove .active-space-test",
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps = %q, want %q", steps, want)
	}
}

func TestCompleteCursorNoReplacePreservesConcurrentValue(t *testing.T) {
	t.Parallel()

	ops := successfulCursorOps()
	ops.link = func(string, string) error { return fs.ErrExist }
	removed := false
	ops.remove = func(string) error {
		removed = true
		return nil
	}
	if err := completeCursorNoReplace("active-space", "team", ops); err != nil {
		t.Errorf("completeCursorNoReplace() error = %v, want concurrent cursor preserved", err)
	}
	if !removed {
		t.Error("owned staging cursor was not removed")
	}
}

func TestCompleteCursorNoReplaceUsesExclusiveBoundedStaging(t *testing.T) {
	t.Parallel()

	t.Run("retries collision", func(t *testing.T) {
		t.Parallel()

		ops := successfulCursorOps()
		attempts := 0
		ops.tempName = func() string {
			attempts++
			return fmt.Sprintf(".active-space-%d", attempts)
		}
		ops.openFile = func(name string, flags int, mode fs.FileMode) (*os.File, error) {
			if attempts == 1 {
				return nil, fs.ErrExist
			}
			wantFlags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
			if name != ".active-space-2" || flags != wantFlags || mode != 0o666 {
				t.Errorf("openFile(%q, %d, %04o), want exclusive second staging with 0666", name, flags, mode)
			}
			return nil, nil
		}
		ops.link = func(string, string) error { return nil }
		if err := completeCursorNoReplace("active-space", "team", ops); err != nil {
			t.Fatal(err)
		}
		if attempts != 2 {
			t.Errorf("attempts = %d, want 2", attempts)
		}
	})

	t.Run("bounded collision", func(t *testing.T) {
		t.Parallel()

		ops := successfulCursorOps()
		attempts := 0
		ops.tempName = func() string {
			attempts++
			return ".active-space-collision"
		}
		ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
			return nil, fs.ErrExist
		}
		ops.remove = func(string) error {
			t.Error("unowned collision was removed")
			return nil
		}
		if err := completeCursorNoReplace("active-space", "team", ops); !errors.Is(err, fs.ErrExist) {
			t.Errorf("error = %v, want fs.ErrExist", err)
		}
		if attempts != cursorTempAttempts {
			t.Errorf("attempts = %d, want %d", attempts, cursorTempAttempts)
		}
	})
}

func TestCompleteCursorNoReplaceFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stage      string
		wantCause  error
		wantLinked bool
	}{
		{name: "create", stage: "create", wantCause: fs.ErrPermission},
		{name: "write", stage: "write", wantCause: fs.ErrPermission},
		{name: "short write", stage: "short", wantCause: io.ErrShortWrite},
		{name: "close", stage: "close", wantCause: fs.ErrPermission},
		{name: "link", stage: "link", wantCause: fs.ErrPermission, wantLinked: true},
		{name: "cleanup", stage: "cleanup", wantCause: fs.ErrPermission, wantLinked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ops := successfulCursorOps()
			linked := false
			removed := false
			if tt.stage == "create" {
				ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
					return nil, tt.wantCause
				}
			}
			ops.write = func(_ *os.File, value string) (int, error) {
				switch tt.stage {
				case "write":
					return 0, tt.wantCause
				case "short":
					return len(value) - 1, nil
				default:
					return len(value), nil
				}
			}
			ops.close = func(*os.File) error {
				if tt.stage == "close" {
					return tt.wantCause
				}
				return nil
			}
			ops.link = func(string, string) error {
				linked = true
				if tt.stage == "link" {
					return tt.wantCause
				}
				return nil
			}
			ops.remove = func(string) error {
				removed = true
				if tt.stage == "cleanup" {
					return tt.wantCause
				}
				return nil
			}
			err := completeCursorNoReplace("active-space", "team", ops)
			if !errors.Is(err, tt.wantCause) {
				t.Errorf("error %v lost cause %v", err, tt.wantCause)
			}
			if linked != tt.wantLinked {
				t.Errorf("linked = %t, want %t", linked, tt.wantLinked)
			}
			if removed != (tt.stage != "create") {
				t.Errorf("removed = %t, want owned temp cleanup only", removed)
			}
		})
	}
}
