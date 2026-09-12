// Package codex contains the Codex documents embedded for fresh installations.
package codex

import "embed"

// Files contains source assets mapped to Codex discovery paths by the installer.
//
//go:embed SKILL.md aidlc-cli agents
var Files embed.FS
