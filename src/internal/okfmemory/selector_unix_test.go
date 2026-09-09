//go:build unix

package okfmemory

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestDocumentSelectorNonRegular(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.md"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := SelectDocuments(root, DocumentMatch{Type: "Rule"}, "optional"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO selection blocked")
	}
}
