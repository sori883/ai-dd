//go:build unix

package flow

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestDocumentRegistrationNonRegular(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s, st := boundaryFixture(t)
			doc := declaredDoc("discovery", "adr/decision", "adr")
			name := filepath.Join(s.Root, doc.Path)
			if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "fifo":
				err = syscall.Mkfifo(name, 0600)
			case "symlink":
				err = os.Symlink("missing", name)
			case "directory":
				err = os.Mkdir(name, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{doc}})
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("nonregular document accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("registration blocked on nonregular file")
			}
			after, err := s.Read(st.ID)
			if err != nil || after.Revision != st.Revision {
				t.Fatal("rejected registration changed state")
			}
			if err = os.Remove(name); err != nil {
				t.Fatal(err)
			}
			if _, err = s.SetDocuments(st.ID, st.Revision, IntentDocuments{Inputs: []DocumentDeclaration{}, Outputs: []DocumentDeclaration{doc}}); err != nil {
				t.Fatalf("lock retained or missing output rejected: %v", err)
			}
		})
	}
}
