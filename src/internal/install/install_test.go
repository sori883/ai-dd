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
	for _, path := range []string{".codex/hooks.json", ".codex/agents/aidlc-reviewer.toml", ".agents/skills/aidlc/SKILL.md", "aidlc/templates/adr.md", "aidlc/spaces/default/knowledge/index.md", "aidlc/spaces/default/knowledge/rules/entry.md", "aidlc/spaces/default/knowledge/rules/rule.md"} {
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

func TestInstallRecoveryGuidanceAndContextLimit(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(skill) > 4096 || !strings.Contains(string(skill), "session bind ID --space SPACE --session SESSION --recover") {
		t.Fatalf("missing bounded recovery syntax: %s", skill)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	for event, groups := range config.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				_, present := handler["additionalContextLimit"]
				if present != (event == "SessionStart") {
					t.Errorf("unexpected context limit for %s: %v", event, handler)
				}
			}
		}
	}
}

func TestInstallMemoryCommandGuidance(t *testing.T) {
	root := t.TempDir()
	binary := "/private/var/folders/example/aidlc-minimal-journey-1234567890/bin/aidlc"
	if _, err := Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 {
		t.Fatalf("deployed skill is %d bytes, limit 4096", len(raw))
	}
	for _, want := range []string{
		"memory create knowledge/NAME --space SPACE --file FILE --actor process:codex",
		"memory update knowledge/NAME --space SPACE --file FILE --actor process:codex --expect HASH",
		"memory show knowledge/NAME --space SPACE",
		"memory search QUERY --space SPACE [--intent-id ID]",
		"Concept ID has no .md", "ADR/NAME", "hash", "content",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("deployed skill lacks %q", want)
		}
	}
}
