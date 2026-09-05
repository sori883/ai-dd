package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
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

type canonicalJSONValue struct {
	kind   canonicalJSONKind
	bool   bool
	str    string
	number string
	array  []canonicalJSONValue
	object []canonicalJSONObjectField
}

type canonicalJSONObjectField struct {
	name  string
	value canonicalJSONValue
}

func canonicalizeJSON(data []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeCanonicalJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}

	return appendCanonicalJSON(nil, value)
}

func decodeCanonicalJSONValue(decoder *json.Decoder) (canonicalJSONValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return canonicalJSONValue{}, err
	}

	switch value := token.(type) {
	case nil:
		return canonicalJSONValue{kind: canonicalJSONNull}, nil
	case bool:
		return canonicalJSONValue{kind: canonicalJSONBoolean, bool: value}, nil
	case string:
		return canonicalJSONValue{kind: canonicalJSONString, str: value}, nil
	case json.Number:
		number, err := canonicalizeJSONNumber(value.String())
		if err != nil {
			return canonicalJSONValue{}, err
		}
		return canonicalJSONValue{kind: canonicalJSONNumber, number: number}, nil
	case json.Delim:
		switch value {
		case '{':
			return decodeCanonicalJSONObject(decoder)
		case '[':
			return decodeCanonicalJSONArray(decoder)
		default:
			return canonicalJSONValue{}, fmt.Errorf("unexpected JSON delimiter %q", value)
		}
	default:
		return canonicalJSONValue{}, fmt.Errorf("unexpected JSON token %T", token)
	}
}

func decodeCanonicalJSONObject(decoder *json.Decoder) (canonicalJSONValue, error) {
	fields := make([]canonicalJSONObjectField, 0)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return canonicalJSONValue{}, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return canonicalJSONValue{}, fmt.Errorf("object key is %T, want string", keyToken)
		}

		value, err := decodeCanonicalJSONValue(decoder)
		if err != nil {
			return canonicalJSONValue{}, err
		}
		updated := false
		for index := range fields {
			if fields[index].name == key {
				fields[index].value = value
				updated = true
				break
			}
		}
		if !updated {
			fields = append(fields, canonicalJSONObjectField{name: key, value: value})
		}
	}

	closing, err := decoder.Token()
	if err != nil {
		return canonicalJSONValue{}, err
	}
	if closing != json.Delim('}') {
		return canonicalJSONValue{}, fmt.Errorf("object ended with %v, want }", closing)
	}
	return canonicalJSONValue{kind: canonicalJSONObject, object: fields}, nil
}

func decodeCanonicalJSONArray(decoder *json.Decoder) (canonicalJSONValue, error) {
	values := make([]canonicalJSONValue, 0)
	for decoder.More() {
		value, err := decodeCanonicalJSONValue(decoder)
		if err != nil {
			return canonicalJSONValue{}, err
		}
		values = append(values, value)
	}

	closing, err := decoder.Token()
	if err != nil {
		return canonicalJSONValue{}, err
	}
	if closing != json.Delim(']') {
		return canonicalJSONValue{}, fmt.Errorf("array ended with %v, want ]", closing)
	}
	return canonicalJSONValue{kind: canonicalJSONArray, array: values}, nil
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

func appendCanonicalJSONString(dst []byte, value string) []byte {
	dst = append(dst, '"')
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch character {
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
			if character < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hexDigits[character>>4], hexDigits[character&0x0f])
			} else {
				dst = append(dst, character)
			}
		}
	}
	return append(dst, '"')
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
