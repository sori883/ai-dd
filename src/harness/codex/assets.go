// Package codex contains the Codex documents embedded for fresh installations.
package codex

import (
	"embed"
	"io/fs"
	"strings"
)

// Files contains source assets mapped to Codex discovery paths by the installer.
//
//go:embed SKILL.md aidlc-cli aidlc-okf agents stage-skills
var Files embed.FS

// WorkflowMarkdown reports whether name is an exact deployed Markdown asset.
func WorkflowMarkdown(name string) bool {
	switch name {
	case ".agents/skills/aidlc/SKILL.md", ".agents/skills/aidlc-cli/SKILL.md", ".agents/skills/aidlc-okf/SKILL.md":
		return true
	}
	const prefix = ".agents/skills/"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".md") {
		return false
	}
	source := "stage-skills/" + strings.TrimPrefix(name, prefix)
	if !fs.ValidPath(source) {
		return false
	}
	info, err := fs.Stat(Files, source)
	return err == nil && info.Mode().IsRegular()
}
