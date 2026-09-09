package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeKBIntegrationFeature(t *testing.T) {
	for _, folder := range []string{"codekb", "knowledge"} {
		t.Run(folder, func(t *testing.T) {
			s, st := boundaryFixture(t)
			fixtureExecutionStage(t, s, &st, "integration")
			prepareBoundaryStage(t, s, &st)
			st.Accepted["s04"] = StageAcceptance{StepID: "s04", Stage: "tdd", ReviewTarget: strings.Repeat("c", 64)}
			head := flowGit(t, s.Root, "rev-parse", "HEAD")
			st.Config = Config{Objective: "Work", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, CodeRevision: head, DirectCommit: head, Plan: "implement", Tests: []string{"go test"}}
			prepareBoundaryResults(t, s, &st)
			boundaryDoc(t, s, st, "CurrentAnalysis")
			boundaryDoc(t, s, st, "Architecture")
			output := boundaryDeclaration(t, s, st, "Knowledge")
			target := "aidlc/spaces/default/knowledge/" + folder + "/feature.md"
			if target != output.Path {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(s.Root, target)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(s.Root, output.Path), filepath.Join(s.Root, target)); err != nil {
					t.Fatal(err)
				}
				output.Path = target
			}
			st.Config.DocumentOutputs = []DocumentDeclaration{output}
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil {
				t.Fatal(err)
			}
			if folder == "codekb" && gate.Status != "pass" {
				t.Fatalf("codekb feature rejected: %+v", gate)
			}
			if folder == "knowledge" && (gate.Status != "fail" || !strings.Contains(gate.Summary, "integration Knowledge must use codekb")) {
				t.Fatalf("old folder substituted for codekb: %+v", gate)
			}
		})
	}
}

func TestCodeKBSharedDocuments(t *testing.T) {
	s, st := boundaryFixture(t)
	for _, kind := range []string{"CurrentAnalysis", "Architecture"} {
		source := boundaryDoc(t, s, st, kind)
		target := "aidlc/spaces/default/knowledge/codekb/" + filepath.Base(source)
		if source != target {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(s.Root, target)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(s.Root, source), filepath.Join(s.Root, target)); err != nil {
				t.Fatal(err)
			}
		}
	}
	boundaryDoc(t, s, st, "Requirements")
	st.Config.NoMaterialsReason = "new project"
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	var err error
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	collector := s.endDocuments(st)
	if len(collector.failures) > 0 {
		t.Fatalf("codekb shared docs rejected: %v", collector.failures)
	}
}
