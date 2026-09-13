package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"strings"
	"testing"
)

func TestFiveCLIContract(t *testing.T) {
	for _, args := range [][]string{{"memory", "rules", "--space", "default"}, {"install", "codex"}} {
		t.Run(args[0], func(t *testing.T) {
			if _, err := ParseCommand(args); err == nil {
				t.Fatal("retired command accepted")
			}
			var out, err bytes.Buffer
			called := false
			code := Run(args, &out, &err, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { called = true; return nil, nil }})
			if code != 2 || called {
				t.Fatalf("code %d callback %v", code, called)
			}
		})
	}
	var out, err bytes.Buffer
	Run([]string{"--help"}, &out, &err, buildinfo.Info{}, Dependencies{})
	if strings.Contains(out.String(), "aidlc memory") || strings.Contains(out.String(), "aidlc install") {
		t.Fatal("retired entry advertised")
	}
}
