// Package cli implements the aidlc command-line contract.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

const helpText = `AI-DLC four-stage workflow

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
  aidlc intent advance <id> --space <space> --expect <revision>
  aidlc intent pause|resume|cancel <id> --space <space> --expect <revision> --reason <text>
  aidlc intent wait <id> --space <space> --expect <revision> --reason <text> --resume-condition <text>
  aidlc intent reopen <id> --space <space> --expect <revision> --reason <text> --stage <stage>
  aidlc unit claim|result|integrate|confirm|reassign <id> --space <space> --expect <revision> --file <request.json>
  aidlc memory create <concept-id> --space <space> --body-file <body> --actor <actor> --type <type> --title <title> --description <description>
  aidlc memory update <concept-id> --space <space> --body-file <body> --actor <actor> --expect <hash>
  aidlc memory show <concept-id> --space <space>
  aidlc memory search [query] --space <space> [--intent-id <id>]
  aidlc memory rules|check --space <space>
  aidlc session bind <id> --space <space> --session <session> [--recover]
  aidlc session inspect --session <session>
  aidlc help | version

Stages: discovery, planning, tdd, integration. Each boundary needs Sensor and independent review.
Use --project-dir <root> for explicit project selection. Concept IDs have no .md extension.
Exit codes: 0 success, 2 invalid input or conflict, 1 operational failure.
`

// Dependencies groups the workspace operations used by Run. Nil callbacks are
// valid for commands that do not invoke the corresponding operation.
type Dependencies struct {
	Minimal       func(MinimalRequest) ([]byte, error)
	CreateSpace   func(rawName, explicitDir string) (string, error)
	ListSpaces    func(explicitDir string) ([]workspace.Space, error)
	SwitchSpace   func(rawName, explicitDir string) (string, error)
	PrepareOutput func()
}

// Run executes the CLI with injected process inputs, outputs, and workspace operations.
// Each callback runs at most once and only for its syntactically valid command.
// PrepareOutput runs once before callback or output for any recognized workspace command,
// including commands with syntax errors.
func Run(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	info buildinfo.Info,
	dependencies Dependencies,
) int {
	if text, ok := Help(args); ok {
		return writeStdout(stdout, stderr, text)
	}
	if len(args) == 0 {
		return writeStdout(stdout, stderr, helpText)
	}

	if len(args) == 1 {
		switch args[0] {
		case "help", "--help":
			return writeStdout(stdout, stderr, helpText)
		case "version", "--version":
			return writeStdout(
				stdout,
				stderr,
				fmt.Sprintf("aidlc %s (commit %s)\n", info.Version, info.Commit),
			)
		}
	}
	if isMinimal(args) {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		return runMinimal(args, stdout, stderr, dependencies)
	}
	command, explicitDir, _, err := workspaceArguments(args, false)
	hasSpaceSubcommand := len(command) >= 2 && command[0] == "space"
	isSpaceCreate := hasSpaceSubcommand && command[1] == "create"
	if isSpaceCreate {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		if err != nil {
			return writeCommandError(stderr, err)
		}
		return runSpaceCreate(
			command[2:],
			explicitDir,
			stdout,
			stderr,
			dependencies.CreateSpace,
		)
	}
	isSpaceSwitch := hasSpaceSubcommand && command[1] == "switch"
	if isSpaceSwitch {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		if err != nil {
			return writeCommandError(stderr, err)
		}
		return runSpaceSwitch(
			command[2:],
			explicitDir,
			stdout,
			stderr,
			dependencies.SwitchSpace,
		)
	}
	isSpaceList := hasSpaceSubcommand && command[1] == "list"
	isBareSpace := len(command) == 1 && command[0] == "space"
	if isSpaceList || isBareSpace {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		// --json is list-only; reparse after classification to preserve create's diagnostics.
		_, explicitDir, jsonOutput, err := workspaceArguments(args, true)
		if err != nil {
			return writeCommandError(stderr, err)
		}
		if len(command) > 2 {
			return writeCommandError(stderr, errors.New("space list does not accept positional arguments"))
		}
		return runSpaceList(
			explicitDir,
			jsonOutput,
			stdout,
			stderr,
			dependencies.ListSpaces,
		)
	}

	_, _ = fmt.Fprintf(stderr, "aidlc: unknown arguments: %q\n\n", strings.Join(args, " "))
	_, _ = io.WriteString(stderr, helpText)
	return 2
}

func writeStdout(stdout, stderr io.Writer, output string) int {
	if _, err := io.WriteString(stdout, output); err != nil {
		_, _ = fmt.Fprintf(stderr, "aidlc: write stdout: %v\n", err)
		return 1
	}

	return 0
}
