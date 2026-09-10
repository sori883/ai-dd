package assignment

import (
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"path"
	"regexp"
	"strings"
)

type DispatchRequest struct {
	Session        string `json:"session"`
	Turn           string `json:"turn"`
	ToolID         string `json:"tool_use_id"`
	TaskName       string `json:"task_name"`
	Agent          string `json:"agent"`
	Space          string `json:"space"`
	IntentID       string `json:"intent_id"`
	StepID         string `json:"step_id"`
	DefinitionHash string `json:"definition_hash"`
}
type Dispatch struct {
	DispatchRequest
	AssignmentID string `json:"assignment_id,omitempty"`
	Canonical    string `json:"canonical_task_path,omitempty"`
	Status       string `json:"status"`
}

func (s Store) PreSpawn(req DispatchRequest) (Dispatch, error) {
	if !validText(req.Session, 160) || !validText(req.Turn, 160) || !validText(req.ToolID, 160) || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,127}$`).MatchString(req.TaskName) || !validText(req.Agent, 160) || !validText(req.Space, 160) || len(req.IntentID) != 32 || !validText(req.StepID, 160) || len(req.DefinitionHash) != 64 {
		return Dispatch{}, errors.New("invalid dispatch request")
	}
	root, err := s.canonicalRoot()
	if err != nil {
		return Dispatch{}, err
	}
	unlock, err := filestore.Lock(root, "assignments")
	if err != nil {
		return Dispatch{}, err
	}
	defer unlock()
	r, err := s.Read()
	if err != nil {
		return Dispatch{}, err
	}
	for _, d := range r.Dispatches {
		if d.Session != req.Session {
			continue
		}
		if d.ToolID == req.ToolID {
			if d.DispatchRequest != req {
				return Dispatch{}, errors.New("dispatch tool id content mismatch")
			}
			if d.AssignmentID != "" && !activeAssignment(r, d.AssignmentID) {
				return Dispatch{}, errors.New("assignment released")
			}
			return d, nil
		}
		if d.TaskName == req.TaskName {
			return Dispatch{}, errors.New("task name already used in this parent session")
		}
	}
	d := Dispatch{DispatchRequest: req, Status: "dispatch_pending"}
	if req.Agent == "aidlc-worker" {
		for i := range r.Reservations {
			v := &r.Reservations[i]
			if v.TaskName != req.TaskName {
				continue
			}
			if v.Status != "reserved" || v.CoordinatorSession != req.Session || v.Space != req.Space || v.IntentID != req.IntentID || v.StepID != req.StepID || v.DefinitionHash != req.DefinitionHash {
				return Dispatch{}, errors.New("worker reservation is not eligible")
			}
			d.AssignmentID = v.ID
			v.Status = "dispatch_pending"
			v.EntryRevision++
		}
		if d.AssignmentID == "" {
			return Dispatch{}, errors.New("registered worker task required")
		}
	}
	r.Dispatches = append(r.Dispatches, d)
	r.Revision++
	if err := admission(r); err != nil {
		return Dispatch{}, err
	}
	if err := s.persist(r); err != nil {
		return Dispatch{}, err
	}
	return d, nil
}
func activeAssignment(r Registry, id string) bool {
	for _, v := range r.Reservations {
		if v.ID == id {
			return v.Status != "released"
		}
	}
	return false
}
func (s Store) PostSpawn(session, tool string, raw []byte) (Dispatch, error) {
	root, err := s.canonicalRoot()
	if err != nil {
		return Dispatch{}, err
	}
	unlock, err := filestore.Lock(root, "assignments")
	if err != nil {
		return Dispatch{}, err
	}
	defer unlock()
	r, err := s.Read()
	if err != nil {
		return Dispatch{}, err
	}
	for i := range r.Dispatches {
		d := &r.Dispatches[i]
		if d.Session != session || d.ToolID != tool {
			continue
		}
		if d.AssignmentID != "" && !activeAssignment(r, d.AssignmentID) {
			return Dispatch{}, errors.New("late post for released assignment")
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			raw = []byte(text)
		}
		var response struct {
			TaskName string `json:"task_name"`
		}
		responseErr := decode(raw, &response)
		if responseErr == nil && (!regexp.MustCompile(`^/root(/[a-z][a-z0-9_]*)+$`).MatchString(response.TaskName) || path.Clean(response.TaskName) != response.TaskName || !strings.HasSuffix(response.TaskName, "/"+d.TaskName) || len(response.TaskName) > 512) {
			responseErr = errors.New("unrecognized task path")
		}
		if responseErr == nil {
			for j, other := range r.Dispatches {
				if j != i && other.Session == session && other.Canonical == response.TaskName {
					responseErr = errors.New("ambiguous task path")
				}
			}
		}
		if d.Status == "bound" {
			if responseErr == nil && d.Canonical == response.TaskName {
				return *d, nil
			}
			return Dispatch{}, errors.New("conflicting duplicate post")
		}
		d.Status = "uncertain"
		if responseErr == nil {
			d.Status = "bound"
			d.Canonical = response.TaskName
		}
		for j := range r.Reservations {
			v := &r.Reservations[j]
			if v.ID == d.AssignmentID {
				v.Status = d.Status
				v.EntryRevision++
			}
		}
		r.Revision++
		if err := s.persist(r); err != nil {
			return Dispatch{}, err
		}
		if responseErr != nil {
			return *d, errors.New("spawn response uncertain; retain reservation and inspect")
		}
		return *d, nil
	}
	return Dispatch{}, errors.New("unmatched spawn post")
}
func (s Store) CheckTarget(session, target, space, intent, step, definition string) (Dispatch, error) {
	r, err := s.Read()
	if err != nil {
		return Dispatch{}, err
	}
	for _, d := range r.Dispatches {
		if d.Session != session || (d.TaskName != target && d.Canonical != target) {
			continue
		}
		if d.Status != "bound" || d.Space != space || d.IntentID != intent || d.StepID != step || d.DefinitionHash != definition {
			return Dispatch{}, errors.New("target is uncertain or belongs to a different step")
		}
		if d.AssignmentID != "" && !activeAssignment(r, d.AssignmentID) {
			return Dispatch{}, errors.New("target assignment released")
		}
		return d, nil
	}
	return Dispatch{}, errors.New("unregistered target")
}
