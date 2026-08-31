package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func spaceCreateArguments(args []string) (command []string, explicitDir string, err error) {
	command = []string{}
	projectDirSeen := false
	recordError := func(argumentErr error) {
		if err == nil {
			err = argumentErr
		}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value, equalsForm := strings.CutPrefix(arg, "--project-dir=")
		if arg == "--project-dir" || equalsForm {
			if projectDirSeen {
				recordError(errors.New("duplicate --project-dir"))
			}
			projectDirSeen = true
			if !equalsForm {
				if i+1 == len(args) || strings.HasPrefix(args[i+1], "-") {
					recordError(errors.New("--project-dir requires a nonempty path"))
					continue
				}
				i++
				value = args[i]
			}
			if value == "" {
				recordError(errors.New("--project-dir requires a nonempty path"))
			}
			explicitDir = value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			recordError(fmt.Errorf("unknown flag %q", arg))
			continue
		}
		command = append(command, arg)
	}
	return command, explicitDir, err
}

func runSpaceCreate(
	names []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	createSpace func(string, string) (string, error),
) int {
	if len(names) != 1 {
		return writeSpaceError(stderr, errors.New("space create requires exactly one name"))
	}
	switch names[0] {
	case "", "help", "-h":
		return writeSpaceError(stderr, fmt.Errorf("invalid space name %q", names[0]))
	}
	name, err := createSpace(names[0], explicitDir)
	if err != nil {
		return writeSpaceError(stderr, err)
	}
	output := fmt.Sprintf("Space created: %s\n", name)
	n, err := io.WriteString(stdout, output)
	if err == nil && n != len(output) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeSpaceError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}

func writeSpaceError(stderr io.Writer, err error) int {
	_ = json.NewEncoder(stderr).Encode(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
	return 1
}
