package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"github.com/sori883/ai-dd/src/internal/workflow"
	"path"
	"path/filepath"
	"strings"
)

func (c *boundaryCollector) snapshot() {
	if c.scanned {
		return
	}
	c.scanned = true
	prefix := "aidlc/spaces/" + c.store.Space + "/knowledge/"
	docs, err := okfmemory.ScanDocuments(filepath.Join(c.store.Root, filepath.FromSlash(prefix)))
	if err != nil {
		c.require(false, "document selection: "+err.Error())
		return
	}
	if c.contents == nil {
		c.contents = map[string][]byte{}
	}
	for _, selected := range docs {
		name := prefix + selected.Path
		raw, exists := c.contents[name]
		if !exists {
			raw = selected.Raw
			c.contents[name] = raw
		}
		doc, err := okfmemory.Parse(raw)
		if err != nil {
			c.require(false, "invalid selected document: "+name)
			continue
		}
		c.documents = append(c.documents, resolvedDocument{Path: name, Metadata: doc})
	}
}

type resolvedDocument struct {
	Path     string
	Metadata okfmemory.Document
}

func expandedMatch(st State, m okfmemory.DocumentMatch) okfmemory.DocumentMatch {
	if m.IntentID != nil && *m.IntentID == "${intent_id}" {
		id := st.ID
		m.IntentID = &id
	}
	return m
}
func (c *boundaryCollector) selected(st State, match okfmemory.DocumentMatch, count string) []string {
	c.snapshot()
	match = expandedMatch(st, match)
	names := []string{}
	for _, doc := range c.documents {
		if match.Matches(doc.Metadata) {
			names = append(names, doc.Path)
		}
	}
	c.require(count == "one" && len(names) == 1 || count == "optional" && len(names) <= 1 || count == "many" && len(names) > 0, "selector count "+count+" mismatch: "+match.Type)
	return names
}
func (c *boundaryCollector) references(st State, refs []workflow.Reference) []FileVersion {
	checked := []FileVersion{}
	seen := map[string]bool{}
	prefix := "aidlc/spaces/" + c.store.Space + "/knowledge/"
	var visit func(workflow.Reference)
	visit = func(ref workflow.Reference) {
		if ref.Version == "accepted" && precedingStep(st, ref.AcceptedAt) == "" {
			return
		}

		if ref.Declared != "" {
			declarations := st.Config.DocumentInputs
			if c.outputs {
				declarations = st.Config.DocumentOutputs
			}
			for _, doc := range declarations {
				if doc.StepID == st.CurrentStepID && doc.Stage == st.Stage {
					metadata := doc.Metadata
					visit(workflow.Reference{Path: doc.Path, Metadata: &metadata})
				}
			}
			return
		}
		names := []string{}
		var match okfmemory.DocumentMatch
		if ref.Match != nil {
			match = expandedMatch(st, *ref.Match)
			names = c.selected(st, match, ref.Count)
		} else if ref.Metadata != nil {
			match = expandedMatch(st, *ref.Metadata)
			names = []string{strings.ReplaceAll(strings.ReplaceAll(ref.Path, "${knowledge_root}", strings.TrimSuffix(prefix, "/")), "${intent_id}", st.ID)}
		}
		for _, name := range names {
			if !strings.HasPrefix(name, prefix) || path.Ext(name) != ".md" || !safeEvidencePath(name) {
				c.require(false, "unsafe declared document: "+name)
				continue
			}
			raw, ok := c.file(name)
			if !ok {
				continue
			}
			doc, err := okfmemory.Parse(raw)
			c.require(err == nil && match.Matches(doc), "declared metadata mismatch: "+name)
			c.document(st, name, match.Type, false)
			if match.Type == "Rule" {
				c.require(name == c.store.documentPath(st, "Rule"), "Rule must use required rules path")
			}
			if ref.Version == "accepted" {
				c.accepted(st, ref.AcceptedAt, name)
			}
			for _, version := range c.files {
				if version.Path == name && !seen[name] {
					checked = append(checked, version)
					seen[name] = true
				}
			}
		}
	}
	for _, ref := range refs {
		visit(ref)
	}
	return checked
}
func (c *boundaryCollector) workInputs(st State, refs []workflow.Reference) {
	current := c.references(st, refs)
	current = append(current, c.requiredInputs(st)...)
	if st.Entry == nil || st.Entry.Stage != st.Stage || st.Entry.StepID != st.CurrentStepID {
		c.require(false, "intent begin required")
		return
	}
	for _, old := range st.Entry.Inputs {
		if st.Stage == "integration" && precedingStep(st, "tdd") != "" {
			proof := false
			for _, f := range st.Accepted[precedingStep(st, "tdd")].Outputs {
				if f.Path == old.Path {
					proof = true
				}
			}
			if proof {
				continue
			}
		}
		found := false
		for _, now := range current {
			if now.Path == old.Path {
				found = true
				mutable := false
				doc, err := okfmemory.Parse(c.contents[now.Path])
				if err == nil {
					mutable = doc.String("type") == "CurrentAnalysis" || doc.String("type") == "Architecture"
				}
				for _, output := range st.Config.DocumentOutputs {
					if output.StepID == st.CurrentStepID && output.Stage == st.Stage && output.Path == now.Path {
						mutable = true
					}
				}
				c.require(mutable || now.SHA256 == old.SHA256, "entry input changed: "+old.Path)
			}
		}
		c.require(found, "entry input disappeared or changed selection: "+old.Path)
	}
	if st.Stage == "integration" && precedingStep(st, "tdd") != "" {
		a, ok := st.Accepted[precedingStep(st, "tdd")]
		c.require(ok, "accepted tdd required")
		for _, f := range a.Outputs {
			c.accepted(st, "tdd", f.Path)
		}
	}
}

type ResolvedReference struct {
	Reference   workflow.Reference `json:"reference"`
	Candidates  []FileVersion      `json:"candidates"`
	Diagnostics []string           `json:"diagnostics"`
}

// ProcedureView is a read-only view of the bound current procedure.
type ProcedureView struct {
	StepID         string                `json:"step_id"`
	PlanRevision   uint64                `json:"plan_revision"`
	Inputs         []ResolvedReference   `json:"inputs"`
	Outputs        []DocumentDeclaration `json:"outputs"`
	Diagnostics    []string              `json:"diagnostics"`
	Stage          string                `json:"stage"`
	DefinitionHash string                `json:"definition_hash"`
	Procedure      workflow.Procedure    `json:"procedure"`
	Advance        string                `json:"advance"`
	Reopen         []string              `json:"reopen"`
}

func (s Store) Procedure(id string) (ProcedureView, error) {
	st, err := s.Read(id)
	if err != nil {
		return ProcedureView{}, err
	}
	d, err := s.boundDefinition(st)
	if err != nil {
		return ProcedureView{}, err
	}
	view := ProcedureView{StepID: st.CurrentStepID, PlanRevision: st.ExecutionPlan.Revision, Stage: st.Stage, DefinitionHash: d.Hash, Procedure: d.Procedures[st.Stage], Reopen: []string{}}
	steps := executionSteps(st)
	for i, step := range steps {
		if step.Status == "completed" {
			view.Reopen = append(view.Reopen, step.ID)
		}
		if step.ID == st.CurrentStepID && i+1 < len(steps) {
			view.Advance = steps[i+1].ID
		}
	}
	filtered := []workflow.Reference{}
	for _, ref := range view.Procedure.Inputs {
		if ref.Version != "accepted" || precedingStep(st, ref.AcceptedAt) != "" {
			filtered = append(filtered, ref)
		}
	}
	view.Procedure.Inputs = filtered
	c := boundaryCollector{store: s}
	view.Inputs = []ResolvedReference{}
	view.Outputs = []DocumentDeclaration{}
	view.Diagnostics = []string{}
	for _, ref := range view.Procedure.Inputs {
		before := len(c.failures)
		candidates := c.references(st, []workflow.Reference{ref})
		view.Inputs = append(view.Inputs, ResolvedReference{Reference: ref, Candidates: candidates, Diagnostics: append([]string{}, c.failures[before:]...)})
	}
	prefix := "aidlc/spaces/" + s.Space + "/knowledge"
	for _, ref := range view.Procedure.Outputs {
		if ref.Declared != "" {
			for _, doc := range st.Config.DocumentOutputs {
				if doc.StepID == st.CurrentStepID && doc.Stage == st.Stage {
					view.Outputs = append(view.Outputs, doc)
				}
			}
			continue
		}
		if ref.Metadata != nil {
			view.Outputs = append(view.Outputs, DocumentDeclaration{StepID: st.CurrentStepID, Stage: st.Stage, Path: strings.ReplaceAll(strings.ReplaceAll(ref.Path, "${knowledge_root}", prefix), "${intent_id}", st.ID), Metadata: expandedMatch(st, *ref.Metadata)})
		}
	}
	view.Diagnostics = append(view.Diagnostics, c.failures...)
	return view, nil
}

// requiredInputs keeps the built-in prerequisite gates independent of optional
// additional declarations, while sharing the same selector and byte collector.
func (c *boundaryCollector) requiredInputs(st State) []FileVersion {
	refs := []workflow.Reference{{Path: c.store.documentPath(st, "Rule"), Metadata: &okfmemory.DocumentMatch{Type: "Rule"}}}
	if st.Stage != "discovery" && st.Stage != "initialization" {
		id := st.ID
		refs = append(refs, workflow.Reference{Match: &okfmemory.DocumentMatch{Type: "Requirements", IntentID: &id}, Count: "one", Version: "accepted", AcceptedAt: "discovery"})
		if st.Stage == "tdd" || st.Stage == "integration" {
			refs = append(refs, workflow.Reference{Match: &okfmemory.DocumentMatch{Type: "ImplementationPlan", IntentID: &id}, Count: "one", Version: "accepted", AcceptedAt: "planning"})
		}
	}
	files := c.references(st, refs)
	if st.Stage == "initialization" {
		for _, name := range []string{".codex/hooks.json", ".agents/skills/aidlc/SKILL.md", ".agents/skills/aidlc-cli/SKILL.md", ".agents/skills/aidlc-okf/SKILL.md"} {
			raw, ok := c.file(name)
			if !ok {
				continue
			}
			c.require(strings.TrimSpace(string(raw)) != "", "empty initialization input: "+name)
			if strings.HasSuffix(name, ".json") {
				c.require(json.Valid(raw), "invalid CLI hook configuration")
			}
		}
		files = append([]FileVersion{}, c.files...)
	}
	return files
}
