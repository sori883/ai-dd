package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

type decisionKind string

const (
	decisionKindApproval  decisionKind = "approval"
	decisionKindRejection decisionKind = "rejection"
	decisionKindAnswer    decisionKind = "answer"
)

type decisionMarker struct {
	category string
	phrase   string
}

var (
	// ErrInvalidDecision indicates that an approval or rejection did not match
	// the offered decision contract.
	ErrInvalidDecision = errors.New("orchestrator: invalid decision")
	// ErrInvalidFeedback indicates that a rejection omitted useful feedback.
	ErrInvalidFeedback = errors.New("orchestrator: invalid feedback")
	// ErrSelfAttributedDecision indicates explicit assistant/conductor
	// attribution in a decision text. This is a narrow tripwire, not proof of
	// human authorship.
	ErrSelfAttributedDecision = errors.New("orchestrator: self-attributed decision")
)

// validateApprovalChoice validates the exact choices offered at an approval
// gate. Accept as-is is available only after three recorded revisions.
func validateApprovalChoice(choice string, revisionCount int) error {
	choice = trimECMAScriptWhitespace(choice)
	if revisionCount < 0 {
		return fmt.Errorf("approval revision count %d is negative: %w", revisionCount, ErrInvalidDecision)
	}
	if isNonAnswer(choice) {
		return fmt.Errorf("approval choice is empty or cancellation boilerplate: %w", ErrInvalidDecision)
	}
	switch choice {
	case "Approve":
		return nil
	case "Accept as-is":
		if revisionCount >= 3 {
			return nil
		}
		return fmt.Errorf("Accept as-is requires at least three revisions: %w", ErrInvalidDecision)
	default:
		return fmt.Errorf("approval choice %q is not offered: %w", choice, ErrInvalidDecision)
	}
}

// validateApprovalDecision is the state-backed approval validator reserved
// for the later approval transaction. It derives Revision Count from the
// validated document instead of trusting a caller-provided boolean, count, or
// timestamp.
func validateApprovalDecision(content []byte, choice string) (string, error) {
	revisionCount, err := state.RevisionCount(content)
	if err != nil {
		return "", fmt.Errorf("read approval revision count: %w", err)
	}
	choice = trimECMAScriptWhitespace(choice)
	if marker := selfAttributedDecisionMarker(choice, decisionKindApproval); marker != nil {
		return "", fmt.Errorf("approval choice %q is self-attributed (%s): %w", marker.phrase, marker.category, ErrSelfAttributedDecision)
	}
	if err := validateApprovalChoice(choice, revisionCount); err != nil {
		return "", err
	}
	return choice, nil
}

// validateApprovalGateDecision composes the state-backed choice validator
// with the reader-backed receipt boundary used by the later approval
// transaction. The receipt is freshly read from the identity-bound roots while
// the caller holds the matching Guard; no caller record, boolean, or timestamp
// is accepted as authority.
func validateApprovalGateDecision(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root, content []byte, progress state.StageProgress, choice string) (string, error) {
	if progress.CheckboxState != state.CheckboxStateAwaitingApproval || progress.CheckboxMarker != string(state.StageMarkerAwaitingApproval) {
		return "", fmt.Errorf("approval requires an awaiting-approval marker: %w", ErrInvalidDecision)
	}
	if err := validateApprovalReceipt(ctx, identity, guard, projectRoot, recordRoot); err != nil {
		return "", err
	}
	return validateApprovalDecision(content, choice)
}

func validateApprovalReceipt(ctx context.Context, identity recordlock.Identity, guard *recordlock.Guard, projectRoot, recordRoot *os.Root) error {
	records, err := audit.ReadEvents(ctx, identity, guard, projectRoot, recordRoot)
	if err != nil {
		return fmt.Errorf("read approval audit: %w", err)
	}
	if !audit.HumanTurnFresh(records) {
		return fmt.Errorf("approval has no fresh HUMAN_TURN receipt: %w", ErrStaleHumanTurn)
	}
	return nil
}

func validateRejectionDecision(choice, feedback string) (string, string, error) {
	choice = trimECMAScriptWhitespace(choice)
	feedback = trimECMAScriptWhitespace(feedback)
	if marker := selfAttributedDecisionMarker(feedback, decisionKindRejection); marker != nil {
		return "", "", fmt.Errorf("rejection feedback is self-attributed (%s): %w", marker.phrase, ErrSelfAttributedDecision)
	}
	if choice != "Request Changes" || isNonAnswer(choice) {
		return "", "", fmt.Errorf("rejection choice %q is not offered: %w", choice, ErrInvalidDecision)
	}
	if isNonAnswer(feedback) {
		return "", "", fmt.Errorf("rejection feedback is empty or cancellation boilerplate: %w", ErrInvalidFeedback)
	}
	return choice, feedback, nil
}

// isNonAnswer recognizes only complete cancellation/dismissal/timeout
// phrases. Substantive prose containing one of these words remains an answer.
func isNonAnswer(text string) bool {
	normalized := strings.ToLower(trimECMAScriptWhitespace(text))
	if normalized == "" {
		return true
	}
	if len(normalized) > 0 {
		last := normalized[len(normalized)-1]
		if last == '.' || last == '!' || last == '?' {
			normalized = trimECMAScriptWhitespace(normalized[:len(normalized)-1])
		}
	}
	switch normalized {
	case "cancel", "cancelled", "canceled", "cancellation",
		"dismiss", "dismissed", "abort", "aborted",
		"timed out", "timed-out", "timedout", "timeout",
		"no answer", "no response",
		"user cancelled", "user canceled", "user dismissed",
		"question cancelled", "question canceled", "question dismissed":
		return true
	default:
		return false
	}
}

// trimECMAScriptWhitespace matches the fixed JavaScript trim boundary used by
// the repository's readers. In particular, ECMAScript trims BOM but does not
// classify U+0085 (NEXT LINE) as whitespace.
func trimECMAScriptWhitespace(value string) string {
	return strings.TrimFunc(value, isECMAScriptWhitespace)
}

func isECMAScriptWhitespace(value rune) bool {
	switch value {
	case '\ufeff':
		return true
	case '\u0085':
		return false
	default:
		return unicode.IsSpace(value)
	}
}

// selfAttributedDecisionMarker is deliberately a narrow tripwire. It rejects
// explicit model/conductor provenance while leaving unknown or merely
// descriptive prose to the separate presence and choice checks.
func selfAttributedDecisionMarker(text string, kind decisionKind) *decisionMarker {
	original := text
	candidate := maskQuotedDecisionExamples(original)
	noun := decisionNoun(kind)
	verb := decisionVerb(kind)
	actor := "(agent|assistant|conductor|model|ai)"

	patterns := []struct {
		category string
		pattern  string
	}{
		{
			category: "non-human-decision",
			pattern:  `(?i)\b(not|isn't)\s+((a|the)\s+)?human(['’]s)?\s+` + noun,
		},
		{
			category: "model-authored-decision",
			pattern:  `(?i)(^|\n)\s*([A-Z]\.\s*)?` + actor + `[-\s]+initiated\s+((this|the|an?)\s+)?` + noun,
		},
		{
			category: "model-authored-decision",
			pattern:  `(?i)(^|\n)\s*([A-Z]\.\s*)?` + actor + `[-\s]+(authored|generated|recorded|written)\s+((this|the|an?)\s+)?` + noun,
		},
		{
			category: "model-authored-decision",
			pattern:  `(?i)` + noun + `\s+(was|is)\s+(generated|authored|written|supplied|entered|selected|chosen|made)\s+by\s+((an?|the)\s+)?` + actor,
		},
		{
			category: "model-authored-decision",
			pattern:  `(?i)\bi\s*,?\s+(as\s+)?(the\s+)?` + actor + `\s*,?\s+(am|have)\s+` + verb,
		},
		{
			category: "model-authored-decision",
			pattern:  `(?i)\b` + actor + `\s+(chose|selected)\s+(this\s+)?` + noun,
		},
	}
	if kind == decisionKindRejection {
		patterns = append(patterns, struct {
			category string
			pattern  string
		}{
			category: "model-authored-decision",
			pattern:  `(?i)\b` + actor + `\s+rejected\s+this`,
		})
	}
	patterns = append(patterns, struct {
		category string
		pattern  string
	}{
		category: "conductor-default",
		pattern:  `(?i)\bconductor(['’]s)?[ -]+default`,
	})

	for _, candidatePattern := range patterns {
		pattern := regexp.MustCompile(candidatePattern.pattern)
		for _, match := range pattern.FindAllStringIndex(candidate, -1) {
			if !hasDecisionTail(candidate, match[1]) {
				continue
			}
			return &decisionMarker{
				category: candidatePattern.category,
				phrase:   original[match[0]:match[1]],
			}
		}
	}
	return nil
}

func decisionNoun(kind decisionKind) string {
	switch kind {
	case decisionKindApproval:
		return `(approval|approve|decision|choice|confirmation)`
	case decisionKindRejection:
		return `(rejection|reject|decision|choice|change[ -]request|request changes)`
	default:
		return `(answer|decision|choice|confirmation)`
	}
}

func decisionVerb(kind decisionKind) string {
	switch kind {
	case decisionKindApproval:
		return `(approv(e|ed|ing)|choos(e|en|ing)|select(ed|ing))`
	case decisionKindRejection:
		return `(reject(ed|ing)?|request(ed|ing)? changes|choos(e|en|ing)|select(ed|ing))`
	default:
		return `(answer(ed|ing)?|respond(ed|ing)?|choos(e|en|ing)|select(ed|ing))`
	}
}

func hasDecisionTail(text string, end int) bool {
	if end >= len(text) {
		return true
	}
	rest := text[end:]
	for len(rest) > 0 {
		runeValue, size := utf8.DecodeRuneInString(rest)
		if !unicode.IsSpace(runeValue) {
			break
		}
		rest = rest[size:]
	}
	if rest == "" {
		return true
	}
	runeValue, _ := utf8.DecodeRuneInString(rest)
	if strings.ContainsRune(",.;:!?。！？；：，、)）]", runeValue) || strings.ContainsRune("-–—", runeValue) {
		return true
	}
	word := rest
	if index := strings.IndexFunc(word, unicode.IsSpace); index >= 0 {
		word = word[:index]
	}
	return word == "to" || word == "because" || word == "so" || word == "for"
}

func maskQuotedDecisionExamples(text string) string {
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile("(?s)```.*?```"),
		regexp.MustCompile("(?s)~~~.*?~~~"),
		regexp.MustCompile("(?s)```.*\\z"),
		regexp.MustCompile("(?s)~~~.*\\z"),
		regexp.MustCompile("``[^`\\n]*``"),
		regexp.MustCompile("`[^`\\n]*`"),
		regexp.MustCompile(`"[^"\n]*"`),
		regexp.MustCompile(`“[^”\n]*”`),
		regexp.MustCompile(`‘[^’\n]*’`),
		regexp.MustCompile(`'[^'\n]*'`),
	} {
		text = maskRegexpMatches(text, pattern)
	}
	return maskBlockQuotes(text)
}

func maskRegexpMatches(text string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		masked := []byte(match)
		for index, value := range masked {
			if value != '\n' {
				masked[index] = ' '
			}
		}
		return string(masked)
	})
}

func maskBlockQuotes(text string) string {
	parts := strings.SplitAfter(text, "\n")
	inQuote := false
	for index, part := range parts {
		line := strings.TrimSuffix(part, "\n")
		trimmed := strings.TrimLeft(line, " \t")
		isQuote := len(trimmed) > 0 && trimmed[0] == '>' && len(line)-len(trimmed) <= 3
		if isQuote || (inQuote && strings.TrimSpace(line) != "") {
			parts[index] = maskLine(part)
			inQuote = true
			continue
		}
		inQuote = false
	}
	return strings.Join(parts, "")
}

func maskLine(line string) string {
	masked := []byte(line)
	for index, value := range masked {
		if value != '\n' {
			masked[index] = ' '
		}
	}
	return string(masked)
}
