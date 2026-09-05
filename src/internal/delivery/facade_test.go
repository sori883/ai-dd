package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestDeliveryNextPublishesLoadSteeringAfterMarkerCommit(t *testing.T) {
	fixture := newRunStageFixture(t)

	rulePath := filepath.Join(fixture.identity.ProjectPath(), "delivery-rule.md")
	ruleText := "# delivery\n\nrule content\n"
	writeRunStageFile(t, rulePath, ruleText)
	writeRunStageFile(t, fixture.stageGraphPath, strings.Replace(runStageGraphJSON,
		`"consumes":[]`, `"consumes":[],"rules_in_context":[{"path":"delivery-rule.md","scope":"project"}]`, 2))

	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	wantComposition, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage() error = %v", err)
	}
	if len(wantComposition.Chunks) != 1 {
		t.Fatalf("ComposeRunStage().Chunks = %d, want 1", len(wantComposition.Chunks))
	}

	got, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got.Kind != ActiveDirectiveKindLoadSteering {
		t.Fatalf("Next().Kind = %q, want %q", got.Kind, ActiveDirectiveKindLoadSteering)
	}
	if got.Part != 1 || got.Parts != 1 {
		t.Errorf("Next() part/parts = %d/%d, want 1/1", got.Part, got.Parts)
	}
	if got.ContinueToken == "" {
		t.Fatal("Next().ContinueToken is empty")
	}
	if !bytes.Contains(got.Wire, []byte(`"kind":"load-steering"`)) {
		t.Errorf("Next().Wire = %s, want load-steering wire", got.Wire)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v; want committed marker", found, err)
	}
	if marker.Kind != ActiveDirectiveKindLoadSteering || marker.ContinueToken != got.ContinueToken {
		t.Errorf("marker = %#v, want load-steering token %q", marker, got.ContinueToken)
	}
	if marker.Delivery != ActiveDirectiveDeliveryIssued {
		t.Errorf("marker.Delivery = %q, want %q", marker.Delivery, ActiveDirectiveDeliveryIssued)
	}
	wantOwner := "sessionless:" + sha256Hex(fixture.identity.ProjectPath())[:16]
	if marker.OwnerSession != wantOwner {
		t.Errorf("marker.OwnerSession = %q, want %q", marker.OwnerSession, wantOwner)
	}
	if marker.CursorHarness != "codex" {
		t.Errorf("marker.CursorHarness = %q, want codex", marker.CursorHarness)
	}
	if _, err := steering.ReadOrCreateContinuationKey(fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("ReadOrCreateContinuationKey() error = %v", err)
	}
}

func TestDeliveryNextWithoutChunksCommitsAndReturnsRunStage(t *testing.T) {
	fixture := newRunStageFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	want, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage() error = %v", err)
	}

	got, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got.Kind != ActiveDirectiveKindRunStage {
		t.Fatalf("Next().Kind = %q, want %q", got.Kind, ActiveDirectiveKindRunStage)
	}
	if got.ContinueToken != "" {
		t.Errorf("Next().ContinueToken = %q, want empty", got.ContinueToken)
	}
	if !bytes.Equal(got.Wire, want.Wire) {
		t.Errorf("Next().Wire = %s, want fresh ComposeRunStage().Wire %s", got.Wire, want.Wire)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v; want committed marker", found, err)
	}
	if marker.Kind != ActiveDirectiveKindRunStage || marker.ContinueToken != "" {
		t.Errorf("marker = %#v, want committed run-stage without token", marker)
	}
}

func TestDeliveryNextRejectsInvalidInput(t *testing.T) {
	_, err := Next(context.Background(), RunStageInput{})
	if err == nil {
		t.Fatal("Next(zero input) error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidDelivery) {
		t.Errorf("Next(zero input) error = %v, want ErrInvalidDelivery", err)
	}
}

func TestDeliveryNextStartsPublicationAtRevisionOneWithFreshAttempt(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || marker.ActiveAttempt == nil {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v, marker %#v", found, err, marker)
	}
	if marker.Revision != 1 {
		t.Errorf("initial marker revision = %d, want 1", marker.Revision)
	}
	if marker.ActiveAttempt.ID != "sessionless" || marker.ActiveAttempt.CommandKind != ActiveDirectiveCommandNext ||
		marker.ActiveAttempt.CommandSHA256 != marker.StateSHA256 || marker.ActiveAttempt.CursorInputSHA256 != "" ||
		marker.ActiveAttempt.ResultSHA256 != "" || marker.ActiveAttempt.ResultRevision != 0 {
		t.Errorf("initial active attempt = %#v, want fixed generic fresh attempt", marker.ActiveAttempt)
	}
}

func TestDeliveryPublicationPreservesGenericMarkerMetadata(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(initial) error = %v", err)
	}
	base, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || base.ActiveAttempt == nil {
		t.Fatalf("ReadActiveDirectiveMarker(base) = found %v, error %v, marker %#v", found, err, base)
	}
	base.HumanSequence = 17
	base.EngineSequence = 19
	base.ConversationSequence = 23
	base.StopFingerprint = "stop-fingerprint"
	base.StopCount = 29
	base.Extra = map[string]json.RawMessage{
		"top_unknown": json.RawMessage(`{"preserve":true}`),
	}
	base.ActiveAttempt.Extra = map[string]json.RawMessage{
		"attempt_unknown": json.RawMessage(`{"preserve":42}`),
	}
	base.Resume = &ActiveDirectiveResume{
		Status:             ActiveDirectiveResumeWaiting,
		IssuingStage:       base.Stage,
		IssuingStateSHA256: base.StateSHA256,
		IssuingSession:     base.OwnerSession,
		IssuingIntentUUID:  base.IntentUUID,
		Action:             ActiveDirectiveResumeActionResume,
		Extra: map[string]json.RawMessage{
			"resume_unknown": json.RawMessage(`{"preserve":"yes"}`),
		},
	}
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, base); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(base): %v", err)
	}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(publication) error = %v", err)
	}
	got, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || got.ActiveAttempt == nil || got.Resume == nil {
		t.Fatalf("ReadActiveDirectiveMarker(publication) = found %v, error %v, marker %#v", found, err, got)
	}
	if got.Revision != base.Revision+1 {
		t.Errorf("publication revision = %d, want %d", got.Revision, base.Revision+1)
	}
	if got.HumanSequence != base.HumanSequence || got.EngineSequence != base.EngineSequence ||
		got.ConversationSequence != base.ConversationSequence || got.StopFingerprint != base.StopFingerprint || got.StopCount != base.StopCount {
		t.Errorf("publication counters = %d/%d/%d/%q/%d, want preserved %d/%d/%d/%q/%d",
			got.HumanSequence, got.EngineSequence, got.ConversationSequence, got.StopFingerprint, got.StopCount,
			base.HumanSequence, base.EngineSequence, base.ConversationSequence, base.StopFingerprint, base.StopCount)
	}
	if string(got.Extra["top_unknown"]) != `{"preserve":true}` || string(got.ActiveAttempt.Extra["attempt_unknown"]) != `{"preserve":42}` ||
		string(got.Resume.Extra["resume_unknown"]) != `{"preserve":"yes"}` {
		t.Errorf("publication unknown metadata lost: top=%#v attempt=%#v resume=%#v", got.Extra, got.ActiveAttempt.Extra, got.Resume.Extra)
	}
}

func TestDeliveryPublicationClearsDirectiveSpecificMetadata(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(initial) error = %v", err)
	}
	base, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(base) = found %v, error %v", found, err)
	}
	base.Unit = "legacy-unit"
	base.Units = []string{"legacy-unit"}
	base.CodeGenerationSourceSHA256 = strings.Repeat("c", sha256.Size*2)
	base.CodeGenerationAuthorityRevision = 7
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, base); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(base): %v", err)
	}
	nextResult, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next(publication) error = %v", err)
	}
	assertDirectiveSpecificMetadataCleared(t, fixture.recordRoot)

	current, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(current) = found %v, error %v", found, err)
	}
	current.Unit = "legacy-unit"
	current.Units = []string{"legacy-unit"}
	current.CodeGenerationSourceSHA256 = strings.Repeat("d", sha256.Size*2)
	current.CodeGenerationAuthorityRevision = 8
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, current); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(current): %v", err)
	}
	if _, err := Continue(context.Background(), input, nextResult.ContinueToken); err != nil {
		t.Fatalf("Continue(publication) error = %v", err)
	}
	assertDirectiveSpecificMetadataCleared(t, fixture.recordRoot)
}

func assertDirectiveSpecificMetadataCleared(t *testing.T, root *os.Root) {
	t.Helper()
	data := mustReadMarkerBytes(t, root)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(data), &fields); err != nil {
		t.Fatalf("Unmarshal(marker): %v", err)
	}
	for _, field := range []string{
		"unit", "units", "code_generation_source_sha256", "code_generation_authority_revision",
	} {
		if _, ok := fields[field]; ok {
			t.Errorf("directive-specific field %q is present in publication: %s", field, data)
		}
	}
}

func TestDeliveryNextPreservesGenericBaseAcrossStateChange(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(initial) error = %v", err)
	}
	base, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(base) = found %v, error %v", found, err)
	}
	base.HumanSequence = 41
	base.Extra = map[string]json.RawMessage{"state_change_base": json.RawMessage(`{"preserve":true}`)}
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, base); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(base): %v", err)
	}
	statePath := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md")
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state): %v", err)
	}
	writeRunStageFile(t, statePath, string(stateBytes)+"\n## Delivery State Change\nchanged\n")
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(state change) error = %v", err)
	}
	got, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(after state change) = found %v, error %v", found, err)
	}
	if got.Revision != base.Revision+1 {
		t.Errorf("state-change next revision = %d, want %d", got.Revision, base.Revision+1)
	}
	if got.HumanSequence != base.HumanSequence || string(got.Extra["state_change_base"]) != `{"preserve":true}` {
		t.Errorf("state-change next lost generic base metadata: sequence=%d extra=%#v", got.HumanSequence, got.Extra)
	}
	if got.StateSHA256 == base.StateSHA256 {
		t.Error("state-change next retained the old state hash")
	}
}

func TestDeliveryContinueAcceptsFixedGenericAttempt(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || marker.ActiveAttempt == nil {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v, marker %#v", found, err, marker)
	}
	marker.ActiveAttempt.CursorInputSHA256 = ""
	marker.ActiveAttempt.ResultSHA256 = ""
	marker.ActiveAttempt.ResultRevision = 0
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(generic): %v", err)
	}
	if _, err := Continue(context.Background(), input, first.ContinueToken); err != nil {
		t.Fatalf("Continue(generic attempt) error = %v, want nil", err)
	}
}

func TestDeliveryNextRecoversFromCorruptMarker(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	if _, err := Next(context.Background(), input); err != nil {
		t.Fatalf("Next(initial) error = %v", err)
	}
	markerPath := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", "team", "intents", "build", activeDirectiveMarkerName)
	if err := os.WriteFile(markerPath, []byte("{broken\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupt marker): %v", err)
	}
	result, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next(recovery) error = %v, want fresh recovery", err)
	}
	if result.Kind != ActiveDirectiveKindLoadSteering {
		t.Fatalf("Next(recovery).Kind = %q, want load-steering", result.Kind)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(recovery) = found %v, error %v", found, err)
	}
	if marker.Revision != 1 {
		t.Errorf("recovered marker revision = %d, want fresh publication revision 1", marker.Revision)
	}
}

func TestDeliveryContinueAdvancesExactlyOnce(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}

	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if first.Kind != ActiveDirectiveKindLoadSteering || first.Parts < 3 {
		t.Fatalf("Next() = kind %q parts %d, want load-steering with at least 3 parts", first.Kind, first.Parts)
	}

	second, err := Continue(context.Background(), input, first.ContinueToken)
	if err != nil {
		t.Fatalf("Continue(first token) error = %v", err)
	}
	if second.Kind != ActiveDirectiveKindLoadSteering || second.Part != 2 || second.ContinueToken == first.ContinueToken {
		t.Fatalf("Continue(first token) = kind %q part %d token %q, want part 2 successor", second.Kind, second.Part, second.ContinueToken)
	}
	markerBeforeReplay, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(before replay) = found %v, error %v", found, err)
	}
	if _, err := Continue(context.Background(), input, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(replayed first token) error = %v, want workflow error", err)
	}
	markerAfterReplay, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(after replay) = found %v, error %v", found, err)
	}
	if markerAfterReplay.ContinueToken != markerBeforeReplay.ContinueToken || markerAfterReplay.Revision != markerBeforeReplay.Revision {
		t.Errorf("replay changed marker: before %#v, after %#v", markerBeforeReplay, markerAfterReplay)
	}
}

func TestDeliveryContinueSerializesParallelRequests(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, continueErr := Continue(context.Background(), input, first.ContinueToken)
			results <- continueErr
		}()
	}
	var success, workflow int
	for range 2 {
		continueErr := <-results
		switch {
		case continueErr == nil:
			success++
		case IsWorkflowError(continueErr):
			workflow++
		default:
			t.Errorf("parallel Continue() error = %v, want nil or workflow error", continueErr)
		}
	}
	if success != 1 || workflow != 1 {
		t.Fatalf("parallel Continue() results = success %d, workflow %d, want one each", success, workflow)
	}
}

func TestDeliveryContinueCommitFailureLeavesMarkerRetryable(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	oldMarker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}

	previousCommit := activeDirectiveMarkerCommit
	activeDirectiveMarkerCommit = func(*os.Root, ActiveDirectiveMarker) error {
		return errors.New("injected marker commit failure")
	}
	_, continueErr := Continue(context.Background(), input, first.ContinueToken)
	activeDirectiveMarkerCommit = previousCommit
	if continueErr == nil {
		t.Fatal("Continue(commit failure) error = nil, want error")
	}
	currentMarker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(after failure) = found %v, error %v", found, err)
	}
	if !reflect.DeepEqual(currentMarker, oldMarker) {
		t.Errorf("marker changed after commit failure: got %#v, want %#v", currentMarker, oldMarker)
	}
	if _, err := Continue(context.Background(), input, first.ContinueToken); err != nil {
		t.Fatalf("Continue(retry after commit failure) error = %v, want nil", err)
	}
}

func newChunkedDeliveryFixture(t *testing.T) runStageFixture {
	t.Helper()
	fixture := newRunStageFixture(t)
	rulePath := filepath.Join(fixture.identity.ProjectPath(), "delivery-rule.md")
	writeRunStageFile(t, rulePath, strings.Repeat("delivery rule content\n", 3000))
	graphJSON := strings.Replace(runStageGraphJSON,
		`"consumes":[]`, `"consumes":[],"rules_in_context":[{"path":"delivery-rule.md","scope":"project"}]`, 2)
	writeRunStageFile(t, fixture.stageGraphPath, graphJSON)
	return fixture
}

func TestDeliveryRejectsStaleContinuations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(runStageFixture)
	}{
		{
			name: "rule change",
			mutate: func(fixture runStageFixture) {
				writeRunStageFile(t, filepath.Join(fixture.identity.ProjectPath(), "delivery-rule.md"), "changed rule\n")
			},
		},
		{
			name: "state change",
			mutate: func(fixture runStageFixture) {
				statePath := filepath.Join(fixture.identity.ProjectPath(), "aidlc", "spaces", "team", "intents", "build", "aidlc-state.md")
				stateBytes, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatalf("ReadFile(state): %v", err)
				}
				writeRunStageFile(t, statePath, string(stateBytes)+"\n## Delivery Freshness\nchanged\n")
			},
		},
		{
			name: "route change",
			mutate: func(fixture runStageFixture) {
				path := filepath.Join(fixture.identity.ProjectPath(), ".codex", "tools", "data", "scope-grid.json")
				writeRunStageFile(t, path, strings.Replace(runStageScopeGridJSON, `"next-stage":"EXECUTE"`, `"next-stage":"SKIP"`, 1))
			},
		},
		{
			name: "directive change",
			mutate: func(fixture runStageFixture) {
				path := fixture.stageGraphPath
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(stage graph): %v", err)
				}
				writeRunStageFile(t, path, strings.Replace(string(contents), `"lead_agent":"product-agent"`, `"lead_agent":"orchestrator"`, 1))
			},
		},
		{
			name: "active selection change",
			mutate: func(fixture runStageFixture) {
				writeRunStageFile(t, fixture.activeIntentPath, "other\n")
			},
		},
		{
			name: "harness change",
			mutate: func(fixture runStageFixture) {
				marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
				if err != nil || !found {
					t.Fatalf("ReadActiveDirectiveMarker(harness): found %v, error %v", found, err)
				}
				marker.CursorHarness = "other"
				if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
					t.Fatalf("WriteActiveDirectiveMarker(harness): %v", err)
				}
			},
		},
		{
			name: "attempt binding change",
			mutate: func(fixture runStageFixture) {
				marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
				if err != nil || !found || marker.ActiveAttempt == nil {
					t.Fatalf("ReadActiveDirectiveMarker(attempt): found %v, error %v, marker %#v", found, err, marker)
				}
				marker.ActiveAttempt.IssuedStateSHA256 = sha256Hex("other state")
				if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
					t.Fatalf("WriteActiveDirectiveMarker(attempt): %v", err)
				}
			},
		},
		{
			name: "superseded delivery",
			mutate: func(fixture runStageFixture) {
				marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
				if err != nil || !found {
					t.Fatalf("ReadActiveDirectiveMarker(superseded): found %v, error %v", found, err)
				}
				marker.Delivery = ActiveDirectiveDeliverySuperseded
				if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
					t.Fatalf("WriteActiveDirectiveMarker(superseded): %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newChunkedDeliveryFixture(t)
			input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
			first, err := Next(context.Background(), input)
			if err != nil {
				t.Fatalf("Next() error = %v", err)
			}
			test.mutate(fixture)
			if _, err := Continue(context.Background(), input, first.ContinueToken); err == nil || !IsWorkflowError(err) {
				t.Fatalf("Continue(stale %s) error = %v, want workflow error", test.name, err)
			} else if !errors.Is(err, ErrDeliveryWorkflow) {
				t.Errorf("Continue(stale %s) error = %v, want ErrDeliveryWorkflow", test.name, err)
			}
		})
	}
}

func TestDeliveryContinueRejectsTamperedMarkerStage(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v", found, err)
	}
	if marker.Stage == "next-stage" {
		marker.Stage = "intent-capture"
	} else {
		marker.Stage = "next-stage"
	}
	if err := WriteActiveDirectiveMarker(fixture.recordRoot, marker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(tampered stage): %v", err)
	}
	if _, err := Continue(context.Background(), input, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(tampered stage) error = %v, want workflow error", err)
	}
}

func TestDeliveryRejectsStaleIntentUUID(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	intentUUID := "intent-uuid"
	input := RunStageInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		IntentUUID:  &intentUUID,
	}
	first, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found || marker.IntentUUID == nil || *marker.IntentUUID != intentUUID {
		t.Fatalf("ReadActiveDirectiveMarker() = found %v, error %v, marker %#v; want intent UUID", found, err, marker)
	}
	changedUUID := "changed-intent-uuid"
	input.IntentUUID = &changedUUID
	if _, err := Continue(context.Background(), input, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(changed intent UUID) error = %v, want workflow error", err)
	}
}

func TestDeliveryRejectsStaleTamperedForeignAndCorruptCursors(t *testing.T) {
	firstFixture := newChunkedDeliveryFixture(t)
	firstInput := RunStageInput{Identity: firstFixture.identity, ProjectRoot: firstFixture.projectRoot, RecordRoot: firstFixture.recordRoot}
	first, err := Next(context.Background(), firstInput)
	if err != nil {
		t.Fatalf("Next(first) error = %v", err)
	}
	if _, err := Continue(context.Background(), firstInput, "tampered."+first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(tampered token) error = %v, want workflow error", err)
	}

	foreignFixture := newChunkedDeliveryFixture(t)
	foreignInput := RunStageInput{Identity: foreignFixture.identity, ProjectRoot: foreignFixture.projectRoot, RecordRoot: foreignFixture.recordRoot}
	if _, err := Continue(context.Background(), foreignInput, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(foreign token) error = %v, want workflow error", err)
	}
	tamperedMarker, found, err := ReadActiveDirectiveMarker(firstFixture.recordRoot)
	if err != nil || !found || tamperedMarker.ActiveAttempt == nil {
		t.Fatalf("ReadActiveDirectiveMarker(for tamper) = found %v, error %v, marker %#v", found, err, tamperedMarker)
	}
	tamperedMarker.ActiveAttempt.ResultSHA256 = sha256Hex("tampered published wire")
	if err := WriteActiveDirectiveMarker(firstFixture.recordRoot, tamperedMarker); err != nil {
		t.Fatalf("WriteActiveDirectiveMarker(tampered result): %v", err)
	}
	if _, err := Continue(context.Background(), firstInput, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(tampered marker result) error = %v, want workflow error", err)
	}

	if err := os.WriteFile(filepath.Join(firstFixture.identity.ProjectPath(), "aidlc", "spaces", "team", "intents", "build", activeDirectiveMarkerName), []byte("{broken\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupt marker): %v", err)
	}
	if _, err := Continue(context.Background(), firstInput, first.ContinueToken); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(corrupt marker) error = %v, want workflow error", err)
	}
	if _, err := Continue(context.Background(), firstInput, strings.Repeat("x", 16*1024+1)); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(oversized token) error = %v, want workflow error", err)
	}
	invalidUTF8 := string([]byte{'x', 0xff})
	if _, err := Continue(context.Background(), firstInput, invalidUTF8); err == nil || !IsWorkflowError(err) {
		t.Fatalf("Continue(invalid UTF-8 token) error = %v, want workflow error", err)
	}
}

func TestDeliveryPublishesRunStageWireAfterFinalPart(t *testing.T) {
	fixture := newChunkedDeliveryFixture(t)
	input := RunStageInput{Identity: fixture.identity, ProjectRoot: fixture.projectRoot, RecordRoot: fixture.recordRoot}
	current, err := Next(context.Background(), input)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	for current.Kind == ActiveDirectiveKindLoadSteering {
		current, err = Continue(context.Background(), input, current.ContinueToken)
		if err != nil {
			t.Fatalf("Continue(part %d) error = %v", current.Part, err)
		}
	}
	if current.Kind != ActiveDirectiveKindRunStage {
		t.Fatalf("final directive kind = %q, want run-stage", current.Kind)
	}
	fresh, err := ComposeRunStage(context.Background(), input)
	if err != nil {
		t.Fatalf("ComposeRunStage(final) error = %v", err)
	}
	if !bytes.Equal(current.Wire, fresh.Wire) {
		t.Errorf("final wire = %s, want fresh run-stage wire %s", current.Wire, fresh.Wire)
	}
	marker, found, err := ReadActiveDirectiveMarker(fixture.recordRoot)
	if err != nil || !found {
		t.Fatalf("ReadActiveDirectiveMarker(final) = found %v, error %v", found, err)
	}
	if marker.Kind != ActiveDirectiveKindRunStage || marker.ContinueToken != "" {
		t.Errorf("final marker = %#v, want run-stage without token", marker)
	}
}
