package flow

import (
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io/fs"
	"strings"
	"testing"
)

func declaredDoc(stage, name, kind string) DocumentDeclaration {
	title, description := "Document", "Purpose"
	return DocumentDeclaration{StepID: fixtureStepID(stage), Stage: stage, Path: "aidlc/spaces/default/knowledge/" + name + ".md", Metadata: okfmemory.DocumentMatch{Type: kind, Title: &title, Description: &description}}
}
func TestIntentDocumentsRegistration(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != 5 {
		t.Errorf("schema=%d want4", st.SchemaVersion)
	}
	docs := IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{declaredDoc("integration", "knowledge/orders", "Knowledge"), declaredDoc("discovery", "adr/storage", "adr")}}
	got, err := s.SetDocuments(st.ID, st.Revision, docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Config.DocumentOutputs) != 2 || got.Config.DocumentOutputs[1].Metadata.IntentID == nil || *got.Config.DocumentOutputs[1].Metadata.IntentID != st.ID {
		t.Fatal("output list or new ADR Intent binding missing")
	}
	if _, err = s.SetDocuments(st.ID, st.Revision, docs); err == nil {
		t.Fatal("CAS replay accepted")
	}
	saved := got
	saved.Config.Objective = "ordinary configure"
	saved, err = saveExecutionFixture(t, s, saved, saved.Revision)
	if err != nil || len(saved.Config.DocumentOutputs) != 2 {
		t.Fatal("configure lost documents")
	}
	saved.Config.DocumentOutputs = nil
	if _, err = saveExecutionFixture(t, s, saved, saved.Revision); err == nil {
		t.Fatal("generic Save replaced declaration")
	}
}
func TestIntentDocumentsFailureAndMetadata(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	docs := IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{declaredDoc("integration", "knowledge/orders", "Knowledge")}}
	fail := s
	fail.write = func(string, string, []byte) error { return fs.ErrPermission }
	if _, err = fail.SetDocuments(st.ID, st.Revision, docs); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("write failure %v", err)
	}
	got, err := s.Read(st.ID)
	if err != nil || got.Revision != st.Revision || len(got.Config.DocumentOutputs) != 0 {
		t.Fatal("partial save")
	}
	for _, fault := range []string{"space", "stage", "title", "intent"} {
		t.Run(fault, func(t *testing.T) {
			d := declaredDoc("discovery", "design/requirements", "Requirements")
			switch fault {
			case "space":
				d.Path = "aidlc/spaces/other/knowledge/a.md"
			case "stage":
				d.Stage = "missing"
			case "title":
				d.Metadata.Title = nil
			case "intent":
				id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				d.Metadata.IntentID = &id
			}
			if _, err := s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{d}}); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
}

func TestIntentDocumentsEntryAndAccepted(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	st.Entry = &StageEntry{StepID: "s02", Stage: "discovery"}
	st.Sensor = Gate{StepID: st.CurrentStepID, Status: "pass"}
	st.Review = Gate{StepID: st.CurrentStepID, Status: "pass"}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	docs := IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{declaredDoc("integration", "knowledge/orders", "Knowledge")}}
	st, err = s.SetDocuments(st.ID, st.Revision, docs)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entry == nil || st.Sensor.Status != "" || st.Review.Status != "" {
		t.Fatal("output-only update lost entry or retained gates")
	}
	docs.Inputs = []DocumentDeclaration{declaredDoc("discovery", "adr/existing", "adr")}
	st, err = s.SetDocuments(st.ID, st.Revision, docs)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entry != nil {
		t.Fatal("changed start input kept entry")
	}
	fixtureExecutionStage(t, s, &st, "planning")
	st.Accepted = map[string]StageAcceptance{"s02": {StepID: "s02", Stage: "discovery", ReviewTarget: strings.Repeat("a", 64)}}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	docs.Inputs = []DocumentDeclaration{}
	if _, err = s.SetDocuments(st.ID, st.Revision, docs); err == nil {
		t.Fatal("accepted declaration changed without reopen")
	}
}
func TestIntentDocumentsNewADRBindingPersists(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "Documents")
	if err != nil {
		t.Fatal(err)
	}
	makeDocs := func() IntentDocuments {
		return IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{declaredDoc("discovery", "adr/new", "adr")}}
	}
	st, err = s.SetDocuments(st.ID, st.Revision, makeDocs())
	if err != nil {
		t.Fatal(err)
	}
	if err = filestore.WriteFile(s.Root, makeDocs().Outputs[0].Path, []byte("---\ntype: adr\ntitle: Document\ndescription: Purpose\n---\nWhy.\n")); err != nil {
		t.Fatal(err)
	}
	st, err = s.SetDocuments(st.ID, st.Revision, makeDocs())
	if err != nil {
		t.Fatal(err)
	}
	if st.Config.DocumentOutputs[0].Metadata.IntentID == nil || *st.Config.DocumentOutputs[0].Metadata.IntentID != st.ID {
		t.Fatal("new ADR binding reclassified after file creation")
	}
}

func TestIntentDocumentsLegacyFieldsRejected(t *testing.T) {
	for _, field := range []string{"feature_knowledge", "refs"} {
		t.Run(field, func(t *testing.T) {
			s := flowStore(t)
			st, err := createExecutionFixture(t, s, "Documents")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err = json.Unmarshal(raw, &object); err != nil {
				t.Fatal(err)
			}
			config := object["config"].(map[string]any)
			if field == "refs" {
				config["adr"].(map[string]any)[field] = []string{}
			} else {
				config[field] = []string{}
			}
			raw, _ = json.Marshal(object)
			if err = filestore.WriteFile(s.Root, s.path(st.ID), raw); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Read(st.ID); err == nil {
				t.Fatal("legacy config field accepted")
			}
		})
	}
}
