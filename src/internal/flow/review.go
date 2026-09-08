package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"strings"
)

type ReviewRequest struct {
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
		gate, err := s.checkState(*st)
		if err != nil {
			return err
		}
		name := "aidlc/.runtime/flow/reviews/" + s.Space + "-" + id + ".json"
		switch request.Action {
		case "assign":
			if request.CoordinatorSession == "" || request.CoordinatorSession == request.Session {
				return invalid("reviewer must be independent")
			}
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
			st.Review = Gate{Target: gate.Target, Status: "pending"}
		case "accept":
			raw, err := filestore.ReadFile(s.Root, name)
			if err != nil {
				return err
			}
			var assignment ReviewRequest
			if err := json.Unmarshal(raw, &assignment); err != nil {
				return err
			}
			if st.Review.Status != "pending" || assignment.Session != request.Session || assignment.Root != root || assignment.Target != request.Target || gate.Target != request.Target {
				return invalid("unassigned or stale review")
			}
			if request.Status != "pass" && request.Status != "fail" || strings.TrimSpace(request.Summary) == "" {
				return invalid("review pass/fail and summary required")
			}
			st.Review = Gate{Target: gate.Target, Status: request.Status, Summary: request.Summary}
		default:
			return invalid("unknown review action")
		}
		return nil
	})
}
func (s Store) change(id string, expect uint64, fn func(*State) error) (State, error) {
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
	if st.Revision != expect || expect == ^uint64(0) {
		return State{}, invalid("revision conflict")
	}
	if err := fn(&st); err != nil {
		return State{}, err
	}
	st.Revision++
	if err := s.persist(st); err != nil {
		return State{}, err
	}
	return st, nil
}
