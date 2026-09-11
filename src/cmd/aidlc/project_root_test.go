package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectRootWithoutGit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		markers  []string
		explicit string
		install  bool
		want     string
		wantErr  string
	}{
		{name: "project", markers: []string{"project"}, want: "project"},
		{name: "application", markers: []string{"project"}, want: "project"},
		{name: "explicit", markers: []string{"project", "project/app"}, explicit: "project", want: "project"},
		{name: "ambiguous", markers: []string{"project", "project/app"}, wantErr: "--project-dir"},
		{name: "missing", wantErr: "install"},
		{name: "new install", install: true, want: "project/app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			cwd := filepath.Join(base, "project/app")
			if tc.name == "project" {
				cwd = filepath.Join(base, "project")
			}
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
			explicit := ""
			if tc.explicit != "" {
				explicit = filepath.Join(base, tc.explicit)
			}
			got, err := resolveProjectRoot(explicit, cwd, tc.install)
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
