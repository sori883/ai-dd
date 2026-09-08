// Package minimal contains the initial documents embedded in the aidlc binary.
package minimal

import "embed"

// Files is used only for installation and initialization, never as a runtime fallback.
//
//go:embed knowledge adr-template.md
var Files embed.FS
