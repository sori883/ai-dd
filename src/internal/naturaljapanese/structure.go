package naturaljapanese

import (
	"fmt"
	"math"
	"unicode/utf8"
)

func meanSD(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(variance / float64(len(values)))
}
func Structure(d Document) []Finding {
	var fs []Finding
	if len(d.Sentences) >= 5 {
		lengths := make([]float64, len(d.Sentences))
		for i, s := range d.Sentences {
			lengths[i] = float64(utf8.RuneCountInString(s.Text))
		}
		mean, sd := meanSD(lengths)
		if mean > 0 && sd/mean < .25 {
			fs = append(fs, Finding{Line: d.Sentences[0].Line, Category: "low_sentence_variance", Severity: "warn", Excerpt: fmt.Sprintf("文数=%d, 平均文長=%.1f字, 変動係数=%.3f", len(lengths), mean, sd/mean), Detail: "文長の変動係数が0.25未満"})
		}
	}
	if len(d.Paragraphs) >= 4 {
		counts := make([]float64, len(d.Paragraphs))
		for i, p := range d.Paragraphs {
			counts[i] = float64(len(p))
		}
		mean, sd := meanSD(counts)
		cv := 0.0
		if mean > 0 {
			cv = sd / mean
		}
		if cv < .15 {
			fs = append(fs, Finding{Line: 1, Category: "uniform_paragraph_structure", Severity: "info", Excerpt: fmt.Sprintf("段落数=%d, 各段落の文数=%v", len(counts), counts), Detail: fmt.Sprintf("段落あたり文数の変動係数=%.3f（0.15未満）", cv)})
		}
	}
	return fs
}
