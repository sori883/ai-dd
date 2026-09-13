package install

import (
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCLIInstallConflict(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".agents/skills/okf-agent-memory/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("user"), 0600); err != nil {
		t.Fatal(err)
	}
	b := codex.Binaries{AIDLC: "/old/aidlc", OKF: "/old/okf", Natural: "/old/natural"}
	if r, err := CodexFrom(root, b, core.Files, codex.Files); err == nil || len(r.Paths) != 0 {
		t.Fatalf("conflict: %+v %v", r, err)
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "user" {
		t.Fatal("overwritten")
	}
}
func TestSplitCLIRelocation(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	old := codex.Binaries{AIDLC: "/old/aidlc", OKF: "/old/okf", Natural: "/old/natural"}
	b := codex.Binaries{AIDLC: "/new/aidlc", OKF: "/new/okf", Natural: "/new/natural"}
	r, err := CodexFrom(root, old, core.Files, codex.Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Paths) == 0 {
		t.Fatal("no assets installed")
	}
	if _, err := RelocateFrom(root, root, b, old, core.Files, codex.Files); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"aidlc", "okf-agent-memory", "natural-japanese-go"} {
		raw, err := os.ReadFile(filepath.Join(root, ".agents/skills", p, "SKILL.md"))
		if err != nil || strings.Contains(string(raw), "/old/") {
			t.Fatalf("%s: %s %v", p, raw, err)
		}
	}
}
