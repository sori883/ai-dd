package install

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFresh(t *testing.T) {
	root := t.TempDir()
	result, err := Codex(root, "/opt/aidlc binary")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".codex/hooks.json", ".codex/agents/aidlc-reviewer.toml", ".agents/skills/aidlc/SKILL.md", "aidlc/templates/adr.md", "aidlc/spaces/default/knowledge/index.md", "aidlc/spaces/default/knowledge/rules/entry.md", "aidlc/spaces/default/knowledge/rules/rule.md"} {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Errorf("missing embedded asset %s: %v", path, err)
			continue
		}
		if len(raw) == 0 {
			t.Errorf("empty asset %s", path)
		}
	}
	if len(result.Paths) < 7 {
		t.Errorf("saved paths = %v", result.Paths)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if !json.Valid(raw) || !strings.Contains(string(raw), "/opt/aidlc binary") {
		t.Errorf("invalid hook configuration: %s", raw)
	}
}

func TestInstallCollisionPreserves(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex/hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("accepted existing destination")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "user configuration" {
		t.Fatalf("overwrote existing file: %s", got)
	}
}

func TestInstallRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "aidlc")); err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/opt/aidlc"); err == nil {
		t.Fatal("accepted symlink destination")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside project")
	}
}

func TestInstallRecoveryGuidanceAndContextLimit(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(skill) > 4096 || !strings.Contains(string(skill), "session bind ID --space SPACE --session SESSION --recover") {
		t.Fatalf("missing bounded recovery syntax: %s", skill)
	}
	for _, condition := range []string{"成功・失敗を問わず終了", "同じSpace・Intent・session", "メインAI"} {
		if !strings.Contains(string(skill), condition) {
			t.Errorf("deployed recovery guidance missing %q", condition)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	for event, groups := range config.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				_, present := handler["additionalContextLimit"]
				if present != (event == "SessionStart") {
					t.Errorf("unexpected context limit for %s: %v", event, handler)
				}
			}
		}
	}
}

func TestInstallMemoryCommandGuidance(t *testing.T) {
	root := t.TempDir()
	binary := "/private/var/folders/example/aidlc-journey-1234567890/bin/aidlc"
	if _, err := Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 {
		t.Fatalf("deployed skill is %d bytes, limit 4096", len(raw))
	}
	procedure, err := os.ReadFile(filepath.Join(root, "aidlc/workflow/stages/discovery.md"))
	if err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc-cli/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, common...)
	raw = append(raw, procedure...)
	for _, action := range []string{"create", "update", "show", "search"} {
		h, ok := cli.Help([]string{"memory", action, "--help"})
		if !ok {
			t.Fatal(action)
		}
		raw = append(raw, []byte(h)...)
	}
	for _, want := range []string{
		"memory create CONCEPT-ID", "memory update CONCEPT-ID", "memory show CONCEPT-ID", "memory search [QUERY]", "--body-file", "--actor", "--type", "--title", "--description", "--expect", "--intent-id",
		"拡張子なし", "adr/NAME", "hash", "content", "memory create --help", "memory update --help", "本文",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("deployed skill lacks %q", want)
		}
	}
}

func TestFlowInstallInactiveResumeGuidance(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/private/var/folders/example/aidlc"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 {
		t.Fatalf("deployed skill %d bytes exceeds 4096", len(raw))
	}
	for _, action := range []string{"resume", "reopen"} {
		h, ok := cli.Help([]string{"intent", action, "--help"})
		if !ok {
			t.Fatal(action)
		}
		raw = append(raw, []byte(h)...)
	}
	for _, want := range []string{"intent procedure ID --space SPACE", "intent resume ID --space SPACE --expect REVISION --reason TEXT", "intent reopen ID --space SPACE --expect REVISION --reason TEXT --step STEP_ID"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing inactive bootstrap grammar %q", want)
		}
	}
}

func TestMemoryHelpPlacedSkill(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4096 || !strings.Contains(string(raw), "../aidlc-cli/SKILL.md") || !strings.Contains(string(raw), "CLI help") {
		t.Fatalf("missing bounded help entry: %s", raw)
	}
}

func TestOKFWorkLogInstalledGuidance(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aidlc/workflow/stages/discovery.md", "aidlc/workflow/stages/planning.md", "aidlc/workflow/stages/tdd.md", "aidlc/workflow/stages/integration.md", ".agents/skills/aidlc-cli/SKILL.md"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(name, "/stages/") {
				if !strings.Contains(string(raw), "aidlc-cli") {
					t.Fatal("stage lacks common-operation reference")
				}
				raw, err = os.ReadFile(filepath.Join(root, ".agents/skills/aidlc-cli/SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
			}
			h, ok := cli.Help([]string{"intent", "reopen", "--help"})
			if !ok {
				t.Fatal("reopen help")
			}
			raw = append(raw, []byte(h)...)
			for _, want := range []string{"knowledge/log/<intent_id>-work-log.md", "memory search work-log --space SPACE --intent-id ID", "memory show log/ID-work-log --space SPACE", "revision"} {
				if !strings.Contains(string(raw), want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}

func TestInstallHookCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binary := "/opt/aidlc binary"
	if _, err := Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	events := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	if len(config.Hooks) != len(events) {
		t.Fatalf("events = %v", config.Hooks)
	}
	want := shellQuote(binary) + " __hook --project-dir " + shellQuote(root)
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			groups := config.Hooks[event]
			if len(groups) != 1 || len(groups[0].Hooks) != 1 {
				t.Fatalf("handlers = %+v", groups)
			}
			if got := groups[0].Hooks[0].Command; got != want {
				t.Fatalf("command = %q, want %q", got, want)
			}
		})
	}
}
