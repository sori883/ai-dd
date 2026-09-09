package okfmemory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentSelectorExactAndCount(t *testing.T) {
	root := t.TempDir()
	writeTestDoc(t, root, "b", sample)
	writeTestDoc(t, root, "a", sample)
	title, status := "Search", "draft"
	tags := []string{"search"}
	for _, tc := range []struct {
		name, count string
		match       DocumentMatch
		want        int
		bad         bool
	}{
		{"many stable order", "many", DocumentMatch{Type: "KDR", Title: &title, Status: &status, Tags: &tags}, 2, false},
		{"ambiguous one", "one", DocumentMatch{Type: "KDR"}, 0, true},
		{"missing one", "one", DocumentMatch{Type: "Missing"}, 0, true},
		{"optional empty", "optional", DocumentMatch{Type: "Missing"}, 0, false},
		{"optional ambiguous", "optional", DocumentMatch{Type: "KDR"}, 0, true},
		{"missing many", "many", DocumentMatch{Type: "Missing"}, 0, true},
		{"case exact", "optional", DocumentMatch{Type: "kdr"}, 0, false},
		{"invalid count", "all", DocumentMatch{Type: "KDR"}, 0, true},
		{"missing type", "many", DocumentMatch{}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectDocuments(root, tc.match, tc.count)
			if tc.bad {
				if err == nil {
					t.Fatal("invalid selection accepted")
				}
				return
			}
			if err != nil || len(got) != tc.want {
				t.Fatalf("selection=%+v err=%v", got, err)
			}
			if len(got) == 2 && (got[0].Path != "a.md" || got[1].Path != "b.md" || got[0].Hash == "" || len(got[0].Raw) == 0) {
				t.Fatal("order or byte evidence missing")
			}
		})
	}
}
func TestDocumentSelectorUnsafeOrBroken(t *testing.T) {
	for _, kind := range []string{"broken", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			writeTestDoc(t, root, "good", sample)
			if kind == "broken" {
				writeTestDoc(t, root, "broken", "not OKF")
			} else {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "outside")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := SelectDocuments(root, DocumentMatch{Type: "KDR"}, "one"); err == nil {
				t.Fatal("unsafe or invalid document ignored")
			}
		})
	}
}
