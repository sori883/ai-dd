package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagePlannerRole(t *testing.T) {
	for _, tc := range []struct {
		name, role, agent string
		valid             bool
	}{{"planner", "execution_planning", "aidlc-stage-planner", true}, {"unknown role", "invented", "aidlc-stage-planner", false}, {"wrong agent", "execution_planning", "aidlc-worker", false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := definitionFixture(t)
			p := filepath.Join(root, "aidlc/workflow/stages/discovery.md")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			raw = []byte(strings.Replace(string(raw), "agents: []", "agents: [{role: "+tc.role+", agent: "+tc.agent+"}]", 1))
			if err = os.WriteFile(p, raw, 0644); err != nil {
				t.Fatal(err)
			}
			_, err = Load(root)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
