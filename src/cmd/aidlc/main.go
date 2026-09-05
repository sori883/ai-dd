// Command aidlc provides the AI-DLC command-line entry point.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	deliverypkg "github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func main() {
	os.Exit(cli.Run(
		os.Args[1:],
		os.Stdout,
		os.Stderr,
		buildinfo.Current(),
		cli.Dependencies{
			CreateSpace: spaceCreator(os.Getwd, os.Getenv, workspace.CreateSpace),
			ListSpaces:  spaceLister(os.Getwd, os.Getenv, workspace.ReadSpaces),
			SwitchSpace: spaceSwitcher(os.Getwd, os.Getenv, workspace.SwitchSpace),
			ListIntents: intentLister(os.Getwd, os.Getenv, workspace.ReadIntents),
			SwitchIntent: intentSwitcher(
				os.Getwd,
				os.Getenv,
				workspace.SwitchIntent,
			),
			NextDelivery: deliveryNext(os.Getwd, os.Getenv, deliverypkg.Next),
			ContinueDelivery: deliveryContinue(
				os.Getwd,
				os.Getenv,
				deliverypkg.Continue,
			),
			PrepareOutput: func() {
				// Recognized workspace commands promise exit 1 for writes to closed stdout or stderr pipes.
				signal.Ignore(syscall.SIGPIPE)
			},
		},
	))
}

func intentLister(
	getwd func() (string, error),
	getenv func(string) string,
	read func(workspace.RootInput) (workspace.IntentListing, error),
) func(string) (workspace.IntentListing, error) {
	return func(explicitDir string) (workspace.IntentListing, error) {
		workingDir, err := getwd()
		if err != nil {
			return workspace.IntentListing{}, fmt.Errorf("read working directory: %w", err)
		}
		return read(workspace.RootInput{
			ExplicitDir:      explicitDir,
			AIDLCProjectDir:  getenv("AIDLC_PROJECT_DIR"),
			ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR"),
			WorkingDir:       workingDir,
		})
	}
}

func intentSwitcher(
	getwd func() (string, error),
	getenv func(string) string,
	switchIntent func(workspace.RootInput, string) (workspace.IntentSelection, error),
) func(string, string) (workspace.IntentSelection, error) {
	return func(target, explicitDir string) (workspace.IntentSelection, error) {
		workingDir, err := getwd()
		if err != nil {
			return workspace.IntentSelection{}, fmt.Errorf("read working directory: %w", err)
		}
		return switchIntent(workspace.RootInput{
			ExplicitDir:      explicitDir,
			AIDLCProjectDir:  getenv("AIDLC_PROJECT_DIR"),
			ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR"),
			WorkingDir:       workingDir,
		}, target)
	}
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

func spaceLister(
	getwd func() (string, error),
	getenv func(string) string,
	read func(workspace.RootInput) ([]workspace.Space, error),
) func(string) ([]workspace.Space, error) {
	return func(explicitDir string) ([]workspace.Space, error) {
		workingDir, err := getwd()
		if err != nil {
			return nil, fmt.Errorf("read working directory: %w", err)
		}
		return read(workspace.RootInput{
			ExplicitDir:      explicitDir,
			AIDLCProjectDir:  getenv("AIDLC_PROJECT_DIR"),
			ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR"),
			WorkingDir:       workingDir,
		})
	}
}

func spaceSwitcher(
	getwd func() (string, error),
	getenv func(string) string,
	switchSpace func(workspace.RootInput, string) (string, error),
) func(string, string) (string, error) {
	return func(rawName, explicitDir string) (string, error) {
		workingDir, err := getwd()
		if err != nil {
			return "", fmt.Errorf("read working directory: %w", err)
		}
		return switchSpace(workspace.RootInput{
			ExplicitDir:      explicitDir,
			AIDLCProjectDir:  getenv("AIDLC_PROJECT_DIR"),
			ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR"),
			WorkingDir:       workingDir,
		}, rawName)
	}
}
