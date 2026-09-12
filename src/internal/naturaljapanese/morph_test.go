package naturaljapanese

import (
	"strings"
	"testing"
)

func TestMorph(t *testing.T) {
	for _, tt := range []struct {
		name, text, category string
		count                int
	}{
		{"translation", "歩くことができる。", "translationese_morph", 1},
		{"non translation", "歩ける。", "translationese_morph", 0},
		{"kanji surface source boundary", "歩くことが出来る。", "translationese_morph", 0},
		{"inflected", "これが結果をもたらした。", "inanimate_subject_morph", 1},
		{"compound subject", "この事実は結果を示した。", "inanimate_subject_morph", 1},
		{"suru predicate", "そのことは意味する。", "inanimate_subject_morph", 1},
		{"animate", "私は猫を見た。", "inanimate_subject_morph", 0},
		{"missing marker", "これを示した。", "inanimate_subject_morph", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := Analyze(Prepare(tt.text).Sentences)
			if err != nil {
				t.Fatal(err)
			}
			fs := Morph(ts, 2000)
			if countCategory(fs, tt.category) != tt.count {
				t.Fatalf("%+v tokens=%+v", fs, ts)
			}
		})
	}
}
func TestMorphNominal(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		count      int
	}{{"short", strings.Repeat("猫が走る。", 5), 0}, {"long", strings.Repeat("猫が走る。", 500), 1}, {"nominal", strings.Repeat("猫が走る。", 500) + "猫。", 0}, {"masked", strings.Repeat("# 猫が走る。\n", 500) + "猫が走る。", 0}} {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := Analyze(Prepare(tt.text).Sentences)
			if err != nil {
				t.Fatal(err)
			}
			fs := Morph(ts, 2000)
			if countCategory(fs, "nominal_ending") != tt.count {
				t.Fatal(fs)
			}
		})
	}
}
func TestMorphCharacterBoundary(t *testing.T) {
	for _, tt := range []struct {
		name        string
		chars, want int
	}{{"below", 1999, 0}, {"at", 2000, 1}} {
		t.Run(tt.name, func(t *testing.T) {
			ts := []TokenizedSentence{}
			for i := 0; i < 5; i++ {
				n := 400
				if i == 0 {
					n += tt.chars - 2000
				}
				ts = append(ts, TokenizedSentence{Sentence: Sentence{Line: i + 1, Raw: strings.Repeat("猫", n)}, Tokens: []Token{{Surface: "走る", Base: "走る", POS: []string{"動詞"}}}})
			}
			if n := countCategory(Morph(ts, 2000), "nominal_ending"); n != tt.want {
				t.Fatal(n)
			}
		})
	}
}
