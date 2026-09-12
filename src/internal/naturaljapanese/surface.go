package naturaljapanese

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Finding struct {
	Line         int    `json:"line"`
	Category     string `json:"category"`
	Excerpt      string `json:"excerpt"`
	Severity     string `json:"severity"`
	Detail       string `json:"detail"`
	RelatedLines []int  `json:"related_lines,omitempty"`
	Status       string `json:"status,omitempty"`
}

var forbiddenPhrases = []string{
	"と言えるでしょう",
	"と言えるだろう",
	"と言えます",
	"ということになるでしょう",
	"のではないでしょうか",
	"重要なのは",
	"大切なのは",
	"ポイントは",
	"結論から言うと",
	"結論として",
	"いかがでしたか",
	"いかがでしょうか",
	"まとめると",
	"総じて",
	"非常に重要",
	"極めて重要",
	"言うまでもなく",
	"言うまでもありません",
	"まさしく",
	"さて、",
	"それでは、",
	"このように",
	"このような中",
	"ここで注目したいのは",
	"見ていきましょう",
	"紹介していきます",
	"解説していきます",
	"深掘りしていきます",
	"一概には言えません",
	"個人差がありますが",
	"あくまで一例ですが",
	"正面から扱う",
	"正面から見る",
	"正面から書く",
	"正面から立てる",
	"正面から回収する",
	"不可欠",
	"核心的",
	"鍵となる",
	"根本的な",
	"多角的",
	"包括的",
	"総合的",
	"掘り下げる",
	"深掘りする",
	"言語化する",
	"について見ていく",
	"を探求する",
}
var weakPhrases = []string{"このように", "さて、", "ポイントは", "不可欠", "重要なのは"}
var translationPatterns = []string{
	"することができ(る|ます|た)",
	"することが可能(です|だ|になる)",
	"と言えるだろう",
	"という点で",
	"という観点(から|で)",
	"にとって(重要|不可欠)",
	"を持つ(こと|存在)",
	"することによって",
	"であることは間違いない",
	"に他ならない",
}

var antithesisPatterns = []string{`ではなく、?.{0,30}`, `だけでなく.{0,10}も`}
var inanimatePatterns = []string{`(これ|それ|この事実|そのこと)(は|が).{0,40}(もたらす|示す|意味する|証明する|生み出す|反映する)`, `.{0,20}(こと|事実)(は|が).{0,40}(もたらす|示す|意味する|証明する|生み出す|反映する)`}

func excerpt(raw, masked string, start, end, pad int) string {
	a := max(0, utf8.RuneCountInString(masked[:start])-pad)
	b := utf8.RuneCountInString(masked[:end]) + pad
	rs := []rune(raw)
	if len(rs) < b {
		rs = []rune(masked)
	}
	return strings.TrimSpace(string(rs[a:min(b, len(rs))]))
}
func Surface(d Document, critical float64) []Finding {
	var findings, hits []Finding
	for i, line := range d.Masked {
		for _, phrase := range forbiddenPhrases {
			at := strings.Index(line, phrase)
			if at < 0 {
				continue
			}
			sev := "warn"
			for _, weak := range weakPhrases {
				if phrase == weak {
					sev = "info"
				}
			}
			findings = append(findings, Finding{Line: i + 1, Category: "forbidden_phrase", Excerpt: excerpt(d.Raw[i], line, at, at+len(phrase), 10), Severity: sev, Detail: "禁止語/LLM常套句ヒット: 「" + phrase + "」"})
		}
		for _, group := range []struct {
			patterns []string
			category string
			pad      int
		}{{translationPatterns, "translationese", 10}, {inanimatePatterns, "english_syntax_inanimate_subject", 0}, {antithesisPatterns, "antithesis_repetition", 0}} {
			for _, pattern := range group.patterns {
				for _, span := range regexp.MustCompile(pattern).FindAllStringIndex(line, -1) {
					f := Finding{Line: i + 1, Category: group.category, Excerpt: excerpt(d.Raw[i], line, span[0], span[1], group.pad), Severity: "info", Detail: "表層パターン: " + pattern}
					if group.category == "antithesis_repetition" {
						hits = append(hits, f)
					} else {
						findings = append(findings, f)
					}
				}
			}
		}
	}
	if len(hits) >= 3 {
		ratio := 0.0
		if len(d.Sentences) > 0 {
			ratio = float64(len(hits)) / float64(len(d.Sentences))
		}
		sev := "warn"
		if ratio < .02 {
			sev = "info"
		} else if ratio >= critical {
			sev = "critical"
		}
		lines := make([]int, len(hits))
		for i, f := range hits {
			lines[i] = f.Line
		}
		for _, f := range hits {
			f.Severity = sev
			f.RelatedLines = lines
			f.Detail = fmt.Sprintf("否定→肯定対比%d回、総文数に対する比率=%.1f%%", len(hits), ratio*100)
			findings = append(findings, f)
		}
	}
	return findings
}
