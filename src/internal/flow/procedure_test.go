package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcedureBoundaryReferences(t *testing.T) {
	for _, boundary := range []string{"start", "end"} {
		t.Run(boundary, func(t *testing.T) {
			s, st := boundaryFixture(t)
			deployFlowDefinition(t, s)
			p := filepath.Join(s.Root, "aidlc/workflow/stages/discovery.md")
			raw, _ := os.ReadFile(p)
			if boundary == "start" {
				raw = []byte(strings.Replace(string(raw), "inputs:\n", "inputs:\n  - path: \"${knowledge_root}/knowledge/extra.md\"\n    version: current\n", 1))
			} else {
				raw = []byte(strings.Replace(string(raw), "outputs: \n", "outputs: \n  - role: supporting_document\n    path: \"${knowledge_root}/knowledge/extra.md\"\n", 1))
			}
			os.WriteFile(p, raw, 0644)
			st, err := s.Create("modified procedure")
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "start" {
				g, _, _ := s.startState(st)
				if g.Status == "pass" {
					t.Fatal("declared input missing but start passed")
				}
			} else {
				st.Entry = &StageEntry{Stage: st.Stage}
				c := s.endDocuments(st)
				if !strings.Contains(strings.Join(c.failures, ";"), "extra.md") {
					t.Fatal("declared output not inspected")
				}
			}
		})
	}
}
func TestProcedureBoundaryEmptyOutputsKeepTests(t *testing.T) {
	s, st := boundaryFixture(t)
	deployFlowDefinition(t, s)
	st.Stage = "tdd"
	prepareBoundaryStage(t, s, &st)
	st.Config.NoMaterialsReason = "new"
	c := s.endDocuments(st)
	if !strings.Contains(strings.Join(c.failures, ";"), "planned test commands required") {
		t.Fatal("empty outputs disabled test gate")
	}
}
