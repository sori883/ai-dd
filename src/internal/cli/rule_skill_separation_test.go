package cli

import (
	codex "github.com/sori883/ai-dd/src/harness/codex/minimal"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

func TestRuleSkillSeparationHelp(t *testing.T) {
	raw, err := fs.ReadFile(codex.Files, "aidlc-cli/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	commands := regexp.MustCompile("`A ([a-z-]+(?: [a-z-]+)?) --help`").FindAllStringSubmatch(string(raw), -1)
	if len(commands) < 20 {
		t.Fatal("missing operation routes")
	}
	for _, m := range commands {
		args := append(strings.Fields(m[1]), "--help")
		if text, ok := Help(args); !ok || text == "" {
			t.Errorf("unreachable %v", args)
		}
	}
	for key, wants := range map[string][]string{"intent procedure": {"固定path", "利用者", "Rule"}, "install codex": {"aidlc-cli", "3", "Pending"}, "intent documents": {"step_id", "metadata", "outputs"}, "intent review": {"coordinator_session", "target", "summary"}, "intent configure": {"test_results", "output_path", "exit_code"}} {
		text, ok := Help(append(strings.Fields(key), "--help"))
		if !ok {
			t.Fatal(key)
		}
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %s", key, want)
			}
		}
	}
}
