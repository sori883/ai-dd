package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestRelocationInstallDispatch(t *testing.T) {
	s, _ := setup(t)
	s.Binary = "/new/aidlc"
	oldRoot, err := filepath.EvalSymlinks(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	r := cli.CommandRequest{Command: "install", Action: "codex", Relocate: true, FromProjectDir: oldRoot, FromBinary: "/opt/aidlc"}
	out, err := s.Execute(r)
	if err != nil {
		t.Fatalf("relocate %s %v", out, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, ".codex/hooks.json"))
	if err != nil || !strings.Contains(string(raw), "/new/aidlc") {
		t.Fatal("not wired", err)
	}
}
func TestRelocationHook(t *testing.T) {
	s, st := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if _, err := s.Execute(cli.CommandRequest{Command: "session", Action: "bind", Target: st.ID, Space: "default", Session: "session"}); err != nil {
		t.Fatal(err)
	}
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if err := os.WriteFile(filepath.Join(s.Root, "request.json"), []byte(`{"coordinator_session":"session"}`), 0600); err != nil {
		t.Fatal(err)
	}
	command := "/opt/aidlc unit reassign " + st.ID + " --space default --expect 1 --file request.json"
	if deny(hook(t, s, "PreToolUse", "Bash", "reassign", command, false)) {
		t.Fatal("same Intent reassign not recognized")
	}
	for _, bad := range []string{strings.Replace(command, st.ID, strings.Repeat("b", 32), 1), strings.Replace(command, "default", "other", 1), "/opt/aidlc install codex --relocate --project-dir " + s.Root + " --from-project-dir /old --from-binary /old/aidlc"} {
		if !deny(hook(t, s, "PreToolUse", "Bash", "bad", bad, false)) {
			t.Fatal("expanded bootstrap exception")
		}
	}
	state, err := s.Inspect("session")
	if err != nil {
		t.Fatal(err)
	}
	state.Tool = "running"
	if err := s.save("session", state); err != nil {
		t.Fatal(err)
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "overlap", command, false)) {
		t.Fatal("reassign bypassed active tool")
	}
}
