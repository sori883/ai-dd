package steering

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// BundleDigest returns the digest of the ordered rule bundle.
func BundleDigest(content []RuleContent) (string, error) {
	for index, rule := range content {
		if !utf8.ValidString(rule.Path) {
			return "", fmt.Errorf("bundle digest: invalid UTF-8 in rule %d path", index)
		}
		if !utf8.ValidString(rule.Text) {
			return "", fmt.Errorf("bundle digest: invalid UTF-8 in rule %d text", index)
		}
	}

	digest := sha256.Sum256([]byte(marshalDigestRules(content)))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func marshalDigestRules(content []RuleContent) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for index, rule := range content {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"path":`)
		appendJSONString(&builder, rule.Path)
		builder.WriteString(`,"text":`)
		appendJSONString(&builder, rule.Text)
		builder.WriteByte('}')
	}
	builder.WriteByte(']')
	return builder.String()
}

func appendJSONString(builder *strings.Builder, value string) {
	const hexDigits = "0123456789abcdef"

	builder.WriteByte('"')
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if value[index] < 0x20 {
				builder.WriteString(`\u00`)
				builder.WriteByte(hexDigits[value[index]>>4])
				builder.WriteByte(hexDigits[value[index]&0x0f])
				continue
			}
			builder.WriteByte(value[index])
		}
	}
	builder.WriteByte('"')
}
