package codex

import (
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness"
)

// Distribution returns the initial Codex files for a resolved root and binary.
func Distribution(root, binary string) ([]harness.Asset, error) {
	return DistributionFrom(root, SiblingBinaries(binary), core.Files, Files)
}
