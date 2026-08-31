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

const helpText = `AI-DLC command-line interface

Usage:
  aidlc <command>
  aidlc space create <name> [--project-dir <path>]
  aidlc space list [--json] [--project-dir <path>]
  aidlc space switch <name> [--project-dir <path>]
  aidlc space [--json] [--project-dir <path>]

Commands:
  help       Show help
  version    Show version information
  space create  Create a new space
  space list    List spaces (space is an alias)
  space switch  Select an existing space

Flags:
  --help     Show help
  --version  Show version information
  --project-dir <path>  Project directory for space commands
  --json     Print space lists as JSON
`

// Run executes the CLI with injected process inputs and outputs and returns an exit code.
// createSpace is called once for a syntactically valid space create command only.
// listSpaces is called once for a syntactically valid space list or bare space command only.
// switchSpace is called once for a syntactically valid space switch command only.
// If non-nil, prepareSpaceOutput runs once for recognized create, switch, list, or bare space,
// before its callback or any output, including syntax errors.
func Run(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	info buildinfo.Info,
	createSpace func(rawName, explicitDir string) (string, error),
	listSpaces func(explicitDir string) ([]workspace.Space, error),
	switchSpace func(rawName, explicitDir string) (string, error),
	prepareSpaceOutput func(),
) int {
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
	command, explicitDir, _, err := spaceArguments(args, false)
	hasSpaceSubcommand := len(command) >= 2 && command[0] == "space"
	isSpaceCreate := hasSpaceSubcommand && command[1] == "create"
	if isSpaceCreate {
		if prepareSpaceOutput != nil {
			prepareSpaceOutput()
		}
		if err != nil {
			return writeSpaceError(stderr, err)
		}
		return runSpaceCreate(
			command[2:],
			explicitDir,
			stdout,
			stderr,
			createSpace,
		)
	}
	isSpaceSwitch := hasSpaceSubcommand && command[1] == "switch"
	if isSpaceSwitch {
		if prepareSpaceOutput != nil {
			prepareSpaceOutput()
		}
		if err != nil {
			return writeSpaceError(stderr, err)
		}
		return runSpaceSwitch(
			command[2:],
			explicitDir,
			stdout,
			stderr,
			switchSpace,
		)
	}
	isSpaceList := hasSpaceSubcommand && command[1] == "list"
	isBareSpace := len(command) == 1 && command[0] == "space"
	if isSpaceList || isBareSpace {
		if prepareSpaceOutput != nil {
			prepareSpaceOutput()
		}
		// --json is list-only; reparse after classification to preserve create's diagnostics.
		_, explicitDir, jsonOutput, err := spaceArguments(args, true)
		if err != nil {
			return writeSpaceError(stderr, err)
		}
		if len(command) > 2 {
			return writeSpaceError(stderr, errors.New("space list does not accept positional arguments"))
		}
		return runSpaceList(
			explicitDir,
			jsonOutput,
			stdout,
			stderr,
			listSpaces,
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
