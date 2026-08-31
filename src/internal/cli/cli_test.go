package cli_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

const wantHelp = `AI-DLC command-line interface

Usage:
  aidlc <command>
  aidlc space create <name> [--project-dir <path>]

Commands:
  help       Show help
  version    Show version information
  space create  Create a new space

Flags:
  --help     Show help
  --version  Show version information
  --project-dir <path>  Project directory for space create
`

func TestRun_Help(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no arguments", args: nil},
		{name: "help command", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				nil,
			)

			if exitCode != 0 {
				t.Errorf("exit code = %d, want 0", exitCode)
			}
			if got := stdout.String(); got != wantHelp {
				t.Errorf("stdout = %q, want %q", got, wantHelp)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRun_HelpWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no arguments", args: nil},
		{name: "help command", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			exitCode := cli.Run(
				tt.args,
				errorWriter{err: errors.New("broken pipe")},
				&stderr,
				buildinfo.Info{},
				nil,
				nil,
			)

			if exitCode != 1 {
				t.Errorf("exit code = %d, want 1", exitCode)
			}
			const wantStderr = "aidlc: write stdout: broken pipe\n"
			if got := stderr.String(); got != wantStderr {
				t.Errorf("stderr = %q, want %q", got, wantStderr)
			}
		})
	}
}

func TestRun_Version(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "version command", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{Version: "v1.2.3", Commit: "abcdef0"},
				nil,
				nil,
			)

			const wantStdout = "aidlc v1.2.3 (commit abcdef0)\n"
			if exitCode != 0 {
				t.Errorf("exit code = %d, want 0", exitCode)
			}
			if got := stdout.String(); got != wantStdout {
				t.Errorf("stdout = %q, want %q", got, wantStdout)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRun_VersionWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "version command", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			exitCode := cli.Run(
				tt.args,
				errorWriter{err: errors.New("broken pipe")},
				&stderr,
				buildinfo.Info{Version: "v1.2.3", Commit: "abcdef0"},
				nil,
				nil,
			)

			if exitCode != 1 {
				t.Errorf("exit code = %d, want 1", exitCode)
			}
			const wantStderr = "aidlc: write stdout: broken pipe\n"
			if got := stderr.String(); got != wantStderr {
				t.Errorf("stderr = %q, want %q", got, wantStderr)
			}
		})
	}
}

func TestRun_UnknownArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "unknown command",
			args:       []string{"unknown"},
			wantStderr: "aidlc: unknown arguments: \"unknown\"\n\n" + wantHelp,
		},
		{
			name:       "extra argument",
			args:       []string{"help", "extra"},
			wantStderr: "aidlc: unknown arguments: \"help extra\"\n\n" + wantHelp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				nil,
			)

			if exitCode != 2 {
				t.Errorf("exit code = %d, want 2", exitCode)
			}
			if got := stdout.String(); got != "" {
				t.Errorf("stdout = %q, want empty", got)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}
