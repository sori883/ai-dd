package app

import (
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
	for _, mode := range []string{"both", "unread", "inflight", "missing", "symlink", "redirect", "compound", "arbitrary", "retired"} {
		t.Run(mode, func(t *testing.T) {
			s, st := setup(t)
			st.Status = "waiting"
			st = writeExecutionFixture(t, flow.Store{Root: s.Root, Space: "default"}, st)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			if mode != "unread" {
				bind(t, s, st.ID)
			}
			command := "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md"
			p := filepath.Join(s.Root, ".agents/skills/aidlc-cli/SKILL.md")
			switch mode {
			case "inflight":
				if deny(hook(t, s, "PreToolUse", "Bash", "first", "cat .agents/skills/aidlc/SKILL.md", false)) {
					t.Fatal("first read denied")
				}
			case "missing", "symlink":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					if err := os.Symlink(filepath.Join(s.Root, ".agents/skills/aidlc/SKILL.md"), p); err != nil {
						t.Fatal(err)
					}
				}
			case "redirect":
				command += " > /tmp/ignored"
			case "compound":
				command += " && echo bad"
			case "arbitrary":
				command = "cat arbitrary.md"
			case "retired":
				command = "cat .agents/skills/aidlc/WORKFLOW.md"
			}
			out := hook(t, s, "PreToolUse", "Bash", "read", command, false)
			if deny(out) == (mode == "both") {
				t.Fatalf("%s: %+v", mode, out)
			}
		})
	}
}

func TestRuleSkillSeparationHookApprovalPending(t *testing.T) { TestExecutionPlanCLIPendingHook(t) }

func TestOKFSkillRead(t *testing.T) {
	for _, mode := range []string{"all", "single", "before begin", "unread", "inflight", "missing", "symlink", "redirect", "compound", "arbitrary", "retired"} {
		t.Run(mode, func(t *testing.T) {
			s, st := setup(t)
			if mode != "before begin" {
				st.Status = "waiting"
			}
			st = writeExecutionFixture(t, flow.Store{Root: s.Root, Space: "default"}, st)
			hook(t, s, "UserPromptSubmit", "", "", "", false)
			if mode != "unread" {
				bind(t, s, st.ID)
			}
			command := "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-cli/SKILL.md .agents/skills/aidlc-okf/SKILL.md"
			if mode == "single" {
				command = "cat .agents/skills/aidlc-okf/SKILL.md"
			}
			p := filepath.Join(s.Root, ".agents/skills/aidlc-okf/SKILL.md")
			switch mode {
			case "inflight":
				if deny(hook(t, s, "PreToolUse", "Bash", "first", "cat .agents/skills/aidlc/SKILL.md", false)) {
					t.Fatal("first read denied")
				}
			case "missing", "symlink":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if mode == "symlink" {
					if err := os.Symlink(filepath.Join(s.Root, ".agents/skills/aidlc/SKILL.md"), p); err != nil {
						t.Fatal(err)
					}
				}
			case "redirect":
				command += " > /tmp/ignored"
			case "compound":
				command += " && echo bad"
			case "arbitrary":
				command = "cat arbitrary.md"
			case "retired":
				command = "cat .agents/skills/aidlc/WORKFLOW.md"
			}
			out := hook(t, s, "PreToolUse", "Bash", "read", command, false)
			if deny(out) == (mode == "all" || mode == "single" || mode == "before begin") {
				t.Fatalf("%s: %+v", mode, out)
			}
		})
	}
}
