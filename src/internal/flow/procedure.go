package flow

import (
	"os"
	"path"
	"strings"

	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"github.com/sori883/ai-dd/src/internal/workflow"
)

func (c *boundaryCollector) references(st State, refs []workflow.Reference) []FileVersion {
	checked := []FileVersion{}
	seen := map[string]bool{}
	prefix := "aidlc/spaces/" + c.store.Space + "/knowledge/"
	for _, ref := range refs {
		if ref.RequiredWhen == "adr_required" && !st.Config.ADR.Required {
			continue
		}
		if ref.RequiredWhen == "materials_present" && len(st.Config.MaterialSources) == 0 {
			continue
		}
		names := []string{}
		switch ref.Refs {
		case "config.adr.refs":
			names = st.Config.ADR.Refs
		case "config.feature_knowledge":
			names = st.Config.FeatureKnowledge
		default:
			names = []string{strings.ReplaceAll(strings.ReplaceAll(ref.Path, "${knowledge_root}", strings.TrimSuffix(prefix, "/")), "${intent_id}", st.ID)}
		}
		if ref.RequiredWhen != "exists" {
			c.require(len(names) > 0, "declared document refs required: "+ref.Refs)
		}
		for _, name := range names {
			if !strings.HasPrefix(name, prefix) || path.Ext(name) != ".md" || !safeEvidencePath(name) {
				c.require(false, "unsafe declared document: "+name)
				continue
			}
			kind := ""
			for _, candidate := range []string{"Rule", "Requirements", "ImplementationPlan", "CurrentAnalysis", "Architecture"} {
				if name == c.store.documentPath(st, candidate) {
					kind = candidate
				}
			}
			if ref.Refs == "config.adr.refs" {
				kind = "ADR"
			}
			if ref.Refs == "config.feature_knowledge" {
				kind = "Knowledge"
			}
			optional := ref.RequiredWhen == "exists"
			if kind != "" {
				c.document(st, name, kind, optional)
			} else {
				if optional {
					root, err := os.OpenRoot(c.store.Root)
					if err == nil {
						_, err = root.Lstat(name)
						root.Close()
					}
					if os.IsNotExist(err) {
						continue
					}
				}
				raw, ok := c.file(name)
				if ok {
					doc, err := okfmemory.Parse(raw)
					c.require(err == nil && strings.TrimSpace(doc.String("type")) != "" && strings.TrimSpace(doc.String("title")) != "" && strings.TrimSpace(doc.String("description")) != "" && strings.TrimSpace(doc.Body) != "", "invalid declared OKF document: "+name)
				}
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
	return checked
}

// ProcedureView is a read-only view of the bound current procedure.
type ProcedureView struct {
	Stage          string             `json:"stage"`
	DefinitionHash string             `json:"definition_hash"`
	Procedure      workflow.Procedure `json:"procedure"`
	Advance        string             `json:"advance"`
	Reopen         []string           `json:"reopen"`
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
	view := ProcedureView{Stage: st.Stage, DefinitionHash: d.Hash, Procedure: d.Procedures[st.Stage], Advance: d.Next(st.Stage), Reopen: []string{}}
	for _, stage := range d.Graph.Stages {
		if d.CanReopen(st.Stage, stage.ID) {
			view.Reopen = append(view.Reopen, stage.ID)
		}
	}
	return view, nil
}
