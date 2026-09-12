package naturaljapanese

import (
	"fmt"
	"github.com/ikawaha/kagome-dict/uni"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// Token retains UniDic surface, lemma, inflected pronunciation, POS and rune offsets.
type Token struct {
	Surface, Base, Reading string
	POS                    []string
	Start, End             int
}
type TokenizedSentence struct {
	Sentence
	Tokens []Token
}

func Analyze(sentences []Sentence) ([]TokenizedSentence, error) {
	if len(sentences) == 0 {
		return nil, nil
	}
	analyzer, err := tokenizer.New(uni.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return nil, fmt.Errorf("create analyzer: %w", err)
	}
	result := make([]TokenizedSentence, 0, len(sentences))
	for _, s := range sentences {
		ts := TokenizedSentence{Sentence: s}
		for _, t := range analyzer.Tokenize(s.Text) {
			base, ok := t.BaseForm()
			if !ok || base == "" || base == "*" {
				base = t.Surface
			}
			// UniDic does not register ReadingIndex; Pron is the inflected pronunciation.
			reading := t.Surface
			features := t.Features()
			if len(features) > uni.Pron && features[uni.Pron] != "" && features[uni.Pron] != "*" {
				reading = features[uni.Pron]
			}

			ts.Tokens = append(ts.Tokens, Token{Surface: t.Surface, Base: base, Reading: reading, POS: t.POS(), Start: t.Start, End: t.End})
		}
		result = append(result, ts)
	}
	return result, nil
}
