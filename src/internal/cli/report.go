package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
)

// reportRequest is the validated public report grammar. The values remain
// untrimmed so exact human choices and feedback reach the workflow boundary
// unchanged.
type reportRequest struct {
	stage       string
	result      string
	userInput   string
	reason      string
	explicitDir string
}

func isReportCommand(args []string) bool {
	return len(args) > 0 && args[0] == "report"
}

func parseReportArguments(args []string) (reportRequest, error) {
	if !isReportCommand(args) {
		return reportRequest{}, errors.New("report command is required")
	}

	var request reportRequest
	seen := make(map[string]bool, 5)
	for index := 1; index < len(args); index++ {
		argument := args[index]
		name, value, equals := strings.Cut(argument, "=")
		switch name {
		case "--stage", "--result", "--user-input", "--reason", "--project-dir":
			if seen[name] {
				return reportRequest{}, fmt.Errorf("duplicate %s", name)
			}
			seen[name] = true
			if !equals {
				if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
					return reportRequest{}, fmt.Errorf("%s requires a nonempty value", name)
				}
				index++
				value = args[index]
			}
			if value == "" {
				return reportRequest{}, fmt.Errorf("%s requires a nonempty value", name)
			}
			switch name {
			case "--stage":
				request.stage = value
			case "--result":
				if !validReportResult(value) {
					return reportRequest{}, fmt.Errorf("unknown report result %q", value)
				}
				request.result = value
			case "--user-input":
				request.userInput = value
			case "--reason":
				request.reason = value
			case "--project-dir":
				request.explicitDir = value
			}
		default:
			if strings.HasPrefix(argument, "-") {
				return reportRequest{}, fmt.Errorf("unknown flag %q", argument)
			}
			return reportRequest{}, fmt.Errorf("report does not accept positional argument %q", argument)
		}
	}
	if !seen["--stage"] {
		return reportRequest{}, errors.New("report requires --stage")
	}
	if !seen["--result"] {
		return reportRequest{}, errors.New("report requires --result")
	}
	return request, nil
}

func validReportResult(value string) bool {
	switch value {
	case "awaiting-approval", "rejected", "revised", "approved":
		return true
	default:
		return false
	}
}

func runReport(
	request reportRequest,
	stdout io.Writer,
	stderr io.Writer,
	callback func(stage, result, userInput, reason, explicitDir string) ([]byte, error),
) int {
	if callback == nil {
		return writeCommandError(stderr, errors.New("report callback is unavailable"))
	}
	wire, err := callback(request.stage, request.result, request.userInput, request.reason, request.explicitDir)
	if err != nil {
		return writeReportResultError(stdout, stderr, err)
	}
	expectedKind := "print"
	if request.result == "approved" {
		expectedKind = "done"
	}
	return writeReportWire(stdout, stderr, expectedKind, wire)
}

func writeReportSyntaxError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "aidlc: %v\n", err)
	return 2
}

func writeReportResultError(stdout, stderr io.Writer, err error) int {
	if delivery.IsWorkflowError(err) || orchestrator.IsWorkflowError(err) {
		wire, marshalErr := json.Marshal(struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}{Kind: "error", Message: err.Error()})
		if marshalErr != nil {
			return writeCommandError(stderr, fmt.Errorf("marshal report workflow error: %w", marshalErr))
		}
		return writeReportWire(stdout, stderr, "error", wire)
	}
	return writeCommandError(stderr, err)
}

func writeReportWire(stdout, stderr io.Writer, expectedKind string, wire []byte) int {
	if err := validateReportWire(expectedKind, wire); err != nil {
		return writeCommandError(stderr, err)
	}
	return writeDeliveryWire(stdout, stderr, wire)
}

func validateReportWire(expectedKind string, wire []byte) error {
	if len(wire) == 0 || !bytes.Equal(bytes.TrimSpace(wire), wire) || bytes.ContainsAny(wire, "\r\n") {
		return errors.New("report callback returned non-canonical directive JSON")
	}
	switch expectedKind {
	case "print":
		var value struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(wire, &value); err != nil || value.Kind != "print" || value.Message == "" {
			return errors.New("report callback returned invalid print directive JSON")
		}
		canonical, err := json.Marshal(value)
		if err != nil || !bytes.Equal(canonical, wire) {
			return errors.New("report callback returned non-canonical print directive JSON")
		}
	case "done":
		var value struct {
			Kind   string `json:"kind"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(wire, &value); err != nil || value.Kind != "done" || value.Reason == "" {
			return errors.New("report callback returned invalid done directive JSON")
		}
		canonical, err := json.Marshal(value)
		if err != nil || !bytes.Equal(canonical, wire) {
			return errors.New("report callback returned non-canonical done directive JSON")
		}
	case "error":
		var value struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(wire, &value); err != nil || value.Kind != "error" || value.Message == "" {
			return errors.New("report callback returned invalid error directive JSON")
		}
		canonical, err := json.Marshal(value)
		if err != nil || !bytes.Equal(canonical, wire) {
			return errors.New("report callback returned non-canonical error directive JSON")
		}
	default:
		return fmt.Errorf("report callback expected unsupported directive kind %q", expectedKind)
	}
	return nil
}
