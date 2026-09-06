//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestIntentCaptureReadersRejectFIFOReplacementWithoutBlocking(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	cases := []struct {
		name string
		read func() error
	}{
		{
			name: "review artifact",
			read: func() error {
				_, err := readReviewArtifact(fixture.recordRoot, "ideation/intent-capture/intent-statement.md")
				return err
			},
		},
		{
			name: "questions",
			read: func() error {
				_, _, err := readIntentCaptureQuestions(fixture.recordRoot)
				return err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := "ideation/intent-capture/intent-statement.md"
			if tc.name == "questions" {
				name = intentCaptureQuestionsFile
			}
			target := filepath.Join(fixture.project, filepath.FromSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build", name)))
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

			if err := tc.read(); err == nil {
				t.Fatal("reader error = nil, want FIFO replacement rejected without blocking")
			} else if errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatalf("reader blocked on FIFO replacement: %v", err)
			}
		})
	}
}
