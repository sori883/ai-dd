package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
