package steering

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestContinuationTokenRoundTripMatchesFixedWire(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	stateHash := "state-hash"
	swarmSettled := false
	common := ContinuationClaims{
		Version:       1,
		Stage:         "段階🚀\"\\\x00\n\u2028\u2029",
		Scope:         "スコープ",
		NextPart:      2,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    true,
		Unit:          nil,
		UnitKind:      nil,
		ForcePersona:  false,
		Gate:          GateUnresolved,
		Single:        false,
		UnitSpecific:  true,
		Wave:          true,
		StateHash:     &stateHash,
	}
	presentClaims := common
	presentClaims.NextStage = OptionalNullableString{Present: true}
	presentClaims.SwarmSettled = &swarmSettled
	wantAbsentSwarmSettled := false
	wantAbsentClaims := common
	wantAbsentClaims.SwarmSettled = &wantAbsentSwarmSettled

	tests := []struct {
		name       string
		claims     ContinuationClaims
		want       string
		wantClaims ContinuationClaims
	}{
		{
			name: "optional fields present",
			claims: func() ContinuationClaims {
				claims := presentClaims
				claims.UnitGate = UnitGateRhythmUnitEnd
				return claims
			}(),
			wantClaims: func() ContinuationClaims {
				claims := presentClaims
				claims.UnitGate = UnitGateRhythmUnitEnd
				return claims
			}(),
			want: "eyJwIjp7InYiOjEsInMiOiLmrrXpmo7wn5qAXCJcXFx1MDAwMFxu4oCo4oCpIiwiYyI6IuOCueOCs-ODvOODlyIsImkiOjIsImIiOiJidW5kbGUiLCJkIjoiZGlyZWN0aXZlLWhhc2giLCJyIjoicm91dGUtaGFzaCIsImEiOnRydWUsInUiOm51bGwsImsiOm51bGwsImYiOmZhbHNlLCJnIjoidW5yZXNvbHZlZCIsIm4iOm51bGwsIngiOmZhbHNlLCJwIjp0cnVlLCJ3Ijp0cnVlLCJ6IjpmYWxzZSwicSI6InVuaXQtZW5kIiwiaCI6InN0YXRlLWhhc2gifSwibSI6Iko4UjVvNWVJNnNSY25mcWNmWlVvVUxib1hUM3lVTlBaRjdTYk9PM2Ixd0kifQ",
		},
		{
			name:       "optional fields absent",
			claims:     common,
			wantClaims: wantAbsentClaims,
			want:       "eyJwIjp7InYiOjEsInMiOiLmrrXpmo7wn5qAXCJcXFx1MDAwMFxu4oCo4oCpIiwiYyI6IuOCueOCs-ODvOODlyIsImkiOjIsImIiOiJidW5kbGUiLCJkIjoiZGlyZWN0aXZlLWhhc2giLCJyIjoicm91dGUtaGFzaCIsImEiOnRydWUsInUiOm51bGwsImsiOm51bGwsImYiOmZhbHNlLCJnIjoidW5yZXNvbHZlZCIsIngiOmZhbHNlLCJwIjp0cnVlLCJ3Ijp0cnVlLCJ6IjpmYWxzZSwiaCI6InN0YXRlLWhhc2gifSwibSI6ImJDWTZ5TFNYb3VBQS1VcGNIQW1Bdm05eUJYWHo4b1JDZVJKUE9xNk80RUUifQ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotToken, err := EncodeContinuationToken(key, test.claims)
			if err != nil {
				t.Fatalf("EncodeContinuationToken() error = %v", err)
			}
			if gotToken != test.want {
				t.Fatalf("EncodeContinuationToken() = %q, want fixed token %q", gotToken, test.want)
			}

			gotClaims, err := DecodeContinuationToken(key, gotToken)
			if err != nil {
				t.Fatalf("DecodeContinuationToken() error = %v", err)
			}
			if !reflect.DeepEqual(gotClaims, test.wantClaims) {
				t.Errorf("DecodeContinuationToken() = %#v, want %#v", gotClaims, test.wantClaims)
			}
		})
	}

	t.Run("decoder accepts absent swarm settled", func(t *testing.T) {
		payload := `{"v":1,"s":"段階🚀\"\\\u0000\n  ","c":"スコープ","i":2,"b":"bundle","d":"directive-hash","r":"route-hash","a":true,"u":null,"k":null,"f":false,"g":"unresolved","x":false,"p":true,"w":true,"h":"state-hash"}`
		token := signedContinuationTokenForTest(key, payload, payload)
		gotClaims, err := DecodeContinuationToken(key, token)
		if err != nil {
			t.Fatalf("DecodeContinuationToken() error = %v", err)
		}
		if !reflect.DeepEqual(gotClaims, common) {
			t.Errorf("DecodeContinuationToken() = %#v, want %#v", gotClaims, common)
		}
		if gotClaims.SwarmSettled != nil {
			t.Errorf("DecodeContinuationToken() SwarmSettled = %v, want nil", *gotClaims.SwarmSettled)
		}
	})
}

func TestContinuationTokenRejectsInvalidSchema(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	validPayload := "{\"v\":1,\"s\":\"stage\",\"c\":\"scope\",\"i\":2,\"b\":\"bundle\",\"d\":\"directive\",\"r\":\"route\",\"a\":true,\"u\":null,\"k\":null,\"f\":false,\"g\":\"unresolved\",\"n\":null,\"x\":false,\"p\":true,\"w\":true,\"z\":false,\"q\":\"unit-end\",\"h\":null}"
	validToken := signedContinuationTokenForTest(key, validPayload, validPayload)

	encodeTests := []struct {
		name   string
		key    []byte
		mutate func(*ContinuationClaims)
	}{
		{name: "key length zero", key: nil},
		{name: "key length 31", key: make([]byte, 31)},
		{name: "key length 33", key: make([]byte, 33)},
		{name: "version", mutate: func(claims *ContinuationClaims) { claims.Version = 2 }},
		{name: "next part", mutate: func(claims *ContinuationClaims) { claims.NextPart = 0 }},
		{name: "stage invalid utf-8", mutate: func(claims *ContinuationClaims) { claims.Stage = string([]byte{0xff}) }},
		{name: "scope invalid utf-8", mutate: func(claims *ContinuationClaims) { claims.Scope = string([]byte{0xff}) }},
		{name: "bundle invalid utf-8", mutate: func(claims *ContinuationClaims) { claims.Bundle = string([]byte{0xff}) }},
		{name: "directive hash invalid utf-8", mutate: func(claims *ContinuationClaims) { claims.DirectiveHash = string([]byte{0xff}) }},
		{name: "route hash invalid utf-8", mutate: func(claims *ContinuationClaims) { claims.RouteHash = string([]byte{0xff}) }},
		{
			name: "unit invalid utf-8",
			mutate: func(claims *ContinuationClaims) {
				value := string([]byte{0xff})
				claims.Unit = &value
			},
		},
		{
			name: "unit kind invalid utf-8",
			mutate: func(claims *ContinuationClaims) {
				value := string([]byte{0xff})
				claims.UnitKind = &value
			},
		},
		{
			name: "next stage invalid utf-8",
			mutate: func(claims *ContinuationClaims) {
				value := string([]byte{0xff})
				claims.NextStage = OptionalNullableString{Present: true, Value: &value}
			},
		},
		{
			name: "state hash invalid utf-8",
			mutate: func(claims *ContinuationClaims) {
				value := string([]byte{0xff})
				claims.StateHash = &value
			},
		},
		{name: "gate zero", mutate: func(claims *ContinuationClaims) { claims.Gate = GateInvalid }},
		{name: "gate unknown", mutate: func(claims *ContinuationClaims) { claims.Gate = GateValue(99) }},
		{name: "unit gate rhythm unknown", mutate: func(claims *ContinuationClaims) { claims.UnitGate = UnitGateRhythm("unknown") }},
		{
			name: "next stage value without present",
			mutate: func(claims *ContinuationClaims) {
				value := "next"
				claims.NextStage = OptionalNullableString{Value: &value}
			},
		},
	}

	for _, test := range encodeTests {
		t.Run("encode/"+test.name, func(t *testing.T) {
			claims := continuationSchemaClaimsForTest()
			if test.mutate != nil {
				test.mutate(&claims)
			}
			got, err := EncodeContinuationToken(test.key, claims)
			if !errors.Is(err, ErrInvalidContinuationToken) {
				t.Errorf("EncodeContinuationToken() error = %v, want ErrInvalidContinuationToken", err)
			}
			if got != "" {
				t.Errorf("EncodeContinuationToken() token = %q, want empty token", got)
			}
		})
	}

	zeroMAC := base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size))
	decodeTests := []struct {
		name  string
		key   []byte
		token string
	}{
		{name: "key length zero", key: nil, token: validToken},
		{name: "key length 31", key: make([]byte, 31), token: validToken},
		{name: "key length 33", key: make([]byte, 33), token: validToken},
		{name: "invalid base64url", key: key, token: "%"},
		{
			name:  "invalid JSON",
			key:   key,
			token: base64.RawURLEncoding.EncodeToString([]byte("{")),
		},
		{
			name:  "payload is not object",
			key:   key,
			token: continuationEnvelopeForTest("[]", zeroMAC),
		},
		{
			name:  "payload is JSON string",
			key:   key,
			token: continuationEnvelopeForTest("\"payload\"", zeroMAC),
		},
		{
			name:  "MAC is not string",
			key:   key,
			token: continuationEnvelopeForTest(validPayload, "false"),
		},
		{
			name:  "MAC is null",
			key:   key,
			token: continuationEnvelopeForTest(validPayload, "null"),
		},
		{
			name:  "MAC is invalid base64url",
			key:   key,
			token: continuationEnvelopeForTest(validPayload, "\"%\""),
		},
		{
			name:  "MAC has wrong length",
			key:   key,
			token: continuationEnvelopeForTest(validPayload, "\"AA\""),
		},
		{
			name:  "required field missing",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"s\":\"stage\"", "", 1), validPayload),
		},
		{
			name: "stage wrong scalar null",
			key:  key,
			token: signedContinuationTokenForTest(
				key,
				strings.Replace(validPayload, ",\"s\":\"stage\"", ",\"s\":null", 1),
				strings.Replace(validPayload, ",\"s\":\"stage\"", ",\"s\":\"\"", 1),
			),
		},
		{
			name: "state aware wrong scalar null",
			key:  key,
			token: signedContinuationTokenForTest(
				key,
				strings.Replace(validPayload, ",\"a\":true", ",\"a\":null", 1),
				strings.Replace(validPayload, ",\"a\":true", ",\"a\":false", 1),
			),
		},
		{
			name:  "next part wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"i\":2", ",\"i\":\"2\"", 1), validPayload),
		},
		{
			name:  "nullable unit wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"u\":null", ",\"u\":false", 1), validPayload),
		},
		{
			name:  "gate wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"g\":\"unresolved\"", ",\"g\":0", 1), validPayload),
		},
		{
			name:  "next stage wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"n\":null", ",\"n\":true", 1), validPayload),
		},
		{
			name:  "swarm settled wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"z\":false", ",\"z\":\"false\"", 1), validPayload),
		},
		{
			name:  "unit gate rhythm wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"q\":\"unit-end\"", ",\"q\":false", 1), validPayload),
		},
		{
			name:  "state hash wrong scalar",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"h\":null", ",\"h\":0", 1), validPayload),
		},
		{
			name:  "version invalid",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, "{\"v\":1", "{\"v\":2", 1), validPayload),
		},
		{
			name:  "next part invalid",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"i\":2", ",\"i\":0", 1), validPayload),
		},
		{
			name:  "gate unknown",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"g\":\"unresolved\"", ",\"g\":\"unknown\"", 1), validPayload),
		},
		{
			name:  "rhythm unknown",
			key:   key,
			token: signedContinuationTokenForTest(key, strings.Replace(validPayload, ",\"q\":\"unit-end\"", ",\"q\":\"unknown\"", 1), validPayload),
		},
		{
			name: "invalid UTF-8 string",
			key:  key,
			token: signedContinuationTokenForTest(
				key,
				strings.Replace(validPayload, ",\"s\":\"stage\"", ",\"s\":\""+string([]byte{0xff})+"\"", 1),
				strings.Replace(validPayload, ",\"s\":\"stage\"", ",\"s\":\"�\"", 1),
			),
		},
		{
			name:  "probe envelope field",
			key:   key,
			token: continuationEnvelopeWithExtraForTest(validPayload, zeroMAC, "\"probe\":true"),
		},
	}

	for _, test := range decodeTests {
		t.Run("decode/"+test.name, func(t *testing.T) {
			got, err := DecodeContinuationToken(test.key, test.token)
			if !errors.Is(err, ErrInvalidContinuationToken) {
				t.Errorf("DecodeContinuationToken() error = %v, want ErrInvalidContinuationToken", err)
			}
			if !reflect.DeepEqual(got, ContinuationClaims{}) {
				t.Errorf("DecodeContinuationToken() claims = %#v, want zero claims", got)
			}
		})
	}
}

func TestContinuationTokenRejectsTampering(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	claims := continuationSchemaClaimsForTest()
	validToken, err := EncodeContinuationToken(key, claims)
	if err != nil {
		t.Fatalf("EncodeContinuationToken() error = %v", err)
	}
	if _, err := DecodeContinuationToken(key, validToken); err != nil {
		t.Fatalf("DecodeContinuationToken() rejected valid token: %v", err)
	}

	mutateEnvelope := func(token, old, replacement string) string {
		t.Helper()
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatalf("base64 decode token: %v", err)
		}
		rawString := string(raw)
		count := strings.Count(rawString, old)
		changed := strings.Replace(rawString, old, replacement, 1)
		if count != 1 {
			t.Fatalf("replace %q in envelope %q: count = %d, want 1", old, string(raw), count)
		}
		return base64.RawURLEncoding.EncodeToString([]byte(changed))
	}

	tamperedMACRaw, err := base64.RawURLEncoding.DecodeString(validToken)
	if err != nil {
		t.Fatalf("base64 decode valid token: %v", err)
	}
	macMarker := `,"m":"`
	macStart := strings.Index(string(tamperedMACRaw), macMarker)
	if macStart < 0 {
		t.Fatalf("valid token envelope lacks MAC marker %q", macMarker)
	}
	macStart += len(macMarker)
	if macStart >= len(tamperedMACRaw) || tamperedMACRaw[macStart] == '"' {
		t.Fatalf("valid token envelope has no MAC value")
	}
	macByte := byte('A')
	if tamperedMACRaw[macStart] == macByte {
		macByte = 'B'
	}
	tamperedMACRaw[macStart] = macByte
	tamperedMAC := base64.RawURLEncoding.EncodeToString(tamperedMACRaw)

	tamperedTokenByte := "A" + validToken[1:]
	if validToken[0] == 'A' {
		tamperedTokenByte = "B" + validToken[1:]
	}

	otherKey := []byte("01234567890123456789012345678902")
	tests := []struct {
		name  string
		key   []byte
		token string
	}{
		{
			name:  "payload part changed while MAC is unchanged",
			key:   key,
			token: mutateEnvelope(validToken, `,"i":2,`, `,"i":3,`),
		},
		{
			name:  "payload field changed while MAC is unchanged",
			key:   key,
			token: mutateEnvelope(validToken, `,"s":"stage",`, `,"s":"changed",`),
		},
		{name: "MAC changed", key: key, token: tamperedMAC},
		{name: "token byte changed", key: key, token: tamperedTokenByte},
		{name: "different key", key: otherKey, token: validToken},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeContinuationToken(test.key, test.token)
			if !errors.Is(err, ErrInvalidContinuationToken) {
				t.Errorf("DecodeContinuationToken() error = %v, want ErrInvalidContinuationToken", err)
			}
			if !reflect.DeepEqual(got, ContinuationClaims{}) {
				t.Errorf("DecodeContinuationToken() claims = %#v, want zero claims", got)
			}
		})
	}
}

func continuationSchemaClaimsForTest() ContinuationClaims {
	unit := "unit"
	unitKind := "kind"
	nextStage := "next-stage"
	stateHash := "state-hash"
	swarmSettled := true
	return ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      2,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    true,
		Unit:          &unit,
		UnitKind:      &unitKind,
		ForcePersona:  false,
		Gate:          GateTrue,
		NextStage:     OptionalNullableString{Present: true, Value: &nextStage},
		Single:        false,
		UnitSpecific:  true,
		Wave:          true,
		SwarmSettled:  &swarmSettled,
		UnitGate:      UnitGateRhythmUnitEnd,
		StateHash:     &stateHash,
	}
}

func signedContinuationTokenForTest(key []byte, payload, macPayload string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(macPayload))
	macText := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return continuationEnvelopeForTest(payload, "\""+macText+"\"")
}

func continuationEnvelopeForTest(payload, mac string) string {
	envelope := "{\"p\":" + payload + ",\"m\":" + mac + "}"
	return base64.RawURLEncoding.EncodeToString([]byte(envelope))
}

func continuationEnvelopeWithExtraForTest(payload, mac, extra string) string {
	envelope := "{\"p\":" + payload + ",\"m\":\"" + mac + "\"," + extra + "}"
	return base64.RawURLEncoding.EncodeToString([]byte(envelope))
}
