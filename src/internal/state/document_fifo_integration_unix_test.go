//go:build integration && (aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris)

package state

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestReadDocumentIntegrationRejectsFIFOWithoutBlocking(t *testing.T) {
	recordDir := t.TempDir()
	if err := syscall.Mkfifo(recordDir+"/"+stateFile, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	done := make(chan error, 1)
	go func() {
		_, err := ReadDocument(root)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("ReadDocument() error = %v, want os.ErrInvalid", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadDocument() blocked while inspecting a FIFO state leaf")
	}
}
