package assignment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

type ReserveRequest struct {
	PayloadHash        string `json:"payload_hash,omitempty"`
	RegistryEpoch      string `json:"registry_epoch"`
	RequestID          string `json:"request_id"`
	CoordinatorSession string `json:"coordinator_session"`
	Session            string `json:"session"`
	Space              string `json:"space"`
	IntentID           string `json:"intent_id"`
	StepID             string `json:"step_id"`
	DefinitionHash     string `json:"definition_hash"`
	Root               string `json:"root"`
	Agent              string `json:"agent"`
	Unit               string `json:"unit,omitempty"`
	RunID              string `json:"run_id,omitempty"`
	SourceRevision     uint64 `json:"source_revision"`
}
type Reservation struct {
	Release         *ReleaseRequest `json:"release,omitempty"`
	ReleaseRevision uint64          `json:"release_revision,omitempty"`

	ID string `json:"assignment_id"`
	ReserveRequest
	RequestHash   string `json:"request_hash"`
	EntryRevision uint64 `json:"entry_revision"`
	TaskName      string `json:"task_name"`
	Status        string `json:"status"`
}
type ReleaseRequest struct {
	RegistryEpoch      string `json:"registry_epoch"`
	RequestID          string `json:"request_id"`
	PreviousRunStopped bool   `json:"previous_run_stopped"`
	NoMoreRequests     bool   `json:"no_more_requests"`
	Reason             string `json:"reason"`
}

func (s Store) Reserve(req ReserveRequest) (Reservation, error) {
	return s.reserve(req, nil)
}

type replacement struct {
	id      string
	expect  uint64
	release ReleaseRequest
}

// Replace releases the previous reservation and admits its replacement atomically.
func (s Store) Replace(id string, expect uint64, release ReleaseRequest, req ReserveRequest) (Reservation, error) {
	return s.reserve(req, &replacement{id, expect, release})
}

func (s Store) reserve(req ReserveRequest, previous *replacement) (Reservation, error) {
	root, err := s.canonicalRoot()
	if err != nil {
		return Reservation{}, err
	}
	req.Root, err = workerRoot(root, req.Root)
	if err != nil {
		return Reservation{}, err
	}
	if !validText(req.RequestID, 160) || !validText(req.CoordinatorSession, 160) || !validText(req.Session, 160) || !validText(req.Space, 160) || len(req.IntentID) != 32 || !validText(req.StepID, 160) || len(req.DefinitionHash) != 64 || req.SourceRevision == 0 || req.Agent != "aidlc-worker" {
		return Reservation{}, errors.New("invalid reservation request")
	}
	unlock, err := filestore.Lock(root, "assignments")
	if err != nil {
		return Reservation{}, err
	}
	defer unlock()
	r, err := s.Read()
	if err != nil {
		return Reservation{}, err
	}
	if req.RegistryEpoch != r.Epoch {
		return Reservation{}, errors.New("stale registry epoch")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return Reservation{}, err
	}
	hash := filestore.Hash(raw)
	if req.RequestID == r.Initialization.RequestID {
		return Reservation{}, errors.New("request id already used for initialization")
	}
	for _, v := range r.Reservations {
		if v.Release != nil && v.Release.RequestID == req.RequestID {
			return Reservation{}, errors.New("request id already used for release")
		}
	}
	for _, v := range r.Reservations {
		if v.RequestID == req.RequestID {
			if v.RequestHash != hash {
				return Reservation{}, errors.New("request id reused with different content")
			}
			return v, nil
		}
	}
	if previous != nil {
		if _, err := releaseIn(&r, previous.id, req.CoordinatorSession, previous.expect, previous.release); err != nil {
			return Reservation{}, err
		}
	}
	for _, v := range r.Reservations {
		if v.Status != "released" && (overlapRoots(v.Root, req.Root) || (req.Unit != "" && v.Space == req.Space && v.IntentID == req.IntentID && v.Unit == req.Unit)) {
			return Reservation{}, errors.New("worker root is already reserved")
		}
	}
	id, err := newID()
	if err != nil {
		return Reservation{}, err
	}
	if req.Unit != "" {
		req.RunID = id
	}
	v := Reservation{ID: id, ReserveRequest: req, RequestHash: hash, EntryRevision: 1, TaskName: "aidlc_" + r.Epoch + "_" + id, Status: "reserved"}
	r.Reservations = append(r.Reservations, v)
	r.Revision++
	if err := admission(r); err != nil {
		return Reservation{}, err
	}
	if err := s.persist(r); err != nil {
		return Reservation{}, err
	}
	return v, nil
}

func (s Store) Release(id, session string, expect uint64, req ReleaseRequest) (Reservation, error) {
	root, err := s.canonicalRoot()
	if err != nil {
		return Reservation{}, err
	}
	unlock, err := filestore.Lock(root, "assignments")
	if err != nil {
		return Reservation{}, err
	}
	defer unlock()
	r, err := s.Read()
	if err != nil {
		return Reservation{}, err
	}
	revision := r.Revision
	v, err := releaseIn(&r, id, session, expect, req)
	if err != nil {
		return Reservation{}, err
	}
	if r.Revision != revision {
		if err := s.persist(r); err != nil {
			return Reservation{}, err
		}
	}
	return v, nil
}

func releaseIn(r *Registry, id, session string, expect uint64, req ReleaseRequest) (Reservation, error) {
	if req.RegistryEpoch != r.Epoch {
		return Reservation{}, errors.New("stale registry epoch")
	}
	if !req.PreviousRunStopped || !req.NoMoreRequests || !validText(req.Reason, 2048) || !validText(req.RequestID, 160) {
		return Reservation{}, errors.New("release requires stopped processes, no more requests and collection reason")
	}
	if req.RequestID == r.Initialization.RequestID {
		return Reservation{}, errors.New("request id already used for initialization")
	}
	for _, v := range r.Reservations {
		if v.RequestID == req.RequestID {
			return Reservation{}, errors.New("request id already used for reserve")
		}
		if v.ID != id && v.Release != nil && v.Release.RequestID == req.RequestID {
			return Reservation{}, errors.New("release request id belongs to another reservation")
		}
	}
	for i := range r.Reservations {
		v := &r.Reservations[i]
		if v.ID != id {
			continue
		}
		if v.CoordinatorSession == session && v.Release != nil && v.Release.RequestID == req.RequestID {
			if *v.Release == req && v.ReleaseRevision == expect {
				return *v, nil
			}
			return Reservation{}, errors.New("release request id content mismatch")
		}
		if v.CoordinatorSession != session || v.EntryRevision != expect {
			return Reservation{}, errors.New("release owner or revision mismatch")
		}
		if v.Status == "released" {
			return Reservation{}, errors.New("already released")
		}
		v.Release = &req
		v.ReleaseRevision = expect
		v.Status = "released"
		v.EntryRevision++
		r.Revision++
		return *v, nil
	}
	return Reservation{}, errors.New("assignment not found")
}

func workerRoot(coordinator, root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", errors.New("worker root must be absolute")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("worker root must be a directory")
	}
	return root, nil
}
func overlapRoots(a, b string) bool {
	relative, err := filepath.Rel(a, b)
	if err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return true
	}
	relative, err = filepath.Rel(b, a)
	return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
