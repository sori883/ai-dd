package knowledge

import (
	"strings"
	"unicode/utf8"
)

var minimalKnowledge = map[string]map[string]map[string]struct{}{
	"intent-capture": {
		"aidlc-shared": {
			"ai-dlc-principles.md": {},
			"rules-reading.md":     {},
			"verification.md":      {},
		},
		"aidlc-product-agent": {
			"requirements-elicitation.md": {},
			"requirements-guide.md":       {},
		},
		"aidlc-architect-agent": {
			"architecture-guide.md": {},
		},
	},
	"requirements-analysis": {
		"aidlc-shared": {
			"ai-dlc-principles.md": {},
			"brownfield.md":        {},
			"rules-reading.md":     {},
			"verification.md":      {},
		},
		"aidlc-product-agent": {
			"requirements-elicitation.md": {},
			"requirements-guide.md":       {},
		},
	},
}

var shippedKnowledge = map[string]map[string]struct{}{
	"aidlc-shared": {
		"ai-dlc-principles.md":         {},
		"audit-format.md":              {},
		"brownfield.md":                {},
		"knowledge-readme-template.md": {},
		"memory-template.md":           {},
		"rules-reading.md":             {},
		"state-template.md":            {},
		"verification.md":              {},
		"worktree-info-schema.md":      {},
	},
	"aidlc-product-agent": {
		"functional-design-guide.md":   {},
		"market-research-methods.md":   {},
		"prioritization-frameworks.md": {},
		"product-guide.md":             {},
		"requirements-elicitation.md":  {},
		"requirements-guide.md":        {},
		"user-story-patterns.md":       {},
	},
	"aidlc-architect-agent": {
		"adr-template.md":          {},
		"architecture-guide.md":    {},
		"architecture-patterns.md": {},
		"ddd-patterns.md":          {},
		"nfr-design-guide.md":      {},
		"nfr-design-patterns.md":   {},
	},
}

func selectCandidates(
	candidates []candidate,
	depth string,
	enabledPlugins []string,
	owners pluginOwners,
) []candidate {
	selected := make([]candidate, 0, len(candidates))
	minimal := strings.ToLower(trimECMAScriptWhitespace(depth)) == "minimal"
	for _, candidate := range candidates {
		if !candidate.readable {
			continue
		}
		if minimal && candidate.sourceKind == frameworkContext && !candidate.persona {
			if !minimalKnowledgeFile(candidate, owners, enabledPlugins) {
				continue
			}
		}
		selected = append(selected, candidate)
	}
	return selected
}

func minimalKnowledgeFile(
	candidate candidate,
	owners pluginOwners,
	enabledPlugins []string,
) bool {
	pathOwners, hasOwners := owners[candidate.relative]
	if hasOwners {
		for plugin := range pathOwners {
			if pluginEnabled(enabledPlugins, plugin) {
				return true
			}
		}
		return false
	}

	selectedForStage := minimalKnowledge[candidate.stage][candidate.owner]
	if selectedForStage == nil {
		return true
	}
	shippedForOwner := shippedKnowledge[candidate.owner]
	if _, shipped := shippedForOwner[candidate.ownerRelative]; !shipped {
		return true
	}
	_, selected := selectedForStage[candidate.ownerRelative]
	return selected
}

func pluginEnabled(enabledPlugins []string, plugin string) bool {
	if enabledPlugins == nil {
		return true
	}
	for _, enabled := range enabledPlugins {
		if enabled == plugin {
			return true
		}
	}
	return false
}

func trimECMAScriptWhitespace(value string) string {
	start := 0
	for start < len(value) {
		runeValue, size := utf8.DecodeRuneInString(value[start:])
		if !isECMAScriptTrimRune(runeValue) {
			break
		}
		start += size
	}
	end := len(value)
	for end > start {
		runeValue, size := utf8.DecodeLastRuneInString(value[:end])
		if !isECMAScriptTrimRune(runeValue) {
			break
		}
		end -= size
	}
	return value[start:end]
}

func isECMAScriptTrimRune(value rune) bool {
	switch value {
	case '\u0009', '\u000a', '\u000b', '\u000c', '\u000d',
		' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000', '\ufeff':
		return true
	}
	return value >= '\u2000' && value <= '\u200a'
}
