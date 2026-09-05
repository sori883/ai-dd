package cli_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestRunCodexHumanTurnHookCommandIsSilentAndFailOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hookError error
	}{
		{name: "success"},
		{name: "hook failure", hookError: errors.New("append failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			var prepareCalls, hookCalls int
			code := cli.Run([]string{"__codex-user-prompt-submit"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				HumanTurnHook: func() error {
					hookCalls++
					return tt.hookError
				},
				PrepareOutput: func() { prepareCalls++ },
			})
			if code != 0 {
				t.Errorf("Run(hidden hook) exit = %d, want 0", code)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("hidden hook stdout/stderr = %q/%q, want empty", stdout.String(), stderr.String())
			}
			if hookCalls != 1 || prepareCalls != 1 {
				t.Errorf("hook/prepare calls = %d/%d, want 1/1", hookCalls, prepareCalls)
			}
		})
	}
}

func TestRunHelpDoesNotExposeCodexHumanTurnHookCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"help"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{}); code != 0 {
		t.Fatalf("Run(help) exit = %d, want 0", code)
	}
	if bytes.Contains(stdout.Bytes(), []byte("__codex-user-prompt-submit")) {
		t.Fatalf("help exposes hidden hook command: %q", stdout.String())
	}
}
