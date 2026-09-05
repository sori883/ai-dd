package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadContextAcceptsFreshSettledRunStage(t *testing.T) {
	fixture := newContextReadFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	result, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if result.Kind != ContextReadKindChunk {
		t.Errorf("ReadContext().Kind = %q, want %q", result.Kind, ContextReadKindChunk)
	}
	if result.Slot != ContextReadSlotStageFile {
		t.Errorf("ReadContext().Slot = %q, want %q", result.Slot, ContextReadSlotStageFile)
	}
	if result.Text != "stage context\n" {
		t.Errorf("ReadContext().Text = %q, want stage context", result.Text)
	}
	if !result.Complete {
		t.Error("ReadContext().Complete = false, want true for the only context file")
	}
}

func TestReadContextRejectsLegacyDigestlessMarkerUntilFreshNext(t *testing.T) {
	fixture := newContextReadFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || marker.ActiveAttempt == nil {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v, marker %#v", found, err, marker)
	}
	marker.ActiveAttempt.ResultSHA256 = ""
	marker.ActiveAttempt.ResultRevision = 0
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(digestless): %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadBinding) {
		t.Fatalf("ReadContext(digestless marker) error = %v, want ErrContextReadBinding", err)
	}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() recovery error = %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err != nil {
		t.Fatalf("ReadContext(after fresh Next) error = %v, want nil", err)
	}
}

func TestReadContextRejectsConsumedRunStageMarker(t *testing.T) {
	fixture := newContextReadFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	marker.Delivery = ActiveDirectiveDeliveryConsumed
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(consumed): %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadBinding) {
		t.Fatalf("ReadContext(consumed marker) error = %v, want ErrContextReadBinding", err)
	}
}

func TestReadContextAbsentExpectedFalsePreflightsBeforeOpeningFiles(t *testing.T) {
	fixture := newContextReadAbsentFixture(t, true)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	var opens int
	previousOpen := contextReadOpen
	contextReadOpen = func(root *os.Root, name string) (contextReadFile, error) {
		opens++
		return previousOpen(root, name)
	}
	t.Cleanup(func() { contextReadOpen = previousOpen })
	_, err := ReadContext(context.Background(), input)
	if err == nil || !errors.Is(err, ErrContextReadAbsent) {
		t.Fatalf("ReadContext() error = %v, want ErrContextReadAbsent", err)
	}
	if opens != 0 {
		t.Fatalf("context file opens = %d, want zero before absent preflight", opens)
	}
}

func TestReadContextAbsentExpectedTrueContinues(t *testing.T) {
	fixture := newContextReadAbsentFixture(t, false)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	var opens int
	previousOpen := contextReadOpen
	contextReadOpen = func(root *os.Root, name string) (contextReadFile, error) {
		opens++
		return previousOpen(root, name)
	}
	t.Cleanup(func() { contextReadOpen = previousOpen })
	result, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v, want nil", err)
	}
	if result.Text != "stage context\n" {
		t.Errorf("ReadContext().Text = %q, want stage context", result.Text)
	}
	if opens == 0 {
		t.Fatal("context file opens = 0, want stage file open after expected absence preflight")
	}
}

func TestReadContextContinuationKeyUsesBoundedECMAScriptContract(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if first.ReadContinueToken == "" {
		t.Fatal("ReadContext() token is empty, want continuation token")
	}
	keyBytes, err := fixture.recordRoot.ReadFile(".aidlc-steering-token-key")
	if err != nil {
		t.Fatalf("ReadFile(continuation key): %v", err)
	}
	keyPath := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), ".aidlc-steering-token-key")
	wrapped := append([]byte("\u2003"), keyBytes...)
	wrapped = append(wrapped, []byte("\u2003")...)
	if err := os.WriteFile(keyPath, wrapped, 0o600); err != nil {
		t.Fatalf("WriteFile(ECMAScript-whitespace key): %v", err)
	}
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err != nil {
		t.Fatalf("ContinueContext(ECMAScript-whitespace key) error = %v, want nil", err)
	}

	if err := os.WriteFile(keyPath, bytes.Repeat([]byte{'A'}, 4<<10+1), 0o600); err != nil {
		t.Fatalf("WriteFile(oversize key): %v", err)
	}
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err == nil || !errors.Is(err, ErrContextReadToken) {
		t.Fatalf("ContinueContext(oversize key) error = %v, want ErrContextReadToken", err)
	}
}

func newContextReadAbsentFixture(t *testing.T, unexpected bool) runStageFixture {
	t.Helper()
	fixture := newContextReadFixture(t)
	graph := strings.Replace(runStageGraphJSON,
		`"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],`,
		`"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[{"artifact":"context-input","required":true}],`,
		1,
	)
	producer := `"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],`
	if unexpected {
		graph = strings.Replace(graph, producer, strings.Replace(producer, `"produces":[]`, `"produces":["context-input"]`, 1), 1)
	}
	writeRunStageFile(t, fixture.stageGraphPath, graph)
	return fixture
}

func TestReadContextOrderAndBoundedChunks(t *testing.T) {
	fixture, want := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	var got []ContextReadResult
	result, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	for {
		wire, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatalf("json.Marshal(context result): %v", marshalErr)
		}
		if len(wire)+1 > maxContextReadResponseBytes {
			t.Fatalf("context response bytes = %d, want <= %d including newline", len(wire)+1, maxContextReadResponseBytes)
		}
		got = append(got, result)
		if result.Complete {
			break
		}
		if result.ReadContinueToken == "" {
			t.Fatal("incomplete context result has no continuation token")
		}
		result, err = ContinueContext(context.Background(), input, result.ReadContinueToken)
		if err != nil {
			t.Fatalf("ContinueContext() error = %v", err)
		}
	}

	var slots []ContextReadSlot
	var texts []string
	for _, chunk := range got {
		slots = append(slots, chunk.Slot)
		texts = append(texts, chunk.Text)
	}
	if len(got) < 4 {
		t.Fatalf("context chunks = %d, want multiple inline chunks followed by stage and consume", len(got))
	}
	if !reflect.DeepEqual(slots[len(slots)-2:], []ContextReadSlot{ContextReadSlotStage, ContextReadSlotConsume}) {
		t.Errorf("context slot order suffix = %#v, want stage then consume", slots[len(slots)-2:])
	}
	allText := strings.Join(texts, "")
	if !strings.Contains(allText, want.inlineLead) || !strings.Contains(allText, want.consume) {
		t.Errorf("context text does not contain expected inline/consume content")
	}
	if got[0].Slot != ContextReadSlotInline || got[0].Index != 1 || got[0].Part != 1 {
		t.Errorf("first context chunk = %#v, want first inline chunk", got[0])
	}
	for index := 1; index < len(got); index++ {
		if got[index].ContentSHA256 == "" {
			t.Errorf("context chunk %d has empty content digest", index)
		}
	}
}

func TestReadContextTokenTamperReplayAndContentChange(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if first.ReadContinueToken == "" {
		t.Fatal("ReadContext() token is empty, want continuation token")
	}
	replacement := byte('A')
	if first.ReadContinueToken[len(first.ReadContinueToken)-1] == replacement {
		replacement = 'B'
	}
	tampered := first.ReadContinueToken[:len(first.ReadContinueToken)-1] + string(replacement)
	if _, err := ContinueContext(context.Background(), input, tampered); err == nil {
		t.Fatal("ContinueContext(tampered token) error = nil, want rejection")
	}
	replay, err := ContinueContext(context.Background(), input, first.ReadContinueToken)
	if err != nil {
		t.Fatalf("ContinueContext(first token) error = %v", err)
	}
	replayAgain, err := ContinueContext(context.Background(), input, first.ReadContinueToken)
	if err != nil {
		t.Fatalf("ContinueContext(replayed token) error = %v", err)
	}
	if !reflect.DeepEqual(replay, replayAgain) {
		t.Errorf("same-token replay = %#v, want %#v", replayAgain, replay)
	}
	writeRunStageFile(t, filepath.Join(fixture.identity.ProjectPath(), ".codex", "agents", "aidlc-product-agent.md"), "changed after publication\n")
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err == nil {
		t.Fatal("ContinueContext(content changed) error = nil, want rejection")
	}
}

func TestReadContextRejectsCrossFileContentChangeWithRestoredMetadata(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	result, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	for result.Slot != ContextReadSlotInline || result.Index != 1 || result.Part != result.Parts {
		if result.Complete {
			t.Fatal("context stream completed before issuing the next-file token")
		}
		if result.ReadContinueToken == "" {
			t.Fatal("incomplete context result has no continuation token")
		}
		result, err = ContinueContext(context.Background(), input, result.ReadContinueToken)
		if err != nil {
			t.Fatalf("ContinueContext() while locating file-boundary token: %v", err)
		}
	}
	if result.ReadContinueToken == "" {
		t.Fatal("last chunk of the first file has no continuation token")
	}

	supportPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "agents", "aidlc-architect-agent.md")
	original, err := os.ReadFile(supportPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", supportPath, err)
	}
	info, err := os.Stat(supportPath)
	if err != nil {
		t.Fatalf("Stat(%q): %v", supportPath, err)
	}
	replacement := bytes.Repeat([]byte{'x'}, len(original))
	if bytes.Equal(replacement, original) {
		replacement[0] = 'y'
	}
	if err := os.WriteFile(supportPath, replacement, info.Mode().Perm()); err != nil {
		t.Fatalf("WriteFile(%q): %v", supportPath, err)
	}
	if err := os.Chtimes(supportPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("Chtimes(%q): %v", supportPath, err)
	}

	if _, err := ContinueContext(context.Background(), input, result.ReadContinueToken); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
		t.Fatalf("ContinueContext(cross-file same metadata) error = %v, want ErrContextReadFileChanged", err)
	}
}

func TestReadContextBoundaryTokenDoesNotRebaseAfterFutureChanges(t *testing.T) {
	t.Run("unchanged replay is identical", func(t *testing.T) {
		fixture, _ := newOrderedContextFixture(t)
		input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
		if _, err := Next(context.Background(), input); err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		boundary := readContextFirstFilePreFinalToken(t, input)
		first, err := ContinueContext(context.Background(), input, boundary)
		if err != nil {
			t.Fatalf("ContinueContext(boundary token) error = %v", err)
		}
		replay, err := ContinueContext(context.Background(), input, boundary)
		if err != nil {
			t.Fatalf("ContinueContext(replayed boundary token) error = %v", err)
		}
		if !reflect.DeepEqual(first, replay) {
			t.Fatalf("same boundary-token replay = %#v, want %#v", replay, first)
		}
	})

	t.Run("next target change is rejected", func(t *testing.T) {
		fixture, _ := newOrderedContextFixture(t)
		input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
		if _, err := Next(context.Background(), input); err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		boundary := readContextFirstFilePreFinalToken(t, input)
		if _, err := ContinueContext(context.Background(), input, boundary); err != nil {
			t.Fatalf("ContinueContext(first boundary replay) error = %v", err)
		}
		supportPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "agents", "aidlc-architect-agent.md")
		replaceContextReadFilePreservingMetadata(t, supportPath, []byte("changed support\n"))
		if _, err := ContinueContext(context.Background(), input, boundary); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
			t.Fatalf("ContinueContext(next target changed) error = %v, want ErrContextReadFileChanged", err)
		}
	})

	t.Run("third target change cannot rebase from earlier boundary", func(t *testing.T) {
		fixture, _ := newOrderedContextFixture(t)
		input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
		if _, err := Next(context.Background(), input); err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		boundary := readContextFirstFilePreFinalToken(t, input)
		middle, err := ContinueContext(context.Background(), input, boundary)
		if err != nil {
			t.Fatalf("ContinueContext(boundary token) error = %v", err)
		}
		if middle.Slot != ContextReadSlotInline || middle.Index != 1 || middle.Part != middle.Parts || middle.ReadContinueToken == "" {
			t.Fatalf("boundary result = %#v, want final first inline chunk with successor token", middle)
		}
		middleToken := middle.ReadContinueToken
		if _, err := ContinueContext(context.Background(), input, middleToken); err != nil {
			t.Fatalf("ContinueContext(middle successor) error = %v", err)
		}
		stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
		replaceContextReadFilePreservingMetadata(t, stagePath, []byte("changed stage\n"))
		if _, err := ContinueContext(context.Background(), input, middleToken); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
			t.Fatalf("ContinueContext(successor token after third target change) error = %v, want ErrContextReadFileChanged", err)
		}
	})
}

func readContextFirstFilePreFinalToken(t *testing.T, input RunStageInput) string {
	t.Helper()
	result, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if result.Parts < 2 {
		t.Fatalf("first inline file parts = %d, want at least two for boundary replay test", result.Parts)
	}
	for result.Slot != ContextReadSlotInline || result.Index != 1 || result.Part != result.Parts-1 {
		if result.Complete {
			t.Fatal("context stream completed before issuing the first-file pre-final token")
		}
		if result.ReadContinueToken == "" {
			t.Fatal("incomplete context result has no continuation token")
		}
		result, err = ContinueContext(context.Background(), input, result.ReadContinueToken)
		if err != nil {
			t.Fatalf("ContinueContext() while locating first-file boundary token: %v", err)
		}
	}
	if result.ReadContinueToken == "" {
		t.Fatal("pre-final chunk of the first file has no continuation token")
	}
	return result.ReadContinueToken
}

func replaceContextReadFilePreservingMetadata(t *testing.T, name string, replacement []byte) {
	t.Helper()
	original, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", name, err)
	}
	if len(original) != len(replacement) {
		t.Fatalf("replacement size = %d, want %d for %q", len(replacement), len(original), name)
	}
	if bytes.Equal(original, replacement) {
		t.Fatalf("replacement for %q unexpectedly matches original", name)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatalf("Stat(%q): %v", name, err)
	}
	if err := os.WriteFile(name, replacement, info.Mode().Perm()); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
	if err := os.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("Chtimes(%q): %v", name, err)
	}
}

type boundedContextReadFile struct {
	contextReadFile
	maximum int
}

func (file boundedContextReadFile) Read(buffer []byte) (int, error) {
	if len(buffer) > file.maximum {
		return 0, errors.New("test reader received an oversized buffer")
	}
	return file.contextReadFile.Read(buffer)
}

func TestReadContextUsesBoundedStreamingReads(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	previousOpen := contextReadOpen
	contextReadOpen = func(root *os.Root, name string) (contextReadFile, error) {
		file, err := previousOpen(root, name)
		if err != nil {
			return nil, err
		}
		return boundedContextReadFile{contextReadFile: file, maximum: contextReadStreamBufferBytes}, nil
	}
	t.Cleanup(func() { contextReadOpen = previousOpen })

	if _, err := ReadContext(context.Background(), input); err != nil {
		t.Fatalf("ReadContext() error = %v, want bounded streaming to succeed", err)
	}
}

type growingContextReadFile struct {
	contextReadFile
	path  string
	grown bool
	reads int
}

func (file *growingContextReadFile) Read(buffer []byte) (int, error) {
	if !file.grown {
		appendFile, err := os.OpenFile(file.path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return 0, err
		}
		_, writeErr := appendFile.Write(bytes.Repeat([]byte("growth\n"), 1024))
		closeErr := appendFile.Close()
		if writeErr != nil {
			return 0, writeErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		file.grown = true
	}
	if file.reads > 0 {
		return 0, errors.New("test growth stream was read past the initial size bound")
	}
	file.reads++
	return file.contextReadFile.Read(buffer)
}

func TestReadContextStopsAtInitialSizeWhenFileGrows(t *testing.T) {
	fixture := newContextReadFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	previousOpen := contextReadOpen
	contextReadOpen = func(root *os.Root, name string) (contextReadFile, error) {
		file, err := previousOpen(root, name)
		if err != nil {
			return nil, err
		}
		if name == ".codex/aidlc-common/stages/ideation/intent-capture.md" {
			return &growingContextReadFile{contextReadFile: file, path: stagePath}, nil
		}
		return file, nil
	}
	t.Cleanup(func() { contextReadOpen = previousOpen })

	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
		t.Fatalf("ReadContext(growing file) error = %v, want ErrContextReadFileChanged before reading past size+1", err)
	}
}

func TestReadContextRejectsTokenAfterMarkerRecoveryAtSameRevision(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if first.ReadContinueToken == "" {
		t.Fatal("ReadContext() token is empty, want continuation token")
	}
	originalMarker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || originalMarker.ActiveAttempt == nil || originalMarker.ActiveAttempt.ID == "" {
		t.Fatalf("ReadActiveDirectiveMarker(original) = found %v, error %v, marker %#v", found, err, originalMarker)
	}
	markerPath := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", fixture.identity.Space(), "intents", fixture.identity.Intent(), activeDirectiveMarkerName)
	if err := os.Remove(markerPath); err != nil {
		t.Fatalf("Remove(active marker): %v", err)
	}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() recovery error = %v", err)
	}
	recovered, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || recovered.Revision != 1 || recovered.ActiveAttempt == nil {
		t.Fatalf("recovered marker = found %v, error %v, marker %#v; want fresh revision 1", found, err, recovered)
	}
	if recovered.ActiveAttempt.ID == originalMarker.ActiveAttempt.ID {
		t.Fatalf("recovered publication generation = %q, want a new generation after marker recovery", recovered.ActiveAttempt.ID)
	}
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err == nil || (!errors.Is(err, ErrContextReadToken) && !errors.Is(err, ErrContextReadBinding)) {
		t.Fatalf("ContinueContext(token from removed marker) error = %v, want token or binding rejection", err)
	}
}

func TestReadContextRejectsNonCanonicalTokenEnvelope(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(first.ReadContinueToken)
	if err != nil {
		t.Fatalf("DecodeString(read token): %v", err)
	}
	var envelope contextReadTokenEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(read envelope): %v", err)
	}
	key, err := readContextContinuationKey(fixture.projectRoot, fixture.recordRoot)
	if err != nil {
		t.Fatalf("readContextContinuationKey(): %v", err)
	}

	cases := []struct {
		name  string
		build func() string
	}{
		{
			name: "unknown outer field",
			build: func() string {
				modified := map[string]json.RawMessage{
					"p":       envelope.Payload,
					"m":       json.RawMessage(quoteJSONForTest(t, envelope.MAC)),
					"unknown": json.RawMessage(`"unexpected"`),
				}
				return encodeContextReadTokenEnvelopeForTest(t, modified)
			},
		},
		{
			name: "duplicate outer field",
			build: func() string {
				duplicate := append([]byte(`{"p":`), envelope.Payload...)
				duplicate = append(duplicate, []byte(`,"m":`)...)
				macJSON, _ := json.Marshal(envelope.MAC)
				duplicate = append(duplicate, macJSON...)
				duplicate = append(duplicate, []byte(`,"m":`)...)
				duplicate = append(duplicate, macJSON...)
				duplicate = append(duplicate, '}')
				return base64.RawURLEncoding.EncodeToString(duplicate)
			},
		},
		{
			name: "unknown claims field with valid MAC",
			build: func() string {
				payload := appendJSONField(t, envelope.Payload, `,"unknown":true`)
				return signContextReadPayloadForTest(t, key, payload)
			},
		},
		{
			name: "duplicate claims field with valid MAC",
			build: func() string {
				payload := appendJSONField(t, envelope.Payload, `,"v":1`)
				return signContextReadPayloadForTest(t, key, payload)
			},
		},
		{
			name: "noncanonical outer base64",
			build: func() string {
				return base64.RawURLEncoding.EncodeToString(raw) + "="
			},
		},
		{
			name: "noncanonical MAC base64",
			build: func() string {
				macBytes, err := base64.RawURLEncoding.DecodeString(envelope.MAC)
				if err != nil {
					t.Fatalf("DecodeString(MAC): %v", err)
				}
				modified := contextReadTokenEnvelope{Payload: envelope.Payload, MAC: base64.URLEncoding.EncodeToString(macBytes)}
				encoded, err := json.Marshal(modified)
				if err != nil {
					t.Fatalf("json.Marshal(noncanonical MAC envelope): %v", err)
				}
				return base64.RawURLEncoding.EncodeToString(encoded)
			},
		},
		{
			name: "outer whitespace",
			build: func() string {
				withWhitespace := append([]byte{' '}, raw...)
				withWhitespace = append(withWhitespace, '\n')
				return base64.RawURLEncoding.EncodeToString(withWhitespace)
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := ContinueContext(context.Background(), input, testCase.build()); err == nil || !errors.Is(err, ErrContextReadToken) {
				t.Fatalf("ContinueContext(%s) error = %v, want ErrContextReadToken", testCase.name, err)
			}
		})
	}
}

func quoteJSONForTest(t *testing.T, value string) []byte {
	t.Helper()
	quoted, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%q): %v", value, err)
	}
	return quoted
}

func encodeContextReadTokenEnvelopeForTest(t *testing.T, envelope map[string]json.RawMessage) string {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal(token envelope): %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func appendJSONField(t *testing.T, payload []byte, field string) []byte {
	t.Helper()
	if len(payload) == 0 || payload[len(payload)-1] != '}' {
		t.Fatalf("token payload = %q, want JSON object", payload)
	}
	modified := append([]byte(nil), payload[:len(payload)-1]...)
	modified = append(modified, field...)
	modified = append(modified, '}')
	return modified
}

func signContextReadPayloadForTest(t *testing.T, key, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(contextReadTokenDomain))
	_, _ = mac.Write(payload)
	envelope := contextReadTokenEnvelope{Payload: payload, MAC: base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal(signed token envelope): %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

type contextReadTreeEntry struct {
	Mode fs.FileMode
	Body string
}

func snapshotContextReadTree(t *testing.T, project string) map[string]contextReadTreeEntry {
	t.Helper()
	root := os.DirFS(project)
	entries := map[string]contextReadTreeEntry{}
	if err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." || !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		entries[path] = contextReadTreeEntry{Mode: info.Mode(), Body: string(body)}
		return nil
	}); err != nil {
		t.Fatalf("snapshot project tree: %v", err)
	}
	return entries
}

func TestReadContextSuccessAndFailureAreReadOnly(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	beforeSuccess := snapshotContextReadTree(t, fixture.identity.ProjectPath())
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if afterSuccess := snapshotContextReadTree(t, fixture.identity.ProjectPath()); !reflect.DeepEqual(beforeSuccess, afterSuccess) {
		t.Fatal("successful ReadContext changed project files, marker, key, state, audit, artifact, or session data")
	}

	beforeFailure := snapshotContextReadTree(t, fixture.identity.ProjectPath())
	tampered := first.ReadContinueToken[:len(first.ReadContinueToken)-1] + "A"
	if tampered == first.ReadContinueToken {
		tampered = first.ReadContinueToken[:len(first.ReadContinueToken)-1] + "B"
	}
	if _, err := ContinueContext(context.Background(), input, tampered); err == nil {
		t.Fatal("ContinueContext(tampered token) error = nil, want rejection")
	}
	if afterFailure := snapshotContextReadTree(t, fixture.identity.ProjectPath()); !reflect.DeepEqual(beforeFailure, afterFailure) {
		t.Fatal("failed ContinueContext changed project files, marker, key, state, audit, artifact, or session data")
	}
}

func TestReadContextRejectsStaleTokenAfterFreshPublication(t *testing.T) {
	fixture, _ := newOrderedContextFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	first, err := ReadContext(context.Background(), input)
	if err != nil {
		t.Fatalf("ReadContext() error = %v", err)
	}
	if first.ReadContinueToken == "" {
		t.Fatal("ReadContext() token is empty, want continuation token")
	}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() fresh publication error = %v", err)
	}
	if _, err := ContinueContext(context.Background(), input, first.ReadContinueToken); err == nil || (!errors.Is(err, ErrContextReadToken) && !errors.Is(err, ErrContextReadBinding)) {
		t.Fatalf("ContinueContext(stale token) error = %v, want token or binding rejection", err)
	}
}

func TestReadContextRejectsLeafSymlinkFile(t *testing.T) {
	fixture := newContextReadFixture(t)
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	outsidePath := filepath.Join(t.TempDir(), "outside.md")
	writeRunStageFile(t, outsidePath, "outside\n")
	if err := os.Remove(stagePath); err != nil {
		t.Fatalf("Remove(stage file): %v", err)
	}
	if err := os.Symlink(outsidePath, stagePath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadUnsafePath) {
		t.Fatalf("ReadContext(leaf symlink) error = %v, want ErrContextReadUnsafePath", err)
	}
}

func TestReadContextRejectsAncestorSymlinkFile(t *testing.T) {
	fixture := newContextReadFixture(t)
	stagesPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages")
	stagesRealPath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages-real")
	outsideDir := t.TempDir()
	outsideStage := filepath.Join(outsideDir, "ideation", "intent-capture.md")
	if err := os.MkdirAll(filepath.Dir(outsideStage), 0o700); err != nil {
		t.Fatalf("MkdirAll(outside stage): %v", err)
	}
	writeRunStageFile(t, outsideStage, "outside\n")
	if err := os.Rename(stagesPath, stagesRealPath); err != nil {
		t.Fatalf("Rename(stages): %v", err)
	}
	if err := os.Symlink(outsideDir, stagesPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadUnsafePath) {
		t.Fatalf("ReadContext(ancestor symlink) error = %v, want ErrContextReadUnsafePath", err)
	}
}

func TestReadContextRejectsInvalidUTF8File(t *testing.T) {
	fixture := newContextReadFixture(t)
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.WriteFile(stagePath, []byte{0xff, 0xfe, '\n'}, 0o600); err != nil {
		t.Fatalf("WriteFile(invalid UTF-8): %v", err)
	}
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadInvalidUTF8) {
		t.Fatalf("ReadContext(invalid UTF-8) error = %v, want ErrContextReadInvalidUTF8", err)
	}
}

func TestReadContextRejectsPathContentRace(t *testing.T) {
	fixture := newContextReadFixture(t)
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	previousOpen := contextReadOpen
	contextReadOpen = func(root *os.Root, name string) (contextReadFile, error) {
		file, err := previousOpen(root, name)
		if err == nil && name == ".codex/aidlc-common/stages/ideation/intent-capture.md" {
			if writeErr := os.WriteFile(stagePath, []byte("replacement after open\n"), 0o600); writeErr != nil {
				t.Fatalf("WriteFile(replacement): %v", writeErr)
			}
		}
		return file, err
	}
	t.Cleanup(func() { contextReadOpen = previousOpen })
	if _, err := ReadContext(context.Background(), input); err == nil || !errors.Is(err, ErrContextReadFileChanged) {
		t.Fatalf("ReadContext(path race) error = %v, want ErrContextReadFileChanged", err)
	}
}

type orderedContextWant struct {
	inlineLead string
	consume    string
}

func newOrderedContextFixture(t *testing.T) (runStageFixture, orderedContextWant) {
	t.Helper()
	fixture := newContextReadFixture(t)
	project := fixture.identity.ProjectPath()
	leadPath := filepath.Join(project, ".codex", "agents", "aidlc-product-agent.md")
	supportPath := filepath.Join(project, ".codex", "agents", "aidlc-architect-agent.md")
	if err := os.MkdirAll(filepath.Dir(leadPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(leadPath), err)
	}
	lead := strings.Repeat("lead begin\n予測不能 middle 🚀\nlead end\n", 600)
	support := "support context\n"
	writeRunStageFile(t, leadPath, lead)
	writeRunStageFile(t, supportPath, support)
	graph := strings.Replace(runStageKnowledgeGraphJSON("inline"),
		`"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]`,
		`"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[{"artifact":"context-input","required":true}],"requires_stage":[]`,
		1,
	)
	graph = strings.Replace(graph,
		`"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[]`,
		`"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"product-agent","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":["context-input"]`,
		1,
	)
	writeRunStageFile(t, fixture.stageGraphPath, graph)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	composition, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(ordered fixture) error = %v", err)
	}
	var wire contextRunStageWire
	if err := json.Unmarshal(composition.Wire, &wire); err != nil {
		t.Fatalf("json.Unmarshal(ordered wire): %v", err)
	}
	if len(wire.ConsumesAbsent) != 1 || wire.ConsumesAbsent[0].Expected {
		t.Fatalf("ordered fixture consumes_absent = %#v, want one unexpected absence", wire.ConsumesAbsent)
	}
	consumePath := filepath.Join(project, filepath.FromSlash(wire.ConsumesAbsent[0].Path))
	if err := os.MkdirAll(filepath.Dir(consumePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(consumePath), err)
	}
	writeRunStageFile(t, consumePath, "consume context 日本語\n")
	return fixture, orderedContextWant{inlineLead: lead, consume: "consume context 日本語\n"}
}

func newContextReadFixture(t *testing.T) runStageFixture {
	t.Helper()
	fixture := newRunStageFixture(t)
	stagePath := filepath.Join(fixture.identity.ProjectPath(), ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(stagePath), err)
	}
	writeRunStageFile(t, stagePath, "stage context\n")
	return fixture
}
