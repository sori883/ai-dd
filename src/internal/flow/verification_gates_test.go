package flow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerificationGates(t *testing.T) {
	for _, change := range []string{"none", "mtime", "state", "code", "scope", "document"} {
		t.Run(change, func(t *testing.T) {
			s, st := sensorFixture(t)
			st.Config.VerificationPaths = []string{"code"}
			var err error
			st, err = s.Save(st, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.RemoveAll(filepath.Join(s.Root, ".git")); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("code"), 0644); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != "pass" {
				t.Fatalf("gitless Sensor %+v %v", gate, err)
			}
			req := ReviewRequest{Action: "assign", CoordinatorSession: "main", Session: "reviewer", Root: s.Root}
			st, err = s.Review(st.ID, st.Revision, req)
			if err != nil {
				t.Fatalf("same-root independent review: %v", err)
			}
			switch change {
			case "mtime":
				if err = os.Chtimes(filepath.Join(s.Root, "code"), time.Unix(5, 0), time.Unix(5, 0)); err != nil {
					t.Fatal(err)
				}
			case "state":
				st, err = s.Save(st, st.Revision)
				if err != nil {
					t.Fatal(err)
				}
			case "code":
				if err = os.WriteFile(filepath.Join(s.Root, "code"), []byte("different"), 0644); err != nil {
					t.Fatal(err)
				}
			case "scope":
				st.Config.VerificationPaths = append(st.Config.VerificationPaths, "extra")
				st, err = s.Save(st, st.Revision)
				if err != nil {
					t.Fatal(err)
				}
			case "document":
				p := filepath.Join(s.Root, st.Config.Artifacts[0].Path)
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(p, append(raw, []byte("\nChanged.\n")...), 0644); err != nil {
					t.Fatal(err)
				}
			}
			req.Action = "accept"
			req.Target = gate.Target
			req.Status = "pass"
			req.Summary = "reviewed"
			accepted, err := s.Review(st.ID, st.Revision, req)
			stable := change == "none" || change == "mtime" || change == "state"
			if stable {
				if err != nil {
					t.Fatal(err)
				}
				current, err := s.Check(accepted.ID)
				if err != nil || current.Target != gate.Target {
					t.Fatalf("review/state changed Target %+v %v", current, err)
				}
			} else if err == nil {
				t.Fatal("stale review accepted")
			}
		})
	}
}
func TestVerificationGatesIndependence(t *testing.T) {
	s, st := sensorFixture(t)
	if _, err := s.Review(st.ID, st.Revision, ReviewRequest{Action: "assign", CoordinatorSession: "same", Session: "same", Root: s.Root}); err == nil {
		t.Fatal("same session reviewer accepted")
	}
}

func TestVerificationGatesUnitEvidence(t *testing.T) {
	for _, change := range []string{"current result", "current output", "previous result", "previous output"} {
		t.Run(change, func(t *testing.T) {
			s, st := boundaryFixture(t)
			fixtureExecutionStage(t, s, &st, "integration")
			prepareBoundaryStage(t, s, &st)
			st.Accepted["s04"] = StageAcceptance{StepID: "s04", Stage: "tdd", ReviewTarget: strings.Repeat("c", 64)}
			st.Config = Config{Objective: "Build", Scope: []string{"code"}, Acceptance: []string{"works"}, NoMaterialsReason: "new", ADR: ADR{Reason: "none"}, VerificationPaths: []string{"code"}, Plan: "Implement", Tests: []string{"test"}}
			boundaryFile(t, s, "code", "current code")
			sha := verificationTestSHA(t, s.Root, st.Config.VerificationPaths)
			oldSHA := strings.Repeat("a", 64)
			st.Config.Units = []Unit{{ID: "a", StepID: st.CurrentStepID, Bolt: "one", Scope: []string{"code"}, Tests: []string{"test"}, Status: "integrated", ResultSHA256: oldSHA}}
			zero := 0
			whole := resultDocument{StepID: st.CurrentStepID, Stage: st.Stage, VerificationScope: "intent", VerificationSHA256: sha, Runs: []resultRun{{UnitID: "a", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/whole.txt"}}}
			current := resultDocument{StepID: st.CurrentStepID, Stage: st.Stage, VerificationScope: "unit", VerificationSHA256: oldSHA, UnitID: "a", RunID: "run-a", Runs: []resultRun{{UnitID: "a", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/unit.txt"}}}
			// This older result is only listed in TestResults, not an accepted input.
			previous := current
			previous.StepID, previous.Stage = "s04", "tdd"
			previous.Runs = []resultRun{{UnitID: "a", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/previous.txt"}}
			for name, doc := range map[string]resultDocument{"whole": whole, "unit": current, "previous": previous} {
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				path := "aidlc/evidence/" + name + ".json"
				boundaryFile(t, s, path, string(raw))
				boundaryFile(t, s, doc.Runs[0].OutputPath, "passed")
				st.Config.TestResults = append(st.Config.TestResults, path)
			}
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			gate, err := s.Check(st.ID)
			if err != nil || gate.Status != "pass" {
				t.Fatalf("initial gate: %+v %v", gate, err)
			}
			st.Review = Gate{StepID: st.CurrentStepID, Status: "pass", Target: gate.Target}
			revision, planHash := evidencePlan(st)
			st.Approval, err = newApproval(st, gate.Target, revision, planHash)
			if err != nil {
				t.Fatal(err)
			}
			st.Approval.Status = "approved"
			st.Approval.Session, st.Approval.Turn, st.Approval.Quote, st.Approval.PromptHash = "fixture", "fixture", "synthetic approval", st.DefinitionHash
			if err := s.persist(st); err != nil {
				t.Fatal(err)
			}
			name := "unit"
			if strings.HasPrefix(change, "previous") {
				name = "previous"
			}
			path := "aidlc/evidence/" + name + ".txt"
			if strings.HasSuffix(change, "result") {
				path = "aidlc/evidence/" + name + ".json"
				raw, err := os.ReadFile(filepath.Join(s.Root, path))
				if err != nil {
					t.Fatal(err)
				}
				boundaryFile(t, s, path, string(raw)+"\n")
			} else {
				boundaryFile(t, s, path, "changed nonempty output")
			}
			after, err := s.Check(st.ID)
			if err != nil || after.Status != "pass" {
				t.Fatalf("content remains valid: %+v %v", after, err)
			}
			currentChanged := name == "unit"
			if (after.Target != gate.Target) != currentChanged {
				t.Errorf("target changed=%v, want %v", after.Target != gate.Target, currentChanged)
			}
			next, err := s.Finish(st.ID, st.Revision)
			if currentChanged {
				if err == nil {
					t.Error("stale review and approval allowed finish")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				seen := map[string]bool{}
				for _, output := range next.Accepted["s05"].Outputs {
					seen[output.Path] = true
				}
				if !seen["aidlc/evidence/unit.json"] || !seen["aidlc/evidence/unit.txt"] {
					t.Error("accepted proof omits current Unit evidence")
				}
				if seen["aidlc/evidence/previous.json"] || seen["aidlc/evidence/previous.txt"] {
					t.Error("accepted proof includes prior step evidence")
				}
			}
		})
	}
}
