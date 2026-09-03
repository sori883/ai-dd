//go:build integration

package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestStartIntentIntegrationPersistsCanonicalInitialState(t *testing.T) {
	project := t.TempDir()
	intentsDir := filepath.Join(project, "aidlc", "spaces", "team", "intents")
	if err := os.MkdirAll(intentsDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intentsDir, "intents.json"), []byte("[]\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	dataFS, scopesFS := startIntentIntegrationFS(t)
	started, err := StartIntent(
		context.Background(),
		StartInput{
			Root:                      workspace.RootInput{ExplicitDir: project},
			SpaceName:                 "team",
			Label:                     "Build Auth",
			Scope:                     "classic",
			Repos:                     []string{"api", "web"},
			DataFS:                    dataFS,
			ScopesFS:                  scopesFS,
			ProjectDescription:        "Build authentication",
			ProjectDescriptionPreview: "Build authentication",
		},
	)
	if err != nil {
		t.Fatalf("StartIntent() error = %v", err)
	}
	if !started.InitializationComplete {
		t.Fatal("InitializationComplete = false, want true")
	}
	if started.Intent == (workspace.CreatedIntent{}) {
		t.Fatal("StartIntent() returned zero committed intent")
	}
	if started.Workspace.ProjectType != "Greenfield" {
		t.Errorf("workspace project type = %q, want Greenfield", started.Workspace.ProjectType)
	}
	if !started.Initial.Plan.GreenfieldAdjusted() {
		t.Error("Initial.Plan.GreenfieldAdjusted() = false, want reverse-engineering correction")
	}
	entries := started.Initial.Plan.Entries()
	if len(entries) != 4 {
		t.Fatalf("initial plan entries = %d, want all graph stages", len(entries))
	}
	if entries[2].Stage.Slug != "reverse-engineering" || entries[2].Action != graph.ActionSkip {
		t.Errorf("reverse-engineering plan entry = %+v, want SKIP correction", entries[2])
	}

	recordDir := started.Intent.RecordDir
	description, err := os.ReadFile(filepath.Join(recordDir, "project-description.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(description) != "\"Build authentication\"\n" {
		t.Errorf("project description = %q, want canonical JSON string", description)
	}
	state, err := os.ReadFile(filepath.Join(recordDir, "aidlc-state.md"))
	if err != nil {
		t.Fatal(err)
	}
	stateText := string(state)
	for _, want := range []string{
		"- [x] workspace-scaffold — EXECUTE",
		"- [-] intent-capture — EXECUTE",
		"- [ ] reverse-engineering — SKIP",
		"- [ ] operation — SKIP",
	} {
		if !strings.Contains(stateText, want) {
			t.Errorf("state does not contain %q", want)
		}
	}

	registry, err := os.ReadFile(filepath.Join(intentsDir, "intents.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(registry, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("registry rows = %d, want one committed row", len(rows))
	}
	var status string
	if err := json.Unmarshal(rows[0]["status"], &status); err != nil || status != "in-flight" {
		t.Errorf("registry status = (%q, %v), want in-flight", status, err)
	}
	for path, want := range map[string]string{
		filepath.Join(project, "aidlc", "active-space"): "default\n",
		filepath.Join(intentsDir, "active-intent"):      started.Intent.DirName + "\n",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("cursor %q = (%q, %v), want %q", path, got, err, want)
		}
	}

	entriesOnDisk, err := os.ReadDir(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entriesOnDisk))
	for index, entry := range entriesOnDisk {
		names[index] = entry.Name()
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"aidlc-state.md", "project-description.json"}) {
		t.Errorf("record sidecars = %q, want only canonical files", names)
	}
	for _, sidecar := range []string{".aidlc-plan.json", ".aidlc-stage-plan.json"} {
		if _, err := os.Stat(filepath.Join(recordDir, sidecar)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("plan sidecar %q exists or returned unexpected error: %v", sidecar, err)
		}
	}
}

func startIntentIntegrationFS(t *testing.T) (fs.FS, fs.FS) {
	t.Helper()
	dataFS := fstest.MapFS{
		"stage-graph.json": {Data: []byte(`[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"aidlc-orchestrator","support_agents":[],"mode":"sequential","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"sequential","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"reverse-engineering","number":"2.1","name":"Reverse Engineering","phase":"inception","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"sequential","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"operation","number":"4.1","name":"Operation","phase":"operation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"sequential","scopes":["classic"],"produces":[],"consumes":[],"requires_stage":[]}
]`)},
		"scope-grid.json": {Data: []byte(`{"classic":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE","reverse-engineering":"EXECUTE","operation":"SKIP"}}}`)},
	}
	scopesFS := fstest.MapFS{
		"classic.md": {Data: []byte("---\nname: classic\ndepth: Standard\ntestStrategy: Standard\n---\n")},
	}
	return dataFS, scopesFS
}
