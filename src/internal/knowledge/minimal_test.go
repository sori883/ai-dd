package knowledge_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterMinimalKeepsOnlyKnownStageKnowledgeAndAllCustomKnowledge(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Slug:      "intent-capture",
			Mode:      "inline",
			LeadAgent: "aidlc-product-agent",
		},
		Depth: "\ufeff Minimal\u2029",
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":                            {Data: []byte("persona")},
				"knowledge/aidlc-shared/ai-dlc-principles.md":              {Data: []byte("selected")},
				"knowledge/aidlc-shared/audit-format.md":                   {Data: []byte("other stage")},
				"knowledge/aidlc-shared/custom.md":                         {Data: []byte("custom")},
				"knowledge/aidlc-shared/nested/audit-format.md":            {Data: []byte("custom collision")},
				"knowledge/aidlc-shared/rules-reading.md":                  {Data: []byte("selected")},
				"knowledge/aidlc-shared/verification.md":                   {Data: []byte("selected")},
				"knowledge/aidlc-product-agent/functional-design-guide.md": {Data: []byte("other stage")},
				"knowledge/aidlc-product-agent/requirements-guide.md":      {Data: []byte("selected")},
				"knowledge/aidlc-product-agent/custom.md":                  {Data: []byte("custom")},
			},
			DisplayPrefix: ".codex",
		},
		SpaceKnowledge: &knowledge.Source{
			FS: fstest.MapFS{
				"aidlc-shared/audit-format.md": {Data: []byte("space keeps")},
				"aidlc-product-agent/space.md": {Data: []byte("space keeps")},
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
		".codex/agents/aidlc-product-agent.md",
		".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
		".codex/knowledge/aidlc-shared/custom.md",
		".codex/knowledge/aidlc-shared/nested/audit-format.md",
		".codex/knowledge/aidlc-shared/rules-reading.md",
		".codex/knowledge/aidlc-shared/verification.md",
		".codex/knowledge/aidlc-product-agent/custom.md",
		".codex/knowledge/aidlc-product-agent/requirements-guide.md",
		"aidlc/spaces/team/knowledge/aidlc-shared/audit-format.md",
		"aidlc/spaces/team/knowledge/aidlc-product-agent/space.md",
	}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, want)
	}
}

func TestBuildRosterMinimalUsesRequirementsAnalysisTable(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{
			Slug:      "requirements-analysis",
			Mode:      "inline",
			LeadAgent: "aidlc-product-agent",
		},
		Depth: "minimal",
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":                       {Data: []byte("persona")},
				"knowledge/aidlc-shared/ai-dlc-principles.md":         {Data: []byte("keep")},
				"knowledge/aidlc-shared/brownfield.md":                {Data: []byte("keep")},
				"knowledge/aidlc-shared/rules-reading.md":             {Data: []byte("keep")},
				"knowledge/aidlc-shared/verification.md":              {Data: []byte("keep")},
				"knowledge/aidlc-shared/audit-format.md":              {Data: []byte("drop")},
				"knowledge/aidlc-product-agent/requirements-guide.md": {Data: []byte("keep")},
				"knowledge/aidlc-product-agent/product-guide.md":      {Data: []byte("drop")},
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
		".codex/agents/aidlc-product-agent.md",
		".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
		".codex/knowledge/aidlc-shared/brownfield.md",
		".codex/knowledge/aidlc-shared/rules-reading.md",
		".codex/knowledge/aidlc-shared/verification.md",
		".codex/knowledge/aidlc-product-agent/requirements-guide.md",
	}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("BuildRoster() paths = %#v, want requirements-analysis table", got.Paths)
	}
}

func TestBuildRosterStandardAndUnknownDepthKeepAllFrameworkKnowledge(t *testing.T) {
	for _, depth := range []string{"Standard", "Comprehensive"} {
		t.Run(depth, func(t *testing.T) {
			input := knowledge.RosterInput{
				Stage: graph.Stage{Slug: "intent-capture", Mode: "inline", LeadAgent: "aidlc-product-agent"},
				Depth: depth,
				Framework: knowledge.Source{
					FS: fstest.MapFS{
						"agents/aidlc-product-agent.md":                            {Data: []byte("persona")},
						"knowledge/aidlc-shared/audit-format.md":                   {Data: []byte("keep")},
						"knowledge/aidlc-product-agent/functional-design-guide.md": {Data: []byte("keep")},
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
				".codex/agents/aidlc-product-agent.md",
				".codex/knowledge/aidlc-shared/audit-format.md",
				".codex/knowledge/aidlc-product-agent/functional-design-guide.md",
			}
			if !reflect.DeepEqual(got.Paths, want) {
				t.Errorf("BuildRoster() paths = %#v, want all knowledge", got.Paths)
			}
		})
	}

	input := knowledge.RosterInput{
		Stage: graph.Stage{Slug: "future-stage", Mode: "inline", LeadAgent: "aidlc-product-agent"},
		Depth: "minimal",
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":                            {Data: []byte("persona")},
				"knowledge/aidlc-shared/audit-format.md":                   {Data: []byte("keep")},
				"knowledge/aidlc-product-agent/functional-design-guide.md": {Data: []byte("keep")},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}
	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("unknown-stage BuildRoster() error = %v, want nil", err)
	}
	if len(got.Paths) != 3 {
		t.Errorf("unknown-stage BuildRoster() paths = %#v, want all framework knowledge", got.Paths)
	}
}

func TestBuildRosterMinimalKeepsPluginKnowledgeOutsideTable(t *testing.T) {
	tests := []struct {
		name  string
		stage string
		owner string
	}{
		{
			name:  "unknown stage",
			stage: "future-stage",
			owner: "aidlc-product-agent",
		},
		{
			name:  "unknown owner in known stage",
			stage: "intent-capture",
			owner: "custom-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framework := fstest.MapFS{
				"agents/" + tt.owner + ".md": {
					Data: []byte("persona"),
				},
				"knowledge/" + tt.owner + "/plugin-only.md": {
					Data: []byte("plugin-owned"),
				},
				"tools/data/plugin-files-disabled.json": {
					Data: []byte(`{"schema_version":1,"plugin":"disabled-plugin","knowledge":["` + tt.owner + `/plugin-only.md"]}`),
				},
			}
			got, err := knowledge.BuildRoster(knowledge.RosterInput{
				Stage: graph.Stage{
					Slug:      tt.stage,
					Mode:      "inline",
					LeadAgent: tt.owner,
				},
				Depth: "minimal",
				Framework: knowledge.Source{
					FS:            framework,
					DisplayPrefix: ".codex",
				},
				FrameworkDir:   "/project/.codex",
				EnabledPlugins: []string{},
			})
			if err != nil {
				t.Fatalf("BuildRoster() error = %v, want nil", err)
			}

			wantPaths := []string{
				".codex/agents/" + tt.owner + ".md",
				".codex/knowledge/" + tt.owner + "/plugin-only.md",
			}
			if !reflect.DeepEqual(got.Paths, wantPaths) {
				t.Errorf("BuildRoster() paths = %#v, want plugin knowledge retained outside Minimal table %#v", got.Paths, wantPaths)
			}
			if !reflect.DeepEqual(got.Warnings, []string{}) {
				t.Errorf("BuildRoster() warnings = %#v, want no warnings", got.Warnings)
			}
		})
	}
}
