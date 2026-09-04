package knowledge

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type contextSource uint8

const (
	frameworkContext contextSource = iota
	spaceContext
)

type candidate struct {
	source        Source
	relative      string
	display       string
	stage         string
	owner         string
	ownerRelative string
	sourceKind    contextSource
	persona       bool
	readable      bool
}

func collectCandidates(input RosterInput, agents []string) ([]candidate, []string) {
	candidates := make([]candidate, 0)
	warnings := []string{}

	for _, agent := range agents {
		relative := path.Join("agents", agent+".md")
		candidates = append(candidates, candidate{
			source:     input.Framework,
			relative:   relative,
			display:    joinDisplay(input.Framework.DisplayPrefix, relative),
			stage:      input.Stage.Slug,
			sourceKind: frameworkContext,
			persona:    true,
		})
	}
	preflightCandidates(candidates, &warnings)

	frameworkGroups := []struct {
		relative string
		owner    string
	}{
		{relative: "knowledge/aidlc-shared", owner: "aidlc-shared"},
	}
	for _, agent := range agents {
		frameworkGroups = append(frameworkGroups, struct {
			relative string
			owner    string
		}{relative: path.Join("knowledge", agent), owner: agent})
	}
	for _, group := range frameworkGroups {
		found, foundWarnings := scanMarkdownDirectory(
			input.Framework,
			group.relative,
			group.owner,
			frameworkContext,
			true,
			input.Stage.Slug,
		)
		warnings = append(warnings, foundWarnings...)
		candidates = append(candidates, found...)
	}

	if input.SpaceKnowledge == nil {
		return candidates, warnings
	}
	space := *input.SpaceKnowledge
	spaceGroups := []struct {
		relative string
		owner    string
	}{
		{relative: "aidlc-shared", owner: "aidlc-shared"},
	}
	for _, agent := range agents {
		spaceGroups = append(spaceGroups, struct {
			relative string
			owner    string
		}{relative: agent, owner: agent})
	}
	for _, group := range spaceGroups {
		found, foundWarnings := scanMarkdownDirectory(
			space,
			group.relative,
			group.owner,
			spaceContext,
			false,
			input.Stage.Slug,
		)
		warnings = append(warnings, foundWarnings...)
		candidates = append(candidates, found...)
	}
	return candidates, warnings
}

func scanMarkdownDirectory(
	source Source,
	relativeDirectory string,
	owner string,
	sourceKind contextSource,
	framework bool,
	stage string,
) ([]candidate, []string) {
	candidates := make([]candidate, 0)
	warnings := []string{}
	walkMarkdownDirectory(
		source,
		relativeDirectory,
		owner,
		sourceKind,
		framework,
		stage,
		&candidates,
		&warnings,
	)
	return candidates, warnings
}

func walkMarkdownDirectory(
	source Source,
	relativeDirectory string,
	owner string,
	sourceKind contextSource,
	framework bool,
	stage string,
	candidates *[]candidate,
	warnings *[]string,
) {
	entries, err := fs.ReadDir(source.FS, relativeDirectory)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		*warnings = append(*warnings, unreadableDirectoryWarning(
			joinDisplay(source.DisplayPrefix, relativeDirectory),
			err,
		))
		return
	}
	sort.SliceStable(entries, func(left, right int) bool {
		return compareUTF16(entries[left].Name(), entries[right].Name()) < 0
	})

	for _, entry := range entries {
		name := entry.Name()
		relative := path.Join(relativeDirectory, name)
		if entry.Type()&fs.ModeSymlink == 0 && entry.IsDir() {
			walkMarkdownDirectory(
				source,
				relative,
				owner,
				sourceKind,
				framework,
				stage,
				candidates,
				warnings,
			)
			continue
		}
		if !isMarkdownFile(entry) {
			continue
		}
		ownerRelative := ""
		if framework {
			ownerRelative = strings.TrimPrefix(
				relative,
				path.Join("knowledge", owner)+"/",
			)
		}
		*candidates = append(*candidates, candidate{
			source:        source,
			relative:      relative,
			display:       joinDisplay(source.DisplayPrefix, relative),
			stage:         stage,
			owner:         owner,
			ownerRelative: ownerRelative,
			sourceKind:    sourceKind,
		})
		preflightCandidates((*candidates)[len(*candidates)-1:], warnings)
	}
}

func isMarkdownFile(entry fs.DirEntry) bool {
	if !strings.HasSuffix(entry.Name(), ".md") {
		return false
	}
	typeBits := entry.Type()
	return typeBits&fs.ModeSymlink != 0 || typeBits.IsRegular()
}

func preflightCandidates(candidates []candidate, warnings *[]string) {
	for index := range candidates {
		candidate := &candidates[index]
		data, err := fs.ReadFile(candidate.source.FS, candidate.relative)
		if err != nil {
			if candidate.persona && errors.Is(err, fs.ErrNotExist) {
				*warnings = append(*warnings, missingPersonaWarning(candidate.display))
			} else {
				*warnings = append(*warnings, unreadableWarning(candidate.display, err))
			}
			continue
		}
		if !utf8.Valid(data) {
			*warnings = append(*warnings, invalidUTF8Warning(candidate.display))
			continue
		}
		candidate.readable = true
	}
}

func compareUTF16(left, right string) int {
	leftUnits := utf16.Encode([]rune(left))
	rightUnits := utf16.Encode([]rune(right))
	limit := len(leftUnits)
	if len(rightUnits) < limit {
		limit = len(rightUnits)
	}
	for index := 0; index < limit; index++ {
		if leftUnits[index] < rightUnits[index] {
			return -1
		}
		if leftUnits[index] > rightUnits[index] {
			return 1
		}
	}
	if len(leftUnits) < len(rightUnits) {
		return -1
	}
	if len(leftUnits) > len(rightUnits) {
		return 1
	}
	return 0
}

func unreadableDirectoryWarning(display string, err error) string {
	return fmt.Sprintf("Warning: optional persona/knowledge directory \"%s\" is unreadable (%v). Fix the directory or its permissions; this stage will continue without that context.", display, err)
}
