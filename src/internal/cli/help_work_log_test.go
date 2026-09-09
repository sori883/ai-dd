package cli_test

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"strings"
	"testing"
)

func TestOKFWorkLogHelp(t *testing.T) {
	text, ok := cli.Help([]string{"intent", "reopen", "--help"})
	if !ok {
		t.Fatal("reopen help unavailable")
	}
	for _, want := range []string{"aidlc/spaces/<space>/knowledge/log/<intent_id>-work-log.md", "type: work-log", "memory search work-log", "--intent-id ID", "memory show log/ID-work-log", "state", "revision", "generated.at"} {
		if !strings.Contains(text, want) {
			t.Errorf("help missing %q", want)
		}
	}
}
