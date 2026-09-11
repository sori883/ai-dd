package assignment

import (
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
)

type ResetRequest struct {
	RequestID      string `json:"request_id"`
	RegistryEpoch  string `json:"registry_epoch,omitempty"`
	RegistryHash   string `json:"registry_hash,omitempty"`
	Diagnosis      string `json:"diagnosis,omitempty"`
	HumanConfirmed bool   `json:"human_confirmed"`
	Reason         string `json:"reason"`
}

func (s Store) Reset(req ResetRequest) (Registry, error) {
	if !req.HumanConfirmed || !validText(req.RequestID, 160) || !validText(req.Reason, 2048) {
		return Registry{}, errors.New("reset requires human confirmation, request id and reason")
	}
	root, err := s.canonicalRoot()
	if err != nil {
		return Registry{}, err
	}
	unlock, err := filestore.Lock(root, "assignments")
	if err != nil {
		return Registry{}, err
	}
	defer unlock()
	old, readErr := s.Read()
	if readErr == nil && old.Reset != nil && old.Reset.RequestID == req.RequestID {
		if *old.Reset == req {
			return old, nil
		}
		return Registry{}, errors.New("reset request id reused")
	}
	raw, rawErr := filestore.ReadFile(root, registryPath)
	if readErr == nil {
		if old.Epoch != req.RegistryEpoch || filestore.Hash(raw) != req.RegistryHash {
			return Registry{}, errors.New("reset source epoch or hash mismatch")
		}
		if req.Diagnosis != "unrecoverable" {
			for _, v := range old.Reservations {
				if v.Status != "released" {
					return Registry{}, errors.New("active reservations require explicit unrecoverable diagnosis and human confirmation")
				}
			}
		}
		if req.Diagnosis != "" && req.Diagnosis != "unrecoverable" {
			return Registry{}, errors.New("invalid reset diagnosis")
		}
	} else if errors.Is(rawErr, os.ErrNotExist) {
		if req.Diagnosis != "missing" || req.RegistryEpoch != "" || req.RegistryHash != "" {
			return Registry{}, errors.New("missing registry diagnosis required")
		}
	} else {
		if rawErr != nil {
			return Registry{}, rawErr
		}
		if req.Diagnosis != "corrupt" || req.RegistryHash != filestore.Hash(raw) {
			return Registry{}, errors.New("corrupt registry exact hash and diagnosis required")
		}
	}
	epoch, err := newID()
	if err != nil {
		return Registry{}, err
	}
	if rawErr == nil {
		if err := filestore.WriteFile(root, "aidlc/.runtime/assignments/archive-"+filestore.Hash(raw)+".json", raw); err != nil {
			return Registry{}, err
		}
	}
	next := Registry{SchemaVersion: 2, Epoch: epoch, Revision: 1, Root: root, Initialization: InitRequest{RequestID: req.RequestID, HumanConfirmed: true, Reason: req.Reason}, Reset: &req}
	if err := s.persist(next); err != nil {
		return Registry{}, err
	}
	return next, nil
}

// New admissions reserve room for every pending response and explicit release.
func admission(r Registry) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	headroom := 1
	for _, v := range r.Reservations {
		if v.Status != "released" {
			headroom += 16384
		}
	}
	for _, d := range r.Dispatches {
		if d.Status != "bound" {
			headroom += 1024
		}
	}
	if len(raw)+headroom > filestore.MaxBytes {
		return errors.New("assignment capacity reserved for release and pending responses; reset after confirmed collection")
	}
	return nil
}
