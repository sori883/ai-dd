package workspace

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceLockPathUsesCanonicalWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	tempDir := filepath.Join(string(filepath.Separator), "system-temp")
	lexical := filepath.Join(string(filepath.Separator), "links", "project")
	canonical := filepath.Join(string(filepath.Separator), "projects", "Canonical")
	got := workspaceLockPath(
		lexical,
		tempDir,
		func(path string) (string, error) {
			if path != lexical {
				t.Errorf("EvalSymlinks path = %q, want %q", path, lexical)
			}
			return canonical, nil
		},
	)
	wantCanonical := canonical
	if runtime.GOOS == "windows" {
		wantCanonical = filepath.Join(string(filepath.Separator), "projects", "canonical")
	}
	digest := md5.Sum([]byte(wantCanonical + "\x00" + workspaceLockSentinel)) //nolint:gosec // Compatibility identity, not security.
	want := filepath.Join(tempDir, fmt.Sprintf(".aidlc-audit-%x.lock", digest[:4]))
	if got != want {
		t.Errorf("workspaceLockPath() = %q, want %q", got, want)
	}
}

func TestWorkspaceLockPathFallsBackToLexicalAbsolutePath(t *testing.T) {
	t.Parallel()

	tempDir := filepath.Join(string(filepath.Separator), "system-temp")
	lexical := filepath.Join(string(filepath.Separator), "missing", "project")
	got := workspaceLockPath(
		lexical,
		tempDir,
		func(string) (string, error) { return "", errors.New("missing") },
	)
	canonical := filepath.Clean(lexical)
	digest := md5.Sum([]byte(canonical + "\x00" + workspaceLockSentinel)) //nolint:gosec // Compatibility identity, not security.
	want := filepath.Join(tempDir, fmt.Sprintf(".aidlc-audit-%x.lock", digest[:4]))
	if got != want {
		t.Errorf("workspaceLockPath() = %q, want lexical fallback %q", got, want)
	}
}

func TestNormalizeWorkspaceLockCanonicalMatchesECMAScriptDefaultLower(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "İ", want: "i\u0307"},
		{input: "AİB", want: "ai\u0307b"},
		{input: "Σ", want: "σ"},
		{input: "ΟΣ", want: "ος"},
		{input: "ΟΣΑ", want: "οσα"},
		{input: "AΣ\u0301", want: "aς\u0301"},
		{input: "AΣ\u0301B", want: "aσ\u0301b"},
		{input: "AΣ'B", want: "aσ'b"},
		{input: "AΣ-B", want: "aς-b"},
		{input: "AΣʰ", want: "aςʰ"},
		{input: "AΣⅠ", want: "aσⅰ"},
		{input: "K", want: "k"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := normalizeWorkspaceLockCanonical(tt.input, "windows"); got != tt.want {
				t.Errorf("normalizeWorkspaceLockCanonical(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWorkspaceLockPathMatchesKnownWindowsUnicodeIdentity(t *testing.T) {
	t.Parallel()

	canonical := `C:\Projects\AİB\ΟΣ`
	wantLower := "c:\\projects\\ai\u0307b\\ος"
	identity := workspaceLockIdentity(canonical, "windows")
	if want := wantLower + "\x00" + workspaceLockSentinel; identity != want {
		t.Errorf("workspaceLockIdentity() = %q, want %q", identity, want)
	}
	digest := md5.Sum([]byte(identity)) //nolint:gosec // Compatibility identity, not security.
	if got := fmt.Sprintf("%x", digest[:4]); got != "211f1998" {
		t.Errorf("workspace lock digest = %q, want %q", got, "211f1998")
	}
	path := workspaceLockPathForPlatform(
		canonical,
		t.TempDir(),
		func(string) (string, error) { return canonical, nil },
		"windows",
	)
	if got := filepath.Base(path); got != ".aidlc-audit-211f1998.lock" {
		t.Errorf("workspace lock name = %q, want known Unicode vector", got)
	}
}

func TestWorkspaceLockPathKeepsBunWindowsUnicode15Identity(t *testing.T) {
	t.Parallel()

	canonical := `C:\Projects\AᲉB`
	wantLower := `c:\projects\aᲉb`
	identity := workspaceLockIdentity(canonical, "windows")
	if want := wantLower + "\x00" + workspaceLockSentinel; identity != want {
		t.Errorf("workspaceLockIdentity() = %q, want Bun Windows Unicode 15 value %q", identity, want)
	}
	digest := md5.Sum([]byte(identity)) //nolint:gosec // Compatibility identity, not security.
	if got := fmt.Sprintf("%x", digest[:4]); got != "a3f33a77" {
		t.Errorf("workspace lock digest = %q, want %q", got, "a3f33a77")
	}
	path := workspaceLockPathForPlatform(
		canonical,
		t.TempDir(),
		func(string) (string, error) { return canonical, nil },
		"windows",
	)
	if got := filepath.Base(path); got != ".aidlc-audit-a3f33a77.lock" {
		t.Errorf("workspace lock name = %q, want Bun Windows Unicode 15 guard", got)
	}
}

func TestAcquireWorkspaceLockWritesCompatibleOwnerAndReleaseRemovesOwnGeneration(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockedAt := time.Date(2026, time.September, 1, 12, 34, 56, 789_000_000, time.UTC)
	ops := systemWorkspaceLockOps()
	ops.now = func() time.Time { return lockedAt }
	ops.random = bytes.NewReader([]byte{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	})
	receipt, err := acquireWorkspaceLock(
		context.Background(),
		project,
		workspaceLockSettings{maxRetries: 0},
		ops,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(receipt.path) })
	if receipt.token != "00010203-0405-4607-8809-0a0b0c0d0e0f" {
		t.Errorf("token = %q, want UUIDv4 generation token", receipt.token)
	}
	data, err := os.ReadFile(filepath.Join(receipt.path, workspaceLockOwnerName))
	if err != nil {
		t.Fatal(err)
	}
	var owner workspaceLockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	wantOwner := workspaceLockOwner{
		PID:                     os.Getpid(),
		StartedAtMS:             lockedAt.UnixMilli(),
		ReapLiveOwnerAfterStale: false,
		Token:                   receipt.token,
	}
	if owner != wantOwner {
		t.Errorf("owner = %+v, want %+v", owner, wantOwner)
	}
	info, err := os.Stat(filepath.Join(receipt.path, receipt.token))
	if err != nil || !info.IsDir() {
		t.Errorf("generation token directory = (%v, %v), want directory", info, err)
	}
	if err := releaseWorkspaceLock(receipt, ops); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(receipt.path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lock path remains after own release: %v", err)
	}
}

func TestAcquireWorkspaceLockWaitsWithBoundedRetriesAndContextPriority(t *testing.T) {
	t.Parallel()

	t.Run("contended then acquired", func(t *testing.T) {
		t.Parallel()

		ops := fakeWorkspaceLockOps()
		lockAttempts := 0
		waits := 0
		ops.mkdir = func(name string, _ fs.FileMode) error {
			if strings.HasSuffix(name, ".lock") {
				lockAttempts++
				if lockAttempts < 3 {
					return fs.ErrExist
				}
			}
			return nil
		}
		ops.wait = func(context.Context, time.Duration) error {
			waits++
			return nil
		}
		receipt, err := acquireWorkspaceLock(
			context.Background(),
			"/project",
			workspaceLockSettings{maxRetries: 4, retryInterval: 100 * time.Millisecond},
			ops,
		)
		if err != nil || receipt.token == "" {
			t.Errorf("acquireWorkspaceLock() = (%+v, %v), want acquired receipt", receipt, err)
		}
		if lockAttempts != 3 || waits != 2 {
			t.Errorf("attempts/waits = %d/%d, want 3/2", lockAttempts, waits)
		}
	})

	t.Run("retry exhaustion", func(t *testing.T) {
		t.Parallel()

		ops := fakeWorkspaceLockOps()
		attempts := 0
		waits := 0
		ops.mkdir = func(string, fs.FileMode) error {
			attempts++
			return fs.ErrExist
		}
		ops.wait = func(context.Context, time.Duration) error {
			waits++
			return nil
		}
		receipt, err := acquireWorkspaceLock(
			context.Background(),
			"/project",
			workspaceLockSettings{maxRetries: 2, retryInterval: 100 * time.Millisecond},
			ops,
		)
		if receipt != (workspaceLockReceipt{}) || !errors.Is(err, fs.ErrExist) {
			t.Errorf("acquireWorkspaceLock() = (%+v, %v), want bounded fs.ErrExist", receipt, err)
		}
		if attempts != 3 || waits != 2 {
			t.Errorf("attempts/waits = %d/%d, want initial plus 2 retries and 2 waits", attempts, waits)
		}
	})

	t.Run("canceled before filesystem access", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("stop waiting")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(cause)
		ops := fakeWorkspaceLockOps()
		ops.mkdir = func(string, fs.FileMode) error {
			t.Error("canceled context reached mkdir")
			return nil
		}
		receipt, err := acquireWorkspaceLock(
			ctx,
			"/project",
			workspaceLockSettings{maxRetries: 600, retryInterval: 100 * time.Millisecond},
			ops,
		)
		if receipt != (workspaceLockReceipt{}) || !errors.Is(err, cause) {
			t.Errorf("acquireWorkspaceLock() = (%+v, %v), want context cause", receipt, err)
		}
	})

	t.Run("canceled during wait", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("deadline won")
		ops := fakeWorkspaceLockOps()
		ops.mkdir = func(string, fs.FileMode) error { return fs.ErrExist }
		ops.wait = func(context.Context, time.Duration) error { return cause }
		receipt, err := acquireWorkspaceLock(
			context.Background(),
			"/project",
			workspaceLockSettings{maxRetries: 600, retryInterval: 100 * time.Millisecond},
			ops,
		)
		if receipt != (workspaceLockReceipt{}) || !errors.Is(err, cause) {
			t.Errorf("acquireWorkspaceLock() = (%+v, %v), want wait cause", receipt, err)
		}
	})
}

func TestWaitForWorkspaceLockPrioritizesAlreadyCanceledContext(t *testing.T) {
	t.Parallel()

	cause := errors.New("stop before retry")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	if err := waitForWorkspaceLock(ctx, 0); !errors.Is(err, cause) {
		t.Errorf("waitForWorkspaceLock() error = %v, want context cause", err)
	}
}

func TestReleaseWorkspaceLockRefusesDifferentOwnerGeneration(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	ops := systemWorkspaceLockOps()
	receipt, err := acquireWorkspaceLock(
		context.Background(),
		project,
		workspaceLockSettings{maxRetries: 0},
		ops,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(receipt.path) })
	ownerPath := filepath.Join(receipt.path, workspaceLockOwnerName)
	data, err := os.ReadFile(ownerPath)
	if err != nil {
		t.Fatal(err)
	}
	var owner workspaceLockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	owner.Token = "ffffffff-ffff-4fff-8fff-ffffffffffff"
	data, err = json.Marshal(owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownerPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := releaseWorkspaceLock(receipt, ops); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("releaseWorkspaceLock() error = %v, want owner mismatch refusal", err)
	}
	if _, err := os.Stat(receipt.path); err != nil {
		t.Errorf("mismatched owner lock was removed: %v", err)
	}
}

func TestAcquireWorkspaceLockDoesNotRecoverStaleOrMalformedLocks(t *testing.T) {
	t.Parallel()

	for _, owner := range []string{"", "not-json", `{"pid":99999999,"startedAtMs":1,"token":"dead"}`} {
		owner := owner
		t.Run(owner, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			ops := systemWorkspaceLockOps()
			path := workspaceLockPath(project, ops.tempDir(), ops.evalSymlinks)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(path) })
			if owner != "" {
				if err := os.WriteFile(filepath.Join(path, workspaceLockOwnerName), []byte(owner), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			receipt, err := acquireWorkspaceLock(
				context.Background(),
				project,
				workspaceLockSettings{maxRetries: 0},
				ops,
			)
			if receipt != (workspaceLockReceipt{}) || !errors.Is(err, fs.ErrExist) {
				t.Errorf("acquireWorkspaceLock() = (%+v, %v), want fail-closed fs.ErrExist", receipt, err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("existing lock was recovered or removed: %v", err)
			}
			if owner != "" {
				data, readErr := os.ReadFile(filepath.Join(path, workspaceLockOwnerName))
				if readErr != nil || string(data) != owner {
					t.Errorf("owner stamp = (%q, %v), want unchanged %q", data, readErr, owner)
				}
			}
		})
	}
}

func TestInitializeWorkspaceLockFailureCleansOnlyOwnedGeneration(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"entropy", "generation", "open", "write", "short write", "close", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()

			cause := errors.New("injected " + stage + " failure")
			cleanupCause := errors.New("injected cleanup failure")
			ops := fakeWorkspaceLockOps()
			removed := []string{}
			if stage == "entropy" {
				ops.random = errorReader{err: cause}
			}
			ops.mkdir = func(name string, _ fs.FileMode) error {
				if stage == "generation" && strings.Contains(filepath.Base(name), "-") {
					return cause
				}
				return nil
			}
			if stage == "open" {
				ops.openFile = func(string, int, fs.FileMode) (*os.File, error) {
					return nil, cause
				}
			}
			ops.write = func(_ *os.File, data []byte) (int, error) {
				switch stage {
				case "write", "cleanup":
					return 0, cause
				case "short write":
					return len(data) - 1, nil
				default:
					return len(data), nil
				}
			}
			ops.close = func(*os.File) error {
				if stage == "close" {
					return cause
				}
				return nil
			}
			ops.remove = func(name string) error {
				removed = append(removed, name)
				if stage == "cleanup" && name == "/tmp/lock" {
					return cleanupCause
				}
				return nil
			}
			receipt, err := initializeWorkspaceLock("/tmp/lock", ops)
			if receipt != (workspaceLockReceipt{}) {
				t.Errorf("receipt = %+v, want zero", receipt)
			}
			expected := cause
			if stage == "short write" {
				expected = io.ErrShortWrite
			}
			if !errors.Is(err, expected) {
				t.Errorf("error %v lost cause %v", err, expected)
			}
			if stage == "cleanup" && !errors.Is(err, cleanupCause) {
				t.Errorf("error %v lost cleanup cause", err)
			}
			if len(removed) == 0 || removed[len(removed)-1] != "/tmp/lock" {
				t.Errorf("cleanup paths = %q, want owned lock path last", removed)
			}
		})
	}
}

func fakeWorkspaceLockOps() workspaceLockOps {
	return workspaceLockOps{
		evalSymlinks: func(path string) (string, error) { return path, nil },
		tempDir:      func() string { return "/tmp" },
		mkdir:        func(string, fs.FileMode) error { return nil },
		openFile:     func(string, int, fs.FileMode) (*os.File, error) { return nil, nil },
		write:        func(_ *os.File, data []byte) (int, error) { return len(data), nil },
		close:        func(*os.File) error { return nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
		remove:       func(string) error { return nil },
		now:          func() time.Time { return time.UnixMilli(1) },
		pid:          func() int { return 123 },
		random: bytes.NewReader([]byte{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
		}),
		wait: func(context.Context, time.Duration) error { return nil },
	}
}
