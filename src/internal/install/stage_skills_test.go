package install

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
				data, err := os.ReadFile(filepath.Join(root, ".agents/skills/"+name, file))
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
			path := filepath.Join(root, ".agents/skills/"+name, "SKILL.md")
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
	if err := os.Symlink(outside, filepath.Join(root, ".agents/skills/tdd")); err != nil {
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

func TestStageSkillsReferencesResolve(t *testing.T) {
	root := t.TempDir()
	result, err := Codex(root, "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range result.Paths {
		if !strings.HasPrefix(name, ".agents/skills/") || !strings.HasSuffix(name, ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).FindAllStringSubmatch(string(raw), -1) {
			link := match[1]
			if strings.Contains(link, "://") || strings.HasPrefix(link, "#") {
				continue
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(name), strings.Split(link, "#")[0]))
			if !strings.HasPrefix(filepath.ToSlash(target), ".agents/skills/") {
				t.Fatalf("skill link leaves deployed skills: %s -> %s", name, link)
			}
			info, err := os.Stat(filepath.Join(root, target))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatalf("unresolved skill link: %s -> %s: %v", name, link, err)
			}
		}
	}
}

func TestUpstreamSkillNames(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	names := append(append([]string{}, stageSkillNames...), "okf-agent-memory", "natural-japanese-go")
	entries, err := os.ReadDir(filepath.Join(root, ".agents/skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 15 {
		t.Fatalf("skills = %d, want 15", len(entries))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, ".agents/skills", name, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "name: "+name+"\n") {
				t.Fatal("frontmatter name differs from directory")
			}
			for _, file := range []string{"LICENSE", "references/source.md"} {
				data, err := os.ReadFile(filepath.Join(root, ".agents/skills", name, file))
				if err != nil || len(data) == 0 {
					t.Fatalf("missing attribution resource %s: %v", file, err)
				}
			}
			old := "aidlc-" + name
			if name == "okf-agent-memory" {
				old = "aidlc-okf"
			}
			if _, err := os.Lstat(filepath.Join(root, ".agents/skills", old)); !os.IsNotExist(err) {
				t.Fatal("old skill deployed")
			}
		})
	}
}

func TestUpstreamSkillReferences(t *testing.T) {
	root := t.TempDir()
	result, err := Codex(root, "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	oldNames := append(append([]string{}, stageSkillNames...), "okf")
	for _, path := range result.Paths {
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".toml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range oldNames {
			if regexp.MustCompile("aidlc-" + regexp.QuoteMeta(name) + `([^a-zA-Z0-9-]|$)`).Match(raw) {
				t.Errorf("old skill reference in %s: %s", path, name)
			}
		}
	}
}
