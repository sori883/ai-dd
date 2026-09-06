package cli

import (
	"errors"
	"fmt"
	"strings"
)

type codexStageRequest struct {
	action      string
	explicitDir string
}

func isCodexStageCommand(args []string) bool {
	return len(args) > 0 && args[0] == codexStageCommand
}

func parseCodexStageArguments(args []string) (codexStageRequest, error) {
	if !isCodexStageCommand(args) {
		return codexStageRequest{}, errors.New("codex stage command is required")
	}
	if len(args) < 2 {
		return codexStageRequest{}, errors.New("codex stage action is required")
	}
	request := codexStageRequest{action: args[1]}
	switch request.action {
	case "decision", "summary", "answer", "review-request", "review-complete", "run-sensors", "learnings-surface", "learnings-persist":
	default:
		return codexStageRequest{}, fmt.Errorf("unknown codex stage action %q", request.action)
	}
	seenProjectDir := false
	for index := 2; index < len(args); index++ {
		argument := args[index]
		value, equals := strings.CutPrefix(argument, "--project-dir=")
		if argument == "--project-dir" || equals {
			if seenProjectDir {
				return codexStageRequest{}, errors.New("duplicate --project-dir")
			}
			seenProjectDir = true
			if !equals {
				if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
					return codexStageRequest{}, errors.New("--project-dir requires a nonempty path")
				}
				index++
				value = args[index]
			}
			if value == "" {
				return codexStageRequest{}, errors.New("--project-dir requires a nonempty path")
			}
			request.explicitDir = value
			continue
		}
		if strings.HasPrefix(argument, "-") {
			return codexStageRequest{}, fmt.Errorf("unknown flag %q", argument)
		}
		return codexStageRequest{}, fmt.Errorf("codex stage does not accept positional argument %q", argument)
	}
	return request, nil
}
