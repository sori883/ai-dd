//go:build integration

package state

import (
	"bytes"
	"os"
	"testing"
)

func TestReadDocumentReturnsValidatedStateAndOwnedRawBytes(t *testing.T) {
	t.Parallel()

	content := []byte("\ufeff" + withRuntimeState(canonicalStateContent(), "7") + "\n## Unknown\nkeep me\n")
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if err := root.WriteFile(stateFile, content, 0o600); err != nil {
		t.Fatal(err)
	}

	document, err := ReadDocument(root)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if document.State.Scope() != "classic" {
		t.Errorf("document state scope = %q, want classic", document.State.Scope())
	}
	if !bytes.Equal(document.Content, content) {
		t.Fatalf("document content = %q, want original bytes %q", document.Content, content)
	}
	if got, err := document.RevisionCount(); err != nil || got != 7 {
		t.Fatalf("Document.RevisionCount() = (%d, %v), want (7, nil)", got, err)
	}
	if got, err := document.LastUpdated(); err != nil || got != "2026-09-02T00:00:00Z" {
		t.Fatalf("Document.LastUpdated() = (%q, %v), want canonical timestamp", got, err)
	}

	document.Content[0] = 'x'
	document.Content[len(document.Content)-1] = 'z'
	if document.State.Scope() != "classic" || document.State.Stages()[0].Slug != "workspace-scaffold" {
		t.Fatal("mutating document content changed validated State")
	}
	if _, err := root.Stat(stateFile); err != nil {
		t.Fatalf("ReadDocument() closed caller root: %v", err)
	}
}
