// Package steering reads the required rule documents supplied by the caller.
package steering

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// GateValue is the typed gate state carried by a continuation token.
type GateValue uint8

const (
	GateInvalid GateValue = iota
	GateFalse
	GateTrue
	GateUnresolved
)

// UnitGateRhythm identifies when a unit gate is evaluated.
type UnitGateRhythm string

const (
	UnitGateRhythmPerStage UnitGateRhythm = "per-stage"
	UnitGateRhythmUnitEnd  UnitGateRhythm = "unit-end"
)

// OptionalNullableString distinguishes an absent field from a present null or string.
type OptionalNullableString struct {
	Present bool
	Value   *string
}

// ContinuationClaims are the values authenticated by a continuation token.
type ContinuationClaims struct {
	Version       int
	Stage         string
	Scope         string
	NextPart      int
	Bundle        string
	DirectiveHash string
	RouteHash     string
	StateAware    bool
	Unit          *string
	UnitKind      *string
	ForcePersona  bool
	Gate          GateValue
	NextStage     OptionalNullableString
	Single        bool
	UnitSpecific  bool
	Wave          bool
	SwarmSettled  *bool
	UnitGate      UnitGateRhythm
	StateHash     *string
}

// ErrInvalidContinuationToken indicates a malformed or unauthenticated token.
var ErrInvalidContinuationToken = errors.New("steering: invalid continuation token")

// EncodeContinuationToken authenticates continuation claims in a canonical
// base64url envelope.
func EncodeContinuationToken(key []byte, claims ContinuationClaims) (string, error) {
	if err := validateContinuationKey(key); err != nil {
		return "", err
	}
	if err := validateContinuationClaims(claims); err != nil {
		return "", err
	}

	payload := marshalContinuationClaims(claims)
	mac := continuationMAC(key, payload)
	var envelope strings.Builder
	envelope.WriteString("{\"p\":")
	envelope.WriteString(payload)
	envelope.WriteString(",\"m\":")
	appendJSONString(&envelope, base64.RawURLEncoding.EncodeToString(mac))
	envelope.WriteByte('}')
	return base64.RawURLEncoding.EncodeToString([]byte(envelope.String())), nil
}

// DecodeContinuationToken decodes and authenticates a continuation token.
func DecodeContinuationToken(key []byte, token string) (ContinuationClaims, error) {
	if err := validateContinuationKey(key); err != nil {
		return ContinuationClaims{}, err
	}
	envelopeBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ContinuationClaims{}, continuationTokenError("invalid envelope encoding")
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(envelopeBytes, &envelope); err != nil || envelope == nil {
		return ContinuationClaims{}, continuationTokenError("invalid envelope JSON")
	}
	if len(envelope) != 2 {
		return ContinuationClaims{}, continuationTokenError("envelope fields are invalid")
	}
	payloadBytes, ok := envelope["p"]
	if !ok {
		return ContinuationClaims{}, continuationTokenError("envelope payload is missing")
	}
	macBytes, ok := envelope["m"]
	if !ok {
		return ContinuationClaims{}, continuationTokenError("envelope MAC is missing")
	}
	for field := range envelope {
		if field != "p" && field != "m" {
			return ContinuationClaims{}, continuationTokenError("envelope field %q is unsupported", field)
		}
	}

	macText, err := decodeContinuationString(macBytes, "envelope MAC")
	if err != nil {
		return ContinuationClaims{}, err
	}
	encodedMAC, err := base64.RawURLEncoding.DecodeString(macText)
	if err != nil || len(encodedMAC) != sha256.Size {
		return ContinuationClaims{}, continuationTokenError("envelope MAC is invalid")
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload == nil {
		return ContinuationClaims{}, continuationTokenError("payload is not an object")
	}
	claims, canonicalPayload, err := decodeContinuationClaims(payload)
	if err != nil {
		return ContinuationClaims{}, err
	}
	if !hmac.Equal(encodedMAC, continuationMAC(key, canonicalPayload)) {
		return ContinuationClaims{}, continuationTokenError("MAC does not match payload")
	}
	return claims, nil
}

func validateContinuationKey(key []byte) error {
	if len(key) != sha256.Size {
		return continuationTokenError("key must be exactly %d bytes", sha256.Size)
	}
	return nil
}

func validateContinuationClaims(claims ContinuationClaims) error {
	if claims.Version != 1 {
		return continuationTokenError("version is invalid")
	}
	if claims.NextPart < 1 {
		return continuationTokenError("next part must be at least 1")
	}
	for name, value := range map[string]string{
		"stage":          claims.Stage,
		"scope":          claims.Scope,
		"bundle":         claims.Bundle,
		"directive hash": claims.DirectiveHash,
		"route hash":     claims.RouteHash,
	} {
		if !utf8.ValidString(value) {
			return continuationTokenError("%s is not valid UTF-8", name)
		}
	}
	for name, value := range map[string]*string{
		"unit":       claims.Unit,
		"unit kind":  claims.UnitKind,
		"state hash": claims.StateHash,
	} {
		if value != nil && !utf8.ValidString(*value) {
			return continuationTokenError("%s is not valid UTF-8", name)
		}
	}
	if claims.NextStage.Value != nil && !utf8.ValidString(*claims.NextStage.Value) {
		return continuationTokenError("next stage is not valid UTF-8")
	}
	if !claims.NextStage.Present && claims.NextStage.Value != nil {
		return continuationTokenError("next stage value is present without the field")
	}
	switch claims.Gate {
	case GateFalse, GateTrue, GateUnresolved:
	default:
		return continuationTokenError("gate is invalid")
	}
	switch claims.UnitGate {
	case "":
	case UnitGateRhythmPerStage, UnitGateRhythmUnitEnd:
	default:
		return continuationTokenError("unit gate rhythm is invalid")
	}
	return nil
}

func marshalContinuationClaims(claims ContinuationClaims) string {
	var builder strings.Builder
	builder.WriteString("{\"v\":")
	builder.WriteString(strconv.Itoa(claims.Version))
	builder.WriteString(",\"s\":")
	appendJSONString(&builder, claims.Stage)
	builder.WriteString(",\"c\":")
	appendJSONString(&builder, claims.Scope)
	builder.WriteString(",\"i\":")
	builder.WriteString(strconv.Itoa(claims.NextPart))
	builder.WriteString(",\"b\":")
	appendJSONString(&builder, claims.Bundle)
	builder.WriteString(",\"d\":")
	appendJSONString(&builder, claims.DirectiveHash)
	builder.WriteString(",\"r\":")
	appendJSONString(&builder, claims.RouteHash)
	builder.WriteString(",\"a\":")
	builder.WriteString(strconv.FormatBool(claims.StateAware))
	builder.WriteString(",\"u\":")
	appendNullableString(&builder, claims.Unit)
	builder.WriteString(",\"k\":")
	appendNullableString(&builder, claims.UnitKind)
	builder.WriteString(",\"f\":")
	builder.WriteString(strconv.FormatBool(claims.ForcePersona))
	builder.WriteString(",\"g\":")
	appendGateValue(&builder, claims.Gate)
	if claims.NextStage.Present {
		builder.WriteString(",\"n\":")
		appendNullableString(&builder, claims.NextStage.Value)
	}
	builder.WriteString(",\"x\":")
	builder.WriteString(strconv.FormatBool(claims.Single))
	builder.WriteString(",\"p\":")
	builder.WriteString(strconv.FormatBool(claims.UnitSpecific))
	builder.WriteString(",\"w\":")
	builder.WriteString(strconv.FormatBool(claims.Wave))
	if claims.SwarmSettled != nil {
		builder.WriteString(",\"z\":")
		builder.WriteString(strconv.FormatBool(*claims.SwarmSettled))
	}
	if claims.UnitGate != "" {
		builder.WriteString(",\"q\":")
		appendJSONString(&builder, string(claims.UnitGate))
	}
	builder.WriteString(",\"h\":")
	appendNullableString(&builder, claims.StateHash)
	builder.WriteByte('}')
	return builder.String()
}

func appendNullableString(builder *strings.Builder, value *string) {
	if value == nil {
		builder.WriteString("null")
		return
	}
	appendJSONString(builder, *value)
}

func appendGateValue(builder *strings.Builder, gate GateValue) {
	switch gate {
	case GateFalse:
		builder.WriteString("false")
	case GateTrue:
		builder.WriteString("true")
	case GateUnresolved:
		appendJSONString(builder, "unresolved")
	}
}

func continuationMAC(key []byte, payload string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func decodeContinuationClaims(payload map[string]json.RawMessage) (ContinuationClaims, string, error) {
	const requiredFields = "v s c i b d r a u k f g x p w h"
	for _, field := range strings.Fields(requiredFields) {
		if _, ok := payload[field]; !ok {
			return ContinuationClaims{}, "", continuationTokenError("payload field %q is missing", field)
		}
	}
	for field := range payload {
		switch field {
		case "v", "s", "c", "i", "b", "d", "r", "a", "u", "k", "f", "g", "n", "x", "p", "w", "z", "q", "h":
		default:
			return ContinuationClaims{}, "", continuationTokenError("payload field %q is unsupported", field)
		}
	}

	var claims ContinuationClaims
	var err error
	if claims.Version, err = decodeContinuationInt(payload["v"], "version"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Stage, err = decodeContinuationString(payload["s"], "stage"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Scope, err = decodeContinuationString(payload["c"], "scope"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.NextPart, err = decodeContinuationInt(payload["i"], "next part"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Bundle, err = decodeContinuationString(payload["b"], "bundle"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.DirectiveHash, err = decodeContinuationString(payload["d"], "directive hash"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.RouteHash, err = decodeContinuationString(payload["r"], "route hash"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.StateAware, err = decodeContinuationBool(payload["a"], "state aware"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Unit, err = decodeContinuationNullableString(payload["u"], "unit"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.UnitKind, err = decodeContinuationNullableString(payload["k"], "unit kind"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.ForcePersona, err = decodeContinuationBool(payload["f"], "force persona"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Gate, err = decodeContinuationGate(payload["g"]); err != nil {
		return ContinuationClaims{}, "", err
	}
	if raw, ok := payload["n"]; ok {
		claims.NextStage.Present = true
		if claims.NextStage.Value, err = decodeContinuationNullableString(raw, "next stage"); err != nil {
			return ContinuationClaims{}, "", err
		}
	}
	if claims.Single, err = decodeContinuationBool(payload["x"], "single"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.UnitSpecific, err = decodeContinuationBool(payload["p"], "unit specific"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if claims.Wave, err = decodeContinuationBool(payload["w"], "wave"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if raw, ok := payload["z"]; ok {
		var settled bool
		if settled, err = decodeContinuationBool(raw, "swarm settled"); err != nil {
			return ContinuationClaims{}, "", err
		}
		claims.SwarmSettled = &settled
	}
	if raw, ok := payload["q"]; ok {
		claims.UnitGate, err = decodeContinuationRhythm(raw)
		if err != nil {
			return ContinuationClaims{}, "", err
		}
	}
	if claims.StateHash, err = decodeContinuationNullableString(payload["h"], "state hash"); err != nil {
		return ContinuationClaims{}, "", err
	}
	if err := validateContinuationClaims(claims); err != nil {
		return ContinuationClaims{}, "", err
	}
	return claims, marshalContinuationClaims(claims), nil
}

func decodeContinuationInt(raw json.RawMessage, field string) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, continuationTokenError("%s is invalid", field)
	}
	return value, nil
}

func decodeContinuationBool(raw json.RawMessage, field string) (bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, continuationTokenError("%s is invalid", field)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, continuationTokenError("%s is invalid", field)
	}
	return value, nil
}

func decodeContinuationString(raw json.RawMessage, field string) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", continuationTokenError("%s is invalid", field)
	}
	if !utf8.Valid(raw) {
		return "", continuationTokenError("%s is not valid UTF-8", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !utf8.ValidString(value) {
		return "", continuationTokenError("%s is invalid", field)
	}
	return value, nil
}

func decodeContinuationNullableString(raw json.RawMessage, field string) (*string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := decodeContinuationString(raw, field)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func decodeContinuationGate(raw json.RawMessage) (GateValue, error) {
	switch string(bytes.TrimSpace(raw)) {
	case "false":
		return GateFalse, nil
	case "true":
		return GateTrue, nil
	}
	value, err := decodeContinuationString(raw, "gate")
	if err != nil || value != "unresolved" {
		return GateInvalid, continuationTokenError("gate is invalid")
	}
	return GateUnresolved, nil
}

func decodeContinuationRhythm(raw json.RawMessage) (UnitGateRhythm, error) {
	value, err := decodeContinuationString(raw, "unit gate rhythm")
	if err != nil {
		return "", err
	}
	switch rhythm := UnitGateRhythm(value); rhythm {
	case UnitGateRhythmPerStage, UnitGateRhythmUnitEnd:
		return rhythm, nil
	default:
		return "", continuationTokenError("unit gate rhythm is invalid")
	}
}

func continuationTokenError(format string, args ...any) error {
	return fmt.Errorf("continuation token: %w: %s", ErrInvalidContinuationToken, fmt.Sprintf(format, args...))
}
