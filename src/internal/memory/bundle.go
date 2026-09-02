package memory

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var closedHTMLComment = regexp.MustCompile(`<!--[\s\S]*?-->`)

var templatePreambleLines = [...]string{
	"> This team's affirmed practices and corrections. Loaded after `org.md` as",
	"> strict-additive guidance; contradictions with broader policy are rejected.",
	"> Populated by the practices-discovery affirmation gate. Edit at the gate,",
	"> not directly.",
	"> Project-specific specialisation and corrections. Loaded after `org.md` and",
	"> `team.md` as strict-additive guidance; contradictions with broader policy",
	"> are rejected. Populated by practices-discovery and the self-learning loop.",
	">",
	"> Use sparingly: most teams don't need a project layer. Reach for it",
	"> only when this specific project needs stable, durable guidance beyond the",
	"> team practice (for example, package-specific release checks or an additional",
	"> regression suite for a legacy component).",
}

// BuildBundle returns the substantive memory sources in their input order.
func BuildBundle(sources []Source) []Source {
	bundle := make([]Source, 0, len(sources))
	for _, source := range sources {
		if isSubstantiveBundleContent(source.Content) {
			bundle = append(bundle, source)
		}
	}
	return bundle
}

func isSubstantiveBundleContent(content string) bool {
	content = closedHTMLComment.ReplaceAllString(content, "")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := trimECMAScriptWhitespace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") ||
			isTemplatePreambleLine(trimmed) || isASCIIHyphenSeparator(trimmed) {
			continue
		}
		return true
	}
	return false
}

func isTemplatePreambleLine(line string) bool {
	for _, preambleLine := range templatePreambleLines {
		if line == preambleLine {
			return true
		}
	}
	return false
}

func isASCIIHyphenSeparator(line string) bool {
	if len(line) < 3 {
		return false
	}
	for index := 0; index < len(line); index++ {
		if line[index] != '-' {
			return false
		}
	}
	return true
}

func trimECMAScriptWhitespace(value string) string {
	start := 0
	for start < len(value) {
		runeValue, size := utf8.DecodeRuneInString(value[start:])
		if !isECMAScriptTrimRune(runeValue) {
			break
		}
		start += size
	}

	end := len(value)
	for end > start {
		runeValue, size := utf8.DecodeLastRuneInString(value[:end])
		if !isECMAScriptTrimRune(runeValue) {
			break
		}
		end -= size
	}
	return value[start:end]
}

func isECMAScriptTrimRune(value rune) bool {
	switch value {
	case '\u0009', '\u000a', '\u000b', '\u000c', '\u000d',
		' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000', '\ufeff':
		return true
	}
	return value >= '\u2000' && value <= '\u200a'
}
