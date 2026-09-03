package audit

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidAudit reports an audit shard that cannot be used as an authority
// record.  It also wraps fs.ErrInvalid so callers can fail closed without
// depending on this package-specific sentinel.
var ErrInvalidAudit = errors.New("audit: invalid ledger")

// AuditRecord is the read-side representation of one audit block.  Shard and
// Position are retained because timestamps alone do not establish order when
// two shards contain events in the same UTC second.
type AuditRecord struct {
	Event     string
	Timestamp time.Time
	Fields    map[string]string
	Shard     string
	Position  int
}

var resolutionEvents = map[string]struct{}{
	"GATE_APPROVED":                 {},
	"GATE_REJECTED":                 {},
	"QUESTION_ANSWERED":             {},
	"SUMMARY_CONFIRMATION_RECORDED": {},
	"PLAN_APPROVAL_RECORDED":        {},
}

// HumanTurnFresh reports whether the most recent human turn follows the
// workflow-global resolution boundary. Audit records are intentionally not
// merged by shard filename: positions prove order only within one shard.
func HumanTurnFresh(records []AuditRecord) bool {
	if len(records) == 0 {
		return false
	}

	var humans []AuditRecord
	var resolutions []AuditRecord
	for _, record := range records {
		if !validAuditRecord(record) {
			return false
		}
		if record.Event == "HUMAN_TURN" {
			humans = append(humans, record)
		}
		if isResolution(record) {
			resolutions = append(resolutions, record)
		}
	}
	if len(humans) == 0 {
		return false
	}
	if len(resolutions) == 0 {
		return true
	}

	latestHumanTimestamp := latestRecordTimestamp(humans)
	latestResolutionTimestamp := latestRecordTimestamp(resolutions)
	if latestHumanTimestamp.After(latestResolutionTimestamp) {
		return true
	}
	if latestHumanTimestamp.Before(latestResolutionTimestamp) {
		return false
	}

	latestHumans := recordsAtTimestamp(humans, latestHumanTimestamp)
	latestResolutions := recordsAtTimestamp(resolutions, latestResolutionTimestamp)
	for _, human := range latestHumans {
		allEarlierInShard := true
		for _, resolution := range latestResolutions {
			if resolution.Shard != human.Shard || resolution.Position >= human.Position {
				allEarlierInShard = false
				break
			}
		}
		if allEarlierInShard {
			return true
		}
	}
	return false
}

func validAuditRecord(record AuditRecord) bool {
	if record.Event == "" || record.Shard == "" || record.Position < 0 || record.Timestamp.IsZero() || record.Timestamp.Nanosecond() != 0 {
		return false
	}
	return formatTimestamp(record.Timestamp) == record.Timestamp.Format(time.RFC3339)
}

func isResolution(record AuditRecord) bool {
	if _, ok := resolutionEvents[record.Event]; ok {
		return true
	}
	return record.Event == "AUTONOMY_MODE_SET" && record.Fields["Mode"] == "autonomous"
}

func latestRecordTimestamp(records []AuditRecord) time.Time {
	latest := time.Time{}
	for _, record := range records {
		if record.Timestamp.After(latest) {
			latest = record.Timestamp
		}
	}
	return latest
}

func recordsAtTimestamp(records []AuditRecord, timestamp time.Time) []AuditRecord {
	matched := make([]AuditRecord, 0, len(records))
	for _, record := range records {
		if record.Timestamp.Equal(timestamp) {
			matched = append(matched, record)
		}
	}
	return matched
}

func invalidAudit(format string, args ...any) error {
	return fmt.Errorf("audit: %w: %w: %s", ErrInvalidAudit, fs.ErrInvalid, fmt.Sprintf(format, args...))
}

// parseAuditShard parses the canonical markdown blocks written by Append.
// The writer's event allowlist is intentionally not consulted here: reading
// must preserve workflow events that a later freshness policy recognizes even
// when the append API does not emit them yet.
func parseAuditShard(shard string, content []byte) ([]AuditRecord, error) {
	if !utf8.Valid(content) {
		return nil, invalidAudit("shard %q is not valid UTF-8", shard)
	}
	content, err := normalizeAuditNewlines(content)
	if err != nil {
		return nil, fmt.Errorf("audit: normalize shard %q: %w", shard, err)
	}

	blocks := strings.Split(string(content), "\n---\n")
	records := make([]AuditRecord, 0, len(blocks))
	var previousTimestamp time.Time
	havePreviousTimestamp := false
	for position, block := range blocks {
		record, present, err := parseAuditBlock(shard, position, block)
		if err != nil {
			return nil, err
		}
		if present {
			if havePreviousTimestamp && record.Timestamp.Before(previousTimestamp) {
				return nil, invalidAudit("shard %q block %d timestamp decreases", shard, position)
			}
			previousTimestamp = record.Timestamp
			havePreviousTimestamp = true
			records = append(records, record)
		}
	}
	return records, nil
}

func normalizeAuditNewlines(content []byte) ([]byte, error) {
	if !bytes.Contains(content, []byte{'\r'}) {
		return content, nil
	}
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if bytes.Contains(normalized, []byte{'\r'}) {
		return nil, invalidAudit("bare carriage return")
	}
	return normalized, nil
}

func parseAuditBlock(shard string, position int, block string) (AuditRecord, bool, error) {
	allFields := make(map[string]string)
	for _, line := range strings.Split(block, "\n") {
		key, value, present, malformed := parseAuditField(line)
		if malformed {
			return AuditRecord{}, false, invalidAudit("shard %q block %d has malformed authority field", shard, position)
		}
		if !present {
			continue
		}
		if !validFieldKey(key) {
			return AuditRecord{}, false, invalidAudit("shard %q block %d has invalid field %q", shard, position, key)
		}
		if _, exists := allFields[key]; exists {
			return AuditRecord{}, false, invalidAudit("shard %q block %d repeats field %q", shard, position, key)
		}
		allFields[key] = value
	}

	event, hasEvent := allFields["Event"]
	timestampText, hasTimestamp := allFields["Timestamp"]
	if !hasEvent {
		if hasTimestamp || hasEventHeading(block) {
			return AuditRecord{}, false, invalidAudit("shard %q block %d has Timestamp without Event", shard, position)
		}
		return AuditRecord{}, false, nil
	}
	if event == "" || !hasTimestamp || timestampText == "" {
		return AuditRecord{}, false, invalidAudit("shard %q block %d has incomplete authority", shard, position)
	}
	timestamp, err := parseAuditTimestamp(timestampText)
	if err != nil {
		return AuditRecord{}, false, invalidAudit("shard %q block %d timestamp: %v", shard, position, err)
	}
	if event == "AUTONOMY_MODE_SET" {
		mode, ok := allFields["Mode"]
		if !ok || (mode != "autonomous" && mode != "gated") {
			return AuditRecord{}, false, invalidAudit("shard %q block %d has invalid autonomy mode", shard, position)
		}
	}

	fields := make(map[string]string, len(allFields)-2)
	for key, value := range allFields {
		if key == "Event" || key == "Timestamp" {
			continue
		}
		fields[key] = value
	}
	return AuditRecord{
		Event:     event,
		Timestamp: timestamp,
		Fields:    fields,
		Shard:     shard,
		Position:  position,
	}, true, nil
}

func hasEventHeading(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			return true
		}
	}
	return false
}

func parseAuditField(line string) (key, value string, present, malformed bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "- ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	}
	if !strings.HasPrefix(line, "**") {
		return "", "", false, false
	}
	end := strings.Index(line[2:], "**:")
	if end < 0 {
		return "", "", false, true
	}
	end += 2
	key = line[2:end]
	value = strings.TrimSpace(line[end+3:])
	return key, value, true, false
}

func parseAuditTimestamp(value string) (time.Time, error) {
	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	if formatTimestamp(timestamp) != value {
		return time.Time{}, fmt.Errorf("timestamp is not canonical UTC seconds")
	}
	return timestamp, nil
}
