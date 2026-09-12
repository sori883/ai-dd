package install

import (
	"os"
	"path/filepath"
	"testing"
)

var stageSkillNames = []string{"grill-with-docs", "grilling", "domain-modeling", "research", "architecture", "to-spec", "planning", "tdd", "systematic-debugging", "code-review", "verification-before-completion"}

func TestStageSkillsInstall(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, name := range stageSkillNames {
		t.Run(name, func(t *testing.T) {
			for _, file := range []string{"SKILL.md", "LICENSE", "references/source.md"} {
				data, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc-"+name, file))
				if err != nil {
					t.Fatal(err)
				}
				if len(data) == 0 {
					t.Fatal("empty asset")
				}
			}
		})
	}
}

func TestStageSkillsCollision(t *testing.T) {
	for _, name := range stageSkillNames {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".agents/skills/aidlc-"+name, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("user asset"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Codex(root, "/opt/aidlc"); err == nil {
				t.Fatal("accepted existing skill")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "user asset" {
				t.Fatal("overwrote user asset")
			}
			if _, err := os.Stat(filepath.Join(root, ".codex/hooks.json")); !os.IsNotExist(err) {
				t.Fatal("partial installation")
			}
		})
	}
}

func TestStageSkillsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agents/skills"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".agents/skills/aidlc-tdd")); err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("accepted symlink skill")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}
func TestNaturalJapaneseSkill(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "LICENSE", "references/source.md", "references/writing.md"} {
		raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/natural-japanese-go", name))
		if err != nil || len(raw) == 0 {
			t.Fatal(name, err)
		}
	}
}
