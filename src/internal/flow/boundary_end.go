package flow

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

func (s Store) endDocuments(st State) *boundaryCollector {
	c := &boundaryCollector{store: s}
	return s.collectEndDocuments(st, c)
}
func (s Store) collectEndDocuments(st State, c *boundaryCollector) *boundaryCollector {
	d, err := s.boundDefinition(st)
	if err != nil {
		c.require(false, err.Error())
		return c
	}
	c.require(st.Status == "active", "Intent is not active")
	c.workInputs(st, d.Procedures[st.Stage].Inputs)
	c.outputs = true
	for _, version := range c.references(st, d.Procedures[st.Stage].Outputs) {
		c.recordProof(version)
	}
	c.outputs = false
	if st.Stage == "initialization" {
		return c
	}
	kinds := []string{"Requirements"}
	if st.Stage == "planning" || precedingStep(st, "planning") != "" {
		kinds = append(kinds, "ImplementationPlan")
	}
	for _, kind := range kinds {
		id := st.ID
		for _, name := range c.selected(st, okfmemory.DocumentMatch{Type: kind, IntentID: &id}, "one") {
			c.document(st, name, kind, false)
			if kind == "Requirements" && st.Stage != "discovery" {
				c.accepted(st, "discovery", name)
			}
			if kind == "ImplementationPlan" && precedingStep(st, "planning") != "" {
				c.accepted(st, "planning", name)
			}
		}
	}
	c.require(len(st.Config.MaterialSources) > 0 || strings.TrimSpace(st.Config.NoMaterialsReason) != "", "material_sources or no_materials_reason required")
	if c.contents == nil {
		c.contents = map[string][]byte{}
	}
	materials := boundaryCollector{store: s, contents: c.contents}
	for _, source := range st.Config.MaterialSources {
		materials.material(source)
	}
	c.failures = append(c.failures, materials.failures...)
	for _, kind := range []string{"CurrentAnalysis", "Architecture"} {
		count := "optional"
		if st.Stage == "architecture-analysis" {
			count = "one"
		}
		for _, name := range c.selected(st, okfmemory.DocumentMatch{Type: kind}, count) {
			c.document(st, name, kind, false)
		}
	}
	if st.Stage == "integration" {
		for _, doc := range st.Config.DocumentOutputs {
			if doc.StepID == st.CurrentStepID && doc.Metadata.Type == "Knowledge" {
				c.require(strings.HasPrefix(doc.Path, "aidlc/spaces/"+s.Space+"/knowledge/codekb/"), "integration Knowledge must use codekb")
			}
		}
	}
	if st.Config.ADR.Required {
		adopted := false
		for _, list := range [][]DocumentDeclaration{st.Config.DocumentInputs, st.Config.DocumentOutputs} {
			for _, declaration := range list {
				if declaration.Metadata.Type != "adr" || (declaration.StepID != st.CurrentStepID && declaration.StepID != precedingStep(st, declaration.Stage)) {
					continue
				}
				match := expandedMatch(st, declaration.Metadata)
				raw, ok := c.file(declaration.Path)
				if !ok {
					continue
				}
				doc, err := okfmemory.Parse(raw)
				valid := err == nil && match.Matches(doc) && strings.HasPrefix(declaration.Path, "aidlc/spaces/"+s.Space+"/knowledge/adr/")
				c.require(valid, "invalid adopted adr: "+declaration.Path)
				c.document(st, declaration.Path, "adr", false)
				adopted = adopted || valid
			}
		}
		c.require(adopted, "declared adr required")
	}
	if st.Stage == "tdd" || st.Stage == "integration" {
		c.results(st)
	}
	c.sources = materials.files
	return c
}
func (c *boundaryCollector) material(name string) {
	if !safeEvidencePath(name) {
		c.require(false, "unsafe material source")
		return
	}
	root, err := os.OpenRoot(c.store.Root)
	if err != nil {
		c.require(false, err.Error())
		return
	}
	defer root.Close()
	// Check every ancestor so a directory symlink cannot hide inside an explicit path.
	current := ""
	for _, part := range strings.Split(name, "/") {
		if current != "" {
			current += "/"
		}
		current += part
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			c.require(false, "invalid material source: "+name)
			return
		}
	}
	var names []string
	var membership []string
	err = fs.WalkDir(root.FS(), name, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !safeEvidencePath(p) || d.Type()&os.ModeSymlink != 0 {
			return fs.ErrInvalid
		}
		membership = append(membership, p+":"+d.Type().String())
		if !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	if err != nil {
		c.require(false, "invalid material tree: "+name)
		return
	}
	sort.Strings(names)
	sort.Strings(membership)
	info, err := root.Lstat(name)
	if err != nil {
		c.require(false, err.Error())
		return
	}
	if info.IsDir() {
		c.files = append(c.files, FileVersion{Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(membership, "\x00"))))})
	}
	for _, p := range names {
		raw, ok := c.file(filepath.ToSlash(p))
		c.require(ok && utf8.Valid(raw), "material must be UTF-8: "+p)
	}
}

type resultRun struct {
	Command    string `json:"command"`
	Commit     string `json:"commit"`
	ExitCode   *int   `json:"exit_code"`
	OutputPath string `json:"output_path"`
}
type resultDocument struct {
	StepID string      `json:"step_id"`
	Stage  string      `json:"stage"`
	Runs   []resultRun `json:"runs"`
}

func (c *boundaryCollector) results(st State) {
	type requirement struct{ command, commit string }
	head, headErr := git(c.store.Root, "rev-parse", "HEAD")
	c.require(headErr == nil, "current result HEAD required")
	successes := map[requirement]bool{}
	required := map[requirement]bool{}
	expected := map[string]bool{}
	if len(st.Config.Units) == 0 {
		for _, command := range st.Config.Tests {
			expected[command] = true
			commit := st.Config.DirectCommit
			if st.Stage == "integration" {
				commit = head
			}
			required[requirement{command, commit}] = true
		}
	}
	for _, u := range st.Config.Units {
		for _, command := range u.Tests {
			expected[command] = true
			commit := u.ResultCommit
			if st.Stage == "integration" {
				commit = head
			}
			required[requirement{command, commit}] = true
		}
	}
	for _, name := range st.Config.TestResults {
		if c.contents == nil {
			c.contents = map[string][]byte{}
		}
		record := boundaryCollector{store: c.store, contents: c.contents}
		raw, ok := record.file(name)
		if !ok {
			c.failures = append(c.failures, record.failures...)
			continue
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		var result resultDocument
		if !utf8.Valid(raw) || uniqueJSON(json.NewDecoder(bytes.NewReader(raw))) != nil || d.Decode(&result) != nil || d.Decode(new(any)) != io.EOF || (result.Stage != "tdd" && result.Stage != "integration") {
			c.require(false, "invalid test results JSON: "+name)
			continue
		}
		if executionStage(st, result.StepID) != result.Stage {
			c.require(false, "test result execution mismatch: "+name)
			continue
		}
		if result.StepID != st.CurrentStepID {
			for _, run := range result.Runs {
				_, err := git(c.store.Root, "cat-file", "-e", run.Commit+"^{commit}")
				output, ok := record.file(run.OutputPath)
				c.require(strings.TrimSpace(run.Command) != "" && run.ExitCode != nil && len(run.Commit) == 40 && err == nil && ok && len(bytes.TrimSpace(output)) > 0, "invalid other-stage test run: "+name)
			}
			continue
		}
		c.files = append(c.files, record.files...)
		for _, version := range record.files {
			c.recordProof(version)
		}
		for _, run := range result.Runs {
			valid := strings.TrimSpace(run.Command) != "" && run.ExitCode != nil && len(run.Commit) == 40
			_, err := git(c.store.Root, "cat-file", "-e", run.Commit+"^{commit}")
			valid = valid && err == nil
			matches := false
			if result.Stage == "integration" {
				matches = headErr == nil && run.Commit == head
			} else if len(st.Config.Units) == 0 {
				matches = run.Commit == st.Config.DirectCommit
			} else {
				for _, unit := range st.Config.Units {
					if slices.Contains(unit.Tests, run.Command) && run.Commit == unit.ResultCommit {
						matches = true
					}
				}
			}
			if !expected[run.Command] {
				head, err := git(c.store.Root, "rev-parse", "HEAD")
				matches = err == nil && run.Commit == head
			}
			output, ok := c.file(run.OutputPath)
			if ok {
				for _, version := range c.files {
					if version.Path == run.OutputPath {
						c.recordProof(version)
						break
					}
				}
			}
			valid = valid && matches && ok && len(bytes.TrimSpace(output)) > 0
			c.require(valid, "invalid test run or result commit: "+name)
			if valid && *run.ExitCode == 0 {
				successes[requirement{run.Command, run.Commit}] = true
			}
		}
	}
	c.require(len(expected) > 0, "planned test commands required")
	for r := range required {
		c.require(successes[r], "successful planned test required: "+r.command+" at "+r.commit)
	}
}

func (c *boundaryCollector) recordProof(version FileVersion) {
	for _, existing := range c.proof {
		if existing.Path == version.Path {
			return
		}
	}
	c.proof = append(c.proof, version)
}
