//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func buildMinimalBinary(t *testing.T) string {
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
func runMinimalCLI(t *testing.T, binary, root string, input []byte, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("aidlc %v: %v: %s", args, err, output)
	}
	return output
}
func runMinimalProcess(t *testing.T, root, name string, args ...string) []byte {
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
func writeMinimalFixture(t *testing.T, path, body string) {
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
func TestFlowJourney(t *testing.T)     { runBoundaryJourney(t) }
func TestBoundaryJourney(t *testing.T) { runBoundaryJourney(t) }
func runBoundaryJourney(t *testing.T) {
	t.Helper()
	binary := buildMinimalBinary(t)
	root := t.TempDir()
	runMinimalProcess(t, root, "git", "init", "-q")
	runMinimalProcess(t, root, "git", "-c", "user.name=Flow", "-c", "user.email=flow@example.invalid", "commit", "--allow-empty", "-qm", "base")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	var st flow.State
	read := func(raw []byte) {
		t.Helper()
		if err := json.Unmarshal(raw, &st); err != nil {
			t.Fatalf("state %s: %v", raw, err)
		}
	}
	read(runMinimalCLI(t, binary, root, nil, "intent", "create", "Fresh journey", "--space", "default"))
	call := func(action string, args ...string) {
		t.Helper()
		base := []string{"intent", action, st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10)}
		read(runMinimalCLI(t, binary, root, nil, append(base, args...)...))
	}
	writeRequest := func(name string, value any) string {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "aidlc/.runtime", name)
		writeMinimalFixture(t, file, string(raw))
		return file
	}
	knowledge := "aidlc/spaces/default/knowledge/knowledge/current.md"
	writeMinimalFixture(t, filepath.Join(root, knowledge), "---\ntype: Design\ntitle: Addition\ndescription: Adds two integers\n---\nAdd returns the sum.\n")
	head := string(bytes.TrimSpace(runMinimalProcess(t, root, "git", "rev-parse", "HEAD")))
	config := flow.Config{NoMaterialsReason: "fresh project", Objective: "Addition", Scope: []string{"add.go"}, Acceptance: []string{"Add(2,3)=5"}, CodeRevision: head, ADR: flow.ADR{Reason: "No architectural decision"}, Artifacts: []flow.Artifact{{Path: knowledge, Kind: "Knowledge", Stage: "discovery"}}}
	call("configure", "--file", writeRequest("config.json", config))
	review := func(status string) {
		t.Helper()
		reviewRoot := filepath.Join(t.TempDir(), "review")
		runMinimalProcess(t, root, "git", "worktree", "add", "--detach", reviewRoot, "HEAD")
		files := runMinimalProcess(t, root, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
		for _, name := range strings.Split(string(files), "\x00") {
			if name == "" || strings.HasPrefix(name, "aidlc/") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			writeMinimalFixture(t, filepath.Join(reviewRoot, name), string(raw))
		}
		call("review", "--file", writeRequest("review.json", flow.ReviewRequest{Action: "assign", CoordinatorSession: "c", Session: "r", Root: reviewRoot}))
		var gate flow.Gate
		if err := json.Unmarshal(runMinimalCLI(t, binary, root, nil, "intent", "check", st.ID, "--space", "default"), &gate); err != nil {
			t.Fatal(err)
		}
		call("review", "--file", writeRequest("review.json", flow.ReviewRequest{Action: "accept", Session: "r", Root: reviewRoot, Target: gate.Target, Status: status, Summary: "Fixture review for CLI integration; not actual AI evidence"}))
	}
	boundaryFixtureDocument(t, root, st.ID, "Requirements")
	call("begin")
	review("fail")
	cmd := exec.Command(binary, "intent", "advance", st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10))
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Fatal("failed review advanced")
	}
	review("pass")
	call("advance")
	boundaryFixtureDocument(t, root, st.ID, "ImplementationPlan")
	call("begin")
	config.Plan = "Implement Add using a failing example then verification"
	config.Tests = []string{"go test -run ^TestAdd$"}
	call("configure", "--file", writeRequest("config.json", config))
	review("pass")
	call("advance")
	call("begin")
	writeMinimalFixture(t, filepath.Join(root, "go.mod"), "module example.invalid/add\n\ngo 1.26\n")
	writeMinimalFixture(t, filepath.Join(root, "add.go"), "package add\nfunc Add(a,b int)int{return 0}\n")
	writeMinimalFixture(t, filepath.Join(root, "add_test.go"), "package add\nimport \"testing\"\nfunc TestAdd(t *testing.T){if Add(2,3)!=5{t.Fatal(\"wrong sum\")}}\n")
	red := exec.Command("go", "test", "-count=1", "-run", "^TestAdd$")
	red.Dir = root
	if raw, err := red.CombinedOutput(); err == nil || !bytes.Contains(raw, []byte("wrong sum")) {
		t.Fatalf("not assertion RED: %s %v", raw, err)
	}
	writeMinimalFixture(t, filepath.Join(root, "add.go"), "package add\nfunc Add(a,b int)int{return a+b}\n")
	green := runMinimalProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
	writeMinimalFixture(t, filepath.Join(root, "results.txt"), string(green))
	runMinimalProcess(t, root, "git", "add", "go.mod", "add.go", "add_test.go", "results.txt")
	runMinimalProcess(t, root, "git", "-c", "user.name=Flow", "-c", "user.email=flow@example.invalid", "commit", "-qm", "verified addition")
	head = string(bytes.TrimSpace(runMinimalProcess(t, root, "git", "rev-parse", "HEAD")))
	config.CodeRevision = head
	config.DirectCommit = head
	config.Artifacts = append(config.Artifacts, flow.Artifact{Path: "results.txt", Kind: "test", Stage: "tdd"})
	config.TestResults = []string{boundaryFixtureResults(t, root, "tdd", head, config.Tests, green)}
	call("configure", "--file", writeRequest("config.json", config))
	review("pass")
	call("advance")
	call("begin")
	call("pause", "--reason", "session ended")
	call("resume", "--reason", "new session inspected persisted state")
	boundaryFixtureDocument(t, root, st.ID, "CurrentAnalysis")
	boundaryFixtureDocument(t, root, st.ID, "Architecture")
	config.FeatureKnowledge = []string{boundaryFixtureDocument(t, root, st.ID, "Knowledge")}
	integrationOutput := runMinimalProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
	config.TestResults = append(config.TestResults, boundaryFixtureResults(t, root, "integration", head, config.Tests, integrationOutput))
	call("configure", "--file", writeRequest("config.json", config))
	runMinimalCLI(t, binary, root, nil, "session", "bind", st.ID, "--space", "default", "--session", "second")
	review("pass")
	call("advance")
	if st.Status != "completed" {
		t.Fatalf("not completed %+v", st)
	}
}
