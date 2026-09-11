package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"path/filepath"
	"strings"
)

type UnitRequest struct {
	VerificationSHA256 string `json:"verification_sha256"`
	RegistryEpoch      string `json:"registry_epoch"`
	RequestID          string `json:"request_id"`
	CoordinatorSession string `json:"coordinator_session"`
	StepID             string `json:"step_id"`
	Reason             string `json:"reason"`
	PreviousRunStopped bool   `json:"previous_run_stopped"`
	Action             string `json:"action"`
	Unit               string `json:"unit"`
	Session            string `json:"session"`
	Root               string `json:"root"`
	RunID              string `json:"run_id"`
}

func (s Store) assignmentPath(id, unit string) string {
	return "aidlc/.runtime/flow/units/" + s.Space + "/" + id + "/" + unit + ".json"
}
func (s Store) assignment(id, unit string) (UnitRequest, error) {
	raw, err := filestore.ReadFile(s.Root, s.assignmentPath(id, unit))
	if err != nil {
		return UnitRequest{}, err
	}
	var r UnitRequest
	err = json.Unmarshal(raw, &r)
	return r, err
}

// Unit records coordinator acceptance of a separate worker's current run.
func (s Store) Unit(id string, expect uint64, r UnitRequest) (State, error) {
	var retry *UnitRequest
	if r.Action == "reassign" || r.Action == "claim" {
		retry = &r
	}
	return s.changeReassignment(id, expect, retry, func(st *State) error {
		if r.StepID != st.CurrentStepID {
			return invalid("Unit execution mismatch")
		}
		if st.Status != "active" || st.Stage != "tdd" {
			return invalid("Unit operations require active TDD stage")
		}
		index := -1
		for i, unit := range st.Config.Units {
			if unit.ID == r.Unit {
				index = i
			}
		}
		if index < 0 {
			return invalid("unknown Unit")
		}
		unit := &st.Config.Units[index]
		switch r.Action {
		case "reassign":
			return s.reassign(st, unit, expect, r)
		case "claim":
			if err := s.checkWorkState(*st); err != nil {
				return err
			}
			if unit.Status != "pending" {
				return invalid("Unit already assigned or finished")
			}
			if len(unitPlanProblems(st.Config.Units)) > 0 {
				return invalid("invalid Unit plan")
			}
			for _, dep := range unit.DependsOn {
				for _, candidate := range st.Config.Units {
					if candidate.ID == dep {
						if candidate.Status != "integrated" || !validHash(candidate.ResultSHA256) {
							return invalid("dependency not integrated")
						}

					}
				}
			}
			root, err := filepath.EvalSymlinks(r.Root)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(root) || strings.TrimSpace(r.Session) == "" {
				return invalid("worker root and session required")
			}
			for _, other := range st.Config.Units {
				if other.Status != "running" && other.Status != "needs_confirmation" {
					continue
				}
				a, err := s.assignment(id, other.ID)
				if err != nil {
					return err
				}
				if a.Root == root || a.Session == r.Session {
					return invalid("worker root or session already assigned")
				}
				for _, left := range unit.Scope {
					for _, right := range other.Scope {
						if left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/") {
							return invalid("concurrent Unit scopes overlap")
						}
					}
				}
			}
			reservation, err := s.reserveUnit(*st, expect, r)
			if err != nil {
				return err
			}
			if reservation.Status == "released" {
				return invalid("released claim cannot be resumed")
			}
			r.RunID = reservation.RunID
			r.Root = root
			raw, err := json.Marshal(r)
			if err != nil {
				return err
			}
			if err := filestore.WriteFile(s.Root, s.assignmentPath(id, r.Unit), raw); err != nil {
				return err
			}
			unit.Status = "running"
		case "confirm", "result":
			expected := "running"
			if r.Action == "confirm" {
				expected = "needs_confirmation"
			}
			if unit.Status != expected && !(r.Action == "result" && unit.Status == "reported") {
				return invalid("Unit state requires explicit confirmation or running assignment")
			}
			a, err := s.assignment(id, r.Unit)
			if err != nil {
				return err
			}
			if err := s.managedUnit(*st, a); err != nil {
				return err
			}
			root, err := filepath.EvalSymlinks(r.Root)
			if err != nil {
				return err
			}
			if a.StepID != st.CurrentStepID || a.Session != r.Session || a.Root != root || a.RunID != r.RunID {
				return invalid("Unit run identity mismatch")
			}
			paths := unitVerificationPaths(st.Config, *unit)
			digest, err := ComputeVerification(root, paths)
			if err != nil {
				return err
			}
			if len(paths) == 0 || !validHash(r.VerificationSHA256) || r.VerificationSHA256 != digest.SHA256 {
				return invalid("worker verification SHA must match current content")
			}
			if r.Action == "confirm" {
				unit.Status = "running"
				return nil
			}
			c := boundaryCollector{store: Store{Root: root, Space: s.Space}}
			required := map[resultRequirement]bool{}
			for _, command := range unit.Tests {
				required[resultRequirement{unit: unit.ID, command: command}] = true
			}
			c.verificationResults(*st, digest.SHA256, unit.ID, r.RunID, required)
			if len(c.failures) > 0 {
				return invalid(strings.Join(c.failures, "; "))
			}
			unit.Status = "reported"
			unit.ResultSHA256 = digest.SHA256
		case "integrate":
			if unit.Status != "reported" {
				return invalid("Unit has no reported result")
			}
			digest, err := ComputeVerification(s.Root, unitVerificationPaths(st.Config, *unit))
			if err != nil {
				return err
			}
			if !validHash(unit.ResultSHA256) || digest.SHA256 != unit.ResultSHA256 {
				return invalid("submitted Unit content is not integrated")
			}
			unit.Status = "integrated"
		default:
			return invalid("unknown Unit action")
		}
		return nil
	})
}

func unitVerificationPaths(config Config, unit Unit) []string {
	if len(unit.VerificationPaths) > 0 {
		return unit.VerificationPaths
	}
	return config.VerificationPaths
}
