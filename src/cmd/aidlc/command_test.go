package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/install"
)

func TestMainHookCommand(t *testing.T) {
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "stdin.json")
	if err := os.WriteFile(p, []byte(`{"hook_event_name":"SessionStart","session_id":"naming"}`), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = previous; input.Close() })
	raw, err := executeCommand(cli.CommandRequest{Command: "__hook", ProjectDir: root})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Specific struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Specific.Event != "SessionStart" || !strings.Contains(out.Specific.Context, "Session: naming.") {
		t.Fatalf("bootstrap = %s", raw)
	}
}
