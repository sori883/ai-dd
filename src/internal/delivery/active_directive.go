package delivery

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

const (
	activeDirectiveMarkerName = ".aidlc-active-directive.json"
	activeDirectiveMaxBytes   = 64 * 1024
	activeDirectiveTempPrefix = activeDirectiveMarkerName + ".tmp-"
)

var (
	// ErrInvalidActiveDirectiveMarker identifies a marker that cannot be
	// trusted as a fixed AI-DLC 2.6.123 v2 cursor.
	ErrInvalidActiveDirectiveMarker = errors.New("delivery: invalid active directive marker")
	// ErrActiveDirectiveNotFound indicates that the record has no marker yet.
	ErrActiveDirectiveNotFound = errors.New("delivery: active directive marker not found")
)

type ActiveDirectiveKind string

const (
	ActiveDirectiveKindLoadSteering     ActiveDirectiveKind = "load-steering"
	ActiveDirectiveKindRunStage         ActiveDirectiveKind = "run-stage"
	ActiveDirectiveKindAsk              ActiveDirectiveKind = "ask"
	ActiveDirectiveKindPrint            ActiveDirectiveKind = "print"
	ActiveDirectiveKindError            ActiveDirectiveKind = "error"
	ActiveDirectiveKindDone             ActiveDirectiveKind = "done"
	ActiveDirectiveKindParked           ActiveDirectiveKind = "parked"
	ActiveDirectiveKindNotice           ActiveDirectiveKind = "notice"
	ActiveDirectiveKindDispatchSubagent ActiveDirectiveKind = "dispatch-subagent"
	ActiveDirectiveKindInvokeSwarm      ActiveDirectiveKind = "invoke-swarm"
	ActiveDirectiveKindPresentGate      ActiveDirectiveKind = "present-gate"
)

type ActiveDirectiveDelivery string

const (
	ActiveDirectiveDeliveryIssued     ActiveDirectiveDelivery = "issued"
	ActiveDirectiveDeliveryDelivered  ActiveDirectiveDelivery = "delivered"
	ActiveDirectiveDeliveryConsumed   ActiveDirectiveDelivery = "consumed"
	ActiveDirectiveDeliverySuperseded ActiveDirectiveDelivery = "superseded"
)

type ActiveDirectiveCommandKind string

const (
	ActiveDirectiveCommandNext     ActiveDirectiveCommandKind = "next"
	ActiveDirectiveCommandContinue ActiveDirectiveCommandKind = "continue"
	ActiveDirectiveCommandReport   ActiveDirectiveCommandKind = "report"
	ActiveDirectiveCommandPark     ActiveDirectiveCommandKind = "park"
)

type ActiveDirectiveAttemptStatus string

const (
	ActiveDirectiveAttemptPending ActiveDirectiveAttemptStatus = "pending"
	ActiveDirectiveAttemptSettled ActiveDirectiveAttemptStatus = "settled"
	ActiveDirectiveAttemptFailed  ActiveDirectiveAttemptStatus = "failed"
)

type ActiveDirectiveResumeStatus string

const (
	ActiveDirectiveResumeWaiting    ActiveDirectiveResumeStatus = "waiting"
	ActiveDirectiveResumeSelected   ActiveDirectiveResumeStatus = "selected"
	ActiveDirectiveResumeSuperseded ActiveDirectiveResumeStatus = "superseded"
)

type ActiveDirectiveResumeAction string

const (
	ActiveDirectiveResumeActionResume     ActiveDirectiveResumeAction = "resume"
	ActiveDirectiveResumeActionRedo       ActiveDirectiveResumeAction = "redo"
	ActiveDirectiveResumeActionJump       ActiveDirectiveResumeAction = "jump"
	ActiveDirectiveResumeActionStartFresh ActiveDirectiveResumeAction = "start-fresh"
)

// ActiveDirectiveAttempt is the v2 attempt evidence embedded in a marker.
type ActiveDirectiveAttempt struct {
	ID                 string
	CommandKind        ActiveDirectiveCommandKind
	CommandSHA256      string
	IssuedStateSHA256  string
	SessionID          string
	OwnerEpoch         int
	ContextEpoch       int
	Status             ActiveDirectiveAttemptStatus
	ClaimRevision      int
	SharedAttempt      bool
	CursorInputSHA256  string
	ResultSHA256       string
	ResultRevision     int
	ResumeRequest      bool
	ResumeAction       ActiveDirectiveResumeAction
	ResumeGateRevision int
	// Extra retains unknown nested fields from the fixed marker snapshot.
	Extra   map[string]json.RawMessage
	present map[string]bool
}

// ActiveDirectiveResume is the optional v2 resume evidence.
type ActiveDirectiveResume struct {
	Status             ActiveDirectiveResumeStatus
	IssuingStage       string
	IssuingStateSHA256 string
	IssuingSession     string
	IssuingIntentUUID  *string
	Action             ActiveDirectiveResumeAction
	// Extra retains unknown nested fields from the fixed marker snapshot.
	Extra   map[string]json.RawMessage
	present map[string]bool
}

// ActiveDirectiveMarker is the fixed AI-DLC 2.6.123 active-directive v2
// schema. Extra preserves forward-compatible top-level fields while all
// fields known by the fixed snapshot are represented explicitly.
type ActiveDirectiveMarker struct {
	Version                         int
	Stage                           string
	Unit                            string
	StateSHA256                     string
	Units                           []string
	Revision                        int
	ProjectSHA256                   string
	IntentUUID                      *string
	StatePresent                    bool
	CodeGenerationSourceSHA256      string
	CodeGenerationAuthorityRevision int
	CursorHarness                   string
	OwnerSession                    string
	OwnerEpoch                      int
	ContextEpoch                    int
	Kind                            ActiveDirectiveKind
	Part                            int
	Parts                           int
	ContinueToken                   string
	ContinueTokenSHA256             string
	Delivery                        ActiveDirectiveDelivery
	NeedsRehydrate                  bool
	ActiveAttempt                   *ActiveDirectiveAttempt
	Resume                          *ActiveDirectiveResume
	EventSequence                   int
	HumanSequence                   int
	EngineSequence                  int
	ConversationSequence            int
	StopFingerprint                 string
	StopCount                       int
	Extra                           map[string]json.RawMessage
	present                         map[string]bool
}

// ActiveDirectiveContext is the identity/state snapshot used to validate a
// marker before a continuation transaction uses it.
type ActiveDirectiveContext struct {
	ProjectSHA256 string
	IntentUUID    *string
	StatePresent  bool
	StateSHA256   string
}

type activeDirectiveOps struct {
	lstat  func(*os.Root, string) (fs.FileInfo, error)
	open   func(*os.Root, string, int, fs.FileMode) (*os.File, error)
	write  func(*os.File, []byte) (int, error)
	close  func(*os.File) error
	rename func(*os.Root, string, string) error
	remove func(*os.Root, string) error
	chmod  func(*os.Root, string, fs.FileMode) error
}

func systemActiveDirectiveOps() activeDirectiveOps {
	return activeDirectiveOps{
		lstat:  (*os.Root).Lstat,
		open:   (*os.Root).OpenFile,
		write:  (*os.File).Write,
		close:  (*os.File).Close,
		rename: (*os.Root).Rename,
		remove: (*os.Root).Remove,
		chmod:  (*os.Root).Chmod,
	}
}

var activeDirectiveTempSequence atomic.Uint64

var (
	activeDirectiveStagePattern        = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	activeDirectiveUnitPattern         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	activeDirectiveHarnessPattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	activeDirectiveSHA256Pattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	activeDirectiveSourceSHA256Pattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64}|unbindable)$`)
)

// MarshalJSON emits the canonical marker object without the trailing newline
// used by the on-disk writer.
func (marker ActiveDirectiveMarker) MarshalJSON() ([]byte, error) {
	return marshalActiveDirectiveMarker(marker)
}

// UnmarshalJSON decodes and validates one marker object.
func (marker *ActiveDirectiveMarker) UnmarshalJSON(data []byte) error {
	decoded, err := decodeActiveDirectiveMarker(data)
	if err != nil {
		return err
	}
	*marker = decoded
	return nil
}

// WriteActiveDirectiveMarker atomically publishes one validated v2 marker.
// The caller retains ownership of root; this function never closes it.
func WriteActiveDirectiveMarker(root *os.Root, marker ActiveDirectiveMarker) error {
	return writeActiveDirectiveMarker(root, marker, systemActiveDirectiveOps())
}

// ReadActiveDirectiveMarker reads one descriptor-pinned marker snapshot.
// Missing markers return found=false without an error. Invalid or unsafe
// marker paths return found=false and an error.
func ReadActiveDirectiveMarker(root *os.Root) (marker ActiveDirectiveMarker, found bool, err error) {
	if root == nil {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: nil root: %w", ErrInvalidActiveDirectiveMarker)
	}
	ops := systemActiveDirectiveOps()
	info, err := ops.lstat(root, activeDirectiveMarkerName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ActiveDirectiveMarker{}, false, nil
		}
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: lstat: %w", err)
	}
	if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: marker is not a regular non-symlink file: %w", ErrInvalidActiveDirectiveMarker)
	}
	file, err := root.Open(activeDirectiveMarkerName)
	if err != nil {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: open: %w", err)
	}
	fileInfo, statErr := file.Stat()
	if statErr != nil {
		closeErr := file.Close()
		return ActiveDirectiveMarker{}, false, errors.Join(fmt.Errorf("read active directive: stat: %w", statErr), closeErr)
	}
	if fileInfo == nil || !fileInfo.Mode().IsRegular() || !os.SameFile(info, fileInfo) {
		closeErr := file.Close()
		return ActiveDirectiveMarker{}, false, errors.Join(fmt.Errorf("read active directive: marker changed or is not regular: %w", ErrInvalidActiveDirectiveMarker), closeErr)
	}
	bounded := make([]byte, activeDirectiveMaxBytes+1)
	length, readErr := io.ReadFull(file, bounded)
	if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
		bounded = bounded[:length]
		readErr = nil
	}
	closeErr := file.Close()
	if readErr != nil {
		return ActiveDirectiveMarker{}, false, errors.Join(fmt.Errorf("read active directive: read: %w", readErr), closeErr)
	}
	if closeErr != nil {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: close: %w", closeErr)
	}
	if length > activeDirectiveMaxBytes {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: marker exceeds %d bytes: %w", activeDirectiveMaxBytes, ErrInvalidActiveDirectiveMarker)
	}
	decoded, err := decodeActiveDirectiveMarker(bounded)
	if err != nil {
		return ActiveDirectiveMarker{}, false, fmt.Errorf("read active directive: %w", err)
	}
	return decoded, true, nil
}

// ValidateActiveDirectiveContext verifies the marker's project, intent, and
// state bindings against a freshly read identity/state snapshot.
func ValidateActiveDirectiveContext(marker ActiveDirectiveMarker, context ActiveDirectiveContext) error {
	if err := validateActiveDirectiveMarker(marker); err != nil {
		return err
	}
	if marker.ProjectSHA256 != context.ProjectSHA256 {
		return fmt.Errorf("active directive: project identity mismatch: %w", ErrInvalidActiveDirectiveMarker)
	}
	if (marker.IntentUUID == nil) != (context.IntentUUID == nil) ||
		(marker.IntentUUID != nil && *marker.IntentUUID != *context.IntentUUID) {
		return fmt.Errorf("active directive: intent identity mismatch: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.StatePresent != context.StatePresent || marker.StateSHA256 != context.StateSHA256 {
		return fmt.Errorf("active directive: state snapshot mismatch: %w", ErrInvalidActiveDirectiveMarker)
	}
	return nil
}

func writeActiveDirectiveMarker(root *os.Root, marker ActiveDirectiveMarker, ops activeDirectiveOps) error {
	if root == nil {
		return fmt.Errorf("write active directive: nil root: %w", ErrInvalidActiveDirectiveMarker)
	}
	if err := validateActiveDirectiveMarker(marker); err != nil {
		return fmt.Errorf("write active directive: validate marker: %w", err)
	}
	data, err := marshalActiveDirectiveMarker(marker)
	if err != nil {
		return fmt.Errorf("write active directive: marshal: %w", err)
	}
	data = append(data, '\n')
	if len(data) > activeDirectiveMaxBytes {
		return fmt.Errorf("write active directive: marker exceeds %d bytes: %w", activeDirectiveMaxBytes, ErrInvalidActiveDirectiveMarker)
	}
	ops = completeActiveDirectiveOps(ops)
	if info, lstatErr := ops.lstat(root, activeDirectiveMarkerName); lstatErr == nil {
		if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("write active directive: existing marker is not a regular non-symlink file: %w", ErrInvalidActiveDirectiveMarker)
		}
	} else if !errors.Is(lstatErr, fs.ErrNotExist) {
		return fmt.Errorf("write active directive: inspect existing marker: %w", lstatErr)
	}

	tempName := fmt.Sprintf("%s%d-%d", activeDirectiveTempPrefix, os.Getpid(), activeDirectiveTempSequence.Add(1))
	file, err := ops.open(root, tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write active directive: create temp: %w", err)
	}
	if file == nil {
		return fmt.Errorf("write active directive: create temp returned nil file: %w", fs.ErrInvalid)
	}
	tempInfo, err := file.Stat()
	if err != nil {
		closeErr := ops.close(file)
		return errors.Join(fmt.Errorf("write active directive: stat temp: %w", err), wrapActiveDirectiveCloseError(closeErr))
	}
	cleanup := func() error {
		return cleanupActiveDirectiveTemp(root, tempName, tempInfo, ops)
	}
	writeErr := writeActiveDirectiveBytes(file, data, ops.write)
	closeErr := ops.close(file)
	if writeErr == nil && closeErr == nil {
		if identityErr := verifyActiveDirectiveTempIdentity(root, tempName, tempInfo, ops.lstat); identityErr != nil {
			writeErr = identityErr
		} else if chmodErr := ops.chmod(root, tempName, 0o600); chmodErr != nil {
			writeErr = fmt.Errorf("chmod temp: %w", chmodErr)
		} else if identityErr := verifyActiveDirectiveTempIdentity(root, tempName, tempInfo, ops.lstat); identityErr != nil {
			writeErr = identityErr
		}
	}
	if writeErr != nil || closeErr != nil {
		cleanupErr := cleanup()
		return errors.Join(writeErr, wrapActiveDirectiveCloseError(closeErr), wrapActiveDirectiveCleanupError(cleanupErr))
	}
	if err := ops.rename(root, tempName, activeDirectiveMarkerName); err != nil {
		cleanupErr := cleanup()
		return errors.Join(fmt.Errorf("write active directive: rename: %w", err), wrapActiveDirectiveCleanupError(cleanupErr))
	}
	return nil
}

func cleanupActiveDirectiveTemp(root *os.Root, tempName string, expected fs.FileInfo, ops activeDirectiveOps) error {
	if expected == nil {
		return nil
	}
	actual, err := ops.lstat(root, tempName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect temporary for cleanup: %w", err)
	}
	if actual == nil || actual.Mode()&fs.ModeSymlink != 0 || !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return nil
	}
	if err := ops.remove(root, tempName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func verifyActiveDirectiveTempIdentity(
	root *os.Root,
	tempName string,
	expected fs.FileInfo,
	lstat func(*os.Root, string) (fs.FileInfo, error),
) error {
	if expected == nil || expected.Mode()&fs.ModeSymlink != 0 || !expected.Mode().IsRegular() {
		return fmt.Errorf("write active directive: temporary descriptor is not regular: %w", ErrInvalidActiveDirectiveMarker)
	}
	actual, err := lstat(root, tempName)
	if err != nil {
		return fmt.Errorf("write active directive: inspect temporary identity: %w", err)
	}
	if actual == nil || actual.Mode()&fs.ModeSymlink != 0 || !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return fmt.Errorf("write active directive: temporary file identity changed: %w", ErrInvalidActiveDirectiveMarker)
	}
	return nil
}

func completeActiveDirectiveOps(ops activeDirectiveOps) activeDirectiveOps {
	system := systemActiveDirectiveOps()
	if ops.lstat == nil {
		ops.lstat = system.lstat
	}
	if ops.open == nil {
		ops.open = system.open
	}
	if ops.write == nil {
		ops.write = system.write
	}
	if ops.close == nil {
		ops.close = system.close
	}
	if ops.rename == nil {
		ops.rename = system.rename
	}
	if ops.remove == nil {
		ops.remove = system.remove
	}
	if ops.chmod == nil {
		ops.chmod = system.chmod
	}
	return ops
}

func writeActiveDirectiveBytes(file *os.File, data []byte, write func(*os.File, []byte) (int, error)) error {
	for len(data) > 0 {
		written, err := write(file, data)
		if written < 0 || written > len(data) {
			return fmt.Errorf("write active directive: invalid short write count %d: %w", written, io.ErrShortWrite)
		}
		if written == 0 {
			return fmt.Errorf("write active directive: short write: %w", io.ErrShortWrite)
		}
		data = data[written:]
		if err != nil {
			return fmt.Errorf("write active directive: write: %w", err)
		}
	}
	return nil
}

func wrapActiveDirectiveCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("write active directive: close: %w", err)
}

func wrapActiveDirectiveCleanupError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("write active directive: cleanup temp: %w", err)
}

type activeDirectiveMarkerWire struct {
	Version                         int                         `json:"version"`
	Stage                           string                      `json:"stage"`
	Unit                            string                      `json:"unit,omitempty"`
	StateSHA256                     string                      `json:"state_sha256"`
	Units                           []string                    `json:"units,omitempty"`
	Revision                        int                         `json:"revision"`
	ProjectSHA256                   string                      `json:"project_sha256"`
	IntentUUID                      *string                     `json:"intent_uuid"`
	StatePresent                    bool                        `json:"state_present"`
	CodeGenerationSourceSHA256      string                      `json:"code_generation_source_sha256,omitempty"`
	CodeGenerationAuthorityRevision int                         `json:"code_generation_authority_revision,omitempty"`
	CursorHarness                   string                      `json:"cursor_harness,omitempty"`
	OwnerSession                    string                      `json:"owner_session"`
	OwnerEpoch                      int                         `json:"owner_epoch"`
	ContextEpoch                    int                         `json:"context_epoch"`
	Kind                            ActiveDirectiveKind         `json:"kind"`
	Part                            int                         `json:"part,omitempty"`
	Parts                           int                         `json:"parts,omitempty"`
	ContinueToken                   string                      `json:"continue_token,omitempty"`
	ContinueTokenSHA256             string                      `json:"continue_token_sha256,omitempty"`
	Delivery                        ActiveDirectiveDelivery     `json:"delivery"`
	NeedsRehydrate                  bool                        `json:"needs_rehydrate"`
	ActiveAttempt                   *activeDirectiveAttemptWire `json:"active_attempt"`
	Resume                          *activeDirectiveResumeWire  `json:"resume,omitempty"`
	EventSequence                   int                         `json:"event_sequence"`
	HumanSequence                   int                         `json:"human_sequence"`
	EngineSequence                  int                         `json:"engine_sequence"`
	ConversationSequence            int                         `json:"conversation_sequence"`
	StopFingerprint                 string                      `json:"stop_fingerprint,omitempty"`
	StopCount                       int                         `json:"stop_count"`
}

type activeDirectiveAttemptWire struct {
	ID                 string                       `json:"id,omitempty"`
	CommandKind        ActiveDirectiveCommandKind   `json:"command_kind"`
	CommandSHA256      string                       `json:"command_sha256"`
	IssuedStateSHA256  string                       `json:"issued_state_sha256"`
	SessionID          string                       `json:"session_id"`
	OwnerEpoch         int                          `json:"owner_epoch"`
	ContextEpoch       int                          `json:"context_epoch"`
	Status             ActiveDirectiveAttemptStatus `json:"status"`
	ClaimRevision      int                          `json:"claim_revision,omitempty"`
	SharedAttempt      bool                         `json:"shared_attempt,omitempty"`
	CursorInputSHA256  string                       `json:"cursor_input_sha256,omitempty"`
	ResultSHA256       string                       `json:"result_sha256,omitempty"`
	ResultRevision     int                          `json:"result_revision,omitempty"`
	ResumeRequest      bool                         `json:"resume_request,omitempty"`
	ResumeAction       ActiveDirectiveResumeAction  `json:"resume_action,omitempty"`
	ResumeGateRevision int                          `json:"resume_gate_revision,omitempty"`
}

type activeDirectiveResumeWire struct {
	Status             ActiveDirectiveResumeStatus `json:"status"`
	IssuingStage       string                      `json:"issuing_stage"`
	IssuingStateSHA256 string                      `json:"issuing_state_sha256"`
	IssuingSession     string                      `json:"issuing_session"`
	IssuingIntentUUID  *string                     `json:"issuing_intent_uuid"`
	Action             ActiveDirectiveResumeAction `json:"action,omitempty"`
}

func (marker ActiveDirectiveMarker) toWire() activeDirectiveMarkerWire {
	wire := activeDirectiveMarkerWire{
		Version:                         marker.Version,
		Stage:                           marker.Stage,
		Unit:                            marker.Unit,
		StateSHA256:                     marker.StateSHA256,
		Units:                           marker.Units,
		Revision:                        marker.Revision,
		ProjectSHA256:                   marker.ProjectSHA256,
		IntentUUID:                      marker.IntentUUID,
		StatePresent:                    marker.StatePresent,
		CodeGenerationSourceSHA256:      marker.CodeGenerationSourceSHA256,
		CodeGenerationAuthorityRevision: marker.CodeGenerationAuthorityRevision,
		CursorHarness:                   marker.CursorHarness,
		OwnerSession:                    marker.OwnerSession,
		OwnerEpoch:                      marker.OwnerEpoch,
		ContextEpoch:                    marker.ContextEpoch,
		Kind:                            marker.Kind,
		Part:                            marker.Part,
		Parts:                           marker.Parts,
		ContinueToken:                   marker.ContinueToken,
		ContinueTokenSHA256:             marker.ContinueTokenSHA256,
		Delivery:                        marker.Delivery,
		NeedsRehydrate:                  marker.NeedsRehydrate,
		EventSequence:                   marker.EventSequence,
		HumanSequence:                   marker.HumanSequence,
		EngineSequence:                  marker.EngineSequence,
		ConversationSequence:            marker.ConversationSequence,
		StopFingerprint:                 marker.StopFingerprint,
		StopCount:                       marker.StopCount,
	}
	if marker.ActiveAttempt != nil {
		value := marker.ActiveAttempt
		wire.ActiveAttempt = &activeDirectiveAttemptWire{
			ID:                 value.ID,
			CommandKind:        value.CommandKind,
			CommandSHA256:      value.CommandSHA256,
			IssuedStateSHA256:  value.IssuedStateSHA256,
			SessionID:          value.SessionID,
			OwnerEpoch:         value.OwnerEpoch,
			ContextEpoch:       value.ContextEpoch,
			Status:             value.Status,
			ClaimRevision:      value.ClaimRevision,
			SharedAttempt:      value.SharedAttempt,
			CursorInputSHA256:  value.CursorInputSHA256,
			ResultSHA256:       value.ResultSHA256,
			ResultRevision:     value.ResultRevision,
			ResumeRequest:      value.ResumeRequest,
			ResumeAction:       value.ResumeAction,
			ResumeGateRevision: value.ResumeGateRevision,
		}
	}
	if marker.Resume != nil {
		value := marker.Resume
		wire.Resume = &activeDirectiveResumeWire{
			Status:             value.Status,
			IssuingStage:       value.IssuingStage,
			IssuingStateSHA256: value.IssuingStateSHA256,
			IssuingSession:     value.IssuingSession,
			IssuingIntentUUID:  value.IssuingIntentUUID,
			Action:             value.Action,
		}
	}
	return wire
}

func marshalActiveDirectiveMarker(marker ActiveDirectiveMarker) ([]byte, error) {
	if err := validateActiveDirectiveMarker(marker); err != nil {
		return nil, err
	}
	if marker.Version == 1 {
		fields := make(map[string]json.RawMessage, 4)
		if err := addActiveDirectiveJSONField(fields, "version", marker.Version); err != nil {
			return nil, err
		}
		if err := addActiveDirectiveJSONField(fields, "stage", marker.Stage); err != nil {
			return nil, err
		}
		if marker.Unit != "" {
			if err := addActiveDirectiveJSONField(fields, "unit", marker.Unit); err != nil {
				return nil, err
			}
		}
		if err := addActiveDirectiveJSONField(fields, "state_sha256", marker.StateSHA256); err != nil {
			return nil, err
		}
		return json.Marshal(fields)
	}

	fields := make(map[string]json.RawMessage, len(activeDirectiveKnownFields())+len(marker.Extra))
	for key, value := range map[string]any{
		"version":               marker.Version,
		"stage":                 marker.Stage,
		"state_sha256":          marker.StateSHA256,
		"revision":              marker.Revision,
		"project_sha256":        marker.ProjectSHA256,
		"intent_uuid":           marker.IntentUUID,
		"state_present":         marker.StatePresent,
		"owner_session":         marker.OwnerSession,
		"owner_epoch":           marker.OwnerEpoch,
		"context_epoch":         marker.ContextEpoch,
		"kind":                  marker.Kind,
		"delivery":              marker.Delivery,
		"needs_rehydrate":       marker.NeedsRehydrate,
		"event_sequence":        marker.EventSequence,
		"human_sequence":        marker.HumanSequence,
		"engine_sequence":       marker.EngineSequence,
		"conversation_sequence": marker.ConversationSequence,
		"stop_count":            marker.StopCount,
	} {
		if err := addActiveDirectiveJSONField(fields, key, value); err != nil {
			return nil, err
		}
	}
	if marker.ActiveAttempt == nil {
		return nil, fmt.Errorf("marker active attempt is required: %w", ErrInvalidActiveDirectiveMarker)
	}
	attempt, err := marshalActiveDirectiveAttempt(*marker.ActiveAttempt)
	if err != nil {
		return nil, err
	}
	fields["active_attempt"] = attempt

	optional := []struct {
		key     string
		include bool
		value   any
	}{
		{"unit", marker.Unit != "" || marker.present["unit"], marker.Unit},
		{"units", len(marker.Units) > 0 || marker.present["units"], marker.Units},
		{"code_generation_source_sha256", marker.CodeGenerationSourceSHA256 != "" || marker.present["code_generation_source_sha256"], marker.CodeGenerationSourceSHA256},
		{"code_generation_authority_revision", marker.CodeGenerationAuthorityRevision != 0 || marker.present["code_generation_authority_revision"], marker.CodeGenerationAuthorityRevision},
		{"cursor_harness", marker.CursorHarness != "" || marker.present["cursor_harness"], marker.CursorHarness},
		{"part", marker.Part != 0 || marker.present["part"], marker.Part},
		{"parts", marker.Parts != 0 || marker.present["parts"], marker.Parts},
		{"continue_token", marker.ContinueToken != "" || marker.present["continue_token"], marker.ContinueToken},
		{"continue_token_sha256", marker.ContinueTokenSHA256 != "" || marker.present["continue_token_sha256"], marker.ContinueTokenSHA256},
		{"stop_fingerprint", marker.StopFingerprint != "" || marker.present["stop_fingerprint"], marker.StopFingerprint},
	}
	for _, field := range optional {
		if !field.include {
			continue
		}
		if err := addActiveDirectiveJSONField(fields, field.key, field.value); err != nil {
			return nil, err
		}
	}
	if marker.Resume != nil {
		resume, err := marshalActiveDirectiveResume(*marker.Resume)
		if err != nil {
			return nil, err
		}
		fields["resume"] = resume
	} else if marker.present["resume"] {
		fields["resume"] = json.RawMessage("null")
	}
	for key, value := range marker.Extra {
		if isActiveDirectiveKnownField(key) {
			continue
		}
		if !json.Valid(value) {
			return nil, fmt.Errorf("marker extra field %q is invalid: %w", key, ErrInvalidActiveDirectiveMarker)
		}
		fields[key] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(fields)
}

func addActiveDirectiveJSONField(fields map[string]json.RawMessage, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marker field %q cannot be encoded: %w", key, err)
	}
	fields[key] = data
	return nil
}

func marshalActiveDirectiveAttempt(attempt ActiveDirectiveAttempt) ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(activeDirectiveKnownAttemptFields())+len(attempt.Extra))
	for key, value := range map[string]any{
		"command_kind":        attempt.CommandKind,
		"command_sha256":      attempt.CommandSHA256,
		"issued_state_sha256": attempt.IssuedStateSHA256,
		"session_id":          attempt.SessionID,
		"owner_epoch":         attempt.OwnerEpoch,
		"context_epoch":       attempt.ContextEpoch,
		"status":              attempt.Status,
	} {
		if err := addActiveDirectiveJSONField(fields, key, value); err != nil {
			return nil, err
		}
	}
	optional := []struct {
		key     string
		include bool
		value   any
	}{
		{"id", attempt.ID != "" || attempt.present["id"], attempt.ID},
		{"claim_revision", attempt.ClaimRevision != 0 || attempt.present["claim_revision"], attempt.ClaimRevision},
		{"shared_attempt", attempt.SharedAttempt || attempt.present["shared_attempt"], attempt.SharedAttempt},
		{"cursor_input_sha256", attempt.CursorInputSHA256 != "" || attempt.present["cursor_input_sha256"], attempt.CursorInputSHA256},
		{"result_sha256", attempt.ResultSHA256 != "" || attempt.present["result_sha256"], attempt.ResultSHA256},
		{"result_revision", attempt.ResultRevision != 0 || attempt.present["result_revision"], attempt.ResultRevision},
		{"resume_request", attempt.ResumeRequest || attempt.present["resume_request"], attempt.ResumeRequest},
		{"resume_action", attempt.ResumeAction != "" || attempt.present["resume_action"], attempt.ResumeAction},
		{"resume_gate_revision", attempt.ResumeGateRevision != 0 || attempt.present["resume_gate_revision"], attempt.ResumeGateRevision},
	}
	for _, field := range optional {
		if !field.include {
			continue
		}
		if err := addActiveDirectiveJSONField(fields, field.key, field.value); err != nil {
			return nil, err
		}
	}
	for key, value := range attempt.Extra {
		if isActiveDirectiveKnownAttemptField(key) {
			continue
		}
		if !json.Valid(value) {
			return nil, fmt.Errorf("marker active attempt extra field %q is invalid: %w", key, ErrInvalidActiveDirectiveMarker)
		}
		fields[key] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(fields)
}

func marshalActiveDirectiveResume(resume ActiveDirectiveResume) ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(activeDirectiveKnownResumeFields())+len(resume.Extra))
	for key, value := range map[string]any{
		"status":               resume.Status,
		"issuing_stage":        resume.IssuingStage,
		"issuing_state_sha256": resume.IssuingStateSHA256,
		"issuing_session":      resume.IssuingSession,
		"issuing_intent_uuid":  resume.IssuingIntentUUID,
	} {
		if err := addActiveDirectiveJSONField(fields, key, value); err != nil {
			return nil, err
		}
	}
	if resume.Action != "" || resume.present["action"] {
		if err := addActiveDirectiveJSONField(fields, "action", resume.Action); err != nil {
			return nil, err
		}
	}
	for key, value := range resume.Extra {
		if isActiveDirectiveKnownResumeField(key) {
			continue
		}
		if !json.Valid(value) {
			return nil, fmt.Errorf("marker resume extra field %q is invalid: %w", key, ErrInvalidActiveDirectiveMarker)
		}
		fields[key] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(fields)
}

func decodeActiveDirectiveMarker(data []byte) (ActiveDirectiveMarker, error) {
	if len(data) == 0 || len(data) > activeDirectiveMaxBytes {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker size is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if !utf8.Valid(data) {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker is not valid UTF-8: %w", ErrInvalidActiveDirectiveMarker)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	var raw map[string]json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker JSON is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if raw == nil {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker JSON is not an object: %w", ErrInvalidActiveDirectiveMarker)
	}
	var extraToken json.RawMessage
	if err := decoder.Decode(&extraToken); !errors.Is(err, io.EOF) {
		if err == nil {
			return ActiveDirectiveMarker{}, fmt.Errorf("marker contains multiple JSON values: %w", ErrInvalidActiveDirectiveMarker)
		}
		return ActiveDirectiveMarker{}, fmt.Errorf("marker has trailing JSON: %w", ErrInvalidActiveDirectiveMarker)
	}
	var version int
	versionRaw, ok := raw["version"]
	if !ok || json.Unmarshal(versionRaw, &version) != nil {
		return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("version", ErrInvalidActiveDirectiveMarker)
	}
	if version == 1 {
		if err := validateActiveDirectiveRawFields(raw, version); err != nil {
			return ActiveDirectiveMarker{}, err
		}
		var stage, unit, stateHash string
		if err := json.Unmarshal(raw["stage"], &stage); err != nil {
			return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("stage", ErrInvalidActiveDirectiveMarker)
		}
		if err := json.Unmarshal(raw["state_sha256"], &stateHash); err != nil {
			return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("state_sha256", ErrInvalidActiveDirectiveMarker)
		}
		if unitRaw, ok := raw["unit"]; ok {
			if err := json.Unmarshal(unitRaw, &unit); err != nil {
				return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("unit", ErrInvalidActiveDirectiveMarker)
			}
		}
		marker := ActiveDirectiveMarker{
			Version:     1,
			Stage:       trimActiveDirectiveText(stage),
			Unit:        trimActiveDirectiveText(unit),
			StateSHA256: stateHash,
		}
		if err := validateActiveDirectiveMarker(marker); err != nil {
			return ActiveDirectiveMarker{}, err
		}
		return marker, nil
	}
	wireBytes, err := json.Marshal(raw)
	if err != nil {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker object encoding failed: %w", ErrInvalidActiveDirectiveMarker)
	}
	var wire activeDirectiveMarkerWire
	if err := json.Unmarshal(wireBytes, &wire); err != nil {
		return ActiveDirectiveMarker{}, fmt.Errorf("marker fields are invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if err := validateActiveDirectiveRawFields(raw, wire.Version); err != nil {
		return ActiveDirectiveMarker{}, err
	}
	marker := ActiveDirectiveMarker{
		Version:                         wire.Version,
		Stage:                           trimActiveDirectiveText(wire.Stage),
		Unit:                            trimActiveDirectiveText(wire.Unit),
		StateSHA256:                     wire.StateSHA256,
		Units:                           append([]string(nil), wire.Units...),
		Revision:                        wire.Revision,
		ProjectSHA256:                   wire.ProjectSHA256,
		IntentUUID:                      wire.IntentUUID,
		StatePresent:                    wire.StatePresent,
		CodeGenerationSourceSHA256:      wire.CodeGenerationSourceSHA256,
		CodeGenerationAuthorityRevision: wire.CodeGenerationAuthorityRevision,
		CursorHarness:                   wire.CursorHarness,
		OwnerSession:                    wire.OwnerSession,
		OwnerEpoch:                      wire.OwnerEpoch,
		ContextEpoch:                    wire.ContextEpoch,
		Kind:                            wire.Kind,
		Part:                            wire.Part,
		Parts:                           wire.Parts,
		ContinueToken:                   wire.ContinueToken,
		ContinueTokenSHA256:             wire.ContinueTokenSHA256,
		Delivery:                        wire.Delivery,
		NeedsRehydrate:                  wire.NeedsRehydrate,
		EventSequence:                   wire.EventSequence,
		HumanSequence:                   wire.HumanSequence,
		EngineSequence:                  wire.EngineSequence,
		ConversationSequence:            wire.ConversationSequence,
		StopFingerprint:                 wire.StopFingerprint,
		StopCount:                       wire.StopCount,
	}
	if wire.ActiveAttempt != nil {
		value := wire.ActiveAttempt
		marker.ActiveAttempt = &ActiveDirectiveAttempt{
			ID:                 value.ID,
			CommandKind:        value.CommandKind,
			CommandSHA256:      value.CommandSHA256,
			IssuedStateSHA256:  value.IssuedStateSHA256,
			SessionID:          value.SessionID,
			OwnerEpoch:         value.OwnerEpoch,
			ContextEpoch:       value.ContextEpoch,
			Status:             value.Status,
			ClaimRevision:      value.ClaimRevision,
			SharedAttempt:      value.SharedAttempt,
			CursorInputSHA256:  value.CursorInputSHA256,
			ResultSHA256:       value.ResultSHA256,
			ResultRevision:     value.ResultRevision,
			ResumeRequest:      value.ResumeRequest,
			ResumeAction:       value.ResumeAction,
			ResumeGateRevision: value.ResumeGateRevision,
		}
		var rawAttempt map[string]json.RawMessage
		if err := json.Unmarshal(raw["active_attempt"], &rawAttempt); err != nil || rawAttempt == nil {
			return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("active_attempt", ErrInvalidActiveDirectiveMarker)
		}
		marker.ActiveAttempt.Extra = activeDirectiveUnknownFields(rawAttempt, activeDirectiveKnownAttemptFields())
		marker.ActiveAttempt.present = activeDirectiveZeroPresence(rawAttempt,
			[]string{"id", "resume_action"},
			[]string{"claim_revision", "result_revision", "resume_gate_revision"},
			[]string{"shared_attempt", "resume_request"},
		)
	}
	if wire.Resume != nil {
		value := wire.Resume
		marker.Resume = &ActiveDirectiveResume{
			Status:             value.Status,
			IssuingStage:       value.IssuingStage,
			IssuingStateSHA256: value.IssuingStateSHA256,
			IssuingSession:     value.IssuingSession,
			IssuingIntentUUID:  value.IssuingIntentUUID,
			Action:             value.Action,
		}
		var rawResume map[string]json.RawMessage
		if err := json.Unmarshal(raw["resume"], &rawResume); err != nil || rawResume == nil {
			return ActiveDirectiveMarker{}, invalidActiveDirectiveRawField("resume", ErrInvalidActiveDirectiveMarker)
		}
		marker.Resume.Extra = activeDirectiveUnknownFields(rawResume, activeDirectiveKnownResumeFields())
		marker.Resume.present = activeDirectiveZeroPresence(rawResume, []string{"action"}, nil, nil)
	}
	marker.Extra = activeDirectiveUnknownFields(raw, activeDirectiveKnownFields())
	marker.present = activeDirectiveZeroPresence(raw,
		[]string{"unit", "code_generation_source_sha256", "cursor_harness", "continue_token", "continue_token_sha256", "stop_fingerprint"},
		[]string{"code_generation_authority_revision", "part", "parts"},
		nil,
	)
	if resume, ok := raw["resume"]; ok && bytes.Equal(bytes.TrimSpace(resume), []byte("null")) {
		if marker.present == nil {
			marker.present = make(map[string]bool)
		}
		marker.present["resume"] = true
	}
	if err := validateActiveDirectiveMarker(marker); err != nil {
		return ActiveDirectiveMarker{}, err
	}
	return marker, nil
}

func trimActiveDirectiveText(value string) string {
	return strings.TrimFunc(value, func(r rune) bool {
		switch r {
		case '\ufeff':
			return true
		case '\u0085':
			return false
		default:
			return unicode.IsSpace(r)
		}
	})
}

func activeDirectiveUnknownFields(raw map[string]json.RawMessage, known map[string]bool) map[string]json.RawMessage {
	var extra map[string]json.RawMessage
	for key, value := range raw {
		if known[key] {
			continue
		}
		extra = appendActiveDirectiveExtra(extra, key, value)
	}
	return extra
}

func activeDirectiveZeroPresence(
	raw map[string]json.RawMessage,
	stringFields, numberFields, boolFields []string,
) map[string]bool {
	var present map[string]bool
	for _, field := range stringFields {
		var value string
		if rawValue, ok := raw[field]; ok && activeDirectiveRawKind(rawValue) == 's' && json.Unmarshal(rawValue, &value) == nil && value == "" {
			if present == nil {
				present = make(map[string]bool)
			}
			present[field] = true
		}
	}
	for _, field := range numberFields {
		var value int64
		if rawValue, ok := raw[field]; ok && activeDirectiveRawInteger(rawValue) && json.Unmarshal(rawValue, &value) == nil && value == 0 {
			if present == nil {
				present = make(map[string]bool)
			}
			present[field] = true
		}
	}
	for _, field := range boolFields {
		var value bool
		if rawValue, ok := raw[field]; ok && activeDirectiveRawKind(rawValue) == 'b' && json.Unmarshal(rawValue, &value) == nil && !value {
			if present == nil {
				present = make(map[string]bool)
			}
			present[field] = true
		}
	}
	return present
}

func validateActiveDirectiveRawFields(raw map[string]json.RawMessage, version int) error {
	if version == 1 {
		if err := requireActiveDirectiveRawField(raw, "version", 'n'); err != nil {
			return err
		}
		if err := requireActiveDirectiveRawField(raw, "stage", 's'); err != nil {
			return err
		}
		if err := requireActiveDirectiveRawField(raw, "state_sha256", 's'); err != nil {
			return err
		}
		if err := validateActiveDirectiveOptionalRawField(raw, "unit", 's'); err != nil {
			return err
		}
		return nil
	}
	if version != 2 {
		return nil
	}

	for _, field := range []string{
		"version", "stage", "state_sha256", "project_sha256", "intent_uuid", "state_present",
		"revision", "owner_session", "owner_epoch", "context_epoch", "kind", "delivery",
		"needs_rehydrate", "active_attempt", "event_sequence", "human_sequence", "engine_sequence",
		"conversation_sequence", "stop_count",
	} {
		kind := byte('s')
		switch field {
		case "version", "revision", "owner_epoch", "context_epoch", "event_sequence", "human_sequence", "engine_sequence", "conversation_sequence", "stop_count":
			kind = 'n'
		case "intent_uuid":
			kind = 'q'
		case "state_present", "needs_rehydrate":
			kind = 'b'
		case "active_attempt":
			kind = 'o'
		}
		if err := requireActiveDirectiveRawField(raw, field, kind); err != nil {
			return err
		}
	}
	for _, field := range []string{
		"unit", "code_generation_source_sha256", "cursor_harness", "continue_token", "continue_token_sha256", "stop_fingerprint",
	} {
		if err := validateActiveDirectiveOptionalRawField(raw, field, 's'); err != nil {
			return err
		}
	}
	for _, field := range []string{"code_generation_authority_revision", "part", "parts"} {
		if err := validateActiveDirectiveOptionalRawField(raw, field, 'n'); err != nil {
			return err
		}
	}
	if units, ok := raw["units"]; ok {
		if activeDirectiveRawKind(units) != 'a' {
			return invalidActiveDirectiveRawField("units", ErrInvalidActiveDirectiveMarker)
		}
		var values []json.RawMessage
		if err := json.Unmarshal(units, &values); err != nil || len(values) == 0 {
			return invalidActiveDirectiveRawField("units", ErrInvalidActiveDirectiveMarker)
		}
		for _, value := range values {
			if activeDirectiveRawKind(value) != 's' {
				return invalidActiveDirectiveRawField("units", ErrInvalidActiveDirectiveMarker)
			}
		}
	}

	var attempt map[string]json.RawMessage
	if err := json.Unmarshal(raw["active_attempt"], &attempt); err != nil || attempt == nil {
		return invalidActiveDirectiveRawField("active_attempt", ErrInvalidActiveDirectiveMarker)
	}
	for _, field := range []string{"command_kind", "command_sha256", "issued_state_sha256", "session_id", "status"} {
		if err := requireActiveDirectiveRawField(attempt, field, 's'); err != nil {
			return err
		}
	}
	for _, field := range []string{"owner_epoch", "context_epoch"} {
		if err := requireActiveDirectiveRawField(attempt, field, 'n'); err != nil {
			return err
		}
	}
	for _, field := range []string{"id", "cursor_input_sha256", "result_sha256", "resume_action"} {
		if err := validateActiveDirectiveOptionalRawField(attempt, field, 's'); err != nil {
			return err
		}
	}
	for _, field := range []string{"claim_revision", "result_revision", "resume_gate_revision"} {
		if err := validateActiveDirectiveOptionalRawField(attempt, field, 'n'); err != nil {
			return err
		}
	}
	for _, field := range []string{"shared_attempt", "resume_request"} {
		if err := validateActiveDirectiveOptionalRawField(attempt, field, 'b'); err != nil {
			return err
		}
	}

	if resume, ok := raw["resume"]; ok && activeDirectiveRawKind(resume) != 'z' {
		var value map[string]json.RawMessage
		if activeDirectiveRawKind(resume) != 'o' || json.Unmarshal(resume, &value) != nil || value == nil {
			return invalidActiveDirectiveRawField("resume", ErrInvalidActiveDirectiveMarker)
		}
		for _, field := range []string{"status", "issuing_stage", "issuing_state_sha256", "issuing_session"} {
			if err := requireActiveDirectiveRawField(value, field, 's'); err != nil {
				return err
			}
		}
		if err := requireActiveDirectiveRawField(value, "issuing_intent_uuid", 'q'); err != nil {
			return err
		}
		if err := validateActiveDirectiveOptionalRawField(value, "action", 's'); err != nil {
			return err
		}
	}
	return nil
}

func requireActiveDirectiveRawField(raw map[string]json.RawMessage, field string, expected byte) error {
	value, ok := raw[field]
	if !ok || !activeDirectiveRawMatches(value, expected) {
		return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
	}
	if expected == 'n' && !activeDirectiveRawInteger(value) {
		return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
	}
	return nil
}

func validateActiveDirectiveOptionalRawField(raw map[string]json.RawMessage, field string, expected byte) error {
	value, ok := raw[field]
	if !ok {
		return nil
	}
	if !activeDirectiveRawMatches(value, expected) || (expected == 'n' && !activeDirectiveRawInteger(value)) {
		return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
	}
	if expected == 's' && field == "unit" {
		var text string
		if json.Unmarshal(value, &text) != nil || trimActiveDirectiveText(text) == "" {
			return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
		}
	}
	if expected == 's' {
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
		}
		switch field {
		case "code_generation_source_sha256":
			if !activeDirectiveSourceSHA256Pattern.MatchString(text) {
				return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
			}
		case "cursor_harness":
			if !activeDirectiveHarnessPattern.MatchString(text) {
				return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
			}
		case "cursor_input_sha256", "result_sha256":
			if !activeDirectiveSHA256Pattern.MatchString(text) {
				return invalidActiveDirectiveRawField(field, ErrInvalidActiveDirectiveMarker)
			}
		}
	}
	return nil
}

func activeDirectiveRawMatches(value json.RawMessage, expected byte) bool {
	actual := activeDirectiveRawKind(value)
	if expected == 'q' {
		return actual == 's' || actual == 'z'
	}
	return actual == expected
}

func activeDirectiveRawKind(value json.RawMessage) byte {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}
	switch value[0] {
	case '"':
		return 's'
	case '{':
		return 'o'
	case '[':
		return 'a'
	case 't', 'f':
		return 'b'
	case 'n':
		return 'z'
	default:
		return 'n'
	}
}

func activeDirectiveRawInteger(value json.RawMessage) bool {
	if activeDirectiveRawKind(value) != 'n' {
		return false
	}
	var number json.Number
	if err := json.Unmarshal(value, &number); err != nil {
		return false
	}
	parsed, err := strconv.ParseInt(string(number), 10, 64)
	return err == nil && parsed >= 0
}

func invalidActiveDirectiveRawField(field string, cause error) error {
	return fmt.Errorf("marker field %q has invalid type or presence: %w", field, cause)
}

func appendActiveDirectiveExtra(extra map[string]json.RawMessage, key string, value json.RawMessage) map[string]json.RawMessage {
	if extra == nil {
		extra = make(map[string]json.RawMessage)
	}
	extra[key] = append(json.RawMessage(nil), value...)
	return extra
}

func activeDirectiveKnownFields() map[string]bool {
	return map[string]bool{
		"version": true, "stage": true, "unit": true, "state_sha256": true, "units": true,
		"revision": true, "project_sha256": true, "intent_uuid": true, "state_present": true,
		"code_generation_source_sha256": true, "code_generation_authority_revision": true,
		"cursor_harness": true, "owner_session": true, "owner_epoch": true, "context_epoch": true,
		"kind": true, "part": true, "parts": true, "continue_token": true,
		"continue_token_sha256": true, "delivery": true, "needs_rehydrate": true,
		"active_attempt": true, "resume": true, "event_sequence": true, "human_sequence": true,
		"engine_sequence": true, "conversation_sequence": true, "stop_fingerprint": true,
		"stop_count": true,
	}
}

func activeDirectiveKnownAttemptFields() map[string]bool {
	return map[string]bool{
		"id": true, "command_kind": true, "command_sha256": true, "issued_state_sha256": true,
		"session_id": true, "owner_epoch": true, "context_epoch": true, "status": true,
		"claim_revision": true, "shared_attempt": true, "cursor_input_sha256": true,
		"result_sha256": true, "result_revision": true, "resume_request": true,
		"resume_action": true, "resume_gate_revision": true,
	}
}

func activeDirectiveKnownResumeFields() map[string]bool {
	return map[string]bool{
		"status": true, "issuing_stage": true, "issuing_state_sha256": true,
		"issuing_session": true, "issuing_intent_uuid": true, "action": true,
	}
}

func isActiveDirectiveKnownField(key string) bool { return activeDirectiveKnownFields()[key] }

func isActiveDirectiveKnownAttemptField(key string) bool {
	return activeDirectiveKnownAttemptFields()[key]
}

func isActiveDirectiveKnownResumeField(key string) bool {
	return activeDirectiveKnownResumeFields()[key]
}

func validateActiveDirectiveMarker(marker ActiveDirectiveMarker) error {
	if marker.Unit != "" && trimActiveDirectiveText(marker.Unit) == "" {
		return fmt.Errorf("marker unit is empty after trim: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.Version == 1 {
		if !activeDirectiveStagePattern.MatchString(marker.Stage) ||
			!activeDirectiveSHA256Pattern.MatchString(marker.StateSHA256) {
			return fmt.Errorf("v1 marker fields are invalid: %w", ErrInvalidActiveDirectiveMarker)
		}
		return nil
	}
	if marker.Version != 2 {
		return fmt.Errorf("marker version %d is unsupported: %w", marker.Version, ErrInvalidActiveDirectiveMarker)
	}
	if !activeDirectiveStagePattern.MatchString(marker.Stage) || !activeDirectiveSHA256Pattern.MatchString(marker.StateSHA256) {
		return fmt.Errorf("marker stage or state hash is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if len(marker.Units) > 0 {
		for _, unit := range marker.Units {
			if !activeDirectiveUnitPattern.MatchString(trimActiveDirectiveText(unit)) {
				return fmt.Errorf("marker units contain an invalid name: %w", ErrInvalidActiveDirectiveMarker)
			}
		}
	}
	if !activeDirectiveSHA256Pattern.MatchString(marker.ProjectSHA256) {
		return fmt.Errorf("marker project hash is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.CodeGenerationSourceSHA256 != "" && !activeDirectiveSourceSHA256Pattern.MatchString(marker.CodeGenerationSourceSHA256) {
		return fmt.Errorf("marker code generation source hash is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.CursorHarness != "" && !activeDirectiveHarnessPattern.MatchString(marker.CursorHarness) {
		return fmt.Errorf("marker cursor harness is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.OwnerSession == "" || marker.Revision < 0 || marker.OwnerEpoch < 0 || marker.ContextEpoch < 0 ||
		marker.EventSequence < 0 || marker.HumanSequence < 0 || marker.EngineSequence < 0 || marker.ConversationSequence < 0 || marker.StopCount < 0 {
		return fmt.Errorf("marker required counters or owner session are invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if !validActiveDirectiveKind(marker.Kind) || !validActiveDirectiveDelivery(marker.Delivery) {
		return fmt.Errorf("marker kind or delivery is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if marker.ActiveAttempt == nil {
		return fmt.Errorf("marker active attempt is required: %w", ErrInvalidActiveDirectiveMarker)
	}
	if err := validateActiveDirectiveAttempt(*marker.ActiveAttempt); err != nil {
		return err
	}
	if marker.Resume != nil {
		if err := validateActiveDirectiveResume(*marker.Resume); err != nil {
			return err
		}
	}
	if marker.ContinueToken != "" || marker.present["continue_token"] {
		if !utf8.ValidString(marker.ContinueToken) || len([]byte(marker.ContinueToken)) > 16*1024 || !activeDirectiveSHA256Equal(marker.ContinueTokenSHA256, marker.ContinueToken) {
			return fmt.Errorf("marker continue token digest is invalid: %w", ErrInvalidActiveDirectiveMarker)
		}
	}
	if marker.Kind == ActiveDirectiveKindLoadSteering {
		if marker.Part < 1 || marker.Parts < marker.Part || marker.ContinueToken == "" {
			return fmt.Errorf("load-steering marker cursor is invalid: %w", ErrInvalidActiveDirectiveMarker)
		}
	}
	return nil
}

func validateActiveDirectiveAttempt(attempt ActiveDirectiveAttempt) error {
	if !validActiveDirectiveCommand(attempt.CommandKind) || !activeDirectiveSHA256Pattern.MatchString(attempt.CommandSHA256) ||
		!activeDirectiveSHA256Pattern.MatchString(attempt.IssuedStateSHA256) || attempt.SessionID == "" ||
		attempt.OwnerEpoch < 0 || attempt.ContextEpoch < 0 || !validActiveDirectiveAttemptStatus(attempt.Status) ||
		attempt.ClaimRevision < 0 || attempt.ResultRevision < 0 || attempt.ResumeGateRevision < 0 {
		return fmt.Errorf("marker active attempt fields are invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if attempt.CursorInputSHA256 != "" && !activeDirectiveSHA256Pattern.MatchString(attempt.CursorInputSHA256) {
		return fmt.Errorf("marker active attempt input hash is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if attempt.ResultSHA256 != "" && !activeDirectiveSHA256Pattern.MatchString(attempt.ResultSHA256) {
		return fmt.Errorf("marker active attempt result hash is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	if attempt.ResumeAction != "" && !validActiveDirectiveResumeAction(attempt.ResumeAction) {
		return fmt.Errorf("marker active attempt resume action is invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	return nil
}

func validateActiveDirectiveResume(resume ActiveDirectiveResume) error {
	if !validActiveDirectiveResumeStatus(resume.Status) || !activeDirectiveStagePattern.MatchString(resume.IssuingStage) ||
		!activeDirectiveSHA256Pattern.MatchString(resume.IssuingStateSHA256) || resume.IssuingSession == "" ||
		(resume.Action != "" && !validActiveDirectiveResumeAction(resume.Action)) {
		return fmt.Errorf("marker resume fields are invalid: %w", ErrInvalidActiveDirectiveMarker)
	}
	return nil
}

func validActiveDirectiveKind(value ActiveDirectiveKind) bool {
	switch value {
	case ActiveDirectiveKindLoadSteering, ActiveDirectiveKindRunStage, ActiveDirectiveKindAsk, ActiveDirectiveKindPrint,
		ActiveDirectiveKindError, ActiveDirectiveKindDone, ActiveDirectiveKindParked, ActiveDirectiveKindNotice,
		ActiveDirectiveKindDispatchSubagent, ActiveDirectiveKindInvokeSwarm, ActiveDirectiveKindPresentGate:
		return true
	default:
		return false
	}
}

func validActiveDirectiveDelivery(value ActiveDirectiveDelivery) bool {
	switch value {
	case ActiveDirectiveDeliveryIssued, ActiveDirectiveDeliveryDelivered, ActiveDirectiveDeliveryConsumed, ActiveDirectiveDeliverySuperseded:
		return true
	default:
		return false
	}
}

func validActiveDirectiveCommand(value ActiveDirectiveCommandKind) bool {
	switch value {
	case ActiveDirectiveCommandNext, ActiveDirectiveCommandContinue, ActiveDirectiveCommandReport, ActiveDirectiveCommandPark:
		return true
	default:
		return false
	}
}

func validActiveDirectiveAttemptStatus(value ActiveDirectiveAttemptStatus) bool {
	switch value {
	case ActiveDirectiveAttemptPending, ActiveDirectiveAttemptSettled, ActiveDirectiveAttemptFailed:
		return true
	default:
		return false
	}
}

func validActiveDirectiveResumeStatus(value ActiveDirectiveResumeStatus) bool {
	switch value {
	case ActiveDirectiveResumeWaiting, ActiveDirectiveResumeSelected, ActiveDirectiveResumeSuperseded:
		return true
	default:
		return false
	}
}

func validActiveDirectiveResumeAction(value ActiveDirectiveResumeAction) bool {
	switch value {
	case ActiveDirectiveResumeActionResume, ActiveDirectiveResumeActionRedo, ActiveDirectiveResumeActionJump, ActiveDirectiveResumeActionStartFresh:
		return true
	default:
		return false
	}
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func activeDirectiveSHA256Equal(expectedHex, value string) bool {
	const digestHexLength = sha256.Size * 2
	if len(expectedHex) != digestHexLength {
		return false
	}
	digest := sha256.Sum256([]byte(value))
	var encoded [digestHexLength]byte
	hex.Encode(encoded[:], digest[:])
	var expected [digestHexLength]byte
	copy(expected[:], expectedHex)
	return subtle.ConstantTimeCompare(expected[:], encoded[:]) == 1
}
