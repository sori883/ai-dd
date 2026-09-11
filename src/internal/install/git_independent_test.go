package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitIndependentInstall(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".agents/skills/aidlc-cli/SKILL.md", ".codex/agents/aidlc-worker.toml", "aidlc/workflow/stages/tdd.md", "aidlc/workflow/stages/integration.md"} {
		raw, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "verification_sha256") {
			t.Errorf("%s omits verification SHA contract", p)
		}
	}
	TestRelocateReferences(t)
}
