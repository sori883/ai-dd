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
				raw = []byte(strings.Replace(string(raw), "inputs:\n", "inputs:\n  - match: {type: Note}\n    count: one\n    version: current\n", 1))
			} else {
				raw = []byte(strings.Replace(string(raw), "outputs:\n", "outputs:\n  - role: supporting_document\n    path: \"${knowledge_root}/codekb/extra.md\"\n    metadata: {type: Note}\n", 1))
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

func TestProcedureBoundaryAcceptedTDDOutput(t *testing.T) {
	s, _ := boundaryFixture(t)
	doc := "aidlc/spaces/default/knowledge/codekb/decision.md"
	for _, stage := range []string{"tdd", "integration"} {
		p := filepath.Join(s.Root, "aidlc/workflow/stages", stage+".md")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if stage == "tdd" {
			raw = []byte(strings.Replace(string(raw), "outputs:\n", "outputs:\n  - role: decision\n    path: \"${knowledge_root}/codekb/decision.md\"\n    metadata: {type: Design}\n", 1))
		} else {
			raw = []byte(strings.Replace(string(raw), "inputs:\n", "inputs:\n  - match: {type: Design}\n    count: one\n    version: accepted\n    accepted_at: tdd\n", 1))
		}
		if err = os.WriteFile(p, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := s.Create("explicit output")
	if err != nil {
		t.Fatal(err)
	}
	st.Stage = "tdd"
	prepareBoundaryStage(t, s, &st)
	head := flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: head, DirectCommit: head, Plan: "Implement", Tests: []string{"go test"}}
	boundaryFile(t, s, doc, "---\ntype: Design\ntitle: Decision\ndescription: Decision\n---\nReviewed decision.\n")
	boundaryDoc(t, s, st, "CurrentAnalysis")
	prepareBoundaryResults(t, s, &st)
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("gate %+v %v", gate, err)
	}
	st.Review = Gate{Status: "pass", Target: gate.Target}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "advance"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range st.Accepted["tdd"].Outputs {
		if v.Path == doc {
			found = true
		}
		if v.Path == s.documentPath(st, "CurrentAnalysis") {
			t.Fatal("shared current input frozen")
		}
	}
	if !found {
		t.Error("explicit TDD output not accepted")
	}
	start, _, _ := s.startState(st)
	if start.Status != "pass" {
		t.Errorf("unchanged explicit output rejected: %s", start.Summary)
	}
	boundaryFile(t, s, doc, "---\ntype: Design\ntitle: Decision\ndescription: Decision\n---\nChanged.\n")
	start, _, _ = s.startState(st)
	if start.Status == "pass" {
		t.Fatal("changed accepted output allowed")
	}
}
