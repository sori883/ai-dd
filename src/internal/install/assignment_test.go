package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestAssignmentContract(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/old/aidlc"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"PreToolUse", "PostToolUse"} {
		matcher := regexp.MustCompile(cfg.Hooks[event][0].Matcher)
		for _, name := range []string{"Bash", "apply_patch", "collaborationspawn_agent", "spawn_agent", "collaborationfollowup_task", "followup_task", "collaborationsend_message", "send_message", "collaborationinterrupt_agent", "interrupt_agent"} {
			if !matcher.MatchString(name) {
				t.Errorf("%s missing %s", event, name)
			}
		}
		if matcher.MatchString("othercollaborationspawn_agent") {
			t.Fatal("matcher broadened")
		}
	}
	if _, err := Relocate(root, "/new/aidlc", root, "/old/aidlc"); err != nil {
		t.Fatal(err)
	}
}
