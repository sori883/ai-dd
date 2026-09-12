package naturaljapanese

import (
	"strings"
	"testing"
)

func TestStructure(t *testing.T) {
	for _, tt := range []struct {
		name, text, category string
		count                int
	}{
		{"four sentences", strings.Repeat("猫が走る。", 4), "low_sentence_variance", 0},
		{"five sentences", strings.Repeat("猫が走る。", 5), "low_sentence_variance", 1},
		{"variable", "猫。猫が窓から出て門を抜けた。犬。長い廊下を走って庭へ出た。鳥。", "low_sentence_variance", 0},
		{"three paragraphs", strings.Repeat("猫。\n\n", 3), "uniform_paragraph_structure", 0},
		{"four paragraphs", strings.Repeat("猫。\n\n", 4), "uniform_paragraph_structure", 1},
		{"variable paragraphs", "猫。\n\n猫。犬。\n\n猫。犬。鳥。\n\n猫。犬。鳥。虫。", "uniform_paragraph_structure", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := Structure(Prepare(tt.text))
			if countCategory(fs, tt.category) != tt.count {
				t.Fatal(fs)
			}
		})
	}
}
func TestStructureCVBoundary(t *testing.T) {
	text := strings.Repeat("猫犬鳥。", 10) + strings.Repeat("猫犬鳥虫花。", 10)
	if n := countCategory(Structure(Prepare(text)), "low_sentence_variance"); n != 0 {
		t.Fatal("CV exactly .25 must not fire")
	}
	text = strings.Repeat("猫。", 17) + "\n\n" + strings.Repeat("猫。", 23) + "\n\n" + strings.Repeat("猫。", 17) + "\n\n" + strings.Repeat("猫。", 23)
	if n := countCategory(Structure(Prepare(text)), "uniform_paragraph_structure"); n != 0 {
		t.Fatal("CV exactly .15 must not fire")
	}
}
