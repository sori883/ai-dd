package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestIntentCaptureStageCoreCompatibility(t *testing.T) {
	ctx := context.Background()
	legacy := newStageReceiptFixture(t, "intent-capture")
	generic := newStageReceiptFixture(t, "intent-capture")
	if err := RecordIntentCaptureDecisionFromQuestionsWithOptions(ctx, legacy.identity, legacy.projectRoot, legacy.recordRoot, "intent-capture", "q1", "yes,no"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStageQuestionDecision(ctx, generic.identity, generic.projectRoot, generic.recordRoot, "intent-capture", "q1", "yes,no"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []humanTurnWorkspaceFixture{legacy, generic} {
		if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
			t.Fatal(err)
		}
	}
	if err := RecordIntentCaptureAnswerByID(ctx, legacy.identity, legacy.projectRoot, legacy.recordRoot, "intent-capture", "q1", "yes"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStageQuestionAnswer(ctx, generic.identity, generic.projectRoot, generic.recordRoot, "intent-capture", "q1", "yes"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []humanTurnWorkspaceFixture{legacy, generic} {
		writeStageQuestions(t, f, "intent-capture", stageSummaryBlank)
	}
	decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value", Options: "forged"}
	if err := RecordIntentCaptureDecision(ctx, legacy.identity, legacy.projectRoot, legacy.recordRoot, decision); err != nil {
		t.Fatal(err)
	}
	if err := RecordStageSummaryDecision(ctx, generic.identity, generic.projectRoot, generic.recordRoot, "intent-capture"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []humanTurnWorkspaceFixture{legacy, generic} {
		if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
			t.Fatal(err)
		}
		writeStageQuestions(t, f, "intent-capture", strings.Replace(stageSummaryBlank, "[Answer]: \n", "[Answer]: Looks correct\n", 1))
	}
	if err := RecordSummaryConfirmation(ctx, legacy.identity, legacy.projectRoot, legacy.recordRoot, decision, "Looks correct", "forged"); err != nil {
		t.Fatal(err)
	}
	if err := RecordStageSummaryConfirmation(ctx, generic.identity, generic.projectRoot, generic.recordRoot, "intent-capture", "Looks correct"); err != nil {
		t.Fatal(err)
	}
	want, got := stageReceiptRecords(t, legacy), stageReceiptRecords(t, generic)
	if len(want) != len(got) {
		t.Fatalf("record count: %d != %d", len(want), len(got))
	}
	for index := range want {
		if want[index].Event != got[index].Event || !reflect.DeepEqual(want[index].Fields, got[index].Fields) {
			t.Fatalf("record %d differs: %#v != %#v", index, want[index], got[index])
		}
	}
	if err := ValidateSummaryConfirmationCurrent(legacy.recordRoot, want[len(want)-1]); err != nil {
		t.Fatal(err)
	}
	if err := RecordSummaryConfirmation(ctx, legacy.identity, legacy.projectRoot, legacy.recordRoot, decision, "Looks correct", "forged"); !errors.Is(err, ErrIntentCaptureStale) {
		t.Fatalf("legacy stale chain: %v", err)
	}
	if err := RecordStageSummaryConfirmation(ctx, generic.identity, generic.projectRoot, generic.recordRoot, "intent-capture", "Looks correct"); !errors.Is(err, ErrIntentCaptureStale) {
		t.Fatalf("generic stale chain: %v", err)
	}
}

func TestSummaryConfirmationDerivesAndRevalidatesQuestionsDigest(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	questions := []byte("# Questions\n\n[Q1] What is the goal?\n\n## Consolidated Summary Confirmation\n[Answer]: \n")
	if err := os.WriteFile(questionsPath, questions, 0o600); err != nil {
		t.Fatalf("WriteFile(questions): %v", err)
	}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value"}); err != nil {
		t.Fatalf("RecordIntentCaptureDecision(summary): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	questions = []byte("# Questions\n\n[Q1] What is the goal?\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n")
	if err := os.WriteFile(questionsPath, questions, 0o600); err != nil {
		t.Fatalf("WriteFile(answered questions): %v", err)
	}
	if err := RecordSummaryConfirmation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value"}, "Looks correct", "caller-digest"); err != nil {
		t.Fatalf("RecordSummaryConfirmation(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %#v, want decision/turn/summary", records)
	}
	wantDigest, err := summaryConfirmationContentHash(questions)
	if err != nil {
		t.Fatalf("summaryConfirmationContentHash(): %v", err)
	}
	if got := records[2].Fields["Questions SHA-256"]; got != wantDigest {
		t.Fatalf("Questions SHA-256 = %q, want derived digest", got)
	}
	if got := records[2].Fields["Questions File"]; got != "ideation/intent-capture/intent-capture-questions.md" {
		t.Fatalf("Questions File = %q, want fixed stage-relative path", got)
	}
	if err := os.WriteFile(questionsPath, []byte("# Changed Questions\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed questions): %v", err)
	}
	if err := ValidateSummaryConfirmationCurrent(fixture.recordRoot, records[2]); err == nil {
		t.Fatal("ValidateSummaryConfirmationCurrent() error = nil, want changed questions rejection")
	}
}

func TestSummaryConfirmationUsesCanonicalFieldContract(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	blank := "# Questions\n\n## Q1\nWhat is the goal?\n[Answer]: yes\n\n## Consolidated Summary Confirmation\n- Looks correct\n- Request changes\n[Answer]: \n"
	if err := os.WriteFile(questionsPath, []byte(blank), 0o600); err != nil {
		t.Fatalf("WriteFile(blank questions): %v", err)
	}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller", Options: "forged"}); err != nil {
		t.Fatalf("RecordIntentCaptureDecision(summary): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	answered := strings.Replace(blank, "[Answer]: \n", "[Answer]: Looks correct\n", 1)
	if err := os.WriteFile(questionsPath, []byte(answered), 0o600); err != nil {
		t.Fatalf("WriteFile(answered questions): %v", err)
	}
	if err := RecordSummaryConfirmation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller"}, "Looks correct", "forged"); err != nil {
		t.Fatalf("RecordSummaryConfirmation(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %#v, want summary decision/turn/confirmation", records)
	}
	if got := records[0].Fields["Options"]; got != "Looks correct,Request changes" {
		t.Fatalf("summary decision Options = %q, want fixed choice set", got)
	}
	wantFields := map[string]string{
		"Stage":             "intent-capture",
		"Details":           "Looks correct",
		"Checkpoint":        "Consolidated Summary Confirmation",
		"Questions File":    intentCaptureQuestionsFile,
		"Questions SHA-256": "",
		"Hash Scope":        intentCaptureHashScope,
	}
	for key, want := range wantFields {
		if key == "Questions SHA-256" {
			if records[2].Fields[key] == "" {
				t.Errorf("summary field %q is empty", key)
			}
			continue
		}
		if got := records[2].Fields[key]; got != want {
			t.Errorf("summary field %q = %q, want %q", key, got, want)
		}
	}
	for _, forbidden := range []string{"Answer", "Decision"} {
		if _, ok := records[2].Fields[forbidden]; ok {
			t.Errorf("summary receipt contains noncanonical authority field %q: %#v", forbidden, records[2].Fields)
		}
	}
	if len(records[2].Fields) != len(wantFields) {
		t.Fatalf("summary fields = %#v, want exactly canonical fields", records[2].Fields)
	}
}

func TestIntentCaptureAnswerEmitsCanonicalDetails(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "q-details", Fingerprint: "question-v1"}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision); err != nil {
		t.Fatalf("RecordIntentCaptureDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	if err := RecordIntentCaptureAnswer(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision, "A"); err != nil {
		t.Fatalf("RecordIntentCaptureAnswer(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if got := records[len(records)-1].Fields["Details"]; got != "A" {
		t.Fatalf("QUESTION_ANSWERED Details = %q, want answer text", got)
	}
}

func TestWriteProjectArtifactAtomicFailuresPreservePublishedContent(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	name := filepath.Join("aidlc", "spaces", "team", "memory", "project.md")
	if err := writeProjectArtifact(fixture.projectRoot, name, []byte("old\n")); err != nil {
		t.Fatalf("writeProjectArtifact(initial): %v", err)
	}
	failureCases := map[string]func(*projectArtifactOps){
		"short write": func(ops *projectArtifactOps) {
			ops.write = func(file *os.File, content []byte) (int, error) { return len(content) - 1, io.ErrShortWrite }
		},
		"close failure": func(ops *projectArtifactOps) {
			ops.close = func(file *os.File) error { _ = file.Close(); return errors.New("close failed") }
		},
		"rename failure": func(ops *projectArtifactOps) {
			ops.rename = func(*os.Root, string, string) error { return errors.New("rename failed") }
		},
	}
	for nameCase, configure := range failureCases {
		t.Run(nameCase, func(t *testing.T) {
			ops := defaultProjectArtifactOps()
			configure(&ops)
			if err := writeProjectArtifactWithOps(fixture.projectRoot, name, []byte("new\n"), ops); err == nil {
				t.Fatal("writeProjectArtifactWithOps() error = nil, want injected failure")
			}
			published, err := os.ReadFile(filepath.Join(fixture.project, filepath.FromSlash(name)))
			if err != nil {
				t.Fatalf("ReadFile(published): %v", err)
			}
			if string(published) != "old\n" {
				t.Fatalf("published content = %q, want old content preserved", published)
			}
			entries, err := os.ReadDir(filepath.Dir(filepath.Join(fixture.project, filepath.FromSlash(name))))
			if err != nil {
				t.Fatalf("ReadDir(memory): %v", err)
			}
			for _, entry := range entries {
				if strings.Contains(entry.Name(), ".tmp-") {
					t.Fatalf("temporary artifact %q remains after failure", entry.Name())
				}
			}
		})
	}
}

func TestWriteProjectArtifactAtomicNewFileDoesNotPublishPartialContent(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	name := filepath.Join("aidlc", "spaces", "team", "memory", "new.md")
	ops := defaultProjectArtifactOps()
	ops.write = func(file *os.File, content []byte) (int, error) { return len(content) - 1, io.ErrShortWrite }
	if err := writeProjectArtifactWithOps(fixture.projectRoot, name, []byte("partial\n"), ops); err == nil {
		t.Fatal("writeProjectArtifactWithOps(new) error = nil, want short write")
	}
	if _, err := os.Stat(filepath.Join(fixture.project, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new artifact stat error = %v, want no published target", err)
	}
}

func TestSummaryConfirmationRejectsLegacyRawHashWithoutScope(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	questions := []byte("## Q1\n\n[Q1] What is the goal?\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n")
	if err := os.WriteFile(questionsPath, questions, 0o600); err != nil {
		t.Fatalf("WriteFile(questions): %v", err)
	}
	raw := sha256.Sum256(questions)
	record := AuditRecord{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: map[string]string{
		"Stage": "intent-capture", "Answer": "Looks correct", "Details": "Looks correct",
		"Checkpoint": "Consolidated Summary Confirmation", "Questions File": intentCaptureQuestionsFile,
		"Questions SHA-256": hex.EncodeToString(raw[:]),
	}}
	if err := ValidateSummaryConfirmationCurrent(fixture.recordRoot, record); err == nil {
		t.Fatal("ValidateSummaryConfirmationCurrent() error = nil, want legacy raw hash without fixed scope rejected")
	}
}

func TestSummaryConfirmationRequiresCanonicalCheckpoint(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	if err := os.WriteFile(questionsPath, []byte("## Q1\n\n[Q1] What is the goal?\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(questions): %v", err)
	}
	decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value"}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision); !errors.Is(err, ErrIntentCaptureAmbiguous) {
		t.Fatalf("RecordIntentCaptureDecision(summary) error = %v, want ErrIntentCaptureAmbiguous", err)
	}
}

func TestSummaryConfirmationRejectsHiddenOrDuplicateCheckpoint(t *testing.T) {
	cases := map[string]string{
		"duplicate checkpoint":                     "## Q1\n\nQuestion\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n",
		"hidden checkpoint":                        "## Q1\n\nQuestion\n\n~~~markdown\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n~~~\n",
		"raw html heading":                         "## Consolidated Summary Confirmation\n[Answer]: Looks correct\n<h2>Unreviewed</h2>\n",
		"setext heading":                           "## Consolidated Summary Confirmation\n[Answer]: Looks correct\nUnreviewed\n----\n",
		"duplicate pre-summary assumption":         "## Assumption Confirmation\n- one\n\n## Assumption Confirmation\n- two\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n",
		"duplicate pre-summary feedback":           "## Requested Changes Feedback\n- one\n\n## Requested Changes Feedback\n- two\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n",
		"duplicate question number before summary": "## Q1. First question\n\nQuestion\n\n## Q1. Second question\n\nQuestion\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n",
		"duplicate question number across summary": "## Q1. First question\n\nQuestion\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Q1. Repeated question\n\nQuestion\n",
		"duplicate post-summary feedback":          "## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Requested Changes Feedback\n- one\n\n## Requested Changes Feedback\n- two\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newHumanTurnWorkspaceFixture(t)
			questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
			if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
				t.Fatalf("MkdirAll(questions): %v", err)
			}
			if err := os.WriteFile(questionsPath, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile(questions): %v", err)
			}
			decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value"}
			err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision)
			if err == nil {
				if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
					t.Fatalf("RecordHumanTurn(): %v", err)
				}
				err = RecordSummaryConfirmation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision, "Looks correct", "ignored")
			}
			if !errors.Is(err, ErrIntentCaptureAmbiguous) {
				t.Fatalf("summary validation error = %v, want ErrIntentCaptureAmbiguous", err)
			}
		})
	}
}

func TestSummaryConfirmationExcludesOnlyPostSummaryAssumptionSection(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	questionsPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "intent-capture-questions.md")
	if err := os.MkdirAll(filepath.Dir(questionsPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(questions): %v", err)
	}
	questions := []byte("## Q1\n\n[Q1] What is the goal?\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Assumption Confirmation\n- one assumption\n")
	if err := os.WriteFile(questionsPath, questions, 0o600); err != nil {
		t.Fatalf("WriteFile(questions): %v", err)
	}
	decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "summary", Fingerprint: "caller-value"}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision); err != nil {
		t.Fatalf("RecordIntentCaptureDecision(summary): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	if err := RecordSummaryConfirmation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision, "Looks correct", "ignored"); err != nil {
		t.Fatalf("RecordSummaryConfirmation(): %v", err)
	}

	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %#v, want decision/turn/summary", records)
	}
	if err := os.WriteFile(questionsPath, []byte("## Q1\n\n[Q1] What is the goal?\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Assumption Confirmation\n- a different assumption\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed assumption): %v", err)
	}
	if err := ValidateSummaryConfirmationCurrent(fixture.recordRoot, records[2]); err != nil {
		t.Fatalf("ValidateSummaryConfirmationCurrent() error = %v, want nil after assumption-only change", err)
	}
}

func TestSummaryConfirmationCanonicalizesLineEndingsAndIncludesFollowUps(t *testing.T) {
	base := []byte("## Sources\n- source [desc]\n\n## Q1\n[Answer]: A\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Assumption Confirmation\n- assumption one\n\n## Q2\n[Answer]: A\n\n## Requested Changes Feedback\n[Answer]: follow up\n")
	withDifferentAssumption := []byte("## Sources\n- source [desc]\n\n## Q1\n[Answer]: A\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Assumption Confirmation\n- assumption two\n\n## Q2\n[Answer]: A\n\n## Requested Changes Feedback\n[Answer]: follow up\n")
	withDifferentFollowUp := []byte("## Sources\n- source [desc]\n\n## Q1\n[Answer]: A\n\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n\n## Assumption Confirmation\n- assumption one\n\n## Q2\n[Answer]: B\n\n## Requested Changes Feedback\n[Answer]: follow up\n")
	first, err := summaryConfirmationContentHash(base)
	if err != nil {
		t.Fatalf("summaryConfirmationContentHash(base): %v", err)
	}
	assumption, err := summaryConfirmationContentHash(withDifferentAssumption)
	if err != nil {
		t.Fatalf("summaryConfirmationContentHash(assumption): %v", err)
	}
	if first != assumption {
		t.Fatal("post-summary assumption body changed the confirmed-content-v1 digest")
	}
	followUp, err := summaryConfirmationContentHash(withDifferentFollowUp)
	if err != nil {
		t.Fatalf("summaryConfirmationContentHash(follow-up): %v", err)
	}
	if first == followUp {
		t.Fatal("follow-up answer did not change the confirmed-content-v1 digest")
	}
	crlf := bytes.ReplaceAll(base, []byte("\n"), []byte("\r\n"))
	crlf = bytes.Replace(crlf, []byte("[Answer]: Looks correct\r\n"), []byte("[Answer]: Looks correct   \r\n"), 1)
	lineEnding, err := summaryConfirmationContentHash(crlf)
	if err != nil {
		t.Fatalf("summaryConfirmationContentHash(CRLF): %v", err)
	}
	if first != lineEnding {
		t.Fatal("line ending or trailing whitespace changed the confirmed-content-v1 digest")
	}
}

func TestIntentCaptureQuestionPairingRejectsReaskingAfterRevision(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	decision := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "q-1", Fingerprint: "question-v1"}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision); err != nil {
		t.Fatalf("first RecordIntentCaptureDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("first RecordHumanTurn(): %v", err)
	}
	if err := RecordIntentCaptureAnswer(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision, "first"); err != nil {
		t.Fatalf("first RecordIntentCaptureAnswer(): %v", err)
	}
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []Event{
			{Event: "GATE_REJECTED", Fields: map[string]string{"Stage": "intent-capture"}},
			{Event: "STAGE_REVISING", Fields: map[string]string{"Stage": "intent-capture"}},
		})
	}); err != nil {
		t.Fatalf("append revision anchor: %v", err)
	}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, decision); !errors.Is(err, ErrIntentCaptureStale) {
		t.Fatalf("fresh revision RecordIntentCaptureDecision(): %v, want ErrIntentCaptureStale for re-asked decision", err)
	}
	clarification := IntentCaptureDecision{Stage: "intent-capture", DecisionID: "q-2", Fingerprint: "clarification-v1"}
	if err := RecordIntentCaptureDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, clarification); err != nil {
		t.Fatalf("clarification RecordIntentCaptureDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("clarification RecordHumanTurn(): %v", err)
	}
	if err := RecordIntentCaptureAnswer(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, clarification, "clarified"); err != nil {
		t.Fatalf("clarification RecordIntentCaptureAnswer(): %v", err)
	}
}

func TestIntentCaptureDecisionAnswerPair(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	appendEvent := func(event Event) {
		t.Helper()
		err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
			return appendIntentCaptureForIdentity(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, []Event{event})
		})
		if err != nil {
			t.Fatalf("Append(%s): %v", event.Event, err)
		}
	}
	appendEvent(Event{Event: "DECISION_RECORDED", Fields: map[string]string{
		"Stage": "intent-capture", "Decision": "q-1", "Question Fingerprint": "question-v1",
	}})
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	appendEvent(Event{Event: "QUESTION_ANSWERED", Fields: map[string]string{
		"Stage": "intent-capture", "Decision": "q-1", "Answer": "A", "Question Fingerprint": "question-v1",
	}})

	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("records = %#v, want decision, human turn, answer", records)
	}
	if records[0].Event != "DECISION_RECORDED" || records[1].Event != "HUMAN_TURN" || records[2].Event != "QUESTION_ANSWERED" {
		t.Fatalf("event order = %q, %q, %q", records[0].Event, records[1].Event, records[2].Event)
	}
}

func TestGenericAppendRejectsIntentCaptureOwnedEvents(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	events := []Event{
		{Event: "HUMAN_TURN", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "DECISION_RECORDED", Fields: map[string]string{"Stage": "intent-capture", "Decision": "q-1"}},
		{Event: "QUESTION_ANSWERED", Fields: map[string]string{"Stage": "intent-capture", "Decision": "q-1"}},
		{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "REVIEW_REQUESTED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "REVIEW_COMPLETED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_FIRED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_COMPLETED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_PASSED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_FAILED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_BUDGET_OVERRIDE", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "RULE_LEARNED", Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "SENSOR_PROPOSED", Fields: map[string]string{"Stage": "intent-capture"}},
	}
	for _, event := range events {
		event := event
		t.Run(event.Event, func(t *testing.T) {
			err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
				return Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []Event{event})
			})
			if !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("generic Append(%s) error = %v, want ErrInvalidEvent", event.Event, err)
			}
		})
	}
	if _, err := fixture.recordRoot.Lstat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generic owned-event attempts created audit = %v", err)
	}
}

func TestSummaryConfirmationReceipt(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	appendEvent := func(event Event) {
		t.Helper()
		err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
			return appendIntentCaptureForIdentity(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, []Event{event})
		})
		if err != nil {
			t.Fatalf("Append(%s): %v", event.Event, err)
		}
	}
	appendEvent(Event{Event: "DECISION_RECORDED", Fields: map[string]string{
		"Stage": "intent-capture", "Decision": "summary", "Question Fingerprint": "summary-v1",
	}})
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	appendEvent(Event{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: map[string]string{
		"Stage": "intent-capture", "Decision": "summary", "Answer": "Looks correct", "Hash Scope": "confirmed-content-v1", "Content Fingerprint": "summary-v1",
	}})
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 3 || records[2].Event != "SUMMARY_CONFIRMATION_RECORDED" {
		t.Fatalf("records = %#v, want summary confirmation after fresh turn", records)
	}
	if got := records[2].Fields["Answer"]; got != "Looks correct" {
		t.Errorf("summary Answer = %q, want Looks correct", got)
	}
	if got := records[2].Fields["Hash Scope"]; got != "confirmed-content-v1" {
		t.Errorf("summary Hash Scope = %q, want confirmed-content-v1", got)
	}
}

func TestSummaryConfirmationRejectsStaleAmbiguousOrChangedQuestions(t *testing.T) {
	if err := validateIntentCaptureSummaryConfirmation("Request changes", "summary-v1", "summary-v1"); err == nil {
		t.Fatal("validateIntentCaptureSummaryConfirmation(wrong answer) error = nil, want rejection")
	}
	if err := validateIntentCaptureSummaryConfirmation("Looks correct", "summary-v1", "changed"); err == nil {
		t.Fatal("validateIntentCaptureSummaryConfirmation(changed fingerprint) error = nil, want rejection")
	}
	if err := validateIntentCaptureSummaryConfirmation("Looks correct", "summary-v1", "summary-v1"); err != nil {
		t.Fatalf("validateIntentCaptureSummaryConfirmation(valid) error = %v, want nil", err)
	}
	if !errors.Is(validateIntentCaptureSummaryConfirmation("Looks correct", "", ""), ErrIntentCaptureAmbiguous) {
		t.Fatal("missing fingerprint did not fail closed with ErrIntentCaptureAmbiguous")
	}
}
