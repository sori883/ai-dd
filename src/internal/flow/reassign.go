package flow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

// unitReassignment is current runtime data, not an operation history.
type unitReassignment struct {
	UnitRequest
	ReassignmentRevision uint64 `json:"reassignment_revision"`
}

func (s Store) reassign(st *State, unit *Unit, expect uint64, r UnitRequest) error {
	if unit.Status != "needs_confirmation" {
		return invalid("reassign requires needs_confirmation; inspect state and current assignment")
	}
	if !r.PreviousRunStopped || strings.TrimSpace(r.Reason) == "" || r.RunID != "" || strings.TrimSpace(r.Session) == "" || !filepath.IsAbs(r.Root) {
		return invalid("reassign requires stopped confirmation, reason, new session/root and no run_id")
	}
	if len(unitPlanProblems(st.Config.Units)) > 0 {
		return invalid("invalid Unit plan")
	}
	root, err := filepath.EvalSymlinks(r.Root)
	if err != nil {
		return err
	}
	for _, dep := range unit.DependsOn {
		for _, other := range st.Config.Units {
			if other.ID == dep {
				if other.Status != "integrated" || !validHash(other.ResultSHA256) {
					return invalid("dependency not integrated")
				}

			}
		}
	}
	for _, other := range st.Config.Units {
		if other.ID == unit.ID || (other.Status != "running" && other.Status != "needs_confirmation") {
			continue
		}
		// Scope remains exclusive even when another paused assignment is not local.
		for _, a := range unit.Scope {
			for _, b := range other.Scope {
				if a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
					return invalid("concurrent Unit scopes overlap")
				}
			}
		}
		assignment, err := s.assignment(st.ID, other.ID)
		if err != nil {
			if other.Status == "needs_confirmation" && os.IsNotExist(err) {
				continue
			}
			return err
		}
		if assignment.Unit != other.ID || assignment.RunID == "" || assignment.Session == "" || !filepath.IsAbs(assignment.Root) {
			return invalid("invalid other Unit assignment")
		}
		if assignment.Root == root || assignment.Session == r.Session {
			return invalid("worker root or session already assigned")
		}
	}
	r.Root = root
	raw, err := filestore.ReadFile(s.Root, s.assignmentPath(st.ID, unit.ID))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var current unitReassignment
	if err == nil {
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.Unit != unit.ID || current.RunID == "" || current.Session == "" || !filepath.IsAbs(current.Root) {
			return invalid("invalid current Unit assignment")
		}
	}
	reservation, err := s.reassignReservation(*st, expect, r)
	if err != nil {
		return err
	}
	if reservation.Status == "released" {
		return invalid("released reassignment cannot resume")
	}
	if current.ReassignmentRevision == expect {
		requested := r
		requested.RunID = current.RunID
		if current.UnitRequest != requested || current.RunID == "" {
			return invalid("incomplete reassignment differs; inspect current assignment before recovery")
		}
		unit.Status = "running"
		return nil
	}
	r.RunID = reservation.RunID
	raw, err = json.Marshal(unitReassignment{UnitRequest: r, ReassignmentRevision: expect})
	if err != nil {
		return err
	}
	if err := filestore.WriteFile(s.Root, s.assignmentPath(st.ID, unit.ID), raw); err != nil {
		return err
	}
	unit.Status = "running"
	return nil
}

// guardReassignment runs under the same Intent lock before any mutation or
// runtime side effect, preserving the revision needed to recover a partial save.
func (s Store) guardReassignment(st State, request *UnitRequest) error {
	if err := s.guardUnitReservation(st, request); err != nil {
		return err
	}
	if err := s.verifyHistoryHead(st); err != nil {
		return err
	}
	if err := s.guardWorkflow(st); err != nil {
		return err
	}
	for _, unit := range st.Config.Units {
		if unit.Status != "needs_confirmation" {
			continue
		}
		raw, err := filestore.ReadFile(s.Root, s.assignmentPath(st.ID, unit.ID))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var current unitReassignment
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current.ReassignmentRevision != st.Revision {
			continue
		}
		failure := invalid("incomplete reassignment; complete the identical reassign request before other state updates")
		if request == nil || request.Unit != unit.ID || request.RunID != "" || current.RunID == "" {
			return failure
		}
		candidate := *request
		root, err := filepath.EvalSymlinks(candidate.Root)
		if err != nil {
			return err
		}
		candidate.Root = root
		candidate.RunID = current.RunID
		if candidate != current.UnitRequest {
			return failure
		}
	}
	return nil
}
