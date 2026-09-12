package naturaljapanese

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

type Sentence struct {
	Line      int
	Text, Raw string
}
type Document struct {
	Raw, Masked []string
	Sentences   []Sentence
	Paragraphs  [][]Sentence
}

var structureLine = textPattern(`^\s*(#{1,6}(\s|$)|([-*+]|\p{Nd}+[.)])(\s|$)|>|\|.*\|)`)
var tableDelimiter = textPattern(`^\s*\|?(?:\s|[:|-])+\|(?:\s|[:|-])*\|?\s*$`)
var fenceLine = textPattern("^\\s*(`{3,}|~{3,})")
var frontmatterLine = textPattern(`^---\s*$`)
var linkURL = textPattern(`\]\([^)]*\)`)

// Python's Unicode whitespace includes the Unicode separators and U+001C–001F.
func textPattern(pattern string) *regexp.Regexp {
	return regexp.MustCompile(strings.ReplaceAll(pattern, `\s`, `[\p{Z}\t\n\v\f\r\x{0085}\x{001C}-\x{001F}]`))
}

func Prepare(text string) Document {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	d := Document{Raw: strings.Split(text, "\n")}
	d.Masked = make([]string, len(d.Raw))
	var fence string
	var front, comment bool
	var paragraph []Sentence
	for i, raw := range d.Raw {
		line := raw
		switch {
		case i == 0 && frontmatterLine.MatchString(line):
			front = true
			line = ""
		case front:
			if frontmatterLine.MatchString(line) {
				front = false
			}
			line = ""
		default:
			if match := fenceLine.FindStringSubmatchIndex(line); match != nil {
				run := line[match[2]:match[3]]
				if fence == "" {
					fence = run
				} else if run[0] == fence[0] && len(run) >= len(fence) && strings.TrimSpace(line[match[1]:]) == "" {
					fence = ""
				}
				line = ""
			} else if fence != "" {
				line = ""
			} else {
				line, comment = maskComment(line, comment)
				if structureLine.MatchString(line) || tableDelimiter.MatchString(line) {
					line = ""
				} else {
					line = maskInline(line)
					line = linkURL.ReplaceAllStringFunc(line, func(s string) string { return "](" + strings.Repeat(" ", utf8.RuneCountInString(s)-3) + ")" })
				}
			}
		}
		d.Masked[i] = line
		if strings.TrimSpace(line) == "" {
			if len(paragraph) > 0 {
				d.Paragraphs = append(d.Paragraphs, paragraph)
				paragraph = nil
			}
			continue
		}
		rs, rr := []rune(line), []rune(raw)
		start := 0
		for end := 0; end <= len(rs); end++ {
			if end < len(rs) && !strings.ContainsRune("。！？", rs[end]) {
				continue
			}
			piece := strings.TrimSpace(string(rs[start:end]))
			if piece != "" {
				s := Sentence{i + 1, piece, strings.TrimSpace(string(rr[start:end]))}
				d.Sentences = append(d.Sentences, s)
				paragraph = append(paragraph, s)
			}
			start = end + 1
		}
	}
	if len(paragraph) > 0 {
		d.Paragraphs = append(d.Paragraphs, paragraph)
	}
	return d
}
func maskComment(line string, open bool) (string, bool) {
	var out strings.Builder
	for len(line) > 0 {
		if open {
			at := strings.Index(line, "-->")
			if at < 0 {
				out.WriteString(strings.Repeat(" ", utf8.RuneCountInString(line)))
				break
			}
			end := at + 3
			out.WriteString(strings.Repeat(" ", utf8.RuneCountInString(line[:end])))
			line = line[end:]
			open = false
		} else {
			at := strings.Index(line, "<!--")
			if at < 0 {
				out.WriteString(line)
				break
			}
			out.WriteString(line[:at])
			line = line[at:]
			open = true
		}
	}
	return out.String(), open
}
func maskInline(line string) string {
	// Try the double-backtick alternative first, as the original regular expression does.
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '`' {
			continue
		}
		end := -1
		if i+1 < len(rs) && rs[i+1] == '`' {
			for j := i + 2; j+1 < len(rs); j++ {
				if rs[j] == '`' && rs[j+1] == '`' {
					if j > i+2 {
						end = j + 2
					}
					break
				}
			}
		}
		if end < 0 {
			for j := i + 1; j < len(rs); j++ {
				if rs[j] == '`' {
					if j > i+1 {
						end = j + 1
					}
					break
				}
			}
		}
		if end > 0 {
			for j := i; j < end; j++ {
				rs[j] = ' '
			}
			i = end - 1
		}
	}
	return string(rs)
}
