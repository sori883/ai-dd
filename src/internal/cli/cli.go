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
		_, _ = io.WriteString(stdout, helpText)
		return 0
	}

	if len(args) == 1 {
		switch args[0] {
		case "help", "--help":
			_, _ = io.WriteString(stdout, helpText)
			return 0
		case "version", "--version":
			_, _ = fmt.Fprintf(stdout, "aidlc %s (commit %s)\n", info.Version, info.Commit)
			return 0
		}
	}

	_, _ = fmt.Fprintf(stderr, "aidlc: unknown arguments: %q\n\n", strings.Join(args, " "))
	_, _ = io.WriteString(stderr, helpText)
	return 2
}
