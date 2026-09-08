package okfmemory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlowADRMetadataSearch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ADR"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("---\ntype: ADR\ntitle: Choose store\ndescription: Why atomic replacement\ntags: [storage]\nstatus: stable\ngenerated:\n  by: process:test\n  at: '2026-09-08T00:00:00Z'\nintent_id: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\ncustom: preserved\n---\n## Decision\nUse atomic replacement.\n")
	if err := WriteFile(root, "ADR/store.md", raw); err != nil {
		t.Fatal(err)
	}
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	docs, err := Search(root, "atomic storage", &id)
	if err != nil || len(docs) != 1 {
		t.Fatalf("search %+v %v", docs, err)
	}
	encoded, err := docs[0].Bytes()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(encoded)
	if err != nil || doc.String("custom") != "preserved" || doc.String("status") != "stable" {
		t.Fatalf("metadata %+v %v", doc, err)
	}
}
