package flow

import (
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"path"
	"reflect"
	"strings"
)

type DocumentDeclaration struct {
	Stage    string                  `json:"stage"`
	Path     string                  `json:"path"`
	Metadata okfmemory.DocumentMatch `json:"metadata"`
}
type IntentDocuments struct {
	Inputs  []DocumentDeclaration `json:"inputs"`
	Outputs []DocumentDeclaration `json:"outputs"`
}

func (s Store) SetDocuments(id string, expect uint64, documents IntentDocuments) (State, error) {
	return s.change(id, expect, func(st *State) error {
		if documents.Inputs == nil || documents.Outputs == nil {
			return invalid("inputs and outputs arrays required")
		}
		for _, list := range [][]DocumentDeclaration{documents.Inputs, documents.Outputs} {
			seen := map[string]bool{}
			for i := range list {
				d := &list[i]
				prefix := "aidlc/spaces/" + s.Space + "/knowledge/"
				if !supportedStage(d.Stage) || !strings.HasPrefix(d.Path, prefix) || !safeEvidencePath(d.Path) || path.Ext(d.Path) != ".md" {
					return invalid("invalid document stage or Space path")
				}
				if _, err := okfmemory.ConceptPath(strings.TrimSuffix(strings.TrimPrefix(d.Path, prefix), ".md")); err != nil {
					return err
				}
				key := d.Stage + "/" + d.Path
				if seen[key] {
					return invalid("duplicate document declaration")
				}
				seen[key] = true
				if d.Metadata.Type == "ADR" || strings.HasPrefix(d.Path, prefix+"ADR/") {
					return invalid("use lowercase adr type and directory")
				}
				if d.Metadata.Type == "adr" && !strings.HasPrefix(d.Path, prefix+"adr/") {
					return invalid("adr must use knowledge/adr")
				}
				if err := d.Metadata.Validate(); err != nil {
					return err
				}
				if d.Metadata.Title == nil || d.Metadata.Description == nil {
					return invalid("document title and description required")
				}
				if d.Metadata.Type == "Requirements" || d.Metadata.Type == "ImplementationPlan" {
					if d.Metadata.IntentID != nil && *d.Metadata.IntentID != id {
						return invalid("document Intent mismatch")
					}
					bound := id
					d.Metadata.IntentID = &bound
				}
			}
		}
		for i := range documents.Outputs {
			d := &documents.Outputs[i]
			if d.Metadata.Type == "adr" {
				for _, old := range st.Config.DocumentOutputs {
					if old.Stage == d.Stage && old.Path == d.Path && old.Metadata.Type == "adr" && old.Metadata.IntentID != nil {
						if d.Metadata.IntentID != nil && *d.Metadata.IntentID != *old.Metadata.IntentID {
							return invalid("registered adr Intent binding changed")
						}
						bound := *old.Metadata.IntentID
						d.Metadata.IntentID = &bound
					}
				}

				if !strings.HasPrefix(d.Path, "aidlc/spaces/"+s.Space+"/knowledge/adr/") {
					return invalid("adr output must use knowledge/adr")
				}
				exists, err := registrationFile(s.Root, d.Path)
				if err == nil && !exists {
					if d.Metadata.IntentID != nil && *d.Metadata.IntentID != id {
						return invalid("new adr Intent mismatch")
					}
					bound := id
					d.Metadata.IntentID = &bound
				} else if err != nil {
					return err
				}
			}
		}
		stageList := func(list []DocumentDeclaration, stage string) []DocumentDeclaration {
			out := []DocumentDeclaration{}
			for _, d := range list {
				if d.Stage == stage {
					out = append(out, d)
				}
			}
			return out
		}
		for stage := range st.Accepted {
			if !reflect.DeepEqual(stageList(st.Config.DocumentInputs, stage), stageList(documents.Inputs, stage)) || !reflect.DeepEqual(stageList(st.Config.DocumentOutputs, stage), stageList(documents.Outputs, stage)) {
				return invalid("accepted document declaration changed; reopen " + stage)
			}
		}
		if !reflect.DeepEqual(stageList(st.Config.DocumentInputs, st.Stage), stageList(documents.Inputs, st.Stage)) {
			st.Entry = nil
		}
		st.Config.DocumentInputs = documents.Inputs
		st.Config.DocumentOutputs = documents.Outputs
		st.Sensor = Gate{}
		st.Review = Gate{}
		return nil
	})
}

// registrationFile checks existence without opening document contents, including FIFOs.
func registrationFile(rootPath, name string) (bool, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return false, err
	}
	defer root.Close()
	parts := strings.Split(name, "/")
	current := ""
	for i, part := range parts {
		if current != "" {
			current += "/"
		}
		current += part
		info, err := root.Lstat(current)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, invalid("document path contains symlink")
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return false, invalid("document parent is not a directory")
			}
		} else if !info.Mode().IsRegular() {
			return false, invalid("document must be a regular file")
		}
	}
	return true, nil
}
