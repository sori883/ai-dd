// Package cli implements the aidlc command-line contract.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
)

const helpText = `AI-DLC command-line interface

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

// Run executes the CLI with injected process inputs and outputs and returns an exit code.
// createSpace is called once for a syntactically valid space create command only.
// If non-nil, prepareSpaceOutput runs once for a recognized space create command,
// before its callback or any output, including syntax errors.
func Run(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	info buildinfo.Info,
	createSpace func(rawName, explicitDir string) (string, error),
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
	command, explicitDir, err := spaceCreateArguments(args)
	isSpaceCreate := len(command) >= 2 && command[0] == "space" && command[1] == "create"
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
