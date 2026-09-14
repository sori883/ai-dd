package app

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleSkillSeparationRule(t *testing.T) {
	s, st := setup(t)
	p := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	raw := "---\ntype: Rule\ntitle: 利用者タイトル\ndescription: 共通制約\n---\n固有の長い末尾まで読む。\n"
	if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	body, hash, err := s.rules("default")
	if err != nil || !strings.Contains(string(body), "固有の長い末尾まで読む。") || hash == "" {
		t.Fatalf("rules %s %s %v", body, hash, err)
	}
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, st.ID)
	if err := os.WriteFile(p, []byte(raw+"変更\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, after, err := s.rules("default")
	if err != nil || hash == after {
		t.Fatal("Rule hash unchanged", err)
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "read", "cat .agents/skills/aidlc/SKILL.md", false)) {
		t.Fatal("changed Rule bypassed reread")
	}
}

func TestRuleSkillSeparationHook(t *testing.T) {
	for _, retired := range []bool{false, true} {
		t.Run(fmt.Sprint(retired), func(t *testing.T) {
			s, st := setup(t)
			st.Status = "waiting"
			st = writeExecutionFixture(t, flow.Store{Root: s.Root, Space: "default"}, st)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			bind(t, s, st.ID)
			command := "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md"
			if retired {
				command = "cat .agents/skills/aidlc/WORKFLOW.md"
			}
			if out := hook(t, s, "PreToolUse", "Bash", "read", command, false); deny(out) != retired {
				t.Fatal(out)
			}
		})
	}
}
func TestOKFSkillRead(t *testing.T) {
	for _, mode := range []string{"all", "single", "before begin", "old-installed"} {
		t.Run(mode, func(t *testing.T) {
			s, st := setup(t)
			if mode != "before begin" {
				st.Status = "waiting"
			}
			st = writeExecutionFixture(t, flow.Store{Root: s.Root, Space: "default"}, st)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			bind(t, s, st.ID)
			command := "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md .agents/skills/okf-agent-memory/SKILL.md"
			if mode == "single" {
				command = "cat .agents/skills/okf-agent-memory/SKILL.md"
			}
			if mode == "old-installed" {
				name := filepath.Join(s.Root, ".agents/skills/aidlc-okf/SKILL.md")
				if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, []byte("old instructions"), 0644); err != nil {
					t.Fatal(err)
				}
				command = "cat .agents/skills/aidlc-okf/SKILL.md"
			}
			if out := hook(t, s, "PreToolUse", "Bash", "read", command, false); deny(out) != (mode == "old-installed") {
				t.Fatal(out)
			}
		})
	}
}
