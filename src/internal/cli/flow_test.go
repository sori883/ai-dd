package cli

import "testing"

func TestFlowGrammar(t *testing.T) {
	for _, args := range [][]string{{"intent", "create", "Work", "--space", "default"}, {"intent", "show", "id", "--space", "default"}, {"intent", "configure", "id", "--space", "default", "--expect", "1", "--file", "config.json"}, {"intent", "advance", "id", "--space", "default", "--expect", "1"}, {"intent", "wait", "id", "--space", "default", "--expect", "1", "--reason", "Question", "--resume-condition", "Answer"}, {"unit", "claim", "id", "--space", "default", "--expect", "1", "--file", "request.json"}} {
		if _, err := ParseMinimal(args); err != nil {
			t.Errorf("valid %v: %v", args, err)
		}
	}
	for _, args := range [][]string{{"kdr", "list", "--space", "default"}, {"intent", "advance", "id", "--space", "default"}, {"intent", "advance", "id", "--space", "default", "--expect", "1", "--force"}} {
		if _, err := ParseMinimal(args); err == nil {
			t.Errorf("invalid accepted %v", args)
		}
	}
}
