package flow

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"path/filepath"
	"strings"
)

type UnitRequest struct {
	Action  string `json:"action"`
	Unit    string `json:"unit"`
	Session string `json:"session"`
	Root    string `json:"root"`
	RunID   string `json:"run_id"`
	Commit  string `json:"commit"`
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
	return s.change(id, expect, func(st *State) error {
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
		case "claim":
			if unit.Status != "pending" {
				return invalid("Unit already assigned or finished")
			}
			if len(unitPlanProblems(st.Config.Units)) > 0 {
				return invalid("invalid Unit plan")
			}
			for _, dep := range unit.DependsOn {
				for _, candidate := range st.Config.Units {
					if candidate.ID == dep {
						if candidate.Status != "integrated" || candidate.IntegratedCommit == "" {
							return invalid("dependency not integrated")
						}
						if _, err := git(s.Root, "merge-base", "--is-ancestor", candidate.IntegratedCommit, unit.BaseCommit); err != nil {
							return invalid("worker base does not include dependency integration")
						}
					}
				}
			}
			root, err := filepath.EvalSymlinks(r.Root)
			if err != nil {
				return err
			}
			project, err := filepath.EvalSymlinks(s.Root)
			if err != nil {
				return err
			}
			if root == project || !filepath.IsAbs(root) || strings.TrimSpace(r.Session) == "" {
				return invalid("separate worker root and session required")
			}
			top, err := git(root, "rev-parse", "--show-toplevel")
			if err != nil {
				return err
			}
			if top != root {
				return invalid("worker root must be worktree root")
			}
			head, err := git(root, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			if head != unit.BaseCommit {
				return invalid("worker must start at Unit base commit")
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
			random := make([]byte, 16)
			rand.Read(random)
			r.RunID = fmt.Sprintf("%x", random)
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
			if unit.Status != expected {
				return invalid("Unit state requires explicit confirmation or running assignment")
			}
			a, err := s.assignment(id, r.Unit)
			if err != nil {
				return err
			}
			root, err := filepath.EvalSymlinks(r.Root)
			if err != nil {
				return err
			}
			if a.Session != r.Session || a.Root != root || a.RunID != r.RunID {
				return invalid("Unit run identity mismatch")
			}
			head, err := git(root, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			if len(r.Commit) != 40 || head != r.Commit {
				return invalid("worker commit must match HEAD")
			}
			if _, err := git(root, "merge-base", "--is-ancestor", unit.BaseCommit, r.Commit); err != nil {
				return invalid("result does not descend from base")
			}
			if r.Action == "confirm" {
				unit.Status = "running"
				return nil
			}
			changed, err := git(root, "diff", "--name-only", unit.BaseCommit, r.Commit)
			if err != nil {
				return err
			}
			for _, name := range strings.Split(changed, "\n") {
				if name == "" {
					continue
				}
				allowed := false
				for _, scope := range unit.Scope {
					if name == scope || strings.HasPrefix(name, scope+"/") {
						allowed = true
					}
				}
				if !allowed {
					return invalid("result changed file outside Unit scope")
				}
			}
			unit.Status = "reported"
			unit.ResultCommit = r.Commit
		case "integrate":
			if unit.Status != "reported" {
				return invalid("Unit has no reported result")
			}
			head, err := git(s.Root, "rev-parse", "HEAD")
			if err != nil {
				return err
			}
			if len(r.Commit) != 40 || r.Commit != head {
				return invalid("integration commit must match coordinator HEAD")
			}
			if _, err := git(s.Root, "merge-base", "--is-ancestor", unit.ResultCommit, r.Commit); err != nil {
				return invalid("result commit is not integrated")
			}
			unit.Status = "integrated"
			unit.IntegratedCommit = r.Commit
			st.Config.CodeRevision = r.Commit
		default:
			return invalid("unknown Unit action")
		}
		return nil
	})
}
