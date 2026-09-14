package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	want := shellQuote(binary) + " __hook --project-dir " + shellQuote(root) + " --okf-binary " + shellQuote(filepath.Join(filepath.Dir(binary), "okf"))
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

func TestOKFSkillInstall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc binary"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents/skills/okf-agent-memory/SKILL.md"))
	if err != nil || !strings.Contains(string(raw), "'/opt/okf'") || strings.Contains(string(raw), "@@BINARY@@") {
		t.Fatalf("binary reference %s %v", raw, err)
	}
	for _, skill := range []string{"aidlc", "aidlc-cli"} {
		entry, err := os.ReadFile(filepath.Join(root, ".agents/skills", skill, "SKILL.md"))
		if err != nil || !strings.Contains(string(entry), "../okf-agent-memory/SKILL.md") {
			t.Fatalf("%s lacks OKF link: %v", skill, err)
		}
	}
}
