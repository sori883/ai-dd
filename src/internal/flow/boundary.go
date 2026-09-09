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
	Stage   string        `json:"stage"`
	Inputs  []FileVersion `json:"inputs"`
	Sources []FileVersion `json:"sources"`
}
type StageAcceptance struct {
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
	st.Entry = &StageEntry{Stage: st.Stage, Inputs: inputs, Sources: sources}
	st.Revision++
	if err = s.persist(st); err != nil {
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
	if st.Status != "active" || st.Entry == nil || st.Entry.Stage != st.Stage {
		return invalid("active Intent and intent begin required")
	}
	c := boundaryCollector{store: s}
	if stageOrder[st.Stage] >= 1 {
		c.accepted(st, "discovery", s.documentPath(st, "Requirements"))
	}
	if stageOrder[st.Stage] >= 2 {
		c.accepted(st, "planning", s.documentPath(st, "ImplementationPlan"))
	}
	if st.Stage == "integration" {
		a, ok := st.Accepted["tdd"]
		c.require(ok, "accepted tdd required")
		for _, f := range a.Outputs {
			if !strings.HasPrefix(f.Path, "aidlc/spaces/"+s.Space+"/knowledge/") {
				c.accepted(st, "tdd", f.Path)
			}
		}
	}
	if len(c.failures) > 0 {
		return invalid(strings.Join(c.failures, "; "))
	}
	return nil
}

var stageOrder = map[string]int{"discovery": 0, "planning": 1, "tdd": 2, "integration": 3}

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
	if st.Entry != nil {
		if st.Entry.Stage != st.Stage {
			return invalid("entry stage mismatch")
		}
		if err := validateFileVersions(st.Entry.Inputs); err != nil {
			return err
		}
		if err := validateFileVersions(st.Entry.Sources); err != nil {
			return err
		}
	}
	if len(st.Accepted) > 4 {
		return invalid("too many accepted stages")
	}
	for stage, a := range st.Accepted {
		if _, ok := stageOrder[stage]; !ok || stage != a.Stage || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(a.ReviewTarget) {
			return invalid("invalid stage acceptance")
		}
		if err := validateFileVersions(a.Outputs); err != nil {
			return err
		}
	}
	return nil
}
