package steering_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestMarshalLoadProducesExactWireJSON(t *testing.T) {
	tests := []struct {
		name  string
		input steering.LoadDirective
		want  string
	}{
		{
			name: "metadata and ordered rules",
			input: steering.LoadDirective{
				Stage:  "implementation",
				Bundle: "sha256:abc123",
				Part:   1,
				Parts:  2,
				RulesContent: []steering.RuleContent{
					{Path: "first.md", Text: "alpha"},
					{Path: "same.md", Text: ""},
					{Path: "same.md", Text: "beta"},
				},
				ContinueToken: "opaque-token-123",
			},
			want: `{"kind":"load-steering","stage":"implementation","bundle":"sha256:abc123","part":1,"parts":2,"rules_content":[{"path":"first.md","text":"alpha"},{"path":"same.md","text":""},{"path":"same.md","text":"beta"}],"continue_token":"opaque-token-123"}`,
		},
		{
			name: "empty metadata and nil rules",
			input: steering.LoadDirective{
				Part:  1,
				Parts: 1,
			},
			want: `{"kind":"load-steering","stage":"","bundle":"","part":1,"parts":1,"rules_content":[],"continue_token":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := steering.MarshalLoad(tt.input)
			if err != nil {
				t.Fatalf("MarshalLoad() error = %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("MarshalLoad() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarshalLoadRejectsInvalidInput(t *testing.T) {
	validInput := func() steering.LoadDirective {
		return steering.LoadDirective{
			Stage:         "stage",
			Bundle:        "sha256:bundle",
			Part:          1,
			Parts:         1,
			RulesContent:  []steering.RuleContent{{Path: "rules.md", Text: "valid"}},
			ContinueToken: "token",
		}
	}

	tests := []struct {
		name  string
		input steering.LoadDirective
	}{
		{
			name: "part zero",
			input: func() steering.LoadDirective {
				input := validInput()
				input.Part = 0
				return input
			}(),
		},
		{
			name: "parts zero",
			input: func() steering.LoadDirective {
				input := validInput()
				input.Parts = 0
				return input
			}(),
		},
		{
			name: "part greater than parts",
			input: func() steering.LoadDirective {
				input := validInput()
				input.Part = 2
				return input
			}(),
		},
		{
			name: "invalid stage",
			input: func() steering.LoadDirective {
				input := validInput()
				input.Stage = string([]byte{0xff})
				return input
			}(),
		},
		{
			name: "invalid bundle",
			input: func() steering.LoadDirective {
				input := validInput()
				input.Bundle = string([]byte{0xc0, 0xaf})
				return input
			}(),
		},
		{
			name: "invalid continue token",
			input: func() steering.LoadDirective {
				input := validInput()
				input.ContinueToken = string([]byte{0xe3, 0x81})
				return input
			}(),
		},
		{
			name: "invalid rule path",
			input: func() steering.LoadDirective {
				input := validInput()
				input.RulesContent[0].Path = string([]byte{0xff})
				return input
			}(),
		},
		{
			name: "invalid rule text",
			input: func() steering.LoadDirective {
				input := validInput()
				input.RulesContent[0].Text = string([]byte{0xe3, 0x81})
				return input
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := steering.MarshalLoad(test.input)
			if err == nil {
				t.Errorf("MarshalLoad() error = nil, want invalid input error")
			}
			if got != nil {
				t.Errorf("MarshalLoad() = %#v, want nil result", got)
			}
		})
	}
}

func TestMarshalLoadEnforcesDirectiveByteLimit(t *testing.T) {
	const maxDirectiveBytes = 28672
	const wirePrefix = `{"kind":"load-steering","stage":"stage","bundle":"sha256:bundle","part":1,"parts":1,"rules_content":[{"path":"rules.md","text":"`
	const wireSuffix = `"}],"continue_token":"token"}`

	textSize := maxDirectiveBytes - len(wirePrefix) - len(wireSuffix)
	input := steering.LoadDirective{
		Stage:         "stage",
		Bundle:        "sha256:bundle",
		Part:          1,
		Parts:         1,
		RulesContent:  []steering.RuleContent{{Path: "rules.md", Text: strings.Repeat("x", textSize)}},
		ContinueToken: "token",
	}

	got, err := steering.MarshalLoad(input)
	if err != nil {
		t.Fatalf("MarshalLoad() error = %v", err)
	}
	if len(got) != maxDirectiveBytes {
		t.Fatalf("len(MarshalLoad()) = %d, want %d", len(got), maxDirectiveBytes)
	}

	overLimit := input
	overLimit.RulesContent = []steering.RuleContent{{Path: "rules.md", Text: strings.Repeat("x", textSize+1)}}
	got, err = steering.MarshalLoad(overLimit)
	if err == nil {
		t.Errorf("MarshalLoad() error = nil, want directive size error")
	}
	if got != nil {
		t.Errorf("MarshalLoad() returned non-nil result, want nil")
	}
}

func TestMarshalLoadComposesRuleChunks(t *testing.T) {
	type decodedRule struct {
		Path string `json:"path"`
		Text string `json:"text"`
	}
	type decodedLoad struct {
		Kind          string        `json:"kind"`
		Stage         string        `json:"stage"`
		Bundle        string        `json:"bundle"`
		Part          int           `json:"part"`
		Parts         int           `json:"parts"`
		RulesContent  []decodedRule `json:"rules_content"`
		ContinueToken string        `json:"continue_token"`
	}

	content := []steering.RuleContent{{
		Path: "rules.md",
		Text: "# 見出し\n" + strings.Repeat("本文の内容です。\n", 5000),
	}}
	bundle, err := steering.BundleDigest(content)
	if err != nil {
		t.Fatalf("BundleDigest() error = %v", err)
	}

	chunks := steering.ChunkRules(content)
	if len(chunks) < 2 {
		t.Fatalf("ChunkRules() returned %d chunks, want multiple chunks", len(chunks))
	}

	decodedPieces := make([]steering.RuleContent, 0)
	for index, chunk := range chunks {
		token := "opaque-token-" + strconv.Itoa(index+1)
		input := steering.LoadDirective{
			Stage:         "implementation",
			Bundle:        bundle,
			Part:          index + 1,
			Parts:         len(chunks),
			RulesContent:  chunk,
			ContinueToken: token,
		}

		wire, err := steering.MarshalLoad(input)
		if err != nil {
			t.Fatalf("MarshalLoad() part %d error = %v", index+1, err)
		}
		if len(wire) > 28672 {
			t.Fatalf("MarshalLoad() part %d length = %d, want <= 28672", index+1, len(wire))
		}

		var decoded decodedLoad
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() part %d error = %v", index+1, err)
		}
		if decoded.Kind != "load-steering" {
			t.Errorf("part %d kind = %q, want %q", index+1, decoded.Kind, "load-steering")
		}
		if decoded.Stage != input.Stage {
			t.Errorf("part %d stage = %q, want %q", index+1, decoded.Stage, input.Stage)
		}
		if decoded.Bundle != input.Bundle {
			t.Errorf("part %d bundle = %q, want %q", index+1, decoded.Bundle, input.Bundle)
		}
		if decoded.Part != input.Part {
			t.Errorf("part %d part = %d, want %d", index+1, decoded.Part, input.Part)
		}
		if decoded.Parts != input.Parts {
			t.Errorf("part %d parts = %d, want %d", index+1, decoded.Parts, input.Parts)
		}
		if decoded.ContinueToken != input.ContinueToken {
			t.Errorf("part %d continue_token = %q, want %q", index+1, decoded.ContinueToken, input.ContinueToken)
		}

		decodedChunk := make([]steering.RuleContent, len(decoded.RulesContent))
		for ruleIndex, rule := range decoded.RulesContent {
			decodedChunk[ruleIndex] = steering.RuleContent{Path: rule.Path, Text: rule.Text}
		}
		if !reflect.DeepEqual(decodedChunk, chunk) {
			t.Errorf("part %d rules_content = %#v, want %#v", index+1, decodedChunk, chunk)
		}
		decodedPieces = append(decodedPieces, decodedChunk...)
	}

	wantPieces := make([]steering.RuleContent, 0)
	for _, chunk := range chunks {
		wantPieces = append(wantPieces, chunk...)
	}
	if !reflect.DeepEqual(decodedPieces, wantPieces) {
		t.Errorf("flattened decoded rules = %#v, want %#v", decodedPieces, wantPieces)
	}
}

func TestMarshalLoadOwnsWireBytes(t *testing.T) {
	input := steering.LoadDirective{
		Stage:  "implementation",
		Bundle: "sha256:bundle",
		Part:   1,
		Parts:  1,
		RulesContent: []steering.RuleContent{
			{Path: "first.md", Text: "alpha"},
			{Path: "second.md", Text: "beta"},
		},
		ContinueToken: "opaque-token",
	}
	wantInputRules := append([]steering.RuleContent(nil), input.RulesContent...)

	first, err := steering.MarshalLoad(input)
	if err != nil {
		t.Fatalf("first MarshalLoad() error = %v", err)
	}
	second, err := steering.MarshalLoad(input)
	if err != nil {
		t.Fatalf("second MarshalLoad() error = %v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatalf("MarshalLoad() returned empty wire bytes")
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("two MarshalLoad() results differ")
	}
	if &first[0] == &second[0] {
		t.Fatalf("two MarshalLoad() results share their backing array")
	}
	wantSecond := append([]byte(nil), second...)

	first[0] ^= 1
	if bytes.Equal(first, second) {
		t.Errorf("mutating first wire result changed no observable content")
	}
	if !bytes.Equal(second, wantSecond) {
		t.Errorf("mutating first wire result changed second result")
	}
	first = append(first, 'x')
	if !bytes.Equal(second, wantSecond) {
		t.Errorf("appending to first wire result changed second result")
	}
	if input.Stage != "implementation" || input.Bundle != "sha256:bundle" || input.ContinueToken != "opaque-token" || !reflect.DeepEqual(input.RulesContent, wantInputRules) {
		t.Errorf("MarshalLoad() mutated input: %#v", input)
	}

	input.RulesContent[0] = steering.RuleContent{Path: "changed.md", Text: "changed"}
	if !bytes.Equal(second, wantSecond) {
		t.Errorf("mutating input rules changed an existing wire result")
	}
}

func TestMarshalLoadMatchesJSONStringifyEscaping(t *testing.T) {
	input := steering.LoadDirective{
		Stage:  "st\"age\\\x00\n\t<>&\u2028日🚀",
		Bundle: "bu\"ndle\\\r<>&\u2029本🎯",
		Part:   1,
		Parts:  1,
		RulesContent: []steering.RuleContent{
			{Path: "pa\"th\\\u2029日", Text: "line\x00\n\t<>&\u2028日本語🚀"},
		},
		ContinueToken: "tok\"en\\<>&\u2028🎵",
	}
	want := `{"kind":"load-steering","stage":"st\"age\\\u0000\n\t<>&` +
		"\u2028日🚀" +
		`","bundle":"bu\"ndle\\\r<>&` +
		"\u2029本🎯" +
		`","part":1,"parts":1,"rules_content":[{"path":"pa\"th\\` +
		"\u2029日" +
		`","text":"line\u0000\n\t<>&` +
		"\u2028日本語🚀" +
		`"}],"continue_token":"tok\"en\\<>&` +
		"\u2028🎵" +
		`"}`

	got, err := steering.MarshalLoad(input)
	if err != nil {
		t.Fatalf("MarshalLoad() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("MarshalLoad() = %q, want %q", got, want)
	}
}
