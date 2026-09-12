package naturaljapanese

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestLexicalMTLD(t *testing.T) {
	same := make([]string, 20)
	unique := make([]string, 20)
	for i := range same {
		same[i] = "猫"
		unique[i] = fmt.Sprint(i)
	}
	for _, tt := range []struct {
		name   string
		tokens []string
		want   float64
	}{{"identical", same, 2}, {"unique", unique, 20}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := MTLD(tt.tokens); math.Abs(got-tt.want) > 1e-9 {
				t.Fatal(got, tt.want)
			}
		})
	}
}
func TestLexical(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		count      int
	}{{"short", strings.Repeat("猫が走る。", 999), 0}, {"boundary", strings.Repeat("猫が走る。", 1000), 2}, {"masked", strings.Repeat("# 猫が走る。\n", 1000) + "猫が走る。", 0}} {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := Analyze(Prepare(tt.text).Sentences)
			if err != nil {
				t.Fatal(err)
			}
			if got := Lexical(ts); len(got) != tt.count {
				t.Fatal(got)
			}
		})
	}
}
func TestLexicalSpecificity(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		count      int
	}{{"short", "問題の側面と課題。", 0}, {"abstract", strings.Repeat("問題の側面と課題の意味と価値。", 8), 1}, {"specific", strings.Repeat("東京の猫が2026年に犬を追った。", 8), 0}} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Specificity(Prepare(tt.text))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.count {
				t.Fatal(got)
			}
		})
	}
}
func TestLexicalTTRBoundary(t *testing.T) {
	for _, tt := range []struct {
		name        string
		types, want int
	}{{"at", 18, 0}, {"below", 17, 1}, {"above", 19, 0}} {
		t.Run(tt.name, func(t *testing.T) {
			tokens := []Token{}
			for i := 0; i < 40; i++ {
				tokens = append(tokens, Token{Base: fmt.Sprint(i % tt.types), POS: []string{"名詞"}})
			}
			ts := []TokenizedSentence{{Sentence: Sentence{Line: 1, Raw: strings.Repeat("猫", 4000)}, Tokens: tokens}}
			if n := countCategory(Lexical(ts), "low_lexical_diversity_ttr"); n != tt.want {
				t.Fatal(n)
			}
		})
	}
}
