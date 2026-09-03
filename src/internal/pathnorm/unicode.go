// Package pathnorm contains platform identity normalization shared by the
// workspace and record locks.
package pathnorm

import (
	"strings"
	"unicode"
)

// NormalizeForPlatform applies the fixed platform case rule used in an
// identity key.  Windows is deliberately pinned to the Bun/ECMAScript
// Unicode-15 behavior from the repository's AI-DLC snapshot.
func NormalizeForPlatform(value, platform string) string {
	if platform == "windows" {
		return ECMAScriptDefaultLower(value)
	}
	return value
}

// ECMAScriptDefaultLower implements the fixed Bun 1.3.14 / ICU 73.2,
// Unicode-15 default-lowercase behavior.  It is kept independent of the
// host's Unicode tables so lock identities remain stable across platforms.
func ECMAScriptDefaultLower(value string) string {
	runes := []rune(value)
	var lowered strings.Builder
	lowered.Grow(len(value))
	for index, char := range runes {
		switch char {
		case '\u0130':
			lowered.WriteString("i\u0307")
		case '\u03a3':
			if isFinalSigma(runes, index) {
				lowered.WriteRune('\u03c2')
			} else {
				lowered.WriteRune('\u03c3')
			}
		default:
			lowered.WriteRune(unicode15Lower(char))
		}
	}
	return lowered.String()
}

// IsCased reports the fixed Unicode-15 cased-property classification.
func IsCased(char rune) bool {
	if char == '\u0295' {
		return true
	}
	if isUnicode15Uncased(char) {
		return false
	}
	return unicode.In(
		char,
		unicode.Lu,
		unicode.Ll,
		unicode.Lt,
		unicode.Other_Uppercase,
		unicode.Other_Lowercase,
	)
}

// IsCaseIgnorable reports the fixed Unicode-15 case-ignorable property.
func IsCaseIgnorable(char rune) bool {
	if char == '\U0001171e' {
		return true
	}
	if isUnicode15NotCaseIgnorable(char) {
		return false
	}
	if unicode.In(char, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk) {
		return true
	}
	switch char {
	case '\u0027', '\u002e', '\u003a', '\u00b7', '\u0387', '\u055f', '\u05f4',
		'\u2018', '\u2019', '\u2024', '\u2027', '\ufe13', '\ufe52', '\ufe55',
		'\uff07', '\uff0e', '\uff1a':
		return true
	default:
		return false
	}
}

func unicode15Lower(char rune) rune {
	switch char {
	case '\U00001c89', '\U0000a7cb', '\U0000a7cc', '\U0000a7ce',
		'\U0000a7d2', '\U0000a7d4', '\U0000a7da', '\U0000a7dc':
		return char
	}
	if runeInRange(char, '\U00010d50', '\U00010d65') ||
		runeInRange(char, '\U00016ea0', '\U00016eb8') {
		return char
	}
	return unicode.ToLower(char)
}

func isFinalSigma(runes []rune, index int) bool {
	if !nearestCasedBefore(runes, index) {
		return false
	}
	for following := index + 1; following < len(runes); following++ {
		if IsCaseIgnorable(runes[following]) {
			continue
		}
		return !IsCased(runes[following])
	}
	return true
}

func nearestCasedBefore(runes []rune, index int) bool {
	for preceding := index - 1; preceding >= 0; preceding-- {
		if IsCaseIgnorable(runes[preceding]) {
			continue
		}
		return IsCased(runes[preceding])
	}
	return false
}

func isUnicode15Uncased(char rune) bool {
	switch char {
	case '\U0000a7d2', '\U0000a7d4', '\U0000a7f1':
		return true
	}
	return runeInRange(char, '\U00001c89', '\U00001c8a') ||
		runeInRange(char, '\U0000a7cb', '\U0000a7cf') ||
		runeInRange(char, '\U0000a7da', '\U0000a7dc') ||
		runeInRange(char, '\U00010d50', '\U00010d65') ||
		runeInRange(char, '\U00010d70', '\U00010d85') ||
		runeInRange(char, '\U00016ea0', '\U00016eb8') ||
		runeInRange(char, '\U00016ebb', '\U00016ed3')
}

func isUnicode15NotCaseIgnorable(char rune) bool {
	switch char {
	case '\u0897', '\U0000a7f1', '\U00010d4e', '\U00010d6f', '\U00010ec5',
		'\U000113ce', '\U000113d0', '\U000113d2', '\U00011b60', '\U00011b66',
		'\U00011dd9', '\U00011f5a', '\U0001e6e3', '\U0001e6e6', '\U0001e6f5',
		'\U0001e6ff':
		return true
	}
	return runeInRange(char, '\U00001acf', '\U00001add') ||
		runeInRange(char, '\U00001ae0', '\U00001aeb') ||
		runeInRange(char, '\U00010d69', '\U00010d6d') ||
		runeInRange(char, '\U00010efa', '\U00010efc') ||
		runeInRange(char, '\U000113bb', '\U000113c0') ||
		runeInRange(char, '\U000113e1', '\U000113e2') ||
		runeInRange(char, '\U00011b62', '\U00011b64') ||
		runeInRange(char, '\U0001611e', '\U00016129') ||
		runeInRange(char, '\U0001612d', '\U0001612f') ||
		runeInRange(char, '\U00016d40', '\U00016d42') ||
		runeInRange(char, '\U00016d6b', '\U00016d6c') ||
		runeInRange(char, '\U00016ff2', '\U00016ff3') ||
		runeInRange(char, '\U0001e5ee', '\U0001e5ef') ||
		runeInRange(char, '\U0001e6ee', '\U0001e6ef')
}

func runeInRange(char, first, last rune) bool {
	return first <= char && char <= last
}
