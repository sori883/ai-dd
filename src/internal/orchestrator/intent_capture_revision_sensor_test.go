package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
)

func TestIntentCaptureRevisionGateRefiresAllAdvisorySensors(t *testing.T) {
	fixture := newIntentCaptureRevisionSensorFixture(t)
	_, err := ReviseGate(context.Background(), GateInput{
		Identity:    fixture.identity,
		ProjectRoot: fixture.projectRoot,
		RecordRoot:  fixture.recordRoot,
		Current:     fixture.stage,
		Catalog:     fixture.catalog,
	})
	if !errors.Is(err, ErrGateNotReady) {
		t.Fatalf("ReviseGate() error = %v, want ErrGateNotReady after sensor attempts", err)
	}
	t.Logf("ReviseGate() returned expected not-ready error: %v", err)
	records := intentCaptureRevisionSensorRecords(t, fixture)
	fired := 0
	for _, record := range records {
		if record.Event == "SENSOR_FIRED" {
			fired++
		}
	}
	if fired != 3 {
		t.Fatalf("revision SENSOR_FIRED count = %d, want all three advisory attempts; records = %#v", fired, records)
	}
}

type intentCaptureRevisionSensorFixture struct {
	identity    recordlock.Identity
	projectRoot *os.Root
	recordRoot  *os.Root
	catalog     graph.Snapshot
	stage       graph.Stage
}

func newIntentCaptureRevisionSensorFixture(t *testing.T) intentCaptureRevisionSensorFixture {
	t.Helper()
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "aidlc", ".aidlc-clone-id"), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(clone id): %v", err)
	}
	graphFS := fstest.MapFS{
		"stage-graph.json": &fstest.MapFile{Data: []byte(`[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"aidlc-orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
	{"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["enterprise","feature","mvp","poc"],"enabled":true,"summary_confirmation":"required","reviewer":"aidlc-product-lead-agent","reviewer_max_iterations":2,"review_class":"advisory","review_artifact":"intent-statement","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"requires_stage":[]}
]`)},
		"scope-grid.json": &fstest.MapFile{Data: []byte(`{"feature":{"stages":{"workspace-scaffold":"EXECUTE","intent-capture":"EXECUTE"}}}`)},
	}
	catalog, err := graph.Load(graphFS)
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph:              catalog,
		Scope:              "feature",
		ScopeMetadata:      scope.Metadata{Name: "feature", Depth: "Standard"},
		Workspace:          state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:        projectDir,
		ProjectDescription: "revision sensor",
		StartDate:          "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	content, err := state.Patch([]byte(initial.StateContent), state.PatchRequest{StageMarkers: []state.StageMarkerPatch{{Slug: "intent-capture", Expected: state.StageMarkerInProgress, Replacement: state.StageMarkerRevising}}})
	if err != nil {
		t.Fatalf("state.Patch(revising): %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, "aidlc-state.md"), content, 0o600); err != nil {
		t.Fatalf("WriteFile(state): %v", err)
	}
	for name, body := range map[string]string{
		"intent-capture-questions.md": "# questions\n",
		"intent-statement.md":         "## Problem\n- [desc] a claim\n## Assumptions & Open Questions\n- None.\n",
		"stakeholder-map.md":          "## Stakeholders\n- [desc] a stakeholder\n## Assumptions & Open Questions\n- None.\n",
	} {
		if err := os.WriteFile(filepath.Join(recordDir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatalf("recordlock.NewIdentity(): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("os.OpenRoot(project): %v", err)
	}
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("os.OpenRoot(record): %v", err)
	}
	t.Cleanup(func() {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
	})
	var stage graph.Stage
	for _, candidate := range catalog.Stages() {
		if candidate.Slug == "intent-capture" {
			stage = candidate
			break
		}
	}
	if stage.Slug == "" {
		t.Fatal(fmt.Errorf("intent-capture stage missing from test graph"))
	}
	return intentCaptureRevisionSensorFixture{identity: identity, projectRoot: projectRoot, recordRoot: recordRoot, catalog: catalog, stage: stage}
}

func intentCaptureRevisionSensorRecords(t *testing.T, fixture intentCaptureRevisionSensorFixture) []audit.AuditRecord {
	t.Helper()
	var records []audit.AuditRecord
	err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = audit.ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	})
	if err != nil {
		t.Fatalf("audit.ReadEvents(): %v", err)
	}
	return records
}
