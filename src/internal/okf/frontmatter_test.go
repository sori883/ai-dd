package okf

import (
	"strings"
	"testing"
	"time"
)

func TestParseConceptMetadata(t *testing.T) {
	t.Parallel()
	input := "---\r\ntype: &kind Custom\r\ntitle: *kind\r\ndescription: |\r\n  first line\r\n  second line\r\nresource: /missing.md\r\ntags: [One, '2']\r\nunknown: {anything: [true, 3]}\r\ngenerated: {by: tool/1, at: 2026-01-01T09:00:00+09:00}\r\nverified: {by: 'human:alice', at: 2026-01-02T00:00:00Z}\r\nstatus: draft\r\nstale_after: 2026-02-01T00:00:00Z\r\nsources: [{resource: /broken.md, usage_count: 2, last_modified: 2025-01-01T00:00:00Z}]\r\n---\r\n[broken](/missing.md) BODY_SECRET"
	got, err := ParseConcept(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "Custom" || got.Title != "Custom" || got.Description != "first line\nsecond line\n" || got.Resource != "/missing.md" || got.Status != "draft" || got.TrustTier != "human-reviewed" {
		t.Fatalf("metadata = %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[1] != "2" || got.GeneratedAt == nil || !got.GeneratedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || got.StaleAfter == nil {
		t.Fatalf("typed metadata = %+v", got)
	}
}

func TestParseConceptOptionalAndTrust(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, extra, tier string }{
		{name: "missing", tier: "unverified"},
		{name: "empty list", extra: "verified: []\n", tier: "unverified"},
		{name: "machine", extra: "verified: [{by: 'process:check', at: 2026-01-01T00:00:00Z}]\n", tier: "machine-confirmed"},
		{name: "human list", extra: "verified: [{by: tool/1, at: 2026-01-01T00:00:00Z}, {by: 'human:a', at: 2026-01-01T00:00:00Z}]\n", tier: "human-reviewed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConcept(strings.NewReader("---\ntype: Unknown\n" + tt.extra + "---\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got.Type != "Unknown" || got.Status != "stable" || got.TrustTier != tt.tier || got.Tags == nil {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestParseConceptInvalid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, input string }{
		{name: "no delimiter", input: "type: A"},
		{name: "leading bytes", input: " \n---\ntype: A\n---\n"},
		{name: "unclosed", input: "---\ntype: A\n"},
		{name: "yaml", input: "---\ntype: [\n---\n"},
		{name: "root", input: "---\n[A]\n---\n"},
		{name: "missing type", input: "---\ntitle: A\n---\n"},
		{name: "empty type", input: "---\ntype: ''\n---\n"},
		{name: "numeric type", input: "---\ntype: 42\n---\n"},
		{name: "utf8 metadata", input: "---\ntype: A\ntitle: \xff\n---\n"},
		{name: "utf8 body", input: "---\ntype: A\n---\nbody\xff"},
		{name: "duplicate", input: "---\ntype: A\ntype: B\n---\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseConcept(strings.NewReader(tt.input)); err == nil {
				t.Fatal("accepted invalid concept")
			}
		})
	}
	for _, field := range []string{"title: 1", "description: null", "resource: []", "tags: [true]", "tags: text", "sources: {}", "sources: [{}]", "sources: [{resource: 1}]", "sources: [{resource: x, usage_count: text}]", "sources: [{resource: x, last_modified: 2026-01-01}]", "generated: {}", "generated: {by: 1}", "generated: {by: x, at: 2026-01-01}", "verified: text", "verified: [{}]", "verified: {by: x}", "verified: {by: x, at: false}", "status: other", "stale_after: 2026-01-01"} {
		t.Run(field, func(t *testing.T) {
			if _, err := ParseConcept(strings.NewReader("---\ntype: A\n" + field + "\n---\n")); err == nil {
				t.Fatal("accepted invalid field")
			}
		})
	}
}

func TestParseConceptBoundsAndStreaming(t *testing.T) {
	t.Parallel()
	prefix := "---\ntype: A\n#"
	valid := prefix + strings.Repeat("x", MaxFrontmatterBytes-len(prefix)-len("\n---\n")) + "\n---\n"
	if _, err := ParseConcept(strings.NewReader(valid + strings.Repeat("日本語", 100000))); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseConcept(strings.NewReader(prefix + "x" + strings.TrimPrefix(valid, prefix))); err == nil {
		t.Fatal("accepted oversized frontmatter")
	}
}

func TestParseConceptClosingDelimiterAtEOF(t *testing.T) {
	got, err := ParseConcept(strings.NewReader("---\ntype: A\n---"))
	if err != nil || got.Type != "A" {
		t.Fatalf("eof delimiter: %+v %v", got, err)
	}
}

func TestParseConceptRejectsTrailingYAMLDocument(t *testing.T) {
	if _, err := ParseConcept(strings.NewReader("---\ntype: A\n...\ninvalid: [\n---\n")); err == nil {
		t.Fatal("accepted trailing invalid yaml")
	}
}
