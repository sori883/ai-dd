package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
)

const stageSummaryBlank = "# Questions\n\n## Q1\nGoal?\n[Answer]: ship\n\n## Consolidated Summary Confirmation\n- Looks correct\n- Request changes\n[Answer]: \n"

func TestStageReceiptCompletedEpochRejectsOperations(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, operation := range []string{"question decision", "question answer", "summary decision", "summary confirmation", "current validation"} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, operation), func(t *testing.T) {
				stage := "market-research"
				if legacy {
					stage = "intent-capture"
				}
				f := newStageReceiptFixture(t, stage)
				ctx := context.Background()
				answered := strings.Replace(stageSummaryBlank, "[Answer]: \n", "[Answer]: Looks correct\n", 1)
				writeStageQuestions(t, f, stage, answered)
				if err := RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage); err != nil {
					t.Fatal(err)
				}
				if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
					t.Fatal(err)
				}
				if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "Looks correct"); err != nil {
					t.Fatal(err)
				}
				if operation == "question answer" {
					if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q2", ""); err != nil {
						t.Fatal(err)
					}
					if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
						t.Fatal(err)
					}
				}
				if operation == "summary decision" || operation == "summary confirmation" {
					writeStageQuestions(t, f, stage, strings.Replace(answered, "ship", "revised", 1))
				}
				if operation == "summary confirmation" {
					if err := RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage); err != nil {
						t.Fatal(err)
					}
					if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
						t.Fatal(err)
					}
				}
				appendStageReceiptEvent(t, f, Event{Event: "STAGE_COMPLETED", Fields: map[string]string{"Stage": stage}})
				before := stageReceiptRecords(t, f)
				var err error
				decision := IntentCaptureDecision{Stage: stage, DecisionID: "summary", Fingerprint: "summary"}
				switch operation {
				case "question decision":
					if legacy {
						err = RecordIntentCaptureDecisionFromQuestions(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q3")
					} else {
						err = RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q3", "")
					}
				case "question answer":
					if legacy {
						err = RecordIntentCaptureAnswerByID(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q2", "yes")
					} else {
						err = RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q2", "yes")
					}
				case "summary decision":
					if legacy {
						err = RecordIntentCaptureDecision(ctx, f.identity, f.projectRoot, f.recordRoot, decision)
					} else {
						err = RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage)
					}
				case "summary confirmation":
					if legacy {
						err = RecordSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, decision, "Looks correct", "")
					} else {
						err = RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "Looks correct")
					}
				case "current validation":
					err = ValidateStageSummaryConfirmationCurrent(ctx, f.identity, f.projectRoot, f.recordRoot, stage)
				}
				if !errors.Is(err, ErrIntentCaptureStale) {
					t.Errorf("completed epoch %s: %v, want stale", operation, err)
				}
				if after := stageReceiptRecords(t, f); !reflect.DeepEqual(before, after) {
					t.Error("completed epoch operation changed audit")
				}
			})
		}
	}
}

func TestStageReceiptCompletedEpochReopensAndIgnoresOtherStages(t *testing.T) {
	for _, name := range []string{"reopened", "other stage"} {
		t.Run(name, func(t *testing.T) {
			f := newStageReceiptFixture(t, "market-research")
			completedStage := "market-research"
			if name == "other stage" {
				completedStage = "scope-definition"
			}
			appendStageReceiptEvent(t, f, Event{Event: "STAGE_COMPLETED", Fields: map[string]string{"Stage": completedStage}})
			if name == "reopened" {
				appendStageReceiptEvent(t, f, Event{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": "market-research"}})
			}
			if err := RecordStageQuestionDecision(context.Background(), f.identity, f.projectRoot, f.recordRoot, "market-research", "q1", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStageSummaryConfirmationLifecycle(t *testing.T) {
	for _, stage := range []string{"intent-capture", "market-research", "scope-definition"} {
		t.Run(stage, func(t *testing.T) {
			f := newStageReceiptFixture(t, stage)
			ctx := context.Background()
			writeStageQuestions(t, f, stage, stageSummaryBlank)
			if err := RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage); err != nil {
				t.Fatal(err)
			}
			answered := strings.Replace(stageSummaryBlank, "[Answer]: \n", "[Answer]: Looks correct\n", 1)
			writeStageQuestions(t, f, stage, answered)
			if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "Looks correct"); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("without human: %v", err)
			}
			if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
				t.Fatal(err)
			}
			if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "Looks correct"); err != nil {
				t.Fatal(err)
			}
			records := stageReceiptRecords(t, f)
			digest, err := summaryConfirmationContentHash([]byte(answered))
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"Stage": stage, "Details": "Looks correct", "Checkpoint": "Consolidated Summary Confirmation", "Questions File": "ideation/" + stage + "/" + stage + "-questions.md", "Questions SHA-256": digest, "Hash Scope": "confirmed-content-v1"}
			if len(records) != 3 || !reflect.DeepEqual(records[2].Fields, want) {
				t.Fatalf("summary receipt = %#v", records)
			}
			if records[0].Fields["Questions File"] != want["Questions File"] || records[0].Fields["Options"] != "Looks correct,Request changes" {
				t.Fatalf("decision = %#v", records[0])
			}
			if err := ValidateStageSummaryConfirmationCurrent(ctx, f.identity, f.projectRoot, f.recordRoot, stage); err != nil {
				t.Fatal(err)
			}
			if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "Looks correct"); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("duplicate: %v", err)
			}
			writeStageQuestions(t, f, stage, strings.Replace(answered, "ship", "changed", 1))
			if err := ValidateStageSummaryConfirmationCurrent(ctx, f.identity, f.projectRoot, f.recordRoot, stage); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("changed content: %v", err)
			}
		})
	}
}

func TestStageSummaryConfirmationRejectsInvalidAuthority(t *testing.T) {
	for _, name := range []string{"wrong answer", "missing decision", "old epoch", "two turns", "hidden heading", "duplicate heading", "wrong stage", "ambiguous shards", "wrong decision path"} {
		t.Run(name, func(t *testing.T) {
			f := newStageReceiptFixture(t, "market-research")
			ctx := context.Background()
			writeStageQuestions(t, f, "market-research", stageSummaryBlank)
			if name != "missing decision" {
				if err := RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research"); err != nil {
					t.Fatal(err)
				}
			}
			if name == "old epoch" {
				appendStageReceiptEvent(t, f, Event{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": "market-research"}})
			}
			if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
				t.Fatal(err)
			}
			answered := strings.Replace(stageSummaryBlank, "[Answer]: \n", "[Answer]: Looks correct\n", 1)
			stage, answer := "market-research", "Looks correct"
			switch name {
			case "wrong answer":
				answer = "Looks correct "
			case "two turns":
				if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
					t.Fatal(err)
				}
			case "hidden heading":
				answered = strings.Replace(answered, "## Consolidated Summary Confirmation", "<!--\n## Consolidated Summary Confirmation", 1) + "-->\n"
			case "duplicate heading":
				answered += "\n## Consolidated Summary Confirmation\n[Answer]: Looks correct\n"
			case "wrong stage":
				stage = "scope-definition"
			case "ambiguous shards":
				duplicateStageShard(t, f)
			case "wrong decision path":
				appendStageReceiptEvent(t, f, Event{Event: "DECISION_RECORDED", Fields: map[string]string{"Stage": stage, "Decision": "summary", "Checkpoint": "Consolidated Summary Confirmation", "Questions File": "ideation/intent-capture/intent-capture-questions.md", "Options": "Looks correct,Request changes"}})
				if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
					t.Fatal(err)
				}
			}
			writeStageQuestions(t, f, "market-research", answered)
			before := len(stageReceiptRecords(t, f))
			if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, stage, answer); err == nil {
				t.Fatal("accepted invalid summary")
			}
			if after := len(stageReceiptRecords(t, f)); after != before {
				t.Fatalf("rejection mutated audit: %d -> %d", before, after)
			}
		})
	}
}

func TestStageSummaryConfirmationCurrentRejectsStaleReceipt(t *testing.T) {
	for _, name := range []string{"new epoch", "new decision", "wrong path", "wrong checkpoint", "wrong scope", "wrong details", "ambiguous shards"} {
		t.Run(name, func(t *testing.T) {
			f := newStageReceiptFixture(t, "market-research")
			ctx := context.Background()
			answered := strings.Replace(stageSummaryBlank, "[Answer]: \n", "[Answer]: Looks correct\n", 1)
			writeStageQuestions(t, f, "market-research", answered)
			if err := RecordStageSummaryDecision(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research"); err != nil {
				t.Fatal(err)
			}
			if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
				t.Fatal(err)
			}
			if err := RecordStageSummaryConfirmation(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research", "Looks correct"); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "new epoch":
				appendStageReceiptEvent(t, f, Event{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": "market-research"}})
			case "new decision":
				appendStageReceiptEvent(t, f, Event{Event: "DECISION_RECORDED", Fields: map[string]string{"Stage": "market-research", "Decision": "summary"}})
			case "ambiguous shards":
				duplicateStageShard(t, f)
			default:
				records := stageReceiptRecords(t, f)
				fields := cloneStringMap(records[len(records)-1].Fields)
				key := map[string]string{"wrong path": "Questions File", "wrong checkpoint": "Checkpoint", "wrong scope": "Hash Scope", "wrong details": "Details"}[name]
				fields[key] = "wrong"
				appendStageReceiptEvent(t, f, Event{Event: "SUMMARY_CONFIRMATION_RECORDED", Fields: fields})
			}
			if err := ValidateStageSummaryConfirmationCurrent(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research"); err == nil {
				t.Fatal("accepted stale receipt")
			}
		})
	}
}

func duplicateStageShard(t *testing.T, f humanTurnWorkspaceFixture) {
	t.Helper()
	records := stageReceiptRecords(t, f)
	content, err := f.recordRoot.ReadFile(records[0].Shard)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.recordRoot.WriteFile("audit/other.md", content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStageSummaryContractFixedCatalog(t *testing.T) {
	catalog, err := graph.Load(os.DirFS("../../../docs/配布_ai-dlc/.codex/tools/data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range catalog.Stages() {
		if stage.Phase != "ideation" {
			continue
		}
		t.Run(stage.Slug, func(t *testing.T) {
			got, err := resolveStageSummaryContract(stage, catalog)
			if err != nil {
				t.Fatal(err)
			}
			want := "ideation/" + stage.Slug + "/" + stage.Slug + "-questions.md"
			if got.questionsFile != want {
				t.Fatalf("questions file = %q, want %q", got.questionsFile, want)
			}
		})
	}
}

func TestStageQuestionReceiptAuthority(t *testing.T) {
	for _, stage := range []string{"intent-capture", "market-research", "scope-definition"} {
		t.Run(stage, func(t *testing.T) {
			f := newStageReceiptFixture(t, stage)
			ctx := context.Background()
			if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", "yes,no"); err != nil {
				t.Fatal(err)
			}
			if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", "yes"); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("without human turn: %v", err)
			}
			if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
				t.Fatal(err)
			}
			if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", "yes"); err != nil {
				t.Fatal(err)
			}
			records := stageReceiptRecords(t, f)
			want := fmt.Sprintf("%x", sha256.Sum256([]byte("# Questions\n")))
			if len(records) != 3 || records[0].Fields["Question Fingerprint"] != want || records[2].Fields["Question Fingerprint"] != want || records[2].Fields["Details"] != "yes" {
				t.Fatalf("receipts = %#v", records)
			}
			if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", "yes"); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("duplicate answer: %v", err)
			}
			appendStageReceiptEvent(t, f, Event{Event: "STAGE_REVISING", Fields: map[string]string{"Stage": stage}})
			if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", ""); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("reasked in epoch: %v", err)
			}
			appendStageReceiptEvent(t, f, Event{Event: "STAGE_STARTED", Fields: map[string]string{"Stage": stage}})
			if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", "yes"); !errors.Is(err, ErrIntentCaptureStale) {
				t.Fatalf("old epoch: %v", err)
			}
			if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, "q1", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStageQuestionReceiptRejectsInvalidAuthority(t *testing.T) {
	for _, name := range []string{"wrong stage", "missing decision", "two human turns", "missing catalog", "summary id", "wrong root"} {
		t.Run(name, func(t *testing.T) {
			f := newStageReceiptFixture(t, "market-research")
			ctx := context.Background()
			stage, id := "market-research", "q1"
			if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, stage, id, ""); err != nil {
				t.Fatal(err)
			}
			if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
				t.Fatal(err)
			}
			root := f.recordRoot
			switch name {
			case "wrong stage":
				stage = "scope-definition"
			case "missing decision":
				id = "q2"
			case "two human turns":
				if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
					t.Fatal(err)
				}
			case "missing catalog":
				if err := f.projectRoot.Remove(".codex/tools/data/stage-graph.json"); err != nil {
					t.Fatal(err)
				}
			case "summary id":
				id = "summary"
			case "wrong root":
				root = f.projectRoot
			}
			before := len(stageReceiptRecords(t, f))
			if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, root, stage, id, "yes"); err == nil {
				t.Fatal("accepted invalid authority")
			}
			if after := len(stageReceiptRecords(t, f)); after != before {
				t.Fatalf("rejection appended receipt: %d -> %d", before, after)
			}
		})
	}
}

func TestStageQuestionReceiptRejectsCrossShardAmbiguity(t *testing.T) {
	f := newStageReceiptFixture(t, "market-research")
	ctx := context.Background()
	if err := RecordStageQuestionDecision(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research", "q1", ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordHumanTurn(ctx, f.identity, f.projectRoot, f.recordRoot); err != nil {
		t.Fatal(err)
	}
	records := stageReceiptRecords(t, f)
	content, err := f.recordRoot.ReadFile(records[0].Shard)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.recordRoot.WriteFile("audit/other.md", content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RecordStageQuestionAnswer(ctx, f.identity, f.projectRoot, f.recordRoot, "market-research", "q1", "yes"); !errors.Is(err, ErrIntentCaptureAmbiguous) {
		t.Fatalf("ambiguous ordering: %v", err)
	}
}

func newStageReceiptFixture(t *testing.T, stage string) humanTurnWorkspaceFixture {
	t.Helper()
	f := newHumanTurnWorkspaceFixture(t)
	stateBytes, err := f.recordRoot.ReadFile("aidlc-state.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.recordRoot.WriteFile("aidlc-state.md", []byte(strings.ReplaceAll(string(stateBytes), "intent-capture", stage)), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"stage-graph.json", "scope-grid.json"} {
		data, err := os.ReadFile(filepath.Join("../../../docs/配布_ai-dlc/.codex/tools/data", name))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(f.project, ".codex/tools/data", name)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeStageQuestions(t, f, stage, "# Questions\n")
	return f
}

func writeStageQuestions(t *testing.T, f humanTurnWorkspaceFixture, stage, content string) {
	t.Helper()
	target := filepath.Join(f.project, "aidlc/spaces/team/intents/build/ideation", stage, stage+"-questions.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func stageReceiptRecords(t *testing.T, f humanTurnWorkspaceFixture) []AuditRecord {
	t.Helper()
	var records []AuditRecord
	err := recordlock.With(context.Background(), f.identity, func(g *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), f.identity, g, f.projectRoot, f.recordRoot)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func appendStageReceiptEvent(t *testing.T, f humanTurnWorkspaceFixture, event Event) {
	t.Helper()
	if err := recordlock.With(context.Background(), f.identity, func(g *recordlock.Guard) error {
		return appendIntentCaptureForIdentity(context.Background(), f.identity, g, f.projectRoot, f.recordRoot, []Event{event})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestStageQuestionsReaderDynamicPath(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	name := "market-research-questions.md"
	content := []byte("# Questions\n市場調査\n")
	if err := root.WriteFile(name, content, 0o600); err != nil {
		t.Fatal(err)
	}
	got, digest, err := readStageQuestions(root, name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) || digest != fmt.Sprintf("%x", sha256.Sum256(content)) {
		t.Fatalf("read = %q, %q", got, digest)
	}
}

func TestStageQuestionsReaderRejectsUnsafeFiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		setup func(*testing.T, *os.Root)
	}{
		{"absolute", "/questions.md", nil},
		{"traversal", "../questions.md", nil},
		{"cleaned traversal", "stage/../questions.md", nil},
		{"backslash", `stage\questions.md`, nil},
		{"directory", ".", nil},
		{"symlink", "questions.md", func(t *testing.T, r *os.Root) {
			if err := r.Symlink("target.md", "questions.md"); err != nil {
				t.Fatal(err)
			}
		}},
		{"oversize", "questions.md", func(t *testing.T, r *os.Root) {
			if err := r.WriteFile("questions.md", bytes.Repeat([]byte("x"), (8<<20)+1), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"invalid utf8", "questions.md", func(t *testing.T, r *os.Root) {
			if err := r.WriteFile("questions.md", []byte{0xff}, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.WriteFile("target.md", []byte("target"), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.setup != nil {
				tc.setup(t, r)
			}
			if _, _, err := readStageQuestions(r, tc.file); err == nil {
				t.Fatal("accepted unsafe questions")
			}
		})
	}
}

func TestStageQuestionsReaderRejectsIdentityReplacement(t *testing.T) {
	for _, when := range []string{"before open", "after read"} {
		t.Run(when, func(t *testing.T) {
			dir := t.TempDir()
			r, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			name := "questions.md"
			if err := r.WriteFile(name, []byte("same content"), 0o600); err != nil {
				t.Fatal(err)
			}
			replace := func() error {
				if err := os.Rename(filepath.Join(dir, name), filepath.Join(dir, "old.md")); err != nil {
					return err
				}
				return r.WriteFile(name, []byte("same content"), 0o600)
			}
			ops := stageQuestionsReadOps{lstat: r.Lstat, open: openAuditLeaf}
			if when == "before open" {
				ops.open = func(root *os.Root, name string) (*os.File, error) {
					if err := replace(); err != nil {
						return nil, err
					}
					return openAuditLeaf(root, name)
				}
			} else {
				calls := 0
				ops.lstat = func(name string) (fs.FileInfo, error) {
					calls++
					if calls == 2 {
						if err := replace(); err != nil {
							return nil, err
						}
					}
					return r.Lstat(name)
				}
			}
			if _, _, err := readStageQuestionsWithOps(r, name, ops); err == nil {
				t.Fatal("accepted replaced identity")
			}
		})
	}
}

func TestStageSummaryContractRejectsUnsupportedMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*graph.Stage)
	}{
		{"phase", func(s *graph.Stage) { s.Phase = "construction" }},
		{"mode", func(s *graph.Stage) { s.Mode = "subagent" }},
		{"per unit", func(s *graph.Stage) { s.ForEach = "unit" }},
		{"policy", func(s *graph.Stage) { s.SummaryConfirmation = "none" }},
		{"missing", func(s *graph.Stage) { s.Produces = nil }},
		{"duplicate", func(s *graph.Stage) { s.Produces = append(s.Produces, "market-research-questions") }},
		{"optional only", func(s *graph.Stage) { s.Produces = nil; s.OptionalProduces = []string{"market-research-questions"} }},
		{"wrong questions", func(s *graph.Stage) { s.Produces = []string{"other-questions"} }},
		{"invalid slug", func(s *graph.Stage) { s.Slug = "../escape" }},
		{"invalid output", func(s *graph.Stage) { s.Produces = append(s.Produces, "../escape") }},
		{"kind outputs", func(s *graph.Stage) { s.ProducesKinds = map[string][]string{"x": {"y"}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := graph.Stage{Slug: "market-research", Phase: "ideation", Mode: "inline", SummaryConfirmation: "required", Produces: []string{"market-research-questions"}}
			tc.change(&s)
			if _, err := resolveStageSummaryContract(s, graph.Snapshot{}); err == nil {
				t.Fatal("accepted unsupported metadata")
			}
		})
	}
}
