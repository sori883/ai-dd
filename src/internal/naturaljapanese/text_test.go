package naturaljapanese

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
		line              int
	}{
		{"frontmatter", "---\ntitle: 非常に重要\n---\n猫が走る。", "猫が走る", 4},
		{"structures", "# 見出し\n- 箇条書き\n> 引用\n| 表 | 値 |\n猫が走る。", "猫が走る", 5},
		{"fences", "````go\n```\n~~~\n````info\n隠れる。\n````\n猫が走る。", "猫が走る", 7},
		{"comment", "<!-- 見えない。\n-->猫が走る。", "猫が走る", 2},
		{"indent", "    猫が走る。", "猫が走る", 1},
		{"inline", "猫`隠れる。`が走る。", "猫      が走る", 1},
		{"double inline", "猫`` `。` ``が走る。", "猫         が走る", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := Prepare(tt.input)
			if len(d.Sentences) != 1 {
				t.Fatalf("sentences=%+v", d.Sentences)
			}
			s := d.Sentences[0]
			if s.Text != tt.want || s.Line != tt.line {
				t.Fatalf("got=%+v want=%q line=%d", s, tt.want, tt.line)
			}
		})
	}
}
func TestTextRawAndParagraphs(t *testing.T) {
	d := Prepare("猫`x`。犬！\n\n猫`x`。\n[リンク](https://example.org)です。")
	if len(d.Paragraphs) != 2 || len(d.Sentences) != 4 {
		t.Fatalf("%+v", d)
	}
	if d.Sentences[2].Raw != "猫`x`" || d.Sentences[2].Line != 3 {
		t.Fatal(d.Sentences)
	}
	if strings.Contains(d.Sentences[3].Text, "https") || !strings.Contains(d.Sentences[3].Raw, "https") {
		t.Fatal(d.Sentences[3])
	}
}
func TestTextUnicodeWhitespace(t *testing.T) {
	d := Prepare("---　\ntitle: example\n---　\n　# 見出し\n　- リスト\n　```\nコード\n　```\n猫。\r\n犬。\r鳥。")
	if len(d.Sentences) != 3 || d.Sentences[0].Line != 9 || d.Sentences[2].Line != 11 {
		t.Fatalf("%+v", d.Sentences)
	}
}
func TestTextUnicodeList(t *testing.T) {
	d := Prepare("１. 項目\n猫。")
	if len(d.Sentences) != 1 || d.Sentences[0].Line != 2 {
		t.Fatal(d.Sentences)
	}
}
