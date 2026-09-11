// Package flow manages the current four-stage Intent state.
package flow

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Artifact struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Stage string `json:"stage"`
}
type ADR struct {
	Required bool   `json:"required"`
	Reason   string `json:"reason"`
}
type Unit struct {
	VerificationPaths []string `json:"verification_paths,omitempty"`
	ResultSHA256      string   `json:"result_sha256"`
	StepID            string   `json:"step_id"`
	ID                string   `json:"id"`
	Bolt              string   `json:"bolt"`
	Status            string   `json:"status"`
	DependsOn         []string `json:"depends_on"`
	Scope             []string `json:"scope"`
	Tests             []string `json:"tests"`
}
type Config struct {
	VerificationPaths []string              `json:"verification_paths"`
	DocumentInputs    []DocumentDeclaration `json:"document_inputs"`
	DocumentOutputs   []DocumentDeclaration `json:"document_outputs"`
	MaterialSources   []string              `json:"material_sources"`
	NoMaterialsReason string                `json:"no_materials_reason"`
	TestResults       []string              `json:"test_results"`
	Tests             []string              `json:"tests"`
	Objective         string                `json:"objective"`
	Plan              string                `json:"plan"`
	Scope             []string              `json:"scope"`
	Acceptance        []string              `json:"acceptance"`
	Unknowns          []string              `json:"unknowns"`
	ADR               ADR                   `json:"adr"`
	Artifacts         []Artifact            `json:"artifacts"`
	Units             []Unit                `json:"units"`
}
type Gate struct {
	StepID  string `json:"step_id"`
	Target  string `json:"target"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}
type State struct {
	HistoryRevision uint64                     `json:"history_revision"`
	HistoryHead     string                     `json:"history_head"`
	Approval        *Approval                  `json:"approval"`
	ExecutionPlan   ExecutionPlan              `json:"execution_plan"`
	CurrentStepID   string                     `json:"current_step_id"`
	DefinitionHash  string                     `json:"definition_hash"`
	PendingReopen   *PendingReopen             `json:"pending_reopen,omitempty"`
	Entry           *StageEntry                `json:"entry"`
	Accepted        map[string]StageAcceptance `json:"accepted"`
	SchemaVersion   int                        `json:"schema_version"`
	ID              string                     `json:"id"`
	Space           string                     `json:"space"`
	Name            string                     `json:"name"`
	Revision        uint64                     `json:"revision"`
	Stage           string                     `json:"stage"`
	Status          string                     `json:"status"`
	Reason          string                     `json:"reason"`
	ResumeCondition string                     `json:"resume_condition"`
	Config          Config                     `json:"config"`
	Sensor          Gate                       `json:"sensor"`
	Review          Gate                       `json:"review"`
}
type Store struct {
	Root, Space string
	write       func(string, string, []byte) error
}

func invalid(message string) error { return fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
func validID(id string) bool       { return regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) }
func (s Store) check() error {
	if !filepath.IsAbs(s.Root) || !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(s.Space) {
		return invalid("invalid project or Space")
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, name := range []string{"aidlc", "aidlc/spaces", "aidlc/spaces/" + s.Space} {
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return invalid("Space is not a real directory")
		}
	}
	return nil
}
func (s Store) path(id string) string {
	return "aidlc/spaces/" + s.Space + "/intents/" + id + "/state.json"
}
func (s Store) validate(st State) error {
	if st.SchemaVersion != 6 || !validID(st.ID) || st.Space != s.Space || st.Revision == 0 || strings.TrimSpace(st.Name) == "" || !utf8.ValidString(st.Name) {
		return invalid("invalid state identity or schema")
	}
	if st.HistoryHead != "" && (!validHash(st.HistoryHead) || st.HistoryRevision == 0 || st.HistoryRevision > st.Revision) || st.HistoryHead == "" && st.HistoryRevision != 0 {
		return invalid("invalid history head")
	}
	for _, a := range []*Approval{st.Approval} {
		if err := validateApproval(a, st); err != nil {
			return err
		}
	}
	for _, p := range []*PlanVersion{st.ExecutionPlan.Approved, st.ExecutionPlan.Draft} {
		if p != nil {
			if err := validateApproval(p.Approval, st); err != nil {
				return err
			}
			if p.Approval != nil && (p.Approval.Target != PlanHash(*p) || p.Approval.PlanRevision != p.Revision || p.Approval.PlanHash != PlanHash(*p)) {
				return invalid("plan approval target mismatch")
			}
		}
	}
	hashPattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	if !hashPattern.MatchString(st.DefinitionHash) {
		return invalid("invalid workflow binding")
	}
	if p := st.PendingReopen; p != nil {
		_, err := time.Parse(time.RFC3339Nano, p.At)
		invalidTarget := p.Revision != st.Revision || p.From != st.Stage || !supportedStage(p.To)
		draft := st.ExecutionPlan.Draft
		if draft == nil || p.StepID != draft.ReopenStepID || p.PlanRevision != draft.Revision || p.PlanHash != PlanHash(*draft) || p.Reason != draft.Reason || p.To != executionStage(st, p.StepID) {
			invalidTarget = true
		}
		invalidReason := strings.TrimSpace(p.Reason) == "" || !utf8.ValidString(p.Reason)
		invalidHashes := !hashPattern.MatchString(p.LogHash) || !hashPattern.MatchString(p.LogAfterHash)
		if invalidTarget || invalidReason || err != nil || invalidHashes {
			return invalid("invalid pending reopen")
		}
	}
	if err := validateExecutionPlan(st); err != nil {
		return err
	}
	if !supportedStage(st.Stage) {
		return invalid("invalid stage")
	}
	if err := validateVerificationConfig(st.Config); err != nil {
		return err
	}
	if err := validateVersions(st); err != nil {
		return err
	}
	switch st.Status {
	case "active", "waiting", "paused", "completed", "cancelled":
	default:
		return invalid("invalid status")
	}
	return nil
}
func (s Store) persist(st State) error {
	if err := s.validate(st); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if len(raw)+1 > filestore.MaxBytes {
		return invalid("state exceeds 256 KiB")
	}
	write := s.write
	if write == nil {
		write = filestore.WriteFile
	}
	return write(s.Root, s.path(st.ID), append(raw, '\n'))
}
func (s Store) Create(name string) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	if strings.TrimSpace(name) == "" {
		return State{}, invalid("name required")
	}
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return State{}, err
	}
	defer release()
	d, err := s.definition()
	if err != nil {
		return State{}, err
	}
	id := make([]byte, 16)
	rand.Read(id)
	st := State{SchemaVersion: 6, DefinitionHash: d.Hash, ID: fmt.Sprintf("%x", id), Space: s.Space, Name: name, Revision: 1, Stage: "initialization", Status: "active", CurrentStepID: "s01",
		ExecutionPlan: ExecutionPlan{NextID: 3, Bootstrap: []ExecutionStep{{ID: "s01", Stage: "initialization", Status: "pending"}, {ID: "s02", Stage: "discovery", Status: "pending"}}}}
	if _, err := os.Lstat(filepath.Join(s.Root, s.path(st.ID))); !os.IsNotExist(err) {
		return State{}, fmt.Errorf("identity already exists: %w", fs.ErrExist)
	}
	err = s.commit(&st)
	return st, err
}
func (s Store) Read(id string) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	if !validID(id) {
		return State{}, invalid("invalid Intent ID")
	}
	raw, err := filestore.ReadFile(s.Root, s.path(id))
	if err != nil {
		return State{}, err
	}
	var st State
	if !utf8.Valid(raw) {
		return st, invalid("invalid UTF-8")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return st, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&st); err != nil {
		return st, invalid(err.Error())
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return st, invalid("trailing JSON")
	}
	if st.ID != id {
		return st, invalid("identity mismatch")
	}
	return st, s.validate(st)
}
func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return invalid("invalid JSON")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim == '{' {
		keys := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || keys[name] {
				return invalid("duplicate JSON key")
			}
			keys[name] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	} else if delim == '[' {
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	} else {
		return invalid("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
func (s Store) Save(st State, expect uint64) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return State{}, err
	}
	defer release()
	current, err := s.Read(st.ID)
	if err != nil {
		return State{}, err
	}
	if current.Revision != expect || st.Revision != expect || expect == ^uint64(0) {
		return State{}, invalid("revision conflict")
	}
	if err := s.guardReassignment(current, nil); err != nil {
		return State{}, err
	}
	if st.HistoryHead != current.HistoryHead || st.HistoryRevision != current.HistoryRevision {
		return State{}, invalid("history head is CLI owned")
	}
	if !reflect.DeepEqual(st.Approval, current.Approval) {
		return State{}, invalid("human approval is CLI owned")
	}
	if !reflect.DeepEqual(st.ExecutionPlan, current.ExecutionPlan) || st.CurrentStepID != current.CurrentStepID || st.Stage != current.Stage || st.Status != current.Status {
		return State{}, invalid("execution progress is CLI owned")
	}
	if !reflect.DeepEqual(st.Config.DocumentInputs, current.Config.DocumentInputs) || !reflect.DeepEqual(st.Config.DocumentOutputs, current.Config.DocumentOutputs) {
		return State{}, invalid("document lists are managed by intent documents")
	}
	if st.DefinitionHash != current.DefinitionHash || !reflect.DeepEqual(st.PendingReopen, current.PendingReopen) {
		return State{}, invalid("workflow binding is CLI owned")
	}
	if !reflect.DeepEqual(st.Entry, current.Entry) || !reflect.DeepEqual(st.Accepted, current.Accepted) {
		return State{}, invalid("entry and accepted are CLI owned")
	}
	if !reflect.DeepEqual(st.Config.MaterialSources, current.Config.MaterialSources) || st.Config.NoMaterialsReason != current.Config.NoMaterialsReason {
		if current.Stage != "discovery" {
			return State{}, invalid("material assumptions changed; reopen discovery")
		}
		st.Entry = nil
	}
	st.Revision++
	if err := s.commit(&st); err != nil {
		return State{}, err
	}
	return st, nil
}
func (s Store) List() ([]State, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), "aidlc/spaces/"+s.Space+"/intents")
	if os.IsNotExist(err) {
		return []State{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []State{}
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) {
			return nil, invalid("invalid Intent directory")
		}
		st, err := s.Read(entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}
func (s Store) Resolve(name string) (string, error) {
	states, err := s.List()
	if err != nil {
		return "", err
	}
	id := ""
	for _, st := range states {
		if st.Name == name {
			if id != "" {
				return "", invalid("ambiguous name; select by ID")
			}
			id = st.ID
		}
	}
	if id == "" {
		return "", fs.ErrNotExist
	}
	return id, nil
}
