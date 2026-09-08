package minimal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

// Hook returns Codex control JSON and never interprets model transcripts.
func (s Service) Hook(input HookInput) (map[string]any, error) {
	out := map[string]any{}
	_, err := s.withSession(input.Session, func(state *Session) ([]byte, error) {
		switch input.Event {
		case "SessionStart":
			state.RuleTurn = ""
			state.RuleHash = ""
			if err := s.save(input.Session, *state); err != nil {
				return nil, err
			}
			skill, err := okfmemory.ReadFile(s.Root, ".agents/skills/aidlc/SKILL.md")
			if err != nil {
				return nil, fmt.Errorf("read deployed aidlc skill: %w", err)
			}
			if len(skill) > 4096 {
				return nil, invalid("deployed aidlc skill exceeds 4 KiB bootstrap limit")
			}
			out["hookSpecificOutput"] = map[string]any{"hookEventName": "SessionStart", "additionalContext": fmt.Sprintf("Session: %s. Draft: %s. Required Rules are NOT loaded by this bootstrap.\n%s", input.Session, s.draftPath(input.Session), skill)}
			return nil, nil

		case "UserPromptSubmit":
			if input.Turn == "" {
				return nil, invalid("missing turn ID")
			}
			state.Turn = input.Turn

			state.RuleTurn = ""
			state.RuleHash = ""
		case "PreToolUse":
			if input.ID == "" || input.Turn == "" {
				return nil, invalid("missing tool or turn ID")
			}
			if input.Tool == "apply_patch" && s.protectedPatch(input.Input.Command) {
				return nil, invalid("use Intent CLI updates; do not patch canonical state or session state")
			}
			if s.exception(input, state) {
				return nil, nil
			}
			if state.Tool != "" {
				return nil, invalid("another tool is still running. " + s.recoveryHint(input.Session, state))
			}
			if state.Intent == "" || state.Space == "" {
				return nil, invalid("select an Intent and read its state and Rules first")
			}
			if state.Turn != input.Turn || state.RuleTurn != state.Turn || state.RuleHash == "" {
				return nil, invalid("read state and Rules for this turn with intent switch")
			}
			selected, err := (flow.Store{Root: s.Root, Space: state.Space}).Read(state.Intent)
			if err != nil {
				return nil, err
			}
			if selected.Status != "active" {
				return nil, invalid("Intent is waiting, paused or finished; resume or reopen explicitly")
			}
			_, hash, err := s.rules(state.Space)
			if err != nil {
				return nil, err
			}
			if hash != state.RuleHash {
				return nil, invalid("required Rules changed; select the Intent again to reread")
			}

			state.Tool = input.ID
		case "PostToolUse":
			if state.Tool == input.ID && input.ID != "" {
				state.Tool = ""
			} else {
				return nil, nil
			}
		case "Stop":
			if state.Tool != "" {
				if input.Active {
					out["systemMessage"] = "A tool is still running. Stopping with a warning; verify the process before recovery."
				} else {
					out["decision"] = "block"
					out["reason"] = "A tool is still running. " + s.recoveryHint(input.Session, state)
				}
			}
			return nil, nil
		default:
			return nil, invalid("unsupported hook event")
		}
		return nil, s.save(input.Session, *state)
	})
	if err != nil {
		if input.Event == "PreToolUse" {
			return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": err.Error()}}, nil
		}
		if input.Event == "Stop" {
			if input.Active {
				return map[string]any{"systemMessage": err.Error()}, nil
			}
			return map[string]any{"decision": "block", "reason": "aidlc needs recovery: " + err.Error()}, nil
		}
		return map[string]any{"continue": false, "stopReason": err.Error()}, nil
	}
	return out, nil
}
func (s Service) exception(input HookInput, state *Session) bool {
	if input.Tool == "apply_patch" {
		files := 0
		for _, line := range strings.Split(input.Input.Command, "\n") {
			if strings.HasPrefix(line, "*** Move to:") || strings.HasPrefix(line, "*** Delete File:") {
				return false
			}
			for _, prefix := range []string{"*** Add File: ", "*** Update File: "} {
				if strings.HasPrefix(line, prefix) {
					files++
					if !s.sameDraft(strings.TrimPrefix(line, prefix), input.Session) {
						return false
					}
				}
			}
		}
		return files == 1 && strings.HasPrefix(input.Input.Command, "*** Begin Patch\n") && strings.HasSuffix(strings.TrimSpace(input.Input.Command), "*** End Patch") && state.Tool == ""
	}
	if input.Tool != "Bash" {
		return false
	}
	argv, ok := shellWords(input.Input.Command)
	if !ok || len(argv) < 2 || !sameBinary(argv[0], s.Binary) {
		return false
	}
	r, err := cli.ParseMinimal(argv[1:])
	if err != nil {
		return false
	}
	if r.ProjectDir != "" && filepath.Clean(r.ProjectDir) != filepath.Clean(s.Root) {
		return false
	}
	switch r.Command + "/" + r.Action {
	case "memory/rules", "memory/search", "memory/show", "memory/check", "intent/list", "intent/show", "intent/check", "session/inspect":
		return true
	case "intent/create":
		return state.Tool == ""
	case "session/bind", "intent/switch":
		if r.Command == "session" && r.Recover && r.Session == input.Session && r.Space == state.Space && r.Target == state.Intent && state.Intent != "" {
			return true
		}
		return state.Tool == "" && r.Session == input.Session
	case "intent/configure", "intent/review", "intent/advance", "intent/wait", "intent/pause", "intent/resume", "intent/reopen", "intent/cancel", "unit/claim", "unit/result", "unit/integrate", "unit/confirm":
		return state.Tool == "" && r.Space == state.Space && r.Target == state.Intent

	}
	return false
}

// shellWords accepts literals only. It rejects expansion, redirection, command
// substitution and compound commands instead of trying to interpret a shell.
func shellWords(command string) ([]string, bool) {
	var words []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	started := false
	for _, r := range command {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '$' || r == '`' || r == '\n' || r == '\r' {
			return nil, false
		}
		if r == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if strings.ContainsRune(";|&<>()", r) {
			return nil, false
		}
		if r == ' ' || r == '\t' {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 || escaped {
		return nil, false
	}
	if started {
		words = append(words, word.String())
	}
	return words, true
}

func (s Service) protectedPatch(patch string) bool {
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			name := strings.TrimPrefix(line, prefix)
			if filepath.IsAbs(name) {
				relative, err := filepath.Rel(s.Root, name)
				if err != nil {
					return true
				}
				name = relative
			}
			name = filepath.ToSlash(filepath.Clean(name))
			if strings.HasPrefix(name, "aidlc/spaces/") && strings.Contains(name, "/intents/") {
				return true
			}
			if strings.HasPrefix(name, "aidlc/.runtime/") && !strings.HasPrefix(name, "aidlc/.runtime/drafts/") {
				return true
			}
		}
	}
	return false
}

func (s Service) recoveryHint(session string, state *Session) string {
	if state.Tool == "" {
		return "No running tool slot."
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	return "Poll a running Bash process to terminal. Only after an edit tool returned a failure and you confirmed it ended, run the same-session recovery as one command: " + quote(s.Binary) + " session bind " + quote(state.Intent) + " --space " + quote(state.Space) + " --session " + quote(session) + " --recover. Recovery clears only the failed tool slot. Retry and verify before advancing the Intent."
}

// sameBinary preserves exact configured paths and resolves absolute aliases only.
func sameBinary(command, configured string) bool {
	if command == configured {
		return true
	}
	if !filepath.IsAbs(command) || !filepath.IsAbs(configured) {
		return false
	}
	actual, err := os.Stat(command)
	if err != nil {
		return false
	}
	expected, err := os.Stat(configured)
	return err == nil && actual.Mode().IsRegular() && expected.Mode().IsRegular() && os.SameFile(actual, expected)
}
