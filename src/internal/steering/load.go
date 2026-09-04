package steering

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// LoadDirective describes one load-steering JSON directive.
type LoadDirective struct {
	Stage         string
	Bundle        string
	Part          int
	Parts         int
	RulesContent  []RuleContent
	ContinueToken string
}

const maxLoadDirectiveBytes = 28 * 1024

// MarshalLoad marshals one load-steering JSON directive.
func MarshalLoad(input LoadDirective) ([]byte, error) {
	if err := validateLoadDirective(input); err != nil {
		return nil, err
	}

	var builder strings.Builder
	builder.WriteString(`{"kind":"load-steering","stage":`)
	appendJSONString(&builder, input.Stage)
	builder.WriteString(`,"bundle":`)
	appendJSONString(&builder, input.Bundle)
	builder.WriteString(`,"part":`)
	builder.WriteString(strconv.Itoa(input.Part))
	builder.WriteString(`,"parts":`)
	builder.WriteString(strconv.Itoa(input.Parts))
	builder.WriteString(`,"rules_content":`)
	builder.WriteString(marshalDigestRules(input.RulesContent))
	builder.WriteString(`,"continue_token":`)
	appendJSONString(&builder, input.ContinueToken)
	builder.WriteByte('}')

	result := []byte(builder.String())
	if len(result) > maxLoadDirectiveBytes {
		return nil, fmt.Errorf("marshal load: directive exceeds %d bytes: got %d", maxLoadDirectiveBytes, len(result))
	}
	return result, nil
}

func validateLoadDirective(input LoadDirective) error {
	if input.Part < 1 {
		return fmt.Errorf("marshal load: part must be at least 1: got %d", input.Part)
	}
	if input.Parts < 1 {
		return fmt.Errorf("marshal load: parts must be at least 1: got %d", input.Parts)
	}
	if input.Part > input.Parts {
		return fmt.Errorf("marshal load: part %d exceeds parts %d", input.Part, input.Parts)
	}
	if !utf8.ValidString(input.Stage) {
		return fmt.Errorf("marshal load: invalid UTF-8 in stage")
	}
	if !utf8.ValidString(input.Bundle) {
		return fmt.Errorf("marshal load: invalid UTF-8 in bundle")
	}
	if !utf8.ValidString(input.ContinueToken) {
		return fmt.Errorf("marshal load: invalid UTF-8 in continue_token")
	}
	for index, rule := range input.RulesContent {
		if !utf8.ValidString(rule.Path) {
			return fmt.Errorf("marshal load: invalid UTF-8 in rules_content[%d].path", index)
		}
		if !utf8.ValidString(rule.Text) {
			return fmt.Errorf("marshal load: invalid UTF-8 in rules_content[%d].text", index)
		}
	}
	return nil
}
