package flow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io/fs"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

func git(root string, args ...string) (string, error) {
	raw, err := gitRaw(root, args...)
	return strings.TrimSpace(raw), err
}

// gitRaw keeps NUL-delimited path bytes intact, including leading whitespace.
func gitRaw(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	raw, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(diagnostic.String()), err)
	}
	return string(raw), nil
}

// Check evaluates current files without saving Sensor or review results.
func (s Store) Check(id string) (Gate, error) {
	st, err := s.Read(id)
	if err != nil {
		return Gate{}, err
	}
	return s.checkState(st)
}
func (s Store) checkState(st State) (Gate, error) {
	gate, _, err := s.checkStateSnapshot(st)
	return gate, err
}
func (s Store) checkStateSnapshot(st State) (Gate, *boundaryCollector, error) {
	if err := s.guardWorkflow(st); err != nil {
		return Gate{}, nil, err
	}
	if st.Stage == "initialization" {
		c := s.endDocuments(st)
		gate := c.gate(st.CurrentStepID)
		gate.StepID = st.CurrentStepID
		return gate, c, nil
	}
	config := st.Config
	config.Units = append([]Unit(nil), st.Config.Units...)
	for i := range config.Units {
		config.Units[i].Status = ""
	}
	revision, planHash := evidencePlan(st)
	raw, err := json.Marshal(struct {
		Stage          string
		StepID         string
		PlanRevision   uint64
		PlanHash       string
		DefinitionHash string
		Config         Config
	}{st.Stage, st.CurrentStepID, revision, planHash, st.DefinitionHash, config})
	if err != nil {
		return Gate{}, nil, err
	}
	h := sha256.New()
	h.Write(raw)
	var failures []string
	require := func(ok bool, message string) {
		if !ok {
			failures = append(failures, message)
		}
	}
	require(strings.TrimSpace(config.Objective) != "", "objective required")
	require(len(config.Scope) > 0, "scope required")
	require(len(config.Acceptance) > 0, "acceptance required")
	require(len(config.Unknowns) == 0, "blocking unknowns remain")
	head, err := git(s.Root, "rev-parse", "HEAD")
	if err != nil {
		return Gate{}, nil, err
	}
	h.Write([]byte(head))
	require(config.CodeRevision == head, "code_revision must match HEAD")
	files, err := git(s.Root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return Gate{}, nil, err
	}
	names := strings.Split(files, "\x00")
	sort.Strings(names)
	for _, name := range names {
		if name == "" || strings.HasPrefix(name, "aidlc/") {
			continue
		}
		content, err := filestore.ReadFile(s.Root, name)
		h.Write([]byte(name))
		if err != nil {
			h.Write([]byte(err.Error()))
			require(false, "unreadable code: "+name)
			continue
		}
		h.Write(content)
	}
	c := &boundaryCollector{store: s}

	stages := map[string]int{}
	for i, step := range executionSteps(st) {
		if _, exists := stages[step.Stage]; !exists {
			stages[step.Stage] = i
		}
	}
	for _, artifact := range config.Artifacts {
		if !fs.ValidPath(artifact.Path) || strings.Contains(artifact.Path, "\\") {
			require(false, "invalid artifact path")
			continue
		}
		artifactStage, selectedStage := stages[artifact.Stage]
		if !supportedStage(artifact.Stage) {
			require(false, "unknown artifact stage")
			continue
		}
		if artifact.Kind != "Knowledge" && artifact.Kind != "ADR" && artifact.Kind != "test" {
			require(false, "unknown artifact kind")
			continue
		}
		if strings.HasPrefix(artifact.Path, "aidlc/.runtime/") || artifact.Path == "aidlc/.runtime" || (strings.HasPrefix(artifact.Path, "aidlc/spaces/") && strings.Contains(artifact.Path, "/intents/")) {
			require(false, "state/runtime cannot be a review artifact")
			continue
		}
		if !selectedStage || artifactStage > stages[st.Stage] {
			continue
		}

		content, good := c.file(artifact.Path)
		h.Write([]byte(artifact.Path))
		if !good {
			require(false, "unreadable artifact: "+artifact.Path)
			continue
		}
		h.Write(content)
		if artifact.Kind == "test" {
			require(len(content) > 0, "empty test artifact")
			continue
		}
		prefix := "aidlc/spaces/" + s.Space + "/knowledge/"
		require(strings.HasPrefix(artifact.Path, prefix), "artifact outside Space")
		doc, err := okfmemory.Parse(content)
		if err != nil {
			require(false, "invalid OKF: "+artifact.Path)
			continue
		}
		require(strings.TrimSpace(doc.String("type")) != "", "artifact type required")
		if artifact.Kind == "ADR" {
			require(doc.String("type") == "adr", "ADR type mismatch")
		}
		if artifact.Kind == "ADR" {
			require(strings.HasPrefix(artifact.Path, prefix+"adr/"), "ADR outside lowercase adr directory")
		}
	}
	if !config.ADR.Required {
		require(strings.TrimSpace(config.ADR.Reason) != "", "reason for no ADR required")
	}
	if st.Stage == "planning" || precedingStep(st, "planning") != "" {
		require(strings.TrimSpace(config.Plan) != "", "implementation plan required")
	}
	if st.Stage == "planning" || st.Stage == "tdd" || st.Stage == "integration" {
		if len(config.Units) == 0 {
			require(len(config.Tests) > 0, "direct implementation verification required")
		}
		for _, message := range unitPlanProblems(config.Units) {
			require(false, message)
		}
	}
	if st.Stage == "tdd" || st.Stage == "integration" {
		if len(config.Units) == 0 {
			require(config.DirectCommit == head, "direct implementation result must match HEAD")
		}
		for _, unit := range st.Config.Units {
			require(unit.Status == "integrated" && unit.IntegratedCommit != "", "Unit not integrated: "+unit.ID)
			if len(unit.ResultCommit) != 40 || len(unit.IntegratedCommit) != 40 {
				require(false, "invalid Unit result commit")
				continue
			}
			_, resultErr := git(s.Root, "merge-base", "--is-ancestor", unit.ResultCommit, unit.IntegratedCommit)
			require(resultErr == nil, "Unit result not present in integration commit")
			_, integrationErr := git(s.Root, "merge-base", "--is-ancestor", unit.IntegratedCommit, head)
			require(integrationErr == nil, "Unit integration not present in current HEAD")
		}
	}
	c = s.collectEndDocuments(st, c)
	failures = append(failures, c.failures...)
	extra, err := json.Marshal(append(append([]FileVersion(nil), c.files...), c.sources...))
	if err != nil {
		return Gate{}, nil, err
	}
	h.Write(extra)
	gate := Gate{StepID: st.CurrentStepID, Target: fmt.Sprintf("%x", h.Sum(nil)), Status: "pass", Summary: "requirements satisfied"}
	if len(failures) > 0 {
		gate.Status = "fail"
		gate.Summary = strings.Join(failures, "; ")
	}
	return gate, c, nil
}
func unitPlanProblems(units []Unit) []string {
	var problems []string
	byID := map[string]Unit{}
	for _, unit := range units {
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`).MatchString(unit.ID) || byID[unit.ID].ID != "" {
			problems = append(problems, "duplicate or empty Unit ID")
		}
		byID[unit.ID] = unit
		if unit.Bolt == "" || len(unit.Scope) == 0 || len(unit.Tests) == 0 || len(unit.BaseCommit) != 40 {
			problems = append(problems, "incomplete Unit plan: "+unit.ID)
		}
		for _, scope := range unit.Scope {
			if scope == "." || path.Clean(scope) != scope || strings.HasPrefix(scope, "../") || strings.HasPrefix(scope, "/") {
				problems = append(problems, "invalid Unit scope")
			}
		}
	}
	visiting := map[string]bool{}
	done := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if visiting[id] {
			problems = append(problems, "Unit dependency cycle")
			return
		}
		if done[id] {
			return
		}
		unit, ok := byID[id]
		if !ok {
			problems = append(problems, "unknown dependency: "+id)
			return
		}
		visiting[id] = true
		for _, dep := range unit.DependsOn {
			visit(dep)
		}
		visiting[id] = false
		done[id] = true
	}
	for id := range byID {
		visit(id)
	}
	return problems
}
