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
		"rules_content",
		"continue_token",
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

func TestSkillDefinesSafeShellBoundary(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.Join(strings.Fields(strings.ToLower(string(data))), " ")
	for _, phrase := range []string{
		"shell tool",
		"commands above",
		"explicitly declared path",
		"read in full",
		"search",
		"glob",
		"directory listing",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("skill body does not define shell boundary phrase %q", phrase)
		}
	}
	for _, forbidden := range []string{
		"never use a shell",
		"exact aidlc command above",
		"search the workspace for replacement context",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("skill body retains contradictory shell boundary %q", forbidden)
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
