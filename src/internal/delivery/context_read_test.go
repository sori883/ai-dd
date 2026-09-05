package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	contextReadOpen = func(root *os.Root, name string) (*os.File, error) {
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
	contextReadOpen = func(root *os.Root, name string) (*os.File, error) {
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
	contextReadOpen = func(root *os.Root, name string) (*os.File, error) {
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
