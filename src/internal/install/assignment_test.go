package install

import (
	"bytes"
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

func TestAssignmentContractDocumentRules(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(bootstrap) > 4096 {
		t.Fatalf("bootstrap %d bytes exceeds 4 KiB", len(bootstrap))
	}
	if !bytes.Contains(bootstrap, []byte("文書記録規約は [aidlc-okf](../aidlc-okf/SKILL.md)")) {
		t.Fatal("bootstrap does not direct readers to document recording rules")
	}
	detail, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc-cli/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	okf, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc-okf/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	detail = append(detail, okf...)
	for _, rule := range []string{"ADRは判断のwhy・代替案・影響", "毎操作の日誌や一律ADRは作らない", "不要なら理由をreviewする", "outputsは期待する文書だけで、なければなし", "プログラム・テストコード・commitを文書outputsへ列挙せず", "共有文書のID/日時を形式だけのために更新しない"} {
		if !bytes.Contains(detail, []byte(rule)) {
			t.Errorf("deployed document rule missing: %s", rule)
		}
	}
}
