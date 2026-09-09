package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionPlanSchemaCatalog(t *testing.T) {
	root := definitionFixture(t)
	stages := []map[string]string{}
	for _, id := range []string{"initialization", "discovery", "architecture-analysis", "planning", "tdd", "integration"} {
		stages = append(stages, map[string]string{"id": id, "name": id, "procedure": "stages/" + id + ".md"})
		body := "---\nstage_id: " + id + "\nagents: []\ninputs: []\noutputs: []\nsensors:\n  start: " + id + "-start\n  end: " + id + "-end\n---\n# Procedure\nInspect.\n"
		if err := os.WriteFile(filepath.Join(root, "aidlc/workflow/stages", id+".md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	catalog := map[string]any{"schema_version": 2, "stages": stages, "required_prefix": []string{"initialization", "discovery"}}
	save := func() {
		t.Helper()
		raw, err := json.Marshal(catalog)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, GraphPath), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	save()
	if d, err := Load(root); err != nil || len(d.Procedures) != 6 {
		t.Fatalf("six-stage catalog rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		prefix []string
	}{
		{name: "missing", prefix: nil}, {name: "reversed", prefix: []string{"discovery", "initialization"}}, {name: "optional mandatory", prefix: []string{"initialization", "discovery", "tdd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog["required_prefix"] = tc.prefix
			save()
			if _, err := Load(root); err == nil {
				t.Fatal("invalid prefix accepted")
			}
		})
	}
	catalog["required_prefix"] = []string{"initialization", "discovery"}
	catalog["advance"] = []map[string]string{{"from": "discovery", "to": "tdd"}}
	save()
	if _, err := Load(root); err == nil {
		t.Fatal("fixed edge accepted")
	}
}
