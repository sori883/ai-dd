package flow

import (
	"github.com/sori883/ai-dd/src/internal/install"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleSkillSeparationRule(t *testing.T) {
	for _, mode := range []string{"default", "custom", "missing", "empty", "wrong type", "missing title", "missing description"} {
		t.Run(mode, func(t *testing.T) {
			s := Store{Root: t.TempDir(), Space: "default"}
			if _, err := install.Codex(s.Root, "/opt/aidlc"); err != nil {
				t.Fatal(err)
			}
			st, err := s.Create("project rule")
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "default" && !strings.Contains(string(raw), "追加ルールはありません") {
				t.Error("initial rule invents project constraints")
			}
			custom := "---\ntype: Rule\ntitle: チームの約束\ndescription: このプロジェクトの制約\n---\n追加ルールはありません。\n"
			switch mode {
			case "custom":
				raw = []byte(custom)
			case "empty":
				raw = []byte(strings.Split(custom, "---\n追加")[0] + "---\n")
			case "wrong type":
				raw = []byte(strings.Replace(custom, "type: Rule", "type: Knowledge", 1))
			case "missing title":
				raw = []byte(strings.Replace(custom, "title: チームの約束\n", "", 1))
			case "missing description":
				raw = []byte(strings.Replace(custom, "description: このプロジェクトの制約\n", "", 1))
			}
			if mode == "missing" {
				err = os.Remove(p)
			} else {
				err = os.WriteFile(p, raw, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Begin(st.ID, st.Revision)
			if mode == "default" || mode == "custom" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("invalid Rule accepted")
			}
		})
	}
}

func TestRuleSkillSeparationHookMissingCLI(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("check install")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(s.Root, ".agents/skills/aidlc-cli/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin(st.ID, st.Revision); err == nil {
		t.Fatal("missing CLI skill accepted")
	}
}
