package state

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Document is one validated state snapshot together with the original file
// bytes used to produce it. Content belongs to the caller after ReadDocument
// returns successfully; Parse does not retain or share its backing array with
// State.
type Document struct {
	State   State
	Content []byte
}

// ReadDocument reads and parses the fixed aidlc-state.md leaf beneath
// recordRoot. The caller owns recordRoot and the returned Content; this
// function never closes or mutates either one. The typed State is independent
// of the returned Content backing array.
func ReadDocument(recordRoot *os.Root) (Document, error) {
	if recordRoot == nil {
		return Document{}, fmt.Errorf("read state: record root is nil: %w", fs.ErrInvalid)
	}

	info, err := recordRoot.Lstat(stateFile)
	if err != nil {
		return Document{}, fmt.Errorf("read state: inspect %q: %w", stateFile, err)
	}
	if info == nil || !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("read state: %q is not a regular file: %w", stateFile, fs.ErrInvalid)
	}

	content, err := recordRoot.ReadFile(stateFile)
	if err != nil {
		return Document{}, fmt.Errorf("read state: read %q: %w", stateFile, err)
	}
	parsed, err := Parse(content)
	if err != nil {
		return Document{}, fmt.Errorf("read state: parse %q: %w", stateFile, err)
	}
	return Document{State: parsed, Content: content}, nil
}

// RevisionCount returns the unique canonical Revision Count in Runtime State.
// Runtime State remains optional to Parse; callers that need a revision must
// request this accessor and handle a missing or malformed field explicitly.
func RevisionCount(content []byte) (int, error) {
	if _, err := Parse(content); err != nil {
		return 0, fmt.Errorf("read revision count: %w", err)
	}
	lines, err := canonicalSectionLines(content, "Runtime State")
	if err != nil {
		return 0, fmt.Errorf("read revision count: %w", err)
	}
	value, err := requiredStringField(lines, "Revision Count")
	if err != nil {
		return 0, fmt.Errorf("read revision count: %w", err)
	}
	parsed, err := parseCanonicalNonNegativeInt(value, "Revision Count")
	if err != nil {
		return 0, fmt.Errorf("read revision count: %w", err)
	}
	return parsed, nil
}

// LastUpdated returns the unique canonical Last Updated field in Current
// Status. It does not impose a timestamp format beyond the existing
// nonempty-field parser contract.
func LastUpdated(content []byte) (string, error) {
	if _, err := Parse(content); err != nil {
		return "", fmt.Errorf("read last updated: %w", err)
	}
	lines, err := canonicalSectionLines(content, "Current Status")
	if err != nil {
		return "", fmt.Errorf("read last updated: %w", err)
	}
	value, err := requiredStringField(lines, "Last Updated")
	if err != nil {
		return "", fmt.Errorf("read last updated: %w", err)
	}
	return value, nil
}

// RevisionCount returns the document's canonical revision count.
func (d Document) RevisionCount() (int, error) { return RevisionCount(d.Content) }

// LastUpdated returns the document's canonical last-updated value.
func (d Document) LastUpdated() (string, error) { return LastUpdated(d.Content) }

func canonicalSectionLines(content []byte, target string) ([]string, error) {
	if len(content) >= 3 && content[0] == 0xef && content[1] == 0xbb && content[2] == 0xbf {
		content = content[3:]
	}

	lines := strings.Split(string(content), "\n")
	current := ""
	found := false
	selected := []string(nil)
	for _, line := range lines[1:] {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimPrefix(line, "## ")
			if current == target {
				if found {
					return nil, invalidState("duplicate section %q", target)
				}
				found = true
				selected = make([]string, 0)
			}
			continue
		}
		if current == target {
			selected = append(selected, line)
		}
	}
	if !found {
		return nil, invalidState("missing section %q", target)
	}
	return selected, nil
}
