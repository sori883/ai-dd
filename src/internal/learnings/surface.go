package learnings

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/sensor"
)

const (
	SelectionTypeLearning = "learning"
	SelectionTypeSensor   = "sensor"
)

var (
	listItemPattern = regexp.MustCompile(`^(?:[-*+]|[0-9]{1,9}[.)])\s+`)
	sensorIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// Candidate is a backend-derived learning candidate. The text and heading are
// pinned in the question identity; callers may select an existing candidate
// but cannot replace its source content.
type Candidate struct {
	ID          string
	Heading     string
	Text        string
	Summary     string
	Context     string
	Scope       string
	Source      string
	Timestamp   string
	ContentHash string
}

// Surface is the stable, visible portion of the intent-capture memory file.
// Open questions are deliberately parked for presentation and are never
// treated as learning candidates.
type Surface struct {
	Stage       string
	Identity    string
	Space       string
	Intent      string
	Candidates  []Candidate
	Parked      []string
	Fingerprint string
}

type SurfaceInput struct {
	Stage    string
	Identity string
	Space    string
	Intent   string
	Content  []byte
}

// Selection is the only ordinary data accepted by the learnings persistence
// backend. Authority values such as generation, digest, timestamps, and
// receipt identifiers are intentionally absent; they are derived from the
// active ledger and current surface.
type Selection struct {
	CandidateID    string
	Type           string
	Scope          string
	Heading        string
	Text           string
	Source         string
	OriginStage    string
	ManifestFields map[string]string
}

func ParseSurface(input SurfaceInput) (Surface, error) {
	if strings.TrimSpace(input.Stage) == "" || strings.TrimSpace(input.Identity) == "" {
		return Surface{}, fmt.Errorf("surface stage and identity are required: %w", ErrInvalid)
	}
	if !utf8.Valid(input.Content) {
		return Surface{}, fmt.Errorf("surface memory is not UTF-8: %w", ErrInvalid)
	}
	surface := Surface{Stage: input.Stage, Identity: input.Identity, Space: input.Space, Intent: input.Intent}
	seen := make(map[string]bool)
	section := ""
	for _, raw := range sensor.VisibleMarkdownLines(string(input.Content)) {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			switch section {
			case "Interpretations", "Deviations", "Tradeoffs", "Open questions":
				if seen[section] {
					return Surface{}, fmt.Errorf("surface repeats heading %q: %w", section, ErrInvalid)
				}
				seen[section] = true
			default:
				// Unknown headings delimit the recognized sections but are not
				// themselves candidate sources.
				section = ""
			}
			continue
		}
		if section == "" {
			continue
		}
		// A visible blockquote is presentation text rather than a memory
		// entry.  Keep this conservative boundary aligned with the fixed
		// parser: ambiguous Markdown is never promoted into a candidate.
		if strings.HasPrefix(line, ">") {
			continue
		}
		match := listItemPattern.FindString(line)
		if match != "" {
			line = strings.TrimSpace(strings.TrimPrefix(line, match))
		}
		text := line
		if text == "" {
			continue
		}
		if section == "Open questions" {
			surface.Parked = append(surface.Parked, text)
			continue
		}
		timestamp, summary, context := parseMemoryEntry(text)
		id := fmt.Sprintf("c%d", len(surface.Candidates)+1)
		digest := sha256.Sum256([]byte(summary))
		surface.Candidates = append(surface.Candidates, Candidate{
			ID: id, Heading: section, Text: summary, Summary: summary, Context: context,
			Scope: "project", Source: section, Timestamp: timestamp, ContentHash: hex.EncodeToString(digest[:]),
		})
	}
	if len(input.Content) != 0 {
		digest := sha256.Sum256(input.Content)
		surface.Fingerprint = "sha256:" + hex.EncodeToString(digest[:])
	}
	return surface, nil
}

func parseMemoryEntry(text string) (timestamp, summary, context string) {
	const separator = " — "
	if index := strings.Index(text, separator); index > 0 {
		candidateTimestamp := strings.TrimSpace(text[:index])
		if _, err := time.Parse(time.RFC3339, candidateTimestamp); err == nil {
			rest := strings.TrimSpace(text[index+len(separator):])
			if semicolon := strings.IndexByte(rest, ';'); semicolon >= 0 {
				return candidateTimestamp, strings.TrimSpace(rest[:semicolon]), strings.TrimSpace(rest[semicolon+1:])
			}
			return candidateTimestamp, rest, ""
		}
	}
	return "", text, ""
}

func ValidateSelections(selections []Selection) error {
	for _, selection := range selections {
		if !validSelectionToken(selection.CandidateID) || selection.CandidateID == "" || strings.Contains(selection.CandidateID, "/") || strings.Contains(selection.CandidateID, "\\") {
			return fmt.Errorf("selection candidate id is unsafe: %w", ErrInvalid)
		}
		switch selection.Type {
		case SelectionTypeLearning:
			if selection.Scope != "project" && selection.Scope != "team" {
				return fmt.Errorf("learning selection scope is invalid: %w", ErrInvalid)
			}
			if strings.TrimSpace(selection.Heading) == "" || strings.TrimSpace(selection.Text) == "" || !validSingleLine(selection.Heading) || !validSingleLine(selection.Text) {
				return fmt.Errorf("learning selection heading/text is required: %w", ErrInvalid)
			}
			if selection.Source != "" && selection.Source != "orchestrator" && selection.Source != "user_addition" {
				return fmt.Errorf("learning selection source is invalid: %w", ErrInvalid)
			}
			if len(selection.ManifestFields) != 0 {
				return fmt.Errorf("learning selection has authority manifest fields: %w", ErrInvalid)
			}
		case SelectionTypeSensor:
			if selection.OriginStage != "intent-capture" || selection.Source != "" && selection.Source != "orchestrator" && selection.Source != "user_addition" {
				return fmt.Errorf("sensor selection identity is invalid: %w", ErrInvalid)
			}
			if err := validateSensorManifest(selection.ManifestFields); err != nil {
				return err
			}
		default:
			return fmt.Errorf("selection type %q is invalid: %w", selection.Type, ErrInvalid)
		}
	}
	return nil
}

func validateSensorManifest(fields map[string]string) error {
	if fields == nil {
		return fmt.Errorf("sensor manifest is required: %w", ErrInvalid)
	}
	allowed := map[string]bool{"id": true, "kind": true, "command": true, "default_severity": true, "description": true, "matches": true, "timeout_seconds": true, "category": true}
	for key, value := range fields {
		if !allowed[key] || strings.TrimSpace(value) == "" || !validSingleLine(value) {
			return fmt.Errorf("sensor manifest field %q is invalid: %w", key, ErrInvalid)
		}
	}
	for _, key := range []string{"id", "kind", "command", "default_severity", "description", "matches"} {
		if strings.TrimSpace(fields[key]) == "" {
			return fmt.Errorf("sensor manifest field %q is missing: %w", key, ErrInvalid)
		}
	}
	if fields["kind"] != "deterministic" {
		return fmt.Errorf("sensor manifest kind %q is not deterministic: %w", fields["kind"], ErrInvalid)
	}
	if fields["default_severity"] != "advisory" && fields["default_severity"] != "blocking" {
		return fmt.Errorf("sensor manifest default_severity %q is invalid: %w", fields["default_severity"], ErrInvalid)
	}
	if timeout := strings.TrimSpace(fields["timeout_seconds"]); timeout != "" {
		for _, char := range timeout {
			if char < '0' || char > '9' {
				return fmt.Errorf("sensor manifest timeout_seconds is invalid: %w", ErrInvalid)
			}
		}
	}
	if !sensorIDPattern.MatchString(fields["id"]) {
		return fmt.Errorf("sensor manifest id is unsafe: %w", ErrInvalid)
	}
	return nil
}

func validSelectionToken(value string) bool {
	if strings.TrimSpace(value) != value || value == "" {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == '/' || char == '\\' {
			return false
		}
	}
	return true
}

func validSingleLine(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
