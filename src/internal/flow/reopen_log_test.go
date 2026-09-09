package flow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

func TestDefinitionBindingDrift(t *testing.T) {
	s := flowStore(t)
	deployFlowDefinition(t, s)
	st, err := s.Create("bound")
	if err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != 3 {
		t.Errorf("schema=%d want 3", st.SchemaVersion)
	}
	p := filepath.Join(s.Root, "aidlc/workflow/stages/tdd.md")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, append(raw, []byte("changed\n")...), 0644)
	if _, err = s.Save(st, st.Revision); err == nil {
		t.Error("configure accepted definition drift")
	}
	if _, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "pause"}); err == nil {
		t.Error("mutation accepted definition drift")
	}
	if _, err = s.Begin(st.ID, st.Revision); err == nil {
		t.Error("begin accepted definition drift")
	}
	if _, err = s.Read(st.ID); err != nil {
		t.Fatal("diagnostic read unavailable", err)
	}
	os.WriteFile(p, raw, 0644)
	if _, err = s.Save(st, st.Revision); err != nil {
		t.Fatal("restored definition rejected", err)
	}
}
func TestReopenLogFailures(t *testing.T) {
	for _, point := range []string{"pending", "log", "final"} {
		t.Run(point, func(t *testing.T) {
			s := flowStore(t)
			deployFlowDefinition(t, s)
			st, err := s.Create("log")
			if err != nil {
				t.Fatal(err)
			}
			logPath := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
			if err = filestore.WriteFile(s.Root, logPath, []byte("Existing note.\n")); err != nil {
				t.Fatal(err)
			}
			count := 0
			s.write = func(root, name string, raw []byte) error {
				count++
				if point == "pending" && count == 1 || point == "log" && strings.HasSuffix(name, "work-log.md") || point == "final" && count == 3 {
					return errors.New("injected save failure")
				}
				return filestore.WriteFile(root, name, raw)
			}
			request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "line one\n## forged marker"}
			if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
				t.Fatal("partial save returned success")
			}
			got, err := s.Read(st.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Revision != st.Revision {
				t.Fatal("partial save changed revision")
			}
			s.write = nil
			if point != "pending" {
				if _, err = s.Save(got, got.Revision); err == nil {
					t.Fatal("pending allowed configure")
				}
				if _, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "other"}); err == nil {
					t.Fatal("pending allowed different operation")
				}
				if err = s.CheckWork(st.ID); err == nil {
					t.Fatal("pending allowed work")
				}
			}
			got, err = s.Transition(st.ID, st.Revision, request)
			if err != nil {
				t.Fatal(err)
			}
			if got.Revision != st.Revision+1 {
				t.Fatal("retry revision mismatch")
			}
			raw, err := filestore.ReadFile(s.Root, logPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(raw), "Existing note.\n") || strings.Count(string(raw), "forged marker") != 1 || strings.Contains(string(raw), "\n## forged marker") {
				t.Fatalf("history or escaped reason corrupted: %s", raw)
			}
			if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
				t.Fatal("old expect accepted")
			}
		})
	}
}

func TestReopenLogChangedHistoryRejected(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("log conflict")
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
	filestore.WriteFile(s.Root, name, []byte("history\n"))
	count := 0
	s.write = func(root, path string, raw []byte) error {
		count++
		if count == 3 {
			return errors.New("final save failure")
		}
		return filestore.WriteFile(root, path, raw)
	}
	request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"}
	if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
		t.Fatal("failure hidden")
	}
	s.write = nil
	for _, mode := range []string{"changed", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "changed" {
				filestore.WriteFile(s.Root, name, []byte("different history\n"))
			} else {
				os.Remove(filepath.Join(s.Root, name))
			}
			if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
				t.Fatal("corrupt history accepted")
			}
		})
	}
}

func TestDefinitionBindingMalformedState(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("schema")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := filestore.ReadFile(s.Root, s.path(st.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ name, old, new string }{
		{"missing hash", st.DefinitionHash, ""},
		{"old schema", `"schema_version": 3`, `"schema_version": 2`},
		{"forged pending", `"entry": null`, `"pending_reopen":{"revision":99,"from":"other","to":"discovery","reason":"x","at":"bad","log_hash":"bad"},"entry": null`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			filestore.WriteFile(s.Root, s.path(st.ID), []byte(strings.Replace(string(raw), bad.old, bad.new, 1)))
			if _, err := s.Read(st.ID); err == nil {
				t.Fatal("malformed binding accepted")
			}
		})
	}
}

func TestReopenLogFreshDeletionRequiresRestore(t *testing.T) {
	s := flowStore(t)
	st, err := s.Create("fresh log")
	if err != nil {
		t.Fatal(err)
	}
	s.write = func(root, name string, raw []byte) error {
		if strings.HasSuffix(name, "state.json") && !strings.Contains(string(raw), `"pending_reopen"`) {
			return errors.New("final failed")
		}
		return filestore.WriteFile(root, name, raw)
	}
	r := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"}
	if _, err = s.Transition(st.ID, st.Revision, r); err == nil {
		t.Fatal("final failed but returned success")
	}
	s.write = nil
	name := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
	if err = os.Remove(filepath.Join(s.Root, name)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Transition(st.ID, st.Revision, r); err == nil {
		t.Fatal("deleted fresh log silently regenerated")
	}
}

func TestReopenLogCapacity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		size   int
		reason string
		fits   bool
	}{
		{"overflow", filestore.MaxBytes - 10, "reconsider", false},
		{"fits", filestore.MaxBytes - 256, "reconsider", true},
		{"fresh encoded reason", -1, strings.Repeat("\x00", filestore.MaxBytes/4), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := flowStore(t)
			st, err := s.Create("capacity")
			if err != nil {
				t.Fatal(err)
			}
			name := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
			original := []byte{}
			if tc.size >= 0 {
				original = []byte(strings.Repeat("a", tc.size))
				if err = filestore.WriteFile(s.Root, name, original); err != nil {
					t.Fatal(err)
				}
			}
			stateBefore, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: tc.reason})
			if tc.fits {
				if err != nil {
					t.Fatal(err)
				}
				raw, err := filestore.ReadFile(s.Root, name)
				if err != nil || len(raw) > filestore.MaxBytes {
					t.Fatal("successful log unreadable", err)
				}
				return
			}
			if err == nil {
				t.Error("overflow returned success")
			}
			stateAfter, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil || string(stateBefore) != string(stateAfter) {
				t.Error("rejected overflow changed state")
			}
			raw, err := filestore.ReadFile(s.Root, name)
			if tc.size < 0 {
				if !os.IsNotExist(err) {
					t.Error("rejected fresh request created log")
				}
			} else if err != nil || string(raw) != string(original) {
				t.Error("rejected overflow changed existing log")
			}
		})
	}
}
