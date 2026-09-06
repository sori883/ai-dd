//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const intentCaptureFIFOSubprocessTimeout = 10 * time.Second

func TestIntentCaptureReadersRejectFIFOReplacementWithoutBlocking(t *testing.T) {
	if os.Getenv("AIDLC_AUDIT_FIFO_HELPER") == "1" {
		t.Skip("helper process is exercised by the parent test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), intentCaptureFIFOSubprocessTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestIntentCaptureReadersRejectFIFOReplacementHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "AIDLC_AUDIT_FIFO_HELPER=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("FIFO helper timed out and was killed: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("FIFO helper: %v\n%s", err, output)
	}
}

func TestIntentCaptureReadersRejectFIFOReplacementHelper(t *testing.T) {
	if os.Getenv("AIDLC_AUDIT_FIFO_HELPER") != "1" {
		return
	}
	fixture := newHumanTurnWorkspaceFixture(t)
	testAuditLeafFIFOReplacement(t, fixture, "review artifact", "ideation/intent-capture/intent-statement.md", func() error {
		_, err := readReviewArtifact(fixture.recordRoot, "ideation/intent-capture/intent-statement.md")
		return err
	})
	testAuditLeafFIFOReplacement(t, fixture, "questions", intentCaptureQuestionsFile, func() error {
		_, _, err := readIntentCaptureQuestions(fixture.recordRoot)
		return err
	})
}

func testAuditLeafFIFOReplacement(t *testing.T, fixture humanTurnWorkspaceFixture, name, relative string, read func() error) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		target := filepath.Join(fixture.project, filepath.FromSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build", relative)))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("## Stable\ncontent\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		backup := target + ".regular"
		restore := setAuditLeafAfterLstat(func(_ *os.Root, _ string) error {
			if err := os.Rename(target, backup); err != nil {
				return err
			}
			if err := syscall.Mkfifo(target, 0o600); err != nil {
				_ = os.Rename(backup, target)
				return err
			}
			return nil
		})
		defer func() {
			restore()
			_ = os.Remove(target)
			_ = os.Rename(backup, target)
		}()
		if err := read(); err == nil {
			t.Fatal("reader error = nil, want FIFO replacement rejected")
		}
	})
}
