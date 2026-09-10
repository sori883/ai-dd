package cli_test

import (
	"bytes"
	"errors"
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

const wantHelp = `AI-DLC 6段階のIntent実行計画
initialization → discoveryが必須。architecture-analysis / planning / tdd / integrationは計画で採否と順序を承認する。

Usage:
  aidlc install codex --project-dir <root>
  aidlc install codex --relocate --project-dir <new-root> --from-project-dir <old-root> --from-binary <old-binary>
  aidlc space create <name> [--project-dir <path>]
  aidlc space list [--json] [--project-dir <path>]
  aidlc space [--json] [--project-dir <path>]
  aidlc space switch <name> [--project-dir <path>]
  aidlc intent create <name> --space <space>
  aidlc intent list --space <space>
  aidlc intent switch <name>|--id <id> --space <space> --session <session>
  aidlc intent show <id> --space <space>
  aidlc intent procedure <id> --space <space>
  aidlc intent configure <id> --space <space> --expect <revision> --file <config.json>
  aidlc intent check <id> --space <space> [--boundary start|end]
  aidlc intent begin <id> --space <space> --expect <revision>
  aidlc intent review <id> --space <space> --expect <revision> --file <review.json>
  aidlc intent plan <id> --space <space> [--expect <revision> --file <plan.json>]
  aidlc intent plan-approval <id> --space <space> --expect <revision> --file <decision.json>
  aidlc intent approval <id> --space <space> --expect <revision> --file <decision.json>
  aidlc intent finish <id> --space <space> --expect <revision>
  aidlc intent history <id> --space <space>
  aidlc intent pause|resume|cancel <id> --space <space> --expect <revision> --reason <text>
  aidlc intent wait <id> --space <space> --expect <revision> --reason <text> --resume-condition <text>
  aidlc intent reopen <id> --space <space> --expect <revision> --reason <text> --step <step-id>
  aidlc unit claim|result|integrate|confirm|reassign <id> --space <space> --expect <revision> --file <request.json>
  aidlc memory create <concept-id> --space <space> --body-file <body> --actor <actor> --type <type> --title <title> --description <description>
  aidlc memory update <concept-id> --space <space> --body-file <body> --actor <actor> --expect <hash>
  aidlc memory show <concept-id> --space <space>
  aidlc memory search [query] --space <space> [--intent-id <id>]
  aidlc memory rules|check --space <space>
  aidlc session bind <id> --space <space> --session <session> [--recover]
  aidlc session inspect --session <session>
  aidlc help | version

Stages: initialization → discoveryは必須。architecture-analysis, planning, tdd, integrationは採否と順序を計画で承認する。各実行の完了にはSensor、独立review、成果の会話承認が必要。
Use --project-dir <root> for explicit project selection. Concept IDs have no .md extension.
Exit codes: 0 success, 2 invalid input or conflict, 1 operational failure.
`
const wantPublicHelp = wantHelp + "\n  assignment init/list/show/reserve/check/release/reset — native worker作業場所の登録と明示解放（各ACTION --help）\n"

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
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					nil,
					nil))

			if exitCode != 0 {
				t.Errorf("exit code = %d, want 0", exitCode)
			}
			if got := stdout.String(); got != wantPublicHelp {
				t.Errorf("stdout = %q, want %q", got, wantPublicHelp)
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
				buildinfo.Info{Version: "v1.2.3", Commit: "abcdef0"}, runDependencies(

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
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}
