package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func runDependencies(
	createSpace func(string, string) (string, error),
	listSpaces func(string) ([]workspace.Space, error),
	switchSpace func(string, string) (string, error),
	prepareOutput func(),
) cli.Dependencies {
	return cli.Dependencies{
		CreateSpace:   createSpace,
		ListSpaces:    listSpaces,
		SwitchSpace:   switchSpace,
		PrepareOutput: prepareOutput,
	}
}

func TestRun_Help(t *testing.T) {
	var baseline string
	for _, args := range [][]string{nil, {"help"}, {"--help"}} {
		var out, errout bytes.Buffer
		calls := 0
		deps := cli.Dependencies{Execute: func(cli.CommandRequest) ([]byte, error) { calls++; return nil, nil }, PrepareOutput: func() { calls++ }, CreateSpace: func(string, string) (string, error) { calls++; return "", nil }, ListSpaces: func(string) ([]workspace.Space, error) { calls++; return nil, nil }, SwitchSpace: func(string, string) (string, error) { calls++; return "", nil }}
		code := cli.Run(args, &out, &errout, buildinfo.Info{}, deps)
		if code != 0 || calls != 0 || errout.Len() != 0 || out.Len() == 0 {
			t.Fatalf("help %v: %d calls=%d %s", args, code, calls, &errout)
		}
		if baseline == "" {
			baseline = out.String()
			for _, operation := range []string{"space create", "space list", "space switch", "intent create", "intent plan ", "intent plan-approval ", "intent approval ", "intent finish ", "intent history ", "unit claim|result|integrate|confirm|reassign", "session bind", "assignment init/list/show/reserve/check/release/reset"} {
				if !strings.Contains(baseline, operation) {
					t.Errorf("missing operation %s", operation)
				}
			}
		} else if out.String() != baseline {
			t.Fatal("help aliases differ")
		}
	}
}

func TestRun_HelpWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
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
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					nil,
					nil))

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
				buildinfo.Info{Version: "v1.2.3", Commit: "abcdef0"}, runDependencies(

					nil,
					nil,
					nil,
					nil))

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
			wantStderr: "aidlc: unknown arguments: \"unknown\"\n\n",
		},
		{
			name:       "extra argument",
			args:       []string{"help", "extra"},
			wantStderr: "aidlc: unknown arguments: \"help extra\"\n\n",
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
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					nil,
					nil))

			if exitCode != 2 {
				t.Errorf("exit code = %d, want 2", exitCode)
			}
			if got := stdout.String(); got != "" {
				t.Errorf("stdout = %q, want empty", got)
			}
			if got := stderr.String(); !strings.HasPrefix(got, tt.wantStderr) || !strings.Contains(got, "aidlc help") {
				t.Errorf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}
