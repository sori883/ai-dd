package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/workflow"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestCodeKBDistribution(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateSpace(workspace.RootInput{ExplicitDir: root}, "Team"); err != nil {
		t.Fatal(err)
	}
	for _, space := range []string{"default", "team"} {
		t.Run(space, func(t *testing.T) {
			bundle := filepath.Join(root, "aidlc/spaces", space, "knowledge")
			raw, err := os.ReadFile(filepath.Join(bundle, "index.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "(codekb/index.md)") {
				t.Error("bundle does not link codekb index")
			}
			for _, name := range []string{"codekb/index.md", "design/index.md", "adr/index.md", "rules/entry.md"} {
				if _, err := os.Stat(filepath.Join(bundle, name)); err != nil {
					t.Errorf("missing linked %s: %v", name, err)
				}
			}
			if _, err := os.Stat(filepath.Join(bundle, "knowledge")); !os.IsNotExist(err) {
				t.Errorf("old inner knowledge folder remains: %v", err)
			}
		})
	}
}

func TestCodeKBGuidance(t *testing.T) {
	root := t.TempDir()
	if _, err := Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	commonPath := filepath.Join(root, ".agents/skills/aidlc-cli/SKILL.md")
	common, err := os.ReadFile(commonPath)
	if err != nil {
		t.Fatal(err)
	}
	_, link, found := strings.Cut(string(common), "[aidlc-okf](")
	if !found {
		t.Fatal("common operations lack OKF skill reference")
	}
	link, _, found = strings.Cut(link, ")")
	if !found {
		t.Fatal("unterminated OKF skill reference")
	}
	knowledge, err := os.ReadFile(filepath.Join(filepath.Dir(commonPath), link))
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"discovery", "planning", "tdd", "integration"} {
		raw, err := os.ReadFile(filepath.Join(root, "aidlc/workflow/stages", stage+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "aidlc-cli") {
			t.Fatal("stage lacks common-operation reference")
		}
		raw = append(raw, common...)
		raw = append(raw, knowledge...)
		if (!strings.Contains(string(raw), "codekb/NAME") || !strings.Contains(string(raw), "memory create --help")) || strings.Contains(string(raw), "memory create knowledge/") {
			t.Errorf("%s uses old Concept guidance", stage)
		}
	}
	definition, err := workflow.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range definition.Procedures["integration"].Outputs {
		if ref.Role == "current_analysis" && ref.Path != "${knowledge_root}/codekb/current-analysis.md" {
			t.Errorf("analysis output: %s", ref.Path)
		}
		if ref.Role == "architecture" && ref.Path != "${knowledge_root}/codekb/architecture.md" {
			t.Errorf("architecture output: %s", ref.Path)
		}
	}
	if !strings.Contains(string(knowledge), "codekb/") {
		t.Fatal("workflow omits current knowledge folder")
	}
}
