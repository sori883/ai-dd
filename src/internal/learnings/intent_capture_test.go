package learnings

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestIntentCaptureLearningsQuestion(t *testing.T) {
	question, err := NewQuestion(QuestionInput{
		Stage: "intent-capture", Identity: "project\x00team\x00build", Generation: 7,
		SensorIDs: []string{"claim-sources", "required-sections", "upstream-coverage"},
	})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	if question.Prompt == "" || question.ID == "" {
		t.Fatal("learnings question has no stable prompt/id")
	}
	if err := ValidateAnswer(question, Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true,
	}); err != nil {
		t.Fatalf("ValidateAnswer(fresh): %v", err)
	}
}

func TestPersistIntentCaptureLearning(t *testing.T) {
	question, err := NewQuestion(QuestionInput{Stage: "intent-capture", Identity: "identity", Generation: 1, SensorIDs: []string{"claim-sources"}})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	answer := Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true, Rule: "Keep source links", ProposedSensor: "claim-sources"}
	events, err := Persist(question, answer)
	if err != nil {
		t.Fatalf("Persist(): %v", err)
	}
	if len(events) != 2 || events[0].Type != EventRuleLearned || events[1].Type != EventSensorProposed {
		t.Fatalf("events = %#v, want RULE_LEARNED then SENSOR_PROPOSED", events)
	}
	if _, err := Persist(question, Answer{Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true}); err != nil {
		t.Fatalf("Persist(no selection): %v", err)
	}
}

func TestPersistSensorSelectionUsesFixedOriginDestination(t *testing.T) {
	question, err := NewQuestion(QuestionInput{
		Stage: "intent-capture", Identity: "identity", Generation: 1,
		Surface: Surface{Candidates: []Candidate{{ID: "c1", Text: "propose a check"}}},
	})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	events, err := Persist(question, Answer{
		Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true,
		Selections: []Selection{{CandidateID: "c1", Type: SelectionTypeSensor, OriginStage: "intent-capture", Source: "orchestrator", ManifestFields: map[string]string{
			"id": "deterministic-check", "kind": "deterministic", "command": "check", "default_severity": "advisory", "description": "a check", "matches": "**/*.md",
		}}},
	})
	if err != nil {
		t.Fatalf("Persist(sensor): %v", err)
	}
	if len(events) != 1 || events[0].Type != EventSensorProposed {
		t.Fatalf("events = %#v, want one SENSOR_PROPOSED", events)
	}
	var destinations []string
	if err := json.Unmarshal([]byte(events[0].Fields["Destinations"]), &destinations); err != nil {
		t.Fatalf("Destinations = %q: %v", events[0].Fields["Destinations"], err)
	}
	if len(destinations) != 1 || destinations[0] != "intent-capture" {
		t.Fatalf("Destinations = %#v, want [intent-capture]", destinations)
	}
}

func TestLearningsRejectsStaleIdentityOrConflict(t *testing.T) {
	question, err := NewQuestion(QuestionInput{Stage: "intent-capture", Identity: "identity", Generation: 3})
	if err != nil {
		t.Fatalf("NewQuestion(): %v", err)
	}
	for name, answer := range map[string]Answer{
		"identity":   {Stage: question.Stage, Identity: "other", Generation: question.Generation, FreshTurn: true},
		"generation": {Stage: question.Stage, Identity: question.Identity, Generation: question.Generation + 1, FreshTurn: true},
		"turn":       {Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: false},
		"conflict":   {Stage: question.Stage, Identity: question.Identity, Generation: question.Generation, FreshTurn: true, Conflict: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateAnswer(question, answer); !errors.Is(err, ErrStale) && !errors.Is(err, ErrConflict) {
				t.Fatalf("ValidateAnswer() error = %v, want stale/conflict", err)
			}
		})
	}
}
