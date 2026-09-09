package flow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundaryStoreSchema(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("New")
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != 4 {
		t.Fatalf("schema=%d want 4", st.SchemaVersion)
	}
	st.Config.NoMaterialsReason = "new project"
	next, err := s.Save(st, st.Revision)
	if err != nil || next.Revision != 2 {
		t.Fatalf("CAS: %+v %v", next, err)
	}
	if _, err = s.Save(st, st.Revision); err == nil {
		t.Fatal("stale save accepted")
	}
	st.SchemaVersion = 1
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(s.Root, s.path(st.ID))
	if err = os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(st.ID); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("legacy schema: %v", err)
	}
	after, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("legacy state changed")
	}
}
func TestBoundaryStoreVersions(t *testing.T) {
	for _, entry := range []*StageEntry{
		{Stage: "other"}, {Stage: "planning"}, {Stage: "discovery", Inputs: []FileVersion{{Path: "../escape", SHA256: strings.Repeat("a", 64)}}},
		{Stage: "discovery", Inputs: []FileVersion{{Path: "file", SHA256: "bad"}}},
		{Stage: "discovery", Sources: []FileVersion{{Path: "aidlc/.runtime/x", SHA256: strings.Repeat("a", 64)}}},
		{Stage: "discovery", Inputs: []FileVersion{{Path: "file", SHA256: strings.Repeat("a", 64)}, {Path: "file", SHA256: strings.Repeat("a", 64)}}},
	} {
		s := flowStore(t)
		st, err := s.Create("New")
		if err != nil {
			t.Fatal(err)
		}
		st.Entry = entry
		if err = s.persist(st); err == nil {
			t.Errorf("invalid entry accepted: %+v", entry)
		}
	}
	s := flowStore(t)
	st, err := s.Create("New")
	if err != nil {
		t.Fatal(err)
	}
	st.Accepted = map[string]StageAcceptance{"bogus": {Stage: "bogus", ReviewTarget: strings.Repeat("b", 64)}}
	if err = s.persist(st); err == nil {
		t.Fatal("unknown acceptance accepted")
	}
}
