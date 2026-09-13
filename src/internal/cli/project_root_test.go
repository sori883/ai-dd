package cli

import "testing"

func TestProjectRootWithoutGitInstall(t *testing.T) {
	if _, err := ParseCommand([]string{"install", "codex"}); err == nil {
		t.Fatalf("installer command must use aidlc-install: %v", err)
	}
}
