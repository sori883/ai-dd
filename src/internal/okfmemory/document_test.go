package okfmemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sample = "---\ntype: KDR\nintent_id: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\ntitle: Search\ndescription: Find things\ntags: [search]\nstatus: draft\ngenerated: {by: 'process:test', at: '2026-09-08T00:00:00Z'}\nverified: {by: 'human:reviewer', at: '2026-09-08T00:00:00Z'}\nresource: https://example.test\ncustom: {nested: [one, two]}\n---\n# Body\n"

func TestParseRoundTrip(t *testing.T) {
	doc, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if doc.String("intent_id") != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || doc.String("title") != "Search" {
		t.Fatalf("metadata missing: %+v", doc)
	}
	encoded, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"custom:", "nested:", "verified:", "human:reviewer", "resource:", "# Body"} {
		if !strings.Contains(string(encoded), text) {
			t.Errorf("lost %s: %s", text, encoded)
		}
	}
}
func TestParseRejectsInvalid(t *testing.T) {
	for _, tc := range []struct{ name, body string }{{"no_frontmatter", "body"}, {"empty_type", "---\ntype: ''\n---\n"}, {"duplicate", "---\ntype: KDR\ntype: Rule\n---\n"}, {"utf8", sample + string([]byte{0xff})}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.body)); err == nil {
				t.Fatal("accepted invalid document")
			}
		})
	}
}
func TestSearchIntentID(t *testing.T) {
	root := t.TempDir()
	writeTestDoc(t, root, "kdr/a", sample)
	writeTestDoc(t, root, "knowledge/body", "---\ntype: Note\ntitle: Other\n---\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n")
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	docs, err := Search(root, "search", &id)
	if err != nil || len(docs) != 1 || docs[0].ID != "kdr/a" {
		t.Fatalf("search = %+v, %v", docs, err)
	}
	docs, err = Search(root, "absent", &id)
	if err != nil || len(docs) != 0 {
		t.Fatalf("AND query = %+v, %v", docs, err)
	}
	empty := ""
	if _, err := Search(root, "", &empty); err == nil {
		t.Fatal("accepted explicit empty id")
	}
}
func TestValidatePathsAndBundle(t *testing.T) {
	root := t.TempDir()
	writeTestDoc(t, root, "knowledge/good", "---\ntype: NewType\n---\n[Future](/missing.md)\n")
	if err := Validate(root); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../escape", "/absolute", "index", "knowledge/log", "a.md", "a/../b"} {
		if _, err := Read(root, id); err == nil {
			t.Errorf("accepted %s", id)
		}
	}
	outside := filepath.Join(t.TempDir(), "external.md")
	if err := os.WriteFile(outside, []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err == nil {
		t.Fatal("accepted symlink Concept")
	}
}
func TestBookkeepingIndexAndLog(t *testing.T) {
	root := t.TempDir()
	writeTestDoc(t, root, "knowledge/first", "---\ntype: Note\ntitle: First\ndescription: A note\n---\nbody\n")
	doc, err := Read(root, "knowledge/first")
	if err != nil {
		t.Fatal(err)
	}
	if err := Bookkeeping(root, doc, "Creation", time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(root, "knowledge/index.md"))
	if err != nil || !strings.Contains(string(index), "[First](first.md)") {
		t.Fatalf("index = %s, %v", index, err)
	}
	log, err := os.ReadFile(filepath.Join(root, "log.md"))
	if err != nil || !strings.Contains(string(log), "## 2026-09-08") || !strings.Contains(string(log), "Creation") {
		t.Fatalf("log = %s, %v", log, err)
	}
}
func writeTestDoc(t *testing.T, root, id, body string) {
	t.Helper()
	path := filepath.Join(root, id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReservedFiles(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"root_version", "index.md", "---\nokf_version: '9'\n---\n# Index\n"},
		{"nested_frontmatter", "knowledge/index.md", "---\ntype: Index\n---\n"},
		{"log_date", "log.md", "## yesterday\n- Creation: note\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, tc.path)
			if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			if err := Validate(root); err == nil {
				t.Fatal("accepted invalid reserved file")
			}
		})
	}
}

func TestBookkeepingRootIndexFrontmatter(t *testing.T) {
	root := t.TempDir()
	header := "---\nokf_version: '0.2'\n---\n"
	writeTestDoc(t, root, "index", header+"# Index\n")
	doc := Document{ID: "root-concept", Metadata: map[string]any{"type": "Knowledge", "title": "Root concept"}, Body: "Body\n"}
	if err := Bookkeeping(root, doc, "Creation", time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), header) {
		t.Fatalf("root frontmatter displaced: %s", raw)
	}
}
