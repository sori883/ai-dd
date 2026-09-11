package flow

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func flowStore(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "aidlc/spaces/default"), 0700); err != nil {
		t.Fatal(err)
	}
	s := Store{Root: root, Space: "default"}
	deployFlowDefinition(t, s)
	return s
}
func TestFlowStoreCreateCAS(t *testing.T) {
	s := flowStore(t)
	state, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.ID) != 32 || state.Revision != 1 || state.Stage != "initialization" || state.Status != "active" {
		t.Fatalf("initial state: %+v", state)
	}
	state.Config.Objective = "Objective"
	next, err := s.Save(state, 1)
	if err != nil || next.Revision != 2 {
		t.Fatalf("save: %+v %v", next, err)
	}
	if _, err := s.Save(state, 1); err == nil {
		t.Fatal("stale revision accepted")
	}
	got, err := s.Read(state.ID)
	if err != nil || got.Config.Objective != "Objective" {
		t.Fatalf("read: %+v %v", got, err)
	}
}
func TestFlowStoreRejectsCorruptAndIsolates(t *testing.T) {
	s := flowStore(t)
	for _, id := range []string{"../escape", "", "ABC"} {
		if _, err := s.Read(id); err == nil {
			t.Errorf("accepted ID %q", id)
		}
	}
	state, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	other := s
	other.Space = "other"
	if _, err := other.Read(state.ID); err == nil {
		t.Fatal("read another Space")
	}
	path := filepath.Join(s.Root, "aidlc/spaces/default/intents", state.ID, "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"schema_version":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(state.ID); err == nil {
		t.Fatal("accepted corrupt state")
	}
}
func TestFlowStoreFailurePreservesState(t *testing.T) {
	s := flowStore(t)
	state, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	s.write = func(string, string, []byte) error { return fs.ErrPermission }
	state.Name = "Changed"
	if _, err := s.Save(state, 1); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("save error %v", err)
	}
	got, err := s.Read(state.ID)
	if err != nil || got.Name != "Work" || got.Revision != 1 {
		t.Fatalf("lost original: %+v %v", got, err)
	}
}
func TestFlowStoreNames(t *testing.T) {
	s := flowStore(t)
	if _, err := s.Create(" "); err == nil {
		t.Fatal("accepted empty name")
	}
	one, err := s.Create("Same")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := s.Resolve("Same"); err != nil || id != one.ID {
		t.Fatalf("resolve %s %v", id, err)
	}
	if _, err := s.Create("Same"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve("Same"); err == nil {
		t.Fatal("ambiguous name accepted")
	}
}
func TestFlowStoreWireSchema(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["schema_version"] != float64(6) || fields["id"] != st.ID {
		t.Fatalf("noncanonical JSON: %s", raw)
	}
	for _, bad := range []string{strings.Replace(string(raw), `"schema_version": 6`, `"schema_version": 6, "schema_version": 6`, 1), strings.Replace(string(raw), `"schema_version": 6`, `"schema_version": 6, "unexpected": true`, 1), string(raw) + `{}`, strings.Replace(string(raw), st.ID, strings.Repeat("a", 32), 1)} {
		if err := os.WriteFile(filepath.Join(s.Root, s.path(st.ID)), []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Read(st.ID); err == nil {
			t.Fatalf("accepted corrupt state: %s", bad)
		}
	}
}
