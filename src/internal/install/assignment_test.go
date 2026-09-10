package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func TestAssignmentContractLegacy(t *testing.T) {
	root, from, binary := relocateFixture(t)
	for source, template := range legacyAssignmentSkills {
		target := ".agents/skills/aidlc/SKILL.md"
		if source != "SKILL.md" {
			target = ".agents/skills/aidlc-cli/SKILL.md"
		}
		if err := os.WriteFile(filepath.Join(root, target), []byte(strings.ReplaceAll(template, "@@BINARY@@", shellQuote(binary))), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(root, ".codex/hooks.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte(assignmentMatcher), []byte("^(Bash|apply_patch)$"))
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Relocate(root, "/new/aidlc", from, binary); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil || !bytes.Contains(got, []byte("^(Bash|apply_patch)$")) {
		t.Fatal("legacy matcher upgraded", err)
	}
	for source, template := range legacyAssignmentSkills {
		target := ".agents/skills/aidlc/SKILL.md"
		if source != "SKILL.md" {
			target = ".agents/skills/aidlc-cli/SKILL.md"
		}
		got, err := os.ReadFile(filepath.Join(root, target))
		if err != nil || string(got) != strings.ReplaceAll(template, "@@BINARY@@", shellQuote("/new/aidlc")) {
			t.Fatal("legacy skill upgraded", err)
		}
	}
}
