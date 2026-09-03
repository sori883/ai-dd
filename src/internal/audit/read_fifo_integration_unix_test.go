//go:build integration && (darwin || linux || freebsd || netbsd || openbsd)

package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadEventsIntegrationRejectsFIFOSHardWithoutBlocking(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	auditDir := filepath.Join(recordDir, auditDirectory)
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(auditDir, "bad.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
	defer func() { _ = projectRoot.Close() }()
	defer func() { _ = recordRoot.Close() }()
	defer func() { _ = guard.Release() }()

	done := make(chan error, 1)
	go func() {
		_, err := ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("ReadEvents() error = %v, want ErrInvalidRoot", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadEvents() blocked while inspecting a FIFO shard")
	}
}
