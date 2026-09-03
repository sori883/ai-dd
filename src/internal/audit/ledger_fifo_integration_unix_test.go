//go:build integration && (darwin || linux || freebsd || netbsd || openbsd)

package audit

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestAuditIntegrationRejectsFIFOSwapWithoutBlocking(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	if err := os.Mkdir(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	shard := filepath.Join(auditDirectory, shardName("host", "abcdef123456"))
	fifoPath := filepath.Join(recordDir, shard)
	actualLstat := recordRoot.Lstat
	swapped := false
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.hostname = func() (string, error) { return "host", nil }
	ops.recordLstat = func(name string) (fs.FileInfo, error) {
		if name == shard && !swapped {
			swapped = true
			if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
				return nil, err
			}
			return nil, fs.ErrNotExist
		}
		return actualLstat(name)
	}
	done := make(chan error, 1)
	go func() {
		done <- appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}, &ops)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrInvalidRoot) {
			t.Errorf("FIFO swap append error = %v, want ErrInvalidRoot", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("append blocked while opening a swapped FIFO")
	}
	if err := os.Remove(fifoPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestAuditIntegrationRejectsParentFIFOSwapWithoutBlocking(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	parentPath := filepath.Join(recordDir, auditDirectory)
	replacedPath := parentPath + ".replaced"
	fifoPath := parentPath
	t.Cleanup(func() {
		_ = os.Remove(fifoPath)
		_ = os.RemoveAll(replacedPath)
	})
	actualLstat := recordRoot.Lstat
	lstatCalls := 0
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.hostname = func() (string, error) { return "host", nil }
	ops.recordLstat = func(name string) (fs.FileInfo, error) {
		info, err := actualLstat(name)
		if name == auditDirectory {
			lstatCalls++
			if lstatCalls == 2 {
				if err := os.Rename(parentPath, replacedPath); err != nil {
					return nil, err
				}
				if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
					return nil, err
				}
			}
		}
		return info, err
	}
	done := make(chan error, 1)
	go func() {
		done <- appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}, &ops)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrInvalidRoot) {
			t.Errorf("parent FIFO swap append error = %v, want ErrInvalidRoot", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("append blocked while opening a swapped FIFO parent")
	}
}
