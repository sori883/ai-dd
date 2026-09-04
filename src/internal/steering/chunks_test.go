package steering_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestChunkRulesPreservesOrderedContent(t *testing.T) {
	tests := []struct {
		name       string
		content    []steering.RuleContent
		want       []steering.RuleContent
		wantChunks int
	}{
		{
			name: "small ordered content",
			content: []steering.RuleContent{
				{Path: "first.md", Text: "first rule"},
				{Path: "same.md", Text: ""},
				{Path: "same.md", Text: "second rule"},
				{Path: "last.md", Text: "last rule"},
			},
			want: []steering.RuleContent{
				{Path: "first.md", Text: "first rule"},
				{Path: "same.md", Text: ""},
				{Path: "same.md", Text: "second rule"},
				{Path: "last.md", Text: "last rule"},
			},
			wantChunks: 1,
		},
		{
			name:       "nil input",
			wantChunks: 0,
		},
		{
			name:       "empty input",
			content:    []steering.RuleContent{},
			wantChunks: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := steering.ChunkRules(tt.content)
			if len(got) != tt.wantChunks {
				t.Fatalf("ChunkRules() returned %d chunks, want %d", len(got), tt.wantChunks)
			}
			if tt.wantChunks == 0 {
				return
			}
			if !slices.Equal(got[0], tt.want) {
				t.Fatalf("ChunkRules() = %#v, want %#v", got[0], tt.want)
			}
		})
	}
}

func TestChunkRulesSplitsMarkdownSections(t *testing.T) {
	const path = "rules.md"

	tests := []struct {
		name string
		text string
		want []steering.RuleContent
	}{
		{
			name: "LF preamble blank lines and trailing newline absence",
			text: "preamble\n# first\nfirst body\n## second\n\nsecond body\n###### sixth\nsixth body",
			want: []steering.RuleContent{
				{Path: path, Text: "preamble\n"},
				{Path: path, Text: "# first\nfirst body\n"},
				{Path: path, Text: "## second\n\nsecond body\n"},
				{Path: path, Text: "###### sixth\nsixth body"},
			},
		},
		{
			name: "CRLF blank lines and trailing newline",
			text: "\r\npreamble\r\n# first\r\n\r\nbody\r\n## second\r\n",
			want: []steering.RuleContent{
				{Path: path, Text: "\r\npreamble\r\n"},
				{Path: path, Text: "# first\r\n\r\nbody\r\n"},
				{Path: path, Text: "## second\r\n"},
			},
		},
		{
			name: "fenced code uses simple line matching",
			text: "before\n```md\n# inside\n```\n# outside\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "before\n```md\n"},
				{Path: path, Text: "# inside\n```\n"},
				{Path: path, Text: "# outside\nbody"},
			},
		},
		{
			name: "space after hash",
			text: "intro\n# Heading\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "intro\n"},
				{Path: path, Text: "# Heading\nbody"},
			},
		},
		{
			name: "tab after hash",
			text: "intro\n##\tHeading\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "intro\n"},
				{Path: path, Text: "##\tHeading\nbody"},
			},
		},
		{
			name: "FEFF after hash",
			text: "intro\n###\ufeffHeading\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "intro\n"},
				{Path: path, Text: "###\ufeffHeading\nbody"},
			},
		},
		{
			name: "U+2028 after hash",
			text: "intro\n####\u2028Heading\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "intro\n"},
				{Path: path, Text: "####\u2028Heading\nbody"},
			},
		},
		{
			name: "U+3000 after hash",
			text: "intro\n#####\u3000Heading\nbody",
			want: []steering.RuleContent{
				{Path: path, Text: "intro\n"},
				{Path: path, Text: "#####\u3000Heading\nbody"},
			},
		},
		{
			name: "U+0085 is not ECMAScript whitespace",
			text: "intro\n#\u0085Heading\nbody",
			want: []steering.RuleContent{{Path: path, Text: "intro\n#\u0085Heading\nbody"}},
		},
		{
			name: "seven hashes are not a heading",
			text: "intro\n####### Heading\nbody",
			want: []steering.RuleContent{{Path: path, Text: "intro\n####### Heading\nbody"}},
		},
		{
			name: "indented hash is not a heading",
			text: "intro\n # Heading\nbody",
			want: []steering.RuleContent{{Path: path, Text: "intro\n # Heading\nbody"}},
		},
		{
			name: "hash without following whitespace is not a heading",
			text: "intro\n#Heading\nbody",
			want: []steering.RuleContent{{Path: path, Text: "intro\n#Heading\nbody"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := steering.ChunkRules([]steering.RuleContent{{Path: path, Text: tt.text}})
			if len(got) != 1 {
				t.Fatalf("ChunkRules() returned %d chunks, want 1", len(got))
			}
			if !slices.Equal(got[0], tt.want) {
				t.Fatalf("ChunkRules() = %#v, want %#v", got[0], tt.want)
			}
		})
	}
}

func TestChunkRulesPacksAtJSONByteBoundary(t *testing.T) {
	const (
		maxChunkBytes = 20 * 1024
	)

	aText := strings.Repeat("x", 10100)
	bText := strings.Repeat("y", 10333)
	overflowText := bText + "y"
	exactContent := []steering.RuleContent{
		{Path: "a", Text: aText},
		{Path: "b", Text: bText},
	}
	overflowContent := []steering.RuleContent{
		{Path: "a", Text: aText},
		{Path: "b", Text: overflowText},
		{Path: "c", Text: "z"},
	}

	tests := []struct {
		name          string
		content       []steering.RuleContent
		boundaryInput []steering.RuleContent
		boundaryBytes int
		want          [][]steering.RuleContent
	}{
		{
			name:          "exactly 20 KiB",
			content:       exactContent,
			boundaryInput: exactContent,
			boundaryBytes: maxChunkBytes,
			want: [][]steering.RuleContent{
				exactContent,
			},
		},
		{
			name:    "one byte over keeps later content in the next chunk",
			content: overflowContent,
			boundaryInput: []steering.RuleContent{
				{Path: "a", Text: aText},
				{Path: "b", Text: overflowText},
			},
			boundaryBytes: maxChunkBytes + 1,
			want: [][]steering.RuleContent{
				{
					{Path: "a", Text: aText},
				},
				{
					{Path: "b", Text: overflowText},
					{Path: "c", Text: "z"},
				},
			},
		},
	}

	type wireRule struct {
		Path string `json:"path"`
		Text string `json:"text"`
	}
	wireSize := func(t *testing.T, rules []steering.RuleContent) int {
		t.Helper()
		wire := make([]wireRule, len(rules))
		for index, rule := range rules {
			wire[index] = wireRule{Path: rule.Path, Text: rule.Text}
		}
		encoded, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		return len(encoded)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wireSize(t, tt.boundaryInput); got != tt.boundaryBytes {
				t.Fatalf("wire size = %d, want %d", got, tt.boundaryBytes)
			}
			for index, rule := range tt.content {
				if got := wireSize(t, []steering.RuleContent{rule}); got >= maxChunkBytes {
					t.Fatalf("rule %d wire size = %d, want less than %d", index, got, maxChunkBytes)
				}
			}

			got := steering.ChunkRules(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("ChunkRules() returned %d chunks, want %d", len(got), len(tt.want))
			}
			for index := range tt.want {
				if !slices.Equal(got[index], tt.want[index]) {
					t.Fatalf("ChunkRules() chunk %d = %#v, want %#v", index, got[index], tt.want[index])
				}
			}
		})
	}
}

func TestChunkRulesSplitsOversizedSections(t *testing.T) {
	const (
		maxChunkBytes = 20 * 1024
		path          = "rules.md"
	)

	exactASCII := strings.Repeat("a", 20449)
	tests := []struct {
		name      string
		text      string
		minPieces int
	}{
		{
			name:      "ASCII exact boundary",
			text:      exactASCII,
			minPieces: 1,
		},
		{
			name:      "ASCII one byte over boundary",
			text:      exactASCII + "b",
			minPieces: 2,
		},
		{
			name:      "Japanese code points",
			text:      strings.Repeat("日", 7000),
			minPieces: 2,
		},
		{
			name:      "emoji code points",
			text:      strings.Repeat("🚀", 6000),
			minPieces: 2,
		},
		{
			name:      "mixed Japanese and emoji code points",
			text:      strings.Repeat("日🚀", 4000),
			minPieces: 2,
		},
	}

	type wireRule struct {
		Path string `json:"path"`
		Text string `json:"text"`
	}
	wireSize := func(t *testing.T, rules []steering.RuleContent) int {
		t.Helper()
		wire := make([]wireRule, len(rules))
		for index, rule := range rules {
			wire[index] = wireRule{Path: rule.Path, Text: rule.Text}
		}
		encoded, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		return len(encoded)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := steering.ChunkRules([]steering.RuleContent{{Path: path, Text: tt.text}})
			if len(got) == 0 {
				t.Fatal("ChunkRules() returned no chunks")
			}

			pieces := make([]steering.RuleContent, 0, tt.minPieces)
			var reconstructed strings.Builder
			reconstructed.Grow(len(tt.text))
			for chunkIndex, chunk := range got {
				if len(chunk) == 0 {
					t.Fatalf("ChunkRules() chunk %d is empty", chunkIndex)
				}
				if gotBytes := wireSize(t, chunk); gotBytes > maxChunkBytes {
					t.Fatalf("ChunkRules() chunk %d wire size = %d, want <= %d", chunkIndex, gotBytes, maxChunkBytes)
				}
				for _, piece := range chunk {
					if piece.Path != path {
						t.Fatalf("ChunkRules() piece path = %q, want %q", piece.Path, path)
					}
					if piece.Text == "" {
						t.Fatal("ChunkRules() returned an empty text piece")
					}
					if !utf8.ValidString(piece.Text) {
						t.Fatal("ChunkRules() returned an invalid UTF-8 piece")
					}
					if gotBytes := wireSize(t, []steering.RuleContent{piece}); gotBytes > maxChunkBytes {
						t.Fatalf("ChunkRules() piece wire size = %d, want <= %d", gotBytes, maxChunkBytes)
					}
					reconstructed.WriteString(piece.Text)
					pieces = append(pieces, piece)
				}
			}

			if len(pieces) < tt.minPieces {
				t.Fatalf("ChunkRules() returned %d pieces, want at least %d", len(pieces), tt.minPieces)
			}
			if got := reconstructed.String(); got != tt.text {
				t.Fatalf("reconstructed text differs from input: got %d bytes, want %d", len(got), len(tt.text))
			}
			for index := 0; index < len(pieces)-1; index++ {
				nextRune, width := utf8.DecodeRuneInString(pieces[index+1].Text)
				if width == 0 {
					t.Fatalf("piece %d has no next code point", index)
				}
				candidate := steering.RuleContent{Path: path, Text: pieces[index].Text + string(nextRune)}
				if gotBytes := wireSize(t, []steering.RuleContent{candidate}); gotBytes <= maxChunkBytes {
					t.Fatalf("piece %d is not maximal: adding next code point gives %d bytes, want > %d", index, gotBytes, maxChunkBytes)
				}
			}
		})
	}
}

func TestChunkRulesUsesJSONWireSize(t *testing.T) {
	const maxChunkBytes = 20 * 1024

	jsonStringifiedBytes := func(value string) int {
		size := 2
		for _, valueRune := range value {
			switch valueRune {
			case '"', '\\':
				size += 2
			case '\b', '\f', '\n', '\r', '\t':
				size += 2
			default:
				if valueRune <= 0x1f {
					size += 6
				} else {
					size += utf8.RuneLen(valueRune)
				}
			}
		}
		return size
	}
	jsonObjectBytes := func(path, text string) int {
		const (
			prefix    = `{"path":`
			separator = `,"text":`
			suffix    = `}`
		)
		return len(prefix) + jsonStringifiedBytes(path) + len(separator) + jsonStringifiedBytes(text) + len(suffix)
	}
	jsonWireBytes := func(path, text string) int {
		return 2 + jsonObjectBytes(path, text)
	}
	jsonWireBytesFromRules := func(rules []steering.RuleContent) int {
		size := 2
		for index, rule := range rules {
			if index > 0 {
				size++
			}
			size += jsonObjectBytes(rule.Path, rule.Text)
		}
		return size
	}
	assertChunks := func(t *testing.T, path, text string, want [][]steering.RuleContent) {
		t.Helper()
		got := steering.ChunkRules([]steering.RuleContent{{Path: path, Text: text}})
		if len(got) != len(want) {
			t.Fatalf("ChunkRules() returned %d chunks, want %d", len(got), len(want))
		}
		for index := range want {
			if !slices.Equal(got[index], want[index]) {
				t.Fatalf("ChunkRules() chunk %d = %#v, want %#v", index, got[index], want[index])
			}
		}

		var reconstructed strings.Builder
		for chunkIndex, chunk := range got {
			if gotBytes := jsonWireBytesFromRules(chunk); gotBytes > maxChunkBytes {
				t.Fatalf("ChunkRules() chunk %d wire size = %d, want <= %d", chunkIndex, gotBytes, maxChunkBytes)
			}
			for _, piece := range chunk {
				if piece.Path != path {
					t.Fatalf("ChunkRules() piece path = %q, want %q", piece.Path, path)
				}
				reconstructed.WriteString(piece.Text)
			}
		}
		if got := reconstructed.String(); got != text {
			t.Fatalf("reconstructed text = %q, want original text", got)
		}
	}

	pathTests := []struct {
		name        string
		path        string
		encodedPath string
	}{
		{
			name:        "escaped quote path",
			path:        `rules"set.md`,
			encodedPath: `"rules\"set.md"`,
		},
		{
			name:        "escaped backslash path",
			path:        `rules\set.md`,
			encodedPath: `"rules\\set.md"`,
		},
		{
			name:        "Japanese path",
			path:        "規則.md",
			encodedPath: `"規則.md"`,
		},
		{
			name:        "emoji path",
			path:        "🚀.md",
			encodedPath: `"🚀.md"`,
		},
	}
	for _, tt := range pathTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonStringifiedBytes(tt.path); got != len(tt.encodedPath) {
				t.Fatalf("independent path encoding size = %d, want literal %d", got, len(tt.encodedPath))
			}

			baseBytes := jsonWireBytes(tt.path, "")
			exactTextLength := maxChunkBytes - baseBytes
			exactText := strings.Repeat("a", exactTextLength)
			if got := jsonWireBytes(tt.path, exactText); got != maxChunkBytes {
				t.Fatalf("exact text wire size = %d, want %d", got, maxChunkBytes)
			}
			assertChunks(t, tt.path, exactText, [][]steering.RuleContent{
				{{Path: tt.path, Text: exactText}},
			})

			overflowText := exactText + "b"
			if got := jsonWireBytes(tt.path, overflowText); got != maxChunkBytes+1 {
				t.Fatalf("one-byte-over text wire size = %d, want %d", got, maxChunkBytes+1)
			}
			assertChunks(t, tt.path, overflowText, [][]steering.RuleContent{
				{{Path: tt.path, Text: exactText}},
				{{Path: tt.path, Text: "b"}},
			})
		})
	}

	textTests := []struct {
		name      string
		value     string
		wireBytes int
	}{
		{name: "quote", value: `"`, wireBytes: 2},
		{name: "backslash", value: `\`, wireBytes: 2},
		{name: "NUL", value: "\x00", wireBytes: 6},
		{name: "vertical tab", value: "\x0b", wireBytes: 6},
		{name: "newline", value: "\n", wireBytes: 2},
		{name: "tab", value: "\t", wireBytes: 2},
		{name: "backspace", value: "\b", wireBytes: 2},
		{name: "formfeed", value: "\f", wireBytes: 2},
		{name: "carriage return", value: "\r", wireBytes: 2},
		{name: "less-than", value: "<", wireBytes: 1},
		{name: "greater-than", value: ">", wireBytes: 1},
		{name: "ampersand", value: "&", wireBytes: 1},
		{name: "U+2028", value: "\u2028", wireBytes: 3},
		{name: "U+2029", value: "\u2029", wireBytes: 3},
		{name: "Japanese", value: "日", wireBytes: 3},
		{name: "emoji", value: "🚀", wireBytes: 4},
	}
	const path = "rules.md"
	for _, tt := range textTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonStringifiedBytes(tt.value) - 2; got != tt.wireBytes {
				t.Fatalf("independent JSON byte cost = %d, want %d", got, tt.wireBytes)
			}

			baseBytes := jsonWireBytes(path, "")
			maxRunes := (maxChunkBytes - baseBytes) / tt.wireBytes
			text := strings.Repeat(tt.value, maxRunes+1)
			prefix := strings.Repeat(tt.value, maxRunes)
			if got := jsonWireBytes(path, prefix); got > maxChunkBytes {
				t.Fatalf("maximal prefix wire size = %d, want <= %d", got, maxChunkBytes)
			}
			if got := jsonWireBytes(path, text); got <= maxChunkBytes {
				t.Fatalf("oversized text wire size = %d, want > %d", got, maxChunkBytes)
			}
			assertChunks(t, path, text, [][]steering.RuleContent{
				{{Path: path, Text: prefix}},
				{{Path: path, Text: tt.value}},
			})
		})
	}
}

func TestChunkRulesPreservesOversizedCodePoint(t *testing.T) {
	const (
		maxChunkBytes = 20 * 1024
		path          = "rules.md"
	)

	oversizedPath := strings.Repeat("p", 20460)
	text := "日🚀x"
	got := steering.ChunkRules([]steering.RuleContent{{Path: oversizedPath, Text: text}})
	want := [][]steering.RuleContent{
		{{Path: oversizedPath, Text: "日"}},
		{{Path: oversizedPath, Text: "🚀"}},
		{{Path: oversizedPath, Text: "x"}},
	}
	if !slices.EqualFunc(got, want, func(gotChunk, wantChunk []steering.RuleContent) bool {
		return slices.Equal(gotChunk, wantChunk)
	}) {
		t.Fatalf("ChunkRules() = %#v, want %#v", got, want)
	}

	var reconstructed strings.Builder
	for chunkIndex, chunk := range got {
		if len(chunk) != 1 {
			t.Fatalf("ChunkRules() chunk %d contains %d pieces, want 1", chunkIndex, len(chunk))
		}
		piece := chunk[0]
		if !utf8.ValidString(piece.Text) || utf8.RuneCountInString(piece.Text) != 1 {
			t.Fatalf("ChunkRules() chunk %d text = %q, want one valid code point", chunkIndex, piece.Text)
		}
		wireBytes := len(`[{"path":"`) + len(piece.Path) + len(`","text":"`) + len(piece.Text) + len(`"}]`)
		if wireBytes <= maxChunkBytes {
			t.Fatalf("ChunkRules() chunk %d wire size = %d, want > %d", chunkIndex, wireBytes, maxChunkBytes)
		}
		reconstructed.WriteString(piece.Text)
	}
	if got := reconstructed.String(); got != text {
		t.Fatalf("reconstructed text = %q, want %q", got, text)
	}
}

func TestChunkRulesDropsEmptyTextWhenPathExceedsTarget(t *testing.T) {
	const maxChunkBytes = 20 * 1024

	jsonWireBytes := func(path, text string) int {
		t.Helper()
		type wireRule struct {
			Path string `json:"path"`
			Text string `json:"text"`
		}
		encoded, err := json.Marshal([]wireRule{{Path: path, Text: text}})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		return len(encoded)
	}

	path := strings.Repeat("p", 20460)
	if got := jsonWireBytes(path, ""); got <= maxChunkBytes {
		t.Fatalf("single empty-text rule wire size = %d, want > %d", got, maxChunkBytes)
	}

	got := steering.ChunkRules([]steering.RuleContent{{Path: path, Text: ""}})
	if len(got) != 0 {
		t.Fatalf("ChunkRules() returned %d chunks, want 0", len(got))
	}
}

func TestChunkRulesOwnsReturnedSlices(t *testing.T) {
	input := []steering.RuleContent{
		{Path: "large.md", Text: strings.Repeat("x", 48*1024)},
		{Path: "small.md", Text: "small"},
	}

	cloneChunks := func(chunks [][]steering.RuleContent) [][]steering.RuleContent {
		cloned := make([][]steering.RuleContent, len(chunks))
		for index, chunk := range chunks {
			cloned[index] = slices.Clone(chunk)
		}
		return cloned
	}
	equalChunks := func(left, right [][]steering.RuleContent) bool {
		if len(left) != len(right) {
			return false
		}
		for index := range left {
			if !slices.Equal(left[index], right[index]) {
				return false
			}
		}
		return true
	}

	first := steering.ChunkRules(input)
	second := steering.ChunkRules(input)
	if len(first) < 2 || len(second) != len(first) {
		t.Fatalf("ChunkRules() returned %d and %d chunks, want at least 2 matching chunks", len(first), len(second))
	}
	firstBefore := cloneChunks(first)
	secondBefore := cloneChunks(second)

	input[0] = steering.RuleContent{Path: "mutated-input.md", Text: "mutated input"}
	input[1] = steering.RuleContent{Path: "mutated-input-2.md", Text: "mutated input 2"}
	inputAfterMutation := slices.Clone(input)
	if !equalChunks(first, firstBefore) || !equalChunks(second, secondBefore) {
		t.Fatalf("changing input elements changed an already returned result")
	}

	first[0] = []steering.RuleContent{{Path: "mutated-outer.md", Text: "mutated outer"}}
	if !slices.Equal(second[0], secondBefore[0]) {
		t.Fatalf("mutating returned outer slice changed the other result")
	}
	if !slices.Equal(input, inputAfterMutation) {
		t.Fatalf("mutating returned outer slice changed the input")
	}

	innerIndex := -1
	for index := 1; index < len(first) && index < len(second); index++ {
		if len(first[index]) > 0 && len(second[index]) > 0 {
			innerIndex = index
			break
		}
	}
	if innerIndex < 0 {
		t.Fatal("ChunkRules() returned no non-empty inner slice to mutate")
	}

	first[innerIndex][0] = steering.RuleContent{Path: "mutated-element.md", Text: "mutated element"}
	if !slices.Equal(second[innerIndex], secondBefore[innerIndex]) {
		t.Fatalf("mutating a returned element changed the other result")
	}
	if !slices.Equal(input, inputAfterMutation) {
		t.Fatalf("mutating a returned element changed the input")
	}

	firstAppend := steering.RuleContent{Path: "first-append.md", Text: "first append"}
	secondAppend := steering.RuleContent{Path: "second-append.md", Text: "second append"}
	firstLength := len(first[innerIndex])
	first[innerIndex] = append(first[innerIndex], firstAppend)
	second[innerIndex] = append(second[innerIndex], secondAppend)
	if got := first[innerIndex][firstLength]; got != firstAppend {
		t.Fatalf("appending to the other result changed this result: got %#v, want %#v", got, firstAppend)
	}
}
