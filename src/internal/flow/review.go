package flow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ReviewRequest struct {
	StepID             string `json:"step_id"`
	Action             string `json:"action"`
	CoordinatorSession string `json:"coordinator_session"`
	Session            string `json:"session"`
	Root               string `json:"root"`
	Target             string `json:"target"`
	Status             string `json:"status"`
	Summary            string `json:"summary"`
}

// Review assigns an independent runtime reviewer or accepts its current result.
func (s Store) Review(id string, expect uint64, request ReviewRequest) (State, error) {
	return s.change(id, expect, func(st *State) error {
		if st.Status != "active" {
			return invalid("Intent is not active")
		}
		root, err := filepath.EvalSymlinks(request.Root)
		if err != nil {
			return err
		}
		project, err := filepath.EvalSymlinks(s.Root)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(root) || root == project || strings.TrimSpace(request.Session) == "" {
			return invalid("independent reviewer root and session required")
		}
		info, err := os.Stat(root)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return invalid("reviewer root is not directory")
		}
		if st.Stage != "initialization" {
			expectedCode, err := reviewCode(s.Root)
			if err != nil {
				return err
			}
			actualCode, err := reviewCode(root)
			if err != nil {
				return err
			}
			if expectedCode != actualCode {
				return invalid("reviewer checkout code version or bytes differ")
			}

		}
		gate, err := s.checkState(*st)
		if err != nil {
			return err
		}
		name := "aidlc/.runtime/flow/reviews/" + s.Space + "-" + id + ".json"
		switch request.Action {
		case "assign":
			if gate.Status != "pass" {
				return invalid("end Sensor failed: " + gate.Summary)
			}
			if request.CoordinatorSession == "" || request.CoordinatorSession == request.Session {
				return invalid("reviewer must be independent")
			}
			request.StepID = st.CurrentStepID
			request.Target = gate.Target
			request.Root = root
			request.Status = ""
			request.Summary = ""
			raw, err := json.Marshal(request)
			if err != nil {
				return err
			}
			if err := filestore.WriteFile(s.Root, name, raw); err != nil {
				return err
			}
			st.Review = Gate{StepID: st.CurrentStepID, Target: gate.Target, Status: "pending"}
		case "accept":
			raw, err := filestore.ReadFile(s.Root, name)
			if err != nil {
				return err
			}
			var assignment ReviewRequest
			if err := json.Unmarshal(raw, &assignment); err != nil {
				return err
			}
			if assignment.StepID != st.CurrentStepID || st.Review.StepID != st.CurrentStepID || st.Review.Status != "pending" || assignment.Session != request.Session || assignment.Root != root || assignment.Target != request.Target || gate.Target != request.Target {
				return invalid("unassigned or stale review")
			}
			if request.Status != "pass" && request.Status != "fail" || strings.TrimSpace(request.Summary) == "" {
				return invalid("review pass/fail and summary required")
			}
			st.Review = Gate{StepID: st.CurrentStepID, Target: gate.Target, Status: request.Status, Summary: request.Summary}
			if request.Status == "pass" {
				revision, hash := evidencePlan(*st)
				st.Approval, err = newApproval(*st, gate.Target, revision, hash)
				if err != nil {
					return err
				}
				currentExecution(st).Status = "awaiting_approval"
			}
		default:
			return invalid("unknown review action")
		}
		return nil
	})
}
func (s Store) change(id string, expect uint64, fn func(*State) error) (State, error) {
	return s.changeReassignment(id, expect, nil, fn)
}
func (s Store) changeReassignment(id string, expect uint64, request *UnitRequest, fn func(*State) error) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return State{}, err
	}
	defer release()
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if request != nil && s.completedUnitRetry(st, expect, *request) {
		return st, nil
	}
	if st.Revision != expect || expect == ^uint64(0) {
		return State{}, invalid("revision conflict")
	}
	if err := s.guardReassignment(st, request); err != nil {
		return State{}, err
	}
	if err := fn(&st); err != nil {
		return State{}, err
	}
	st.Revision++
	if err := s.commit(&st); err != nil {
		return State{}, err
	}
	return st, nil
}

// reviewCode identifies code bytes independently of shared Space documents.
func reviewCode(root string) (string, error) {
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	top, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if top != canonical {
		return "", invalid("review root must be a Git worktree root")
	}
	files, err := git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	names := strings.Split(files, "\x00")
	sort.Strings(names)
	h := sha256.New()
	h.Write([]byte(head))
	for _, name := range names {
		if name == "" || strings.HasPrefix(name, "aidlc/") {
			continue
		}
		raw, err := filestore.ReadFile(root, name)
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(raw)
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
