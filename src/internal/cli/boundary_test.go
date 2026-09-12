package cli

import (
	"strings"
	"testing"
)

func TestBoundaryCLI(t *testing.T) {
	for _, args := range [][]string{{"intent", "check", "id", "--space", "default", "--boundary", "start"}, {"intent", "check", "id", "--space", "default", "--boundary", "end"}, {"intent", "begin", "id", "--space", "default", "--expect", "1"}} {
		if _, err := ParseCommand(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{{"intent", "check", "id", "--space", "default", "--boundary", "bad"}, {"intent", "begin", "id", "--space", "default"}} {
		if _, err := ParseCommand(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if text, ok := Help([]string{"intent", "begin", "--help"}); !ok || !strings.Contains(text, "--expect") {
		t.Fatal("begin help missing")
	}
}
func TestBoundaryRootHelp(t *testing.T) {
	text, ok := Help([]string{"--help"})
	if !ok || !strings.Contains(text, "intent begin") || !strings.Contains(text, "--boundary") {
		t.Fatal("root help hides stage start commands")
	}
}
