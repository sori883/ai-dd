package minimal

import (
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/assignment"
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
	if in.Tool == "send_message" || in.Tool == "collaborationsend_message" {
		return s.childReport(in, parent, st, store)
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

func (s Service) childReport(in HookInput, parent Session, st flow.State, store flow.Store) error {
	if !regexp.MustCompile(`^/root(/[a-z][a-z0-9_]*)*$`).MatchString(in.Input.Target) || len(in.Input.Target) > 512 {
		return invalid("child report requires an absolute canonical parent target")
	}
	clock := s.hookTiming()
	deadline := clock.now().Add(2 * time.Second)
	epoch := ""
	for {
		current, err := s.Inspect(in.Session)
		if err != nil {
			return err
		}
		if current.Space != parent.Space || current.Intent != parent.Intent {
			return invalid("child report parent binding changed")
		}
		currentStage, err := store.AssignmentStage(st.ID, st.CurrentStepID, in.AgentType)
		if err != nil {
			return err
		}
		if currentStage.DefinitionHash != st.DefinitionHash {
			return invalid("child report definition changed")
		}
		r, err := (assignment.Store{Root: s.Root}).Read()
		if err != nil {
			return err
		}
		if epoch != "" && r.Epoch != epoch {
			return invalid("child report registry reset during wait")
		}
		epoch = r.Epoch
		parentPath, sameRole, pending, err := reportParent(r, in, parent, st, store)
		if err != nil {
			return err
		}
		if sameRole && parentPath != "" && in.Input.Target == parentPath {
			return nil
		}
		if !pending || parentPath != "" && in.Input.Target != parentPath {
			return invalid("child report requires the exact registered parent task path")
		}
		remaining := deadline.Sub(clock.now())
		if remaining <= 0 {
			return invalid("parent dispatch is pending; retry this child's report after parent binding completes; do not respawn or release")
		}
		clock.wait(min(20*time.Millisecond, remaining))
	}
}

func reportParent(r assignment.Registry, in HookInput, parent Session, st flow.State, store flow.Store) (string, bool, bool, error) {
	parentPath := ""
	sameRole := false
	pending := false
	for _, d := range r.Dispatches {
		if d.Session != in.Session || d.Space != parent.Space || d.IntentID != parent.Intent || d.StepID != st.CurrentStepID || d.DefinitionHash != st.DefinitionHash {
			continue
		}
		eligible, err := store.AssignmentStage(st.ID, st.CurrentStepID, d.Agent)
		if err != nil {
			continue
		}
		if eligible.CurrentStepID != st.CurrentStepID || eligible.DefinitionHash != st.DefinitionHash {
			return "", false, false, invalid("child report stage changed")
		}
		if d.Agent == "aidlc-worker" {
			active, err := reportWorkerActive(r, d)
			if err != nil {
				return "", false, false, err
			}
			if !active {
				continue
			}
		}
		if d.Status == "uncertain" {
			return "", false, false, invalid("child report dispatch is uncertain")
		}
		if d.Status == "dispatch_pending" {
			if d.Canonical != "" {
				return "", false, false, invalid("pending child report has an unexpected canonical path")
			}
			pending = pending || d.Agent == in.AgentType
			continue
		}
		if !regexp.MustCompile(`^/root(/[a-z][a-z0-9_]*)+$`).MatchString(d.Canonical) || path.Clean(d.Canonical) != d.Canonical || !strings.HasSuffix(d.Canonical, "/"+d.TaskName) || len(d.Canonical) > 512 {
			return "", false, false, invalid("child report has invalid canonical task path")
		}
		candidate := path.Dir(d.Canonical)
		if parentPath != "" && parentPath != candidate {
			return "", false, false, invalid("child report parent is ambiguous")
		}
		parentPath = candidate
		if d.Agent == in.AgentType {
			sameRole = true
		}
	}
	return parentPath, sameRole, pending, nil
}

func reportWorkerActive(r assignment.Registry, d assignment.Dispatch) (bool, error) {
	for _, v := range r.Reservations {
		if v.ID != d.AssignmentID {
			continue
		}
		if v.Status == "released" {
			return false, nil
		}
		if v.CoordinatorSession != d.Session || v.Space != d.Space || v.IntentID != d.IntentID || v.StepID != d.StepID || v.DefinitionHash != d.DefinitionHash || v.Agent != d.Agent || v.TaskName != d.TaskName {
			return false, invalid("child report worker reservation context mismatch")
		}
		return true, nil
	}
	return false, invalid("child report worker reservation required")
}
