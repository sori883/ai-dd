package app

import (
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func nativeAction(name string) string {
	switch name {
	case "spawn_agent", "collaborationspawn_agent":
		return "spawn"
	case "followup_task", "collaborationfollowup_task", "send_message", "collaborationsend_message":
		return "request"
	case "interrupt_agent", "collaborationinterrupt_agent":
		return "interrupt"
	}
	return ""
}
func (s Service) agentPre(in HookInput, state Session) error {
	if in.ID == "" || in.Turn == "" {
		return invalid("agent tool and turn ids required")
	}
	registry := assignment.Store{Root: s.Root}
	action := nativeAction(in.Tool)
	if action == "interrupt" {
		reg, err := registry.Read()
		if err != nil {
			return err
		}
		for _, d := range reg.Dispatches {
			if d.Session == in.Session && (d.TaskName == in.Input.Target || d.Canonical != "" && d.Canonical == in.Input.Target) {
				return nil
			}
		}
		return invalid("interrupt target is not registered in this parent session")
	}
	if state.Intent == "" || state.Space == "" || state.Turn != in.Turn || state.RuleTurn != in.Turn || state.RuleHash == "" {
		return invalid("selected Intent and current turn Rules required")
	}
	_, hash, err := s.rules(state.Space)
	if err != nil {
		return err
	}
	if hash != state.RuleHash {
		return invalid("Rules changed; reload before agent request")
	}
	unlock, err := filestore.Lock(s.Root, "flow-"+state.Space)
	if err != nil {
		return err
	}
	defer unlock()
	store := flow.Store{Root: s.Root, Space: state.Space}
	st, err := store.Read(state.Intent)
	if err != nil {
		return err
	}
	agent := in.Input.AgentType
	var target assignment.Dispatch
	if action == "request" {
		target, err = registry.CheckTarget(in.Session, in.Input.Target, state.Space, state.Intent, st.CurrentStepID, st.DefinitionHash)
		if err != nil {
			return err
		}
		agent = target.Agent
	}
	if _, err := store.AssignmentStage(st.ID, st.CurrentStepID, agent); err != nil {
		return err
	}
	if agent == "aidlc-worker" {
		reg, err := registry.Read()
		if err != nil {
			return err
		}
		found := false
		for _, v := range reg.Reservations {
			if action == "spawn" && v.TaskName == in.Input.TaskName || action == "request" && v.ID == target.AssignmentID {
				if v.CoordinatorSession != in.Session {
					return invalid("assignment belongs to another parent session")
				}
				if err := store.CheckAssignment(v); err != nil {
					return err
				}
				found = true
			}
		}
		if !found {
			return invalid("worker reservation required")
		}
	}
	if action == "spawn" {
		_, err = registry.PreSpawn(assignment.DispatchRequest{Session: in.Session, Turn: in.Turn, ToolID: in.ID, TaskName: in.Input.TaskName, Agent: agent, Space: state.Space, IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash})
	}
	return err
}
