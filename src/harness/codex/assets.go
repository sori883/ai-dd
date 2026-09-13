// Package codex contains the Codex documents embedded for fresh installations.
package codex

import (
	"embed"
	"github.com/sori883/ai-dd/src/core"
	"io/fs"
	"strings"
)

// Files contains source assets mapped to Codex discovery paths by the installer.
//
//go:embed skills agents
var Files embed.FS

// WorkflowMarkdown reports whether name is an exact deployed Markdown asset.
func WorkflowMarkdown(name string) bool {
	if !fs.ValidPath(name) || !strings.HasPrefix(name, ".agents/skills/") || !strings.HasSuffix(name, ".md") {
		return false
	}
	assets, err := contentAssets(core.Files, Files, "aidlc")
	if err != nil {
		return false
	}
	for _, asset := range assets {
		if asset.Path == name {
			return true
		}
	}
	return false
}
