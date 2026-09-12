package naturaljapanese

import (
	"fmt"
	"regexp"
	"strings"
)

func Mora(tokens []Token) int {
	total := 0
	for _, t := range tokens {
		reading := t.Reading
		if reading == "" {
			reading = t.Surface
		}
		count := 0
		for _, r := range reading {
			if count > 0 && strings.ContainsRune("ァィゥェォャュョヮ", r) {
				continue
			}
			count++
		}
		total += count
	}
	return total
}

var latinTech = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9\-_.]*$`)

func Rhythm(ts []TokenizedSentence, leadThreshold int) []Finding {
	var fs []Finding
	if len(ts) >= 6 {
		lengths := make([]float64, len(ts))
		for i, s := range ts {
			lengths[i] = float64(Mora(s.Tokens))
		}
		mean, sd := meanSD(lengths)
		b := 0.0
		if sd+mean > 0 {
			b = (sd - mean) / (sd + mean)
		}
		if b < -0.24 {
			fs = append(fs, Finding{Line: ts[0].Line, Category: "low_burstiness", Severity: "warn", Excerpt: fmt.Sprintf("burstiness=%.3f (モーラ近似長 平均=%.1f, 標準偏差=%.1f)", b, mean, sd), Detail: "burstinessが-0.24未満。文の長短のメリハリが乏しい疑い"})
		}
	}
	type lead struct {
		s    TokenizedSentence
		key  string
		tech bool
	}
	var leads []lead
	groups := map[string][]int{}
	var order []string
	for _, s := range ts {
		tokens := s.Tokens
		for len(tokens) > 0 && symbol(tokens[0]) {
			tokens = tokens[1:]
		}
		if len(tokens) < 2 {
			continue
		}
		key := tokens[0].Surface + tokens[1].Surface
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], s.Line)
		tech := latinTech.MatchString(tokens[0].Surface) || (len(tokens[0].POS) > 1 && tokens[0].POS[0] == "名詞" && tokens[0].POS[1] == "固有名詞")
		leads = append(leads, lead{s, key, tech})
	}
	for _, key := range order {
		lines := groups[key]
		if len(lines) < leadThreshold {
			continue
		}
		for _, lead := range leads {
			if lead.key != key {
				continue
			}
			reason := "人間の意図的な反復との区別がつかないため参考情報"
			if lead.tech {
				reason = "固有名詞/技術用語由来の可能性"
			}
			rs := []rune(lead.s.Raw)
			fs = append(fs, Finding{Line: lead.s.Line, Category: "repeated_sentence_lead", Severity: "info", Excerpt: string(rs[:min(20, len(rs))]), Detail: fmt.Sprintf("文頭2形態素「%s」が%d回反復（閾値%d回）。%s", key, len(lines), leadThreshold, reason), RelatedLines: lines})
		}
	}
	return fs
}
