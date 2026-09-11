package flow

import (
	"github.com/sori883/ai-dd/src/internal/filestore"
	"testing"
)

func TestLowercaseADRDeclaredAdoption(t *testing.T) {
	s, st := boundaryFixture(t)
	st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", VerificationPaths: []string{"."}, ADR: ADR{Required: true}}
	var err error
	st, err = saveExecutionFixture(t, s, st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	boundaryDoc(t, s, st, "Requirements")
	doc := declaredDoc("discovery", "adr/storage", "adr")
	raw := []byte("---\ntype: adr\ntitle: Document\ndescription: Purpose\n---\nDecision reason.\n")
	if err = filestore.WriteFile(s.Root, doc.Path, raw); err != nil {
		t.Fatal(err)
	}
	st, err = s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{doc}, Outputs: []DocumentDeclaration{}})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("declared past adr rejected: %+v %v", g, err)
	}
	st, err = s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{}})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	g, err = s.Check(st.ID)
	if err != nil || g.Status == "pass" {
		t.Fatalf("unadopted adr satisfied gate: %+v %v", g, err)
	}
}
func TestLowercaseADRRejectUppercase(t *testing.T) {
	s, st := boundaryFixture(t)
	doc := declaredDoc("discovery", "ADR/storage", "ADR")
	if _, err := s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{doc}, Outputs: []DocumentDeclaration{}}); err == nil {
		t.Fatal("legacy uppercase ADR declaration accepted")
	}
}
