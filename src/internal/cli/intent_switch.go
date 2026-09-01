package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/sori883/ai-dd/src/internal/workspace"
)

func runIntentSwitch(
	targets []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	switchIntent func(string, string) (workspace.IntentSelection, error),
) int {
	if len(targets) != 1 {
		return writeCommandError(stderr, errors.New("intent switch requires exactly one target"))
	}
	switch targets[0] {
	case "", "help", "-h":
		return writeCommandError(stderr, fmt.Errorf("invalid intent target %q", targets[0]))
	}
	selection, err := switchIntent(targets[0], explicitDir)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	output := fmt.Sprintf(
		"Active intent → %s (space: %s)\n",
		selection.DirName,
		selection.SpaceName,
	)
	n, err := io.WriteString(stdout, output)
	if err == nil && n != len(output) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeCommandError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}
