package minimal

import (
	"bytes"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"io"
	"reflect"
	"strconv"
)

func (s Service) executeFlow(r cli.MinimalRequest) ([]byte, error) {
	store := flow.Store{Root: s.Root, Space: r.Space}
	if r.Action == "create" {
		st, err := store.Create(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(st)
	}
	if r.Action == "list" {
		st, err := store.List()
		if err != nil {
			return nil, err
		}
		return encode(st)
	}
	if r.Action == "procedure" {
		view, err := store.Procedure(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(view)
	}
	if r.Action == "documents" && r.File == "" {
		st, err := store.Read(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(flow.IntentDocuments{Inputs: append([]flow.DocumentDeclaration{}, st.Config.DocumentInputs...), Outputs: append([]flow.DocumentDeclaration{}, st.Config.DocumentOutputs...)})
	}
	if r.Action == "history" {
		records, err := store.History(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(records)
	}
	if r.Action == "plan" && r.File == "" {
		st, err := store.Read(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(st.ExecutionPlan)
	}
	if r.Action == "show" {
		st, err := store.Read(r.Target)
		if err != nil {
			return nil, err
		}
		return encode(st)
	}
	if r.Action == "check" {
		boundary := flow.Boundary(r.Boundary)
		if boundary == "" {
			boundary = flow.BoundaryEnd
		}
		gate, err := store.CheckBoundary(r.Target, boundary)
		out, _ := encode(gate)
		return out, err
	}
	if r.Action == "switch" {
		id := r.Target
		if r.IntentID != nil {
			id = *r.IntentID
		} else {
			var err error
			id, err = store.Resolve(r.Target)
			if err != nil {
				return nil, err
			}
		}
		return s.bindFlow(r.Session, r.Space, id, r.Recover)
	}
	expect, err := strconv.ParseUint(r.Expect, 10, 64)
	if err != nil || expect == 0 {
		return nil, invalid("positive --expect revision required")
	}
	var result flow.State
	switch {
	case r.Action == "plan":
		var request flow.PlanRequest
		if err := s.decodeDraft(r.File, &request); err != nil {
			return nil, err
		}
		result, err = store.ProposePlan(r.Target, expect, request)
	case r.Action == "approval" || r.Action == "plan-approval":
		var request flow.ApprovalDecision
		if err := s.decodeDraft(r.File, &request); err != nil {
			return nil, err
		}
		if r.Action == "approval" {
			result, err = store.Decide(r.Target, expect, request)
		} else {
			result, err = store.DecidePlan(r.Target, expect, request)
		}
	case r.Action == "finish":
		result, err = store.Finish(r.Target, expect)
	case r.Action == "reopen":
		result, err = store.Reopen(r.Target, expect, r.Step, r.Reason)
	case r.Action == "documents":
		var documents flow.IntentDocuments
		if err := s.decodeDraft(r.File, &documents); err != nil {
			return nil, err
		}
		result, err = store.SetDocuments(r.Target, expect, documents)
	case r.Action == "begin":
		result, err = store.Begin(r.Target, expect)
	case r.Command == "unit":
		var request flow.UnitRequest
		if err := s.decodeDraft(r.File, &request); err != nil {
			return nil, err
		}
		request.Action = r.Action
		result, err = store.Unit(r.Target, expect, request)
	case r.Action == "configure":
		var fields map[string]json.RawMessage
		if err := s.decodeDraft(r.File, &fields); err != nil {
			return nil, err
		}
		for _, key := range []string{"document_inputs", "document_outputs"} {
			if _, ok := fields[key]; ok {
				return nil, invalid("document lists are managed by intent documents")
			}
		}
		var config flow.Config
		if err := s.decodeDraft(r.File, &config); err != nil {
			return nil, err
		}
		result, err = store.Read(r.Target)
		if err == nil {
			previous := map[string]flow.Unit{}
			for _, old := range result.Config.Units {
				previous[old.ID] = old
			}
			for i, next := range config.Units {
				old, exists := previous[next.ID]
				if !exists {
					if next.Status != "" && next.Status != "pending" || next.ResultCommit != "" || next.IntegratedCommit != "" {
						return nil, invalid("new Unit must be pending with empty results")
					}
					config.Units[i].Status = "pending"
					continue
				}
				if next.Status != old.Status || next.ResultCommit != old.ResultCommit || next.IntegratedCommit != old.IntegratedCommit {
					return nil, invalid("Unit progress is managed by Unit operations")
				}
				if (old.Status == "running" || old.Status == "needs_confirmation" || old.Status == "reported") && !reflect.DeepEqual(old, next) {
					return nil, invalid("configure cannot replace an active Unit assignment")
				}
				delete(previous, next.ID)
			}
			for _, old := range previous {
				if old.Status != "pending" && old.Status != "integrated" {
					return nil, invalid("configure cannot remove an uncollected Unit")
				}
			}
			config.DocumentInputs = result.Config.DocumentInputs
			config.DocumentOutputs = result.Config.DocumentOutputs
			result.Config = config
			result, err = store.Save(result, expect)
		}
	case r.Action == "review":
		var request flow.ReviewRequest
		if err := s.decodeDraft(r.File, &request); err != nil {
			return nil, err
		}
		result, err = store.Review(r.Target, expect, request)
	default:
		result, err = store.Transition(r.Target, expect, flow.TransitionRequest{Action: r.Action, Reason: r.Reason, Stage: r.Stage, ResumeCondition: r.ResumeCondition})
	}
	if err != nil {
		return nil, err
	}
	return encode(result)
}
func (s Service) decodeDraft(file string, value any) error {
	raw, err := s.readDraft(file)
	if err != nil {
		return err
	}
	if err := uniqueDraftJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return invalid(err.Error())
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return invalid("trailing JSON")
	}
	return nil
}
func (s Service) bindFlow(session, space, id string, recover bool) ([]byte, error) {
	return s.withSession(session, func(state *Session) ([]byte, error) {
		if state.Tool != "" && !recover {
			return nil, invalid("tool is running; poll or explicitly recover after checking it ended")
		}
		if recover && (state.Intent != id || state.Space != space || state.Intent == "") {
			return nil, invalid("recovery must match current Intent and Space")
		}
		st, err := (flow.Store{Root: s.Root, Space: space}).Read(id)
		if err != nil {
			return nil, err
		}
		rules, hash, err := s.rules(space)
		if err != nil {
			return nil, err
		}
		state.Intent = id
		state.Space = space
		state.RuleHash = hash
		state.RuleTurn = state.Turn
		if recover {
			state.Tool = ""
		}
		if err := s.save(session, *state); err != nil {
			return nil, err
		}
		return encode(map[string]any{"state": st, "rules": rules, "rules_hash": hash, "draft": s.draftPath(session)})
	})
}

func uniqueDraftJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return invalid("duplicate JSON key")
			}
			seen[name] = true
			if err = uniqueDraftJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err = uniqueDraftJSON(d); err != nil {
				return err
			}
		}
	default:
		return invalid("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
