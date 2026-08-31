package cli

import (
	"errors"
	"fmt"
	"io"
)

func runSpaceSwitch(
	names []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	switchSpace func(string, string) (string, error),
) int {
	if len(names) != 1 {
		return writeSpaceError(stderr, errors.New("space switch requires exactly one name"))
	}
	switch names[0] {
	case "", "help", "-h":
		return writeSpaceError(stderr, fmt.Errorf("invalid space name %q", names[0]))
	}
	name, err := switchSpace(names[0], explicitDir)
	if err != nil {
		return writeSpaceError(stderr, err)
	}
	output := fmt.Sprintf("Active space → %s\n", name)
	n, err := io.WriteString(stdout, output)
	if err == nil && n != len(output) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeSpaceError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}
