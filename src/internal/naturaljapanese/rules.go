package naturaljapanese

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

type Report struct {
	SchemaVersion int            `json:"schema_version"`
	Engine        string         `json:"engine"`
	Dictionary    string         `json:"dictionary"`
	File          string         `json:"file"`
	Stats         map[string]any `json:"stats"`
	Findings      []Finding      `json:"findings"`
	Resolved      []Finding      `json:"resolved,omitempty"`
	Comparison    map[string]int `json:"comparison,omitempty"`
}

var Categories = []string{"forbidden_phrase", "translationese", "antithesis_repetition", "low_sentence_variance", "english_syntax_inanimate_subject", "nominal_ending", "uniform_paragraph_structure", "translationese_morph", "inanimate_subject_morph", "low_burstiness", "repeated_sentence_lead", "low_lexical_diversity_ttr", "low_lexical_diversity_mtld", "low_specificity"}

func Check(text, file, genre string) (Report, error) {
	r := Report{SchemaVersion: 1, Engine: "Kagome v2.11.0", Dictionary: "UniDic v1.2.6", File: file, Stats: map[string]any{}, Findings: []Finding{}}
	if !member(genre, "", "essay", "tech", "business") {
		return r, fmt.Errorf("unknown genre %q", genre)
	}
	if !utf8.ValidString(text) {
		return r, fmt.Errorf("input is not utf-8")
	}
	nominal, lead, critical := 2000, 6, .03
	switch genre {
	case "essay":
		nominal, lead = 1500, 5
	case "tech":
		nominal, lead, critical = 3000, 7, .045
	case "business":
		nominal, lead = 3000, 7
	}
	d := Prepare(text)
	ts, err := Analyze(d.Sentences)
	if err != nil {
		return r, err
	}
	r.Findings = append(r.Findings, Surface(d, critical)...)
	r.Findings = append(r.Findings, Structure(d)...)
	r.Findings = append(r.Findings, Morph(ts, nominal)...)
	r.Findings = append(r.Findings, Rhythm(ts, lead)...)
	r.Findings = append(r.Findings, Lexical(ts)...)
	specific, err := Specificity(d)
	if err != nil {
		return r, err
	}
	r.Findings = append(r.Findings, specific...)
	sort.SliceStable(r.Findings, func(i, j int) bool { return r.Findings[i].Line < r.Findings[j].Line })
	counts := map[string]int{}
	for _, f := range r.Findings {
		counts[f.Category]++
	}
	chars, words, nominals := 0, 0, 0
	types := map[string]bool{}
	var bases []string
	var moras []float64
	for _, s := range ts {
		chars += utf8.RuneCountInString(s.Raw)
		moras = append(moras, float64(Mora(s.Tokens)))
		effective := s.Tokens
		for len(effective) > 0 && symbol(effective[len(effective)-1]) {
			effective = effective[:len(effective)-1]
		}
		if len(effective) > 0 && pos(effective[len(effective)-1]) == "名詞" {
			nominals++
		}
		for _, t := range s.Tokens {
			if content(t) {
				words++
				types[t.Base] = true
				bases = append(bases, t.Base)
			}
		}
	}
	r.Stats = map[string]any{"genre": genre, "total_findings": len(r.Findings), "by_category": counts, "total_sentences": len(ts), "total_paragraphs": len(d.Paragraphs), "doc_char_count": chars, "nominal_ending_count": nominals, "content_token_count": words, "lexical_skipped_too_short": chars < 4000, "ttr": nil, "mtld": nil, "burstiness": nil}
	if chars >= 4000 && words >= 30 {
		r.Stats["ttr"] = float64(len(types)) / float64(words)
		r.Stats["mtld"] = MTLD(bases)
	}
	if len(ts) >= 6 {
		mean, sd := meanSD(moras)
		b := 0.0
		if sd+mean > 0 {
			b = (sd - mean) / (sd + mean)
		}
		r.Stats["burstiness"] = b
		r.Stats["mora_mean"] = mean
		r.Stats["mora_stdev"] = sd
	}
	return r, nil
}
