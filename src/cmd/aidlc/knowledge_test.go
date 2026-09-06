package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/okf"
)

func TestKnowledgeSearchAdapter(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "aidlc", "spaces", "team", "knowledge", "okf")
	if err := os.MkdirAll(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aidlc", "active-space"), []byte("team\n"), 0600); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(bundle, "日本語.md")
	if err := os.WriteFile(name, []byte("---\ntype: A\n---\nBODY_SECRET"), 0600); err != nil {
		t.Fatal(err)
	}
	search := knowledgeSearcher(func() (string, error) { return dir, nil }, func(string) string { return "" }, func() time.Time { return time.Time{} })
	got, err := search(okf.SearchOptions{Types: []string{"A"}}, dir)
	if err != nil || len(got.Results) != 1 || got.Results[0].Path != "aidlc/spaces/team/knowledge/okf/日本語.md" {
		t.Fatalf("result %+v err %v", got, err)
	}
	if err := os.WriteFile(name, []byte("---\ntype: B\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = search(okf.SearchOptions{Types: []string{"A"}}, dir)
	if err != nil || len(got.Results) != 0 {
		t.Fatalf("not fresh: %+v %v", got, err)
	}
}

func TestKnowledgeSearchAdapterMissingRoot(t *testing.T) {
	dir := t.TempDir()
	search := knowledgeSearcher(func() (string, error) { return dir, nil }, func(string) string { return "" }, time.Now)
	if _, err := search(okf.SearchOptions{Types: []string{"A"}}, dir); err == nil {
		t.Fatal("accepted missing root")
	}
}

func TestKnowledgeSearchRejectsUnsafeActiveSpace(t *testing.T) {
	for _, tt := range []struct{ name, cursor, bundle string }{
		{name: "parent traversal", cursor: "../outside", bundle: "aidlc/outside/knowledge/okf"},
		{name: "slash", cursor: "team/nested", bundle: "aidlc/spaces/team/nested/knowledge/okf"},
		{name: "backslash", cursor: "team\\nested"},
		{name: "dot", cursor: ".", bundle: "aidlc/spaces/knowledge/okf"},
		{name: "dot dot", cursor: "..", bundle: "aidlc/knowledge/okf"},
		{name: "invalid utf8", cursor: "\xff"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "aidlc"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "aidlc", "active-space"), []byte(tt.cursor), 0600); err != nil {
				t.Fatal(err)
			}
			if tt.bundle != "" {
				bundle := filepath.Join(dir, filepath.FromSlash(tt.bundle))
				if err := os.MkdirAll(bundle, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(bundle, "outside.md"), []byte("---\ntype: A\n---\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			search := knowledgeSearcher(func() (string, error) { return dir, nil }, func(string) string { return "" }, time.Now)
			got, err := search(okf.SearchOptions{Types: []string{"A"}}, dir)
			if err == nil || !strings.Contains(err.Error(), "invalid active space") || len(got.Results) != 0 {
				t.Fatalf("unsafe cursor %q: result %+v error %v; want active-space validation failure before child access", tt.cursor, got, err)
			}
		})
	}
}

func TestKnowledgeSearchPreservesDefaultSpaceFallback(t *testing.T) {
	for _, name := range []string{"missing cursor", "blank cursor"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			bundle := filepath.Join(dir, "aidlc", "spaces", "default", "knowledge", "okf")
			if err := os.MkdirAll(bundle, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bundle, "a.md"), []byte("---\ntype: A\n---\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if name == "blank cursor" {
				if err := os.WriteFile(filepath.Join(dir, "aidlc", "active-space"), []byte(" \n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			search := knowledgeSearcher(func() (string, error) { return dir, nil }, func(string) string { return "" }, time.Now)
			got, err := search(okf.SearchOptions{Types: []string{"A"}}, dir)
			if err != nil || len(got.Results) != 1 || got.Results[0].Path != "aidlc/spaces/default/knowledge/okf/a.md" {
				t.Fatalf("default result %+v error %v", got, err)
			}
		})
	}
}

func TestKnowledgeSearchRejectsOKFLeafSymlink(t *testing.T) {
	dir := t.TempDir()
	knowledge := filepath.Join(dir, "aidlc", "spaces", "default", "knowledge")
	target := filepath.Join(knowledge, "target")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a.md"), []byte("---\ntype: A\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(knowledge, "okf")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	search := knowledgeSearcher(func() (string, error) { return dir, nil }, func(string) string { return "" }, time.Now)
	got, err := search(okf.SearchOptions{Types: []string{"A"}}, dir)
	if err == nil || !strings.Contains(err.Error(), "non-symlink directory") || len(got.Results) != 0 {
		t.Fatalf("symlink result %+v error %v", got, err)
	}
}
