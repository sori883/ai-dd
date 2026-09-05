package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/delivery"
)

func isDeliveryCommand(args []string) bool {
	return len(args) > 0 && (args[0] == "next" || args[0] == "continue")
}

func isContextReadCommand(args []string) bool {
	return len(args) > 0 && args[0] == "read-context"
}

// contextReadArguments parses the public read-context grammar. A continuation
// token is secured as one opaque argument before flags are parsed; callers
// cannot select a path, slot, or part.
func contextReadArguments(args []string) (command []string, explicitDir string, err error) {
	if len(args) == 0 || !isContextReadCommand(args) {
		return nil, "", errors.New("read-context command is required")
	}
	command = []string{args[0]}
	projectDirSeen := false
	recordError := func(argumentErr error) {
		if err == nil {
			err = argumentErr
		}
	}
	argumentStart := 1
	if len(args) > argumentStart && args[argumentStart] == "continue" {
		command = append(command, args[argumentStart])
		argumentStart++
		if argumentStart == len(args) {
			return command, "", errors.New("read-context continue requires exactly one token")
		}
		command = append(command, args[argumentStart])
		argumentStart++
	}
	for i := argumentStart; i < len(args); i++ {
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
		recordError(fmt.Errorf("read-context does not accept positional argument %q", arg))
	}
	return command, explicitDir, err
}

// deliveryArguments parses the delivery grammar while leaving the
// continuation token opaque. In particular, a malformed token may begin with
// a dash; it must reach the delivery callback so that it can be reported as a
// workflow terminal directive instead of a CLI syntax error.
func deliveryArguments(args []string) (command []string, explicitDir string, err error) {
	if len(args) == 0 || !isDeliveryCommand(args) {
		return nil, "", errors.New("delivery command is required")
	}
	command = []string{args[0]}
	projectDirSeen := false
	recordError := func(argumentErr error) {
		if err == nil {
			err = argumentErr
		}
	}
	argumentStart := 1
	if args[0] == "continue" && len(args) > argumentStart {
		// The first argument after continue is always the opaque token slot.
		// Parse project-dir options only after the token has been secured.
		command = append(command, args[argumentStart])
		argumentStart++
	}
	for i := argumentStart; i < len(args); i++ {
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

func runDeliveryNext(
	command []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	callback func(string) ([]byte, error),
) int {
	if len(command) != 1 {
		return writeDeliverySyntaxError(stderr, errors.New("next does not accept positional arguments"))
	}
	if callback == nil {
		return writeCommandError(stderr, errors.New("next delivery callback is unavailable"))
	}
	wire, err := callback(explicitDir)
	if err != nil {
		return writeDeliveryResultError(stdout, stderr, err)
	}
	return writeDeliveryWire(stdout, stderr, wire)
}

func runDeliveryContinue(
	command []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	callback func(string, string) ([]byte, error),
) int {
	if len(command) != 2 || command[1] == "" {
		return writeDeliverySyntaxError(stderr, errors.New("continue requires exactly one token"))
	}
	if callback == nil {
		return writeCommandError(stderr, errors.New("continue delivery callback is unavailable"))
	}
	wire, err := callback(command[1], explicitDir)
	if err != nil {
		return writeDeliveryResultError(stdout, stderr, err)
	}
	return writeDeliveryWire(stdout, stderr, wire)
}

func writeContextReadWire(stdout, stderr io.Writer, wire []byte) int {
	if !bytes.Equal(bytes.TrimSpace(wire), wire) {
		return writeCommandError(stderr, errors.New("context read callback returned non-canonical JSON whitespace"))
	}
	return writeDeliveryWire(stdout, stderr, wire)
}

func runContextReadStart(
	command []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	callback func(string) ([]byte, error),
) int {
	if len(command) != 1 {
		return writeDeliverySyntaxError(stderr, errors.New("read-context does not accept positional arguments"))
	}
	if callback == nil {
		return writeCommandError(stderr, errors.New("read-context callback is unavailable"))
	}
	wire, err := callback(explicitDir)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	return writeContextReadWire(stdout, stderr, wire)
}

func runContextReadContinue(
	command []string,
	explicitDir string,
	stdout io.Writer,
	stderr io.Writer,
	callback func(string, string) ([]byte, error),
) int {
	if len(command) != 3 || command[1] != "continue" || command[2] == "" {
		return writeDeliverySyntaxError(stderr, errors.New("read-context continue requires exactly one token"))
	}
	if callback == nil {
		return writeCommandError(stderr, errors.New("read-context continuation callback is unavailable"))
	}
	wire, err := callback(command[2], explicitDir)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	return writeContextReadWire(stdout, stderr, wire)
}

func writeDeliveryResultError(stdout, stderr io.Writer, err error) int {
	if delivery.IsWorkflowError(err) {
		message := err.Error()
		wire, marshalErr := json.Marshal(struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}{Kind: "error", Message: message})
		if marshalErr != nil {
			return writeCommandError(stderr, fmt.Errorf("marshal workflow error: %w", marshalErr))
		}
		return writeDeliveryWire(stdout, stderr, wire)
	}
	return writeCommandError(stderr, err)
}

func writeDeliveryWire(stdout, stderr io.Writer, wire []byte) int {
	if len(wire) == 0 || !json.Valid(wire) || bytes.ContainsAny(wire, "\r\n") {
		return writeCommandError(stderr, errors.New("delivery callback returned invalid directive JSON"))
	}
	output := make([]byte, 0, len(wire)+1)
	output = append(output, wire...)
	output = append(output, '\n')
	written, err := stdout.Write(output)
	if err == nil && written != len(output) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return writeCommandError(stderr, fmt.Errorf("write stdout: %w", err))
	}
	return 0
}

func writeDeliverySyntaxError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "aidlc: %v\n", err)
	return 2
}
