package flow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerificationGates(t *testing.T) {
	for _, change := range []string{"none", "mtime", "state", "code", "scope", "document"} {
		t.Run(change, func(t *testing.T) {
			s, st := sensorFixture(t)
			st.Config.VerificationPaths = []string{"code"}
			var err error
			st, err = s.Save(st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.RemoveAll(filepath.Join(s.Root, ".git")); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("code"), 0644); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != "pass" {
				t.Fatalf("gitless Sensor %+v %v", gate, err)
			}
			req := ReviewRequest{Action: "assign", CoordinatorSession: "main", Session: "reviewer", Root: s.Root}
			st, err = s.Review(st.ID, st.Revision, req)
			if err != nil {
				t.Fatalf("same-root independent review: %v", err)
			}
			switch change {
			case "mtime":
				if err = os.Chtimes(filepath.Join(s.Root, "code"), time.Unix(5, 0), time.Unix(5, 0)); err != nil {
					t.Fatal(err)
				}
			case "state":
				st, err = s.Save(st, st.Revision)
				if err != nil {
					t.Fatal(err)
				}
			case "code":
				if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("different"), 0644); err != nil {
					t.Fatal(err)
				}
			case "scope":
				st.Config.VerificationPaths = append(st.Config.VerificationPaths, "extra")
				st, err = s.Save(st, st.Revision)
				if err != nil {
					t.Fatal(err)
				}
			case "document":
				p := filepath.Join(s.Root, st.Config.Artifacts[0].Path)
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(p, append(raw, []byte("\nChanged.\n")...), 0644); err != nil {
					t.Fatal(err)
				}
			}
			req.Action = "accept"
			req.Target = gate.Target
			req.Status = "pass"
			req.Summary = "reviewed"
			accepted, err := s.Review(st.ID, st.Revision, req)
			stable := change == "none" || change == "mtime" || change == "state"
			if stable {
				if err != nil {
					t.Fatal(err)
				}
				current, err := s.Check(accepted.ID)
				if err != nil || current.Target != gate.Target {
					t.Fatalf("review/state changed Target %+v %v", current, err)
				}
			} else if err == nil {
				t.Fatal("stale review accepted")
			}
		})
	}
}
func TestVerificationGatesIndependence(t *testing.T) {
	s, st := sensorFixture(t)
	if _, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", CoordinatorSession: "same", Session: "same", Root: s.Root}); err == nil {
		t.Fatal("same session reviewer accepted")
	}
}
