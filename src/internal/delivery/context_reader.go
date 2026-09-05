package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/steering"
)

// ContextReadKind identifies a bounded context response.
type ContextReadKind string

const ContextReadKindChunk ContextReadKind = "context-chunk"

const maxContextReadResponseBytes = 8192

const contextReadChunkBytes = 512

const contextReadStreamBufferBytes = contextReadChunkBytes

const contextReadTokenDomain = "ai-dd/read-context/v1\x00"

// ContextReadSlot identifies the source group of a context file.
type ContextReadSlot string

const (
	ContextReadSlotInline    ContextReadSlot = "inline-context"
	ContextReadSlotStage     ContextReadSlot = "stage-file"
	ContextReadSlotStageFile                 = ContextReadSlotStage
	ContextReadSlotConsume   ContextReadSlot = "consume"
)

// ContextReadResult is one bounded context chunk returned to a receiver.
type ContextReadResult struct {
	Kind              ContextReadKind `json:"kind"`
	Stage             string          `json:"stage"`
	Slot              ContextReadSlot `json:"slot"`
	Index             int             `json:"index"`
	Part              int             `json:"part"`
	Parts             int             `json:"parts"`
	ContentSHA256     string          `json:"content_sha256"`
	Text              string          `json:"text"`
	ReadContinueToken string          `json:"read_continue_token,omitempty"`
	Complete          bool            `json:"complete"`
}

var (
	ErrContextRead              = errors.New("delivery: context read failed")
	ErrContextReadBinding       = errors.New("delivery: context is not bound to the active run-stage")
	ErrContextReadInvalidWire   = errors.New("delivery: invalid run-stage context wire")
	ErrContextReadUnsafePath    = errors.New("delivery: unsafe context path")
	ErrContextReadInvalidUTF8   = errors.New("delivery: context is not valid UTF-8")
	ErrContextReadFileChanged   = errors.New("delivery: context file changed during read")
	ErrContextReadToken         = errors.New("delivery: invalid context read token")
	ErrContextReadAbsent        = errors.New("delivery: required context input is unexpectedly absent")
	ErrContextReadNoActiveStage = errors.New("delivery: no active run-stage context")
	contextReadOpen             = func(root *os.Root, name string) (contextReadFile, error) {
		return openContextReadFile(root, name)
	}
)

type contextReadFile interface {
	io.Reader
	io.Seeker
	Stat() (fs.FileInfo, error)
	Close() error
}

type contextRunStageWire struct {
	Kind               string              `json:"kind"`
	Stage              string              `json:"stage"`
	InlineContextPaths []string            `json:"inline_context_paths"`
	StageFile          string              `json:"stage_file"`
	Consumes           []string            `json:"consumes"`
	ConsumesAbsent     []contextReadAbsent `json:"consumes_absent"`
}

type contextReadAbsent struct {
	Path     string `json:"path"`
	Expected bool   `json:"expected"`
}

type contextReadTarget struct {
	Slot         ContextReadSlot
	Index        int
	Path         string
	RelativePath string
	Root         *os.Root
}

type contextReadPlan struct {
	Stage          string
	WireSHA256     string
	MarkerRevision int
	GenerationID   string
	ProjectSHA256  string
	StateSHA256    string
	Targets        []contextReadTarget
}

type contextReadSnapshot struct {
	Data    []byte
	Digest  string
	Parts   int
	Size    int64
	ModTime time.Time
}

type contextReadTokenClaims struct {
	Version        int    `json:"v"`
	GenerationID   string `json:"a"`
	ProjectSHA256  string `json:"p"`
	Space          string `json:"s"`
	Intent         string `json:"i"`
	MarkerRevision int    `json:"r"`
	WireSHA256     string `json:"w"`
	Stage          string `json:"g"`
	Slot           string `json:"o"`
	Index          int    `json:"x"`
	Part           int    `json:"n"`
	Parts          int    `json:"t"`
	Path           string `json:"f"`
	ContentSHA256  string `json:"c"`
	Size           int64  `json:"z"`
	ModTimeUnix    int64  `json:"m"`
}

type contextReadTokenEnvelope struct {
	Payload json.RawMessage `json:"p"`
	MAC     string          `json:"m"`
}

// ReadContext returns the first context chunk for the active run-stage.
func ReadContext(ctx context.Context, input RunStageInput) (ContextReadResult, error) {
	if err := validateDeliveryInput(ctx, input); err != nil {
		return ContextReadResult{}, fmt.Errorf("read context: invalid input: %w", err)
	}
	var result ContextReadResult
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		result, err = readContextWithGuard(ctx, guard, input)
		return err
	})
	if err != nil {
		return ContextReadResult{}, err
	}
	return result, nil
}

// ContinueContext returns the context chunk identified by an opaque read token.
func ContinueContext(ctx context.Context, input RunStageInput, token string) (ContextReadResult, error) {
	if err := validateDeliveryInput(ctx, input); err != nil {
		return ContextReadResult{}, fmt.Errorf("continue context: invalid input: %w", err)
	}
	if token == "" {
		return ContextReadResult{}, fmt.Errorf("continue context: token is required: %w", ErrContextReadToken)
	}
	var result ContextReadResult
	err := recordlock.With(ctx, input.Identity, func(guard *recordlock.Guard) error {
		var err error
		result, err = continueContextWithGuard(ctx, guard, input, token)
		return err
	})
	if err != nil {
		return ContextReadResult{}, err
	}
	return result, nil
}

func readContextWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput) (ContextReadResult, error) {
	composition, err := ComposeRunStageWithGuard(ctx, guard, input)
	if err != nil {
		return ContextReadResult{}, fmt.Errorf("read context: compose run-stage: %w", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(input.RecordRoot)
	if err != nil {
		return ContextReadResult{}, fmt.Errorf("read context: read active marker: %w", err)
	}
	if !found {
		return ContextReadResult{}, fmt.Errorf("read context: %w", ErrContextReadNoActiveStage)
	}
	if err := validateContextRunStageMarker(marker, input, composition); err != nil {
		return ContextReadResult{}, err
	}
	var wire contextRunStageWire
	if err := json.Unmarshal(composition.Wire, &wire); err != nil {
		return ContextReadResult{}, fmt.Errorf("read context: decode run-stage wire: %w: %w", ErrContextReadInvalidWire, err)
	}
	if wire.Kind != string(ActiveDirectiveKindRunStage) || wire.Stage == "" || wire.StageFile == "" {
		return ContextReadResult{}, fmt.Errorf("read context: run-stage wire is incomplete: %w", ErrContextReadInvalidWire)
	}
	if err := preflightContextAbsent(wire.ConsumesAbsent); err != nil {
		return ContextReadResult{}, err
	}
	plan, err := buildContextReadPlan(input, marker, composition, wire)
	if err != nil {
		return ContextReadResult{}, err
	}
	return readContextAt(ctx, input, plan, 0, 1, nil, "", 0, 0, 0)
}

func continueContextWithGuard(ctx context.Context, guard *recordlock.Guard, input RunStageInput, token string) (ContextReadResult, error) {
	composition, err := ComposeRunStageWithGuard(ctx, guard, input)
	if err != nil {
		return ContextReadResult{}, fmt.Errorf("continue context: compose run-stage: %w", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(input.RecordRoot)
	if err != nil {
		return ContextReadResult{}, fmt.Errorf("continue context: read active marker: %w", err)
	}
	if !found {
		return ContextReadResult{}, fmt.Errorf("continue context: %w", ErrContextReadNoActiveStage)
	}
	if err := validateContextRunStageMarker(marker, input, composition); err != nil {
		return ContextReadResult{}, err
	}
	var wire contextRunStageWire
	if err := json.Unmarshal(composition.Wire, &wire); err != nil {
		return ContextReadResult{}, fmt.Errorf("continue context: decode run-stage wire: %w: %w", ErrContextReadInvalidWire, err)
	}
	if wire.Kind != string(ActiveDirectiveKindRunStage) || wire.Stage == "" || wire.StageFile == "" {
		return ContextReadResult{}, fmt.Errorf("continue context: run-stage wire is incomplete: %w", ErrContextReadInvalidWire)
	}
	if err := preflightContextAbsent(wire.ConsumesAbsent); err != nil {
		return ContextReadResult{}, err
	}
	plan, err := buildContextReadPlan(input, marker, composition, wire)
	if err != nil {
		return ContextReadResult{}, err
	}
	key, err := readContextContinuationKey(input.ProjectRoot, input.RecordRoot)
	if err != nil {
		return ContextReadResult{}, err
	}
	claims, err := decodeContextReadToken(key, token)
	if err != nil {
		return ContextReadResult{}, err
	}
	if err := validateContextReadToken(claims, input, plan); err != nil {
		return ContextReadResult{}, err
	}
	targetIndex := contextReadTargetIndex(plan.Targets, claims.Slot, claims.Index)
	if targetIndex < 0 {
		return ContextReadResult{}, fmt.Errorf("continue context: token target is not in current plan: %w", ErrContextReadToken)
	}
	return readContextAt(ctx, input, plan, targetIndex, claims.Part, key, claims.ContentSHA256, claims.Size, claims.ModTimeUnix, claims.Parts)
}

func preflightContextAbsent(absents []contextReadAbsent) error {
	for _, absent := range absents {
		if !absent.Expected {
			return fmt.Errorf("read context: consume %q is unexpectedly absent: %w", absent.Path, ErrContextReadAbsent)
		}
	}
	return nil
}

func buildContextReadPlan(input RunStageInput, marker ActiveDirectiveMarker, composition RunStageComposition, wire contextRunStageWire) (contextReadPlan, error) {
	plan := contextReadPlan{
		Stage:          wire.Stage,
		WireSHA256:     sha256Hex(string(composition.Wire)),
		MarkerRevision: marker.Revision,
		GenerationID:   marker.ActiveAttempt.ID,
		ProjectSHA256:  sha256Hex(input.Identity.ProjectPath()),
	}
	if composition.Freshness.StateHash != nil {
		plan.StateSHA256 = *composition.Freshness.StateHash
	}
	for index, name := range wire.InlineContextPaths {
		if err := addContextReadTarget(&plan, ContextReadSlotInline, index+1, name, name, input.ProjectRoot); err != nil {
			return contextReadPlan{}, fmt.Errorf("read context: inline path %q: %w", name, err)
		}
	}
	if err := addContextReadTarget(&plan, ContextReadSlotStage, 1, wire.StageFile, wire.StageFile, input.ProjectRoot); err != nil {
		return contextReadPlan{}, fmt.Errorf("read context: stage path %q: %w", wire.StageFile, err)
	}
	prefix := path.Join("aidlc", "spaces", input.Identity.Space(), "intents", input.Identity.Intent())
	for index, name := range wire.Consumes {
		if name == "" || !strings.HasPrefix(name, prefix+"/") {
			return contextReadPlan{}, fmt.Errorf("read context: consume path %q is outside active record: %w", name, ErrContextReadUnsafePath)
		}
		relative := strings.TrimPrefix(name, prefix+"/")
		if err := addContextReadTarget(&plan, ContextReadSlotConsume, index+1, name, relative, input.RecordRoot); err != nil {
			return contextReadPlan{}, fmt.Errorf("read context: consume path %q: %w", name, err)
		}
	}
	if len(plan.Targets) == 0 {
		return contextReadPlan{}, fmt.Errorf("read context: no context files are declared: %w", ErrContextReadInvalidWire)
	}
	return plan, nil
}

func addContextReadTarget(plan *contextReadPlan, slot ContextReadSlot, index int, displayPath, relativePath string, root *os.Root) error {
	if root == nil || !validContextRelativePath(relativePath) {
		return fmt.Errorf("path %q: %w", relativePath, ErrContextReadUnsafePath)
	}
	plan.Targets = append(plan.Targets, contextReadTarget{
		Slot:         slot,
		Index:        index,
		Path:         displayPath,
		RelativePath: relativePath,
		Root:         root,
	})
	return nil
}

func readContextAt(ctx context.Context, input RunStageInput, plan contextReadPlan, targetIndex, part int, key []byte, expectedDigest string, expectedSize, expectedModTime int64, expectedParts int) (ContextReadResult, error) {
	if err := contextReadContext(ctx); err != nil {
		return ContextReadResult{}, err
	}
	if targetIndex < 0 || targetIndex >= len(plan.Targets) || part < 1 {
		return ContextReadResult{}, fmt.Errorf("read context: target cursor is invalid: %w", ErrContextReadToken)
	}
	target := plan.Targets[targetIndex]
	if expectedDigest == "" && (expectedSize != 0 || expectedModTime != 0) {
		info, infoErr := contextReadPathInfo(target.Root, target.RelativePath)
		if infoErr != nil {
			return ContextReadResult{}, fmt.Errorf("read context: %s %q changed before read: %w", target.Slot, target.Path, ErrContextReadFileChanged)
		}
		if info.Size() != expectedSize || info.ModTime().UnixNano() != expectedModTime {
			return ContextReadResult{}, fmt.Errorf("read context: %s %q changed before read: %w", target.Slot, target.Path, ErrContextReadFileChanged)
		}
	}
	snapshot, err := readContextSnapshot(ctx, target.Root, target.RelativePath, part)
	if err != nil {
		return ContextReadResult{}, fmt.Errorf("read context: %s %q: %w", target.Slot, target.Path, err)
	}
	if expectedDigest != "" && (expectedDigest != snapshot.Digest || expectedSize != snapshot.Size || expectedModTime != snapshot.ModTime.UnixNano() || expectedParts != snapshot.Parts) {
		return ContextReadResult{}, fmt.Errorf("read context: %s %q content or metadata changed: %w", target.Slot, target.Path, ErrContextReadFileChanged)
	}
	if part > snapshot.Parts {
		return ContextReadResult{}, fmt.Errorf("read context: part %d exceeds %d for %q: %w", part, snapshot.Parts, target.Path, ErrContextReadToken)
	}
	nextTargetIndex, nextPart := nextContextCursor(plan.Targets, targetIndex, part, snapshot.Parts)
	continueToken := ""
	if nextTargetIndex >= 0 {
		if len(key) == 0 {
			key, err = readContextContinuationKey(input.ProjectRoot, input.RecordRoot)
			if err != nil {
				return ContextReadResult{}, err
			}
		}
		claims, err := contextReadTokenForNext(ctx, plan, input, target, snapshot, nextTargetIndex, nextPart)
		if err != nil {
			return ContextReadResult{}, err
		}
		continueToken, err = encodeContextReadToken(key, claims)
		if err != nil {
			return ContextReadResult{}, err
		}
	}
	result := ContextReadResult{
		Kind:              ContextReadKindChunk,
		Stage:             plan.Stage,
		Slot:              target.Slot,
		Index:             target.Index,
		Part:              part,
		Parts:             snapshot.Parts,
		ContentSHA256:     snapshot.Digest,
		Text:              string(snapshot.Data),
		ReadContinueToken: continueToken,
		Complete:          nextTargetIndex < 0,
	}
	if _, err := marshalContextReadResult(result); err != nil {
		return ContextReadResult{}, err
	}
	if err := verifyContextReadMarker(input, plan); err != nil {
		return ContextReadResult{}, err
	}
	return result, nil
}

func nextContextCursor(targets []contextReadTarget, targetIndex, part, parts int) (int, int) {
	if part < parts {
		return targetIndex, part + 1
	}
	if targetIndex+1 >= len(targets) {
		return -1, 0
	}
	return targetIndex + 1, 1
}

func contextReadTargetIndex(targets []contextReadTarget, slot string, index int) int {
	for targetIndex, target := range targets {
		if string(target.Slot) == slot && target.Index == index {
			return targetIndex
		}
	}
	return -1
}

func validateContextRunStageMarker(marker ActiveDirectiveMarker, input RunStageInput, composition RunStageComposition) error {
	if marker.Kind != ActiveDirectiveKindRunStage || marker.CursorHarness != "codex" ||
		(marker.Delivery != ActiveDirectiveDeliveryIssued && marker.Delivery != ActiveDirectiveDeliveryDelivered) ||
		marker.NeedsRehydrate || marker.ContinueToken != "" || marker.Revision < 1 || marker.ActiveAttempt == nil || marker.ActiveAttempt.ID == "" ||
		marker.ActiveAttempt.Status != ActiveDirectiveAttemptSettled || marker.ActiveAttempt.ResultSHA256 == "" ||
		marker.ActiveAttempt.ResultRevision != marker.Revision ||
		!activeDirectiveSHA256Equal(marker.ActiveAttempt.ResultSHA256, string(composition.Wire)) {
		return fmt.Errorf("read context: active run-stage marker is not current and settled: %w", ErrContextReadBinding)
	}
	stateHash := ""
	if composition.Freshness.StateHash != nil {
		stateHash = *composition.Freshness.StateHash
	}
	if marker.Stage != composition.Freshness.Stage || marker.StateSHA256 != stateHash {
		return fmt.Errorf("read context: active run-stage marker does not match composition: %w", ErrContextReadBinding)
	}
	if err := ValidateActiveDirectiveContext(marker, ActiveDirectiveContext{
		ProjectSHA256: sha256Hex(input.Identity.ProjectPath()),
		IntentUUID:    input.IntentUUID,
		StatePresent:  true,
		StateSHA256:   stateHash,
	}); err != nil {
		return fmt.Errorf("read context: validate active run-stage marker: %w: %w", ErrContextReadBinding, err)
	}
	return nil
}

func contextReadContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("read context: nil context: %w", ErrContextRead)
	}
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("read context: cancelled: %w", err)
	}
	select {
	case <-ctx.Done():
		if err := context.Cause(ctx); err != nil {
			return fmt.Errorf("read context: cancelled: %w", err)
		}
		return fmt.Errorf("read context: cancelled: %w", ctx.Err())
	default:
		return nil
	}
}

func readContextContinuationKey(projectRoot, recordRoot *os.Root) ([]byte, error) {
	if projectRoot == nil || recordRoot == nil {
		return nil, fmt.Errorf("read context: roots are required for continuation key: %w", ErrContextReadToken)
	}
	key, err := steering.ReadContinuationKey(projectRoot, recordRoot)
	if err != nil {
		return nil, fmt.Errorf("read context: continuation key: %w: %w", ErrContextReadToken, err)
	}
	return key, nil
}

type contextReadStreamPass struct {
	Data   []byte
	Digest string
	Parts  int
	Size   int64
}

const maxContextReadFileSize = int64(1<<63 - 1)

func readContextSnapshot(ctx context.Context, root *os.Root, name string, requestedPart int) (contextReadSnapshot, error) {
	if root == nil || !validContextRelativePath(name) {
		return contextReadSnapshot{}, fmt.Errorf("path %q: %w", name, ErrContextReadUnsafePath)
	}
	if err := validateContextPathAncestors(root, name); err != nil {
		return contextReadSnapshot{}, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return contextReadSnapshot{}, fmt.Errorf("inspect path: %w", err)
	}
	if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return contextReadSnapshot{}, fmt.Errorf("path %q is not a regular file: %w", name, ErrContextReadUnsafePath)
	}
	file, err := contextReadOpen(root, name)
	if err != nil {
		return contextReadSnapshot{}, fmt.Errorf("open: %w", err)
	}
	if file == nil {
		return contextReadSnapshot{}, fmt.Errorf("open returned nil file: %w", ErrContextRead)
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	opened, statErr := file.Stat()
	if statErr != nil {
		return contextReadSnapshot{}, fmt.Errorf("stat opened file: %w", statErr)
	}
	if opened == nil || opened.Mode()&fs.ModeSymlink != 0 || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() != info.Size() || !opened.ModTime().Equal(info.ModTime()) {
		return contextReadSnapshot{}, fmt.Errorf("path %q changed identity: %w", name, ErrContextReadFileChanged)
	}
	if info.Size() == maxContextReadFileSize {
		return contextReadSnapshot{}, fmt.Errorf("path %q is too large to check for growth: %w", name, ErrContextReadFileChanged)
	}
	passLimit := info.Size() + 1
	first, err := readContextStreamPass(ctx, file, 0, passLimit)
	if err != nil {
		return contextReadSnapshot{}, err
	}
	if first.Size != info.Size() {
		return contextReadSnapshot{}, fmt.Errorf("path %q grew or shrank during read: %w", name, ErrContextReadFileChanged)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return contextReadSnapshot{}, fmt.Errorf("rewind: %w", err)
	}
	second, err := readContextStreamPass(ctx, file, requestedPart, passLimit)
	if err != nil {
		return contextReadSnapshot{}, err
	}
	if second.Size != info.Size() || first.Size != second.Size || first.Parts != second.Parts || first.Digest != second.Digest {
		return contextReadSnapshot{}, fmt.Errorf("path %q changed during repeated read: %w", name, ErrContextReadFileChanged)
	}
	openedFinal, statErr := file.Stat()
	if statErr != nil {
		return contextReadSnapshot{}, fmt.Errorf("stat after read: %w", statErr)
	}
	if openedFinal == nil || !openedFinal.Mode().IsRegular() || !os.SameFile(info, openedFinal) || openedFinal.Size() != info.Size() || !openedFinal.ModTime().Equal(info.ModTime()) {
		return contextReadSnapshot{}, fmt.Errorf("path %q changed after read: %w", name, ErrContextReadFileChanged)
	}
	if err := validateContextPathAncestors(root, name); err != nil {
		return contextReadSnapshot{}, err
	}
	final, statErr := root.Lstat(name)
	if statErr != nil {
		return contextReadSnapshot{}, fmt.Errorf("verify path after read: %w", statErr)
	}
	if final == nil || final.Mode()&fs.ModeSymlink != 0 || !final.Mode().IsRegular() || !os.SameFile(info, final) || final.Size() != info.Size() || !final.ModTime().Equal(info.ModTime()) {
		return contextReadSnapshot{}, fmt.Errorf("path %q changed after read: %w", name, ErrContextReadFileChanged)
	}
	closeErr := file.Close()
	closed = true
	if closeErr != nil {
		return contextReadSnapshot{}, fmt.Errorf("close: %w", closeErr)
	}
	return contextReadSnapshot{Data: second.Data, Digest: second.Digest, Parts: second.Parts, Size: second.Size, ModTime: final.ModTime()}, nil
}

func readContextStreamPass(ctx context.Context, file contextReadFile, requestedPart int, limit int64) (contextReadStreamPass, error) {
	hash := sha256.New()
	buffer := make([]byte, contextReadStreamBufferBytes)
	pending := make([]byte, 0, utf8.UTFMax)
	chunk := make([]byte, 0, contextReadChunkBytes)
	var result contextReadStreamPass
	part := 0
	for {
		if err := contextReadContext(ctx); err != nil {
			return contextReadStreamPass{}, err
		}
		remaining := limit - result.Size
		if remaining <= 0 {
			return contextReadStreamPass{}, ErrContextReadFileChanged
		}
		readBuffer := buffer
		if int64(len(readBuffer)) > remaining {
			readBuffer = readBuffer[:int(remaining)]
		}
		n, readErr := file.Read(readBuffer)
		if n < 0 || n > len(readBuffer) {
			return contextReadStreamPass{}, fmt.Errorf("read returned invalid byte count %d: %w", n, ErrContextRead)
		}
		if n > 0 {
			_, _ = hash.Write(readBuffer[:n])
			result.Size += int64(n)
			pending = append(pending, readBuffer[:n]...)
			if result.Size > limit-1 {
				return contextReadStreamPass{}, ErrContextReadFileChanged
			}
			for len(pending) > 0 {
				if !utf8.FullRune(pending) {
					break
				}
				runeValue, runeBytes := utf8.DecodeRune(pending)
				if runeValue == utf8.RuneError && runeBytes == 1 {
					return contextReadStreamPass{}, ErrContextReadInvalidUTF8
				}
				if len(chunk) > 0 && len(chunk)+runeBytes > contextReadChunkBytes {
					if err := incrementContextReadPart(&part); err != nil {
						return contextReadStreamPass{}, err
					}
					if part == requestedPart {
						result.Data = append([]byte(nil), chunk...)
					}
					chunk = chunk[:0]
				}
				chunk = append(chunk, pending[:runeBytes]...)
				pending = pending[runeBytes:]
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return contextReadStreamPass{}, fmt.Errorf("read: %w", readErr)
		}
		if n == 0 {
			return contextReadStreamPass{}, fmt.Errorf("read made no progress: %w", io.ErrNoProgress)
		}
	}
	if len(pending) != 0 {
		return contextReadStreamPass{}, ErrContextReadInvalidUTF8
	}
	if len(chunk) > 0 || result.Size == 0 {
		if err := incrementContextReadPart(&part); err != nil {
			return contextReadStreamPass{}, err
		}
		if part == requestedPart {
			result.Data = append([]byte(nil), chunk...)
		}
	}
	result.Parts = part
	result.Digest = hex.EncodeToString(hash.Sum(nil))
	return result, nil
}

func incrementContextReadPart(part *int) error {
	if part == nil || *part == int(^uint(0)>>1) {
		return fmt.Errorf("read context: chunk count overflow: %w", ErrContextRead)
	}
	*part = *part + 1
	return nil
}

func validateContextPathAncestors(root *os.Root, name string) error {
	parts := strings.Split(name, "/")
	for index := 1; index < len(parts); index++ {
		ancestor := strings.Join(parts[:index], "/")
		info, err := root.Lstat(ancestor)
		if err != nil {
			return fmt.Errorf("inspect ancestor %q: %w", ancestor, err)
		}
		if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("ancestor %q is unsafe: %w", ancestor, ErrContextReadUnsafePath)
		}
	}
	return nil
}

func validContextRelativePath(name string) bool {
	return name != "" && name != "." && fs.ValidPath(name) && path.Clean(name) == name
}

func marshalContextReadResult(result ContextReadResult) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("read context: marshal response: %w", err)
	}
	if len(data)+1 > maxContextReadResponseBytes {
		return nil, fmt.Errorf("read context: response is %d bytes including newline, limit is %d: %w", len(data)+1, maxContextReadResponseBytes, ErrContextRead)
	}
	return data, nil
}

func contextReadTokenForNext(ctx context.Context, plan contextReadPlan, input RunStageInput, current contextReadTarget, snapshot contextReadSnapshot, targetIndex, part int) (contextReadTokenClaims, error) {
	if targetIndex < 0 || targetIndex >= len(plan.Targets) {
		return contextReadTokenClaims{}, fmt.Errorf("read context: next target is invalid: %w", ErrContextReadToken)
	}
	target := plan.Targets[targetIndex]
	claims := contextReadTokenClaims{
		Version:        1,
		ProjectSHA256:  plan.ProjectSHA256,
		Space:          input.Identity.Space(),
		Intent:         input.Identity.Intent(),
		MarkerRevision: plan.MarkerRevision,
		GenerationID:   plan.GenerationID,
		WireSHA256:     plan.WireSHA256,
		Stage:          plan.Stage,
		Slot:           string(target.Slot),
		Index:          target.Index,
		Part:           part,
		Path:           target.Path,
	}
	if targetIndex == contextReadTargetIndex(plan.Targets, string(current.Slot), current.Index) {
		claims.Parts = snapshot.Parts
		claims.ContentSHA256 = snapshot.Digest
		claims.Size = snapshot.Size
		claims.ModTimeUnix = snapshot.ModTime.UnixNano()
		return claims, nil
	}
	nextSnapshot, err := readContextSnapshot(ctx, target.Root, target.RelativePath, 0)
	if err != nil {
		return contextReadTokenClaims{}, fmt.Errorf("read context: snapshot next path %q: %w", target.Path, err)
	}
	claims.Parts = nextSnapshot.Parts
	claims.ContentSHA256 = nextSnapshot.Digest
	claims.Size = nextSnapshot.Size
	claims.ModTimeUnix = nextSnapshot.ModTime.UnixNano()
	return claims, nil
}

func contextReadPathInfo(root *os.Root, name string) (fs.FileInfo, error) {
	if root == nil || !validContextRelativePath(name) {
		return nil, fmt.Errorf("path %q: %w", name, ErrContextReadUnsafePath)
	}
	if err := validateContextPathAncestors(root, name); err != nil {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("inspect path: %w", err)
	}
	if info == nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path %q is not a regular file: %w", name, ErrContextReadUnsafePath)
	}
	return info, nil
}

func encodeContextReadToken(key []byte, claims contextReadTokenClaims) (string, error) {
	if len(key) != sha256.Size {
		return "", fmt.Errorf("read context: token key is invalid: %w", ErrContextReadToken)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("read context: marshal token claims: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(contextReadTokenDomain))
	_, _ = mac.Write(payload)
	envelope, err := json.Marshal(contextReadTokenEnvelope{Payload: payload, MAC: base64.RawURLEncoding.EncodeToString(mac.Sum(nil))})
	if err != nil {
		return "", fmt.Errorf("read context: marshal token envelope: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(envelope), nil
}

func decodeContextReadToken(key []byte, token string) (contextReadTokenClaims, error) {
	if len(key) != sha256.Size || token == "" {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token is invalid: %w", ErrContextReadToken)
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return contextReadTokenClaims{}, fmt.Errorf("read context: decode token: %w: %w", ErrContextReadToken, err)
	}
	if base64.RawURLEncoding.EncodeToString(data) != token {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token encoding is not canonical: %w", ErrContextReadToken)
	}
	var envelope contextReadTokenEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Payload) == 0 {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token envelope is invalid: %w", ErrContextReadToken)
	}
	canonicalEnvelope, err := json.Marshal(envelope)
	if err != nil || !bytes.Equal(data, canonicalEnvelope) {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token envelope is not canonical: %w", ErrContextReadToken)
	}
	macBytes, err := base64.RawURLEncoding.DecodeString(envelope.MAC)
	if err != nil || len(macBytes) != sha256.Size || base64.RawURLEncoding.EncodeToString(macBytes) != envelope.MAC {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token MAC is invalid: %w", ErrContextReadToken)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(contextReadTokenDomain))
	_, _ = mac.Write(envelope.Payload)
	if !hmac.Equal(mac.Sum(nil), macBytes) {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token MAC mismatch: %w", ErrContextReadToken)
	}
	var claims contextReadTokenClaims
	if err := json.Unmarshal(envelope.Payload, &claims); err != nil {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token claims are invalid: %w", ErrContextReadToken)
	}
	canonicalPayload, err := json.Marshal(claims)
	if err != nil || !bytes.Equal(envelope.Payload, canonicalPayload) {
		return contextReadTokenClaims{}, fmt.Errorf("read context: token claims are not canonical: %w", ErrContextReadToken)
	}
	return claims, nil
}

func validateContextReadToken(claims contextReadTokenClaims, input RunStageInput, plan contextReadPlan) error {
	if claims.Version != 1 || claims.GenerationID == "" || claims.GenerationID != plan.GenerationID || claims.ProjectSHA256 != plan.ProjectSHA256 || claims.Space != input.Identity.Space() ||
		claims.Intent != input.Identity.Intent() || claims.MarkerRevision != plan.MarkerRevision ||
		claims.WireSHA256 != plan.WireSHA256 || claims.Stage != plan.Stage || claims.Part < 1 || claims.Parts < 1 ||
		claims.Index < 1 || claims.Path == "" || claims.ContentSHA256 == "" {
		return fmt.Errorf("continue context: token binding mismatch: %w", ErrContextReadToken)
	}
	targetIndex := contextReadTargetIndex(plan.Targets, claims.Slot, claims.Index)
	if targetIndex < 0 || plan.Targets[targetIndex].Path != claims.Path {
		return fmt.Errorf("continue context: token path mismatch: %w", ErrContextReadToken)
	}
	return nil
}

func verifyContextReadMarker(input RunStageInput, plan contextReadPlan) error {
	marker, found, err := ReadActiveDirectiveMarker(input.RecordRoot)
	if err != nil {
		return fmt.Errorf("read context: verify active marker: %w", err)
	}
	if !found || marker.Revision != plan.MarkerRevision || marker.Kind != ActiveDirectiveKindRunStage ||
		marker.ActiveAttempt == nil || marker.ActiveAttempt.ID == "" || marker.ActiveAttempt.ID != plan.GenerationID || marker.ActiveAttempt.ResultSHA256 != plan.WireSHA256 ||
		marker.ActiveAttempt.ResultRevision != plan.MarkerRevision ||
		(marker.Delivery != ActiveDirectiveDeliveryIssued && marker.Delivery != ActiveDirectiveDeliveryDelivered) ||
		marker.CursorHarness != "codex" || marker.NeedsRehydrate || marker.ContinueToken != "" {
		return fmt.Errorf("read context: active marker changed during read: %w", ErrContextReadBinding)
	}
	if err := ValidateActiveDirectiveContext(marker, ActiveDirectiveContext{
		ProjectSHA256: plan.ProjectSHA256,
		IntentUUID:    input.IntentUUID,
		StatePresent:  true,
		StateSHA256:   plan.StateSHA256,
	}); err != nil {
		return fmt.Errorf("read context: active marker identity changed during read: %w: %w", ErrContextReadBinding, err)
	}
	return nil
}
