package kdr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const draft = "---\ntype: KDR\ntitle: Search\ndescription: Search results\ntags: [kdr]\ncustom: keep\n---\n## 目的と完成条件\nFilter search results by one tag.\n## 参照する設計・ルール\nRead rules/rule.\n## 不明点と進め方\nNo unresolved questions.\n## 判断と結果\nNot implemented: starting the work.\n## 検証・レビュー\nNot run: implementation has not started.\n## 残件と再開\nWrite the first test.\n"

func TestDocumentContract(t *testing.T) {
	if _, err := Parse([]byte(draft), ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, raw, id string }{
		{"missing_section", strings.Replace(draft, "## 判断と結果", "## Other", 1), ""},
		{"placeholder", strings.Replace(draft, "Filter search results by one tag.", "<目的を記入>", 1), ""},
		{"empty_section", strings.Replace(draft, "Read rules/rule.", " ", 1), ""},
		{"wrong_id", draft, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.raw), tc.id); err == nil {
				t.Fatal("accepted invalid KDR")
			}
		})
	}
}
func TestStoreCreateUpdateAndConflict(t *testing.T) {
	store := testStore(t)
	created, err := store.Create([]byte(draft), "process:test")
	if err != nil {
		t.Fatal(err)
	}
	if len(created.ID) != 32 || created.Hash == "" {
		t.Fatalf("creation = %+v", created)
	}
	changed := strings.Replace(string(created.Raw), "Write the first test.", "Tests passed; next is independent review.", 1)
	updated, err := store.Update(created.ID, []byte(changed), created.Hash, "process:test")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Hash == created.Hash {
		t.Fatalf("update = %+v", updated)
	}
	if _, err := store.Update(created.ID, []byte(changed), created.Hash, "process:test"); err == nil {
		t.Fatal("accepted stale hash")
	}
	if _, err := store.Update(created.ID, updated.Raw, updated.Hash, "process:test"); err == nil {
		t.Fatal("accepted no-op")
	}
	dropped := strings.Replace(string(updated.Raw), "custom: keep\n", "", 1)
	dropped = strings.Replace(dropped, "Tests passed; next is independent review.", "Review passed.", 1)
	if _, err := store.Update(created.ID, []byte(dropped), updated.Hash, "process:test"); err == nil {
		t.Fatal("dropped unknown metadata")
	}
}
func TestResolveAmbiguousAndRename(t *testing.T) {
	store := testStore(t)
	a, err := store.Create([]byte(draft), "process:test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create([]byte(draft), "process:test"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve("Search"); err == nil {
		t.Fatal("chose ambiguous title")
	}
	raw := strings.Replace(string(a.Raw), "title: Search", "title: Renamed", 1)
	raw = strings.Replace(raw, "Write the first test.", "Name changed; continue tests.", 1)
	if _, err := store.Update(a.ID, []byte(raw), a.Hash, "process:test"); err != nil {
		t.Fatal(err)
	}
	id, err := store.Resolve("Renamed")
	if err != nil || id != a.ID {
		t.Fatalf("resolve = %q, %v", id, err)
	}
}
func TestRepairPartialAndMissing(t *testing.T) {
	store := testStore(t)
	index := filepath.Join(store.Bundle, "kdr/index.md")
	if err := os.MkdirAll(index, 0755); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Create([]byte(draft), "process:test")
	if err == nil || saved.ID == "" || saved.Hash == "" {
		t.Fatalf("partial result = %+v, %v", saved, err)
	}
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	repaired, err := store.Repair(saved.ID, saved.Raw, saved.Hash, "process:test")
	if err != nil {
		t.Fatal(err)
	}
	if string(repaired.Raw) != string(saved.Raw) {
		t.Fatal("repair changed valid KDR body")
	}
	if err := os.Remove(filepath.Join(store.Bundle, "kdr", saved.ID+".md")); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Repair(saved.ID, saved.Raw, "missing", "process:test")
	if err != nil || restored.ID != saved.ID {
		t.Fatalf("restore = %+v, %v", restored, err)
	}
}
func testStore(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	bundle := filepath.Join(root, "aidlc/spaces/main/knowledge")
	if err := os.MkdirAll(bundle, 0755); err != nil {
		t.Fatal(err)
	}
	return Store{Root: root, Bundle: bundle, Space: "main"}
}
