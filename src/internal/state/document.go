package state

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
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

	content, err := readStateLeaf(recordRoot, info)
	if err != nil {
		return Document{}, fmt.Errorf("read state: read %q: %w", stateFile, err)
	}
	parsed, err := Parse(content)
	if err != nil {
		return Document{}, fmt.Errorf("read state: parse %q: %w", stateFile, err)
	}
	return Document{State: parsed, Content: content}, nil
}

// readStateLeaf reads only the descriptor whose identity was observed by the
// caller's Root.  The initial Lstat alone is not sufficient: another actor
// could replace a regular state leaf with a FIFO or a different file between
// inspection and open.  The platform helper uses O_NONBLOCK where FIFOs are
// possible, and the descriptor/path checks make every such replacement fail
// closed without waiting on the replacement.
func readStateLeaf(recordRoot *os.Root, pathInfo fs.FileInfo) (content []byte, err error) {
	file, err := openStateLeaf(recordRoot, stateFile)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("open state returned nil file: %w", fs.ErrInvalid)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close state: %w", closeErr))
		}
	}()

	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened state: %w", err)
	}
	if !regularStateIdentity(pathInfo, opened) {
		return nil, fmt.Errorf("state leaf changed identity before read: %w", fs.ErrInvalid)
	}

	content, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read opened state: %w", err)
	}

	final, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened state after read: %w", err)
	}
	current, err := recordRoot.Lstat(stateFile)
	if err != nil {
		return nil, fmt.Errorf("inspect state after read: %w", err)
	}
	if !regularStateIdentity(pathInfo, final) || !regularStateIdentity(pathInfo, current) {
		return nil, fmt.Errorf("state leaf changed identity during read: %w", fs.ErrInvalid)
	}
	return content, nil
}

func regularStateIdentity(expected, actual fs.FileInfo) bool {
	return expected != nil && actual != nil && expected.Mode().IsRegular() &&
		actual.Mode().IsRegular() && os.SameFile(expected, actual)
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

// ActiveAgent returns the unique canonical Active Agent field in Project
// Information. The field is optional to Parse for compatibility with older
// state documents; callers that route a transition must handle a missing
// field explicitly through this accessor.
func ActiveAgent(content []byte) (string, error) {
	if _, err := Parse(content); err != nil {
		return "", fmt.Errorf("read active agent: %w", err)
	}
	lines, err := canonicalSectionLines(content, "Project Information")
	if err != nil {
		return "", fmt.Errorf("read active agent: %w", err)
	}
	value, err := requiredStringField(lines, "Active Agent")
	if err != nil {
		return "", fmt.Errorf("read active agent: %w", err)
	}
	return value, nil
}

// LastCompletedStage returns the unique canonical Last Completed Stage field
// in Session Resume Point. The stage slug is checked independently of the
// parser so routing never treats arbitrary text as a completed stage.
func LastCompletedStage(content []byte) (string, error) {
	if _, err := Parse(content); err != nil {
		return "", fmt.Errorf("read last completed stage: %w", err)
	}
	lines, err := canonicalSectionLines(content, "Session Resume Point")
	if err != nil {
		return "", fmt.Errorf("read last completed stage: %w", err)
	}
	value, err := requiredStringField(lines, "Last Completed Stage")
	if err != nil {
		return "", fmt.Errorf("read last completed stage: %w", err)
	}
	if value != "none" && !validCanonicalStage(value) {
		return "", invalidState("invalid Last Completed Stage %q", value)
	}
	return value, nil
}

// NextAction returns the unique canonical Next Action field in Session
// Resume Point. Unlike a slug/scalar field, this value permits internal
// spaces, but remains a safe nonempty single line.
func NextAction(content []byte) (string, error) {
	if _, err := Parse(content); err != nil {
		return "", fmt.Errorf("read next action: %w", err)
	}
	lines, err := canonicalSectionLines(content, "Session Resume Point")
	if err != nil {
		return "", fmt.Errorf("read next action: %w", err)
	}
	value, err := requiredStringField(lines, "Next Action")
	if err != nil {
		return "", fmt.Errorf("read next action: %w", err)
	}
	if !validCanonicalSingleLine(value) {
		return "", invalidState("invalid Next Action %q", value)
	}
	return value, nil
}

// RevisionCount returns the document's canonical revision count.
func (d Document) RevisionCount() (int, error) { return RevisionCount(d.Content) }

// LastUpdated returns the document's canonical last-updated value.
func (d Document) LastUpdated() (string, error) { return LastUpdated(d.Content) }

// ActiveAgent returns the document's canonical active agent.
func (d Document) ActiveAgent() (string, error) { return ActiveAgent(d.Content) }

// LastCompletedStage returns the document's canonical completed stage.
func (d Document) LastCompletedStage() (string, error) {
	return LastCompletedStage(d.Content)
}

// NextAction returns the document's canonical next action.
func (d Document) NextAction() (string, error) { return NextAction(d.Content) }

func validCanonicalStage(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validCanonicalSingleLine(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character == '\r' || character == '\n' || character == '\u2028' || character == '\u2029' || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

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
