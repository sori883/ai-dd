package cli

import "testing"

func TestProjectRootWithoutGitInstall(t *testing.T) {
	if _, err := ParseCommand([]string{"install", "codex"}); err != nil {
		t.Fatalf("new install must allow cwd root: %v", err)
	}
}
