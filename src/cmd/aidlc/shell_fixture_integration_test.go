//go:build integration

package main

import "strings"

func flowShellWords(command string) ([]string, bool) {
	var words []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	started := false
	for _, r := range command {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '$' || r == '`' || r == '\n' || r == '\r' {
			return nil, false
		}
		if r == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if strings.ContainsRune(";|&<>()", r) {
			return nil, false
		}
		if r == ' ' || r == '\t' {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 || escaped {
		return nil, false
	}
	if started {
		words = append(words, word.String())
	}
	return words, true
}
