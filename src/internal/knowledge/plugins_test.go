package knowledge_test

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterPluginOwnershipOverridesMinimalAndSupportsSelection(t *testing.T) {
	base := fstest.MapFS{
		"agents/aidlc-product-agent.md":               {Data: []byte("persona")},
		"knowledge/aidlc-shared/ai-dlc-principles.md": {Data: []byte("selected")},
		"knowledge/aidlc-shared/audit-format.md":      {Data: []byte("plugin owned")},
		"knowledge/aidlc-shared/plugin-only.md":       {Data: []byte("plugin only")},
		"tools/data/plugin-files-one.json":            {Data: []byte(`{"schema_version":1.0,"plugin":"one","knowledge":["aidlc-shared/audit-format.md","aidlc-shared/plugin-only.md"]}`)},
		"tools/data/plugin-files-two.json":            {Data: []byte(`{"schema_version":1,"plugin":"two","knowledge":["aidlc-shared/plugin-only.md"]}`)},
	}
	input := func(enabled []string) knowledge.RosterInput {
		return knowledge.RosterInput{
			Stage: graph.Stage{
				Slug:      "intent-capture",
				Mode:      "inline",
				LeadAgent: "aidlc-product-agent",
			},
			Framework: knowledge.Source{
				FS:            base,
				DisplayPrefix: ".codex",
			},
			FrameworkDir:   "/project/.codex",
			Depth:          "Minimal",
			EnabledPlugins: enabled,
		}
	}

	tests := []struct {
		name    string
		enabled []string
		want    []string
	}{
		{
			name:    "nil enables all owners",
			enabled: nil,
			want: []string{
				".codex/agents/aidlc-product-agent.md",
				".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
				".codex/knowledge/aidlc-shared/audit-format.md",
				".codex/knowledge/aidlc-shared/plugin-only.md",
			},
		},
		{
			name:    "explicit empty disables all owners",
			enabled: []string{},
			want: []string{
				".codex/agents/aidlc-product-agent.md",
				".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
			},
		},
		{
			name:    "exact plugin selection uses union",
			enabled: []string{"two"},
			want: []string{
				".codex/agents/aidlc-product-agent.md",
				".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
				".codex/knowledge/aidlc-shared/plugin-only.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := knowledge.BuildRoster(input(tt.enabled))
			if err != nil {
				t.Fatalf("BuildRoster() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got.Paths, tt.want) {
				t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, tt.want)
			}
		})
	}
}

func TestBuildRosterKeepsPriorPluginOwnersWhenLaterKnowledgeItemIsInvalid(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{Slug: "intent-capture", Mode: "inline", LeadAgent: "aidlc-product-agent"},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":               {Data: []byte("persona")},
				"knowledge/aidlc-shared/audit-format.md":      {Data: []byte("plugin owned")},
				"knowledge/aidlc-shared/ai-dlc-principles.md": {Data: []byte("selected")},
				"tools/data/plugin-files-partial.json": {
					Data: []byte(`{"schema_version":1,"plugin":"partial","knowledge":["aidlc-shared/audit-format.md",3,"aidlc-shared/never.md"]}`),
				},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
		Depth:        "minimal",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got.Paths, []string{
		".codex/agents/aidlc-product-agent.md",
		".codex/knowledge/aidlc-shared/ai-dlc-principles.md",
		".codex/knowledge/aidlc-shared/audit-format.md",
	}) {
		t.Errorf("BuildRoster() paths = %#v, want prior owner retained", got.Paths)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "plugin-files-partial.json") {
		t.Errorf("BuildRoster() warnings = %#v, want partial manifest warning", got.Warnings)
	}
}

func TestBuildRosterRequiresExactPluginMetadataKeys(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{Slug: "intent-capture", Mode: "inline", LeadAgent: "aidlc-product-agent"},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":          {Data: []byte("persona")},
				"knowledge/aidlc-shared/audit-format.md": {Data: []byte("known")},
				"tools/data/plugin-files-wrong.json": {
					Data: []byte(`{"schema_version":1,"Plugin":"plugin","knowledge":["aidlc-shared/audit-format.md"]}`),
				},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
		Depth:        "minimal",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if len(got.Paths) != 1 || got.Paths[0] != ".codex/agents/aidlc-product-agent.md" {
		t.Errorf("BuildRoster() paths = %#v, want persona only after invalid metadata", got.Paths)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "plugin-files-wrong.json") {
		t.Errorf("BuildRoster() warnings = %#v, want exact-key warning", got.Warnings)
	}
}

func TestBuildRosterUsesLastExactDuplicatePluginKey(t *testing.T) {
	input := knowledge.RosterInput{
		Stage: graph.Stage{Slug: "intent-capture", Mode: "inline", LeadAgent: "aidlc-product-agent"},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/aidlc-product-agent.md":          {Data: []byte("persona")},
				"knowledge/aidlc-shared/audit-format.md": {Data: []byte("plugin owned")},
				"tools/data/plugin-files-duplicate.json": {
					Data: []byte(`{"schema_version":1,"plugin":"first","plugin":"second","knowledge":["aidlc-shared/audit-format.md"]}`),
				},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
		Depth:        "minimal",
	}

	input.EnabledPlugins = []string{"second"}
	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("second BuildRoster() error = %v, want nil", err)
	}
	if !containsPath(got.Paths, ".codex/knowledge/aidlc-shared/audit-format.md") {
		t.Fatalf("second BuildRoster() paths = %#v, want last duplicate key", got.Paths)
	}

	input.EnabledPlugins = []string{"first"}
	got, err = knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("first BuildRoster() error = %v, want nil", err)
	}
	if containsPath(got.Paths, ".codex/knowledge/aidlc-shared/audit-format.md") {
		t.Errorf("first BuildRoster() paths = %#v, want first duplicate key ignored", got.Paths)
	}
}

func TestBuildRosterDoesNotOpenMetadataKnowledgeValues(t *testing.T) {
	fsys := &trackingFS{base: fstest.MapFS{
		"agents/aidlc-product-agent.md": {Data: []byte("persona")},
		"tools/data/plugin-files-values.json": {
			Data: []byte(`{"schema_version":1,"plugin":"plugin","knowledge":["aidlc-shared/not-installed.md"]}`),
		},
	}}
	_, err := knowledge.BuildRoster(knowledge.RosterInput{
		Stage: graph.Stage{Slug: "intent-capture", Mode: "inline", LeadAgent: "aidlc-product-agent"},
		Framework: knowledge.Source{
			FS:            fsys,
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
		Depth:        "minimal",
	})
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	for _, call := range fsys.calls {
		if call == "knowledge/aidlc-shared/not-installed.md" {
			t.Errorf("BuildRoster() opened metadata knowledge value %q", call)
		}
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

type trackingFS struct {
	base  fstest.MapFS
	calls []string
}

func (f *trackingFS) Open(name string) (fs.File, error) {
	f.calls = append(f.calls, name)
	return f.base.Open(name)
}
