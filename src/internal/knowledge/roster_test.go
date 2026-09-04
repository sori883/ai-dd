package knowledge_test

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterReturnsInlineLeadPersona(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Mode:      "inline",
			LeadAgent: "aidlc-product-agent",
		},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md": {Data: []byte("# product")},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	want := []string{".codex/agents/aidlc-product-agent.md"}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Fatalf("BuildRoster() paths = %#v, want %#v", got.Paths, want)
	}
	if got.Warnings == nil {
		t.Fatal("BuildRoster() warnings = nil, want non-nil")
	}
}

func TestBuildRosterSelectsAgentsByModeAndDeclarationOrder(t *testing.T) {
	framework := knowledge.Source{
		FS: fstest.MapFS{
			"agents/lead.md":    {Data: []byte("lead")},
			"agents/support.md": {Data: []byte("support")},
		},
		DisplayPrefix: ".codex",
	}

	tests := []struct {
		name string
		mode string
		want []string
	}{
		{
			name: "inline keeps lead and supports",
			mode: "inline",
			want: []string{
				".codex/agents/lead.md",
				".codex/agents/support.md",
			},
		},
		{
			name: "mob keeps lead only",
			mode: "mob",
			want: []string{".codex/agents/lead.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := knowledge.RosterInput{
				Stage: graph.Stage{
					Mode:          tt.mode,
					LeadAgent:     "lead",
					SupportAgents: []string{"orchestrator", "support", "lead"},
				},
				Framework:    framework,
				FrameworkDir: "/project/.codex",
			}

			got, err := knowledge.BuildRoster(input)
			if err != nil {
				t.Fatalf("BuildRoster() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got.Paths, tt.want) {
				t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, tt.want)
			}
		})
	}
}

func TestBuildRosterReturnsEmptyWithoutAgentsOrFilesystemAccess(t *testing.T) {
	fsys := &countingFS{}
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Mode:      "subagent",
			LeadAgent: "bad/name",
		},
		Framework: knowledge.Source{
			FS:            fsys,
			DisplayPrefix: "invalid/../prefix",
		},
		FrameworkDir: "relative",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if got.Paths == nil || got.Warnings == nil {
		t.Fatalf("BuildRoster() = %#v, want non-nil empty slices", got)
	}
	if len(got.Paths) != 0 || len(got.Warnings) != 0 {
		t.Errorf("BuildRoster() = %#v, want empty roster", got)
	}
	if fsys.opens != 0 {
		t.Errorf("filesystem opens = %d, want zero", fsys.opens)
	}
}

func TestBuildRosterUsesFiveGroupsAndUTF16DepthFirstMarkdownOrder(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Mode:          "inline",
			LeadAgent:     "lead",
			SupportAgents: []string{"support"},
		},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/lead.md":                      {Data: []byte("lead persona")},
				"agents/support.md":                   {Data: []byte("support persona")},
				"knowledge/aidlc-shared/a.md":         {Data: []byte("shared a")},
				"knowledge/aidlc-shared/z.md":         {Data: []byte("shared z")},
				"knowledge/aidlc-shared/sub/inner.md": {Data: []byte("shared inner")},
				"knowledge/aidlc-shared/skip.MD":      {Data: []byte("wrong extension")},
				"knowledge/lead/lead.md":              {Data: []byte("lead knowledge")},
				"knowledge/support/support.md":        {Data: []byte("support knowledge")},
			},
			DisplayPrefix: ".codex",
		},
		SpaceKnowledge: &knowledge.Source{
			FS: fstest.MapFS{
				"aidlc-shared/space.md": {Data: []byte("space shared")},
				"lead/space.md":         {Data: []byte("space lead")},
				"support/space.md":      {Data: []byte("space support")},
			},
			DisplayPrefix: "aidlc/spaces/team/knowledge",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	want := []string{
		".codex/agents/lead.md",
		".codex/agents/support.md",
		".codex/knowledge/aidlc-shared/a.md",
		".codex/knowledge/aidlc-shared/sub/inner.md",
		".codex/knowledge/aidlc-shared/z.md",
		".codex/knowledge/lead/lead.md",
		".codex/knowledge/support/support.md",
		"aidlc/spaces/team/knowledge/aidlc-shared/space.md",
		"aidlc/spaces/team/knowledge/lead/space.md",
		"aidlc/spaces/team/knowledge/support/space.md",
	}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, want)
	}
}

func TestBuildRosterUsesUTF16CodeUnitOrderForSupplementaryNames(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/lead.md":                   {Data: []byte("persona")},
				"knowledge/aidlc-shared/\ue000.md": {Data: []byte("bmp")},
				"knowledge/aidlc-shared/𐀀.md":      {Data: []byte("supplementary")},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	want := []string{
		".codex/agents/lead.md",
		".codex/knowledge/aidlc-shared/𐀀.md",
		".codex/knowledge/aidlc-shared/\ue000.md",
	}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, want)
	}
}

func TestBuildRosterPreflightsAllCandidatesAndKeepsEmptyKnowledge(t *testing.T) {
	base := fstest.MapFS{
		"agents/lead.md": {Data: []byte("persona")},
		"knowledge/aidlc-shared/ai-dlc-principles.md": {Data: []byte{}},
		"knowledge/aidlc-shared/audit-format.md":      {Data: []byte("not selected")},
	}
	fsys := &readErrorFS{
		base: base,
		fail: map[string]error{
			"knowledge/aidlc-shared/audit-format.md": errors.New("permission denied"),
		},
	}
	input := knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework: knowledge.Source{
			FS:            fsys,
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
		Depth:        " minimal ",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got.Paths, []string{
		".codex/agents/lead.md",
		".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
	}) {
		t.Errorf("BuildRoster() paths = %#v, want persona and empty knowledge", got.Paths)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "audit-format.md") {
		t.Errorf("BuildRoster() warnings = %#v, want preflight warning for pruned candidate", got.Warnings)
	}
}

func TestBuildRosterWarnsMissingPersonaAndInvalidUTF8(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Mode:          "inline",
			LeadAgent:     "missing",
			SupportAgents: []string{"lead"},
		},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/lead.md":                {Data: []byte("lead")},
				"knowledge/aidlc-shared/bad.md": {Data: []byte{0xff}},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got.Paths, []string{".codex/agents/lead.md"}) {
		t.Errorf("BuildRoster() paths = %#v, want lead persona only", got.Paths)
	}
	joinedWarnings := strings.Join(got.Warnings, "\n")
	if !strings.Contains(joinedWarnings, ".codex/agents/missing.md") ||
		!strings.Contains(joinedWarnings, ".codex/knowledge/aidlc-shared/bad.md") ||
		!strings.Contains(joinedWarnings, "invalid UTF-8") {
		t.Errorf("BuildRoster() warnings = %#v, want missing and UTF-8 warnings", got.Warnings)
	}
}

func TestBuildRosterReadsFreshContentOnEveryCall(t *testing.T) {
	fsys := fstest.MapFS{
		"agents/lead.md": {Data: []byte{0xff}},
	}
	input := knowledge.RosterInput{
		Stage:        graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework:    knowledge.Source{FS: fsys, DisplayPrefix: ".codex"},
		FrameworkDir: "/project/.codex",
	}

	first, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("first BuildRoster() error = %v, want nil", err)
	}
	if len(first.Paths) != 0 || len(first.Warnings) == 0 {
		t.Fatalf("first BuildRoster() = %#v, want invalid-file warning", first)
	}

	fsys["agents/lead.md"] = &fstest.MapFile{Data: []byte("fixed")}
	second, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("second BuildRoster() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(second.Paths, []string{".codex/agents/lead.md"}) || len(second.Warnings) != 0 {
		t.Errorf("second BuildRoster() = %#v, want fresh valid read", second)
	}
}

type readErrorFS struct {
	base fstest.MapFS
	fail map[string]error
}

func (f *readErrorFS) Open(name string) (fs.File, error) {
	if err, ok := f.fail[name]; ok {
		return nil, err
	}
	return f.base.Open(name)
}

type countingFS struct {
	opens int
}

func (f *countingFS) Open(string) (fs.File, error) {
	f.opens++
	return nil, errors.New("unexpected filesystem access")
}
