package codex

import (
	"github.com/sori883/ai-dd/src/core"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	"github.com/sori883/ai-dd/src/harness"
)

// Distribution returns the initial Codex files for a resolved root and binary.
func Distribution(root, binary string) ([]harness.Asset, error) {
	hooks, err := hookConfiguration(root, binary)
	if err != nil {
		return nil, err
	}
	manifest := harness.Manifest{
		Mappings: []harness.Mapping{
			{Files: core.Files, Source: "adr-template.md", Destination: "aidlc/templates/adr.md"},
			{Files: core.Files, Source: "knowledge", Destination: "aidlc/spaces/default/knowledge", Tree: true},
			{Files: coreworkflow.Files, Source: ".", Destination: "aidlc/workflow", Tree: true},
			{Files: Files, Source: "SKILL.md", Destination: ".agents/skills/aidlc/SKILL.md"},
			{Files: Files, Source: "aidlc-cli/SKILL.md", Destination: ".agents/skills/aidlc-cli/SKILL.md"},
			{Files: Files, Source: "aidlc-okf/SKILL.md", Destination: ".agents/skills/aidlc-okf/SKILL.md"},
			{Files: Files, Source: "stage-skills", Destination: ".agents/skills", Tree: true},
			{Files: Files, Source: "agents", Destination: ".codex/agents", Tree: true},
		},
		Generated: []harness.Asset{{Path: ".codex/hooks.json", Data: hooks}},
	}
	return manifest.Render(binary)
}
