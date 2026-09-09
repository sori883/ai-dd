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
	config := st.Config
	config.Units = append([]Unit(nil), st.Config.Units...)
	for i := range config.Units {
		config.Units[i].Status = ""
	}
	raw, err := json.Marshal(struct {
		Stage  string
		Config Config
	}{st.Stage, config})
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
	adrRefs := map[string]bool{}
	stages := map[string]int{"discovery": 0, "planning": 1, "tdd": 2, "integration": 3}
	for _, artifact := range config.Artifacts {
		if !fs.ValidPath(artifact.Path) || strings.Contains(artifact.Path, "\\") {
			require(false, "invalid artifact path")
			continue
		}
		artifactStage, knownStage := stages[artifact.Stage]
		if !knownStage {
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
		if artifactStage > stages[st.Stage] {
			continue
		}

		content, err := filestore.ReadFile(s.Root, artifact.Path)
		h.Write([]byte(artifact.Path))
		if err != nil {
			h.Write([]byte(err.Error()))
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
			require(doc.String("type") == "ADR", "ADR type mismatch")
		}
		if artifact.Kind == "ADR" && strings.HasPrefix(artifact.Path, prefix+"ADR/") {
			adrRefs[artifact.Path] = true
		}
	}
	if config.ADR.Required {
		require(len(config.ADR.Refs) > 0, "ADR reference required")
		for _, ref := range config.ADR.Refs {
			require(adrRefs[ref], "ADR reference missing or outside ADR directory")
		}
	} else {
		require(strings.TrimSpace(config.ADR.Reason) != "", "reason for no ADR required")
	}
	if st.Stage != "discovery" {
		require(strings.TrimSpace(config.Plan) != "", "implementation plan required")
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
	c := s.endDocuments(st)
	failures = append(failures, c.failures...)
	extra, err := json.Marshal(append(append([]FileVersion(nil), c.files...), c.sources...))
	if err != nil {
		return Gate{}, nil, err
	}
	h.Write(extra)
	gate := Gate{Target: fmt.Sprintf("%x", h.Sum(nil)), Status: "pass", Summary: "requirements satisfied"}
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
