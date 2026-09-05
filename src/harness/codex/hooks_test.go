package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksSourceRoutesUserPromptSubmitToBuiltAidlc(t *testing.T) {
	content, err := os.ReadFile("hooks.json")
	if err != nil {
		t.Fatalf("ReadFile(hooks.json): %v", err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("Unmarshal(hooks.json): %v", err)
	}
	entries := document.Hooks["UserPromptSubmit"]
	if len(entries) != 1 || len(entries[0].Hooks) != 1 {
		t.Fatalf("UserPromptSubmit hooks = %#v, want one command hook", entries)
	}
	hook := entries[0].Hooks[0]
	if hook.Type != "command" {
		t.Errorf("UserPromptSubmit hook type = %q, want command", hook.Type)
	}
	if hook.Command != "aidlc __codex-user-prompt-submit" {
		t.Errorf("UserPromptSubmit command = %q, want PATH aidlc hidden command", hook.Command)
	}
	for _, forbidden := range []string{"bun", "node", "deno", "tsx"} {
		if strings.Contains(strings.ToLower(hook.Command), forbidden) {
			t.Errorf("UserPromptSubmit command uses external runtime %q: %q", forbidden, hook.Command)
		}
	}
}

func TestHooksSourceCanBePlacedInFreshProject(t *testing.T) {
	content, err := os.ReadFile("hooks.json")
	if err != nil {
		t.Fatalf("ReadFile(hooks.json): %v", err)
	}
	project := t.TempDir()
	configDir := filepath.Join(project, ".codex")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(.codex): %v", err)
	}
	placed := filepath.Join(configDir, "hooks.json")
	if err := os.WriteFile(placed, content, 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", placed, err)
	}

	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	placedContent, err := os.ReadFile(placed)
	if err != nil {
		t.Fatalf("ReadFile(placed hooks): %v", err)
	}
	if err := json.Unmarshal(placedContent, &document); err != nil {
		t.Fatalf("Unmarshal(placed hooks): %v", err)
	}
	entries := document.Hooks["UserPromptSubmit"]
	if len(entries) != 1 || len(entries[0].Hooks) != 1 {
		t.Fatalf("placed UserPromptSubmit hooks = %#v, want one command hook", entries)
	}
	if got := entries[0].Hooks[0].Command; got != "aidlc __codex-user-prompt-submit" {
		t.Fatalf("placed UserPromptSubmit command = %q, want PATH aidlc hidden command", got)
	}
}
