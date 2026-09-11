package assignment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Run("explicit initialization and aliases", func(t *testing.T) {
		root := t.TempDir()
		s := Store{Root: root}
		if _, err := s.Read(); err == nil {
			t.Fatal("missing registry accepted")
		}
		r, err := s.Init(InitRequest{RequestID: "init-1", HumanConfirmed: true, Reason: "known work stopped and collected"})
		if err != nil {
			t.Fatalf("explicit init: %v", err)
		}
		if r.SchemaVersion != 2 || r.Epoch == "" || r.Revision != 1 {
			t.Fatalf("invalid initialized registry: %+v", r)
		}
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		got, err := (Store{Root: alias}).Read()
		if err != nil || got.Epoch != r.Epoch {
			t.Fatalf("alias read: %+v %v", got, err)
		}
		if _, err := s.Init(InitRequest{RequestID: "other", HumanConfirmed: true, Reason: "confirmed"}); err == nil {
			t.Fatal("reinitialization accepted")
		}
	})
	t.Run("confirmation required", func(t *testing.T) {
		if _, err := (Store{Root: t.TempDir()}).Init(InitRequest{RequestID: "init", Reason: "unconfirmed"}); err == nil {
			t.Fatal("unconfirmed init accepted")
		}
	})
	t.Run("save failure", func(t *testing.T) {
		s := Store{Root: t.TempDir(), write: func(string, string, []byte) error { return errors.New("disk failure") }}
		if _, err := s.Init(InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"}); err == nil {
			t.Fatal("failed write accepted")
		}
	})
	for _, tc := range []struct{ name, raw string }{
		{"broken", `{`}, {"unknown version", `{"schema_version":2}`}, {"unknown field", `{"schema_version":1,"unexpected":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, registryPath)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := (Store{Root: root}).Read(); err == nil {
				t.Fatal("invalid registry accepted")
			}
		})
	}
	t.Run("root binding", func(t *testing.T) {
		s := Store{Root: t.TempDir()}
		r, err := s.Init(InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
		if err != nil {
			t.Fatal(err)
		}
		r.Root = t.TempDir()
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(s.Root, registryPath), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Read(); err == nil {
			t.Fatal("different coordinator accepted")
		}
	})
}

func TestRegistryRejectsDamagedRecords(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	if _, err := s.Reserve(reserveRequest(r, a)); err != nil {
		t.Fatal(err)
	}
	reg, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Registry)
	}{
		{"missing root", func(r *Registry) { r.Reservations[0].Root = "" }},
		{"unknown state", func(r *Registry) { r.Reservations[0].Status = "finished" }},
		{"duplicate id", func(r *Registry) { r.Reservations = append(r.Reservations, r.Reservations[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original, _ := json.Marshal(reg)
			var bad Registry
			if err := json.Unmarshal(original, &bad); err != nil {
				t.Fatal(err)
			}
			tc.change(&bad)
			raw, _ := json.Marshal(bad)
			if err := os.WriteFile(filepath.Join(s.Root, registryPath), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(); err == nil {
				t.Fatal("damaged records accepted")
			}
		})
	}
}
