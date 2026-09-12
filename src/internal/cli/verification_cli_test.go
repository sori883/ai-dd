package cli

import (
	"strings"
	"testing"
)

func TestVerificationCLI(t *testing.T) {
	args := []string{"intent", "hash", "abc", "--space", "default", "--unit", "a", "--root", "/project"}
	r, err := ParseCommand(args)
	if err != nil {
		t.Fatal(err)
	}
	if r.Unit != "a" || r.Root != "/project" || !isServiceCommand(args) {
		t.Fatalf("hash request %+v", r)
	}
	h, ok := Help([]string{"intent", "hash", "--help"})
	if !ok || !strings.Contains(h, "--unit") {
		t.Fatal("missing hash help")
	}
}
