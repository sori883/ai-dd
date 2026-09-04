package knowledge_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterDoesNotShareInputOrResultSlices(t *testing.T) {
	supports := []string{"support"}
	enabledPlugins := []string{"plugin"}
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Mode:          "inline",
			LeadAgent:     "lead",
			SupportAgents: supports,
		},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/lead.md":    {Data: []byte("lead")},
				"agents/support.md": {Data: []byte("support")},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir:   "/project/.codex",
		EnabledPlugins: enabledPlugins,
	}

	first, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("first BuildRoster() error = %v, want nil", err)
	}
	if len(first.Paths) != 2 {
		t.Fatalf("first BuildRoster() paths = %#v, want two personas", first.Paths)
	}
	if supports[0] != "support" || enabledPlugins[0] != "plugin" {
		t.Fatalf("BuildRoster() mutated input slices: supports=%q plugins=%q", supports, enabledPlugins)
	}
	first.Paths[0] = "changed"
	first.Warnings = append(first.Warnings, "changed")

	second, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("second BuildRoster() error = %v, want nil", err)
	}
	want := []string{
		".codex/agents/lead.md",
		".codex/agents/support.md",
	}
	if !reflect.DeepEqual(second.Paths, want) {
		t.Errorf("second BuildRoster() paths = %#v, want fresh independent result", second.Paths)
	}
}
