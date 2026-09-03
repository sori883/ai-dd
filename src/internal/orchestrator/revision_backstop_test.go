package orchestrator

import (
	"errors"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
)

func TestRevisionBackstopRequiredAfterOrganicGateHumanAndArtifactWrite(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	records := []audit.AuditRecord{
		{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
		{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{
			"Stage": "intent-capture",
			"File":  "/record/ideation/intent-capture/intent-statement.md",
		}},
	}

	got, err := revisionBackstopRequired(records, graph.Stage{
		Slug:     "intent-capture",
		Phase:    "ideation",
		Produces: []string{"intent-statement"},
	})
	if err != nil {
		t.Fatalf("revisionBackstopRequired() error = %v", err)
	}
	if !got {
		t.Fatal("revisionBackstopRequired() = false, want true")
	}
}

func TestRevisionBackstopRejectsCrossShardEqualTimestampOrder(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	got, err := revisionBackstopRequired([]audit.AuditRecord{
		{Event: "STAGE_AWAITING_APPROVAL", Timestamp: when, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "HUMAN_TURN", Timestamp: when, Shard: "audit/b.md", Position: 0},
		{Event: "ARTIFACT_UPDATED", Timestamp: when, Shard: "audit/a.md", Position: 1, Fields: map[string]string{
			"File": "/record/ideation/intent-capture/intent-statement.md",
		}},
	}, graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}})
	if !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("revisionBackstopRequired() error = %v, want ErrUnsupportedGate", err)
	}
	if got {
		t.Fatal("revisionBackstopRequired() = true with ambiguous order, want false")
	}
}

func TestRevisionBackstopStageStartRequiresEvidenceBeforeFirstHuman(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}}
	tests := []struct {
		name    string
		records []audit.AuditRecord
		want    bool
	}{
		{
			name: "write only after human is ordinary coaching",
			records: []audit.AuditRecord{
				{Event: "STAGE_STARTED", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
				{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
			},
			want: false,
		},
		{
			name: "write before and after human is revision",
			records: []audit.AuditRecord{
				{Event: "STAGE_STARTED", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "ARTIFACT_CREATED", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
				{Event: "HUMAN_TURN", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2},
				{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(3 * time.Second), Shard: "audit/a.md", Position: 3, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := revisionBackstopRequired(tt.records, stage)
			if err != nil {
				t.Fatalf("revisionBackstopRequired() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("revisionBackstopRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRevisionBackstopIgnoresRecoveredAnchorAndRecordedReject(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}}
	tests := []struct {
		name    string
		records []audit.AuditRecord
		want    bool
	}{
		{
			name: "recovered gate row does not replace organic anchor",
			records: []audit.AuditRecord{
				{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1, Fields: map[string]string{"Stage": stage.Slug, "Recovered": "true"}},
				{Event: "HUMAN_TURN", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2},
				{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(3 * time.Second), Shard: "audit/a.md", Position: 3, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
			},
			want: true,
		},
		{
			name: "reject after anchor disables backstop",
			records: []audit.AuditRecord{
				{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
				{Event: "GATE_REJECTED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(3 * time.Second), Shard: "audit/a.md", Position: 3, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
			},
			want: false,
		},
		{
			name: "other stage reject does not disable target",
			records: []audit.AuditRecord{
				{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
				{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
				{Event: "GATE_REJECTED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{"Stage": "other-stage"}},
				{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(3 * time.Second), Shard: "audit/a.md", Position: 3, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := revisionBackstopRequired(tt.records, stage)
			if err != nil {
				t.Fatalf("revisionBackstopRequired() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("revisionBackstopRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRevisionBackstopMatchesDeclaredFilenameAndWindowsSeparators(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	got, err := revisionBackstopRequired([]audit.AuditRecord{
		{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": "intent-capture"}},
		{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
		{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{
			"File": `C:\record\ideation\intent-capture\traceability.json`,
		}},
	}, graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"traceability"}})
	if err != nil {
		t.Fatalf("revisionBackstopRequired() error = %v", err)
	}
	if !got {
		t.Fatal("revisionBackstopRequired() = false, want true")
	}
}

func TestRevisionBackstopSameShardOrderAndCrossShardIndependentWrites(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}}
	records := []audit.AuditRecord{
		{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
		{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
		{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(2 * time.Second), Shard: "audit/b.md", Position: 0, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
	}
	got, err := revisionBackstopRequired(records, stage)
	if err != nil {
		t.Fatalf("revisionBackstopRequired() error = %v, want no ambiguity for independent writes", err)
	}
	if !got {
		t.Fatal("revisionBackstopRequired() = false, want true")
	}
}

func TestRevisionBackstopUsesLatestStageStartAsRestartAnchor(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	stage := graph.Stage{Slug: "intent-capture", Phase: "ideation", Produces: []string{"intent-statement"}}
	records := []audit.AuditRecord{
		{Event: "STAGE_AWAITING_APPROVAL", Timestamp: base, Shard: "audit/a.md", Position: 0, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "audit/a.md", Position: 1},
		{Event: "STAGE_STARTED", Timestamp: base.Add(2 * time.Second), Shard: "audit/a.md", Position: 2, Fields: map[string]string{"Stage": stage.Slug}},
		{Event: "HUMAN_TURN", Timestamp: base.Add(3 * time.Second), Shard: "audit/a.md", Position: 3},
		{Event: "ARTIFACT_UPDATED", Timestamp: base.Add(4 * time.Second), Shard: "audit/a.md", Position: 4, Fields: map[string]string{"File": "/record/ideation/intent-capture/intent-statement.md"}},
	}
	got, err := revisionBackstopRequired(records, stage)
	if err != nil {
		t.Fatalf("revisionBackstopRequired() error = %v", err)
	}
	if got {
		t.Fatal("revisionBackstopRequired() = true, want false after restart without pre-human output")
	}
}

func TestRevisionBackstopRejectsOrderEnumerationBeyondLimit(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	shards := []string{"audit/a.md", "audit/b.md", "audit/c.md", "audit/d.md"}
	records := make([]audit.AuditRecord, 0, len(shards)*4)
	for _, shard := range shards {
		for position := 0; position < 4; position++ {
			records = append(records, audit.AuditRecord{
				Event: "HUMAN_TURN", Timestamp: when, Shard: shard, Position: position,
			})
		}
	}

	got, err := revisionBackstopRequired(records, graph.Stage{Slug: "intent-capture", Phase: "ideation"})
	if !errors.Is(err, ErrUnsupportedGate) {
		t.Fatalf("revisionBackstopRequired() error = %v, want ErrUnsupportedGate", err)
	}
	if got {
		t.Fatal("revisionBackstopRequired() = true with excessive order set, want false")
	}
}
