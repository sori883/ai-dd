package knowledge_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterRejectsInvalidInputsBeforeFilesystemAccess(t *testing.T) {
	tests := []struct {
		name  string
		input knowledge.RosterInput
	}{
		{
			name: "nil framework filesystem",
			input: knowledge.RosterInput{
				Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
				Framework: knowledge.Source{
					DisplayPrefix: ".codex",
				},
				FrameworkDir: "/project/.codex",
			},
		},
		{
			name: "invalid display prefix",
			input: knowledge.RosterInput{
				Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
				Framework: knowledge.Source{
					FS:            &countingFS{},
					DisplayPrefix: "prefix/../bad",
				},
				FrameworkDir: "/project/.codex",
			},
		},
		{
			name: "invalid agent path",
			input: knowledge.RosterInput{
				Stage: graph.Stage{Mode: "inline", LeadAgent: "lead/name"},
				Framework: knowledge.Source{
					FS:            &countingFS{},
					DisplayPrefix: ".codex",
				},
				FrameworkDir: "/project/.codex",
			},
		},
		{
			name: "relative framework directory",
			input: knowledge.RosterInput{
				Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
				Framework: knowledge.Source{
					FS:            &countingFS{},
					DisplayPrefix: ".codex",
				},
				FrameworkDir: "project/.codex",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := knowledge.BuildRoster(tt.input)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("BuildRoster() error = %v, want fs.ErrInvalid", err)
			}
			if got.Paths != nil || got.Warnings != nil {
				t.Errorf("BuildRoster() result = %#v, want zero roster on input error", got)
			}
		})
	}
}

func TestBuildRosterAllowsLiteralBackslashOnPOSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX display and agent locality contract")
	}
	input := knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: `lead\agent`},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				`agents/lead\agent.md`: {Data: []byte("persona")},
			},
			DisplayPrefix: `prefix\root`,
		},
		FrameworkDir: filepath.Join("/project", "prefix"),
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	want := `prefix\root/agents/lead\agent.md`
	if len(got.Paths) != 1 || got.Paths[0] != want {
		t.Errorf("BuildRoster() paths = %#v, want literal backslash path %q", got.Paths, want)
	}
}

func TestBuildRosterRejectsTypedNilFrameworkFilesystem(t *testing.T) {
	var fsys *countingFS
	input := knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework: knowledge.Source{
			FS:            fsys,
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("BuildRoster() error = %v, want fs.ErrInvalid", err)
	}
	if got.Paths != nil || got.Warnings != nil {
		t.Errorf("BuildRoster() result = %#v, want zero roster", got)
	}
	if strings.Contains(err.Error(), "filesystem access") {
		t.Errorf("BuildRoster() error = %v, want validation before access", err)
	}
}
