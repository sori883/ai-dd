package projectroot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAncestorFiles(t *testing.T) {
	for _, name := range []string{"aidlc", "aidlc/workflow"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			project := filepath.Join(base, "project")
			marker := filepath.Join(project, "aidlc/workflow/stage-graph.json")
			if err := os.MkdirAll(filepath.Dir(marker), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(marker, []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}
			obstruction := filepath.Join(base, name)
			if err := os.MkdirAll(filepath.Dir(obstruction), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(obstruction, []byte("standalone binary"), 0755); err != nil {
				t.Fatal(err)
			}
			want, err := filepath.EvalSymlinks(project)
			if err != nil {
				t.Fatal(err)
			}
			for _, explicit := range []string{"", project} {
				got, err := Resolve(explicit, project, false)
				if err != nil || got != want {
					t.Fatalf("Resolve(%q) = %q, %v; want %q", explicit, got, err, want)
				}
			}
		})
	}
}
func TestResolvePreservesErrors(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{base, project} {
		marker := filepath.Join(root, "aidlc/workflow/stage-graph.json")
		if err := os.MkdirAll(filepath.Dir(marker), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Resolve("", project, false); err == nil || !strings.Contains(err.Error(), "multiple installed") {
		t.Fatal("ambiguous roots accepted", err)
	}
	if err := os.RemoveAll(filepath.Join(base, "aidlc")); err != nil {
		t.Fatal(err)
	}
	loop := filepath.Join(base, "aidlc")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("", project, false); err == nil || strings.Contains(err.Error(), "not installed") {
		t.Fatal("filesystem error hidden", err)
	}
	want, _ := filepath.EvalSymlinks(project)
	if got, err := Resolve(project, project, false); err != nil || got != want {
		t.Fatal("explicit root changed", got, err)
	}
}
