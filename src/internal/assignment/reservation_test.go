package assignment

import (
	"os"
	"os/exec"
	"testing"
)

func registryFixture(t *testing.T) (Store, Registry, string, string) {
	t.Helper()
	root := t.TempDir()
	a, b := t.TempDir(), t.TempDir()
	s := Store{Root: root}
	r, err := s.Init(InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	return s, r, a, b
}
func reserveRequest(r Registry, root string) ReserveRequest {
	return ReserveRequest{RegistryEpoch: r.Epoch, RequestID: "request-1", CoordinatorSession: "main", Session: "worker", Space: "default", IntentID: "11111111111111111111111111111111", StepID: "s01", DefinitionHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Root: root, Agent: "aidlc-worker", SourceRevision: 1}
}
func TestReservation(t *testing.T) {
	s, r, a, b := registryFixture(t)
	req := reserveRequest(r, a)
	first, err := s.Reserve(req)
	if err != nil {
		t.Fatalf("reserve first: %v", err)
	}
	changed := req
	changed.Root = b
	if _, err := s.Reserve(changed); err == nil {
		t.Fatal("changed request id accepted")
	}
	for _, field := range []string{"space", "intent", "session"} {
		t.Run(field, func(t *testing.T) {
			other := req
			other.RequestID = "other-" + field
			switch field {
			case "space":
				other.Space = "other"
			case "intent":
				other.IntentID = "22222222222222222222222222222222"
			case "session":
				other.CoordinatorSession = "other"

			}
			if _, err := s.Reserve(other); err == nil {
				t.Fatal("occupied root accepted")
			}
		})
	}
	release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release-1", PreviousRunStopped: true, NoMoreRequests: true, Reason: "results collected, no pending processes"}
	if _, err := s.Release(first.ID, "main", first.EntryRevision, release); err != nil {
		t.Fatal(err)
	}
	retry, err := s.Reserve(req)
	if err != nil || retry.ID != first.ID || retry.Status != "released" {
		t.Fatalf("released retry resurrected: %+v %v", retry, err)
	}
	fresh := req
	fresh.RequestID = "fresh"
	if _, err := s.Reserve(fresh); err != nil {
		t.Fatalf("released root: %v", err)
	}
}
func TestReservationProcess(t *testing.T) {
	if os.Getenv("AIDLC_RESERVATION_CHILD") == "1" {
		s := Store{Root: os.Getenv("AIDLC_RESERVATION_ROOT")}
		r, err := s.Read()
		if err != nil {
			t.Fatal(err)
		}
		req := reserveRequest(r, os.Getenv("AIDLC_RESERVATION_WORKER"))
		req.RequestID = os.Getenv("AIDLC_RESERVATION_REQUEST")
		req.Space = req.RequestID
		req.CoordinatorSession = req.RequestID
		if _, err := s.Reserve(req); err != nil {
			os.Exit(3)
		}
		return
	}
	s, _, a, _ := registryFixture(t)
	commands := make([]*exec.Cmd, 2)
	for i, id := range []string{"one", "two"} {
		c := exec.Command(os.Args[0], "-test.run=^TestReservationProcess$")
		c.Env = append(os.Environ(), "AIDLC_RESERVATION_CHILD=1", "AIDLC_RESERVATION_ROOT="+s.Root, "AIDLC_RESERVATION_WORKER="+a, "AIDLC_RESERVATION_REQUEST="+id)
		commands[i] = c
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
	}
	successes := 0
	for _, c := range commands {
		if c.Wait() == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful separate processes=%d, want 1", successes)
	}
}

func TestReservationRequestIDAcrossOperations(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	req := reserveRequest(r, a)
	v, err := s.Reserve(req)
	if err != nil {
		t.Fatal(err)
	}
	release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: req.RequestID, PreviousRunStopped: true, NoMoreRequests: true, Reason: "known work stopped and collected"}
	if _, err := s.Release(v.ID, "main", v.EntryRevision, release); err == nil {
		t.Fatal("reserve request id reused for a different operation")
	}
	release.RequestID = "release"
	if _, err := s.Release(v.ID, "main", v.EntryRevision, release); err != nil {
		t.Fatal(err)
	}
	req.RequestID = "release"
	if _, err := s.Reserve(req); err == nil {
		t.Fatal("release request id reused for reserve")
	}
}

func TestReservationReplacementSaveFailure(t *testing.T) {
	s, r, a, b := registryFixture(t)
	req := reserveRequest(r, a)
	old, err := s.Reserve(req)
	if err != nil {
		t.Fatal(err)
	}
	next := req
	next.RequestID, next.Root = "replacement", b
	release := ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "replace-release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "collected"}
	s.write = func(string, string, []byte) error { return os.ErrPermission }
	if _, err := s.Replace(old.ID, old.EntryRevision, release, next); err == nil {
		t.Fatal("save failure accepted")
	}
	got, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reservations) != 1 || got.Reservations[0].Status != "reserved" || got.Reservations[0].Release != nil {
		t.Fatalf("failed replacement changed registry: %+v", got.Reservations)
	}
	s.write = nil
	replaced, err := s.Replace(old.ID, old.EntryRevision, release, next)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Replace(old.ID, old.EntryRevision, release, next)
	if err != nil || again.ID != replaced.ID {
		t.Fatalf("replacement retry: %+v %v", again, err)
	}
}
