// Package core contains the initial documents embedded in the aidlc binary.
package core

import "embed"

// Files is used only for installation and initialization, never as a runtime fallback.
//
//go:embed knowledge adr-template.md skills agents workflow/stage-graph.json workflow/stages
var Files embed.FS
