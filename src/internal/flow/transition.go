package flow

import "strings"

type TransitionRequest struct{ Action, Reason, ResumeCondition, Stage string }

// Transition changes only current progress, enforcing both current boundary gates.
func (s Store) Transition(id string, expect uint64, r TransitionRequest) (State, error) {
	if r.Action == "reopen" {
		return s.reopen(id, expect, r)
	}
	return s.change(id, expect, func(st *State) error {
		d, err := s.definition()
		if err != nil {
			return err
		}
		if r.Action == "advance" {
			if st.Status != "active" {
				return invalid("only active Intent can advance")
			}
			gate, c, err := s.checkStateSnapshot(*st)
			if err != nil {
				return err
			}
			if gate.Status != "pass" {
				return invalid("Sensor failed: " + gate.Summary)
			}
			if st.Review.Status != "pass" || st.Review.Target != gate.Target {
				return invalid("current independent review pass required")
			}
			if st.Accepted == nil {
				st.Accepted = map[string]StageAcceptance{}
			}
			outputs := c.files
			if st.Stage == "tdd" {
				outputs = c.proof
			}
			st.Accepted[st.Stage] = StageAcceptance{Stage: st.Stage, ReviewTarget: gate.Target, Outputs: outputs}
			st.Entry = nil
			st.Sensor = Gate{}
			st.Review = Gate{}
			if st.Stage == d.Graph.Completion {
				st.Status = "completed"
			} else {
				st.Stage = d.Next(st.Stage)
			}
			return nil
		}
		if strings.TrimSpace(r.Reason) == "" {
			return invalid("reason required")
		}
		switch r.Action {
		case "wait", "pause":
			if st.Status != "active" {
				return invalid("only active Intent may wait or pause")
			}
			if r.Action == "wait" {
				if strings.TrimSpace(r.ResumeCondition) == "" {
					return invalid("resume condition required")
				}
				st.Status = "waiting"
			} else {
				st.Status = "paused"
			}
			st.Reason = r.Reason
			st.ResumeCondition = r.ResumeCondition
			for i := range st.Config.Units {
				if st.Config.Units[i].Status == "running" {
					st.Config.Units[i].Status = "needs_confirmation"
				}
			}
		case "resume":
			if st.Status != "waiting" && st.Status != "paused" {
				return invalid("Intent is not waiting or paused")
			}
			st.Status = "active"
			st.Reason = r.Reason
			st.ResumeCondition = ""
		case "cancel":
			if st.Status == "completed" || st.Status == "cancelled" {
				return invalid("Intent already finished")
			}
			st.Status = "cancelled"
			st.Reason = r.Reason
		default:
			return invalid("unknown transition")
		}
		return nil
	})
}
