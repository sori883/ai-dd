package cli

import (
	"strings"
	"testing"
)

func TestExecutionPlanCLIGrammar(t *testing.T) {
	for _, args := range [][]string{
		{"intent", "plan", "id", "--space", "default"},
		{"intent", "plan", "id", "--space", "default", "--expect", "1", "--file", "plan.json"},
		{"intent", "plan-approval", "id", "--space", "default", "--expect", "1", "--file", "decision.json"},
		{"intent", "approval", "id", "--space", "default", "--expect", "1", "--file", "decision.json"},
		{"intent", "finish", "id", "--space", "default", "--expect", "1"},
		{"intent", "history", "id", "--space", "default"},
		{"intent", "reopen", "id", "--space", "default", "--expect", "1", "--step", "s01", "--reason", "retry"},
	} {
		if _, err := ParseMinimal(args); err != nil {
			t.Errorf("valid %v: %v", args, err)
		}
	}
	for _, action := range []string{"plan", "plan-approval", "approval", "finish", "history", "reopen"} {
		text, ok := Help([]string{"intent", action, "--help"})
		if !ok || !strings.Contains(text, "実行") {
			t.Errorf("missing execution help for %s", action)
		}
	}
	for _, args := range [][]string{{"intent", "plan", "id", "--space", "default", "--file", "plan.json"}, {"intent", "reopen", "id", "--space", "default", "--expect", "1", "--stage", "discovery", "--reason", "retry"}} {
		if _, err := ParseMinimal(args); err == nil {
			t.Errorf("invalid accepted: %v", args)
		}
	}
}
