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
		name, text   string
		burst, leads int
	}{
		{"five", strings.Repeat("猫が走る。", 5), 0, 0}, {"six", strings.Repeat("猫が走る。", 6), 1, 6},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := Analyze(Prepare(tt.text).Sentences)
			if err != nil {
				t.Fatal(err)
			}
			got := Rhythm(ts, 6)
			if countCategory(got, "low_burstiness") != tt.burst || countCategory(got, "repeated_sentence_lead") != tt.leads {
				t.Fatal(got)
			}
		})
	}
	t.Run("distinct", func(t *testing.T) {
		ts, err := Analyze(Prepare("猫が走る。犬も行く。花を摘む。雨に濡れる。鳥の声。虫で驚く。").Sentences)
		if err != nil {
			t.Fatal(err)
		}
		if got := Rhythm(ts, 6); countCategory(got, "repeated_sentence_lead") != 0 {
			t.Fatal(got)
		}
	})
}
func TestRhythmBurstinessBoundary(t *testing.T) {
	for _, tt := range []struct {
		name            string
		low, high, want int
	}{{"at", 12, 50, 0}, {"below", 13, 49, 1}} {
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
