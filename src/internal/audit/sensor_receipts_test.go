package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/sensor"
)

func TestSensorReceiptRecording(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := sensor.RunAll(context.Background(), sensor.Input{
		Stage:              "intent-capture",
		ArtifactPath:       "intent-statement.md",
		ProjectDescription: "A project",
		Scope:              "classic",
		Content:            []byte("# Intent\n## Initial Scope Signal\n- [scope] Workflow-selected scope: `classic`.\n## Sources\n- [desc] Initial description: \"A project\"\n## Assumptions & Open Questions\n- None.\n"),
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
	}, nil)[0]
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 2 || records[0].Event != "SENSOR_FIRED" || records[1].Event != "SENSOR_PASSED" {
		t.Fatalf("records = %#v, want FIRED then PASSED", records)
	}
	if records[0].Fields["Fire id"] == "" || records[0].Fields["Fire id"] != records[1].Fields["Fire id"] {
		t.Fatalf("sensor fire id fields = %#v / %#v, want matching id", records[0].Fields, records[1].Fields)
	}
}

func TestSensorEventsUseFixedVocabulary(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	invocation := sensor.RunAll(context.Background(), sensor.Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("# Intent\n## Sources\n- Source [desc]\n## Assumptions & Open Questions\n- None [assumption]\n")}, nil)[0]
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %#v, want fired and fixed terminal", records)
	}
	switch records[1].Event {
	case "SENSOR_PASSED", "SENSOR_FAILED", "SENSOR_BUDGET_OVERRIDE":
	default:
		t.Fatalf("terminal event = %q, want fixed sensor vocabulary", records[1].Event)
	}
}

func TestSensorEventsUseFixedFieldContractAndNote(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	input := sensor.Input{Stage: "intent-capture", ArtifactPath: "ideation/intent-capture/intent-statement.md", Content: []byte("## Problem\n## Outcome\n")}
	invocation := sensor.RunAll(context.Background(), input, map[string]sensor.CheckFunc{
		sensor.ClaimSourcesID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{}, os.ErrPermission
		},
	})[0]
	if invocation.TerminalResult().Status != sensor.StatusCompleted || invocation.TerminalResult().Note == "" {
		t.Fatalf("script-error terminal = %#v, want passed with Note", invocation.TerminalResult())
	}
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	records := readSensorRecords(t, fixture)
	if got := records[0].Fields["Fire id"]; got == "" || got != records[1].Fields["Fire id"] {
		t.Fatalf("canonical fire ids = %q/%q, want matching non-empty values", records[0].Fields["Fire id"], records[1].Fields["Fire id"])
	}
	for _, field := range []string{"Sensor ID", "Stage slug", "Output path"} {
		if records[0].Fields[field] == "" || records[1].Fields[field] == "" {
			t.Fatalf("canonical field %q missing: fired=%#v terminal=%#v", field, records[0].Fields, records[1].Fields)
		}
	}
	if records[1].Fields["Note"] == "" || records[1].Event != "SENSOR_PASSED" {
		t.Fatalf("script-error terminal record = %#v, want SENSOR_PASSED with Note", records[1])
	}
	if got, want := records[1].Fields["Note"], "script-error: permission denied"; got != want {
		t.Fatalf("script-error note = %q, want %q", got, want)
	}
	for _, field := range []string{"Stage", "Sensor", "Fire ID", "Status", "Detail", "Findings"} {
		if _, present := records[0].Fields[field]; present {
			t.Errorf("fired record contains legacy field %q: %#v", field, records[0].Fields)
		}
		if _, present := records[1].Fields[field]; present {
			t.Errorf("terminal record contains legacy field %q: %#v", field, records[1].Fields)
		}
	}
}

func TestSensorFailureWritesDetailWithoutGateAuthority(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	input := sensor.Input{Stage: "intent-capture", ArtifactPath: "ideation/intent-capture/intent-statement.md", Content: []byte("## Problem\n## Outcome\n")}
	invocation := sensor.RunAll(context.Background(), input, map[string]sensor.CheckFunc{
		sensor.ClaimSourcesID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{Status: sensor.StatusFailed, Detail: "bad output", Findings: []sensor.Finding{{Path: "intent-statement.md", Message: "missing source"}}}, nil
		},
	})[0]
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	records := readSensorRecords(t, fixture)
	if records[1].Event != "SENSOR_FAILED" || records[1].Fields["Detail path"] == "" || records[1].Fields["Findings count"] != "1" {
		t.Fatalf("failure record = %#v, want fixed detail fields", records[1])
	}
	detailPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", filepath.FromSlash(records[1].Fields["Detail path"]))
	if detail, err := os.ReadFile(detailPath); err != nil || !strings.Contains(string(detail), "missing source") {
		t.Fatalf("detail file = %q, err = %v, want finding body", detail, err)
	}
	if sensor.AdvisoryOutcomesBlockGate([]sensor.Invocation{invocation}) || sensor.AdvisoryOutcomesAuthorizeGate([]sensor.Invocation{invocation}) {
		t.Fatal("sensor failure changed gate authority")
	}
}

func TestSensorDetailWriteFailureNormalizesToPassedNote(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	if err := os.WriteFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", ".aidlc-sensors"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile(detail directory blocker): %v", err)
	}
	invocation := sensor.RunAll(context.Background(), sensor.Input{
		Stage: "intent-capture", ArtifactPath: "ideation/intent-capture/intent-statement.md",
	}, map[string]sensor.CheckFunc{
		sensor.ClaimSourcesID: func(sensor.Input) (sensor.CheckResult, error) {
			return sensor.CheckResult{Status: sensor.StatusFailed, Detail: "detail should not be written", Findings: []sensor.Finding{{Message: "finding"}}}, nil
		},
	})[0]
	if err := RecordSensorInvocation(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, invocation); err != nil {
		t.Fatalf("RecordSensorInvocation(): %v", err)
	}
	records := readSensorRecords(t, fixture)
	if records[1].Event != "SENSOR_PASSED" {
		t.Fatalf("detail-write failure event = %q, want SENSOR_PASSED", records[1].Event)
	}
	if got := records[1].Fields["Note"]; !strings.HasPrefix(got, "script-error: detail-write-failed: ") {
		t.Fatalf("detail-write failure note = %q, want script-error prefix", got)
	}
	if _, present := records[1].Fields["Detail path"]; present {
		t.Fatalf("detail-write failure unexpectedly advertised detail path: %#v", records[1].Fields)
	}
}

func readSensorRecords(t *testing.T, fixture humanTurnWorkspaceFixture) []AuditRecord {
	t.Helper()
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("sensor records = %#v, want FIRED plus terminal", records)
	}
	return records
}
