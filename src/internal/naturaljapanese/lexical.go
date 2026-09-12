package naturaljapanese

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MTLD computes the source algorithm in both directions; fewer than 20 tokens return zero.
func MTLD(tokens []string) float64 {
	if len(tokens) < 20 {
		return 0
	}
	direction := func(seq []string) float64 {
		factors := 0.0
		types := map[string]bool{}
		n := 0
		for _, tok := range seq {
			types[tok] = true
			n++
			if float64(len(types))/float64(n) <= .72 {
				factors++
				types = map[string]bool{}
				n = 0
			}
		}
		if n > 0 {
			ttr := float64(len(types)) / float64(n)
			factors += min((1-ttr)/(1-.72), 1)
		}
		if factors == 0 {
			return float64(len(seq))
		}
		return float64(len(seq)) / factors
	}
	reversed := slices.Clone(tokens)
	slices.Reverse(reversed)
	return (direction(tokens) + direction(reversed)) / 2
}
func content(t Token) bool { return member(pos(t), "名詞", "動詞", "形容詞", "副詞") }
func Lexical(ts []TokenizedSentence) []Finding {
	chars := 0
	var words []string
	types := map[string]bool{}
	for _, s := range ts {
		chars += utf8.RuneCountInString(s.Raw)
		for _, tok := range s.Tokens {
			if content(tok) {
				words = append(words, tok.Base)
				types[tok.Base] = true
			}
		}
	}
	if chars < 4000 || len(words) < 30 {
		return nil
	}
	var fs []Finding
	ttr := float64(len(types)) / float64(len(words))
	mtld := MTLD(words)
	if ttr < .45 {
		fs = append(fs, Finding{Line: ts[0].Line, Category: "low_lexical_diversity_ttr", Severity: "info", Excerpt: fmt.Sprintf("TTR=%.3f（内容語%d語中%d種類）", ttr, len(words), len(types)), Detail: "原形TTRが0.45未満"})
	}
	if mtld < 40 {
		fs = append(fs, Finding{Line: ts[0].Line, Category: "low_lexical_diversity_mtld", Severity: "info", Excerpt: fmt.Sprintf("MTLD=%.1f", mtld), Detail: "前後方向平均MTLDが40未満"})
	}
	return fs
}

var abstractWords = []string{"側面", "観点", "重要性", "可能性", "あり方", "存在", "意味", "本質", "価値", "意義", "課題", "問題", "要素", "要因", "背景", "傾向", "姿勢", "視点", "概念", "特徴", "性質", "状況", "状態", "変化"}
var exampleWords = []string{"たとえば", "例えば", "実際に", "実際には", "具体的には", "具体例として", "一例として", "先日", "昨日", "現に", "実例として"}
var numericQuantity = regexp.MustCompile(`[0-9０-９]+(年代|年間|世紀|年|月|日|時間|時|分|秒|人|円|%|％|kg|km|cm|mm|g|m|回|件|個|つ|割|倍|台|社|名|冊|本|杯|軒)?`)

func Specificity(d Document) ([]Finding, error) {
	var fs []Finding
	// Analyze masked lines, retaining punctuation and paragraph newlines as in the source.
	var para []Sentence
	evaluate := func() error {
		if len(para) == 0 {
			return nil
		}
		var lines []string
		for _, s := range para {
			lines = append(lines, s.Text)
		}
		text := strings.Join(lines, "\n")
		if utf8.RuneCountInString(text) < 80 {
			return nil
		}
		analyzed, err := Analyze(para)
		if err != nil {
			return err
		}
		n, proper, abstract := 0, 0, 0
		for _, s := range analyzed {
			for _, tok := range s.Tokens {
				if !content(tok) {
					continue
				}
				n++
				if pos(tok) == "名詞" {
					if len(tok.POS) > 1 && tok.POS[1] == "固有名詞" {
						proper++
					}
					if member(tok.Base, abstractWords...) {
						abstract++
					}
				}
			}
		}
		if n < 15 {
			return nil
		}
		pd := float64(proper) / float64(n)
		nd := float64(len(numericQuantity.FindAllStringIndex(text, -1))) / float64(n)
		ad := float64(abstract) / float64(n)
		bonus := 0.0
		for _, word := range exampleWords {
			if strings.Contains(text, word) {
				bonus = .1
				break
			}
		}
		score := pd + nd + bonus - 1.5*ad
		if score < -0.15 {
			raw := []rune(strings.TrimSpace(para[0].Raw))
			fs = append(fs, Finding{Line: para[0].Line, Category: "low_specificity", Severity: "info", Excerpt: string(raw[:min(40, len(raw))]), Detail: fmt.Sprintf("具体性スコア=%.3f、固有名詞密度=%.3f、数値密度=%.3f、抽象名詞率=%.3f、例示加点=%.1f。素材不足の可能性があり情報収集を検討", score, pd, nd, ad, bonus)})
		}
		return nil
	}
	for i, line := range d.Masked {
		if strings.TrimSpace(line) == "" {
			if err := evaluate(); err != nil {
				return nil, err
			}
			para = nil
		} else {
			para = append(para, Sentence{i + 1, line, d.Raw[i]})
		}
	}
	if err := evaluate(); err != nil {
		return nil, err
	}
	return fs, nil
}
