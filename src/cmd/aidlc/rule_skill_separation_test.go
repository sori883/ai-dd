package main

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"strings"
	"testing"
)

func TestRuleSkillSeparationHelp(t *testing.T) {
	for _, args := range [][]string{{"memory", "create", "--help"}, {"intent", "procedure", "--help"}, {"intent", "approval", "--help"}} {
		text, ok := cli.Help(args)
		if args[0] == "memory" {
			text, ok = okfcli.Help(args[1:])
		}
		if !ok || !strings.Contains(text, "--project-dir") {
			t.Fatalf("public help unavailable: %v", args)
		}
	}
}
