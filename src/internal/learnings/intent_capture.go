// Package learnings models the intent-capture learnings question and its
// purpose-specific, non-generic audit events.
package learnings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrInvalid  = errors.New("learnings: invalid question or answer")
	ErrStale    = errors.New("learnings: stale identity or generation")
	ErrConflict = errors.New("learnings: conflicting answer")
)

const (
	EventRuleLearned    = "RULE_LEARNED"
	EventSensorProposed = "SENSOR_PROPOSED"
)

type QuestionInput struct {
	Stage      string
	Identity   string
	Generation uint64
	SensorIDs  []string
	Surface    Surface
}

type Question struct {
	ID                 string
	Stage              string
	Identity           string
	Generation         uint64
	SensorIDs          []string
	Prompt             string
	Space              string
	Intent             string
	Candidates         []Candidate
	Parked             []string
	SurfaceFingerprint string
}

type Answer struct {
	Stage          string
	Identity       string
	Generation     uint64
	FreshTurn      bool
	Conflict       bool
	Rule           string
	ProposedSensor string
	Selections     []Selection
}

type Event struct {
	Type   string
	Fields map[string]string
}

func NewQuestion(input QuestionInput) (Question, error) {
	if strings.TrimSpace(input.Stage) == "" || strings.TrimSpace(input.Identity) == "" {
		return Question{}, fmt.Errorf("stage and identity are required: %w", ErrInvalid)
	}
	sensors := slices.Clone(input.SensorIDs)
	for _, sensor := range sensors {
		if strings.TrimSpace(sensor) == "" {
			return Question{}, fmt.Errorf("sensor id is empty: %w", ErrInvalid)
		}
	}
	question := Question{
		Stage:              input.Stage,
		Identity:           input.Identity,
		Generation:         input.Generation,
		SensorIDs:          sensors,
		Prompt:             "Which sensor or workflow rule should be retained as a learning? Select none if no learning is needed. Anything to add for next time? Choose Nothing to add or Add a note.",
		Space:              input.Surface.Space,
		Intent:             input.Surface.Intent,
		Candidates:         slices.Clone(input.Surface.Candidates),
		Parked:             slices.Clone(input.Surface.Parked),
		SurfaceFingerprint: input.Surface.Fingerprint,
	}
	hash := sha256.New()
	fmt.Fprintf(hash, "%d:%s\x00%d\x00", len(question.Stage), question.Stage, question.Generation)
	hash.Write([]byte(question.Identity))
	for _, sensor := range sensors {
		fmt.Fprintf(hash, "\x00%d:%s", len(sensor), sensor)
	}
	if question.SurfaceFingerprint != "" {
		fmt.Fprintf(hash, "\x00surface:%s", question.SurfaceFingerprint)
	}
	question.ID = hex.EncodeToString(hash.Sum(nil))
	return question, nil
}

func ValidateAnswer(question Question, answer Answer) error {
	if question.ID == "" || question.Stage == "" || question.Identity == "" || answer.Stage == "" || answer.Identity == "" {
		return fmt.Errorf("question/answer identity is incomplete: %w", ErrInvalid)
	}
	if answer.Stage != question.Stage || answer.Identity != question.Identity || answer.Generation != question.Generation || !answer.FreshTurn {
		return fmt.Errorf("learning answer does not match the fresh question turn: %w", ErrStale)
	}
	if answer.Conflict {
		return ErrConflict
	}
	if answer.ProposedSensor != "" && !slices.Contains(question.SensorIDs, answer.ProposedSensor) {
		return fmt.Errorf("proposed sensor %q was not fired for this question: %w", answer.ProposedSensor, ErrInvalid)
	}
	return nil
}

func Persist(question Question, answer Answer) ([]Event, error) {
	if err := ValidateAnswer(question, answer); err != nil {
		return nil, err
	}
	if len(answer.Selections) != 0 {
		if err := ValidateSelections(answer.Selections); err != nil {
			return nil, err
		}
		known := make(map[string]Candidate, len(question.Candidates))
		for _, candidate := range question.Candidates {
			known[candidate.ID] = candidate
		}
		events := make([]Event, 0, len(answer.Selections))
		for _, selection := range answer.Selections {
			source := selection.Source
			if source == "" {
				source = "orchestrator"
			}
			candidate, ok := known[selection.CandidateID]
			if !ok && selection.Source == "user_addition" {
				candidate = Candidate{ID: selection.CandidateID, Heading: selection.Heading, Text: selection.Text, Summary: selection.Text, Scope: selection.Scope, Source: selection.Source}
				ok = true
			}
			if !ok {
				return nil, fmt.Errorf("learning selection candidate %q is not on the current surface: %w", selection.CandidateID, ErrStale)
			}
			if selection.Type == SelectionTypeLearning && selection.Text != candidate.Text {
				return nil, fmt.Errorf("learning selection candidate %q changed: %w", selection.CandidateID, ErrStale)
			}
			if selection.Type == SelectionTypeLearning {
				hash := sha256.Sum256([]byte(selection.Text))
				events = append(events, Event{Type: EventRuleLearned, Fields: map[string]string{
					"Stage": question.Stage, "Candidate-ID": selection.CandidateID,
					"Content-Hash": hex.EncodeToString(hash[:]), "Destination": selection.Scope,
					"Heading": selection.Heading, "Source": source,
				}})
				continue
			}
			destinations, _ := json.Marshal([]string{"intent-capture"})
			events = append(events, Event{Type: EventSensorProposed, Fields: map[string]string{
				"Stage": question.Stage, "Candidate-ID": selection.CandidateID,
				"Sensor ID": selection.ManifestFields["id"], "Manifest path": ".codex/sensors/aidlc-" + selection.ManifestFields["id"] + ".md",
				"Matches": selection.ManifestFields["matches"], "Destinations": string(destinations), "Source": source,
			}})
		}
		return events, nil
	}
	events := make([]Event, 0, 2)
	if rule := strings.TrimSpace(answer.Rule); rule != "" {
		events = append(events, Event{Type: EventRuleLearned, Fields: map[string]string{
			"Stage": stageField(question), "Question": question.ID, "Rule": rule,
		}})
	}
	if sensor := strings.TrimSpace(answer.ProposedSensor); sensor != "" {
		events = append(events, Event{Type: EventSensorProposed, Fields: map[string]string{
			"Stage": stageField(question), "Question": question.ID, "Sensor": sensor,
		}})
	}
	return events, nil
}

func stageField(question Question) string { return question.Stage }
