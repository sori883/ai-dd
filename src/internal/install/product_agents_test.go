package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductAgentAssets(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	names := []string{"aidlc-researcher", "aidlc-requirements", "aidlc-worker", "aidlc-reviewer", "aidlc-stage-planner"}
	entries, err := os.ReadDir(filepath.Join(root, ".codex/agents"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Errorf("agent definitions=%d, want %d", len(entries), len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			placed, err := os.ReadFile(filepath.Join(root, ".codex/agents", name+".toml"))
			if err != nil {
				t.Fatal(err)
			}
			settings, encoded, ok := strings.Cut(string(placed), "developer_instructions = ")
			if !ok {
				t.Fatal("missing generated instructions")
			}
			var body string
			if err := json.Unmarshal([]byte(strings.TrimSpace(encoded)), &body); err != nil {
				t.Fatal(err)
			}
			sandbox := "read-only"
			if name == "aidlc-worker" {
				sandbox = "workspace-write"
			}
			if !strings.Contains(settings, `sandbox_mode = "`+sandbox+`"`) {
				t.Fatal("wrong permissions")
			}
			for _, want := range []string{"必要なRule全文", "共有stateとOKF Knowledge/ADRを直接更新しない", "既存の他担当・利用者の変更を保全", "send_messageのtarget"} {
				if !strings.Contains(body, want) {
					t.Errorf("%s missing %s", name, want)
				}
			}

		})
	}
}
