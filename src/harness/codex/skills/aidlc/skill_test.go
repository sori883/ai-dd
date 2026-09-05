package aidlc

import (
	"os"
	"strings"
	"testing"
)

func TestSkillDefinesReceiverContract(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	text := string(data)
	frontmatter, body, ok := splitSkillFrontmatter(text)
	if !ok {
		t.Fatal("SKILL.md has no complete YAML frontmatter")
	}
	if !hasFrontmatterValue(frontmatter, "name", "aidlc") {
		t.Errorf("frontmatter name is missing or not aidlc")
	}
	if !hasNonemptyFrontmatterValue(frontmatter, "description") {
		t.Errorf("frontmatter description is missing or empty")
	}

	for _, phrase := range []string{
		"aidlc next --project-dir .",
		"aidlc continue \"<opaque token>\" --project-dir .",
		"aidlc read-context --project-dir .",
		"aidlc read-context continue \"<opaque read token>\" --project-dir .",
		"rules_content",
		"continue_token",
		"read_continue_token",
		"complete:true",
		"context_warnings",
		"inline_context_paths",
		"stage_file",
		"consumes",
		"context ready",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not contain %q", phrase)
		}
	}

	for _, forbidden := range []string{
		"emit progress",
		"write artifacts",
		"request approval",
		"execute the stage",
	} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("skill body contains forbidden stage-action wording %q", forbidden)
		}
	}
}

func TestSkillPreservesDirectiveOrderAndFailClosedRules(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := string(data)
	orderedPhrases := []string{
		"load-steering",
		"rules_content",
		"array order",
		"continue_token",
		"run-stage",
		"inline_context_paths",
		"stage_file",
		"consumes",
		"read-context",
		"read_continue_token",
		"complete:true",
	}
	last := -1
	for _, phrase := range orderedPhrases {
		position := strings.Index(body, phrase)
		if position < 0 {
			t.Fatalf("skill body does not contain ordered phrase %q", phrase)
		}
		if position <= last {
			t.Fatalf("skill phrase %q appears out of order", phrase)
		}
		last = position
	}

	for _, phrase := range []string{
		"unknown directive",
		"read failure",
		"fail closed",
		"do not skip",
		"do not run",
		"stop",
	} {
		if !strings.Contains(strings.ToLower(body), phrase) {
			t.Errorf("skill body does not state fail-closed rule %q", phrase)
		}
	}
}

func TestSkillUsesSafeGoReadContextBoundary(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.Join(strings.Fields(strings.ToLower(string(data))), " ")
	for _, phrase := range []string{"opaque read token", "read-context", "complete:true", "stop on any error"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not define read-context boundary phrase %q", phrase)
		}
	}
	for _, forbidden := range []string{
		"shell tool",
		"direct reads",
		"explicitly declared path",
		"resolve relative paths",
		"directory listing",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("skill body retains raw path read boundary %q", forbidden)
		}
	}
}

func TestSkillLimitsReadReceiptException(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.Join(strings.Fields(strings.ToLower(string(data))), " ")
	for _, phrase := range []string{
		"ordinary invocation",
		"machine-readable",
		"read receipt",
		"output schema",
		"only the schema-conforming receipt",
		"verification",
		"in either case",
		"do not run the stage",
		"create outputs",
		"do not send any additional progress message",
		"claim a stage result",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not limit read-receipt exception with %q", phrase)
		}
	}
	if strings.Contains(body, "do not send a progress or result message") {
		t.Error("skill body retains wording that can prohibit the requested context-ready or verification receipt")
	}
}

func TestSkillDefinesVerificationReceiptFieldSemantics(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.Join(strings.Fields(strings.ToLower(string(data))), " ")
	body = strings.ReplaceAll(body, "`", "")
	for _, phrase := range []string{
		"rules contains the last non-empty line of each received rules_content entry, in received order",
		"inline_context contains each inline file's full text after concatenating its chunks",
		"stage_file contains the full text after concatenating its chunks",
		"consumes contains each consume file's full text after concatenating its chunks",
		"slot/index/part order",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not define verification receipt field semantics %q", phrase)
		}
	}
}

func TestSkillDefinesCompactVerificationReceiptFieldSemantics(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.Join(strings.Fields(strings.ToLower(string(data))), " ")
	body = strings.ReplaceAll(body, "`", "")
	for _, phrase := range []string{
		"files contains one compact proof for each delivered file in slot/index order",
		"each files entry has these fields in this order: slot, index, parts, content_sha256, first_non_empty_line, middle_marker_line, last_non_empty_line",
		"parts is the total number of context chunks for that file",
		"content_sha256 is the sha-256 digest of the concatenated chunk text",
		"first_non_empty_line is the first line whose trimmed text is non-empty",
		"middle_marker_line is the first line beginning with middle-",
		"last_non_empty_line is the final line whose trimmed text is non-empty",
		"legacy inline_context, stage_file, and consumes fields retain their full-text meanings",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not define compact verification receipt semantics %q", phrase)
		}
	}
}

func splitSkillFrontmatter(text string) (frontmatter, body string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", false
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", "", false
	}
	return rest[:end], rest[end+len("\n---\n"):], true
}

func hasFrontmatterValue(frontmatter, key, want string) bool {
	prefix := key + ":"
	for _, line := range strings.Split(frontmatter, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, prefix)) == want
	}
	return false
}

func hasNonemptyFrontmatterValue(frontmatter, key string) bool {
	prefix := key + ":"
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)) != ""
		}
	}
	return false
}
