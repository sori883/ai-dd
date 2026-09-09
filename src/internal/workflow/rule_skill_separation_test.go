package workflow

import (
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"testing"
)

func TestRuleSkillSeparationRuleReference(t *testing.T) {
	for _, mode := range []string{"valid", "path", "type", "version", "match", "count", "role", "accepted_at", "invalid metadata"} {
		t.Run(mode, func(t *testing.T) {
			r := Reference{Path: "${knowledge_root}/rules/rule.md", Metadata: &okfmemory.DocumentMatch{Type: "Rule"}, Version: "current"}
			switch mode {
			case "path":
				r.Path = "${knowledge_root}/rules/other.md"
			case "type":
				r.Metadata.Type = "Knowledge"
			case "version":
				r.Version = "accepted"
			case "match":
				r.Match = &okfmemory.DocumentMatch{Type: "Rule"}
			case "count":
				r.Count = "one"
			case "role":
				r.Role = "rules"
			case "accepted_at":
				r.AcceptedAt = "discovery"
			case "invalid metadata":
				empty := ""
				r.Metadata.Title = &empty
			}
			if err := validateReference(r, false); (err == nil) != (mode == "valid") {
				t.Fatalf("%s: %v", mode, err)
			}
		})
	}
}
