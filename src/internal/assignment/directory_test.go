package assignment

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDirectoryAssignment(t *testing.T) {
	s := Store{Root: t.TempDir()}
	r, err := s.Init(InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "new project"})
	if err != nil {
		t.Fatal(err)
	}
	req := reserveRequest(r, s.Root)
	v, err := s.Reserve(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(s.Root, "app")
	if err = os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err = os.Symlink(s.Root, alias); err != nil {
		t.Fatal(err)
	}
	for i, p := range []string{s.Root, child, filepath.Dir(s.Root), alias} {
		other := req
		other.Root = p
		other.RequestID = string(rune('a' + i))
		if _, err = s.Reserve(other); err == nil {
			t.Fatalf("accepted conflicting root %s", p)
		}
	}
	other := req
	other.RequestID = "independent"
	other.Root = t.TempDir()
	if _, err = s.Reserve(other); err != nil {
		t.Fatal(err)
	}
	again, err := s.Reserve(req)
	if err != nil || again.ID != v.ID {
		t.Fatalf("retry %+v %v", again, err)
	}
}
func TestDirectoryAssignmentConcurrentAndSaveFailure(t *testing.T) {
	s := Store{Root: t.TempDir()}
	r, err := s.Init(InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "new project"})
	if err != nil {
		t.Fatal(err)
	}
	req := reserveRequest(r, s.Root)
	fail := s
	fail.write = func(string, string, []byte) error { return fs.ErrPermission }
	if _, err := fail.Reserve(req); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("save failure %v", err)
	}
	ready := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{"one", "two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ready
			other := req
			other.RequestID = id
			_, err := s.Reserve(other)
			results <- err
		}()
	}
	close(ready)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes %d, want 1", success)
	}
}
