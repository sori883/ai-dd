package cli

import (
	"strings"
	"testing"
)

func TestRelocationCLI(t *testing.T) {
	if _, err := ParseCommand([]string{"unit", "reassign", strings.Repeat("a", 32), "--space", "default", "--expect", "1", "--file", "request.json"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		action []string
		words  []string
	}{{[]string{"unit", "reassign", "--help"}, []string{"previous_run_stopped", "true", "reason", "run_id"}}} {
		text, ok := Help(tc.action)
		if !ok {
			t.Fatal("missing help")
		}
		for _, word := range tc.words {
			if !strings.Contains(text, word) {
				t.Errorf("help lacks %s", word)
			}
		}
	}
}
