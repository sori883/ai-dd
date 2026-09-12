package naturaljapanese

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func identity(f Finding) string {
	if member(f.Category, "low_burstiness", "low_sentence_variance", "uniform_paragraph_structure", "low_lexical_diversity_ttr", "low_lexical_diversity_mtld") {
		return f.Category
	}
	normalized := []rune(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) {
			return -1
		}
		return r
	}, f.Excerpt))
	return f.Category + "\x00" + string(normalized[:min(20, len(normalized))])
}
func Compare(current Report, raw []byte) (Report, error) {
	var old Report
	if err := json.Unmarshal(raw, &old); err != nil {
		return current, fmt.Errorf("read baseline json: %w", err)
	}
	if old.SchemaVersion != current.SchemaVersion || old.Engine != current.Engine || old.Dictionary != current.Dictionary {
		return current, fmt.Errorf("baseline schema, engine or dictionary is incompatible")
	}
	if old.Findings == nil || old.Stats == nil || old.File == "" {
		return current, fmt.Errorf("baseline requires file, stats and findings")
	}
	for _, f := range old.Findings {
		if f.Excerpt == "" {
			return current, fmt.Errorf("baseline finding requires non-empty excerpt")
		}
		if f.Line < 1 || !member(f.Category, Categories...) || !member(f.Severity, "info", "warn", "critical") || f.Detail == "" {
			return current, fmt.Errorf("invalid baseline finding")
		}
	}
	used := make([]bool, len(old.Findings))
	current.Comparison = map[string]int{"new": 0, "persisting": 0, "resolved": 0}
	current.Resolved = []Finding{}
	for i := range current.Findings {
		f := &current.Findings[i]
		f.Status = "new"
		for j, previous := range old.Findings {
			if !used[j] && identity(*f) == identity(previous) {
				used[j] = true
				f.Status = "persisting"
				break
			}
		}
		current.Comparison[f.Status]++
	}
	for i, f := range old.Findings {
		if !used[i] {
			f.Status = "resolved"
			current.Resolved = append(current.Resolved, f)
			current.Comparison["resolved"]++
		}
	}
	return current, nil
}
