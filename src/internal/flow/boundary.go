package flow

import (
	"github.com/sori883/ai-dd/src/internal/filestore"
	"io/fs"
	"regexp"
	"strings"
)

type Boundary string

const (
	BoundaryStart Boundary = "start"
	BoundaryEnd   Boundary = "end"
)

type FileVersion struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type StageEntry struct {
	StepID  string        `json:"step_id"`
	Stage   string        `json:"stage"`
	Inputs  []FileVersion `json:"inputs"`
	Sources []FileVersion `json:"sources"`
}
type StageAcceptance struct {
	StepID       string        `json:"step_id"`
	Stage        string        `json:"stage"`
	ReviewTarget string        `json:"review_target"`
	Outputs      []FileVersion `json:"outputs"`
}

func (s Store) CheckBoundary(id string, boundary Boundary) (Gate, error) {
	st, err := s.Read(id)
	if err != nil {
		return Gate{}, err
	}
	switch boundary {
	case BoundaryStart:
		g, _, _ := s.startState(st)
		return g, nil
	case BoundaryEnd:
		return s.checkState(st)
	default:
		return Gate{}, invalid("unknown boundary")
	}
}
func (s Store) Begin(id string, expect uint64) (State, error) {
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
	if err = s.guardReassignment(st, nil); err != nil {
		return State{}, err
	}
	if st.Status != "active" {
		return State{}, invalid("Intent is not active")
	}
	if st.Entry != nil {
		return st, nil
	}
	gate, inputs, sources := s.startState(st)
	if gate.Status != "pass" {
		return State{}, invalid("start Sensor failed: " + gate.Summary)
	}
	st.Entry = &StageEntry{StepID: st.CurrentStepID, Stage: st.Stage, Inputs: inputs, Sources: sources}
	currentExecution(&st).Status = "active"
	st.Revision++
	if err = s.commit(&st); err != nil {
		return State{}, err
	}
	return st, nil
}
func (s Store) CheckWork(id string) error {
	st, err := s.Read(id)
	if err != nil {
		return err
	}
	return s.checkWorkState(st)
}
func (s Store) checkWorkState(st State) error {
	if st.ExecutionPlan.Draft != nil || st.Approval != nil && st.Approval.Status == "pending" {
		return invalid("human approval pending")
	}

	if err := s.guardWorkflow(st); err != nil {
		return err
	}
	d, err := s.boundDefinition(st)
	if err != nil {
		return err
	}
	if st.Status != "active" || st.Entry == nil || st.Entry.Stage != st.Stage || st.Entry.StepID != st.CurrentStepID {
		return invalid("active Intent and intent begin required")
	}
	c := boundaryCollector{store: s}
	c.workInputs(st, d.Procedures[st.Stage].Inputs)
	if len(c.failures) > 0 {
		return invalid(strings.Join(c.failures, "; "))
	}
	return nil
}

func safeEvidencePath(name string) bool {
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".git" {
			return false
		}
	}
	return name != "aidlc/.runtime" && !strings.HasPrefix(name, "aidlc/.runtime/") && !(strings.HasPrefix(name, "aidlc/spaces/") && strings.Contains(name, "/intents/"))
}
func validateFileVersions(files []FileVersion) error {
	seen := map[string]bool{}
	for _, file := range files {
		if !safeEvidencePath(file.Path) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(file.SHA256) || seen[file.Path] {
			return invalid("invalid or duplicate file version")
		}
		seen[file.Path] = true
	}
	return nil
}
func validateVersions(st State) error {
	for _, gate := range []Gate{st.Sensor, st.Review} {
		if gate.Status != "" && gate.StepID != st.CurrentStepID {
			return invalid("gate execution mismatch")
		}
	}
	for _, unit := range st.Config.Units {
		if unit.StepID != st.CurrentStepID {
			return invalid("Unit execution mismatch")
		}
	}

	if st.Entry != nil {
		if st.Entry.Stage != st.Stage || st.Entry.StepID != st.CurrentStepID {
			return invalid("entry stage mismatch")
		}
		if err := validateFileVersions(st.Entry.Inputs); err != nil {
			return err
		}
		if err := validateFileVersions(st.Entry.Sources); err != nil {
			return err
		}
	}
	for id, a := range st.Accepted {
		if id != a.StepID || executionStage(st, id) != a.Stage || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(a.ReviewTarget) {
			return invalid("invalid execution acceptance")
		}
		if err := validateFileVersions(a.Outputs); err != nil {
			return err
		}
	}
	return nil
}
