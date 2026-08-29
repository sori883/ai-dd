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

Commands:
  help       Show help
  version    Show version information

Flags:
  --help     Show help
  --version  Show version information
`

// Run executes the CLI with injected process inputs and outputs and returns an exit code.
func Run(args []string, stdout, stderr io.Writer, info buildinfo.Info) int {
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
