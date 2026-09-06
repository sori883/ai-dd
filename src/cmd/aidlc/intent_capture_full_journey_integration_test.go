//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/audit"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
)

const intentCaptureJourneyGraphJSON = `[
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture & Framing","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["enterprise","feature","mvp","poc"],"enabled":true,"summary_confirmation":"required","reviewer":"aidlc-product-lead-agent","reviewer_max_iterations":2,"review_class":"advisory","review_artifact":"intent-statement","produces":["intent-statement","stakeholder-map","intent-capture-questions"],"consumes":[],"sensors":["claim-sources","required-sections","upstream-coverage"],"requires_stage":[]},
  {"slug":"market-research","number":"1.2","name":"Market Research","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":[],"mode":"inline","scopes":["feature"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

const intentCaptureJourneyScopeGridJSON = `{"feature":{"stages":{"intent-capture":"EXECUTE","market-research":"EXECUTE"}}}`

type intentCaptureJourneyFixture struct {
	project         string
	questions       string
	intentStatement string
	stakeholders    string
}

type intentCommandResult struct {
	exitCode int
	stdout   bytes.Buffer
	stderr   bytes.Buffer
}

func runCodexIntentCaptureFreshApprove(t *testing.T) {
	t.Helper()
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := buildIntentCaptureBinary(t, moduleRoot)
	fixture := newIntentCaptureJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, fixture.project)
	runIntentCaptureContext(t, binaryPath, fixture.project)
	driveIntentCaptureEvidence(t, binaryPath, fixture, "initial")

	awaiting := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if awaiting.exitCode != 0 || awaiting.kind != "print" {
		t.Fatalf("awaiting report = %#v, want successful print", awaiting)
	}
	content, err := os.ReadFile(filepath.Join(fixture.project, "aidlc", "spaces", "team", "memory", "project.md"))
	if err != nil || !strings.Contains(string(content), "Keep the intent traceable.") {
		t.Fatalf("project learning memory = %q, %v; want selected practice persisted", content, err)
	}
	if count := countJourneyAuditEvents(t, fixture.project, "RULE_LEARNED"); count != 1 {
		t.Fatalf("RULE_LEARNED count = %d, want one durable selection event", count)
	}
	assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"Approve"}`)
	approved := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "approved", "--user-input", "Approve")
	if approved.exitCode != 0 || approved.kind != "done" || !strings.Contains(approved.reason, "State advanced; run next to continue.") {
		t.Fatalf("approved report = %#v, want done with next-stage instruction", approved)
	}
	next := runDeliveryBinary(t, binaryPath, fixture.project, "next")
	if next.kind != "run-stage" {
		t.Fatalf("next after intent approval = %#v, want run-stage", next)
	}
	var nextWire struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(next.stdout.Bytes()), &nextWire); err != nil {
		t.Fatalf("decode next wire: %v; stdout=%q", err, next.stdout.String())
	}
	if nextWire.Stage != "market-research" {
		t.Fatalf("next stage = %q, want market-research", nextWire.Stage)
	}
}

func runCodexIntentCaptureRejectReviseApprove(t *testing.T) {
	t.Helper()
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := buildIntentCaptureBinary(t, moduleRoot)
	fixture := newIntentCaptureJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, fixture.project)
	runIntentCaptureContext(t, binaryPath, fixture.project)
	driveIntentCaptureEvidence(t, binaryPath, fixture, "initial")

	awaiting := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if awaiting.kind != "print" || awaiting.exitCode != 0 {
		t.Fatalf("initial awaiting report = %#v, want print", awaiting)
	}
	assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"Request Changes"}`)
	rejected := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "rejected", "--user-input", "Request Changes", "--reason", "revise the captured intent")
	if rejected.kind != "print" || rejected.exitCode != 0 {
		t.Fatalf("rejected report = %#v, want print", rejected)
	}
	oldEvidence := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "revised")
	if oldEvidence.kind != "error" || oldEvidence.exitCode != 0 {
		t.Fatalf("revised report with old evidence = %#v, want workflow error", oldEvidence)
	}
	driveIntentCaptureEvidence(t, binaryPath, fixture, "revision")
	revised := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "revised")
	if revised.kind != "print" || revised.exitCode != 0 {
		t.Fatalf("revised report = %#v, want print after fresh evidence", revised)
	}
	assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"Approve"}`)
	approved := runReportBinary(t, binaryPath, fixture.project, nil, "report", "--stage", "intent-capture", "--result", "approved", "--user-input", "Approve")
	if approved.kind != "done" || approved.exitCode != 0 {
		t.Fatalf("approved after revision = %#v, want done", approved)
	}
	if count := countJourneyAuditEvents(t, fixture.project, "RULE_LEARNED"); count != 1 {
		t.Fatalf("revision RULE_LEARNED count = %d, want original learning only", count)
	}
	if count := countJourneyAuditEvents(t, fixture.project, "QUESTION_ANSWERED"); count < 2 {
		t.Fatalf("revision QUESTION_ANSWERED count = %d, want original question and learning answers", count)
	}
	if count := countJourneyAuditEventsWithField(t, fixture.project, "QUESTION_ANSWERED", "Learning Selection", "learning"); count != 1 {
		t.Fatalf("revision learning QUESTION_ANSWERED count = %d, want one original learning answer", count)
	}
	if count := countJourneyAuditEvents(t, fixture.project, "SENSOR_FIRED"); count != 6 {
		t.Fatalf("revision SENSOR_FIRED count = %d, want one set per review cycle", count)
	}
	if count := countJourneyAuditEvents(t, fixture.project, "REVIEW_COMPLETED"); count != 2 {
		t.Fatalf("revision REVIEW_COMPLETED count = %d, want initial and challenged recovery review", count)
	}
}

func runCodexIntentCaptureFailsClosed(t *testing.T) {
	t.Helper()
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := buildIntentCaptureBinary(t, moduleRoot)

	changedSummary := newIntentCaptureJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, changedSummary.project)
	driveIntentCaptureEvidence(t, binaryPath, changedSummary, "summary")
	if err := os.WriteFile(changedSummary.questions, []byte("# changed questions\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed questions): %v", err)
	}
	summaryFailure := runReportBinary(t, binaryPath, changedSummary.project, nil, "report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if summaryFailure.kind != "error" || summaryFailure.exitCode != 0 {
		t.Fatalf("changed summary report = %#v, want workflow error", summaryFailure)
	}

	changedReview := newIntentCaptureJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, changedReview.project)
	driveIntentCaptureEvidence(t, binaryPath, changedReview, "review")
	if file, err := os.OpenFile(changedReview.intentStatement, os.O_APPEND|os.O_WRONLY, 0); err != nil {
		t.Fatalf("OpenFile(changed review): %v", err)
	} else {
		_, writeErr := file.WriteString("\nchanged after review completion\n")
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatalf("change review artifact: write=%v close=%v", writeErr, closeErr)
		}
	}
	reviewFailure := runReportBinary(t, binaryPath, changedReview.project, nil, "report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if reviewFailure.kind != "error" || reviewFailure.exitCode != 0 {
		t.Fatalf("changed review report = %#v, want workflow error", reviewFailure)
	}

	sensorFailure := newIntentCaptureJourneyProject(t)
	installCodexHookConfig(t, moduleRoot, sensorFailure.project)
	driveIntentCaptureEvidence(t, binaryPath, sensorFailure, "sensor")
	projectDescription := filepath.Join(sensorFailure.project, "aidlc", "spaces", "team", "intents", "build", "project-description.json")
	if err := os.WriteFile(projectDescription, []byte(`"<document>unclosed"`), 0o600); err != nil {
		t.Fatalf("WriteFile(malformed project description): %v", err)
	}
	sensorWire := runCodexStageCommand(t, binaryPath, sensorFailure.project, "run-sensors", `{}`)
	if sensorWire.exitCode != 0 {
		t.Fatalf("sensor failure invocation = %#v, want successful advisory wire", sensorWire)
	}
	var sensorResult struct {
		Kind    string `json:"kind"`
		Results []struct {
			Status   string `json:"status"`
			Terminal bool   `json:"terminal"`
		} `json:"results"`
	}
	decodeIntentWire(t, sensorWire, &sensorResult)
	if sensorResult.Kind != "sensor-results" || len(sensorResult.Results) != 3 {
		t.Fatalf("sensor result wire = %#v, want all three advisory results", sensorResult)
	}
	foundFailure := false
	for _, result := range sensorResult.Results {
		if result.Status == "FAILED" {
			foundFailure = true
		}
	}
	if !foundFailure {
		t.Fatalf("sensor results = %#v, want an observable non-authoritative failure", sensorResult.Results)
	}
	assertConfiguredHook(t, binaryPath, sensorFailure.project, `{"prompt":"learning answer"}`)
	learnings := runCodexStageCommand(t, binaryPath, sensorFailure.project, "learnings-surface", `{}`)
	if learnings.exitCode != 0 {
		t.Fatalf("learnings-surface after sensor failure = %#v", learnings)
	}
	assertConfiguredHook(t, binaryPath, sensorFailure.project, `{"prompt":"none"}`)
	persisted := runCodexStageCommand(t, binaryPath, sensorFailure.project, "learnings-persist", `{"selections":[]}`)
	if persisted.exitCode != 0 {
		t.Fatalf("learnings-persist after sensor failure = %#v", persisted)
	}
	awaiting := runReportBinary(t, binaryPath, sensorFailure.project, nil, "report", "--stage", "intent-capture", "--result", "awaiting-approval")
	if awaiting.kind != "print" || awaiting.exitCode != 0 {
		t.Fatalf("gate after advisory sensor failure = %#v, want print", awaiting)
	}
}

func buildIntentCaptureBinary(t *testing.T, moduleRoot string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "aidlc")
	build := exec.Command("go", "build", "-o", binaryPath, "./src/cmd/aidlc")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build aidlc: %v\n%s", err, output)
	}
	return binaryPath
}

func newIntentCaptureJourneyProject(t *testing.T) intentCaptureJourneyFixture {
	t.Helper()
	project := t.TempDir()
	dataDir := filepath.Join(project, ".codex", "tools", "data")
	recordDir := filepath.Join(project, "aidlc", "spaces", "team", "intents", "build")
	for _, directory := range []string{
		dataDir,
		recordDir,
		filepath.Join(recordDir, "ideation", "intent-capture"),
		filepath.Join(project, "aidlc", "spaces", "team", "knowledge"),
		filepath.Join(project, "aidlc", "spaces", "team", "memory"),
		filepath.Join(project, ".codex", "agents"),
		filepath.Join(project, ".codex", "sensors"),
		filepath.Join(project, ".codex", "aidlc-common", "protocols"),
		filepath.Join(project, ".codex", "aidlc-common", "stages", "ideation"),
		filepath.Join(project, ".codex", "skills", "aidlc"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", directory, err)
		}
	}
	writeIntentCaptureFile(t, filepath.Join(project, "aidlc", "active-space"), "team\n")
	writeIntentCaptureFile(t, filepath.Join(project, "aidlc", "spaces", "team", "intents", "active-intent"), "build\n")
	writeIntentCaptureFile(t, filepath.Join(project, "aidlc", "spaces", "team", "intents", "intents.json"), `[{"uuid":"intent-capture-journey","slug":"intent-capture-journey","status":"planning","dirName":"build"}]`)
	writeIntentCaptureFile(t, filepath.Join(dataDir, "stage-graph.json"), intentCaptureJourneyGraphJSON)
	writeIntentCaptureFile(t, filepath.Join(dataDir, "scope-grid.json"), intentCaptureJourneyScopeGridJSON)
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "agents", "aidlc-product-agent.md"), "product persona\n")
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "agents", "aidlc-architect-agent.md"), "architect persona\n")
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "aidlc-common", "protocols", "stage-protocol.md"), "base protocol\n")
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "aidlc-common", "protocols", "stage-protocol-reviewer.md"), "reviewer protocol\n")
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "aidlc-common", "protocols", "stage-protocol-ensemble.md"), "ensemble protocol\n")
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "skills", "aidlc", "question-rendering.md"), "question annex\n")
	for _, source := range []string{
		"src/harness/codex/skills/aidlc/SKILL.md",
		"src/harness/codex/skills/aidlc/question-rendering.md",
		"src/harness/codex/agents/aidlc-product-lead-agent.toml",
	} {
		content, err := os.ReadFile(filepath.Join(deliveryModuleRoot(t), filepath.FromSlash(source)))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", source, err)
		}
		destination := filepath.Join(project, ".codex", filepath.FromSlash(strings.TrimPrefix(source, "src/harness/codex/")))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s): %v", destination, err)
		}
		if strings.HasSuffix(source, "aidlc-product-lead-agent.toml") && !strings.Contains(string(content), `name = "aidlc-product-lead-agent"`) {
			t.Fatalf("%s does not contain the canonical product-lead identity", source)
		}
		writeIntentCaptureFile(t, destination, string(content))
	}
	writeIntentCaptureFile(t, filepath.Join(project, "aidlc", "spaces", "team", "memory", "project.md"), "## Corrections\n\n")
	writeIntentCaptureFile(t, filepath.Join(project, "aidlc", "spaces", "team", "memory", "team.md"), "## Corrections\n\n")
	writeIntentCaptureFile(t, filepath.Join(recordDir, "ideation", "intent-capture", "memory.md"), "## Interpretations\n- Keep the intent traceable.\n\n## Open questions\n- Which rollout cohort comes first?\n")
	for _, sensorID := range []string{"claim-sources", "required-sections", "upstream-coverage"} {
		writeIntentCaptureFile(t, filepath.Join(project, ".codex", "sensors", "aidlc-"+sensorID+".md"), "---\nid: "+sensorID+"\nkind: check\ncommand: "+sensorID+"\ndefault_severity: advisory\nfire_on: gate\n---\n")
	}
	writeIntentCaptureFile(t, filepath.Join(project, ".codex", "aidlc-common", "stages", "ideation", "intent-capture.md"), "---\nslug: intent-capture\nphase: ideation\nsensors:\n  - claim-sources\n  - required-sections\n  - upstream-coverage\n---\nintent-capture stage protocol\n")

	catalog, err := graph.Load(os.DirFS(dataDir))
	if err != nil {
		t.Fatalf("graph.Load(): %v", err)
	}
	initial, err := state.BuildInitial(state.Input{
		Graph: catalog, Scope: "feature",
		ScopeMetadata:             scope.Metadata{Name: "feature", Depth: "Standard", TestStrategy: "Standard"},
		Workspace:                 state.WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               project,
		ProjectDescription:        "intent capture journey",
		ProjectDescriptionPreview: "intent capture journey",
		StartDate:                 "2026-09-06T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("state.BuildInitial(): %v", err)
	}
	writeIntentCaptureFile(t, filepath.Join(recordDir, "aidlc-state.md"), initial.StateContent)
	writeIntentCaptureFile(t, filepath.Join(recordDir, "project-description.json"), initial.ProjectDescriptionJSON)
	questions := filepath.Join(recordDir, "ideation", "intent-capture", "intent-capture-questions.md")
	intentStatement := filepath.Join(recordDir, "ideation", "intent-capture", "intent-statement.md")
	stakeholders := filepath.Join(recordDir, "ideation", "intent-capture", "stakeholder-map.md")
	if err := os.MkdirAll(filepath.Dir(questions), 0o700); err != nil {
		t.Fatalf("MkdirAll(intent-capture artifacts): %v", err)
	}
	writeIntentCaptureQuestions(t, questions, "")
	return intentCaptureJourneyFixture{project: project, questions: questions, intentStatement: intentStatement, stakeholders: stakeholders}
}

func driveIntentCaptureEvidence(t *testing.T, binaryPath string, fixture intentCaptureJourneyFixture, label string) {
	t.Helper()
	if label != "revision" {
		decision := runCodexStageCommand(t, binaryPath, fixture.project, "decision", `{"decision":"q1","options":"A. Keep it local,B. Use a service"}`)
		assertIntentWireKind(t, decision, "decision")
		assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"A. Keep it local"}`)
		answer := runCodexStageCommand(t, binaryPath, fixture.project, "answer", `{"decision":"q1","answer":"A. Keep it local"}`)
		assertIntentWireKind(t, answer, "answer")
		writeIntentCaptureQuestions(t, fixture.questions, "A. Keep it local")

		summary := runCodexStageCommand(t, binaryPath, fixture.project, "summary", `{}`)
		assertIntentWireKind(t, summary, "decision")
		assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"Looks correct"}`)
		writeIntentCaptureSummaryAnswer(t, fixture.questions, "Looks correct")
		summaryAnswer := runCodexStageCommand(t, binaryPath, fixture.project, "answer", `{"decision":"summary","answer":"Looks correct"}`)
		assertIntentWireKind(t, summaryAnswer, "answer")
		writeIntentCaptureArtifacts(t, fixture, label)
	} else {
		// A revision keeps valid question, summary, and learning receipts. Only
		// the artifact prefix is revised before the old appendix is challenged.
		reviseIntentCaptureArtifacts(t, fixture)
	}

	reviewRequest := runCodexStageCommand(t, binaryPath, fixture.project, "review-request", `{}`)
	var requestWire struct {
		Kind            string `json:"kind"`
		Artifact        string `json:"artifact"`
		AppendixOffset  int64  `json:"appendix_offset"`
		Iteration       int    `json:"iteration"`
		ReviewChallenge string `json:"review_challenge"`
	}
	decodeIntentWire(t, reviewRequest, &requestWire)
	if requestWire.Kind != "review-requested" {
		t.Fatalf("review request wire = %#v, want review-requested", requestWire)
	}
	if requestWire.Iteration < 1 || (label == "revision" && requestWire.Iteration != 2) {
		t.Fatalf("review request iteration = %d, want initial positive or revision 2", requestWire.Iteration)
	}
	if label == "revision" {
		if requestWire.ReviewChallenge == "" || requestWire.Artifact == "" {
			t.Fatalf("revision review request = %#v, want challenge and artifact", requestWire)
		}
		removeIntentCaptureReview(t, fixture.intentStatement, requestWire.AppendixOffset)
	}
	appendIntentCaptureReview(t, fixture.intentStatement, "READY", requestWire.Iteration, label, requestWire.ReviewChallenge)
	reviewComplete := runCodexStageCommand(t, binaryPath, fixture.project, "review-complete", `{"verdict":"READY"}`)
	assertIntentWireKind(t, reviewComplete, "review-completed")

	sensors := runCodexStageCommand(t, binaryPath, fixture.project, "run-sensors", `{}`)
	if sensors.exitCode != 0 {
		t.Fatalf("run-sensors = %#v", sensors)
	}
	var sensorWire struct {
		Kind    string `json:"kind"`
		Results []struct {
			Sensor string `json:"sensor"`
		} `json:"results"`
	}
	decodeIntentWire(t, sensors, &sensorWire)
	if sensorWire.Kind != "sensor-results" || len(sensorWire.Results) != 3 {
		t.Fatalf("sensor wire = %#v, want all three sensors", sensorWire)
	}
	// The failure journey corrupts an artifact after the sensor attempt so the
	// subsequent learning question is the first one for this gate attempt.
	// A revision keeps the original learning receipt and therefore does not
	// surface or persist a second learning decision.
	if label == "sensor" || label == "revision" {
		return
	}

	learnings := runCodexStageCommand(t, binaryPath, fixture.project, "learnings-surface", `{}`)
	if learnings.exitCode != 0 {
		t.Fatalf("learnings-surface = %#v", learnings)
	}
	var learningWire struct {
		Candidates []struct {
			ID      string
			Heading string
			Text    string
		} `json:"candidates"`
	}
	decodeIntentWire(t, learnings, &learningWire)
	if len(learningWire.Candidates) == 0 {
		t.Fatalf("learnings-surface = %s, want fixture practice candidate", learnings.stdout.String())
	}
	candidate := learningWire.Candidates[0]
	selection, err := json.Marshal(map[string]any{"selections": []any{map[string]any{
		"candidate_id": candidate.ID,
		"type":         "learning",
		"scope":        "project",
		"heading":      candidate.Heading,
		"text":         candidate.Text,
		"source":       "orchestrator",
	}}})
	if err != nil {
		t.Fatalf("marshal learning selection: %v", err)
	}
	assertConfiguredHook(t, binaryPath, fixture.project, `{"prompt":"Keep the intent traceable"}`)
	persisted := runCodexStageCommand(t, binaryPath, fixture.project, "learnings-persist", string(selection))
	if persisted.exitCode != 0 {
		t.Fatalf("learnings-persist = %#v", persisted)
	}
}

func writeIntentCaptureQuestions(t *testing.T, name, answer string) {
	t.Helper()
	content := "# Intent Capture Questions\n\n" +
		"## Sources\n" +
		"- [desc] Initial description: \"intent capture journey\"\n" +
		"- [scope] Workflow-selected scope: `feature`.\n\n" +
		"## Q1\n" +
		"Should the first release stay local?\n" +
		"[Answer]: " + answer + "\n\n" +
		"## Consolidated Summary Confirmation\n" +
		"- Looks correct\n" +
		"- Request changes\n" +
		"[Answer]: \n\n" +
		"## Assumption Confirmation\n" +
		"[Answer]: A. Accept assumptions\n"
	writeIntentCaptureFile(t, name, content)
}

func writeIntentCaptureSummaryAnswer(t *testing.T, name, answer string) {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(summary questions): %v", err)
	}
	updated := strings.Replace(string(content), "## Consolidated Summary Confirmation\n- Looks correct\n- Request changes\n[Answer]: \n", "## Consolidated Summary Confirmation\n- Looks correct\n- Request changes\n[Answer]: "+answer+"\n", 1)
	if updated == string(content) {
		t.Fatalf("summary answer placeholder not found in %s", name)
	}
	writeIntentCaptureFile(t, name, updated)
}

func writeIntentCaptureArtifacts(t *testing.T, fixture intentCaptureJourneyFixture, label string) {
	t.Helper()
	intent := "# Intent Statement\n\n## Initial Scope Signal\n- [scope] Workflow-selected scope: `feature`.\n\n## Problem\n- [desc] The captured workflow should remain understandable and locally testable (" + label + ").\n\n## Assumptions & Open Questions\n- None.\n"
	stakeholders := "# Stakeholder Map\n\n## Stakeholders\n- [desc] The product team needs a traceable intent record (" + label + ").\n\n## Assumptions & Open Questions\n- None.\n"
	writeIntentCaptureFile(t, fixture.intentStatement, intent)
	writeIntentCaptureFile(t, fixture.stakeholders, stakeholders)
}

func reviseIntentCaptureArtifacts(t *testing.T, fixture intentCaptureJourneyFixture) {
	t.Helper()
	for _, name := range []string{fixture.intentStatement, fixture.stakeholders} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(revision artifact %s): %v", name, err)
		}
		updated := append([]byte(nil), content...)
		if name == fixture.intentStatement {
			marker := []byte("\n## Review")
			offset := bytes.Index(content, marker)
			if offset < 0 {
				t.Fatalf("revision review artifact %s has no retained review appendix", name)
			}
			updated = append([]byte(nil), content[:offset]...)
			updated = append(updated, []byte("\nRevision clarification retained before the new review.\n")...)
			updated = append(updated, content[offset:]...)
		} else {
			updated = append(updated, []byte("\nRevision clarification retained before the new review.\n")...)
		}
		writeIntentCaptureFile(t, name, string(updated))
	}
}

func removeIntentCaptureReview(t *testing.T, name string, offset int64) {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(remove review): %v", err)
	}
	if offset < 0 || offset > int64(len(content)) {
		t.Fatalf("review appendix offset %d outside %d-byte artifact", offset, len(content))
	}
	writeIntentCaptureFile(t, name, string(content[:offset]))
}

func appendIntentCaptureReview(t *testing.T, name, verdict string, iteration int, label, challenge string) {
	t.Helper()
	file, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(review artifact): %v", err)
	}
	challengeLine := ""
	if challenge != "" {
		challengeLine = "**Request Challenge:** " + challenge + "\n"
	}
	_, writeErr := fmt.Fprintf(file, "\n## Review\n**Verdict:** %s\n**Reviewer:** aidlc-product-lead-agent\n**Date:** 2026-09-06T00:00:00Z\n**Iteration:** %d\n%s\n### Findings\n\n| ID | Severity | Location | Finding | Required action | Status |\n|---|---|---|---|---|---|\n\n### Summary\n\n%s review\n", verdict, iteration, challengeLine, label)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append review: write=%v close=%v", writeErr, closeErr)
	}
}

func assertConfiguredHook(t *testing.T, binaryPath, project, payload string) {
	t.Helper()
	result := runConfiguredHumanTurnHook(t, binaryPath, project, []byte(payload), nil)
	if result.exitCode != 0 || result.stdout.Len() != 0 || result.stderr.Len() != 0 {
		t.Fatalf("configured UserPromptSubmit hook = code %d stdout=%q stderr=%q, want silent success", result.exitCode, result.stdout.String(), result.stderr.String())
	}
}

func runCodexStageCommand(t *testing.T, binaryPath, project, action, payload string) intentCommandResult {
	t.Helper()
	return runIntentCommand(t, binaryPath, project, []byte(payload), "__codex-stage", action, "--project-dir", project)
}

func runIntentCommand(t *testing.T, binaryPath, project string, stdin []byte, args ...string) intentCommandResult {
	t.Helper()
	command := exec.Command(binaryPath, args...)
	command.Dir = project
	command.Env = os.Environ()
	command.Stdin = bytes.NewReader(stdin)
	var result intentCommandResult
	command.Stdout = &result.stdout
	command.Stderr = &result.stderr
	err := command.Run()
	if err == nil {
		result.exitCode = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.exitCode = exitErr.ExitCode()
	} else {
		t.Fatalf("run %q: %v", args, err)
	}
	return result
}

func assertIntentWireKind(t *testing.T, result intentCommandResult, want string) {
	t.Helper()
	if result.exitCode != 0 {
		t.Fatalf("hidden command exit = %d stderr=%q", result.exitCode, result.stderr.String())
	}
	var wire struct {
		Kind string `json:"kind"`
	}
	decodeIntentWire(t, result, &wire)
	if wire.Kind != want {
		t.Fatalf("hidden command wire kind = %q, want %q; stdout=%q", wire.Kind, want, result.stdout.String())
	}
}

func decodeIntentWire(t *testing.T, result intentCommandResult, target any) {
	t.Helper()
	if strings.Count(result.stdout.String(), "\n") != 1 {
		t.Fatalf("hidden command stdout = %q, want one JSON line", result.stdout.String())
	}
	if err := json.Unmarshal(bytes.TrimSpace(result.stdout.Bytes()), target); err != nil {
		t.Fatalf("decode hidden command wire: %v; stdout=%q", err, result.stdout.String())
	}
}

func countJourneyAuditEvents(t *testing.T, project, event string) int {
	t.Helper()
	count := 0
	for _, record := range journeyAuditRecords(t, project) {
		if record.Event == event {
			count++
		}
	}
	return count
}

func countJourneyAuditEventsWithField(t *testing.T, project, event, field, value string) int {
	t.Helper()
	count := 0
	for _, record := range journeyAuditRecords(t, project) {
		if record.Event == event && record.Fields[field] == value {
			count++
		}
	}
	return count
}

func journeyAuditRecords(t *testing.T, project string) []audit.AuditRecord {
	t.Helper()
	identity, err := recordlock.NewIdentity(project, "team", "build")
	if err != nil {
		t.Fatalf("NewIdentity(): %v", err)
	}
	projectRoot, err := os.OpenRoot(project)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	defer projectRoot.Close()
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "team", "intents", "build")))
	if err != nil {
		t.Fatalf("OpenRoot(record): %v", err)
	}
	defer recordRoot.Close()
	var records []audit.AuditRecord
	err = recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		readRecords, readErr := audit.ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
		if readErr != nil {
			return readErr
		}
		records = readRecords
		return nil
	})
	if err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	return records
}

func runIntentCaptureContext(t *testing.T, binaryPath, project string) {
	t.Helper()
	directive := runDeliveryBinary(t, binaryPath, project, "next")
	for directive.kind == "load-steering" {
		if directive.token == "" {
			t.Fatalf("load-steering directive has no continuation token: %#v", directive)
		}
		directive = runDeliveryBinary(t, binaryPath, project, "continue", directive.token)
	}
	if directive.kind != "run-stage" {
		t.Fatalf("intent-capture directive = %#v, want run-stage", directive)
	}
	args := []string{"read-context", "--project-dir", project}
	chunks := 0
	for {
		result := runIntentCommand(t, binaryPath, project, nil, args...)
		if result.exitCode != 0 {
			t.Fatalf("read-context exit = %d stderr=%q", result.exitCode, result.stderr.String())
		}
		var wire struct {
			Complete          bool   `json:"complete"`
			ReadContinueToken string `json:"read_continue_token"`
		}
		decodeIntentWire(t, result, &wire)
		chunks++
		if wire.Complete {
			break
		}
		if wire.ReadContinueToken == "" {
			t.Fatalf("read-context chunk has no continuation token: %q", result.stdout.String())
		}
		args = []string{"read-context", "continue", wire.ReadContinueToken, "--project-dir", project}
	}
	if chunks == 0 {
		t.Fatal("read-context returned no chunks")
	}
}

func writeIntentCaptureFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", name, err)
	}
}
