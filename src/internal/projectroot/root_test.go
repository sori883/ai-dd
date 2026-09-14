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
			explicit := ""
			got, err := Resolve(explicit, project, false)
			if err != nil || got != want {
				t.Fatalf("Resolve(%q) = %q, %v; want %q", explicit, got, err, want)
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

func TestResolveInstallationContext(t *testing.T) {
	for _, tc := range []struct {
		name    string
		markers []string
		install bool
		want    string
		wantErr string
	}{
		{name: "application", markers: []string{"project"}, want: "project"},
		{name: "missing", wantErr: "install"},
		{name: "new install", install: true, want: "project/app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			cwd := filepath.Join(base, "project/app")
			if err := os.MkdirAll(cwd, 0755); err != nil {
				t.Fatal(err)
			}
			for _, p := range tc.markers {
				dir := filepath.Join(base, p, "aidlc/workflow")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "stage-graph.json"), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Resolve("", cwd, tc.install)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %q, %v; want error containing %q", got, err, tc.wantErr)
				}
				return
			}
			want, _ := filepath.EvalSymlinks(filepath.Join(base, tc.want))
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
}
