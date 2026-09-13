package install

import (
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"io/fs"
	"path/filepath"
)

func CodexFrom(root string, b codex.Binaries, common, host fs.FS) (Result, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Result{}, err
	}
	assets, err := codex.DistributionFrom(root, b, common, host)
	if err != nil {
		return Result{}, err
	}
	return installAssets(root, assets)
}
func RelocateFrom(root, fromRoot string, b, from codex.Binaries, common, host fs.FS) (RelocationResult, error) {
	return relocateFrom(root, fromRoot, b, from, common, host, filestore.WriteFile)
}
