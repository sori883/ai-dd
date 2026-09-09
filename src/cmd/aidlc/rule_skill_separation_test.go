package main

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"strings"
	"testing"
)

func TestRuleSkillSeparationHelp(t *testing.T) {
	for _, args := range [][]string{{"memory", "create", "--help"}, {"intent", "procedure", "--help"}, {"intent", "approval", "--help"}} {
		text, ok := cli.Help(args)
		if !ok || !strings.Contains(text, "--project-dir") {
			t.Fatalf("public help unavailable: %v", args)
		}
	}
}
