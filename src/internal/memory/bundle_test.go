package memory_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/memory"
)

func TestBuildBundleFiltersEmptyWhitespaceAndHeadings(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		substantive bool
	}{
		{name: "empty", content: "", substantive: false},
		{name: "whitespace", content: " \t\n", substantive: false},
		{name: "heading", content: "# heading\n## another", substantive: false},
		{name: "body", content: "a substantive rule", substantive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []memory.Source{{Layer: memory.LayerProject, Path: "project.md", Content: tt.content}}
			got := memory.BuildBundle(input)
			if tt.substantive {
				if len(got) != 1 || got[0] != input[0] {
					t.Fatalf("BuildBundle() = %#v, want %#v", got, input)
				}
				return
			}
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}

func TestBuildBundleFiltersPreambleAndASCIISeparators(t *testing.T) {
	preamble := strings.Join([]string{
		"> This team's affirmed practices and corrections. Loaded after `org.md` as",
		"> strict-additive guidance; contradictions with broader policy are rejected.",
		"> Populated by the practices-discovery affirmation gate. Edit at the gate,",
		"> not directly.",
		"> Project-specific specialisation and corrections. Loaded after `org.md` and",
		"> `team.md` as strict-additive guidance; contradictions with broader policy",
		"> are rejected. Populated by practices-discovery and the self-learning loop.",
		">",
		"> Use sparingly: most teams don't need a project layer. Reach for it",
		"> only when this specific project needs stable, durable guidance beyond the",
		"> team practice (for example, package-specific release checks or an additional",
		"> regression suite for a legacy component).",
	}, "\n")

	tests := []struct {
		name        string
		content     string
		substantive bool
	}{
		{name: "exact shipped preamble", content: preamble, substantive: false},
		{name: "exact shipped preamble with CRLF", content: strings.ReplaceAll(preamble, "\n", "\r\n") + "\r\n", substantive: false},
		{name: "exact shipped preamble with BOM and surrounding whitespace", content: "\ufeff" + preamble + "\ufeff", substantive: false},
		{
			name:        "modified preamble",
			content:     strings.Replace(preamble, "> not directly.", "> edit directly.", 1),
			substantive: true,
		},
		{name: "three ASCII hyphens", content: "---", substantive: false},
		{name: "many ASCII hyphens", content: "-----", substantive: false},
		{name: "two ASCII hyphens", content: "--", substantive: true},
		{name: "unicode hyphens", content: "－－－", substantive: true},
		{name: "unicode em dashes", content: "———", substantive: true},
		{name: "hyphens with internal space", content: "- - -", substantive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []memory.Source{{Layer: memory.LayerProject, Path: "project.md", Content: tt.content}}
			got := memory.BuildBundle(input)
			if tt.substantive {
				if len(got) != 1 || got[0] != input[0] {
					t.Fatalf("BuildBundle() = %#v, want %#v", got, input)
				}
				return
			}
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}

func TestBuildBundleUsesECMAScriptTrimAndLineSplitting(t *testing.T) {
	trimmedRunes := []struct {
		name string
		rune rune
	}{
		{name: "tab", rune: '\u0009'},
		{name: "vertical tab", rune: '\u000b'},
		{name: "form feed", rune: '\u000c'},
		{name: "space", rune: '\u0020'},
		{name: "no-break space", rune: '\u00a0'},
		{name: "ogham space mark", rune: '\u1680'},
		{name: "unicode space lower bound", rune: '\u2000'},
		{name: "unicode space middle", rune: '\u2005'},
		{name: "unicode space upper bound", rune: '\u200a'},
		{name: "narrow no-break space", rune: '\u202f'},
		{name: "medium mathematical space", rune: '\u205f'},
		{name: "ideographic space", rune: '\u3000'},
		{name: "byte order mark", rune: '\ufeff'},
		{name: "carriage return", rune: '\r'},
		{name: "line separator", rune: '\u2028'},
		{name: "paragraph separator", rune: '\u2029'},
	}

	for _, tt := range trimmedRunes {
		t.Run(tt.name, func(t *testing.T) {
			content := string(tt.rune) + "---" + string(tt.rune)
			got := memory.BuildBundle([]memory.Source{{Content: content}})
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}

	tests := []struct {
		name        string
		content     string
		substantive bool
	}{
		{name: "next line with LF", content: "---\n---", substantive: false},
		{name: "next line with CRLF", content: "---\r\n---", substantive: false},
		{name: "lone CR is not a line split", content: "---\r---", substantive: true},
		{name: "U+0085 is not trimmed", content: "\u0085---\u0085", substantive: true},
		{name: "U+2028 is not a line split", content: "---\u2028---", substantive: true},
		{name: "U+2029 is not a line split", content: "---\u2029---", substantive: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []memory.Source{{Content: tt.content}}
			got := memory.BuildBundle(input)
			if tt.substantive {
				if len(got) != 1 || got[0] != input[0] {
					t.Fatalf("BuildBundle() = %#v, want %#v", got, input)
				}
				return
			}
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}

func TestBuildBundleRemovesClosedHTMLCommentsGlobally(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		substantive bool
	}{
		{name: "comment only", content: "<!-- hidden -->", substantive: false},
		{name: "multiline comment only", content: "<!--\n# hidden\n-->", substantive: false},
		{name: "multiple comments", content: "<!-- one -->\n<!-- two -->", substantive: false},
		{name: "adjacent comments", content: "<!-- one --><!-- two -->", substantive: false},
		{name: "non greedy comments leave body", content: "<!-- one -->body<!-- two -->", substantive: true},
		{name: "comment removal joins separator", content: "--<!-- hidden -->-", substantive: false},
		{name: "comment removal joins multiline separator", content: "---<!--\ncomment\n-->---", substantive: false},
		{name: "unclosed comment is retained", content: "<!-- unclosed", substantive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []memory.Source{{Layer: memory.LayerProject, Path: "project.md", Content: tt.content}}
			got := memory.BuildBundle(input)
			if tt.substantive {
				if len(got) != 1 || got[0] != input[0] {
					t.Fatalf("BuildBundle() = %#v, want %#v", got, input)
				}
				return
			}
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}

func TestBuildBundleFiltersOnlyMarkdownScaffolding(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		substantive bool
	}{
		{name: "general blockquote", content: "> authored guidance", substantive: true},
		{name: "frontmatter delimiters only", content: "---\n---", substantive: false},
		{name: "frontmatter field", content: "---\nowner: team\n---", substantive: true},
		{name: "changed delimiter text", content: "----\n----", substantive: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []memory.Source{{Content: tt.content}}
			got := memory.BuildBundle(input)
			if tt.substantive {
				if len(got) != 1 || got[0] != input[0] {
					t.Fatalf("BuildBundle() = %#v, want %#v", got, input)
				}
				return
			}
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}

func TestBuildBundlePreservesOrderDuplicatesAndCallerOwnership(t *testing.T) {
	input := []memory.Source{
		{Layer: memory.LayerOrg, Path: "org.md", Content: "org guidance"},
		{Layer: memory.LayerTeam, Path: "team.md", Content: "# team heading"},
		{Layer: memory.LayerProject, Path: "same.md", Content: "project guidance"},
		{Layer: memory.LayerProject, Path: "same.md", Content: "another project guidance"},
		{Layer: memory.LayerPhase, Path: "phases/ideation.md", Content: "phase guidance"},
	}
	want := []memory.Source{
		input[0],
		input[2],
		input[3],
		input[4],
	}
	original := slices.Clone(input)

	got := memory.BuildBundle(input)
	if !slices.Equal(got, want) {
		t.Fatalf("BuildBundle() = %#v, want %#v", got, want)
	}
	if !slices.Equal(input, original) {
		t.Fatalf("BuildBundle() mutated input: got %#v, want %#v", input, original)
	}

	got[0].Content = "caller mutation"
	if input[0].Content != original[0].Content {
		t.Fatalf("BuildBundle() result shares input backing storage: input[0] = %#v, want %#v", input[0], original[0])
	}
	got = append(got, memory.Source{Path: "caller-only", Content: "caller-only"})
	if len(input) != len(original) {
		t.Fatalf("append to result changed input length: got %d, want %d", len(input), len(original))
	}
}

func TestBuildBundleAppliesTheSameFilterToEveryLayerAndRetainsExactSource(t *testing.T) {
	input := []memory.Source{
		{Layer: memory.LayerOrg, Path: "org.md", Content: "\ufeff> org guidance\r\n"},
		{Layer: memory.LayerTeam, Path: "team.md", Content: "\ufeff> team guidance\r\n"},
		{Layer: memory.LayerProject, Path: "project.md", Content: "\ufeff> project guidance\r\n"},
		{Layer: memory.LayerPhase, Path: "phases/ideation.md", Content: "\ufeff> phase guidance\r\n"},
	}

	got := memory.BuildBundle(input)
	if !slices.Equal(got, input) {
		t.Fatalf("BuildBundle() = %#v, want exact input sources %#v", got, input)
	}
}

func TestBuildBundleReturnsNonNilEmptySliceForNilEmptyAndFilteredInput(t *testing.T) {
	tests := []struct {
		name  string
		input []memory.Source
	}{
		{name: "nil input", input: nil},
		{name: "empty input", input: []memory.Source{}},
		{name: "all filtered", input: []memory.Source{{Content: "# heading"}, {Content: "---"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memory.BuildBundle(tt.input)
			if got == nil {
				t.Fatal("BuildBundle() returned nil, want non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("BuildBundle() = %#v, want empty slice", got)
			}
		})
	}
}
