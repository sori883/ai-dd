//go:build unix

package flow

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestEndSensorRejectsNonRegularMaterial(t *testing.T) {
	s := flowStore(t)
	if err := syscall.Mkfifo(filepath.Join(s.Root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() { c := boundaryCollector{store: s}; c.material("pipe"); done <- len(c.failures) > 0 }()
	select {
	case rejected := <-done:
		if !rejected {
			t.Fatal("nonregular material accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("material inspection opened and blocked on FIFO")
	}
}
func TestStartSensorRejectsNonRegularShared(t *testing.T) {
	s, st := boundaryFixture(t)
	name := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/current-analysis.md")
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() { g, err := s.CheckBoundary(st.ID, BoundaryStart); done <- err == nil && g.Status == "fail" }()
	select {
	case rejected := <-done:
		if !rejected {
			t.Fatal("nonregular shared document accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("optional document inspection blocked on FIFO")
	}
}
