package assignment

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

func TestAssignmentRecovery(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	req := reserveRequest(r, a)
	v, err := s.Reserve(req)
	if err != nil {
		t.Fatal(err)
	}
	release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "known work stopped and collected"}
	bad := release
	bad.NoMoreRequests = false
	if _, err := s.Release(v.ID, "main", v.EntryRevision, bad); err == nil {
		t.Fatal("unconfirmed release accepted")
	}
	if _, err := s.Release(v.ID, "other", v.EntryRevision, release); err == nil {
		t.Fatal("other owner release accepted")
	}
	broken := s
	broken.write = func(string, string, []byte) error { return errors.New("disk failed") }
	if _, err := broken.Release(v.ID, "main", v.EntryRevision, release); err == nil {
		t.Fatal("save failure hidden")
	}
	got, err := s.Release(v.ID, "main", v.EntryRevision, release)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := s.Release(v.ID, "main", v.EntryRevision, release)
	if err != nil || retry.EntryRevision != got.EntryRevision {
		t.Fatalf("release response loss retry: %+v %v", retry, err)
	}
	bad = release
	bad.Reason = "changed"
	if _, err := s.Release(v.ID, "main", v.EntryRevision, bad); err == nil {
		t.Fatal("release request id conflict accepted")
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, registryPath))
	if err != nil {
		t.Fatal(err)
	}
	reset := ResetRequest{RequestID: "reset", RegistryEpoch: r.Epoch, RegistryHash: filestore.Hash(raw), HumanConfirmed: true, Reason: "human confirmed all known work stopped and collected"}
	newer, err := s.Reset(reset)
	if err != nil {
		t.Fatal(err)
	}
	if newer.Epoch == r.Epoch {
		t.Fatal("reset reused epoch")
	}
	if _, err := s.Reserve(req); err == nil {
		t.Fatal("old epoch accepted")
	}
	same, err := s.Reset(reset)
	if err != nil || same.Epoch != newer.Epoch {
		t.Fatalf("reset response loss: %+v %v", same, err)
	}
}
func TestAssignmentRecoveryCapacity(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	v, err := s.Reserve(reserveRequest(r, a))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		_, err := s.PreSpawn(DispatchRequest{Session: "parent", Turn: "turn", ToolID: "tool-" + strconv.Itoa(i), TaskName: "task_" + strconv.Itoa(i), Agent: "aidlc-reviewer", Space: "default", IntentID: v.IntentID, StepID: v.StepID, DefinitionHash: v.DefinitionHash})
		if err != nil {
			break
		}
		if i == 999 {
			t.Fatal("capacity never reached")
		}
	}
	release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "all results collected and processes stopped"}
	if _, err := s.Release(v.ID, "main", v.EntryRevision, release); err != nil {
		t.Fatalf("capacity prevented release: %v", err)
	}
	// Every admitted pending response has reserved room for the longest accepted target.
	reg, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range reg.Dispatches {
		prefix := "/root/"
		for len(prefix)+len(d.TaskName) < 500 {
			prefix += "a/"
		}
		raw, _ := json.Marshal(map[string]string{"task_name": prefix + d.TaskName})
		if _, err := s.PostSpawn(d.Session, d.ToolID, raw); err != nil {
			t.Fatalf("capacity prevented post: %v", err)
		}
	}
}
func TestAssignmentRecoveryMissing(t *testing.T) {
	s := Store{Root: t.TempDir()}
	req := ResetRequest{RequestID: "reset", Diagnosis: "missing", HumanConfirmed: true, Reason: "human confirmed lost registry cannot be restored and known work stopped"}
	bad := req
	bad.HumanConfirmed = false
	if _, err := s.Reset(bad); err == nil {
		t.Fatal("reset without human confirmation")
	}
	if _, err := s.Reset(req); err != nil {
		t.Fatal(err)
	}
}

func TestAssignmentRecoveryInitRetry(t *testing.T) {
	s := Store{Root: t.TempDir()}
	req := InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "human confirmed known work stopped"}
	first, err := s.Init(req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Init(req)
	if err != nil || again.Epoch != first.Epoch {
		t.Fatalf("init response loss: %+v %v", again, err)
	}
}
func TestAssignmentRecoveryEscapedReleaseCapacity(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	for i := 0; i < 100; i++ {
		req := reserveRequest(r, a)
		req.RequestID = "request-" + strconv.Itoa(i)
		v, err := s.Reserve(req)
		if err != nil {
			return
		}
		release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release-" + strconv.Itoa(i), PreviousRunStopped: true, NoMoreRequests: true, Reason: strings.Repeat("\x01", 2048)}
		if _, err := s.Release(v.ID, "main", v.EntryRevision, release); err != nil {
			t.Fatalf("accepted reservation cannot save valid escaped confirmation: %v", err)
		}
	}
	t.Fatal("new admissions did not stop at capacity")
}
