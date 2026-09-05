package graph

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// RouteHash returns the SHA-256 hash of the selected canonical graph node and the
// enabled stages routed to execute in the named scope.
func (s Snapshot) RouteHash(stageSlug, scope string) (string, error) {
	node, ok := s.routeNodes[stageSlug]
	if !ok {
		return "", fmt.Errorf("route hash: unknown stage %q", stageSlug)
	}
	scopeRoute, ok := s.scopes[scope]
	if !ok {
		return "", fmt.Errorf("route hash: unknown scope %q", scope)
	}

	scopeStages := make([]Stage, 0, len(s.stages))
	for _, stage := range s.stages {
		if scopeRoute.Action(stage.Slug) == ActionExecute {
			scopeStages = append(scopeStages, stage)
		}
	}
	sort.SliceStable(scopeStages, func(i, j int) bool {
		return stageNumberBefore(scopeStages[i].Number, scopeStages[j].Number)
	})

	var wire bytes.Buffer
	wire.WriteString(`{"node":`)
	wire.Write(node)
	wire.WriteString(`,"scopeStages":[`)
	for index, stage := range scopeStages {
		if index > 0 {
			wire.WriteByte(',')
		}
		encodedSlug, err := json.Marshal(stage.Slug)
		if err != nil {
			return "", fmt.Errorf("route hash: encode stage %q: %w", stage.Slug, err)
		}
		wire.Write(encodedSlug)
	}
	wire.WriteString(`]}`)

	digest := sha256.Sum256(wire.Bytes())
	return hex.EncodeToString(digest[:]), nil
}
