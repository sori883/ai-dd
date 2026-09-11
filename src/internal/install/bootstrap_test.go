package install_test

import (
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/minimal"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallBootstrapBinaryPathBudget(t *testing.T) {
	for name, binary := range map[string]string{
		"observed":      "/Users/const/sori883/ai-dd-validation/hook-reliability-169/candidate-ca6893a/aidlc",
		"long_absolute": "/" + strings.Repeat("directory/", 50) + "binary/aidlc",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if _, err := install.Codex(root, binary); err != nil {
				t.Fatal(err)
			}
			skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(skill), "担当の初回起動前") {
				t.Error("missing initialization guidance for all native roles")
			}
			if len(skill) > 4096 {
				t.Errorf("deployed skill = %d bytes, limit 4096 (binary %d bytes)", len(skill), len(binary))
			}
			out, err := (minimal.Service{Root: root, Binary: binary}).Hook(minimal.HookInput{Event: "SessionStart", Session: "bootstrap"})
			if err != nil {
				t.Fatal(err)
			}
			specific, ok := out["hookSpecificOutput"].(map[string]any)
			if !ok || out["continue"] == false {
				t.Fatalf("SessionStart rejected: %v", out)
			}
			context, ok := specific["additionalContext"].(string)
			if !ok || !strings.Contains(context, string(skill)) || !strings.Contains(context, "Required Rules are NOT loaded") {
				t.Fatalf("missing bootstrap context: %v", out)
			}
			t.Logf("binary=%d bytes, deployed skill=%d bytes", len(binary), len(skill))
		})
	}
}
