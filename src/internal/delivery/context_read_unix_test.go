//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package delivery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadContextRejectsFIFOWithoutBlocking(t *testing.T) {
	fixture := newContextReadFixture(t)
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.Remove(stagePath); err != nil {
		t.Fatalf("Remove(stage file): %v", err)
	}
	if err := syscall.Mkfifo(stagePath, 0o600); err != nil {
		t.Fatalf("Mkfifo(stage file): %v", err)
	}
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadUnsafePath) {
		t.Fatalf("ReadContext(FIFO) error = %v, want ErrContextReadUnsafePath", err)
	}
}
