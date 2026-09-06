package agents

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestReviewerAgentConfig(t *testing.T) {
	data, err := os.ReadFile("aidlc-product-lead-agent.toml")
	if err != nil {
		t.Fatalf("ReadFile(aidlc-product-lead-agent.toml): %v", err)
	}
	values, err := parseAgentTOML(string(data))
	if err != nil {
		t.Fatalf("parse aidlc-product-lead-agent.toml: %v", err)
	}
	if values["name"] != "aidlc-product-lead-agent" || values["model_reasoning_effort"] != "medium" {
		t.Fatalf("agent identity = %#v, want product-lead/medium", values)
	}
	instructions := values["developer_instructions"]
	for _, phrase := range []string{
		"**Reviewer:** aidlc-product-lead-agent",
		"one effective iteration",
		"NOT-READY is surfaced",
		"exact `Looks correct`",
		"does not mint workflow authority",
	} {
		if !strings.Contains(instructions, phrase) {
			t.Errorf("developer_instructions lacks %q", phrase)
		}
	}
	if strings.Contains(strings.ToLower(instructions), "architect subagent") || strings.Contains(strings.ToLower(instructions), "contribution file") {
		t.Fatal("product-lead config authorizes an architect subagent or contribution file")
	}
}

// parseAgentTOML is the small, strict subset needed for the shipped Codex
// agent source: quoted scalar keys and one basic triple-quoted instruction
// value. It intentionally rejects arrays/tables so a JSON-shaped replacement
// cannot silently become a valid receiver configuration.
func parseAgentTOML(body string) (map[string]string, error) {
	values := make(map[string]string)
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("line %d is not key/value", index+1)
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if strings.HasPrefix(raw, "\"\"\"") {
			value := strings.TrimPrefix(raw, "\"\"\"")
			if value != "" {
				value = "\n" + value
			}
			closed := strings.HasSuffix(value, "\"\"\"")
			if closed {
				value = strings.TrimSuffix(value, "\"\"\"")
			} else {
				var builder strings.Builder
				builder.WriteString(value)
				for index++; index < len(lines); index++ {
					part := lines[index]
					if strings.HasSuffix(part, "\"\"\"") {
						builder.WriteByte('\n')
						builder.WriteString(strings.TrimSuffix(part, "\"\"\""))
						closed = true
						break
					}
					builder.WriteByte('\n')
					builder.WriteString(part)
				}
				value = builder.String()
			}
			if !closed {
				return nil, fmt.Errorf("key %q has unterminated multiline value", key)
			}
			values[key] = value
			continue
		}
		parsed, err := strconv.Unquote(raw)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		values[key] = parsed
	}
	return values, nil
}
