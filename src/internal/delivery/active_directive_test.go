package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestActiveDirectiveRoundTripPreservesV2Fields(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	intentUUID := "intent-uuid"
	marker := ActiveDirectiveMarker{
		Version:                         2,
		Stage:                           "code-generation",
		Unit:                            "unit-a",
		StateSHA256:                     strings.Repeat("a", sha256.Size*2),
		Units:                           []string{"unit-a", "unit-b"},
		Revision:                        7,
		ProjectSHA256:                   strings.Repeat("b", sha256.Size*2),
		IntentUUID:                      &intentUUID,
		StatePresent:                    true,
		CodeGenerationSourceSHA256:      strings.Repeat("c", sha256.Size*2),
		CodeGenerationAuthorityRevision: 8,
		CursorHarness:                   "codex",
		OwnerSession:                    "sessionless:test",
		OwnerEpoch:                      2,
		ContextEpoch:                    3,
		Kind:                            ActiveDirectiveKindLoadSteering,
		Part:                            1,
		Parts:                           2,
		ContinueToken:                   "signed-token",
		ContinueTokenSHA256:             sha256Hex("signed-token"),
		Delivery:                        ActiveDirectiveDeliveryIssued,
		NeedsRehydrate:                  false,
		ActiveAttempt: &ActiveDirectiveAttempt{
			ID:                 "attempt-1",
			CommandKind:        ActiveDirectiveCommandNext,
			CommandSHA256:      strings.Repeat("d", sha256.Size*2),
			IssuedStateSHA256:  strings.Repeat("e", sha256.Size*2),
			SessionID:          "sessionless:test",
			OwnerEpoch:         2,
			ContextEpoch:       3,
			Status:             ActiveDirectiveAttemptPending,
			ClaimRevision:      4,
			SharedAttempt:      true,
			CursorInputSHA256:  strings.Repeat("f", sha256.Size*2),
			ResultSHA256:       strings.Repeat("0", sha256.Size*2),
			ResultRevision:     5,
			ResumeRequest:      true,
			ResumeAction:       ActiveDirectiveResumeActionResume,
			ResumeGateRevision: 6,
		},
		Resume: &ActiveDirectiveResume{
			Status:             ActiveDirectiveResumeWaiting,
			IssuingStage:       "code-generation",
			IssuingStateSHA256: strings.Repeat("1", sha256.Size*2),
			IssuingSession:     "sessionless:test",
			IssuingIntentUUID:  &intentUUID,
			Action:             ActiveDirectiveResumeActionResume,
		},
		EventSequence:        9,
		HumanSequence:        10,
		EngineSequence:       11,
		ConversationSequence: 12,
		StopFingerprint:      strings.Repeat("2", sha256.Size*2),
		StopCount:            13,
		Extra: map[string]json.RawMessage{
			"future_field": json.RawMessage(`{"value":true}`),
		},
	}

	if err := WriteActiveDirectiveMarker(root, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker() error = %v", err)
	}
	got, found, err := ReadActiveDirectiveMarker(root)
	if err != nil {
		t.Fatalf("ReadActiveDirectiveMarker() error = %v", err)
	}
	if !found {
		t.Fatal("ReadActiveDirectiveMarker() found = false, want true")
	}
	if !reflect.DeepEqual(got, marker) {
		t.Errorf("ReadActiveDirectiveMarker() = %#v, want %#v", got, marker)
	}

	data, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(marker): %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("active marker is not newline terminated")
	}
}

func TestActiveDirectiveRoundTripPreservesExplicitNullIntent(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	marker := minimalActiveDirectiveMarker()
	marker.IntentUUID = nil
	if err := WriteActiveDirectiveMarker(root, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker() error = %v", err)
	}
	data, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(marker): %v", err)
	}
	if !bytes.Contains(data, []byte(`"intent_uuid":null`)) {
		t.Errorf("marker = %s, want explicit null intent_uuid", data)
	}
}

func TestActiveDirectivePreservesOptionalPresenceAndUnknownNestedFields(t *testing.T) {
	rootPath := t.TempDir()
	markerPath := filepath.Join(rootPath, activeDirectiveMarkerName)
	stateHash := strings.Repeat("a", sha256.Size*2)
	projectHash := strings.Repeat("b", sha256.Size*2)
	raw := fmt.Sprintf(`{"version":2,"stage":"intent-capture","state_sha256":%q,"revision":0,"project_sha256":%q,"intent_uuid":null,"state_present":false,"code_generation_authority_revision":0,"part":0,"parts":0,"continue_token":"","continue_token_sha256":%q,"stop_fingerprint":"","owner_session":"session","owner_epoch":0,"context_epoch":0,"kind":"run-stage","delivery":"delivered","needs_rehydrate":false,"active_attempt":{"id":"","command_kind":"next","command_sha256":%q,"issued_state_sha256":%q,"session_id":"session","owner_epoch":0,"context_epoch":0,"status":"settled","claim_revision":0,"shared_attempt":false,"resume_request":false,"resume_action":"","resume_gate_revision":0,"attempt_unknown":{"kept":true}},"resume":null,"event_sequence":0,"human_sequence":0,"engine_sequence":0,"conversation_sequence":0,"stop_count":0,"top_unknown":[1,true]}`,
		stateHash,
		projectHash,
		sha256Hex(""),
		strings.Repeat("c", sha256.Size*2),
		stateHash,
	)
	if err := os.WriteFile(markerPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	marker, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if err := WriteActiveDirectiveMarker(root, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker() error = %v", err)
	}
	gotBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(round-trip marker): %v", err)
	}
	assertJSONFieldsPresent(t, gotBytes,
		"code_generation_authority_revision", "part", "parts", "continue_token", "continue_token_sha256",
		"stop_fingerprint", "resume", "top_unknown",
	)
	assertJSONFieldsPresent(t, nestedJSONField(gotBytes, "active_attempt"),
		"id", "claim_revision", "shared_attempt", "resume_request", "resume_action", "resume_gate_revision", "attempt_unknown",
	)
	var want, got map[string]any
	if err := json.Unmarshal([]byte(raw), &want); err != nil {
		t.Fatalf("Unmarshal(want): %v", err)
	}
	if err := json.Unmarshal(bytes.TrimSpace(gotBytes), &got); err != nil {
		t.Fatalf("Unmarshal(got): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic marker round-trip = %#v, want %#v", got, want)
	}
}

func TestActiveDirectivePreservesUnknownResumeFields(t *testing.T) {
	marker := minimalActiveDirectiveMarker()
	marker.Resume = &ActiveDirectiveResume{
		Status:             ActiveDirectiveResumeWaiting,
		IssuingStage:       marker.Stage,
		IssuingStateSHA256: marker.StateSHA256,
		IssuingSession:     marker.OwnerSession,
		IssuingIntentUUID:  marker.IntentUUID,
		Action:             "",
		Extra: map[string]json.RawMessage{
			"resume_unknown": json.RawMessage(`{"value":42}`),
		},
		present: map[string]bool{"action": true},
	}
	data, err := json.Marshal(marker)
	if err != nil {
		t.Fatalf("Marshal(marker): %v", err)
	}
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), data, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	got, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if got.Resume == nil || string(got.Resume.Extra["resume_unknown"]) != `{"value":42}` {
		t.Fatalf("decoded resume = %#v, want unknown field preserved", got.Resume)
	}
	if err := WriteActiveDirectiveMarker(root, got); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker() error = %v", err)
	}
	assertJSONFieldsPresent(t, nestedJSONField(mustReadMarkerBytes(t, root), "resume"), "action", "resume_unknown")
}

func TestActiveDirectiveRejectsDisallowedExplicitEmptyOptionals(t *testing.T) {
	base, err := json.Marshal(minimalActiveDirectiveMarker())
	if err != nil {
		t.Fatalf("Marshal(minimal marker): %v", err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]json.RawMessage) error
	}{
		{
			name: "code_generation_source_sha256",
			mutate: func(raw map[string]json.RawMessage) error {
				raw["code_generation_source_sha256"] = json.RawMessage(`""`)
				return nil
			},
		},
		{
			name: "cursor_harness",
			mutate: func(raw map[string]json.RawMessage) error {
				raw["cursor_harness"] = json.RawMessage(`""`)
				return nil
			},
		},
		{
			name: "active_attempt.cursor_input_sha256",
			mutate: func(raw map[string]json.RawMessage) error {
				var attempt map[string]json.RawMessage
				if err := json.Unmarshal(raw["active_attempt"], &attempt); err != nil {
					return err
				}
				attempt["cursor_input_sha256"] = json.RawMessage(`""`)
				encoded, err := json.Marshal(attempt)
				if err != nil {
					return err
				}
				raw["active_attempt"] = encoded
				return nil
			},
		},
		{
			name: "active_attempt.result_sha256",
			mutate: func(raw map[string]json.RawMessage) error {
				var attempt map[string]json.RawMessage
				if err := json.Unmarshal(raw["active_attempt"], &attempt); err != nil {
					return err
				}
				attempt["result_sha256"] = json.RawMessage(`""`)
				encoded, err := json.Marshal(attempt)
				if err != nil {
					return err
				}
				raw["active_attempt"] = encoded
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(base, &raw); err != nil {
				t.Fatalf("Unmarshal(marker): %v", err)
			}
			if err := test.mutate(raw); err != nil {
				t.Fatalf("mutate marker: %v", err)
			}
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("Marshal(mutated marker): %v", err)
			}
			assertActiveDirectiveMarkerRejected(t, data)
		})
	}
}

func TestActiveDirectivePreservesWhitespaceInFixedUnits(t *testing.T) {
	base, err := json.Marshal(minimalActiveDirectiveMarker())
	if err != nil {
		t.Fatalf("Marshal(minimal marker): %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(base, &raw); err != nil {
		t.Fatalf("Unmarshal(marker): %v", err)
	}
	raw["units"] = json.RawMessage(`[" unit-a ","\tunit-b\t"]`)
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal(marker with whitespace units): %v", err)
	}
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), data, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	marker, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if !reflect.DeepEqual(marker.Units, []string{" unit-a ", "\tunit-b\t"}) {
		t.Fatalf("decoded units = %#v, want fixed parser values with whitespace preserved", marker.Units)
	}
	if err := WriteActiveDirectiveMarker(root, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(round-trip): %v", err)
	}
	var got map[string]any
	roundTrip := mustReadMarkerBytes(t, root)
	if err := json.Unmarshal(bytes.TrimSpace(roundTrip), &got); err != nil {
		t.Fatalf("Unmarshal(round-trip marker): %v", err)
	}
	if !reflect.DeepEqual(got["units"], []any{" unit-a ", "\tunit-b\t"}) {
		t.Errorf("round-trip units = %#v, want whitespace-preserving values", got["units"])
	}
}

func TestActiveDirectiveV1RoundTripRemainsNarrow(t *testing.T) {
	rootPath := t.TempDir()
	raw := []byte(`{"version":1,"stage":"intent-capture","unit":"unit-a","state_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","project_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","kind":"run-stage","top_unknown":true}`)
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), raw, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	marker, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if marker.Version != 1 || marker.ProjectSHA256 != "" || marker.Kind != "" || len(marker.Extra) != 0 {
		t.Fatalf("decoded v1 marker = %#v, want narrow representation", marker)
	}
	if err := WriteActiveDirectiveMarker(root, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker() error = %v", err)
	}
	var got map[string]any
	data := mustReadMarkerBytes(t, root)
	if err := json.Unmarshal(bytes.TrimSpace(data), &got); err != nil {
		t.Fatalf("Unmarshal(round-trip v1): %v", err)
	}
	if len(got) != 4 || got["version"] != float64(1) || got["stage"] != "intent-capture" || got["unit"] != "unit-a" || got["state_sha256"] != strings.Repeat("a", sha256.Size*2) {
		t.Errorf("round-trip v1 = %#v, want exactly fixed v1 fields", got)
	}
}

func TestActiveDirectiveV1IgnoresMalformedV2Fields(t *testing.T) {
	raw := []byte(`{"version":1,"stage":"intent-capture","state_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","revision":"not-a-number","active_attempt":"not-an-attempt","unknown":[1,true]}`)
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), raw, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	marker, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v, want narrow v1 read", found, err)
	}
	if marker.Version != 1 || marker.Stage != "intent-capture" || len(marker.Extra) != 0 {
		t.Errorf("decoded v1 marker = %#v, want malformed v2 fields ignored", marker)
	}
}

func TestActiveDirectiveTrimsFixedJavaScriptWhitespace(t *testing.T) {
	marker := minimalActiveDirectiveMarker()
	marker.Stage = "\ufeff" + marker.Stage + " "
	data := mustMarshalActiveDirectiveMarkerWithoutValidation(marker)
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), data, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	got, found, err := ReadActiveDirectiveMarker(root)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if got.Stage != minimalActiveDirectiveMarker().Stage {
		t.Errorf("decoded stage = %q, want trimmed %q", got.Stage, minimalActiveDirectiveMarker().Stage)
	}
}

func TestActiveDirectiveAcceptsFixedTopLevelUnitNames(t *testing.T) {
	for _, version := range []int{1, 2} {
		for _, unit := range []string{".unit", "_unit", "-unit", strings.Repeat("u", 65)} {
			t.Run(fmt.Sprintf("v%d/%s", version, unit), func(t *testing.T) {
				rootPath := t.TempDir()
				root, err := os.OpenRoot(rootPath)
				if err != nil {
					t.Fatalf("OpenRoot(%q): %v", rootPath, err)
				}
				defer func() { _ = root.Close() }()
				marker := minimalActiveDirectiveMarker()
				marker.Version = version
				marker.Unit = unit
				if err := WriteActiveDirectiveMarker(root, marker); err != nil {
					t.Fatalf("WriteActiveDirectiveMarker() error = %v, want fixed top-level unit accepted", err)
				}
				got, found, err := ReadActiveDirectiveMarker(root)
				if err != nil || !found {
					t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
				}
				if got.Unit != unit {
					t.Errorf("round-trip top-level unit = %q, want %q", got.Unit, unit)
				}
			})
		}
	}
}

func TestActiveDirectiveRejectsInvalidFixedUnitNames(t *testing.T) {
	invalid := []struct {
		name   string
		mutate func(*ActiveDirectiveMarker)
	}{
		{name: "units leading dot", mutate: func(marker *ActiveDirectiveMarker) { marker.Units = []string{".unit"} }},
		{name: "units leading underscore", mutate: func(marker *ActiveDirectiveMarker) { marker.Units = []string{"_unit"} }},
		{name: "units leading hyphen", mutate: func(marker *ActiveDirectiveMarker) { marker.Units = []string{"-unit"} }},
		{name: "units too long", mutate: func(marker *ActiveDirectiveMarker) { marker.Units = []string{strings.Repeat("u", 65)} }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatalf("OpenRoot(%q): %v", rootPath, err)
			}
			defer func() { _ = root.Close() }()
			marker := minimalActiveDirectiveMarker()
			test.mutate(&marker)
			if err := WriteActiveDirectiveMarker(root, marker); err == nil {
				t.Fatal("WriteActiveDirectiveMarker() error = nil, want fixed units rejection")
			} else if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
				t.Errorf("WriteActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
			}
		})
	}
}

func TestActiveDirectiveRejectsWhitespaceOnlyTopLevelUnit(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			rootPath := t.TempDir()
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatalf("OpenRoot(%q): %v", rootPath, err)
			}
			defer func() { _ = root.Close() }()
			marker := minimalActiveDirectiveMarker()
			marker.Version = version
			marker.Unit = " \t\n"
			if err := WriteActiveDirectiveMarker(root, marker); err == nil {
				t.Fatal("WriteActiveDirectiveMarker() error = nil, want whitespace-only top-level unit rejection")
			} else if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
				t.Errorf("WriteActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
			}
			data := mustMarshalActiveDirectiveMarkerWithoutValidation(marker)
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("Unmarshal(marker): %v", err)
			}
			raw["unit"] = json.RawMessage(`" \t\n"`)
			data, err = json.Marshal(raw)
			if err != nil {
				t.Fatalf("Marshal(marker with whitespace unit): %v", err)
			}
			assertActiveDirectiveMarkerRejected(t, data)
		})
	}
}

func mustMarshalActiveDirectiveMarkerWithoutValidation(marker ActiveDirectiveMarker) []byte {
	data, err := json.Marshal(activeDirectiveMarkerWire{
		Version:        marker.Version,
		Stage:          marker.Stage,
		StateSHA256:    marker.StateSHA256,
		Revision:       marker.Revision,
		ProjectSHA256:  marker.ProjectSHA256,
		IntentUUID:     marker.IntentUUID,
		StatePresent:   marker.StatePresent,
		OwnerSession:   marker.OwnerSession,
		OwnerEpoch:     marker.OwnerEpoch,
		ContextEpoch:   marker.ContextEpoch,
		Kind:           marker.Kind,
		Delivery:       marker.Delivery,
		NeedsRehydrate: marker.NeedsRehydrate,
		ActiveAttempt: &activeDirectiveAttemptWire{
			CommandKind:       marker.ActiveAttempt.CommandKind,
			CommandSHA256:     marker.ActiveAttempt.CommandSHA256,
			IssuedStateSHA256: marker.ActiveAttempt.IssuedStateSHA256,
			SessionID:         marker.ActiveAttempt.SessionID,
			OwnerEpoch:        marker.ActiveAttempt.OwnerEpoch,
			ContextEpoch:      marker.ActiveAttempt.ContextEpoch,
			Status:            marker.ActiveAttempt.Status,
		},
		EventSequence:        marker.EventSequence,
		HumanSequence:        marker.HumanSequence,
		EngineSequence:       marker.EngineSequence,
		ConversationSequence: marker.ConversationSequence,
		StopCount:            marker.StopCount,
	})
	if err != nil {
		panic(err)
	}
	return data
}

func assertJSONFieldsPresent(t *testing.T, data []byte, fields ...string) {
	t.Helper()
	var values map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(data), &values); err != nil {
		t.Fatalf("Unmarshal(JSON object): %v", err)
	}
	for _, field := range fields {
		if _, ok := values[field]; !ok {
			t.Errorf("JSON field %q is absent", field)
		}
	}
}

func nestedJSONField(data []byte, field string) []byte {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(data), &values); err != nil {
		return nil
	}
	return values[field]
}

func mustReadMarkerBytes(t *testing.T, root *os.Root) []byte {
	t.Helper()
	data, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(marker): %v", err)
	}
	return data
}

func TestActiveDirectiveRejectsInvalidExtra(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	marker := minimalActiveDirectiveMarker()
	marker.Extra = map[string]json.RawMessage{"future_field": json.RawMessage("{")}
	err = WriteActiveDirectiveMarker(root, marker)
	if err == nil {
		t.Fatal("WriteActiveDirectiveMarker() error = nil, want invalid Extra rejection")
	}
	if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
		t.Errorf("WriteActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
	}
}

func TestActiveDirectiveRejectsInvalidTokenUTF8(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	marker := minimalActiveDirectiveMarker()
	marker.ContinueToken = string([]byte{'x', 0xff})
	marker.ContinueTokenSHA256 = sha256Hex(marker.ContinueToken)
	if err := WriteActiveDirectiveMarker(root, marker); err == nil {
		t.Fatal("WriteActiveDirectiveMarker() error = nil, want invalid UTF-8 rejection")
	} else if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
		t.Errorf("WriteActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
	}
}

func TestActiveDirectiveRejectsMalformedMarkers(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "unknown version", data: []byte(`{"version":3}`)},
		{name: "multiple JSON values", data: []byte(`{"version":2}{"version":2}`)},
		{name: "invalid UTF-8", data: append([]byte(`{"version":2,"stage":"`), 0xff, byte('"'), byte('}'))},
		{name: "oversized", data: bytes.Repeat([]byte{'x'}, activeDirectiveMaxBytes+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), test.data, 0o600); err != nil {
				t.Fatalf("WriteFile(marker): %v", err)
			}
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatalf("OpenRoot(%q): %v", rootPath, err)
			}
			defer func() { _ = root.Close() }()
			_, found, err := ReadActiveDirectiveMarker(root)
			if err == nil {
				t.Fatal("ReadActiveDirectiveMarker() error = nil, want rejection")
			}
			if found {
				t.Error("ReadActiveDirectiveMarker() found = true, want false")
			}
			if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
				t.Errorf("ReadActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
			}
		})
	}
}

func TestActiveDirectiveRejectsNullKnownFields(t *testing.T) {
	base, err := json.Marshal(minimalActiveDirectiveMarker())
	if err != nil {
		t.Fatalf("Marshal(minimal marker): %v", err)
	}
	for _, field := range []string{
		"state_present",
		"revision",
		"needs_rehydrate",
		"units",
		"code_generation_source_sha256",
		"cursor_harness",
	} {
		t.Run(field, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(base, &raw); err != nil {
				t.Fatalf("Unmarshal(marker): %v", err)
			}
			raw[field] = json.RawMessage("null")
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("Marshal(mutated marker): %v", err)
			}
			assertActiveDirectiveMarkerRejected(t, data)
		})
	}

	t.Run("active_attempt.shared_attempt", func(t *testing.T) {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(base, &raw); err != nil {
			t.Fatalf("Unmarshal(marker): %v", err)
		}
		var attempt map[string]json.RawMessage
		if err := json.Unmarshal(raw["active_attempt"], &attempt); err != nil {
			t.Fatalf("Unmarshal(active_attempt): %v", err)
		}
		attempt["shared_attempt"] = json.RawMessage("null")
		raw["active_attempt"], err = json.Marshal(attempt)
		if err != nil {
			t.Fatalf("Marshal(mutated attempt): %v", err)
		}
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("Marshal(mutated marker): %v", err)
		}
		assertActiveDirectiveMarkerRejected(t, data)
	})
}

func assertActiveDirectiveMarkerRejected(t *testing.T, data []byte) {
	t.Helper()
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, activeDirectiveMarkerName), data, 0o600); err != nil {
		t.Fatalf("WriteFile(marker): %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	_, found, err := ReadActiveDirectiveMarker(root)
	if err == nil {
		t.Fatal("ReadActiveDirectiveMarker() error = nil, want rejection")
	}
	if found {
		t.Error("ReadActiveDirectiveMarker() found = true, want false")
	}
	if !errors.Is(err, ErrInvalidActiveDirectiveMarker) {
		t.Errorf("ReadActiveDirectiveMarker() error = %v, want ErrInvalidActiveDirectiveMarker", err)
	}
}

func TestActiveDirectiveRejectsSymlinkAndNonRegular(t *testing.T) {
	if err := os.Symlink("outside", filepath.Join(t.TempDir(), "probe")); err != nil && !errors.Is(err, fs.ErrPermission) {
		t.Skipf("symlink unsupported: %v", err)
	}

	tests := []struct {
		name  string
		setup func(string) error
	}{
		{
			name: "symlink",
			setup: func(rootPath string) error {
				return os.Symlink("target", filepath.Join(rootPath, activeDirectiveMarkerName))
			},
		},
		{
			name: "directory",
			setup: func(rootPath string) error {
				return os.Mkdir(filepath.Join(rootPath, activeDirectiveMarkerName), 0o700)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			if err := test.setup(rootPath); err != nil {
				t.Fatalf("setup: %v", err)
			}
			root, err := os.OpenRoot(rootPath)
			if err != nil {
				t.Fatalf("OpenRoot(%q): %v", rootPath, err)
			}
			defer func() { _ = root.Close() }()
			_, found, err := ReadActiveDirectiveMarker(root)
			if err == nil {
				t.Fatal("ReadActiveDirectiveMarker() error = nil, want rejection")
			}
			if found {
				t.Error("ReadActiveDirectiveMarker() found = true, want false")
			}
		})
	}
}

func TestActiveDirectiveRejectsIdentityStateMismatch(t *testing.T) {
	marker := minimalActiveDirectiveMarker()
	state := []byte("state bytes")
	marker.ProjectSHA256 = sha256Hex("project")
	marker.StateSHA256 = sha256Hex(string(state))
	context := ActiveDirectiveContext{
		ProjectSHA256: sha256Hex("project"),
		IntentUUID:    marker.IntentUUID,
		StatePresent:  true,
		StateSHA256:   sha256Hex(string(state)),
	}

	for _, test := range []struct {
		name   string
		mutate func(*ActiveDirectiveContext)
	}{
		{name: "project", mutate: func(value *ActiveDirectiveContext) { value.ProjectSHA256 = sha256Hex("other") }},
		{name: "intent", mutate: func(value *ActiveDirectiveContext) { other := "other"; value.IntentUUID = &other }},
		{name: "state presence", mutate: func(value *ActiveDirectiveContext) { value.StatePresent = false }},
		{name: "state digest", mutate: func(value *ActiveDirectiveContext) { value.StateSHA256 = sha256Hex("other") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := context
			test.mutate(&want)
			if err := ValidateActiveDirectiveContext(marker, want); err == nil {
				t.Fatal("ValidateActiveDirectiveContext() error = nil, want mismatch")
			}
		})
	}
	if err := ValidateActiveDirectiveContext(marker, context); err != nil {
		t.Fatalf("ValidateActiveDirectiveContext(valid) error = %v, want nil", err)
	}
}

func TestActiveDirectiveAtomicWritePreservesPreviousOnFailure(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()

	previous := minimalActiveDirectiveMarker()
	if err := WriteActiveDirectiveMarker(root, previous); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(previous): %v", err)
	}
	oldBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(previous): %v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*activeDirectiveOps)
	}{
		{
			name: "write failure",
			mutate: func(ops *activeDirectiveOps) {
				ops.write = func(*os.File, []byte) (int, error) { return 0, errors.New("write failed") }
			},
		},
		{
			name: "close failure",
			mutate: func(ops *activeDirectiveOps) {
				ops.close = func(*os.File) error { return errors.New("close failed") }
			},
		},
		{
			name: "rename failure",
			mutate: func(ops *activeDirectiveOps) {
				ops.rename = func(*os.Root, string, string) error { return errors.New("rename failed") }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ops := systemActiveDirectiveOps()
			test.mutate(&ops)
			if err := writeActiveDirectiveMarker(root, minimalActiveDirectiveMarker(), ops); err == nil {
				t.Fatal("writeActiveDirectiveMarker() error = nil, want failure")
			}
			gotBytes, err := root.ReadFile(activeDirectiveMarkerName)
			if err != nil {
				t.Fatalf("ReadFile(after failure): %v", err)
			}
			if !bytes.Equal(gotBytes, oldBytes) {
				t.Errorf("marker changed after failed commit: got %q, want %q", gotBytes, oldBytes)
			}
			entries, err := os.ReadDir(rootPath)
			if err != nil {
				t.Fatalf("ReadDir(%q): %v", rootPath, err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), activeDirectiveMarkerName+".tmp-") {
					t.Errorf("owned temp file remains after failed commit: %q", entry.Name())
				}
			}
			if _, err := root.Stat("."); err != nil {
				t.Errorf("caller root was closed by failed commit: %v", err)
			}
		})
	}
}

func TestActiveDirectiveAtomicWriteRejectsReplacedTemporary(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	previous := minimalActiveDirectiveMarker()
	if err := WriteActiveDirectiveMarker(root, previous); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(previous): %v", err)
	}
	oldBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(previous): %v", err)
	}
	ops := systemActiveDirectiveOps()
	var replacementPath string
	ops.close = func(file *os.File) error {
		if err := file.Close(); err != nil {
			return err
		}
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), activeDirectiveMarkerName+".tmp-") {
				continue
			}
			tempPath := filepath.Join(rootPath, entry.Name())
			if err := os.Remove(tempPath); err != nil {
				return err
			}
			replacementPath = tempPath
			return os.WriteFile(replacementPath, []byte("replacement"), 0o600)
		}
		return errors.New("temporary marker not found")
	}
	if err := writeActiveDirectiveMarker(root, previous, ops); err == nil {
		t.Fatal("writeActiveDirectiveMarker() error = nil, want replaced temporary rejection")
	}
	gotBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(after replacement): %v", err)
	}
	if !bytes.Equal(gotBytes, oldBytes) {
		t.Errorf("marker changed after replaced temporary: got %q, want %q", gotBytes, oldBytes)
	}
	if replacementPath == "" {
		t.Fatal("replacement path was not captured")
	}
	replacementBytes, err := os.ReadFile(replacementPath)
	if err != nil {
		t.Fatalf("ReadFile(replacement): %v", err)
	}
	if string(replacementBytes) != "replacement" {
		t.Errorf("replacement temporary bytes = %q, want replacement to remain owned by other object", replacementBytes)
	}
}

func TestActiveDirectiveAtomicWriteRejectsMutatedTemporary(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", rootPath, err)
	}
	defer func() { _ = root.Close() }()
	previous := minimalActiveDirectiveMarker()
	if err := WriteActiveDirectiveMarker(root, previous); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(previous): %v", err)
	}
	oldBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(previous): %v", err)
	}
	ops := systemActiveDirectiveOps()
	var replacementPath string
	ops.close = func(file *os.File) error {
		before, err := file.Stat()
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), activeDirectiveMarkerName+".tmp-") {
				continue
			}
			replacementPath = filepath.Join(rootPath, entry.Name())
			if err := os.WriteFile(replacementPath, []byte("replacement"), 0o600); err != nil {
				return err
			}
			after, err := os.Stat(replacementPath)
			if err != nil {
				return err
			}
			if !os.SameFile(before, after) {
				return errors.New("temporary replacement did not preserve file identity")
			}
			return nil
		}
		return errors.New("temporary marker not found")
	}
	if err := writeActiveDirectiveMarker(root, previous, ops); err == nil {
		t.Fatal("writeActiveDirectiveMarker() error = nil, want mutated temporary rejection")
	}
	gotBytes, err := root.ReadFile(activeDirectiveMarkerName)
	if err != nil {
		t.Fatalf("ReadFile(after mutation): %v", err)
	}
	if !bytes.Equal(gotBytes, oldBytes) {
		t.Errorf("marker changed after mutated temporary: got %q, want %q", gotBytes, oldBytes)
	}
	if replacementPath == "" {
		t.Fatal("replacement path was not captured")
	}
	replacementBytes, err := os.ReadFile(replacementPath)
	if err != nil {
		t.Fatalf("ReadFile(replacement): %v", err)
	}
	if string(replacementBytes) != "replacement" {
		t.Errorf("mutated temporary bytes = %q, want replacement to remain owned by other object", replacementBytes)
	}
}

func minimalActiveDirectiveMarker() ActiveDirectiveMarker {
	intentUUID := "intent-uuid"
	return ActiveDirectiveMarker{
		Version:              2,
		Stage:                "intent-capture",
		StateSHA256:          strings.Repeat("a", sha256.Size*2),
		ProjectSHA256:        strings.Repeat("b", sha256.Size*2),
		IntentUUID:           &intentUUID,
		StatePresent:         true,
		OwnerSession:         "sessionless:test",
		OwnerEpoch:           0,
		ContextEpoch:         0,
		Kind:                 ActiveDirectiveKindRunStage,
		Delivery:             ActiveDirectiveDeliveryDelivered,
		NeedsRehydrate:       false,
		Revision:             0,
		EventSequence:        0,
		HumanSequence:        0,
		EngineSequence:       0,
		ConversationSequence: 0,
		StopCount:            0,
		ActiveAttempt: &ActiveDirectiveAttempt{
			CommandKind:       ActiveDirectiveCommandNext,
			CommandSHA256:     strings.Repeat("c", sha256.Size*2),
			IssuedStateSHA256: strings.Repeat("d", sha256.Size*2),
			SessionID:         "sessionless:test",
			OwnerEpoch:        0,
			ContextEpoch:      0,
			Status:            ActiveDirectiveAttemptSettled,
		},
	}
}
