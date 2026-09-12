package codex

import (
	"encoding/json"
	"strings"
)

func hookConfiguration(root, binary string) ([]byte, error) {
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellQuote(binary) + " __hook --project-dir " + shellQuote(root), "timeout": 10}}}
		if event == "SessionStart" {
			group["hooks"].([]any)[0].(map[string]any)["additionalContextLimit"] = 8192
		}
		if event == "PreToolUse" || event == "PostToolUse" {
			group["matcher"] = assignmentMatcher
		}
		hooks[event] = []any{group}
	}
	data, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

const assignmentMatcher = "^(Bash|apply_patch|spawn_agent|collaborationspawn_agent|followup_task|collaborationfollowup_task|send_message|collaborationsend_message|interrupt_agent|collaborationinterrupt_agent)$"
