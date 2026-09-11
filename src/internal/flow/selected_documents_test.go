package flow

import (
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedDocumentsStartAndIdentity(t *testing.T) {
	s, st := boundaryFixture(t)
	var err error
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatalf("Rule-only start: %v", err)
	}
	if len(st.Entry.Inputs) != 1 {
		t.Fatal("resolved Rule not captured")
	}
	boundaryDoc(t, s, st, "CurrentAnalysis")
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatalf("optional shared creation rejected: %v", err)
	}
	raw, err := filestore.ReadFile(s.Root, s.documentPath(st, "Rule"))
	if err != nil {
		t.Fatal(err)
	}
	if err = filestore.WriteFile(s.Root, "aidlc/spaces/default/knowledge/rules/duplicate.md", raw); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatalf("unrelated Rule changed fixed selection: %v", err)
	}
	canonical := s.documentPath(st, "Rule")
	if err = filestore.WriteFile(s.Root, canonical, append(raw, []byte("\nChanged project rule.\n")...)); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err == nil {
		t.Fatal("changed canonical Rule accepted")
	}
	if err = filestore.WriteFile(s.Root, canonical, raw); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(s.Root, canonical)); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err == nil {
		t.Fatal("duplicate substituted missing canonical Rule")
	}
}
func TestSelectedDocumentsSharedUpdate(t *testing.T) {
	s, st := boundaryFixture(t)
	name := boundaryDoc(t, s, st, "CurrentAnalysis")
	var err error
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := filestore.ReadFile(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("\nUpdated current facts.\n")...)
	if err = filestore.WriteFile(s.Root, name, raw); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err != nil {
		t.Fatalf("shared current update rejected: %v", err)
	}
	if err = os.Rename(filepath.Join(s.Root, name), filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/replaced.md")); err != nil {
		t.Fatal(err)
	}
	if err = s.CheckWork(st.ID); err == nil {
		t.Fatal("same metadata silently swapped input path")
	}
}
func TestSelectedDocumentsOutputPath(t *testing.T) {
	s, st := boundaryFixture(t)
	st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", VerificationPaths: []string{"."}, ADR: ADR{Reason: "none"}}
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	boundaryDoc(t, s, st, "Requirements")
	output := declaredDoc("discovery", "codekb/exact", "Note")
	st, err = s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{output}})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("---\ntype: Note\ntitle: Document\ndescription: Purpose\n---\nContent.\n")
	if err = filestore.WriteFile(s.Root, "aidlc/spaces/default/knowledge/codekb/other.md", content); err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Status == "pass" {
		t.Fatal("different output path substituted")
	}
	if err = filestore.WriteFile(s.Root, output.Path, content); err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("valid explicit output: %+v %v", gate, err)
	}
}

func TestSelectedDocumentsSharedPaths(t *testing.T) {
	s, st := boundaryFixture(t)
	st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, MaterialSources: []string{"material.txt"}, VerificationPaths: []string{"."}, ADR: ADR{Reason: "none"}}
	if err := filestore.WriteFile(s.Root, "material.txt", []byte("source facts")); err != nil {
		t.Fatal(err)
	}
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	boundaryDoc(t, s, st, "Requirements")
	for _, kind := range []string{"CurrentAnalysis", "Architecture"} {
		name := boundaryDoc(t, s, st, kind)
		if err = os.Rename(filepath.Join(s.Root, name), filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/selected-"+kind+".md")); err != nil {
			t.Fatal(err)
		}
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("metadata-selected shared paths rejected: %+v %v", gate, err)
	}
}

func TestSelectedDocumentsCollectorSnapshot(t *testing.T) {
	s, st := boundaryFixture(t)
	name := boundaryDoc(t, s, st, "CurrentAnalysis")
	c := boundaryCollector{store: s}
	before, ok := c.file(name)
	if !ok {
		t.Fatal(c.failures)
	}
	if err := filestore.WriteFile(s.Root, name, append(append([]byte(nil), before...), []byte("\nLater version\n")...)); err != nil {
		t.Fatal(err)
	}
	c.selected(st, okfmemory.DocumentMatch{Type: "CurrentAnalysis"}, "one")
	c.material(name)
	if string(c.contents[name]) != string(before) {
		t.Fatal("selection/material reread different version")
	}
	if len(c.files) != 1 || c.files[0].SHA256 != filestore.Hash(before) {
		t.Fatal("snapshot hash mismatch")
	}
}

func TestSelectedDocumentsIntegrationFeature(t *testing.T) {
	s, st := boundaryFixture(t)
	fixtureExecutionStage(t, s, &st, "integration")
	prepareBoundaryStage(t, s, &st)
	st.Accepted["s04"] = StageAcceptance{StepID: "s04", Stage: "tdd", ReviewTarget: strings.Repeat("c", 64)}
	_ = flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}, Plan: "implement", Tests: []string{"go test"}}
	prepareBoundaryResults(t, s, &st)
	boundaryDoc(t, s, st, "CurrentAnalysis")
	boundaryDoc(t, s, st, "Architecture")
	name := boundaryDoc(t, s, st, "Knowledge")
	raw, err := filestore.ReadFile(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := okfmemory.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	title, description := doc.String("title"), doc.String("description")
	st.Config.DocumentOutputs = []DocumentDeclaration{{Stage: "integration", Path: name, Metadata: okfmemory.DocumentMatch{Type: "Knowledge", Title: &title, Description: &description}}}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("registered feature rejected: %+v %v", gate, err)
	}
}

func boundaryDeclaration(t *testing.T, s Store, st State, kind string) DocumentDeclaration {
	t.Helper()
	name := boundaryDoc(t, s, st, kind)
	raw, err := filestore.ReadFile(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := okfmemory.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	title, description := doc.String("title"), doc.String("description")
	return DocumentDeclaration{StepID: st.CurrentStepID, Stage: st.Stage, Path: name, Metadata: okfmemory.DocumentMatch{Type: kind, Title: &title, Description: &description}}
}

func TestSelectedDocumentsRequiredTypes(t *testing.T) {
	s, _ := boundaryFixture(t)
	name := filepath.Join(s.Root, "aidlc/workflow/stages/discovery.md")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(raw), "outputs:\n")
	end := strings.Index(string(raw)[start:], "sensors:") + start
	raw = []byte(string(raw[:start]) + "outputs: []\n" + string(raw[end:]))
	if err = os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	st, err := createExecutionFixture(t, s, "No output declaration")
	if err != nil {
		t.Fatal(err)
	}
	st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", VerificationPaths: []string{"."}, ADR: ADR{Reason: "none"}}
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Status == "pass" {
		t.Fatal("empty declarations bypassed mandatory Requirements")
	}
}

func TestSelectedDocumentsRequiredInputs(t *testing.T) {
	s, _ := boundaryFixture(t)
	name := filepath.Join(s.Root, "aidlc/workflow/stages/planning.md")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(raw), "inputs:\n")
	end := strings.Index(string(raw)[start:], "outputs:") + start
	if err = os.WriteFile(name, []byte(string(raw[:start])+"inputs: []\n"+string(raw[end:])), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := createExecutionFixture(t, s, "Required accepted inputs")
	if err != nil {
		t.Fatal(err)
	}
	fixtureExecutionStage(t, s, &st, "planning")
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Begin(st.ID, st.Revision); err == nil {
		t.Fatal("empty inputs bypassed accepted Requirements")
	}
}
