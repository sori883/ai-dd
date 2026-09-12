package naturaljapanese

import (
	"strings"
	"testing"
)

func TestRhythmMora(t *testing.T) {
	for _, tt := range []struct {
		name, reading string
		want          int
	}{{"digraph", "キャ", 1}, {"long stop", "キャットー", 4}, {"initial small", "ャ", 1}, {"punctuation", "、", 1}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mora([]Token{{Reading: tt.reading}}); got != tt.want {
				t.Fatal(got, tt.want)
			}
		})
	}
	if got := Mora([]Token{{Reading: "カ"}, {Reading: "ャ"}}); got != 2 {
		t.Fatal(got)
	}
}
func TestRhythm(t *testing.T) {
	for _, tt := range []struct {
		name, text, category string
		count                int
	}{{"five", strings.Repeat("猫が走る。", 5), "low_burstiness", 0}, {"six", strings.Repeat("猫が走る。", 6), "low_burstiness", 1}, {"five leads", strings.Repeat("猫が走る。", 5), "repeated_sentence_lead", 0}, {"six leads", strings.Repeat("猫が走る。", 6), "repeated_sentence_lead", 6}, {"distinct", "猫が走る。犬も行く。花を摘む。雨に濡れる。鳥の声。虫で驚く。", "repeated_sentence_lead", 0}} {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := Analyze(Prepare(tt.text).Sentences)
			if err != nil {
				t.Fatal(err)
			}
			fs := Rhythm(ts, 6)
			if countCategory(fs, tt.category) != tt.count {
				t.Fatal(fs)
			}
		})
	}
}
func TestRhythmBurstinessBoundary(t *testing.T) {
	for _, tt := range []struct {
		name            string
		low, high, want int
	}{{"at", 12, 50, 0}, {"below", 13, 49, 1}, {"above", 11, 51, 0}} {
		t.Run(tt.name, func(t *testing.T) {
			ts := []TokenizedSentence{}
			for i := 0; i < 6; i++ {
				length := tt.low
				if i >= 3 {
					length = tt.high
				}
				ts = append(ts, TokenizedSentence{Sentence: Sentence{Line: i + 1}, Tokens: []Token{{Reading: strings.Repeat("カ", length)}}})
			}
			if n := countCategory(Rhythm(ts, 6), "low_burstiness"); n != tt.want {
				t.Fatal(n, tt.want)
			}
		})
	}
}
