package workspace

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode/utf16"
)

// ListIntentDirs returns direct child names with an aidlc-state.md marker,
// sorted in JavaScript UTF-16 code-unit order. Marker contents and types are
// not validated. A ReadDir error discards partial entries and returns a non-nil
// empty slice; an individual Stat error skips only that candidate.
//
// intentsFS must be non-nil and rooted at an already selected space's intents
// directory. The caller is responsible for the filesystem's containment policy.
func ListIntentDirs(intentsFS fs.FS) []string {
	names := []string{}
	entries, err := fs.ReadDir(intentsFS, ".")
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if _, err := fs.Stat(intentsFS, entry.Name()+"/aidlc-state.md"); err != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.SortFunc(names, func(a, b string) int {
		return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
	})
	return names
}

// ActiveIntent returns a selected name; its bool reports selection, not safety
// or existence. A non-empty explicit value is returned unchanged without any
// filesystem access or validation.
//
// Otherwise, it trims active-intent using JavaScript whitespace rules and
// selects the cursor only if it is an fs.ValidPath with an existing marker.
// Failed reads discard partial data. An unusable cursor falls back to exactly
// one candidate from ListIntentDirs, or returns ("", false).
//
// intentsFS has the same root and containment requirements as ListIntentDirs.
// Callers must validate explicit values before using them as paths.
func ActiveIntent(intentsFS fs.FS, explicit string) (string, bool) {
	if explicit != "" {
		return explicit, true
	}
	data, err := fs.ReadFile(intentsFS, "active-intent")
	if err == nil {
		name := strings.TrimFunc(string(data), isJavaScriptWhitespace)
		// Validate before joining so traversal is not normalized into a valid path.
		if fs.ValidPath(name) {
			if _, err := fs.Stat(intentsFS, path.Join(name, "aidlc-state.md")); err == nil {
				return name, true
			}
		}
	}
	if names := ListIntentDirs(intentsFS); len(names) == 1 {
		return names[0], true
	}
	return "", false
}
