package flow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	core "github.com/sori883/ai-dd/src/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boundaryFixture(t *testing.T) (Store, State) {
	t.Helper()
	s := flowStore(t)
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	st, err := createExecutionFixture(t, s, "Boundary")
	if err != nil {
		t.Fatal(err)
	}
	fixtureExecutionStage(t, s, &st, "discovery")
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	raw, err := core.Files.ReadFile("knowledge/rules/rule.md")
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, "aidlc/spaces/default/knowledge/rules/rule.md", string(raw))
	return s, st
}
func boundaryFile(t *testing.T, s Store, name, body string) {
	t.Helper()
	p := filepath.Join(s.Root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func boundaryDoc(t *testing.T, s Store, st State, kind string) string {
	t.Helper()
	rel := map[string]string{"Requirements": "design/" + st.ID + "/requirements.md", "ImplementationPlan": "design/" + st.ID + "/implementation-plan.md", "CurrentAnalysis": "codekb/current-analysis.md", "Architecture": "codekb/architecture.md", "Knowledge": "codekb/feature.md"}[kind]
	sections := map[string][]string{"Requirements": {"目的", "範囲", "要件", "受入条件", "未確定事項"}, "ImplementationPlan": {"変更箇所", "実装手順", "検証方法"}, "CurrentAnalysis": {"現状", "構成・動作", "根拠", "未確認事項"}, "Architecture": {"構成図", "構成要素", "データフロー"}, "Knowledge": {"機能", "利用手順", "制約"}}[kind]
	body := "---\ntype: " + kind + "\ntitle: Document\ndescription: Contract\nintent_id: " + st.ID + "\n---\n"
	for _, heading := range sections {
		body += "\n## " + heading + "\nConcrete content.\n"
		if heading == "構成図" {
			body += "```mermaid\ngraph LR\n A-->B\n```\n"
		}
	}
	name := "aidlc/spaces/default/knowledge/" + rel
	boundaryFile(t, s, name, body)
	return name
}
func boundaryVersion(t *testing.T, s Store, name string) FileVersion {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.Root, name))
	if err != nil {
		t.Fatal(err)
	}
	return FileVersion{Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
}
func TestStartSensorInitialAndReadOnly(t *testing.T) {
	s, st := boundaryFixture(t)
	before, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	g, err := s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "pass" || len(g.Target) != 64 {
		t.Fatalf("first discovery: %+v %v", g, err)
	}
	after, _ := os.ReadFile(filepath.Join(s.Root, s.path(st.ID)))
	if string(before) != string(after) {
		t.Fatal("check wrote state")
	}
	if _, err = s.CheckBoundary(st.ID, Boundary("other")); err == nil {
		t.Fatal("unknown boundary accepted")
	}
	boundaryFile(t, s, "aidlc/spaces/default/knowledge/codekb/current-analysis.md", "broken")
	g, err = s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "fail" {
		t.Fatalf("existing malformed shared: %+v %v", g, err)
	}
}
func TestStartSensorAcceptedInputs(t *testing.T) {
	s, st := boundaryFixture(t)
	name := boundaryDoc(t, s, st, "Requirements")
	fixtureExecutionStage(t, s, &st, "planning")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "fail" {
		t.Fatalf("unaccepted requirements: %+v %v", g, err)
	}
	st.Accepted = map[string]StageAcceptance{"s02": {StepID: "s02", Stage: "discovery", ReviewTarget: strings.Repeat("a", 64), Outputs: []FileVersion{boundaryVersion(t, s, name)}}}
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err = s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "pass" {
		t.Fatalf("accepted: %+v %v", g, err)
	}
	boundaryFile(t, s, name, "changed")
	g, err = s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "fail" {
		t.Fatalf("stale accepted: %+v %v", g, err)
	}
}

func TestEndSensorDocuments(t *testing.T) {
	s, st := boundaryFixture(t)
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}}
	st.Entry = &StageEntry{StepID: "s02", Stage: "discovery"}
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.CheckBoundary(st.ID, BoundaryEnd)
	if err != nil || g.Status != "fail" {
		t.Fatalf("missing requirements: %+v %v", g, err)
	}
	name := boundaryDoc(t, s, st, "Requirements")
	g, err = s.CheckBoundary(st.ID, BoundaryEnd)
	if err != nil || g.Status != "pass" {
		t.Fatalf("valid requirements: %+v %v", g, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, name))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(raw), st.ID, strings.Repeat("a", 32), 1), strings.Replace(string(raw), "type: Requirements", "type: Knowledge", 1), strings.Replace(string(raw), "## 範囲\nConcrete content.", "## 範囲", 1)} {
		boundaryFile(t, s, name, bad)
		g, err = s.CheckBoundary(st.ID, BoundaryEnd)
		if err != nil || g.Status != "fail" {
			t.Fatalf("bad requirements accepted: %+v %v", g, err)
		}
	}
}
func TestEndSensorMaterials(t *testing.T) {
	s, st := boundaryFixture(t)
	boundaryDoc(t, s, st, "Requirements")
	st.Entry = &StageEntry{StepID: "s02", Stage: "discovery"}
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, MaterialSources: []string{"inputs"}, ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}}
	boundaryFile(t, s, "inputs/source.txt", "source")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("unselected analysis required: %+v %v", g, err)
	}
	boundaryDoc(t, s, st, "CurrentAnalysis")
	boundaryDoc(t, s, st, "Architecture")
	g, err = s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("materials: %+v %v", g, err)
	}
	old := g.Target
	boundaryFile(t, s, "inputs/added.txt", "new")
	g, err = s.Check(st.ID)
	if err != nil || g.Status != "pass" || g.Target == old {
		t.Fatalf("added source missed: %+v %v", g, err)
	}
	boundaryFile(t, s, "inputs/binary", string([]byte{255}))
	g, err = s.Check(st.ID)
	if err != nil || g.Status != "fail" {
		t.Fatalf("non UTF8: %+v %v", g, err)
	}
}
func TestEndSensorResults(t *testing.T) {
	s, st := boundaryFixture(t)
	req := boundaryDoc(t, s, st, "Requirements")
	plan := boundaryDoc(t, s, st, "ImplementationPlan")
	head := verificationTestSHA(t, s.Root, []string{"."})
	fixtureExecutionStage(t, s, &st, "tdd")
	st.Entry = &StageEntry{StepID: "s04", Stage: "tdd"}
	st.Accepted = map[string]StageAcceptance{"s02": {StepID: "s02", Stage: "discovery", ReviewTarget: strings.Repeat("a", 64), Outputs: []FileVersion{boundaryVersion(t, s, req)}}, "s03": {StepID: "s03", Stage: "planning", ReviewTarget: strings.Repeat("b", 64), Outputs: []FileVersion{boundaryVersion(t, s, plan)}}}
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}, Plan: "Implement", Tests: []string{"go test ./target"}, TestResults: []string{"aidlc/evidence/results.json"}}
	boundaryFile(t, s, "aidlc/evidence/output.txt", "ok target")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	good := fmt.Sprintf(`{"step_id":"s04","stage":"tdd","verification_scope":"intent","verification_sha256":%q,"runs":[{"command":"go test ./target","exit_code":0,"output_path":"aidlc/evidence/output.txt"}]}`, head)
	for _, raw := range []string{good, strings.Replace(good, `"exit_code":0,`, "", 1), strings.Replace(good, `"exit_code":0`, `"exit_code":1`, 1), strings.Replace(good, head, strings.Repeat("a", 40), 1), strings.Replace(good, "go test ./target", "echo no test", 1), strings.Replace(good, `"stage":"tdd"`, `"stage":"tdd","extra":1`, 1)} {
		boundaryFile(t, s, "aidlc/evidence/results.json", raw)
		g, err := s.Check(st.ID)
		want := "fail"
		if raw == good {
			want = "pass"
		}
		if err != nil || g.Status != want {
			t.Fatalf("result %s: %+v %v", raw, g, err)
		}
	}
}

func TestStartSensorMaterialVersions(t *testing.T) {
	s, st := boundaryFixture(t)
	boundaryFile(t, s, "inputs/source.txt", "original")
	st.Config.MaterialSources = []string{"inputs"}
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "pass" {
		t.Fatalf("sources: %+v %v", g, err)
	}
	if err = os.Mkdir(filepath.Join(s.Root, "inputs/empty"), 0700); err != nil {
		t.Fatal(err)
	}
	next, err := s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || next.Target == g.Target {
		t.Fatalf("directory membership ignored: %+v %v", next, err)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil || st.Entry == nil || len(st.Entry.Sources) == 0 {
		t.Fatalf("sources not captured: %+v %v", st, err)
	}
	if err = os.Symlink("source.txt", filepath.Join(s.Root, "inputs/link")); err != nil {
		t.Fatal(err)
	}
	g, err = s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "fail" {
		t.Fatalf("symlink material: %+v %v", g, err)
	}
}

// prepareBoundaryStage supplies prior accepted documents to isolated legacy tests.
// End-result assertions remain in their callers; this does not fabricate a review pass.
func prepareBoundaryStage(t *testing.T, s Store, st *State) {
	t.Helper()
	req := boundaryDoc(t, s, *st, "Requirements")
	plan := boundaryDoc(t, s, *st, "ImplementationPlan")
	fixtureExecutionStage(t, s, st, st.Stage)
	st.Entry = &StageEntry{StepID: st.CurrentStepID, Stage: st.Stage}
	if st.Accepted == nil {
		st.Accepted = map[string]StageAcceptance{}
	}
	if st.Stage != "discovery" {
		st.Accepted["s02"] = StageAcceptance{StepID: "s02", Stage: "discovery", ReviewTarget: strings.Repeat("a", 64), Outputs: []FileVersion{boundaryVersion(t, s, req)}}
	}
	if st.Stage == "tdd" || st.Stage == "integration" {
		st.Accepted["s03"] = StageAcceptance{StepID: "s03", Stage: "planning", ReviewTarget: strings.Repeat("b", 64), Outputs: []FileVersion{boundaryVersion(t, s, plan)}}
	}
	if err := s.persist(*st); err != nil {
		t.Fatal(err)
	}
}
func prepareBoundaryResults(t *testing.T, s Store, st *State) {
	t.Helper()
	name := "aidlc/evidence/" + st.Stage + ".json"
	output := "aidlc/evidence/" + st.Stage + ".txt"
	boundaryFile(t, s, output, "observed fixture output")
	var runs []resultRun
	zero := 0
	for _, command := range st.Config.Tests {
		runs = append(runs, resultRun{Command: command, ExitCode: &zero, OutputPath: output})
	}
	raw, err := json.Marshal(resultDocument{StepID: st.CurrentStepID, Stage: st.Stage, VerificationScope: "intent", VerificationSHA256: verificationTestSHA(t, s.Root, st.Config.VerificationPaths), Runs: runs})
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, name, string(raw))
	st.Config.TestResults = append(st.Config.TestResults, name)
}

func TestEndSensorIntegrationDocuments(t *testing.T) {
	s, st := boundaryFixture(t)
	prepareBoundaryStage(t, s, &st)
	fixtureExecutionStage(t, s, &st, "integration")
	prepareBoundaryStage(t, s, &st)
	st.Accepted["s04"] = StageAcceptance{StepID: "s04", Stage: "tdd", ReviewTarget: strings.Repeat("c", 64)}
	_ = flowGit(t, s.Root, "rev-parse", "HEAD")
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}, Plan: "Implement", Tests: []string{"go test"}}
	prepareBoundaryResults(t, s, &st)
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err := s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("undeclared integration docs required: %+v %v", g, err)
	}
	analysis := boundaryDoc(t, s, st, "CurrentAnalysis")
	diagram := boundaryDoc(t, s, st, "Architecture")
	st.Config.DocumentOutputs = []DocumentDeclaration{boundaryDeclaration(t, s, st, "Knowledge")}
	raw, err := os.ReadFile(filepath.Join(s.Root, analysis))
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, analysis, strings.Replace(string(raw), st.ID, strings.Repeat("b", 32), 1))
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	g, err = s.Check(st.ID)
	if err != nil || g.Status != "pass" {
		t.Fatalf("shared other Intent rejected: %+v %v", g, err)
	}
	raw, err = os.ReadFile(filepath.Join(s.Root, diagram))
	if err != nil {
		t.Fatal(err)
	}
	boundaryFile(t, s, diagram, strings.Replace(string(raw), "```mermaid\ngraph LR\n A-->B\n```", "", 1))
	g, err = s.Check(st.ID)
	if err != nil || g.Status != "fail" {
		t.Fatalf("missing diagram: %+v %v", g, err)
	}
}

func TestEndSensorUnitCommandPair(t *testing.T) { TestVerificationResults(t) }

func TestStartSensorMissingViaSymlinkIsNotAbsent(t *testing.T) {
	s, st := boundaryFixture(t)
	if err := os.Symlink(filepath.Join(s.Root, "missing"), filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb")); err != nil {
		t.Fatal(err)
	}
	g, err := s.CheckBoundary(st.ID, BoundaryStart)
	if err != nil || g.Status != "fail" {
		t.Fatalf("symlink treated as missing optional document: %+v %v", g, err)
	}
}
