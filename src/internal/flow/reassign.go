package flow

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	project, err := filepath.EvalSymlinks(s.Root)
	if err != nil {
		return err
	}
	if root == project {
		return invalid("separate worker root required")
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
	if len(r.Commit) != 40 || r.Commit != head {
		return invalid("worker commit must match HEAD")
	}
	if _, err := git(root, "merge-base", "--is-ancestor", unit.BaseCommit, head); err != nil {
		return invalid("reassignment does not descend from base")
	}
	for _, dep := range unit.DependsOn {
		for _, other := range st.Config.Units {
			if other.ID == dep {
				if other.Status != "integrated" || other.IntegratedCommit == "" {
					return invalid("dependency not integrated")
				}
				if _, err := git(root, "merge-base", "--is-ancestor", other.IntegratedCommit, head); err != nil {
					return invalid("worker does not include dependency integration")
				}
			}
		}
	}
	changed, err := gitRaw(root, "diff", "--name-only", "-z", unit.BaseCommit)
	if err != nil {
		return err
	}
	untracked, err := gitRaw(root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return err
	}
	for _, name := range strings.Split(changed+untracked, "\x00") {
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
			return invalid("reassignment changed file outside Unit scope")
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
	if current.ReassignmentRevision == expect {
		requested := r
		requested.RunID = current.RunID
		if current.UnitRequest != requested || current.RunID == "" {
			return invalid("incomplete reassignment differs; inspect current assignment before recovery")
		}
		unit.Status = "running"
		return nil
	}
	random := make([]byte, 16)
	rand.Read(random)
	r.RunID = fmt.Sprintf("%x", random)
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
