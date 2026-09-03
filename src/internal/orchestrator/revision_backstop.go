package orchestrator

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
)

var errRevisionBackstopAmbiguous = errors.New("orchestrator: revision backstop order is ambiguous")

type revisionBackstopEvent struct {
	event     string
	timestamp time.Time
	shard     string
	position  int
	stage     string
	file      string
	recovered bool
}

// revisionBackstopRequired detects evidence that a human revised a declared
// stage artifact after the gate opened without a recorded rejection. It is a
// pure predicate: the caller must supply the records returned by its own
// identity-bound fresh audit read and must make the unsupported result
// authoritative before appending any approval event.
func revisionBackstopRequired(records []audit.AuditRecord, stage graph.Stage) (bool, error) {
	if stage.Slug == "" || len(records) == 0 {
		return false, nil
	}

	events := make([]revisionBackstopEvent, 0, len(records))
	for _, record := range records {
		if !revisionBackstopRelevantEvent(record, stage) {
			continue
		}
		if record.Timestamp.IsZero() || record.Timestamp.Nanosecond() != 0 || record.Shard == "" || record.Position < 0 {
			return false, fmt.Errorf("revision backstop: invalid audit ordering metadata: %w", ErrInvalidGate)
		}
		events = append(events, revisionBackstopEvent{
			event:     record.Event,
			timestamp: record.Timestamp,
			shard:     record.Shard,
			position:  record.Position,
			stage:     record.Fields["Stage"],
			file:      record.Fields["File"],
			recovered: record.Fields["Recovered"] == "true",
		})
	}
	if len(events) == 0 {
		return false, nil
	}

	sort.SliceStable(events, func(left, right int) bool {
		if !events[left].timestamp.Equal(events[right].timestamp) {
			return events[left].timestamp.Before(events[right].timestamp)
		}
		if events[left].shard != events[right].shard {
			return events[left].shard < events[right].shard
		}
		return events[left].position < events[right].position
	})
	orders, err := revisionBackstopOrders(events)
	if err != nil {
		return false, err
	}
	var (
		decided bool
		result  bool
	)
	for _, ordered := range orders {
		candidate := revisionBackstopEvaluate(ordered, stage)
		if !decided {
			result = candidate
			decided = true
			continue
		}
		if candidate != result {
			return false, fmt.Errorf("revision backstop: equal-timestamp events span audit shards and change the decision: %w", errors.Join(ErrUnsupportedGate, errRevisionBackstopAmbiguous))
		}
	}
	return result, nil
}

func revisionBackstopEvaluate(events []revisionBackstopEvent, stage graph.Stage) bool {

	anchor := -1
	anchorIsGateOpen := false
	for index, event := range events {
		if event.stage != stage.Slug {
			continue
		}
		switch event.event {
		case "STAGE_AWAITING_APPROVAL":
			if !event.recovered {
				anchor = index
				anchorIsGateOpen = true
			}
		case "STAGE_STARTED":
			anchor = index
			anchorIsGateOpen = false
		}
	}
	if anchor < 0 {
		return false
	}

	firstHuman := -1
	wroteBeforeHuman := false
	for index := anchor + 1; index < len(events); index++ {
		event := events[index]
		if event.event == "GATE_REJECTED" && event.stage == stage.Slug {
			return false
		}
		if firstHuman >= 0 {
			continue
		}
		if event.event == "HUMAN_TURN" {
			firstHuman = index
			continue
		}
		if revisionBackstopArtifactWrite(event, stage) {
			wroteBeforeHuman = true
		}
	}
	if firstHuman < 0 || (!anchorIsGateOpen && !wroteBeforeHuman) {
		return false
	}
	for index := firstHuman + 1; index < len(events); index++ {
		if revisionBackstopArtifactWrite(events[index], stage) {
			return true
		}
	}
	return false
}

func revisionBackstopRelevantEvent(record audit.AuditRecord, stage graph.Stage) bool {
	switch record.Event {
	case "HUMAN_TURN":
		return true
	case "STAGE_AWAITING_APPROVAL", "STAGE_STARTED", "GATE_REJECTED":
		return record.Fields["Stage"] == stage.Slug
	case "ARTIFACT_CREATED", "ARTIFACT_UPDATED":
		return revisionBackstopArtifactWrite(revisionBackstopEvent{
			event: record.Event,
			file:  record.Fields["File"],
		}, stage)
	default:
		return false
	}
}

const revisionBackstopOrderLimit = 256

func revisionBackstopOrders(events []revisionBackstopEvent) ([][]revisionBackstopEvent, error) {
	groups := make([][]revisionBackstopEvent, 0)
	for left := 0; left < len(events); {
		right := left + 1
		for right < len(events) && events[right].timestamp.Equal(events[left].timestamp) {
			right++
		}
		group := append([]revisionBackstopEvent(nil), events[left:right]...)
		groups = append(groups, group)
		left = right
	}

	orders := [][]revisionBackstopEvent{{}}
	for _, group := range groups {
		interleavings, err := revisionBackstopInterleavings(group)
		if err != nil {
			return nil, err
		}
		combined := make([][]revisionBackstopEvent, 0, len(orders)*len(interleavings))
		for _, prefix := range orders {
			for _, suffix := range interleavings {
				if len(combined) >= revisionBackstopOrderLimit {
					return nil, fmt.Errorf("revision backstop: too many equal-time audit orders to establish authority: %w", errors.Join(ErrUnsupportedGate, errRevisionBackstopAmbiguous))
				}
				order := make([]revisionBackstopEvent, 0, len(prefix)+len(suffix))
				order = append(order, prefix...)
				order = append(order, suffix...)
				combined = append(combined, order)
			}
		}
		orders = combined
	}
	return orders, nil
}

func revisionBackstopInterleavings(group []revisionBackstopEvent) ([][]revisionBackstopEvent, error) {
	byShard := make(map[string][]revisionBackstopEvent)
	for _, event := range group {
		byShard[event.shard] = append(byShard[event.shard], event)
	}
	if len(byShard) <= 1 {
		return [][]revisionBackstopEvent{group}, nil
	}
	shards := make([]string, 0, len(byShard))
	for shard := range byShard {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		sort.SliceStable(byShard[shard], func(left, right int) bool {
			return byShard[shard][left].position < byShard[shard][right].position
		})
	}

	orders := make([][]revisionBackstopEvent, 0)
	positions := make(map[string]int, len(shards))
	var build func([]revisionBackstopEvent)
	build = func(prefix []revisionBackstopEvent) {
		if len(orders) >= revisionBackstopOrderLimit {
			return
		}
		if len(prefix) == len(group) {
			orders = append(orders, append([]revisionBackstopEvent(nil), prefix...))
			return
		}
		for _, shard := range shards {
			index := positions[shard]
			if index >= len(byShard[shard]) {
				continue
			}
			positions[shard] = index + 1
			build(append(prefix, byShard[shard][index]))
			positions[shard] = index
		}
	}
	build(nil)
	if len(orders) == 0 || (len(orders) >= revisionBackstopOrderLimit && len(group) > 1) {
		return nil, fmt.Errorf("revision backstop: too many equal-time audit orders to establish authority: %w", errors.Join(ErrUnsupportedGate, errRevisionBackstopAmbiguous))
	}
	return orders, nil
}

func revisionBackstopArtifactWrite(event revisionBackstopEvent, stage graph.Stage) bool {
	if event.event != "ARTIFACT_CREATED" && event.event != "ARTIFACT_UPDATED" {
		return false
	}
	if event.file == "" {
		return false
	}
	file := strings.ReplaceAll(event.file, "\\", "/")
	for _, declared := range stage.Produces {
		filename := artifact.Filename(declared)
		if strings.HasSuffix(file, "/"+stage.Slug+"/"+filename) {
			return true
		}
	}
	return false
}
