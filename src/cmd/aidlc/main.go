// Command aidlc provides the AI-DLC command-line entry point.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func main() {
	os.Exit(cli.Run(
		os.Args[1:],
		os.Stdout,
		os.Stderr,
		buildinfo.Current(),
		spaceCreator(os.Getwd, os.Getenv, workspace.CreateSpace),
		func() {
			// Only space create promises exit 1 for writes to closed stdout or stderr pipes.
			signal.Ignore(syscall.SIGPIPE)
		},
	))
}

func spaceCreator(
	getwd func() (string, error),
	getenv func(string) string,
	create func(workspace.RootInput, string) (string, error),
) func(string, string) (string, error) {
	return func(rawName, explicitDir string) (string, error) {
		workingDir, err := getwd()
		if err != nil {
			return "", fmt.Errorf("read working directory: %w", err)
		}
		return create(workspace.RootInput{
			ExplicitDir:      explicitDir,
			AIDLCProjectDir:  getenv("AIDLC_PROJECT_DIR"),
			ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR"),
			WorkingDir:       workingDir,
		}, rawName)
	}
}
