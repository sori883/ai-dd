package codex

import (
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness"
)

// Distribution returns the initial Codex files for a resolved root and binary.
func Distribution(root, binary string) ([]harness.Asset, error) {
	hooks, err := hookConfiguration(root, binary)
	if err != nil {
		return nil, err
	}
	content, err := contentAssets(core.Files, Files, binary)
	if err != nil {
		return nil, err
	}
	manifest := harness.Manifest{
		Mappings: []harness.Mapping{
			{Files: core.Files, Source: "adr-template.md", Destination: "aidlc/templates/adr.md"},
			{Files: core.Files, Source: "knowledge", Destination: "aidlc/spaces/default/knowledge", Tree: true},
		},
		Generated: append(content, harness.Asset{Path: ".codex/hooks.json", Data: hooks}),
	}
	return manifest.Render(binary)
}
