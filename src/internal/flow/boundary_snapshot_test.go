package flow

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundaryTransitionCollectorSnapshot(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			s := flowStore(t)
			boundaryFile(t, s, "evidence.txt", "reviewed")
			c := boundaryCollector{store: s}
			first, ok := c.file("evidence.txt")
			if !ok {
				t.Fatal(c.failures)
			}
			if remove {
				if err := os.Remove(filepath.Join(s.Root, "evidence.txt")); err != nil {
					t.Fatal(err)
				}
			} else {
				boundaryFile(t, s, "evidence.txt", "changed")
			}
			second, ok := c.file("evidence.txt")
			if !ok || string(first) != string(second) {
				t.Fatal("collector reread changed or missing bytes under the first hash")
			}
		})
	}
}
func TestBoundaryTransitionUsesOneSensorSnapshot(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			s, st := boundaryFixture(t)
			fixtureExecutionStage(t, s, &st, "tdd")
			prepareBoundaryStage(t, s, &st)
			_ = flowGit(t, s.Root, "rev-parse", "HEAD")
			st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}, Plan: "Implement", Tests: []string{"go test"}}
			prepareBoundaryResults(t, s, &st)
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			g, err := s.Check(st.ID)
			if err != nil || g.Status != "pass" {
				t.Fatalf("gate %+v %v", g, err)
			}
			st.Review = Gate{StepID: st.CurrentStepID, Status: "pass", Target: g.Target}
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			output := "aidlc/evidence/tdd.txt"
			want := boundaryVersion(t, s, output)
			revision, hash := evidencePlan(st)
			approval, err := newApproval(st, st.Review.Target, revision, hash)
			if err != nil {
				t.Fatal(err)
			}
			approval.Status = "approved"
			approval.Session = "fixture"
			approval.Turn = "fixture"
			approval.Quote = "fixture"
			approval.PromptHash = st.DefinitionHash
			st.Approval = approval
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			writes := 0
			s.write = func(root, name string, raw []byte) error {
				writes++
				if writes == 1 && remove {
					if err := os.Remove(filepath.Join(root, output)); err != nil {
						return err
					}
				} else if writes == 1 {
					if err := os.WriteFile(filepath.Join(root, output), []byte("changed after snapshot"), 0600); err != nil {
						return err
					}
				}
				return filestore.WriteFile(root, name, raw)
			}
			next, err := s.Finish(st.ID, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range next.Accepted["s04"].Outputs {
				if f.Path == output {
					found = f == want
				}
			}
			if !found {
				t.Fatal("accepted outputs differ from the reviewed Sensor snapshot")
			}
			if writes == 0 {
				t.Fatalf("state writes=%d", writes)
			}
		})
	}
}

func TestBoundaryTransitionEvidenceRole(t *testing.T) {
	for _, changed := range []string{"results.json", "output.log"} {
		t.Run(changed, func(t *testing.T) {
			s, st := boundaryFixture(t)
			fixtureExecutionStage(t, s, &st, "tdd")
			prepareBoundaryStage(t, s, &st)
			head := verificationTestSHA(t, s.Root, []string{"."})
			st.Config = Config{Objective: "Build", Scope: []string{"src"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"."}, Plan: "Implement", Tests: []string{"go test"}}
			prefix := "aidlc/evidence/"
			boundaryFile(t, s, prefix+"output.log", "PASS")
			boundaryFile(t, s, prefix+"results.json", fmt.Sprintf(`{"step_id":"s04","stage":"tdd","verification_scope":"intent","verification_sha256":%q,"runs":[{"command":"go test","exit_code":0,"output_path":%q}]}`, head, prefix+"output.log"))
			st.Config.TestResults = []string{prefix + "results.json"}
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != "pass" {
				t.Fatalf("%+v %v", gate, err)
			}
			st.Review = Gate{StepID: st.CurrentStepID, Status: "pass", Target: gate.Target}
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "advance"})
			if err != nil {
				t.Fatal(err)
			}
			if len(st.Accepted["s04"].Outputs) != 2 {
				t.Errorf("TDD acceptance includes shared documents: %+v", st.Accepted["s04"].Outputs)
			}
			boundaryFile(t, s, prefix+changed, "changed")
			start, _, _ := s.startState(st)
			if start.Status == "pass" {
				t.Error("integration start ignored changed knowledge evidence")
			}
			st.Entry = &StageEntry{StepID: "s05", Stage: "integration"}
			if err = s.checkWorkState(st); err == nil {
				t.Error("work ignored changed knowledge evidence")
			}
		})
	}
}
