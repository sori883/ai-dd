package naturaljapanese

import (
	"strings"
	"testing"
)

func countCategory(fs []Finding, c string) int {
	n := 0
	for _, f := range fs {
		if f.Category == c {
			n++
		}
	}
	return n
}
func TestSurface(t *testing.T) {
	for _, tt := range []struct {
		name, text, category string
		count                int
	}{
		{"catalog", "重要なのは非常に重要ということ。非常に重要。", "forbidden_phrase", 2},
		{"ordinary", "最後に、まさに猫を見た。", "forbidden_phrase", 0},
		{"translation", "実行することができる。することができます。", "translationese", 2},
		{"masked", "`することができる`。", "translationese", 0},
		{"inanimate", "これは結果をもたらす。", "english_syntax_inanimate_subject", 1},
		{"human", "私は猫を見る。", "english_syntax_inanimate_subject", 0},
		{"two antitheses", "猫ではなく犬。\n犬ではなく猫。", "antithesis_repetition", 0},
		{"three antitheses", "猫ではなく犬。\n犬ではなく猫。\n鳥ではなく虫。", "antithesis_repetition", 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := Surface(Prepare(tt.text), .03)
			if n := countCategory(fs, tt.category); n != tt.count {
				t.Fatalf("got %d want %d: %+v", n, tt.count, fs)
			}
		})
	}
}
func TestSurfaceSeverity(t *testing.T) {
	for _, tt := range []struct {
		name      string
		sentences int
		want      string
	}{{"critical", 100, "critical"}, {"warn", 101, "warn"}, {"info", 151, "info"}} {
		t.Run(tt.name, func(t *testing.T) {
			text := strings.Repeat("猫ではなく犬。\n", 3) + strings.Repeat("猫。\n", tt.sentences-3)
			fs := Surface(Prepare(text), .03)
			for _, f := range fs {
				if f.Category == "antithesis_repetition" && (f.Severity != tt.want || len(f.RelatedLines) != 3) {
					t.Fatal(f)
				}
			}
			if countCategory(fs, "antithesis_repetition") != 3 {
				t.Fatal(fs)
			}
		})
	}
}
func TestSurfaceCatalogOrder(t *testing.T) {
	fs := Surface(Prepare("大切なのは重要なのはという話。"), .03)
	if len(fs) != 2 || !strings.Contains(fs[0].Detail, "重要なのは") {
		t.Fatal(fs)
	}
}
