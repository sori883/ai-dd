package flow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/filestore"
)

type AssignmentRequest struct {
	RegistryEpoch string `json:"registry_epoch"`
	RequestID     string `json:"request_id"`
	StepID        string `json:"step_id"`
	Agent         string `json:"agent"`
	Root          string `json:"root"`
	Session       string `json:"session"`
}

func (s Store) ReserveAssignment(id string, expect uint64, session string, req AssignmentRequest) (assignment.Reservation, error) {
	if err := s.check(); err != nil {
		return assignment.Reservation{}, err
	}
	unlock, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return assignment.Reservation{}, err
	}
	defer unlock()
	st, err := s.Read(id)
	if err != nil {
		return assignment.Reservation{}, err
	}
	if err := s.guardUnitReservation(st, nil); err != nil {
		return assignment.Reservation{}, err
	}
	if st.Revision != expect || st.CurrentStepID != req.StepID {
		return assignment.Reservation{}, invalid("revision or step mismatch")
	}
	if _, err := s.AssignmentStage(st.ID, req.StepID, req.Agent); err != nil {
		return assignment.Reservation{}, err
	}
	return (assignment.Store{Root: s.Root}).Reserve(assignment.ReserveRequest{RegistryEpoch: req.RegistryEpoch, RequestID: req.RequestID, CoordinatorSession: session, Session: req.Session, Space: s.Space, IntentID: id, StepID: req.StepID, DefinitionHash: st.DefinitionHash, Root: req.Root, Agent: req.Agent, SourceRevision: expect})
}

func unitRequestHash(r UnitRequest) (string, error) {
	root, err := filepath.EvalSymlinks(r.Root)
	if err != nil {
		return "", err
	}
	r.Root = root
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return filestore.Hash(raw), nil
}
func (s Store) reserveUnit(st State, expect uint64, r UnitRequest, previous ...assignment.Reservation) (assignment.Reservation, error) {
	hash, err := unitRequestHash(r)
	if err != nil {
		return assignment.Reservation{}, err
	}
	req := assignment.ReserveRequest{RegistryEpoch: r.RegistryEpoch, RequestID: r.RequestID, CoordinatorSession: r.CoordinatorSession, Session: r.Session, Space: s.Space, IntentID: st.ID, StepID: r.StepID, DefinitionHash: st.DefinitionHash, Root: r.Root, Agent: "aidlc-worker", Unit: r.Unit, SourceRevision: expect, PayloadHash: hash}
	registry := assignment.Store{Root: s.Root}
	if len(previous) != 0 {
		old := previous[0]
		return registry.Replace(old.ID, old.EntryRevision, assignment.ReleaseRequest{RegistryEpoch: r.RegistryEpoch, RequestID: "reassign-" + filestore.Hash([]byte(r.RequestID)), PreviousRunStopped: r.PreviousRunStopped, NoMoreRequests: r.PreviousRunStopped, Reason: r.Reason}, req)
	}
	return registry.Reserve(req)
}

// guardUnitReservation keeps partially saved claims exclusive until identical recovery.
func (s Store) guardUnitReservation(st State, request *UnitRequest) error {
	reg, err := (assignment.Store{Root: s.Root}).Read()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, v := range reg.Reservations {
		if v.Space != s.Space || v.IntentID != st.ID || v.Unit == "" || v.Status == "released" || v.SourceRevision != st.Revision {
			continue
		}
		if request == nil || request.RequestID != v.RequestID {
			return invalid("incomplete Unit assignment; retry original request")
		}
		hash, err := unitRequestHash(*request)
		if err != nil {
			return err
		}
		if hash != v.PayloadHash {
			return invalid("incomplete Unit assignment request differs")
		}
	}
	return nil
}
func (s Store) completedUnitRetry(st State, expect uint64, r UnitRequest) bool {
	if st.Revision != expect+1 {
		return false
	}
	reg, err := (assignment.Store{Root: s.Root}).Read()
	if err != nil {
		return false
	}
	hash, err := unitRequestHash(r)
	if err != nil {
		return false
	}
	for _, v := range reg.Reservations {
		if v.RegistryEpoch != r.RegistryEpoch || v.RequestID != r.RequestID || v.Space != s.Space || v.IntentID != st.ID || v.SourceRevision != expect || v.PayloadHash != hash || v.Status == "released" {
			continue
		}
		runtime, err := s.assignment(st.ID, r.Unit)
		if err != nil || runtime.RunID != v.RunID {
			return false
		}
		for _, u := range st.Config.Units {
			if u.ID == r.Unit && u.Status == "running" {
				return true
			}
		}
	}
	return false
}

func (s Store) reassignReservation(st State, expect uint64, req UnitRequest) (assignment.Reservation, error) {
	registry := assignment.Store{Root: s.Root}
	reg, err := registry.Read()
	if err != nil {
		return assignment.Reservation{}, err
	}
	for _, v := range reg.Reservations {
		if v.RequestID == req.RequestID {
			return s.reserveUnit(st, expect, req)
		}
	}
	old, err := s.assignment(st.ID, req.Unit)
	if err != nil {
		return assignment.Reservation{}, invalid("existing managed Unit reservation required")
	}
	for _, v := range reg.Reservations {
		if v.Space != s.Space || v.IntentID != st.ID || v.Unit != req.Unit || v.RunID != old.RunID {
			continue
		}
		if v.Status != "released" {
			return s.reserveUnit(st, expect, req, v)
		}
		return s.reserveUnit(st, expect, req)
	}
	return assignment.Reservation{}, invalid("legacy Unit has no managed reservation; start a new Intent after human confirmation")
}

func (s Store) AssignmentStage(id, step, agent string) (State, error) {
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if st.Status != "active" || st.CurrentStepID != step {
		return State{}, invalid("inactive Intent or different step")
	}
	view, err := s.Procedure(id)
	if err != nil {
		return State{}, err
	}
	allowed := false
	for _, role := range view.Procedure.Agents {
		if role.Agent == agent {
			allowed = true
		}
	}
	if !allowed {
		return State{}, invalid("agent is not allowed in current stage")
	}
	if agent == "aidlc-worker" {
		if err := s.checkWorkState(st); err != nil {
			return State{}, err
		}
	}
	return st, nil
}

// CheckAssignment validates managed eligibility, not the actual child process root.
func (s Store) CheckAssignment(v assignment.Reservation) error {
	if v.Status == "released" || v.Space != s.Space {
		return invalid("assignment released or different Space")
	}
	st, err := s.AssignmentStage(v.IntentID, v.StepID, v.Agent)
	if err != nil {
		return err
	}
	if st.DefinitionHash != v.DefinitionHash {
		return invalid("assignment definition changed")
	}
	if v.Unit != "" {
		runtime, err := s.assignment(st.ID, v.Unit)
		if err != nil {
			return err
		}
		if runtime.RunID != v.RunID || runtime.Root != v.Root || runtime.Session != v.Session {
			return invalid("unit runtime does not match reservation")
		}
		for _, u := range st.Config.Units {
			if u.ID == v.Unit && u.Status == "running" && st.Revision > v.SourceRevision {
				return nil
			}
		}
		return invalid("Unit progress is incomplete or no longer running")
	}
	return nil
}

func (s Store) managedUnit(st State, runtime UnitRequest) error {
	reg, err := (assignment.Store{Root: s.Root}).Read()
	if err != nil {
		return err
	}
	for _, v := range reg.Reservations {
		if v.RegistryEpoch == runtime.RegistryEpoch && v.Space == s.Space && v.IntentID == st.ID && v.Unit == runtime.Unit && v.RunID == runtime.RunID && v.Root == runtime.Root && v.Session == runtime.Session && v.Status != "released" {
			return nil
		}
	}
	return invalid("Unit run has no active managed reservation")
}
