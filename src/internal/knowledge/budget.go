package knowledge

import (
	"fmt"
)

const (
	maxRosterPathBytes    = 8 * 1024
	maxRosterWarningBytes = 6 * 1024
)

func deduplicatePaths(candidates []candidate) []string {
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.display]; ok {
			continue
		}
		seen[candidate.display] = struct{}{}
		paths = append(paths, candidate.display)
	}
	return paths
}

func boundRoster(paths, warnings []string) ([]string, []string) {
	boundedPaths := make([]string, 0, len(paths))
	for _, candidate := range paths {
		candidatePaths := append(append([]string{}, boundedPaths...), candidate)
		if jsonStringArraySize(candidatePaths) > maxRosterPathBytes {
			break
		}
		boundedPaths = append(boundedPaths, candidate)
	}
	if omitted := len(paths) - len(boundedPaths); omitted > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Warning: %d optional persona/knowledge path(s) were omitted because there was no room to pass them all (inline_context_paths is capped at %d bytes). Configure fewer knowledge files if this matters; the stage runs without the omitted optional context.",
			omitted,
			maxRosterPathBytes,
		))
	}
	return boundedPaths, boundWarnings(warnings)
}

func boundWarnings(warnings []string) []string {
	if jsonStringArraySize(warnings) <= maxRosterWarningBytes {
		return append([]string{}, warnings...)
	}

	kept := make([]string, 0, len(warnings))
	for index, warning := range warnings {
		omitted := len(warnings) - index - 1
		candidate := append(append([]string{}, kept...), warning)
		if omitted > 0 {
			candidate = append(candidate, warningSummary(omitted))
		}
		if jsonStringArraySize(candidate) > maxRosterWarningBytes {
			break
		}
		kept = append(kept, warning)
	}
	omitted := len(warnings) - len(kept)
	return append(kept, warningSummary(omitted))
}

func warningSummary(omitted int) string {
	return fmt.Sprintf(
		"Warning: %d additional optional persona/knowledge warning(s) were omitted from this directive. Inspect the configured context directories and repair missing, unreadable, or invalid UTF-8 files.",
		omitted,
	)
}

func jsonStringArraySize(values []string) int {
	size := 2
	for index, value := range values {
		if index > 0 {
			size++
		}
		size += 2 + jsonStringSize(value)
	}
	return size
}

func jsonStringSize(value string) int {
	size := 0
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch character {
		case '"', '\\':
			size += 2
		case '\b', '\f', '\n', '\r', '\t':
			size += 2
		default:
			if character < 0x20 {
				size += len("\\u00") + 2
				continue
			}
			size++
		}
	}
	return size
}
