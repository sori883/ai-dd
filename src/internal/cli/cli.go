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
  aidlc intent list [--json] [--project-dir <path>]
  aidlc intent [--json] [--project-dir <path>]
  aidlc intent switch <target> [--project-dir <path>]
  aidlc intent <target> [--project-dir <path>]

Commands:
  help       Show help
  version    Show version information
  space create  Create a new space
  space list    List spaces (space is an alias)
  space switch  Select an existing space
  intent list   List intents (intent is an alias)
  intent switch Select an existing intent

Flags:
  --help     Show help
  --version  Show version information
  --project-dir <path>  Project directory for workspace commands
  --json     Print space or intent lists as JSON
`

// Dependencies groups the workspace operations used by Run. Nil callbacks are
// valid for commands that do not invoke the corresponding operation.
type Dependencies struct {
	CreateSpace   func(rawName, explicitDir string) (string, error)
	ListSpaces    func(explicitDir string) ([]workspace.Space, error)
	SwitchSpace   func(rawName, explicitDir string) (string, error)
	ListIntents   func(explicitDir string) (workspace.IntentListing, error)
	SwitchIntent  func(target, explicitDir string) (workspace.IntentSelection, error)
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
	isIntentSwitch := len(command) >= 2 && command[0] == "intent" && command[1] == "switch"
	if isIntentSwitch {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		if err != nil {
			return writeCommandError(stderr, err)
		}
		return runIntentSwitch(
			command[2:],
			explicitDir,
			stdout,
			stderr,
			dependencies.SwitchIntent,
		)
	}
	isIntentList := len(command) >= 2 && command[0] == "intent" && command[1] == "list"
	isBareIntent := len(command) == 1 && command[0] == "intent"
	if isIntentList || isBareIntent {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		_, explicitDir, jsonOutput, err := workspaceArguments(args, true)
		if err != nil {
			return writeCommandError(stderr, err)
		}
		if len(command) > 2 {
			return writeCommandError(stderr, errors.New("intent list does not accept positional arguments"))
		}
		return runIntentList(
			explicitDir,
			jsonOutput,
			stdout,
			stderr,
			dependencies.ListIntents,
		)
	}
	isBareIntentSwitch := len(command) >= 2 && command[0] == "intent" && !isIntentVerb(command[1])
	if isBareIntentSwitch {
		if dependencies.PrepareOutput != nil {
			dependencies.PrepareOutput()
		}
		if err != nil {
			return writeCommandError(stderr, err)
		}
		return runIntentSwitch(
			command[1:],
			explicitDir,
			stdout,
			stderr,
			dependencies.SwitchIntent,
		)
	}

	_, _ = fmt.Fprintf(stderr, "aidlc: unknown arguments: %q\n\n", strings.Join(args, " "))
	_, _ = io.WriteString(stderr, helpText)
	return 2
}

func isIntentVerb(value string) bool {
	switch value {
	case "help", "list", "switch", "create", "archive", "rename", "show", "birth":
		return true
	default:
		return false
	}
}

func writeStdout(stdout, stderr io.Writer, output string) int {
	if _, err := io.WriteString(stdout, output); err != nil {
		_, _ = fmt.Fprintf(stderr, "aidlc: write stdout: %v\n", err)
		return 1
	}

	return 0
}
