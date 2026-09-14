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
	for _, tc := range []struct{ name, from, to string }{
		{"duplicate key", `"schema_version": 6`, `"schema_version": 6, "schema_version": 6`},
		{"unknown field", `"schema_version": 6`, `"schema_version": 6, "unexpected": true`},
		{"unsupported schema", `"schema_version": 6`, `"schema_version": 5`},
		{"trailing JSON", "", ""},
		{"identity", "", strings.Repeat("a", 32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			if tc.name == "identity" {
				tc.from = st.ID
			}
			bad := strings.Replace(string(raw), tc.from, tc.to, 1)
			if tc.name == "trailing JSON" {
				bad = string(raw) + `{}`
			}
			name := filepath.Join(s.Root, s.path(st.ID))
			if err := os.WriteFile(name, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(st.ID); err == nil {
				t.Fatal("corrupt state accepted")
			}
			after, err := os.ReadFile(name)
			if err != nil || string(after) != bad {
				t.Fatal("rejected state changed", err)
			}
		})
	}
}
