package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlowReviewIdentityTarget(t *testing.T) {
	s, st := sensorFixture(t)
	reviewRoot := flowReviewRoot(t, s.Root)
	assign := ReviewRequest{Action: "assign", CoordinatorSession: "coordinator", Session: "reviewer", Root: reviewRoot}
	for _, bad := range []ReviewRequest{{Action: "assign", CoordinatorSession: "same", Session: "same", Root: reviewRoot}} {
		if _, err := s.Review(st.ID, st.Revision, bad); err == nil {
			t.Fatal("nonindependent review accepted")
		}
	}
	st, err := s.Review(st.ID, st.Revision, assign)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	accept := ReviewRequest{Action: "accept", Session: "reviewer", Root: reviewRoot, Target: gate.Target, Status: "pass", Summary: "Reviewed current behavior and ADR reason"}
	bad := accept
	bad.Session = "other"
	if _, err := s.Review(st.ID, st.Revision, bad); err == nil {
		t.Fatal("unassigned session accepted")
	}
	st, err = s.Review(st.ID, st.Revision, accept)
	if err != nil || st.Review.Status != "pass" || st.Review.Target != gate.Target {
		t.Fatalf("valid review %+v %v", st, err)
	}
	if _, err := s.Review(st.ID, st.Revision, accept); err == nil {
		t.Fatal("review assignment replay accepted")
	}
	st, err = s.Review(st.ID, st.Revision, assign)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, "changed.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Review(st.ID, st.Revision, accept); err == nil {
		t.Fatal("stale review accepted")
	}
}
func TestFlowReviewFailRecorded(t *testing.T) {
	s, st := sensorFixture(t)
	root := flowReviewRoot(t, s.Root)
	st, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", CoordinatorSession: "c", Session: "r", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Session: "r", Root: root, Target: gate.Target, Status: "fail", Summary: "Acceptance unclear"})
	if err != nil || st.Review.Status != "fail" {
		t.Fatalf("failure not recorded: %+v %v", st, err)
	}
}
func TestFlowReviewRejectsWrongCheckout(t *testing.T) {
	s, st := sensorFixture(t)
	boundaryFile(t, s, "code", "content absent from other root")
	r := ReviewRequest{Action: "assign", CoordinatorSession: "c", Session: "r", Root: t.TempDir()}
	if _, err := s.Review(st.ID, st.Revision, r); err == nil {
		t.Fatal("empty directory accepted as reviewer checkout")
	}
}

func flowReviewRoot(t *testing.T, project string) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if entry.IsDir() {
			if name == ".git" || name == "aidlc" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(root, name), 0700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, name), raw, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}

	return root
}
func TestFlowReviewRejectsChangedCheckout(t *testing.T) {
	s, st := sensorFixture(t)
	root := flowReviewRoot(t, s.Root)
	st, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", CoordinatorSession: "c", Session: "r", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "different.go"), []byte("package different\n"), 0600)
	if _, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "accept", Session: "r", Root: root, Target: gate.Target, Status: "pass", Summary: "wrong checkout"}); err == nil {
		t.Fatal("changed reviewer bytes accepted")
	}
}
