package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagePlannerDistribution(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".codex/agents/aidlc-stage-planner.toml"))
	if err != nil {
		t.Fatal("planner missing", err)
	}
	for _, want := range []string{`name = "aidlc-stage-planner"`, `sandbox_mode = "read-only"`, "PLAN.json", "省略理由", "完了prefix", "メインAI", "承認", "researcher", "期待する文書（なければなし）", "プログラム・テストコード・commitを文書outputsへ列挙しない", "検証証拠の必要性は文書outputsとは別に説明する"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("planner instruction missing %s", want)
		}
	}
	for _, name := range []string{"aidlc", "aidlc-cli"} {
		raw, err := os.ReadFile(filepath.Join(root, ".agents/skills", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if name == "aidlc" && !strings.Contains(string(raw), "aidlc-stage-planner") {
			t.Errorf("%s omits planner", name)
		}
	}
}
