package graph

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type canonicalJSONKind uint8

const (
	canonicalJSONInvalid canonicalJSONKind = iota
	canonicalJSONNull
	canonicalJSONBoolean
	canonicalJSONString
	canonicalJSONNumber
	canonicalJSONArray
	canonicalJSONObject
)

type canonicalJSONStringValue []uint16

type canonicalJSONValue struct {
	kind   canonicalJSONKind
	bool   bool
	str    canonicalJSONStringValue
	number string
	array  []canonicalJSONValue
	object []canonicalJSONObjectField
}

type canonicalJSONObjectField struct {
	name  canonicalJSONStringValue
	value canonicalJSONValue
}

type canonicalJSONParser struct {
	data   []byte
	offset int
}

func canonicalizeJSON(data []byte) ([]byte, error) {
	parser := canonicalJSONParser{data: data}
	value, err := parser.parseValue()
	if err != nil {
		return nil, err
	}
	parser.skipWhitespace()
	if parser.offset != len(parser.data) {
		return nil, parser.errorf("multiple JSON values")
	}

	return appendCanonicalJSON(nil, value)
}

func (p *canonicalJSONParser) parseValue() (canonicalJSONValue, error) {
	p.skipWhitespace()
	if p.offset == len(p.data) {
		return canonicalJSONValue{}, p.errorf("unexpected end of JSON input")
	}

	switch p.data[p.offset] {
	case 'n':
		if err := p.consumeLiteral("null"); err != nil {
			return canonicalJSONValue{}, err
		}
		return canonicalJSONValue{kind: canonicalJSONNull}, nil
	case 't':
		if err := p.consumeLiteral("true"); err != nil {
			return canonicalJSONValue{}, err
		}
		return canonicalJSONValue{kind: canonicalJSONBoolean, bool: true}, nil
	case 'f':
		if err := p.consumeLiteral("false"); err != nil {
			return canonicalJSONValue{}, err
		}
		return canonicalJSONValue{kind: canonicalJSONBoolean}, nil
	case '"':
		value, err := p.parseString()
		if err != nil {
			return canonicalJSONValue{}, err
		}
		return canonicalJSONValue{kind: canonicalJSONString, str: value}, nil
	case '[':
		return p.parseArray()
	case '{':
		return p.parseObject()
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumber()
	default:
		return canonicalJSONValue{}, p.errorf("unexpected character %q", p.data[p.offset])
	}
}

func (p *canonicalJSONParser) parseObject() (canonicalJSONValue, error) {
	p.offset++
	p.skipWhitespace()
	if p.consumeByte('}') {
		return canonicalJSONValue{kind: canonicalJSONObject}, nil
	}

	fields := make([]canonicalJSONObjectField, 0)
	for {
		p.skipWhitespace()
		if p.offset == len(p.data) || p.data[p.offset] != '"' {
			return canonicalJSONValue{}, p.errorf("object key must be a string")
		}
		name, err := p.parseString()
		if err != nil {
			return canonicalJSONValue{}, err
		}
		p.skipWhitespace()
		if !p.consumeByte(':') {
			return canonicalJSONValue{}, p.errorf("expected colon after object key")
		}
		value, err := p.parseValue()
		if err != nil {
			return canonicalJSONValue{}, err
		}

		updated := false
		for index := range fields {
			if slices.Equal(fields[index].name, name) {
				fields[index].value = value
				updated = true
				break
			}
		}
		if !updated {
			fields = append(fields, canonicalJSONObjectField{name: name, value: value})
		}

		p.skipWhitespace()
		if p.consumeByte('}') {
			break
		}
		if !p.consumeByte(',') {
			return canonicalJSONValue{}, p.errorf("expected comma or object end")
		}
	}

	return canonicalJSONValue{kind: canonicalJSONObject, object: orderCanonicalJSONObjectFields(fields)}, nil
}

func (p *canonicalJSONParser) parseArray() (canonicalJSONValue, error) {
	p.offset++
	p.skipWhitespace()
	if p.consumeByte(']') {
		return canonicalJSONValue{kind: canonicalJSONArray}, nil
	}

	values := make([]canonicalJSONValue, 0)
	for {
		value, err := p.parseValue()
		if err != nil {
			return canonicalJSONValue{}, err
		}
		values = append(values, value)

		p.skipWhitespace()
		if p.consumeByte(']') {
			break
		}
		if !p.consumeByte(',') {
			return canonicalJSONValue{}, p.errorf("expected comma or array end")
		}
	}

	return canonicalJSONValue{kind: canonicalJSONArray, array: values}, nil
}

func (p *canonicalJSONParser) parseString() (canonicalJSONStringValue, error) {
	p.offset++
	value := make(canonicalJSONStringValue, 0)
	for p.offset < len(p.data) {
		character := p.data[p.offset]
		switch character {
		case '"':
			p.offset++
			return value, nil
		case '\\':
			p.offset++
			if p.offset == len(p.data) {
				return nil, p.errorf("unterminated string escape")
			}
			escaped := p.data[p.offset]
			p.offset++
			switch escaped {
			case '"', '\\', '/':
				value = append(value, uint16(escaped))
			case 'b':
				value = append(value, '\b')
			case 'f':
				value = append(value, '\f')
			case 'n':
				value = append(value, '\n')
			case 'r':
				value = append(value, '\r')
			case 't':
				value = append(value, '\t')
			case 'u':
				unit, err := p.parseHexCodeUnit()
				if err != nil {
					return nil, err
				}
				value = append(value, unit)
			default:
				return nil, p.errorf("invalid string escape %q", escaped)
			}
		default:
			if character < 0x20 {
				return nil, p.errorf("unescaped control character in string")
			}
			r, size := utf8.DecodeRune(p.data[p.offset:])
			p.offset += size
			if r <= 0xffff {
				value = append(value, uint16(r))
				continue
			}
			high, low := utf16.EncodeRune(r)
			value = append(value, uint16(high), uint16(low))
		}
	}

	return nil, p.errorf("unterminated string")
}

func (p *canonicalJSONParser) parseHexCodeUnit() (uint16, error) {
	if len(p.data)-p.offset < 4 {
		return 0, p.errorf("short Unicode escape")
	}
	var value uint16
	for range 4 {
		digit, ok := hexadecimalValue(p.data[p.offset])
		if !ok {
			return 0, p.errorf("invalid Unicode escape")
		}
		value = value<<4 | uint16(digit)
		p.offset++
	}
	return value, nil
}

func (p *canonicalJSONParser) parseNumber() (canonicalJSONValue, error) {
	start := p.offset
	if p.consumeByte('-') && p.offset == len(p.data) {
		return canonicalJSONValue{}, p.errorf("incomplete JSON number")
	}

	if p.consumeByte('0') {
		if p.offset < len(p.data) && isDecimalDigit(p.data[p.offset]) {
			return canonicalJSONValue{}, p.errorf("leading zero in JSON number")
		}
	} else {
		if p.offset == len(p.data) || p.data[p.offset] < '1' || p.data[p.offset] > '9' {
			return canonicalJSONValue{}, p.errorf("invalid JSON number")
		}
		for p.offset < len(p.data) && isDecimalDigit(p.data[p.offset]) {
			p.offset++
		}
	}

	if p.consumeByte('.') {
		fractionStart := p.offset
		for p.offset < len(p.data) && isDecimalDigit(p.data[p.offset]) {
			p.offset++
		}
		if p.offset == fractionStart {
			return canonicalJSONValue{}, p.errorf("JSON number fraction has no digits")
		}
	}
	if p.offset < len(p.data) && (p.data[p.offset] == 'e' || p.data[p.offset] == 'E') {
		p.offset++
		if p.offset < len(p.data) && (p.data[p.offset] == '+' || p.data[p.offset] == '-') {
			p.offset++
		}
		exponentStart := p.offset
		for p.offset < len(p.data) && isDecimalDigit(p.data[p.offset]) {
			p.offset++
		}
		if p.offset == exponentStart {
			return canonicalJSONValue{}, p.errorf("JSON number exponent has no digits")
		}
	}

	number, err := canonicalizeJSONNumber(string(p.data[start:p.offset]))
	if err != nil {
		return canonicalJSONValue{}, p.errorf("invalid JSON number: %v", err)
	}
	return canonicalJSONValue{kind: canonicalJSONNumber, number: number}, nil
}

func (p *canonicalJSONParser) consumeLiteral(literal string) error {
	if len(p.data)-p.offset < len(literal) || string(p.data[p.offset:p.offset+len(literal)]) != literal {
		return p.errorf("invalid JSON literal")
	}
	p.offset += len(literal)
	return nil
}

func (p *canonicalJSONParser) consumeByte(want byte) bool {
	if p.offset == len(p.data) || p.data[p.offset] != want {
		return false
	}
	p.offset++
	return true
}

func (p *canonicalJSONParser) skipWhitespace() {
	for p.offset < len(p.data) {
		switch p.data[p.offset] {
		case ' ', '\t', '\n', '\r':
			p.offset++
		default:
			return
		}
	}
}

func (p *canonicalJSONParser) errorf(format string, args ...any) error {
	return fmt.Errorf("canonical JSON at byte %d: %s", p.offset, fmt.Sprintf(format, args...))
}

func orderCanonicalJSONObjectFields(fields []canonicalJSONObjectField) []canonicalJSONObjectField {
	type indexedField struct {
		index uint32
		field canonicalJSONObjectField
	}
	indexed := make([]indexedField, 0, len(fields))
	ordinary := make([]canonicalJSONObjectField, 0, len(fields))
	for _, field := range fields {
		index, ok := canonicalJSONArrayIndex(field.name)
		if ok {
			indexed = append(indexed, indexedField{index: index, field: field})
			continue
		}
		ordinary = append(ordinary, field)
	}
	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].index < indexed[j].index
	})

	ordered := make([]canonicalJSONObjectField, 0, len(fields))
	for _, entry := range indexed {
		ordered = append(ordered, entry.field)
	}
	return append(ordered, ordinary...)
}

func canonicalJSONArrayIndex(value canonicalJSONStringValue) (uint32, bool) {
	if len(value) == 0 || len(value) > 1 && value[0] == '0' {
		return 0, false
	}
	const maximumArrayIndex uint64 = 1<<32 - 2
	var number uint64
	for _, unit := range value {
		if unit < '0' || unit > '9' {
			return 0, false
		}
		digit := uint64(unit - '0')
		if number > (maximumArrayIndex-digit)/10 {
			return 0, false
		}
		number = number*10 + digit
	}
	return uint32(number), true
}

func appendCanonicalJSON(dst []byte, value canonicalJSONValue) ([]byte, error) {
	switch value.kind {
	case canonicalJSONNull:
		return append(dst, "null"...), nil
	case canonicalJSONBoolean:
		if value.bool {
			return append(dst, "true"...), nil
		}
		return append(dst, "false"...), nil
	case canonicalJSONString:
		return appendCanonicalJSONString(dst, value.str), nil
	case canonicalJSONNumber:
		return append(dst, value.number...), nil
	case canonicalJSONArray:
		dst = append(dst, '[')
		for index, item := range value.array {
			if index > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = appendCanonicalJSON(dst, item)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case canonicalJSONObject:
		dst = append(dst, '{')
		for index, field := range value.object {
			if index > 0 {
				dst = append(dst, ',')
			}
			dst = appendCanonicalJSONString(dst, field.name)
			dst = append(dst, ':')
			var err error
			dst, err = appendCanonicalJSON(dst, field.value)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, '}'), nil
	default:
		return nil, fmt.Errorf("invalid canonical JSON value kind %d", value.kind)
	}
}

func appendCanonicalJSONString(dst []byte, value canonicalJSONStringValue) []byte {
	dst = append(dst, '"')
	for index := 0; index < len(value); index++ {
		unit := value[index]
		switch unit {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			switch {
			case unit < 0x20:
				dst = appendUnicodeEscape(dst, unit)
			case utf16.IsSurrogate(rune(unit)):
				if unit <= 0xdbff && index+1 < len(value) && value[index+1] >= 0xdc00 && value[index+1] <= 0xdfff {
					dst = utf8.AppendRune(dst, utf16.DecodeRune(rune(unit), rune(value[index+1])))
					index++
				} else {
					dst = appendUnicodeEscape(dst, unit)
				}
			default:
				dst = utf8.AppendRune(dst, rune(unit))
			}
		}
	}
	return append(dst, '"')
}

func appendUnicodeEscape(dst []byte, value uint16) []byte {
	return append(dst,
		'\\', 'u',
		hexDigits[value>>12],
		hexDigits[value>>8&0x0f],
		hexDigits[value>>4&0x0f],
		hexDigits[value&0x0f],
	)
}

func hexadecimalValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func isDecimalDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

const hexDigits = "0123456789abcdef"

func canonicalizeJSONNumber(raw string) (string, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if value == 0 {
		return "0", nil
	}
	if math.IsInf(value, 0) {
		return "null", nil
	}
	if err != nil {
		return "", err
	}

	absolute := math.Abs(value)
	if absolute >= 1e-6 && absolute < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	}

	formatted := strconv.FormatFloat(value, 'e', -1, 64)
	mantissa, exponent, ok := strings.Cut(formatted, "e")
	if !ok {
		return formatted, nil
	}
	exponentValue, err := strconv.Atoi(exponent)
	if err != nil {
		return "", err
	}
	if exponentValue >= 0 {
		return mantissa + "e+" + strconv.Itoa(exponentValue), nil
	}
	return mantissa + "e" + strconv.Itoa(exponentValue), nil
}
