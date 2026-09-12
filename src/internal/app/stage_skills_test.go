package app

import (
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"testing"
)

func TestStageSkillsRead(t *testing.T) {
	for _, mode := range []string{"all", "single", "before begin", "changed Rule", "parent", "glob", "four", "script", "unread", "inflight", "missing", "symlink", "redirect", "compound", "arbitrary", "retired"} {
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
			command := "cat .agents/skills/aidlc/SKILL.md .agents/skills/aidlc-tdd/references/source.md .agents/skills/aidlc-tdd/SKILL.md"
			if mode == "single" {
				command = "cat .agents/skills/aidlc-tdd/SKILL.md"
			}
			p := filepath.Join(s.Root, ".agents/skills/aidlc-tdd/SKILL.md")
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
			case "changed Rule":
				file := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, append(raw, []byte("\nChanged\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "parent":
				command = "cat .agents/skills/aidlc-tdd/../aidlc-tdd/SKILL.md"
			case "glob":
				command = "cat .agents/skills/aidlc-tdd/*.md"
			case "four":
				command += " .agents/skills/aidlc/SKILL.md"
			case "script":
				command = "natural-japanese-go text.md"
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
