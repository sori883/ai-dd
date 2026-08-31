package workspace

import (
	"io/fs"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"
)

const defaultSpace = "default"

// Space describes a listed name and whether it matches the active selection.
// The synthetic "default" entry does not imply that a directory exists.
type Space struct {
	Name   string
	Active bool
}

// ActiveSpace reads aidlc/active-space from projectFS, rooted at the project
// directory. It trims JavaScript whitespace and returns "default" for blank
// content or any read error, including errors accompanied by partial data.
// The name is not validated or used for further filesystem access.
// No files are created or changed.
func ActiveSpace(projectFS fs.FS) string {
	data, err := fs.ReadFile(projectFS, "aidlc/active-space")
	if err != nil {
		return defaultSpace
	}
	name := strings.TrimFunc(string(data), isJavaScriptWhitespace)
	if name == "" {
		return defaultSpace
	}
	return name
}

// ListSpaces returns the immediate directories under aidlc/spaces in projectFS,
// rooted at the project directory, plus "default" exactly once. Names are sorted
// by JavaScript UTF-16 code-unit order. A nil activeOverride reads [ActiveSpace];
// otherwise the override is used verbatim, including an empty string.
// Active is an exact name match and may be false for every entry.
//
// ReadDir errors yield only "default"; a Stat error ends enumeration but retains
// earlier directories. Stat follows symlinks as supported by projectFS; an
// os.DirFS filesystem does not prevent links outside its root.
// No files are created or changed.
func ListSpaces(projectFS fs.FS, activeOverride *string) []Space {
	var active string
	if activeOverride == nil {
		active = ActiveSpace(projectFS)
	} else {
		active = *activeOverride
	}
	spaces := []Space{{Name: defaultSpace, Active: active == defaultSpace}}
	entries, err := fs.ReadDir(projectFS, "aidlc/spaces")
	if err != nil {
		return spaces
	}
	seen := map[string]struct{}{defaultSpace: {}}
	for _, entry := range entries {
		name := entry.Name()
		info, err := fs.Stat(projectFS, "aidlc/spaces/"+name)
		if err != nil {
			break
		}
		if !info.IsDir() {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		spaces = append(spaces, Space{Name: name, Active: name == active})
		seen[name] = struct{}{}
	}
	// JavaScript sorts strings by UTF-16 code units rather than UTF-8 bytes.
	slices.SortFunc(spaces, func(a, b Space) int {
		return slices.Compare(utf16.Encode([]rune(a.Name)), utf16.Encode([]rune(b.Name)))
	})
	return spaces
}

// JavaScript trim includes the byte order mark but excludes the next line
// character; unicode.IsSpace alone has the opposite behavior for these two.
func isJavaScriptWhitespace(r rune) bool {
	switch r {
	case '\ufeff':
		return true
	case '\u0085':
		return false
	default:
		return unicode.IsSpace(r)
	}
}
