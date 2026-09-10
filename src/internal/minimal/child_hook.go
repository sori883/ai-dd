package minimal

import (
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
)

// childHook never writes the parent conversation or interprets child transcripts.
func (s Service) childHook(input HookInput) (map[string]any, error) {
	if input.Event != "PreToolUse" {
		return map[string]any{}, nil
	}
	if err := s.childPre(input); err != nil {
		return map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": err.Error()}}, nil
	}
	return map[string]any{}, nil
}
func (s Service) childPre(in HookInput) error {
	if in.AgentID == "" || in.AgentType == "" || in.ID == "" || in.Turn == "" {
		return invalid("child agent type and tool identity required")
	}
	parent, err := s.Inspect(in.Session)
	if err != nil {
		return err
	}
	if parent.Intent == "" || parent.Space == "" {
		return invalid("parent selected Intent required")
	}
	store := flow.Store{Root: s.Root, Space: parent.Space}
	st, err := store.Read(parent.Intent)
	if err != nil {
		return err
	}
	if _, err := store.AssignmentStage(st.ID, st.CurrentStepID, in.AgentType); err != nil {
		return err
	}
	if in.Tool != "Bash" && in.Tool != "apply_patch" {
		return invalid("child operation must be returned to coordinator")
	}
	if in.Tool == "apply_patch" && s.protectedPatch(in.Input.Command) {
		return invalid("child cannot patch shared state")
	}
	if in.Tool == "Bash" {
		argv, ok := shellWords(in.Input.Command)
		if ok && len(argv) > 0 && sameBinary(argv[0], s.Binary) {
			if _, help := cli.Help(argv[1:]); help {
				return nil
			}
			if len(argv) == 2 && argv[1] == "version" {
				return nil
			}
			r, err := cli.ParseMinimal(argv[1:])
			if err != nil {
				return invalid("unclassified product command must be returned to coordinator")
			}
			switch r.Command + "/" + r.Action {
			case "assignment/list", "assignment/show", "assignment/check", "memory/rules", "memory/search", "memory/show", "memory/check", "intent/list", "intent/show", "intent/procedure", "intent/history", "intent/check", "session/inspect":
				return nil
			case "intent/plan", "intent/documents":
				if r.File == "" {
					return nil
				}
			}
			return invalid("child product mutations must be returned to coordinator")
		}
	}
	return nil
}
