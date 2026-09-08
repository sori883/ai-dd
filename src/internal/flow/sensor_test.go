package flow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func flowGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Flow", "GIT_AUTHOR_EMAIL=flow@example.invalid", "GIT_COMMITTER_NAME=Flow", "GIT_COMMITTER_EMAIL=flow@example.invalid")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, raw, err)
	}
	return strings.TrimSpace(string(raw))
}
func sensorFixture(t *testing.T) (Store, State) {
	t.Helper()
	s := flowStore(t)
	flowGit(t, s.Root, "init", "-q")
	flowGit(t, s.Root, "commit", "--allow-empty", "-qm", "base")
	st, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	name := "aidlc/spaces/default/knowledge/current.md"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(s.Root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	raw := "---\ntitle: Current\ndescription: Current behavior\ntype: Knowledge\n---\n# Current\nA concrete behavior.\n"
	if err := os.WriteFile(filepath.Join(s.Root, name), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, ADR: ADR{Reason: "No architectural decision"}, Artifacts: []Artifact{{Path: name, Kind: "Knowledge", Stage: "discovery"}}, CodeRevision: flowGit(t, s.Root, "rev-parse", "HEAD")}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}
func TestFlowSensorDiscovery(t *testing.T) {
	s, st := sensorFixture(t)
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" || gate.Target == "" {
		t.Fatalf("valid discovery: %+v %v", gate, err)
	}
	original := gate.Target
	st.Config.Unknowns = []string{"Blocking design choice"}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "fail" {
		t.Fatalf("unresolved blocker: %+v %v", gate, err)
	}
	st.Config.Unknowns = nil
	st.Config.ADR = ADR{Required: true}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "fail" {
		t.Fatalf("missing ADR: %+v %v", gate, err)
	}
	st.Config.ADR = ADR{Reason: "No architectural decision"}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Target != original {
		t.Fatalf("revision alone changed target: %+v %v", gate, err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, st.Config.Artifacts[0].Path), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "fail" || gate.Target == original {
		t.Fatalf("changed artifact: %+v %v", gate, err)
	}
}
func TestFlowSensorUnitGraph(t *testing.T) {
	for _, units := range [][]Unit{{{ID: "a", DependsOn: []string{"missing"}}}, {{ID: "a", DependsOn: []string{"b"}}, {ID: "b", DependsOn: []string{"a"}}}, {{ID: "a"}, {ID: "a"}}} {
		s, st := sensorFixture(t)
		st.Stage = "planning"
		st.Config.Plan = "A plan"
		st.Config.Units = units
		st, err := s.Save(st, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
		gate, err := s.Check(st.ID)
		if err != nil || gate.Status != "fail" {
			t.Fatalf("bad graph: %+v %v", gate, err)
		}
	}
}
func TestFlowSensorDirectImplementation(t *testing.T) {
	s, st := sensorFixture(t)
	st.Stage = "planning"
	st.Config.Plan = "Direct implementation"
	st, err := s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "fail" {
		t.Fatalf("empty direct verification accepted: %+v %v", gate, err)
	}
	st.Config.Tests = []string{"go test ./..."}
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("direct plan rejected: %+v %v", gate, err)
	}
	st.Stage = "tdd"
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "fail" {
		t.Fatalf("missing direct result accepted: %+v %v", gate, err)
	}
	file := "test-results.txt"
	if err := os.WriteFile(filepath.Join(s.Root, file), []byte("PASS: acceptance verified"), 0600); err != nil {
		t.Fatal(err)
	}
	st.Config.Artifacts = append(st.Config.Artifacts, Artifact{Path: file, Kind: "test", Stage: "tdd"})
	st.Config.DirectCommit = st.Config.CodeRevision
	st, err = s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err = s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("direct result rejected: %+v %v", gate, err)
	}
}
func TestFlowSensorKnowledgePreservesOKFType(t *testing.T) {
	s, st := sensorFixture(t)
	name := filepath.Join(s.Root, st.Config.Artifacts[0].Path)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "type: Knowledge", "type: Design", 1))
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("valid typed Knowledge rejected: %+v %v", gate, err)
	}
}
func TestFlowSensorRequiredADR(t *testing.T) {
	s, st := sensorFixture(t)
	name := "aidlc/spaces/default/knowledge/ADR/store.md"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(s.Root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Root, name), []byte("---\ntype: ADR\ntitle: Store\ndescription: Why atomic replacement\n---\nAtomic replacement avoids partial state.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st.Config.ADR = ADR{Required: true, Refs: []string{name}}
	st.Config.Artifacts = append(st.Config.Artifacts, Artifact{Path: name, Kind: "ADR", Stage: "discovery"})
	st, err := s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "pass" {
		t.Fatalf("valid ADR %+v %v", gate, err)
	}
}
func TestFlowSensorRejectsInventedIntegratedCommit(t *testing.T) {
	s, st := sensorFixture(t)
	st.Stage = "tdd"
	st.Config.Plan = "Unit plan"
	st.Config.Units = []Unit{{ID: "a", Bolt: "one", BaseCommit: st.Config.CodeRevision, Scope: []string{"a.go"}, Tests: []string{"test a"}, Status: "integrated", ResultCommit: strings.Repeat("f", 40), IntegratedCommit: strings.Repeat("f", 40)}}
	if err := os.WriteFile(filepath.Join(s.Root, "results.txt"), []byte("PASS"), 0600); err != nil {
		t.Fatal(err)
	}
	st.Config.Artifacts = append(st.Config.Artifacts, Artifact{Path: "results.txt", Kind: "test", Stage: "tdd"})
	st, err := s.Save(st, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := s.Check(st.ID)
	if err != nil || gate.Status != "fail" {
		t.Fatalf("invented result accepted %+v %v", gate, err)
	}
}
func TestFlowSensorArtifactStageAndAuthority(t *testing.T) {
	for _, tc := range []struct{ name, kind, stage, path, want string }{{"future", "test", "tdd", "future.txt", "pass"}, {"future escape", "test", "tdd", "../future", "fail"}, {"state", "test", "discovery", "STATE", "fail"}, {"runtime", "test", "discovery", "aidlc/.runtime/proof.txt", "fail"}, {"kind", "unknown", "discovery", "KNOWLEDGE", "fail"}, {"stage", "Knowledge", "tomorrow", "KNOWLEDGE", "fail"}} {
		t.Run(tc.name, func(t *testing.T) {
			s, st := sensorFixture(t)
			name := tc.path
			if name == "STATE" {
				name = s.path(st.ID)
			}
			if name == "KNOWLEDGE" {
				name = st.Config.Artifacts[0].Path
			}
			if tc.name == "runtime" {
				os.MkdirAll(filepath.Dir(filepath.Join(s.Root, name)), 0700)
				os.WriteFile(filepath.Join(s.Root, name), []byte("proof"), 0600)
			}
			st.Config.Artifacts = append(st.Config.Artifacts, Artifact{Path: name, Kind: tc.kind, Stage: tc.stage})
			st, err := s.Save(st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != tc.want {
				t.Fatalf("want %s: %+v %v", tc.want, gate, err)
			}
		})
	}
}
func TestFlowSensorUnitIDsAreComponents(t *testing.T) {
	for _, id := range []string{"../escape", "a/b", "a\\b", ".", ""} {
		s, st := sensorFixture(t)
		st.Stage = "planning"
		st.Config.Plan = "Plan"
		st.Config.Units = []Unit{{ID: id, Bolt: "one", BaseCommit: st.Config.CodeRevision, Scope: []string{"a.go"}, Tests: []string{"test"}}}
		st, err := s.Save(st, st.Revision)
		if err != nil {
			t.Fatal(err)
		}
		gate, err := s.Check(st.ID)
		if err != nil || gate.Status != "fail" {
			t.Errorf("unsafe ID %q: %+v %v", id, gate, err)
		}
	}
}
