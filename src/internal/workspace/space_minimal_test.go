package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSpaceOKF(t *testing.T) {
	root := t.TempDir()
	rule := "---\ntype: Rule\ntitle: Edited rule\n---\nKeep the user's choice.\n"
	source := filepath.Join(root, "aidlc/spaces/default/knowledge/rules/rule.md")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(rule), 0644); err != nil {
		t.Fatal(err)
	}
	name, err := CreateSpace(RootInput{ExplicitDir: root}, "Team Alpha")
	if err != nil || name != "team-alpha" {
		t.Fatalf("create = %q, %v", name, err)
	}
	copied := filepath.Join(root, "aidlc/spaces/team-alpha/knowledge/rules/rule.md")
	got, err := os.ReadFile(copied)
	if err != nil || string(got) != rule {
		t.Fatalf("copied Rule = %q, %v", got, err)
	}
	if err := os.WriteFile(source, []byte("changed later"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(copied)
	if string(got) != rule {
		t.Fatal("new Space tracks mutable default")
	}
	for _, p := range []string{"index.md", "rules/entry.md", "knowledge/index.md", "design/index.md", "ADR/index.md"} {
		if _, err := os.Stat(filepath.Join(root, "aidlc/spaces/team-alpha/knowledge", p)); err != nil {
			t.Error(err)
		}
	}
}
func TestCreateSpaceOKFFallbackAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		exists        bool
	}{{"missing", "", false}, {"invalid", "not OKF", true}, {"wrong_type", "---\ntype: Note\n---\nwrong\n", true}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "aidlc/spaces/default/knowledge/rules/rule.md")
			if tc.exists {
				if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(source, []byte(tc.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := CreateSpace(RootInput{ExplicitDir: root}, "test")
			if tc.exists {
				if err == nil {
					t.Fatal("invalid default Rule was ignored")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(root, "aidlc/spaces/test/knowledge/rules/rule.md"))
			if err != nil || !strings.Contains(string(got), "type: Rule") {
				t.Fatalf("fallback = %q, %v", got, err)
			}
		})
	}
}
