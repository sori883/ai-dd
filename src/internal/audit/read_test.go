package audit

import (
	"errors"
	"io/fs"
	"testing"
	"time"
)

func TestParseAuditShardRetainsCanonicalEventAuthorityAndPosition(t *testing.T) {
	t.Parallel()

	content := "# AI-DLC Audit Log\n" +
		"\n## Human Turn\n" +
		"**Timestamp**: 2026-09-04T10:00:00Z\n" +
		"**Event**: HUMAN_TURN\n" +
		"**Prompt**: inspect\n\n---\n" +
		"\n## Gate Approved\n" +
		"**Timestamp**: 2026-09-04T10:00:01Z\n" +
		"**Event**: GATE_APPROVED\n" +
		"**Stage**: feasibility\n\n---\n"

	rows, err := parseAuditShard("audit/first.md", []byte(content))
	if err != nil {
		t.Fatalf("parseAuditShard() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("parseAuditShard() rows = %d, want 2", len(rows))
	}
	wantTime := time.Date(2026, time.September, 4, 10, 0, 0, 0, time.UTC)
	if rows[0].Event != "HUMAN_TURN" || !rows[0].Timestamp.Equal(wantTime) ||
		rows[0].Shard != "audit/first.md" || rows[0].Position != 0 {
		t.Errorf("first row = %#v, want human turn at shard position 0", rows[0])
	}
	if rows[0].Fields["Prompt"] != "inspect" {
		t.Errorf("first row fields = %#v, want Prompt=inspect", rows[0].Fields)
	}
	if rows[1].Event != "GATE_APPROVED" || rows[1].Position != 1 || rows[1].Fields["Stage"] != "feasibility" {
		t.Errorf("second row = %#v, want gate approval at position 1", rows[1])
	}
}

func TestParseAuditShardRejectsAmbiguousAuthorityFields(t *testing.T) {
	t.Parallel()

	content := "# Audit\n\n## Human Turn\n" +
		"**Timestamp**: 2026-09-04T10:00:00Z\n" +
		"**Timestamp**: 2026-09-04T10:00:01Z\n" +
		"**Event**: HUMAN_TURN\n\n---\n"
	if _, err := parseAuditShard("audit/first.md", []byte(content)); err == nil {
		t.Fatal("parseAuditShard() error = nil, want duplicate authority rejection")
	} else if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("parseAuditShard() error = %v, want fs.ErrInvalid", err)
	}
}

func TestHumanTurnFreshRequiresHumanTurn(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	if HumanTurnFresh([]AuditRecord{{Event: "GATE_APPROVED", Timestamp: when, Shard: "a.md", Position: 0}}) {
		t.Fatal("resolution without HUMAN_TURN was treated as fresh")
	}
	if !HumanTurnFresh([]AuditRecord{{Event: "HUMAN_TURN", Timestamp: when, Shard: "a.md", Position: 0}}) {
		t.Fatal("HUMAN_TURN without a resolution was not treated as fresh")
	}
}

func TestHumanTurnFreshRecognizesEveryResolutionKind(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	for _, event := range []string{
		"GATE_APPROVED",
		"GATE_REJECTED",
		"QUESTION_ANSWERED",
		"SUMMARY_CONFIRMATION_RECORDED",
		"PLAN_APPROVAL_RECORDED",
	} {
		event := event
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			if HumanTurnFresh([]AuditRecord{
				{Event: "HUMAN_TURN", Timestamp: base, Shard: "a.md", Position: 0},
				{Event: event, Timestamp: base.Add(time.Second), Shard: "a.md", Position: 1},
			}) {
				t.Fatalf("%s did not consume an earlier HUMAN_TURN", event)
			}
			if !HumanTurnFresh([]AuditRecord{
				{Event: event, Timestamp: base, Shard: "a.md", Position: 0},
				{Event: "HUMAN_TURN", Timestamp: base.Add(time.Second), Shard: "a.md", Position: 1},
			}) {
				t.Fatalf("HUMAN_TURN after %s was not fresh", event)
			}
		})
	}

	if HumanTurnFresh([]AuditRecord{
		{Event: "HUMAN_TURN", Timestamp: base, Shard: "a.md", Position: 0},
		{Event: "AUTONOMY_MODE_SET", Timestamp: base.Add(time.Second), Fields: map[string]string{"Mode": "autonomous"}, Shard: "a.md", Position: 1},
	}) {
		t.Fatal("autonomous AUTONOMY_MODE_SET did not consume HUMAN_TURN")
	}
	if !HumanTurnFresh([]AuditRecord{
		{Event: "HUMAN_TURN", Timestamp: base, Shard: "a.md", Position: 0},
		{Event: "AUTONOMY_MODE_SET", Timestamp: base.Add(time.Second), Fields: map[string]string{"Mode": "gated"}, Shard: "a.md", Position: 1},
	}) {
		t.Fatal("gated AUTONOMY_MODE_SET incorrectly consumed HUMAN_TURN")
	}
}

func TestHumanTurnFreshRejectsMissingTimestampAuthority(t *testing.T) {
	t.Parallel()
	if HumanTurnFresh([]AuditRecord{{Event: "HUMAN_TURN", Shard: "a.md", Position: 0}}) {
		t.Fatal("HUMAN_TURN without a timestamp was treated as fresh")
	}
	when := time.Date(2026, 9, 4, 10, 0, 0, 500_000_000, time.UTC)
	if HumanTurnFresh([]AuditRecord{{Event: "HUMAN_TURN", Timestamp: when, Shard: "a.md", Position: 0}}) {
		t.Fatal("subsecond HUMAN_TURN timestamp was treated as canonical")
	}
}

func TestHumanTurnFreshUsesShardAndPositionForEqualSeconds(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		records []AuditRecord
		want    bool
	}{
		{
			name: "different shards fail closed",
			records: []AuditRecord{
				{Event: "GATE_APPROVED", Timestamp: when, Shard: "a.md", Position: 0},
				{Event: "HUMAN_TURN", Timestamp: when, Shard: "b.md", Position: 0},
			},
			want: false,
		},
		{
			name: "same shard resolution before turn",
			records: []AuditRecord{
				{Event: "GATE_APPROVED", Timestamp: when, Shard: "a.md", Position: 0},
				{Event: "HUMAN_TURN", Timestamp: when, Shard: "a.md", Position: 1},
			},
			want: true,
		},
		{
			name: "same shard turn before resolution",
			records: []AuditRecord{
				{Event: "HUMAN_TURN", Timestamp: when, Shard: "a.md", Position: 0},
				{Event: "GATE_APPROVED", Timestamp: when, Shard: "a.md", Position: 1},
			},
			want: false,
		},
		{
			name: "later cross shard turn",
			records: []AuditRecord{
				{Event: "GATE_APPROVED", Timestamp: when, Shard: "a.md", Position: 0},
				{Event: "HUMAN_TURN", Timestamp: when.Add(time.Second), Shard: "b.md", Position: 0},
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := HumanTurnFresh(test.records); got != test.want {
				t.Fatalf("HumanTurnFresh() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestParseAuditShardRejectsTruncatedEventBlock(t *testing.T) {
	t.Parallel()
	content := "# AI-DLC Audit Log\n\n## Human Turn\n" +
		"**Timestamp**: 2026-09-04T10:00:00Z\n" +
		"**Event**: HUMAN_TURN\n\n---\n" +
		"\n## Gate Approved\n"
	if _, err := parseAuditShard("audit/first.md", []byte(content)); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("parseAuditShard() error = %v, want fs.ErrInvalid", err)
	}
}

func TestParseAuditShardRejectsDecreasingTimestamps(t *testing.T) {
	t.Parallel()
	content := "# AI-DLC Audit Log\n\n## Human Turn\n" +
		"**Timestamp**: 2026-09-04T10:00:01Z\n" +
		"**Event**: HUMAN_TURN\n\n---\n" +
		"\n## Gate Approved\n" +
		"**Timestamp**: 2026-09-04T10:00:00Z\n" +
		"**Event**: GATE_APPROVED\n\n---\n"
	if _, err := parseAuditShard("audit/first.md", []byte(content)); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("parseAuditShard() error = %v, want fs.ErrInvalid", err)
	}
}
