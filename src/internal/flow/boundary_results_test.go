package flow

import (
	"encoding/json"
	"testing"
)

func resultPairFixture(t *testing.T) (Store, State, string, string) {
	t.Helper()
	s, st := boundaryFixture(t)
	a := flowGit(t, s.Root, "rev-parse", "HEAD")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "second")
	b := flowGit(t, s.Root, "rev-parse", "HEAD")
	fixtureExecutionStage(t, s, &st, "tdd")
	st.Config.Units = []Unit{{ID: "a", Tests: []string{"go test"}, ResultCommit: a, IntegratedCommit: a}, {ID: "b", Tests: []string{"go test"}, ResultCommit: b, IntegratedCommit: b}}
	st.Config.TestResults = []string{"aidlc/evidence/tdd.json"}
	return s, st, a, b
}
func writeResultRuns(t *testing.T, s Store, name, stage string, runs ...resultRun) {
	t.Helper()
	raw, err := json.Marshal(resultDocument{StepID: fixtureStepID(stage), Stage: stage, Runs: runs})
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, name, string(raw))
}
func successfulRun(t *testing.T, s Store, command, commit, output string) resultRun {
	t.Helper()
	boundaryFile(t, s, output, "actual fixture output")
	zero := 0
	return resultRun{Command: command, Commit: commit, ExitCode: &zero, OutputPath: output}
}
func TestEndSensorSharedCommandRequiresEachUnitResult(t *testing.T) {
	s, st, a, b := resultPairFixture(t)
	one := successfulRun(t, s, "go test", a, "aidlc/evidence/a.txt")
	two := successfulRun(t, s, "go test", b, "aidlc/evidence/b.txt")
	writeResultRuns(t, s, st.Config.TestResults[0], "tdd", one)
	c := boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) == 0 {
		t.Fatal("one Unit result satisfied both Units")
	}
	writeResultRuns(t, s, st.Config.TestResults[0], "tdd", one, two)
	c = boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) != 0 {
		t.Fatalf("both Unit results: %v", c.failures)
	}
}
func TestEndSensorIntegrationRequiresFinalHEAD(t *testing.T) {
	s, st, a, b := resultPairFixture(t)
	fixtureExecutionStage(t, s, &st, "integration")
	st.Config.Units[0].Tests = []string{"test a"}
	st.Config.Units[1].Tests = []string{"test b"}
	one := successfulRun(t, s, "test a", a, "aidlc/evidence/a.txt")
	two := successfulRun(t, s, "test b", b, "aidlc/evidence/b.txt")
	writeResultRuns(t, s, st.Config.TestResults[0], "integration", one, two)
	c := boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) == 0 {
		t.Fatal("pre-final integration result accepted")
	}
	one.Commit = b
	writeResultRuns(t, s, st.Config.TestResults[0], "integration", one, two)
	c = boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) != 0 {
		t.Fatalf("final HEAD tests: %v", c.failures)
	}
}
func TestEndSensorOtherStageDoesNotEnterAcceptedOutputs(t *testing.T) {
	s, st, a, b := resultPairFixture(t)
	one := successfulRun(t, s, "go test", a, "aidlc/evidence/a.txt")
	two := successfulRun(t, s, "go test", b, "aidlc/evidence/b.txt")
	writeResultRuns(t, s, st.Config.TestResults[0], "tdd", one, two)
	c := boundaryCollector{store: s}
	c.results(st)
	baseline := c.gate(st.Stage).Target
	other := "aidlc/evidence/integration.json"
	writeResultRuns(t, s, other, "integration", two)
	st.Config.TestResults = append(st.Config.TestResults, other)
	c = boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) != 0 {
		t.Fatal(c.failures)
	}
	for _, f := range c.files {
		if f.Path == other {
			t.Fatal("other-stage JSON entered accepted outputs")
		}
	}
	if c.gate(st.Stage).Target != baseline {
		t.Fatal("other-stage JSON changed current results digest")
	}
	boundaryFile(t, s, other, `{"step_id":"s05","stage":"integration","runs":[{"command":"go test","commit":"bad"}]}`)
	c = boundaryCollector{store: s}
	c.results(st)
	if len(c.failures) == 0 {
		t.Fatal("malformed other-stage run skipped strict validation")
	}
}
