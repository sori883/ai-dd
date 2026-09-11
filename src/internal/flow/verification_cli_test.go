package flow

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationCLI(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("hash")
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != 6 {
		t.Errorf("schema=%d, want 6", st.SchemaVersion)
	}
	st.Config.VerificationPaths = []string{"src", "config"}
	st.Config.Units = []Unit{{ID: "a", StepID: st.CurrentStepID, Status: "pending", VerificationPaths: []string{"src/a"}}}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Hash(st.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.IntentID != st.ID || got.StepID != st.CurrentStepID || len(got.SHA256) != 64 || len(got.Missing) != 2 {
		t.Fatalf("hash view %+v", got)
	}
	before, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if _, err = s.Hash(st.ID, "a", ""); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if string(before) != string(after) {
		t.Fatal("hash mutated state")
	}
	if _, err = s.Hash(st.ID, "unknown", ""); err == nil {
		t.Fatal("accepted unknown Unit")
	}
	if _, err = s.Hash(st.ID, "a", t.TempDir()); err == nil {
		t.Fatal("accepted unregistered root")
	}
	for _, p := range []string{"outside", "../escape", "aidlc/results", "src/.git"} {
		copy := st
		copy.Config.Units = append([]Unit{}, st.Config.Units...)
		copy.Config.Units[0].VerificationPaths = []string{p}
		if _, err = s.Save(copy, st.Revision); err == nil {
			t.Fatalf("accepted invalid Unit paths %s", p)
		}
	}
	raw, _ := json.Marshal(st)
	for _, field := range []string{"code_revision", "direct_commit", "base_commit", "result_commit", "integrated_commit"} {
		if strings.Contains(string(raw), `"`+field+`"`) {
			t.Errorf("old field in new schema: %s", field)
		}
	}
}

func TestVerificationCLIAssignmentSchema(t *testing.T) {
	r, err := (assignment.Store{Root: t.TempDir()}).Init(assignment.InitRequest{RequestID: "schema", HumanConfirmed: true, Reason: "new schema"})
	if err != nil {
		t.Fatal(err)
	}
	if r.SchemaVersion != 2 {
		t.Fatalf("assignment schema=%d, want 2", r.SchemaVersion)
	}
}
