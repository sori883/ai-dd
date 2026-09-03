//go:build integration && (aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris)

package recordlock

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRecordLockIntegrationReleaseRejectsLockDirectoryFIFOSwap(t *testing.T) {
	project := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := identity.LockPath()
	replacedPath := lockPath + ".replaced"
	t.Cleanup(func() {
		_ = os.Remove(lockPath)
		_ = os.RemoveAll(lockPath)
		_ = os.RemoveAll(replacedPath)
	})
	actualLstat := guard.state.ops.lstat
	swapped := false
	guard.state.ops.lstat = func(name string) (fs.FileInfo, error) {
		info, err := actualLstat(name)
		if name == lockPath && !swapped && err == nil {
			swapped = true
			if err := os.Rename(lockPath, replacedPath); err != nil {
				return nil, err
			}
			if err := syscall.Mkfifo(lockPath, 0o600); err != nil {
				return nil, err
			}
		}
		return info, err
	}
	done := make(chan error, 1)
	go func() { done <- guard.Release() }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrOwnerMismatch) && !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Release() after lock FIFO swap = %v, want safe owner error", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Release() blocked while lock directory was swapped to FIFO")
	}
	if info, err := os.Lstat(lockPath); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
		t.Errorf("swapped lock path = (%v, %v), want preserved FIFO", info, err)
	}
	if info, err := os.Stat(replacedPath); err != nil || !info.IsDir() {
		t.Errorf("original lock path = (%v, %v), want preserved directory", info, err)
	}
}

func TestRecordLockIntegrationReleaseRejectsOwnerFIFOSwap(t *testing.T) {
	project := t.TempDir()
	identity, err := NewIdentity(project, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	lockPath := identity.LockPath()
	ownerName := ownerMarkerName(guard.state.token)
	ownerPath := filepath.Join(lockPath, ownerName)
	replacedPath := ownerPath + ".replaced"
	t.Cleanup(func() {
		_ = os.Remove(ownerPath)
		_ = os.Remove(replacedPath)
		_ = os.RemoveAll(lockPath)
	})
	actualRootLstat := guard.state.ops.rootLstat
	ownerChecks := 0
	guard.state.ops.rootLstat = func(root *os.Root, name string) (fs.FileInfo, error) {
		info, err := actualRootLstat(root, name)
		if name == ownerName && err == nil {
			ownerChecks++
			if ownerChecks == 2 {
				if err := os.Rename(ownerPath, replacedPath); err != nil {
					return nil, err
				}
				if err := syscall.Mkfifo(ownerPath, 0o600); err != nil {
					return nil, err
				}
			}
		}
		return info, err
	}
	done := make(chan error, 1)
	go func() { done <- guard.Release() }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrOwnerMismatch) && !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Release() after owner FIFO swap = %v, want safe owner error", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Release() blocked while owner marker was swapped to FIFO")
	}
	if info, err := os.Lstat(ownerPath); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
		t.Errorf("swapped owner path = (%v, %v), want preserved FIFO", info, err)
	}
	if info, err := os.Stat(replacedPath); err != nil || !info.Mode().IsRegular() {
		t.Errorf("original owner marker = (%v, %v), want preserved regular file", info, err)
	}
}
