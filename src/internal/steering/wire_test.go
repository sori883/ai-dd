package steering_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestBundleDigestPreservesOrderedRules(t *testing.T) {
	tests := []struct {
		name    string
		content []steering.RuleContent
		wire    string
	}{
		{
			name: "ordered ASCII rules with duplicate path and empty text",
			content: []steering.RuleContent{
				{Path: "first.md", Text: "alpha"},
				{Path: "same.md", Text: ""},
				{Path: "same.md", Text: "beta"},
				{Path: "last.md", Text: "omega"},
			},
			wire: `[{"path":"first.md","text":"alpha"},{"path":"same.md","text":""},{"path":"same.md","text":"beta"},{"path":"last.md","text":"omega"}]`,
		},
		{
			name:    "nil content",
			content: nil,
			wire:    `[]`,
		},
		{
			name:    "non-nil empty content",
			content: []steering.RuleContent{},
			wire:    `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := sha256.Sum256([]byte(tt.wire))
			want := "sha256:" + hex.EncodeToString(sum[:])

			got, err := steering.BundleDigest(tt.content)
			if err != nil {
				t.Fatalf("BundleDigest() error = %v", err)
			}
			if got != want {
				t.Fatalf("BundleDigest() = %q, want %q", got, want)
			}
		})
	}
}

func TestBundleDigestMatchesJSONStringifyEscaping(t *testing.T) {
	path := "p\"q\\r<>&\u2028\u2029日🚀\\u003c"
	text := "t\x00\x0b\b\f\n\r\t\"\\<>&\u2028\u2029日本語🚀\\u003c"
	wire := `[{"path":"p\"q\\r<>&` +
		"\u2028\u2029" +
		`日🚀\\u003c","text":"t\u0000\u000b\b\f\n\r\t\"\\<>&` +
		"\u2028\u2029" +
		`日本語🚀\\u003c"}]`

	sum := sha256.Sum256([]byte(wire))
	want := "sha256:" + hex.EncodeToString(sum[:])
	got, err := steering.BundleDigest([]steering.RuleContent{{Path: path, Text: text}})
	if err != nil {
		t.Fatalf("BundleDigest() error = %v", err)
	}
	if got != want {
		t.Fatalf("BundleDigest() = %q, want %q", got, want)
	}
}

func TestBundleDigestRejectsInvalidUTF8(t *testing.T) {
	tests := []struct {
		name    string
		content []steering.RuleContent
	}{
		{
			name: "invalid path",
			content: []steering.RuleContent{{
				Path: string([]byte{0xff}),
				Text: "valid text",
			}},
		},
		{
			name: "truncated text",
			content: []steering.RuleContent{{
				Path: "valid.md",
				Text: string([]byte{0xe3, 0x81}),
			}},
		},
		{
			name: "overlong text",
			content: []steering.RuleContent{{
				Path: "valid.md",
				Text: string([]byte{0xc0, 0xaf}),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := steering.BundleDigest(tt.content)
			if err == nil {
				t.Errorf("BundleDigest() error = nil, want invalid UTF-8 error")
			}
			if got != "" {
				t.Errorf("BundleDigest() = %q, want empty digest", got)
			}
		})
	}
}
