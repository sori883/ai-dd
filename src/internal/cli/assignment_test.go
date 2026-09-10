package cli

import (
	"strings"
	"testing"
)

func TestAssignmentContract(t *testing.T) {
	for _, args := range [][]string{
		{"assignment", "init", "--file", "init.json"},
		{"assignment", "reset", "--file", "reset.json"},
		{"assignment", "list"},
		{"assignment", "show", "id"},
		{"assignment", "check", "id"},
		{"assignment", "reserve", "intent", "--space", "default", "--session", "main", "--expect", "1", "--file", "reserve.json"},
		{"assignment", "release", "id", "--session", "main", "--expect", "1", "--file", "release.json"},
	} {
		t.Run(args[1], func(t *testing.T) {
			if _, err := ParseMinimal(args); err != nil {
				t.Fatal(err)
			}
			text, ok := Help([]string{"assignment", args[1], "--help"})
			if !ok || !strings.Contains(text, "registry_epoch") {
				t.Fatal("missing assignment contract help")
			}
		})
	}
	for _, args := range [][]string{{"assignment", "release", "id", "--expect", "1", "--file", "release.json"}, {"assignment", "init"}, {"assignment", "reserve", "id", "--space", "default", "--file", "r.json"}} {
		if _, err := ParseMinimal(args); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}
