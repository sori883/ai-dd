//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func buildAIDLCBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "aidlc")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	return binary
}
func runAIDLCCLI(t *testing.T, binary, root string, input []byte, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	productPath := os.Getenv("AIDLC_TEST_PRODUCT_PATH")
	if productPath != "" {
		cmd.Env = gitIndependentEnvironment(os.Environ(), productPath)
	}
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aidlc %v: %v: %s", args, err, output)
	}
	return output
}
func runFixtureProcess(t *testing.T, root, name string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, output)
	}
	return output
}
func writeAIDLCFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// This deterministic executable journey covers failure/recovery boundaries;
// actual asynchronous Codex transport pairing is separately live-probed.
func TestFlowJourney(t *testing.T)      { runBoundaryJourney(t) }
func TestBoundaryJourney(t *testing.T)  { runBoundaryJourney(t) }
func TestProcedureJourney(t *testing.T) { runBoundaryJourney(t) }
func runBoundaryJourney(t *testing.T) {
	runGitIndependentBoundaryJourney(t, buildAIDLCBinary(t), t.TempDir(), false)
}
func runGitIndependentBoundaryJourney(t *testing.T, binary, root string, units bool) {
	t.Helper()
	runAIDLCCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	var st flow.State
	read := func(raw []byte) {
		t.Helper()
		if err := json.Unmarshal(raw, &st); err != nil {
			t.Fatalf("state %s: %v", raw, err)
		}
	}
	read(runAIDLCCLI(t, binary, root, nil, "intent", "create", "Fresh journey", "--space", "default"))
	procedure := func() {
		t.Helper()
		var view flow.ProcedureView
		raw := runAIDLCCLI(t, binary, root, nil, "intent", "procedure", st.ID, "--space", "default")
		if json.Unmarshal(raw, &view) != nil || view.Stage != st.Stage || view.DefinitionHash != st.DefinitionHash || view.Procedure.Text == "" {
			t.Fatalf("procedure mismatch: %s", raw)
		}
	}
	procedure()
	call := func(action string, args ...string) {
		t.Helper()
		base := []string{"intent", action, st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10)}
		read(runAIDLCCLI(t, binary, root, nil, append(base, args...)...))
		procedure()
	}
	writeRequest := func(name string, value any) string {
		t.Helper()
		raw, err := marshalFlowRequest(value)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "aidlc/.runtime", name)
		writeAIDLCFixture(t, file, string(raw))
		return file
	}
	knowledge := "aidlc/spaces/default/knowledge/codekb/current.md"
	writeAIDLCFixture(t, filepath.Join(root, knowledge), "---\ntype: Design\ntitle: Addition\ndescription: Adds two integers\n---\nAdd returns the sum.\n")
	config := flow.Config{NoMaterialsReason: "fresh project", Objective: "Addition", Scope: []string{"add.go"}, Acceptance: []string{"Add(2,3)=5"}, VerificationPaths: []string{"add.go", "add_test.go", "go.mod"}, ADR: flow.ADR{Reason: "No architectural decision"}, Artifacts: []flow.Artifact{{Path: knowledge, Kind: "Knowledge", Stage: "discovery"}}}
	review := func(status string) {
		t.Helper()
		reviewRoot := root
		call("review", "--file", writeRequest("review.json", flow.ReviewRequest{Action: "assign", CoordinatorSession: "c", Session: "r", Root: reviewRoot}))
		var gate flow.Gate
		if err := json.Unmarshal(runAIDLCCLI(t, binary, root, nil, "intent", "check", st.ID, "--space", "default"), &gate); err != nil {
			t.Fatal(err)
		}
		call("review", "--file", writeRequest("review.json", flow.ReviewRequest{Action: "accept", Session: "r", Root: reviewRoot, Target: gate.Target, Status: status, Summary: "Fixture review for CLI integration; not actual AI evidence"}))
	}
	f := operationsFixture{t: t, binary: binary, root: root}
	call("begin")
	review("pass")
	st = f.finish(st)
	call("configure", "--file", writeRequest("config.json", config))
	st = f.selectPlan(st)
	boundaryFixtureDocument(t, root, st.ID, "Requirements")
	call("begin")
	review("fail")
	cmd := exec.Command(binary, "intent", "advance", st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10))
	cmd.Dir = root
	if productPath := os.Getenv("AIDLC_TEST_PRODUCT_PATH"); productPath != "" {
		cmd.Env = gitIndependentEnvironment(os.Environ(), productPath)
	}
	if err := cmd.Run(); err == nil {
		t.Fatal("failed review advanced")
	}
	review("pass")
	st = f.finish(st)
	boundaryFixtureDocument(t, root, st.ID, "ImplementationPlan")
	call("begin")
	config.Plan = "Implement Add using a failing example then verification"
	config.Tests = []string{"go test -count=1 -run ^TestAdd$"}
	call("configure", "--file", writeRequest("config.json", config))
	review("pass")
	st = f.finish(st)
	call("reopen", "--step", "s03", "--reason", "recheck implementation plan")
	st = f.approve(st, true)
	log, err := os.ReadFile(filepath.Join(root, "aidlc/spaces/default/knowledge/log", st.ID+"-work-log.md"))
	if err != nil || !bytes.Contains(log, []byte("recheck implementation plan")) {
		t.Fatal("missing reopen log", err)
	}
	if _, err := okfmemory.Parse(log); err != nil {
		t.Fatal("reopen log is not OKF", err)
	}
	found := runAIDLCCLI(t, binary, root, nil, "memory", "search", "work-log", "--space", "default", "--intent-id", st.ID)
	var records []map[string]string
	if err := json.Unmarshal(found, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0]["concept_id"] != "log/"+st.ID+"-work-log" {
		t.Fatalf("reopen log search: %s", found)
	}
	shown := runAIDLCCLI(t, binary, root, nil, "memory", "show", records[0]["concept_id"], "--space", "default")
	var record map[string]string
	if err := json.Unmarshal(shown, &record); err != nil {
		t.Fatal(err)
	}
	if record["content"] != string(log) {
		t.Fatalf("reopen log show: %s", shown)
	}
	call("begin")
	review("pass")
	st = f.finish(st)
	call("begin")
	writeAIDLCFixture(t, filepath.Join(root, "go.mod"), "module example.invalid/add\n\ngo 1.26\n")
	writeAIDLCFixture(t, filepath.Join(root, "add.go"), "package add\nfunc Add(a,b int)int{return 0}\n")
	writeAIDLCFixture(t, filepath.Join(root, "add_test.go"), "package add\nimport \"testing\"\nfunc TestAdd(t *testing.T){if Add(2,3)!=5{t.Fatal(\"wrong sum\")}}\n")
	red := exec.Command("go", "test", "-count=1", "-run", "^TestAdd$")
	red.Dir = root
	if raw, err := red.CombinedOutput(); err == nil || !bytes.Contains(raw, []byte("wrong sum")) {
		t.Fatalf("not assertion RED: %s %v", raw, err)
	}
	writeAIDLCFixture(t, filepath.Join(root, "add.go"), "package add\nfunc Add(a,b int)int{return a+b}\n")
	green := runFixtureProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
	writeAIDLCFixture(t, filepath.Join(root, "aidlc/evidence/results.txt"), string(green))
	config.Artifacts = append(config.Artifacts, flow.Artifact{Path: "aidlc/evidence/results.txt", Kind: "test", Stage: "tdd"})
	if units {
		config = runGitIndependentUnits(t, binary, root, &st, config)
		green = runFixtureProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
	}
	call("configure", "--file", writeRequest("config.json", config))
	config.TestResults = append(config.TestResults, boundaryFixtureResults(t, root, st.CurrentStepID, "tdd", "", config.Tests, green))
	call("configure", "--file", writeRequest("config.json", config))
	review("pass")
	st = f.finish(st)
	call("begin")
	call("pause", "--reason", "session ended")
	call("resume", "--reason", "new session inspected persisted state")
	boundaryFixtureDocument(t, root, st.ID, "CurrentAnalysis")
	boundaryFixtureDocument(t, root, st.ID, "Architecture")
	name := boundaryFixtureDocument(t, root, st.ID, "Knowledge")
	content, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	document, err := okfmemory.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	title, description := document.String("title"), document.String("description")
	call("documents", "--file", writeRequest("documents.json", flow.IntentDocuments{Inputs: []flow.DocumentDeclaration{}, Outputs: []flow.DocumentDeclaration{{StepID: st.CurrentStepID, Stage: "integration", Path: name, Metadata: okfmemory.DocumentMatch{Type: "Knowledge", Title: &title, Description: &description}}}}))
	config.Units = nil
	call("configure", "--file", writeRequest("config.json", config))
	integrationOutput := runFixtureProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
	config.TestResults = append(config.TestResults, boundaryFixtureResults(t, root, st.CurrentStepID, "integration", "", config.Tests, integrationOutput))
	call("configure", "--file", writeRequest("config.json", config))
	runAIDLCCLI(t, binary, root, nil, "session", "bind", st.ID, "--space", "default", "--session", "second")
	review("pass")
	st = f.finish(st)
	if st.Status != "completed" {
		t.Fatalf("not completed %+v", st)
	}
}

func marshalFlowRequest(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if _, ok := value.(flow.Config); !ok {
		return raw, nil
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	delete(fields, "document_inputs")
	delete(fields, "document_outputs")
	return json.Marshal(fields)
}
