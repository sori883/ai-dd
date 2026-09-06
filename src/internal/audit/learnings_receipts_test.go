package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/learnings"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/sensor"
)

func TestLearningSelectionPersistsDurableProjectPractice(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Interpretations\n- Keep source links visible\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if len(question.Candidates) != 1 {
		t.Fatalf("question candidates = %#v, want one", question.Candidates)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	selection := learnings.Selection{CandidateID: question.Candidates[0].ID, Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: question.Candidates[0].Text, Source: "orchestrator"}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection}}); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(selected): %v", err)
	}
	practicePath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md")
	practice, err := os.ReadFile(practicePath)
	if err != nil {
		t.Fatalf("ReadFile(project memory): %v", err)
	}
	if !strings.Contains(string(practice), selection.Text) || !strings.Contains(string(practice), "<!-- cid:build:intent-capture:") {
		t.Fatalf("project memory = %q, want selected practice marker", practice)
	}
	if !strings.HasPrefix(string(practice), "# Project-Level Rules\n\n") {
		t.Fatalf("project memory = %q, want fixed project-level template", practice)
	}
	records := mustReadAuditRecords(t, fixture)
	answered := records[len(records)-2]
	if got := answered.Fields["Learning Selection"]; got != "learning" {
		t.Fatalf("Learning Selection = %q, want learning", got)
	}
	if got := answered.Event; got != "QUESTION_ANSWERED" {
		t.Fatalf("last event = %q, want QUESTION_ANSWERED", got)
	}
	if got := answered.Fields["Learning Question"]; got == "" {
		t.Fatal("QUESTION_ANSWERED missing Learning Question")
	}
}

func TestLearningSelectionAllowsMultipleContentHashesInOneStage(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Interpretations\n- Keep source links visible\n- Preserve exact output\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if len(question.Candidates) != 2 {
		t.Fatalf("question candidates = %#v, want two", question.Candidates)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	selections := make([]learnings.Selection, 0, len(question.Candidates))
	for _, candidate := range question.Candidates {
		selections = append(selections, learnings.Selection{CandidateID: candidate.ID, Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: candidate.Text, Source: "orchestrator"})
	}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: selections}); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(multiple): %v", err)
	}
	practice, err := os.ReadFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md"))
	if err != nil {
		t.Fatalf("ReadFile(project memory): %v", err)
	}
	for _, candidate := range question.Candidates {
		if strings.Count(string(practice), candidate.Text) != 1 {
			t.Fatalf("project memory = %q, want one durable line for %q", practice, candidate.Text)
		}
	}
	records := mustReadAuditRecords(t, fixture)
	learned := 0
	for _, record := range records {
		if record.Event == learnings.EventRuleLearned {
			learned++
		}
	}
	if learned != len(question.Candidates) {
		t.Fatalf("RULE_LEARNED count = %d, want %d for distinct content hashes", learned, len(question.Candidates))
	}
}

func TestLearningUserAdditionPersistsFreeTextCandidate(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, question)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	selection := learnings.Selection{CandidateID: "user-addition-1", Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: "Remember the user's explicit constraint.", Source: "user_addition"}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection},
	})
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearning(user_addition): %v", err)
	}
	practice, err := os.ReadFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md"))
	if err != nil {
		t.Fatalf("ReadFile(project memory): %v", err)
	}
	if !strings.Contains(string(practice), selection.Text) {
		t.Fatalf("project memory = %q, want explicit user addition", practice)
	}
}

func TestLearningSelectionRejectsChangedSurfaceBeforeWrite(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Tradeoffs\n- Prefer deterministic output\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Tradeoffs\n- Changed after question\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed memory): %v", err)
	}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{{CandidateID: "c1", Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: "Prefer deterministic output", Source: "orchestrator"}}})
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("RecordIntentCaptureLearning(changed surface) error = %v, want stale", err)
	}
}

func TestLearningPersistenceRetriesMissingOwnedRows(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Interpretations\n- Keep source links visible\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return appendIntentCaptureForIdentity(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, []Event{{
			Event: "QUESTION_ANSWERED",
			Fields: map[string]string{
				"Stage":               question.Stage,
				"Learning Question":   question.ID,
				"Learning Generation": fmt.Sprint(question.Generation),
				"Learning Selection":  "learning",
				"Details":             "learning",
			},
		}})
	}); err != nil {
		t.Fatalf("append partial learning answer: %v", err)
	}
	selection := learnings.Selection{CandidateID: question.Candidates[0].ID, Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: question.Candidates[0].Text, Source: "orchestrator"}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection}}); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(retry): %v", err)
	}
	records := mustReadAuditRecords(t, fixture)
	answered, learned := 0, 0
	for _, record := range records {
		switch record.Event {
		case "QUESTION_ANSWERED":
			answered++
		case "RULE_LEARNED":
			learned++
		}
	}
	if answered != 1 || learned != 1 {
		t.Fatalf("learning receipts = answered %d, learned %d; want one existing answer and one missing rule receipt: %#v", answered, learned, records)
	}
	practicePath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md")
	practice, err := os.ReadFile(practicePath)
	if err != nil {
		t.Fatalf("ReadFile(project memory): %v", err)
	}
	if !strings.Contains(string(practice), selection.Text) {
		t.Fatalf("project memory = %q, want retried durable selection", practice)
	}
}

func TestLearningSelectionRejectsForgedQuestionWithoutSurfaceBinding(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Interpretations\n- Keep source links visible\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if len(question.Candidates) != 1 {
		t.Fatalf("question candidates = %#v, want one", question.Candidates)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	forged := question
	forged.SurfaceFingerprint = ""
	forged.Candidates = append([]learnings.Candidate(nil), question.Candidates...)
	forged.Candidates[0].Text = "caller-controlled practice"
	forged.Candidates[0].Summary = forged.Candidates[0].Text
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, forged, learnings.Answer{
		Stage: forged.Stage, Identity: forged.Identity, Generation: forged.Generation,
		Selections: []learnings.Selection{{CandidateID: forged.Candidates[0].ID, Type: learnings.SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: forged.Candidates[0].Text, Source: "orchestrator"}},
	})
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("RecordIntentCaptureLearning(forged question) error = %v, want stale", err)
	}
	practicePath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md")
	if _, readErr := os.Stat(practicePath); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("forged learning practice stat error = %v, want no durable write", readErr)
	}
}

func TestSensorLearningSelectionPersistsManifestAndBinding(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Tradeoffs\n- Add a deterministic check\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	stagePath := filepath.Join(fixture.project, ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(stage): %v", err)
	}
	if err := os.WriteFile(stagePath, []byte("---\nslug: intent-capture\nphase: ideation\nsensors:\n  - claim-sources\nscopes:\n  - feature\n---\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(stage): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	selection := learnings.Selection{CandidateID: question.Candidates[0].ID, Type: learnings.SelectionTypeSensor, OriginStage: "intent-capture", Source: "orchestrator", ManifestFields: map[string]string{
		"id": "deterministic-check", "kind": "deterministic", "command": "go run check", "default_severity": "advisory", "description": "checks deterministic output", "matches": "**/intents/**", "category": "document-shape",
	}}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection}}); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(sensor): %v", err)
	}
	manifestPath := filepath.Join(fixture.project, ".codex", "sensors", "aidlc-deterministic-check.md")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("Stat(sensor manifest): %v", err)
	}
	stageContent, err := os.ReadFile(stagePath)
	if err != nil {
		t.Fatalf("ReadFile(stage): %v", err)
	}
	if !strings.Contains(string(stageContent), "  - deterministic-check") {
		t.Fatalf("stage content = %q, want sensor binding", stageContent)
	}
	manifestContent, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(sensor manifest): %v", err)
	}
	manifestText := string(manifestContent)
	if strings.Contains(manifestText, "fire_on:") || !strings.Contains(manifestText, "description: \"checks deterministic output\"") || !strings.Contains(manifestText, "\nchecks deterministic output\n") || !strings.Contains(manifestText, "Scaffolded by the §13 learning gate (project-tier).") {
		t.Fatalf("sensor manifest = %q, want fixed project-tier scaffold with description body and without fire_on", manifestText)
	}
}

func TestSensorLearningSelectionRejectsNonCanonicalManifestFields(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Tradeoffs\n- Add a deterministic check\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	stagePath := filepath.Join(fixture.project, ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(stage): %v", err)
	}
	if err := os.WriteFile(stagePath, []byte("---\nslug: intent-capture\nphase: ideation\nsensors:\n  - claim-sources\n---\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(stage): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	base := map[string]string{
		"id": "deterministic-check", "kind": "deterministic", "command": "go run check",
		"default_severity": "advisory", "description": "checks deterministic output", "matches": "**/intents/**",
	}
	for name, mutate := range map[string]func(map[string]string){
		"kind":                func(fields map[string]string) { fields["kind"] = "check" },
		"severity":            func(fields map[string]string) { fields["default_severity"] = "warning" },
		"id":                  func(fields map[string]string) { fields["id"] = "foo # comment" },
		"leading punctuation": func(fields map[string]string) { fields["id"] = "-deterministic-check" },
	} {
		t.Run(name, func(t *testing.T) {
			fields := make(map[string]string, len(base))
			for key, value := range base {
				fields[key] = value
			}
			mutate(fields)
			selection := learnings.Selection{CandidateID: question.Candidates[0].ID, Type: learnings.SelectionTypeSensor, OriginStage: "intent-capture", Source: "orchestrator", ManifestFields: fields}
			if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection}}); !errors.Is(err, learnings.ErrInvalid) {
				t.Fatalf("RecordIntentCaptureLearning(%s) error = %v, want ErrInvalid", name, err)
			}
		})
	}
}

func TestSensorLearningSelectionRejectsInvalidStageBeforeDurableWrite(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	memoryPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "memory.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(memory): %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("## Tradeoffs\n- Add a deterministic check\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memory): %v", err)
	}
	stagePath := filepath.Join(fixture.project, ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o700); err != nil {
		t.Fatalf("MkdirAll(stage): %v", err)
	}
	if err := os.WriteFile(stagePath, []byte("---\nslug: intent-capture\nslug: intent-capture\nphase: ideation\nsensors:\n  - claim-sources\n---\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(stage): %v", err)
	}
	question, err := RecordIntentCaptureLearningDecision(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, "intent-capture")
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearningDecision(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	selection := learnings.Selection{CandidateID: question.Candidates[0].ID, Type: learnings.SelectionTypeSensor, OriginStage: "intent-capture", Source: "orchestrator", ManifestFields: map[string]string{
		"id": "deterministic-check", "kind": "deterministic", "command": "go run check", "default_severity": "advisory", "description": "checks deterministic output", "matches": "**/intents/**", "category": "document-shape",
	}}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, Selections: []learnings.Selection{selection}})
	if !errors.Is(err, learnings.ErrInvalid) {
		t.Fatalf("RecordIntentCaptureLearning(invalid stage) error = %v, want invalid", err)
	}
	manifestPath := filepath.Join(fixture.project, ".codex", "sensors", "aidlc-deterministic-check.md")
	if _, statErr := os.Stat(manifestPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("sensor manifest stat error = %v, want no durable write", statErr)
	}
	for _, record := range mustReadAuditRecords(t, fixture) {
		if record.Event == "QUESTION_ANSWERED" || record.Event == learnings.EventSensorProposed {
			t.Fatalf("record %s = %#v, want no learning authority after invalid stage", record.Event, record.Fields)
		}
	}
}

func TestIntentCaptureNoLearningStillRecordsAnsweredReceipt(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := learningsSensorInvocation(t, fixture)
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, question)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	answer := learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: false}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, answer); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(none): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 5 || records[4].Event != "QUESTION_ANSWERED" {
		t.Fatalf("records = %#v, want sensor events, learning decision, HUMAN_TURN, then learning QUESTION_ANSWERED", records)
	}
	if got := records[4].Fields["Learning Selection"]; got != "none" {
		t.Errorf("Learning Selection = %q, want none", got)
	}
}

func TestLearningPersistRequiresMatchingDecisionAndOneFreshHumanTurn(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := learningsSensorInvocation(t, fixture)
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true,
	})
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("RecordIntentCaptureLearning() error = %v, want stale without matching learning decision", err)
	}
}

func TestLearningPersistsWithoutSensorReceipts(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, question)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: false,
	})
	if err != nil {
		t.Fatalf("RecordIntentCaptureLearning() without sensor receipts = %v, want nil", err)
	}
}

func TestLearningRevisionUsesFreshPair(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := learningsSensorInvocation(t, fixture)
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	oldQuestion, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(old): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, oldQuestion)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("old RecordHumanTurn(): %v", err)
	}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, oldQuestion, learnings.Answer{
		Stage: oldQuestion.Stage, Identity: oldQuestion.Identity, Generation: oldQuestion.Generation,
	}); err != nil {
		t.Fatalf("old RecordIntentCaptureLearning(): %v", err)
	}
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []Event{
			{Event: "GATE_REJECTED", Fields: map[string]string{"Stage": "intent-capture"}},
			{Event: "STAGE_REVISING", Fields: map[string]string{"Stage": "intent-capture"}},
		})
	}); err != nil {
		t.Fatalf("append revision anchor: %v", err)
	}
	newQuestion, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 2})
	if err != nil {
		t.Fatalf("NewQuestion(new): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, newQuestion)
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, newQuestion, learnings.Answer{
		Stage: newQuestion.Stage, Identity: newQuestion.Identity, Generation: newQuestion.Generation,
	})
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("new RecordIntentCaptureLearning() error = %v, want stale without a fresh post-revision HUMAN_TURN", err)
	}
}

func appendLearningDecisionForTest(t *testing.T, fixture humanTurnWorkspaceFixture, question learnings.Question) {
	t.Helper()
	err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return appendIntentCaptureForIdentity(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot, []Event{{
			Event: "DECISION_RECORDED",
			Fields: map[string]string{
				"Stage": question.Stage, "Decision": question.ID, "Learning Question": question.ID,
				"Learning Generation": fmt.Sprint(question.Generation), "Options": "none",
			},
		}})
	})
	if err != nil {
		t.Fatalf("append learning decision: %v", err)
	}
}

func TestIntentCaptureLearningsRequiresSensorAttempt(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, learnings.Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: false,
	})
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("RecordIntentCaptureLearning() error = %v, want learnings.ErrStale without sensor attempt", err)
	}
}

func TestIntentCaptureLearningsRequiresPostSensorFreshTurn(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	invocation := learningsSensorInvocation(t, fixture)
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1, SensorIDs: []string{"claim-sources"}})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	answer := learnings.Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true, Rule: "must not persist"}
	err = RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, answer)
	if !errors.Is(err, learnings.ErrStale) {
		t.Fatalf("RecordIntentCaptureLearning(before post-sensor turn) error = %v, want learnings.ErrStale", err)
	}
}

func learningsSensorInvocation(t *testing.T, fixture humanTurnWorkspaceFixture) sensor.Invocation {
	t.Helper()
	return sensor.RunAll(context.Background(), sensor.Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("# Intent\n")}, nil)[0]
}

func TestPersistIntentCaptureLearning(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := learningsSensorInvocation(t, fixture)
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	question, err := learnings.NewQuestion(learnings.QuestionInput{
		Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1,
		SensorIDs: []string{"claim-sources"},
	})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	appendLearningDecisionForTest(t, fixture, question)
	if err := RecordHumanTurn(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn(): %v", err)
	}
	answer := learnings.Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation,
		FreshTurn: false, Rule: "Keep source links", ProposedSensor: "claim-sources",
	}
	if err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, answer); err != nil {
		t.Fatalf("RecordIntentCaptureLearning(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 7 || records[4].Event != "QUESTION_ANSWERED" || records[5].Event != "RULE_LEARNED" || records[6].Event != "SENSOR_PROPOSED" {
		t.Fatalf("records = %#v, want sensor events, learning decision, HUMAN_TURN, learning answer, RULE_LEARNED, SENSOR_PROPOSED", records)
	}
}

func TestLearningsRejectsStaleIdentityOrConflict(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	question, err := learnings.NewQuestion(learnings.QuestionInput{Stage: "intent-capture", Identity: fixture.identity.String(), Generation: 1})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	for name, answer := range map[string]learnings.Answer{
		"identity": {Stage: question.Stage, Identity: "other", Generation: question.Generation, FreshTurn: true},
		"conflict": {Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true, Conflict: true},
	} {
		t.Run(name, func(t *testing.T) {
			err := RecordIntentCaptureLearning(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, question, answer)
			if !errors.Is(err, learnings.ErrStale) && !errors.Is(err, learnings.ErrConflict) {
				t.Fatalf("RecordIntentCaptureLearning() error = %v, want stale/conflict", err)
			}
		})
	}
}
