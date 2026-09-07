package minimal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/kdr"
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
			state.Dirty = true
			state.RuleTurn = ""
			state.RuleHash = ""
		case "PreToolUse":
			if input.ID == "" || input.Turn == "" {
				return nil, invalid("missing tool or turn ID")
			}
			if input.Tool == "apply_patch" && s.protectedPatch(input.Input.Command) {
				return nil, invalid("use KDR CLI updates; do not patch canonical records or session state")
			}
			if s.exception(input, state) {
				return nil, nil
			}
			if state.Tool != "" {
				return nil, invalid("another tool is still running; poll it to completion")
			}
			if state.Intent == "" || state.Space == "" {
				return nil, invalid("select an Intent and read its KDR and Rules first")
			}
			if state.Turn != input.Turn || state.RuleTurn != state.Turn || state.RuleHash == "" {
				return nil, invalid("read KDR and Rules for this turn with intent switch")
			}
			saved, err := s.store(state.Space).Read(state.Intent)
			if err != nil {
				return nil, err
			}
			if _, err := kdr.Parse(saved.Raw, state.Intent); err != nil {
				return nil, err
			}
			_, hash, err := s.rules(state.Space)
			if err != nil {
				return nil, err
			}
			if hash != state.RuleHash {
				return nil, invalid("required Rules changed; select the Intent again to reread")
			}
			state.Dirty = true
			state.Tool = input.ID
		case "PostToolUse":
			if state.Tool == input.ID && input.ID != "" {
				state.Tool = ""
			} else {
				return nil, nil
			}
		case "Stop":
			if state.Dirty || state.Tool != "" {
				if input.Active {
					out["systemMessage"] = "KDR is still unrecorded or a tool is running. Stopping with a warning; this is not a completion claim."
				} else {
					out["decision"] = "block"
					out["reason"] = "Record the current findings, verification and remaining work in the same KDR using the aidlc skill. Poll any running tool first. Then finish; do not repeat completed work."
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
	if !ok || len(argv) < 2 || argv[0] != s.Binary {
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
	case "kdr/template", "kdr/list", "kdr/show", "kdr/check", "memory/rules", "memory/search", "memory/show", "memory/check", "intent/list", "session/inspect":
		return true
	case "kdr/create", "intent/create":
		return state.Tool == "" && s.sameDraft(r.File, input.Session)
	case "session/bind", "intent/switch":
		return state.Tool == "" && r.Session == input.Session
	case "kdr/repair":
		return state.Tool == "" && r.Session == input.Session && s.sameDraft(r.File, input.Session)
	case "kdr/update":
		return state.Tool == "" && r.Session == input.Session && r.Space == state.Space && r.Target == state.Intent && s.sameDraft(r.File, input.Session)
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
			if strings.HasPrefix(name, "aidlc/spaces/") && strings.Contains(name, "/knowledge/kdr/") {
				return true
			}
			if strings.HasPrefix(name, "aidlc/.runtime/") && !strings.HasPrefix(name, "aidlc/.runtime/drafts/") {
				return true
			}
		}
	}
	return false
}
