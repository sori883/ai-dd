package install

import (
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleSkillSeparationAssets(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aidlc", "aidlc-cli", "aidlc-okf"} {
		raw, err := os.ReadFile(filepath.Join(root, ".agents/skills", name, "SKILL.md"))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(raw), "/opt/aidlc") || strings.Contains(string(raw), "@@BINARY@@") {
			t.Errorf("unresolved binary in %s", name)
		}
		if strings.Contains(string(raw), "WORKFLOW.md") {
			t.Errorf("retired reference in %s", name)
		}
		if name == "aidlc" && len(raw) > 4096 {
			t.Errorf("bootstrap bytes=%d", len(raw))
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".agents/skills/aidlc/WORKFLOW.md")); !os.IsNotExist(err) {
		t.Errorf("retired workflow still deployed: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/other/aidlc"); err == nil {
		t.Fatal("overwrote existing deployment")
	}
	after, _ := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if string(before) != string(after) {
		t.Fatal("changed existing skill")
	}
}

func TestRuleSkillSeparationRelocate(t *testing.T) {
	for _, mode := range []string{"success", "partial", "missing", "edited", "legacy", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			paths := []string{".agents/skills/aidlc/SKILL.md", ".agents/skills/aidlc-cli/SKILL.md", ".agents/skills/aidlc-okf/SKILL.md", ".codex/hooks.json"}
			p := filepath.Join(root, paths[1])
			before := map[string]string{}
			for _, name := range paths {
				raw, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				before[name] = string(raw)
			}
			if mode == "missing" || mode == "legacy" || mode == "symlink" {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "legacy" {
				if err := os.WriteFile(filepath.Join(root, ".agents/skills/aidlc/WORKFLOW.md"), []byte("legacy user file"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "edited" {
				if err := os.WriteFile(p, []byte("user edited CLI skill"), 0600); err != nil {
					t.Fatal(err)
				}
				before[paths[1]] = "user edited CLI skill"
			}
			if mode == "symlink" {
				if err := os.Symlink(filepath.Join(root, paths[0]), p); err != nil {
					t.Fatal(err)
				}
			}
			result, err := relocate(root, "/new/aidlc", oldRoot, oldBinary, func(r, p string, b []byte) error {
				if mode == "partial" && p == paths[1] {
					return errors.New("injected")
				}
				return filestore.WriteFile(r, p, b)
			})
			if mode == "success" {
				if err != nil || len(result.Paths) != len(paths) {
					t.Fatalf("relocate %+v %v", result, err)
				}
			} else if mode == "partial" {
				if err == nil || len(result.Paths) != 1 || len(result.Pending) != len(paths)-1 || result.Pending[0] != paths[1] {
					t.Fatalf("partial %+v %v", result, err)
				}
				retry, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
				if err != nil || len(retry.Paths) != len(paths)-1 {
					t.Fatalf("retry %+v %v", retry, err)
				}
			} else {
				if err == nil || len(result.Paths) != 0 {
					t.Fatalf("accepted unknown deployment %+v %v", result, err)
				}
				for _, name := range []string{paths[0], paths[2], paths[3]} {
					raw, _ := os.ReadFile(filepath.Join(root, name))
					if string(raw) != before[name] {
						t.Fatal("changed before validation", name)
					}
				}
				return
			}
			for _, name := range paths {
				raw, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || strings.Contains(string(raw), oldBinary) || !strings.Contains(string(raw), "/new/aidlc") {
					t.Fatalf("unmoved %s: %s %v", name, raw, err)
				}
			}
			again, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
			if err != nil || len(again.Paths) != 0 || len(again.Pending) != 0 {
				t.Fatalf("idempotence %+v %v", again, err)
			}
		})
	}
}
