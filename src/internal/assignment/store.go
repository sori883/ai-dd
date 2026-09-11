package assignment

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

const registryPath = "aidlc/.runtime/assignments/registry.json"

type Store struct {
	Root  string
	write func(string, string, []byte) error
}
type Registry struct {
	Initialization InitRequest   `json:"initialization"`
	Reset          *ResetRequest `json:"reset,omitempty"`

	Dispatches    []Dispatch    `json:"dispatches"`
	Reservations  []Reservation `json:"reservations"`
	SchemaVersion int           `json:"schema_version"`
	Epoch         string        `json:"epoch"`
	Revision      uint64        `json:"revision"`
	Root          string        `json:"root"`
}
type InitRequest struct {
	RequestID      string `json:"request_id"`
	HumanConfirmed bool   `json:"human_confirmed"`
	Reason         string `json:"reason"`
}

func (s Store) canonicalRoot() (string, error) {
	if !filepath.IsAbs(s.Root) {
		return "", errors.New("assignment root must be absolute")
	}
	root, err := filepath.EvalSymlinks(s.Root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("assignment root is not a directory")
	}
	return root, nil
}

func (s Store) Read() (Registry, error) {
	root, err := s.canonicalRoot()
	if err != nil {
		return Registry{}, err
	}
	raw, err := filestore.ReadFile(root, registryPath)
	if err != nil {
		return Registry{}, fmt.Errorf("assignment registry unavailable; restore or explicitly initialize: %w", err)
	}
	var r Registry
	if err := decode(raw, &r); err != nil {
		return Registry{}, fmt.Errorf("assignment registry invalid: %w", err)
	}
	if r.SchemaVersion != 2 || len(r.Epoch) != 32 || r.Revision == 0 || r.Root != root {
		return Registry{}, errors.New("assignment registry version, identity or root mismatch")
	}
	if _, err := hex.DecodeString(r.Epoch); err != nil {
		return Registry{}, errors.New("assignment registry invalid epoch")
	}
	if err := validateRecords(r); err != nil {
		return Registry{}, err
	}
	return r, nil
}

func (s Store) Init(req InitRequest) (Registry, error) {
	if !req.HumanConfirmed || !validText(req.RequestID, 160) || !validText(req.Reason, 2048) {
		return Registry{}, errors.New("initialization requires request id, human confirmation and reason")
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
	if _, err := filestore.ReadFile(root, registryPath); !errors.Is(err, os.ErrNotExist) {
		if old, readErr := s.Read(); readErr == nil && old.Reset == nil && old.Initialization == req {
			return old, nil
		}
		return Registry{}, errors.New("registry already exists or is unreadable; use recovery")
	}
	epoch, err := newID()
	if err != nil {
		return Registry{}, err
	}
	r := Registry{SchemaVersion: 2, Epoch: epoch, Revision: 1, Root: root, Initialization: req}
	if err := s.persist(r); err != nil {
		return Registry{}, err
	}
	return r, nil
}

func (s Store) persist(r Registry) error {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > filestore.MaxBytes {
		return errors.New("assignment registry capacity exceeded")
	}
	write := s.write
	if write == nil {
		write = filestore.WriteFile
	}
	return write(r.Root, registryPath, raw)
}

func decode(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one json value")
	}
	return nil
}

func validText(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func validateRecords(r Registry) error {
	ids, requests, roots := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, v := range r.Reservations {
		validStatus := v.Status == "reserved" || v.Status == "dispatch_pending" || v.Status == "bound" || v.Status == "uncertain" || v.Status == "released"
		if !validStatus || !hexID(v.ID, 32) || v.RegistryEpoch != r.Epoch || ids[v.ID] || requests[v.RequestID] || !validText(v.RequestID, 160) || !validText(v.CoordinatorSession, 160) || !validText(v.Session, 160) || !validText(v.Space, 160) || !hexID(v.IntentID, 32) || !hexID(v.DefinitionHash, 64) || !validText(v.StepID, 160) || !filepath.IsAbs(v.Root) || filepath.Clean(v.Root) != v.Root || v.EntryRevision == 0 || v.SourceRevision == 0 || v.Agent != "aidlc-worker" || v.TaskName != "aidlc_"+r.Epoch+"_"+v.ID {
			return errors.New("assignment registry contains invalid reservation")
		}
		if v.Status != "released" {
			for other := range roots {
				if overlapRoots(other, v.Root) {
					return errors.New("assignment registry contains overlapping occupied roots")
				}
			}
		}
		if v.Unit != "" && v.RunID != v.ID {
			return errors.New("assignment registry run identity mismatch")
		}
		req := v.ReserveRequest
		if req.Unit != "" {
			req.RunID = ""
		}
		raw, err := json.Marshal(req)
		if err != nil {
			return err
		}
		if filestore.Hash(raw) != v.RequestHash {
			return errors.New("assignment registry request hash mismatch")
		}
		ids[v.ID] = true
		requests[v.RequestID] = true
		if v.Status != "released" {
			roots[v.Root] = true
		}
	}
	keys, names, paths := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, d := range r.Dispatches {
		key, name := d.Session+"\x00"+d.ToolID, d.Session+"\x00"+d.TaskName
		if !validText(d.Session, 160) || !validText(d.ToolID, 160) || !validText(d.Turn, 160) || !validText(d.TaskName, 128) || !validText(d.Agent, 160) || !hexID(d.IntentID, 32) || !hexID(d.DefinitionHash, 64) || keys[key] || names[name] || d.Status != "dispatch_pending" && d.Status != "bound" && d.Status != "uncertain" {
			return errors.New("assignment registry contains invalid dispatch")
		}
		if d.AssignmentID != "" && !ids[d.AssignmentID] {
			return errors.New("assignment registry dispatch reservation missing")
		}
		if d.Status == "bound" {
			p := d.Session + "\x00" + d.Canonical
			if d.Canonical == "" || paths[p] {
				return errors.New("assignment registry ambiguous task path")
			}
			paths[p] = true
		}
		keys[key] = true
		names[name] = true
	}
	return nil
}
func hexID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
