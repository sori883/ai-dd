// Package scope reads AI-DLC scope metadata without modifying its filesystem.
package scope

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"unicode/utf16"
)

// ReviewCap limits the effective review class for a scope.
type ReviewCap string

const (
	// ReviewCapAdversarial preserves adversarial reviews.
	ReviewCapAdversarial ReviewCap = "adversarial"
	// ReviewCapAdvisory caps reviews at one advisory pass.
	ReviewCapAdvisory ReviewCap = "advisory"
	// ReviewCapNone disables reviewer dispatch.
	ReviewCapNone ReviewCap = "none"
)

// Metadata is the frontmatter metadata for one scope definition.
type Metadata struct {
	Name            string
	Plugin          string
	Depth           string
	Description     string
	Keywords        []string
	TestStrategy    string
	Runner          *bool
	Skeleton        bool
	ReviewCap       ReviewCap
	FreeformDefault bool
}

// ReadAll reads scope metadata from the root of scopesFS.
func ReadAll(scopesFS fs.FS) ([]Metadata, error) {
	if scopesFS == nil {
		return nil, errors.New("read scopes: nil filesystem")
	}
	entries, err := fs.ReadDir(scopesFS, ".")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Metadata{}, nil
		}
		return nil, fmt.Errorf("read scopes directory: %w", err)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return slices.Compare(
			utf16.Encode([]rune(a.Name())),
			utf16.Encode([]rune(b.Name())),
		)
	})

	metadata := make([]Metadata, 0, len(entries))
	nameToFile := make(map[string]string, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		body, err := fs.ReadFile(scopesFS, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read scope file %q: %w", entry.Name(), err)
		}
		frontmatter, ok := frontmatterBlock(string(body))
		if !ok {
			return nil, fmt.Errorf("scope file %q: missing frontmatter", entry.Name())
		}
		item, err := parseMetadata(frontmatter, entry.Name())
		if err != nil {
			return nil, err
		}
		if previousFile, exists := nameToFile[item.Name]; exists {
			return nil, fmt.Errorf(
				"duplicate scope name %q in %q: already declared in %q",
				item.Name,
				entry.Name(),
				previousFile,
			)
		}
		nameToFile[item.Name] = entry.Name()
		metadata = append(metadata, item)
	}
	return metadata, nil
}

func parseMetadata(frontmatter, filename string) (Metadata, error) {
	item := Metadata{
		Name:         scalarField(frontmatter, "name"),
		Depth:        scalarField(frontmatter, "depth"),
		Description:  scalarField(frontmatter, "description"),
		Keywords:     listField(frontmatter, "keywords"),
		Plugin:       scalarField(frontmatter, "plugin"),
		TestStrategy: scalarField(frontmatter, "testStrategy"),
		Runner:       booleanPointer(scalarField(frontmatter, "runner")),
	}
	if item.Name == "" {
		return Metadata{}, fmt.Errorf("scope file %q: missing required frontmatter: name", filename)
	}
	if strings.HasPrefix(item.Plugin, "aidlc-") {
		return Metadata{}, fmt.Errorf(
			"scope file %q declares plugin %q: the aidlc- prefix is reserved",
			filename,
			item.Plugin,
		)
	}
	switch value := scalarField(frontmatter, "skeleton"); value {
	case "":
	case "on":
		item.Skeleton = true
	case "off":
	default:
		return Metadata{}, fmt.Errorf(
			"scope file %q has invalid skeleton value %q: expected on or off",
			filename,
			value,
		)
	}
	item.FreeformDefault = scalarField(frontmatter, "freeform_default") == "true"
	switch value := scalarField(frontmatter, "review_cap"); value {
	case "":
	case string(ReviewCapAdversarial), string(ReviewCapAdvisory), string(ReviewCapNone):
		item.ReviewCap = ReviewCap(value)
	default:
		return Metadata{}, fmt.Errorf(
			"scope file %q has invalid review_cap value %q: expected adversarial, advisory, or none",
			filename,
			value,
		)
	}
	return item, nil
}

func booleanPointer(value string) *bool {
	if value != "true" && value != "false" {
		return nil
	}
	parsed := value == "true"
	return &parsed
}

func frontmatterBlock(body string) (string, bool) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.HasPrefix(body, "---\n") {
		return "", false
	}
	frontmatter, _, ok := strings.Cut(body[len("---\n"):], "\n---")
	return frontmatter, ok
}

func scalarField(frontmatter, key string) string {
	prefix := key + ":"
	for line := range strings.Lines(frontmatter) {
		line = strings.TrimSuffix(line, "\n")
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if raw == ">" || raw == "|" || raw == ">-" || raw == "|-" {
			return ""
		}
		return unquoteScalar(raw)
	}
	return ""
}

func listField(frontmatter, key string) []string {
	lines := strings.Split(frontmatter, "\n")
	prefix := key + ":"
	for index, line := range lines {
		if strings.TrimRight(line, " \t") != prefix {
			continue
		}
		if items, ok := blockListItems(lines[index+1:]); ok {
			return items
		}
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		raw := strings.TrimLeft(strings.TrimPrefix(line, prefix), " \t")
		if strings.HasPrefix(raw, "[") {
			return parseInlineList(raw)
		}
	}
	return []string{}
}

func blockListItems(lines []string) ([]string, bool) {
	items := []string{}
	matched := false
	for _, line := range lines {
		withoutIndent := strings.TrimLeft(line, " \t")
		if withoutIndent == line || len(withoutIndent) < 2 || withoutIndent[0] != '-' {
			break
		}
		if withoutIndent[1] != ' ' && withoutIndent[1] != '\t' {
			break
		}
		item := strings.TrimSpace(withoutIndent[2:])
		if item == "" {
			break
		}
		matched = true
		item = trimListQuotes(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items, matched
}

func parseInlineList(raw string) []string {
	text := strings.TrimSpace(raw)
	if text == "" || text == "[]" {
		return []string{}
	}

	closeIndex := -1
	var quote byte
	for index := 1; index < len(text); index++ {
		char := text[index]
		if quote != 0 {
			if quote == '"' && char == '\\' {
				index++
			} else if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ']':
			closeIndex = index
			index = len(text)
		}
	}
	if quote != 0 || closeIndex == -1 {
		return []string{}
	}
	suffix := strings.TrimLeft(text[closeIndex+1:], " \t")
	if suffix != "" && !strings.HasPrefix(suffix, "#") {
		return []string{}
	}

	body := text[1:closeIndex]
	items := []string{}
	start := 0
	quote = 0
	for index := 0; index < len(body); index++ {
		char := body[index]
		if quote != 0 {
			if quote == '"' && char == '\\' {
				index++
			} else if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ',':
			items = append(items, body[start:index])
			start = index + 1
		}
	}
	if quote != 0 {
		return []string{}
	}
	items = append(items, body[start:])

	parsed := make([]string, 0, len(items))
	for _, item := range items {
		item = unquoteScalar(item)
		if item != "" {
			parsed = append(parsed, item)
		}
	}
	return parsed
}

func unquoteScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	first, last := value[0], value[len(value)-1]
	if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
		return value[1 : len(value)-1]
	}
	return value
}

func trimListQuotes(value string) string {
	if value != "" && (value[0] == '\'' || value[0] == '"') {
		value = value[1:]
	}
	if value != "" && (value[len(value)-1] == '\'' || value[len(value)-1] == '"') {
		value = value[:len(value)-1]
	}
	return value
}
