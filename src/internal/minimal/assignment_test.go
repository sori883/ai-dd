package minimal

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/cli"
	"os"
	"path/filepath"
	"testing"
)

func TestAssignmentContract(t *testing.T) {
	s := Service{Root: t.TempDir(), Binary: "/opt/aidlc"}
	if err := os.WriteFile(filepath.Join(s.Root, "init.json"), []byte(`{"request_id":"init","human_confirmed":true,"reason":"human confirmed work stopped"}`), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "assignment", Action: "init", File: "init.json"})
	if err != nil {
		t.Fatal(err)
	}
	var r assignment.Registry
	if err := json.Unmarshal(raw, &r); err != nil || r.Epoch == "" {
		t.Fatalf("init output: %s %v", raw, err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "assignment", Action: "list"}); err != nil {
		t.Fatal(err)
	}
	// A forged owner argument must be denied even before normal selected-Intent checks.
	out := hook(t, s, "PreToolUse", "Bash", "tool", "/opt/aidlc assignment release id --session another --expect 1 --file release.json", false)
	if !deny(out) {
		t.Fatal("forged release owner accepted")
	}
	out = hook(t, s, "PreToolUse", "Bash", "tool", "/opt/aidlc assignment list", false)
	if deny(out) {
		t.Fatal("registry diagnostics blocked without selection")
	}
}
