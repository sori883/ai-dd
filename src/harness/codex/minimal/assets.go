// Package minimal contains the Codex documents embedded for fresh installations.
package minimal

import "embed"

// Files contains source assets mapped to Codex discovery paths by the installer.
//
//go:embed SKILL.md agents
var Files embed.FS
