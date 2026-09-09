package cli_test

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"strings"
	"testing"
)

func TestCodeKBHelp(t *testing.T) {
	for _, tc := range []struct{ group, action, want string }{
		{group: "memory", action: "create", want: "memory create codekb/authentication"},
		{group: "memory", action: "update", want: "memory update codekb/authentication"},
		{group: "memory", action: "show", want: "codekb/authentication"},
		{group: "intent", action: "documents", want: "knowledge/codekb/feature.md"},
		{group: "intent", action: "configure", want: "knowledge/codekb/current.md"},
	} {
		t.Run(tc.group+"/"+tc.action, func(t *testing.T) {
			text, ok := cli.Help([]string{tc.group, tc.action, "--help"})
			if !ok || !strings.Contains(text, tc.want) {
				t.Errorf("missing codekb example %q", tc.want)
			}
			if strings.Contains(text, "knowledge/knowledge/") || strings.Contains(text, "knowledge/authentication") {
				t.Error("help retains old current-knowledge path")
			}
		})
	}
}
