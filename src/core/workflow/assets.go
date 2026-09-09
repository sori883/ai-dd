// Package workflow contains the fresh-install workflow definition.
package workflow

import "embed"

// Files contains the graph and stage procedures.
//
//go:embed stage-graph.json stages/*.md
var Files embed.FS
