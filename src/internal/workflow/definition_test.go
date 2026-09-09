package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func definitionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "aidlc/workflow/stages")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	ids := []string{"discovery", "planning", "tdd", "integration"}
	g := Graph{SchemaVersion: 1, Start: ids[0], Completion: ids[3], Reopen: Reopen{Current: true, Ancestors: true}}
	for i, id := range ids {
		g.Stages = append(g.Stages, Stage{ID: id, Name: id, Procedure: "stages/" + id + ".md"})
		if i > 0 {
			g.Advance = append(g.Advance, Edge{From: ids[i-1], To: id})
		}
		body := "---\nstage_id: " + id + "\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: " + id + "-start\n  end: " + id + "-end\n---\n# Procedure\nDo the work.\n"
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(g)
	if err := os.WriteFile(filepath.Join(root, "aidlc/workflow/stage-graph.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestDefinitionValid(t *testing.T) {
	root := definitionFixture(t)
	d, err := Load(root)
	if err != nil || len(d.Hash) != 64 || d.Next("tdd") != "integration" || !d.Before("discovery", "tdd") || !d.CanReopen("tdd", "planning") || d.CanReopen("planning", "tdd") {
		t.Fatalf("invalid definition: %+v %v", d, err)
	}
	if !strings.Contains(d.Procedures["tdd"].Text, "# Procedure") {
		t.Fatal("procedure body missing")
	}
	before := d.Hash
	p := filepath.Join(root, "aidlc/workflow/stages/tdd.md")
	raw, _ := os.ReadFile(p)
	os.WriteFile(p, append(raw, []byte("Changed.\n")...), 0644)
	d, err = Load(root)
	if err != nil || d.Hash == before {
		t.Fatal("body not bound to hash", err)
	}
}
func TestDefinitionRejectsInvalid(t *testing.T) {
	cases := []struct{ name, file, old, new string }{
		{"unknown graph", "stage-graph.json", `"schema_version":1`, `"schema_version":1,"unknown":1`},
		{"duplicate graph key", "stage-graph.json", `"schema_version":1`, `"schema_version":1,"schema_version":1`},
		{"cycle", "stage-graph.json", `"to":"integration"`, `"to":"discovery"`},
		{"outside procedure", "stage-graph.json", "stages/tdd.md", "../tdd.md"},
		{"stage mismatch", "stages/tdd.md", "stage_id: tdd", "stage_id: planning"},
		{"unknown yaml", "stages/tdd.md", "agents: []", "unknown: true\nagents: []"},
		{"duplicate yaml", "stages/tdd.md", "agents: []", "agents: []\nagents: []"},
		{"alias", "stages/tdd.md", "inputs: []", "inputs: &a []\noutputs_alias: *a"},
		{"sensor", "stages/tdd.md", "tdd-end", "custom"},
		{"unsafe document", "stages/tdd.md", "outputs: []", "outputs:\n  - path: /tmp/code.go\n    role: code"},
		{"empty body", "stages/tdd.md", "# Procedure\nDo the work.", ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			root := definitionFixture(t)
			p := filepath.Join(root, "aidlc/workflow", tt.file)
			raw, _ := os.ReadFile(p)
			os.WriteFile(p, []byte(strings.Replace(string(raw), tt.old, tt.new, 1)), 0644)
			if _, err := Load(root); err == nil {
				t.Fatal("invalid definition accepted")
			}
		})
	}
	for _, kind := range []string{"missing", "symlink", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			root := definitionFixture(t)
			p := filepath.Join(root, "aidlc/workflow/stages/tdd.md")
			switch kind {
			case "missing":
				os.Remove(p)
			case "symlink":
				raw, _ := os.ReadFile(p)
				os.Remove(p)
				target := filepath.Join(t.TempDir(), "tdd.md")
				os.WriteFile(target, raw, 0644)
				os.Symlink(target, p)
			case "oversized":
				os.WriteFile(p, []byte(strings.Repeat("x", 256*1024+1)), 0644)
			}
			if _, err := Load(root); err == nil {
				t.Fatal("invalid file accepted")
			}
		})
	}
}

func TestDefinitionAcceptedInputMustPrecedeStage(t *testing.T) {
	root := definitionFixture(t)
	p := filepath.Join(root, "aidlc/workflow/stages/planning.md")
	raw, _ := os.ReadFile(p)
	raw = []byte(strings.Replace(string(raw), "inputs: []", "inputs:\n  - path: \"${knowledge_root}/design/${intent_id}/requirements.md\"\n    version: accepted\n    accepted_at: tdd", 1))
	os.WriteFile(p, raw, 0644)
	if _, err := Load(root); err == nil {
		t.Fatal("future accepted input creates an impossible prerequisite")
	}
}

func TestDefinitionRejectsEmptyOrUnknownAgent(t *testing.T) {
	for _, agent := range []string{"{}", "{role: unsupported}", "{agent: aidlc-worker}"} {
		t.Run(agent, func(t *testing.T) {
			root := definitionFixture(t)
			p := filepath.Join(root, "aidlc/workflow/stages/tdd.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(p, []byte(strings.Replace(string(raw), "agents: []", "agents: ["+agent+"]", 1)), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root); err == nil {
				t.Fatal("empty or unknown agent accepted")
			}
		})
	}
}
