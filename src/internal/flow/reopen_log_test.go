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
	st, err := createExecutionFixture(t, s, "bound")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Root, "aidlc/workflow/stages/tdd.md")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, append(raw, []byte("changed\n")...), 0644)
	if _, err = saveExecutionFixture(t, s, st, st.Revision); err == nil {
		t.Error("configure accepted definition drift")
	}
	if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "pause"}); err == nil {
		t.Error("mutation accepted definition drift")
	}
	if _, err = s.Begin(st.ID, st.Revision); err == nil {
		t.Error("begin accepted definition drift")
	}
	if _, err = s.Read(st.ID); err != nil {
		t.Fatal("diagnostic read unavailable", err)
	}
	os.WriteFile(p, raw, 0644)
	if _, err = saveExecutionFixture(t, s, st, st.Revision); err != nil {
		t.Fatal("restored definition rejected", err)
	}
}

func TestDefinitionBindingMalformedState(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "schema")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := filestore.ReadFile(s.Root, s.path(st.ID))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ name, old, new string }{
		{"missing hash", st.DefinitionHash, ""},
		{"forged pending", `"entry": null`, `"pending_reopen":{"revision":99,"from":"other","to":"discovery","reason":"x","at":"bad","log_hash":"bad"},"entry": null`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if !strings.Contains(string(raw), bad.old) {
				t.Fatal("fixture mutation target missing")
			}
			if err := filestore.WriteFile(s.Root, s.path(st.ID), []byte(strings.Replace(string(raw), bad.old, bad.new, 1))); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(st.ID); err == nil {
				t.Fatal("malformed binding accepted")
			}
		})
	}
}

func TestReopenLogFreshDeletionRequiresRestore(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "fresh log")
	if err != nil {
		t.Fatal(err)
	}
	r := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"}
	st = prepareReopenFixture(t, s, st, r)
	s.write = func(root, name string, raw []byte) error {
		if strings.HasSuffix(name, "state.json") && !strings.Contains(string(raw), `"pending_reopen"`) {
			return errors.New("final failed")
		}
		return filestore.WriteFile(root, name, raw)
	}
	if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, r); err == nil {
		t.Fatal("final failed but returned success")
	}
	s.write = nil
	name := workLogPath(s, st.ID)
	if err = os.Remove(filepath.Join(s.Root, name)); err != nil {
		t.Fatal(err)
	}
	if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, r); err == nil {
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
		{"fits", filestore.MaxBytes - 1024, "reconsider", true},
		{"fresh encoded reason", -1, strings.Repeat("\x00", filestore.MaxBytes/4), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := flowStore(t)
			st, err := createExecutionFixture(t, s, "capacity")
			if err != nil {
				t.Fatal(err)
			}
			name := workLogPath(s, st.ID)
			original := []byte{}
			if tc.size >= 0 {
				base := seedWorkLog(t, s, st, "")
				original = seedWorkLog(t, s, st, strings.Repeat("a", tc.size-len(base)))
			}
			if tc.size >= 0 {
				st = prepareReopenFixture(t, s, st, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: tc.reason})
			}
			stateBefore, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil {
				t.Fatal(err)
			}
			_, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: tc.reason})
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
