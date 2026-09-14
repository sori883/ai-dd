package cli

import (
	codex "github.com/sori883/ai-dd/src/harness/codex"
	"regexp"
	"strings"
	"testing"
)

func TestRuleSkillSeparationHelp(t *testing.T) {
	assets, err := codex.Distribution("/project", "/opt/aidlc")
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for _, asset := range assets {
		if asset.Path == ".agents/skills/aidlc-cli/SKILL.md" {
			raw = asset.Data
		}
	}
	commands := regexp.MustCompile("`A ([a-z-]+(?: [a-z-]+)?) --help`").FindAllStringSubmatch(string(raw), -1)
	if len(commands) == 0 {
		t.Fatal("missing operation routes")
	}
	for _, m := range commands {
		args := append(strings.Fields(m[1]), "--help")
		if text, ok := Help(args); !ok || text == "" {
			t.Errorf("unreachable %v", args)
		}
	}
}
