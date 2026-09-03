package recordlock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewIdentityValidatesAndCanonicalizesProjectPath(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	alias := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(project, alias); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}

	canonical, err := NewIdentity(alias, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}
	lexical, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity(canonical) error = %v", err)
	}
	if canonical != lexical {
		t.Errorf("symlink and target identities differ: %q != %q", canonical, lexical)
	}

	tests := []struct {
		name    string
		project string
		space   string
		intent  string
		wantErr bool
	}{
		{name: "missing project", project: filepath.Join(project, "missing"), space: "default", intent: "build", wantErr: false},
		{name: "empty project", project: "", space: "default", intent: "build", wantErr: true},
		{name: "empty space", project: project, space: "", intent: "build", wantErr: true},
		{name: "space separator", project: project, space: "a/b", intent: "build", wantErr: true},
		{name: "empty intent", project: project, space: "default", intent: "", wantErr: true},
		{name: "intent separator", project: project, space: "default", intent: "a\\b", wantErr: true},
		{name: "control", project: project, space: "default", intent: "build\nnow", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIdentity(tt.project, tt.space, tt.intent)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIdentity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidIdentity) {
				t.Errorf("NewIdentity() error = %v, want ErrInvalidIdentity", err)
			}
		})
	}
}

func TestIdentityLockPathSeparatesSpaceAndIntent(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	first, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	secondSpace, err := NewIdentity(project, "release", "build")
	if err != nil {
		t.Fatal(err)
	}
	secondIntent, err := NewIdentity(project, "default", "test")
	if err != nil {
		t.Fatal(err)
	}
	if first.lockPath() == secondSpace.lockPath() {
		t.Error("different spaces share a lock path")
	}
	if first.lockPath() == secondIntent.lockPath() {
		t.Error("different intents share a lock path")
	}
}

func TestIdentityWindowsNormalizationUsesFixedUnicode15Vectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "AİB", want: "ai\u0307b"},
		{input: "ΟΣ", want: "ος"},
		{input: "ΟΣΑ", want: "οσα"},
		{input: "AΣ\u0301", want: "aς\u0301"},
		{input: "AΣ\u0301B", want: "aσ\u0301b"},
		{input: "AΣ\U0001171eB", want: "aσ\U0001171eb"},
		{input: "AΣ\uA7F1B", want: "aς\uA7F1b"},
		{input: "AᲉB", want: "aᲉb"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeIdentityPathForPlatform(tt.input, "windows"); got != tt.want {
				t.Errorf("Windows identity normalization = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAcquirePersistsOwnerAndReleaseRemovesOwnLock(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockedAt := time.Date(2026, time.September, 3, 1, 2, 3, 456_000_000, time.UTC)
	ops.now = func() time.Time { return lockedAt }
	ops.pid = func() int { return 4242 }
	ops.random = bytes.NewReader(bytes.Repeat([]byte{0xab}, lockTokenByteCount))

	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatalf("acquireWithOps() error = %v", err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })
	data, err := os.ReadFile(filepath.Join(lockPath, ownerMarkerName(guard.state.token)))
	if err != nil {
		t.Fatal(err)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.Token != fmt.Sprintf("%x", bytes.Repeat([]byte{0xab}, lockTokenByteCount)) {
		t.Errorf("owner token = %q, want deterministic token", owner.Token)
	}
	if owner.PID != 4242 || owner.StartedAt != lockedAt.Format(time.RFC3339Nano) {
		t.Errorf("owner = %+v, want pid/start %d/%q", owner, 4242, lockedAt.Format(time.RFC3339Nano))
	}
	if got := guard.Identity(); got != identity {
		t.Errorf("guard identity = %v, want %v", got, identity)
	}
	if !guard.Held() {
		t.Error("new guard is not held")
	}
	if err := guard.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if guard.Held() {
		t.Error("released guard still reports held")
	}
	if _, err := os.Lstat(lockPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("lock path after release = %v, want not exist", err)
	}
	if err := guard.Release(); !errors.Is(err, ErrNotHeld) {
		t.Errorf("second Release() error = %v, want ErrNotHeld", err)
	}
}

func TestReleaseUsesPinnedLockRootWithoutPathReopen(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}

	openRootCalled := errors.New("release must not reopen lock root by pathname")
	ops.openRoot = func(string) (*os.Root, error) { return nil, openRootCalled }
	guard.state.ops = ops
	if err := guard.Release(); err != nil {
		t.Fatalf("Release() = %v, want pinned-root release", err)
	}
}

func TestAcquireRejectsLockPathReplacementBeforeOwnerWrite(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	ops.random = bytes.NewReader(bytes.Repeat([]byte{0xab}, lockTokenByteCount))
	lockPath := lockPathForTemp(identity, lockTemp)
	replacedPath := lockPath + ".replaced"
	t.Cleanup(func() {
		_ = os.RemoveAll(lockPath)
		_ = os.RemoveAll(replacedPath)
	})
	actualRootCreate := ops.rootCreate
	ops.rootCreate = func(root *os.Root, name string) (*os.File, error) {
		if err := os.Rename(lockPath, replacedPath); err != nil {
			return nil, err
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			return nil, err
		}
		other := lockOwner{Token: "other-owner", PID: 99, StartedAt: "other"}
		data, err := json.Marshal(other)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(lockPath, ownerMarkerName(other.Token)), data, 0o600); err != nil {
			return nil, err
		}
		return actualRootCreate(root, name)
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if guard != nil || !errors.Is(err, ErrOwnerMismatch) {
		t.Fatalf("acquire after lock replacement = (%v, %v), want no Guard and ErrOwnerMismatch", guard, err)
	}
	if _, err := os.Stat(filepath.Join(lockPath, ownerMarkerName("other-owner"))); err != nil {
		t.Fatalf("replacement owner marker = %v, want preserved marker", err)
	}
	ownerName := ownerMarkerName(fmt.Sprintf("%x", bytes.Repeat([]byte{0xab}, lockTokenByteCount)))
	if _, err := os.Stat(filepath.Join(lockPath, ownerName)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("replacement received acquired owner marker: %v", err)
	}
}

func TestAcquireOpenRootFailurePreservesReplacementLockPath(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockPath := lockPathForTemp(identity, lockTemp)
	replacedPath := lockPath + ".replaced"
	openErr := errors.New("pin refused")
	ops.openRoot = func(path string) (*os.Root, error) {
		if err := os.Rename(path, replacedPath); err != nil {
			return nil, err
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return nil, err
		}
		return nil, openErr
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if guard != nil || !errors.Is(err, openErr) {
		t.Fatalf("acquire after rejected pin = (%v, %v), want no Guard and pin error", guard, err)
	}
	if info, err := os.Stat(lockPath); err != nil || !info.IsDir() {
		t.Errorf("replacement lock path = (%v, %v), want preserved directory", info, err)
	}
	if info, err := os.Stat(replacedPath); err != nil || !info.IsDir() {
		t.Errorf("original lock path = (%v, %v), want preserved directory", info, err)
	}
}

func TestAcquireOwnerCreateDoesNotUseGlobalPathOpen(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	globalOpenErr := errors.New("global owner open must not be called")
	ops.openFile = func(string, int, fs.FileMode) (*os.File, error) { return nil, globalOpenErr }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatalf("acquire with global owner open disabled = %v", err)
	}
	if err := guard.Release(); err != nil {
		t.Fatalf("Release() = %v", err)
	}
}

func TestAcquireOwnerCreateWriteAndCloseFailuresCleanOwnGeneration(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*lockOps, error)
	}{
		{
			name: "write",
			configure: func(ops *lockOps, cause error) {
				ops.write = func(*os.File, []byte) (int, error) { return 0, cause }
			},
		},
		{
			name: "close",
			configure: func(ops *lockOps, cause error) {
				actualClose := ops.close
				ops.close = func(file *os.File) error {
					if err := actualClose(file); err != nil {
						return err
					}
					return cause
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			lockTemp := t.TempDir()
			identity, err := NewIdentity(project, "default", "build")
			if err != nil {
				t.Fatal(err)
			}
			ops := systemLockOps()
			ops.tempDir = func() string { return lockTemp }
			cause := errors.New(tt.name + " owner failure")
			tt.configure(&ops, cause)
			guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
			if guard != nil || !errors.Is(err, cause) {
				t.Fatalf("acquire after owner %s failure = (%v, %v), want no Guard and cause", tt.name, guard, err)
			}
			if _, err := os.Lstat(lockPathForTemp(identity, lockTemp)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("lock path after owner %s failure = %v, want removed own generation", tt.name, err)
			}
		})
	}
}

func TestAcquireOwnerFailurePreservesReplacedLockPath(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockPath := lockPathForTemp(identity, lockTemp)
	replacedPath := lockPath + ".replaced"
	ownerWriteErr := errors.New("owner write failed")
	ops.write = func(file *os.File, data []byte) (int, error) {
		if err := os.Rename(lockPath, replacedPath); err != nil {
			return 0, err
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			return 0, err
		}
		return 0, ownerWriteErr
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if guard != nil || !errors.Is(err, ownerWriteErr) {
		t.Fatalf("acquire after owner write/path replacement = (%v, %v), want no Guard and write error", guard, err)
	}
	if info, err := os.Stat(lockPath); err != nil || !info.IsDir() {
		t.Errorf("replacement lock path = (%v, %v), want preserved directory", info, err)
	}
	if info, err := os.Stat(replacedPath); err != nil || !info.IsDir() {
		t.Errorf("original lock path = (%v, %v), want preserved directory", info, err)
	}
}

func TestAcquireOwnerCloseFailurePreservesReplacedLockPath(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockPath := lockPathForTemp(identity, lockTemp)
	replacedPath := lockPath + ".replaced"
	ownerCloseErr := errors.New("owner close failed")
	actualClose := ops.close
	ops.close = func(file *os.File) error {
		if err := actualClose(file); err != nil {
			return err
		}
		if err := os.Rename(lockPath, replacedPath); err != nil {
			return err
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			return err
		}
		return ownerCloseErr
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if guard != nil || !errors.Is(err, ownerCloseErr) {
		t.Fatalf("acquire after owner close/path replacement = (%v, %v), want no Guard and close error", guard, err)
	}
	if info, err := os.Stat(lockPath); err != nil || !info.IsDir() {
		t.Errorf("replacement lock path = (%v, %v), want preserved directory", info, err)
	}
	if info, err := os.Stat(replacedPath); err != nil || !info.IsDir() {
		t.Errorf("original lock path = (%v, %v), want preserved directory", info, err)
	}
}

func TestAcquireWritesOwnerWithShortSuccessfulWritesUntilComplete(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	actualWrite := ops.write
	writeCalls := 0
	ops.write = func(file *os.File, data []byte) (int, error) {
		writeCalls++
		if len(data) > 1 {
			return actualWrite(file, data[:1])
		}
		return actualWrite(file, data)
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatalf("acquireWithOps() error = %v", err)
	}
	if writeCalls < 2 {
		t.Errorf("owner write calls = %d, want short-write loop", writeCalls)
	}
	if err := guard.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireRetriesOnlyOnExistingLockAndObservesContext(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockPath := lockPathForTemp(identity, lockTemp)
	attempts := 0
	waits := 0
	actualMkdir := ops.mkdir
	ops.mkdir = func(path string, mode fs.FileMode) error {
		if path == lockPath {
			attempts++
			if attempts < 3 {
				return fs.ErrExist
			}
		}
		return actualMkdir(path, mode)
	}
	ops.wait = func(context.Context, time.Duration) error {
		waits++
		return nil
	}
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 4}, ops)
	if err != nil {
		t.Fatalf("contended acquire error = %v", err)
	}
	if attempts != 3 || waits != 2 {
		t.Errorf("attempts/waits = %d/%d, want 3/2", attempts, waits)
	}
	if err := guard.Release(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("stop waiting")
	cancel(cause)
	accessed := false
	ops.mkdirAll = func(string, fs.FileMode) error {
		accessed = true
		return nil
	}
	if guard, err := acquireWithOps(ctx, identity, lockSettings{maxRetries: 4}, ops); guard != nil || !errors.Is(err, cause) {
		t.Errorf("canceled acquire = (%v, %v), want context cause", guard, err)
	}
	if accessed {
		t.Error("canceled acquire touched filesystem")
	}
}

func TestReleaseRefusesChangedOwner(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })
	ownerPath := filepath.Join(lockPath, ownerMarkerName(guard.state.token))
	data, err := os.ReadFile(ownerPath)
	if err != nil {
		t.Fatal(err)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	owner.Token = "different-owner"
	data, err = json.Marshal(owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownerPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := guard.Release(); !errors.Is(err, ErrOwnerMismatch) {
		t.Errorf("Release() error = %v, want owner mismatch", err)
	}
	if guard.Held() {
		t.Error("guard remained held after a definitive owner mismatch")
	}
	if err := guard.WithLease(context.Background(), func() error { return nil }); !errors.Is(err, ErrNotHeld) {
		t.Errorf("WithLease() after owner mismatch = %v, want ErrNotHeld", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("mismatched lock was removed: %v", err)
	}
}

func TestReleaseLockDisappearancePermanentlyInvalidatesGuard(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	if err := os.RemoveAll(lockPath); err != nil {
		t.Fatal(err)
	}
	if err := guard.Release(); !errors.Is(err, ErrOwnerMismatch) {
		t.Errorf("Release() after lock disappearance = %v, want ErrOwnerMismatch", err)
	}
	if guard.Held() {
		t.Error("guard remained held after lock disappearance")
	}
	if err := guard.Release(); !errors.Is(err, ErrNotHeld) {
		t.Errorf("second Release() after lock disappearance = %v, want ErrNotHeld", err)
	}
	if err := guard.WithLease(context.Background(), func() error { return nil }); !errors.Is(err, ErrNotHeld) {
		t.Errorf("WithLease() after lock disappearance = %v, want ErrNotHeld", err)
	}
}

func TestReleaseRetryableInspectionErrorKeepsGuardHeld(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(lockPathForTemp(identity, lockTemp)) })
	inspectionErr := errors.New("temporary inspection failure")
	ops.lstat = func(string) (fs.FileInfo, error) { return nil, inspectionErr }
	guard.state.ops = ops
	if err := guard.Release(); !errors.Is(err, inspectionErr) {
		t.Errorf("Release() = %v, want retryable inspection error", err)
	}
	if !guard.Held() {
		t.Error("guard became unheld after retryable inspection error")
	}
	guard.state.ops = systemLockOps()
	guard.state.ops.tempDir = func() string { return lockTemp }
	if err := guard.Release(); err != nil {
		t.Fatalf("Release() retry = %v", err)
	}
}

func TestReleaseDoesNotDeleteOwnerAfterLockPathReplacement(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })
	t.Cleanup(func() { _ = os.RemoveAll(lockPath + ".replaced") })
	ownerPath := filepath.Join(lockPath, ownerMarkerName(guard.state.token))
	ops.beforeOwnerProof = func(name string) error {
		if name != ownerPath {
			return nil
		}
		if err := os.Rename(lockPath, lockPath+".replaced"); err != nil {
			return err
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			return err
		}
		other := lockOwner{Token: "other-owner", PID: 777, StartedAt: "other"}
		otherBytes, err := json.Marshal(other)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(lockPath, ownerMarkerName(other.Token)), otherBytes, 0o600); err != nil {
			return err
		}
		return nil
	}
	guard.state.ops = ops
	if err := guard.Release(); !errors.Is(err, ErrOwnerMismatch) {
		t.Errorf("Release() error = %v, want ErrOwnerMismatch", err)
	}
	data, err := os.ReadFile(filepath.Join(lockPath, ownerMarkerName("other-owner")))
	if err != nil {
		t.Fatal(err)
	}
	var owner lockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.Token != "other-owner" {
		t.Errorf("replacement owner token = %q, want other-owner", owner.Token)
	}
}

func TestReleaseDoesNotRemoveReplacementAfterOwnerCheck(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })
	t.Cleanup(func() { _ = os.RemoveAll(lockPath + ".replaced") })
	ownerName := ownerMarkerName(guard.state.token)
	actualRootRemove := ops.rootRemove
	ops.rootRemove = func(root *os.Root, name string) error {
		if name != ownerName {
			return actualRootRemove(root, name)
		}
		if err := os.Rename(lockPath, lockPath+".replaced"); err != nil {
			return err
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil {
			return err
		}
		other := lockOwner{Token: "other-owner", PID: 778, StartedAt: "other"}
		data, err := json.Marshal(other)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(lockPath, ownerMarkerName(other.Token)), data, 0o600)
	}
	guard.state.ops = ops
	if err := guard.Release(); !errors.Is(err, ErrOwnerMismatch) {
		t.Errorf("Release() error = %v, want ErrOwnerMismatch", err)
	}
	if _, err := os.Stat(filepath.Join(lockPath, ownerMarkerName("other-owner"))); err != nil {
		t.Fatalf("replacement owner was removed: %v", err)
	}
}

func TestReleaseDoesNotRemoveReplacedOwnerMarkerAtRemovalBoundary(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := lockPathForTemp(identity, lockTemp)
	ownerName := ownerMarkerName(guard.state.token)
	ownerPath := filepath.Join(lockPath, ownerName)
	replacedPath := ownerPath + ".replaced"
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })
	t.Cleanup(func() { _ = os.Remove(replacedPath) })
	actualRootLstat := ops.rootLstat
	ownerChecks := 0
	ops.rootLstat = func(root *os.Root, name string) (fs.FileInfo, error) {
		info, err := actualRootLstat(root, name)
		if name != ownerName || err != nil {
			return info, err
		}
		ownerChecks++
		if ownerChecks == 3 {
			if err := os.Rename(ownerPath, replacedPath); err != nil {
				return nil, err
			}
			other := lockOwner{Token: "other-owner", PID: 779, StartedAt: "other"}
			data, err := json.Marshal(other)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(ownerPath, data, 0o600); err != nil {
				return nil, err
			}
		}
		return info, nil
	}
	guard.state.ops = ops
	if err := guard.Release(); !errors.Is(err, ErrOwnerMismatch) {
		t.Errorf("Release() error = %v, want owner mismatch", err)
	}
	if _, err := os.Stat(ownerPath); err != nil {
		t.Fatalf("replacement owner marker was removed: %v", err)
	}
	if guard.Held() {
		t.Error("guard remained held after owner marker replacement")
	}
}

func TestWithJoinsCallbackAndReleaseErrors(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	removeCause := errors.New("remove failed")
	lockPath := lockPathForTemp(identity, lockTemp)
	actualRemove := ops.remove
	ops.remove = func(path string) error {
		if path == lockPath {
			return removeCause
		}
		return actualRemove(path)
	}
	callbackCause := errors.New("callback failed")
	err = withLockOps(context.Background(), identity, func(*Guard) error {
		return callbackCause
	}, lockSettings{maxRetries: 0}, ops)
	if !errors.Is(err, callbackCause) || !errors.Is(err, removeCause) {
		t.Errorf("With() error = %v, want joined callback/release causes", err)
	}
	_ = os.RemoveAll(lockPath)
}

func TestWithReleasesBeforeRepanicsing(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	lockPath := lockPathForTemp(identity, lockTemp)
	panicValue := "panic from callback"
	func() {
		defer func() {
			if recovered := recover(); recovered != panicValue {
				t.Errorf("panic value = %v, want %q", recovered, panicValue)
			}
		}()
		_ = withLockOps(context.Background(), identity, func(*Guard) error {
			panic(panicValue)
		}, lockSettings{maxRetries: 0}, ops)
	}()
	if _, err := os.Lstat(lockPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("lock path after panic = %v, want not exist", err)
	}
}

func TestAcquireDoesNotImplicitlyReenterHeldIdentity(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	first, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })
	second, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if second != nil || !errors.Is(err, fs.ErrExist) {
		t.Errorf("nested acquire = (%v, %v), want bounded existing-lock refusal", second, err)
	}
	if !first.Held() {
		t.Error("first guard lost ownership after nested acquire refusal")
	}
}

func TestGuardLeaseSerializesCopiesAndReleaseWaits(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })

	leaseStarted := make(chan struct{})
	allowLease := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- guard.WithLease(context.Background(), func() error {
			close(leaseStarted)
			<-allowLease
			return nil
		})
	}()
	select {
	case <-leaseStarted:
	case <-time.After(time.Second):
		t.Fatal("first WithLease() did not enter callback")
	}
	blockedCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- guard.WithLease(blockedCtx, func() error { return nil })
	}()
	if err := <-secondDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("second WithLease() = %v, want context deadline while first lease is held", err)
	}
	released := make(chan error, 1)
	go func() { released <- guard.Release() }()
	select {
	case err := <-released:
		t.Fatalf("Release() completed while lease was held: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(allowLease)
	if err := <-firstDone; err != nil {
		t.Fatalf("first WithLease() = %v", err)
	}
	select {
	case err := <-released:
		if err != nil {
			t.Fatalf("Release() after lease = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Release() did not finish after lease release")
	}
}

func TestGuardLeaseReleaseClosureCopyIsSafe(t *testing.T) {
	project := t.TempDir()
	lockTemp := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	ops := systemLockOps()
	ops.tempDir = func() string { return lockTemp }
	guard, err := acquireWithOps(context.Background(), identity, lockSettings{maxRetries: 0}, ops)
	if err != nil {
		t.Fatal(err)
	}
	release, err := guard.acquireLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	copied := release
	if err := release(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- copied() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("copied release closure = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("copied release closure blocked")
	}
	if err := guard.Release(); err != nil {
		t.Fatal(err)
	}
}
