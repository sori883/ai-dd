package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFresh(t *testing.T) {
	root := t.TempDir()
	result, err := Codex(root, "/opt/aidlc binary")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".codex/hooks.json", ".codex/agents/aidlc-reviewer.toml", ".agents/skills/aidlc/SKILL.md", "aidlc/templates/kdr.md", "aidlc/spaces/default/knowledge/index.md", "aidlc/spaces/default/knowledge/rules/entry.md", "aidlc/spaces/default/knowledge/rules/rule.md"} {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Errorf("missing embedded asset %s: %v", path, err)
			continue
		}
		if len(raw) == 0 {
			t.Errorf("empty asset %s", path)
		}
	}
	if len(result.Paths) < 7 {
		t.Errorf("saved paths = %v", result.Paths)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if !json.Valid(raw) || !strings.Contains(string(raw), "/opt/aidlc binary") {
		t.Errorf("invalid hook configuration: %s", raw)
	}
}

func TestInstallCollisionPreserves(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex/hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("accepted existing destination")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "user configuration" {
		t.Fatalf("overwrote existing file: %s", got)
	}
}

func TestInstallRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "aidlc")); err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("accepted symlink destination")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside project")
	}
}
