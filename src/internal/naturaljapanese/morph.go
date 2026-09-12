package naturaljapanese

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

var smellVerbs = []string{"もたらす", "示す", "意味する", "証明する", "生み出す", "反映する", "示唆する", "物語る", "浮き彫りにする", "後押しする"}

func pos(t Token) string {
	if len(t.POS) == 0 {
		return ""
	}
	return t.POS[0]
}
func member(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func symbol(t Token) bool { return member(pos(t), "補助記号", "空白") }
func tokenExcerpt(s TokenizedSentence, start, end int) string {
	rs := []rune(s.Raw)
	a := min(s.Tokens[start].Start, len(rs))
	b := min(s.Tokens[end].End, len(rs))
	return string(rs[a:b])
}
func Morph(ts []TokenizedSentence, minChars int) []Finding {
	var fs []Finding
	chars, nominal := 0, 0
	for _, s := range ts {
		chars += utf8.RuneCountInString(s.Raw)
		tokens := s.Tokens
		last := len(tokens) - 1
		for last >= 0 && symbol(tokens[last]) {
			last--
		}
		if last >= 0 && pos(tokens[last]) == "名詞" {
			nominal++
		}
		skip := -1
		for i, t := range tokens {
			if i+2 < len(tokens) && t.Surface == "こと" && pos(t) == "名詞" && pos(tokens[i+1]) == "助詞" && member(tokens[i+1].Surface, "が", "は") && pos(tokens[i+2]) == "動詞" && strings.HasPrefix(tokens[i+2].Surface, "でき") {
				fs = append(fs, Finding{Line: s.Line, Category: "translationese_morph", Excerpt: tokenExcerpt(s, max(0, i-4), i+2), Severity: "info", Detail: "品詞列マッチ: こと+が/は+できる型"})
			}
			if i <= skip {
				continue
			}
			end := i
			subject := member(t.Surface, "これ", "それ", "あれ", "それら") || (pos(t) == "名詞" && member(t.Surface, "こと", "事実", "の"))
			if !subject && i+1 < len(tokens) && member(t.Surface+tokens[i+1].Surface, "この事実", "そのこと") {
				subject = true
				end++
			}
			if !subject {
				continue
			}
			skip = end
			j := end + 1
			if j >= len(tokens) || pos(tokens[j]) != "助詞" || !member(tokens[j].Surface, "が", "は") {
				continue
			}
			for k := j + 1; k < len(tokens); k++ {
				if pos(tokens[k]) != "動詞" {
					continue
				}
				verb := tokens[k].Base
				// UniDic separates サ変 nouns and する, unlike the source's long units.
				if verb == "する" && k > j+1 {
					verb = tokens[k-1].Base + verb
					if k > j+2 && tokens[k-1].Surface == "に" {
						verb = tokens[k-2].Base + "にする"
					}
				}
				if member(verb, smellVerbs...) {
					fs = append(fs, Finding{Line: s.Line, Category: "inanimate_subject_morph", Excerpt: tokenExcerpt(s, max(0, i-3), k), Severity: "info", Detail: "抽象主語+助詞+他動詞的述語「" + verb + "」"})
					break
				}
			}
		}
	}
	if len(ts) >= 5 && chars >= minChars && nominal == 0 {
		fs = append(fs, Finding{Line: ts[len(ts)-1].Line, Category: "nominal_ending", Excerpt: fmt.Sprintf("体言止め0件（全%d文、約%d字）", len(ts), chars), Severity: "info", Detail: "長文で名詞終止が一つもない。人間の修辞との比較材料"})
	}
	return fs
}
