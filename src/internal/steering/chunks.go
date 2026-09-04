// Package steering reads the required rule documents supplied by the caller.
package steering

import (
	"strings"
	"unicode/utf8"
)

// ChunkRules groups validated UTF-8 rule content into ordered delivery chunks.
// It preserves paths and text without I/O, splits Markdown sections first, and
// splits oversized sections at code-point boundaries toward the 20 KiB JSON
// target; final transport must enforce the whole-directive cap.
func ChunkRules(content []RuleContent) [][]RuleContent {
	if len(content) == 0 {
		return nil
	}

	chunks := make([][]RuleContent, 0, 1)
	chunk := make([]RuleContent, 0, len(content))
	chunkBytes := 2
	for _, rule := range content {
		for _, section := range splitMarkdownSections(rule.Text) {
			for _, piece := range splitOversizedSection(rule.Path, section) {
				pieceBytes := jsonRuleSize(piece)
				candidateBytes := chunkBytes + pieceBytes
				if len(chunk) > 0 {
					candidateBytes++
				}
				if len(chunk) > 0 && candidateBytes > maxChunkBytes {
					chunks = append(chunks, chunk)
					chunk = make([]RuleContent, 0, len(content))
					chunkBytes = 2
					candidateBytes = chunkBytes + pieceBytes
				}
				chunk = append(chunk, piece)
				chunkBytes = candidateBytes
			}
		}
	}
	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}
	return chunks
}

const maxChunkBytes = 20 * 1024

func jsonRuleSize(rule RuleContent) int {
	return 17 + jsonStringSize(rule.Path) + jsonStringSize(rule.Text)
}

func jsonStringSize(value string) int {
	return 2 + jsonStringByteSize(value)
}

func jsonStringByteSize(value string) int {
	size := 0
	for index := 0; index < len(value); index++ {
		size++
		switch value[index] {
		case '"', '\\', '\b', '\f', '\n', '\r', '\t':
			size += 1
		case 0, 1, 2, 3, 4, 5, 6, 7, 11, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31:
			size += 5
		}
	}
	return size
}

func splitOversizedSection(path, text string) []RuleContent {
	whole := RuleContent{Path: path, Text: text}
	if 2+jsonRuleSize(whole) <= maxChunkBytes {
		return []RuleContent{whole}
	}

	baseBytes := 2 + jsonRuleSize(RuleContent{Path: path})
	pieces := make([]RuleContent, 0, 2)
	start := 0
	for start < len(text) {
		end := start
		pieceBytes := baseBytes
		for end < len(text) {
			_, width := utf8.DecodeRuneInString(text[end:])
			next := end + width
			runeBytes := jsonStringByteSize(text[end:next])
			if end > start && pieceBytes+runeBytes > maxChunkBytes {
				break
			}

			pieceBytes += runeBytes
			end = next
			if pieceBytes > maxChunkBytes {
				break
			}
		}
		pieces = append(pieces, RuleContent{Path: path, Text: text[start:end]})
		start = end
	}
	return pieces
}

func splitMarkdownSections(text string) []string {
	if text == "" {
		return []string{""}
	}

	sections := make([]string, 0, 1)
	sectionStart := 0
	lineStart := 0
	for lineStart < len(text) {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += lineStart + 1
		}

		if isMarkdownHeading(text[lineStart:lineEnd]) {
			if lineStart > sectionStart {
				sections = append(sections, text[sectionStart:lineStart])
			}
			sectionStart = lineStart
		}
		lineStart = lineEnd
	}

	if sectionStart < len(text) {
		sections = append(sections, text[sectionStart:])
	}
	return sections
}

func isMarkdownHeading(line string) bool {
	hashCount := 0
	for hashCount < len(line) && line[hashCount] == '#' {
		hashCount++
	}
	if hashCount == 0 || hashCount > 6 || hashCount == len(line) {
		return false
	}

	runeValue, _ := utf8.DecodeRuneInString(line[hashCount:])
	return isECMAScriptWhitespace(runeValue)
}

func isECMAScriptWhitespace(value rune) bool {
	switch value {
	case '\u0009', '\u000a', '\u000b', '\u000c', '\u000d',
		' ', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000', '\ufeff':
		return true
	}
	return value >= '\u2000' && value <= '\u200a'
}
