package flow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationResults(t *testing.T) {
	for _, change := range []string{"valid", "sha", "step", "scope", "unit", "command", "output", "exit", "second unit missing", "self cycle"} {
		t.Run(change, func(t *testing.T) {
			s := flowStore(t)
			if err := os.WriteFile(filepath.Join(s.Root, "code"), []byte("code"), 0644); err != nil {
				t.Fatal(err)
			}
			digest, err := ComputeVerification(s.Root, []string{"."})
			if err != nil {
				t.Fatal(err)
			}
			zero := 0
			st := State{Stage: "tdd", CurrentStepID: "s04", Config: Config{VerificationPaths: []string{"."}, Units: []Unit{{ID: "a", Tests: []string{"test"}}, {ID: "b", Tests: []string{"test"}}}, TestResults: []string{"aidlc/evidence/result.json"}}}
			doc := resultDocument{StepID: "s04", Stage: "tdd", VerificationScope: "intent", VerificationSHA256: digest.SHA256, Runs: []resultRun{{UnitID: "a", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/output"}, {UnitID: "b", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/output"}}}
			body := "passed"
			switch change {
			case "sha":
				doc.VerificationSHA256 = strings.Repeat("a", 64)
			case "step":
				doc.StepID = "s03"
			case "scope":
				doc.VerificationScope = "unit"
			case "unit":
				doc.Runs[1].UnitID = "c"
			case "command":
				doc.Runs[1].Command = "other"
			case "output":
				body = ""
			case "exit":
				one := 1
				doc.Runs[1].ExitCode = &one
			case "second unit missing":
				doc.Runs = doc.Runs[:1]
			}
			if err = os.MkdirAll(filepath.Join(s.Root, "aidlc/evidence"), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(s.Root, "aidlc/evidence/output"), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(doc)
			if err = os.WriteFile(filepath.Join(s.Root, st.Config.TestResults[0]), raw, 0644); err != nil {
				t.Fatal(err)
			}
			c := boundaryCollector{store: s}
			c.results(st)
			if change == "valid" || change == "self cycle" {
				if len(c.failures) > 0 {
					t.Fatal(c.failures)
				}
				after, err := ComputeVerification(s.Root, st.Config.VerificationPaths)
				if err != nil || after.SHA256 != digest.SHA256 {
					t.Fatal("result changed code SHA")
				}
			} else if len(c.failures) == 0 {
				t.Fatal("invalid result accepted")
			}
		})
	}
}

func TestVerificationResultsUnit(t *testing.T) {
	for _, bad := range []string{"", "run", "unit", "scope"} {
		t.Run("binding "+bad, func(t *testing.T) {
			s := flowStore(t)
			digest, err := ComputeVerification(s.Root, []string{"code"})
			if err != nil {
				t.Fatal(err)
			}
			zero := 0
			st := State{Stage: "tdd", CurrentStepID: "s04", Config: Config{TestResults: []string{"aidlc/evidence/result.json"}}}
			doc := resultDocument{StepID: "s04", Stage: "tdd", VerificationScope: "unit", VerificationSHA256: digest.SHA256, UnitID: "a", RunID: "run-a", Runs: []resultRun{{UnitID: "a", Command: "test", ExitCode: &zero, OutputPath: "aidlc/evidence/output"}}}
			switch bad {
			case "run":
				doc.RunID = "other"
			case "unit":
				doc.Runs[0].UnitID = "b"
			case "scope":
				doc.VerificationScope = "intent"
			}
			if err := os.MkdirAll(filepath.Join(s.Root, "aidlc/evidence"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.Root, "aidlc/evidence/output"), []byte("pass"), 0644); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(doc)
			if err := os.WriteFile(filepath.Join(s.Root, st.Config.TestResults[0]), raw, 0644); err != nil {
				t.Fatal(err)
			}
			c := boundaryCollector{store: s}
			c.verificationResults(st, digest.SHA256, "a", "run-a", map[resultRequirement]bool{{unit: "a", command: "test"}: true})
			if bad == "" && len(c.failures) > 0 {
				t.Fatal(c.failures)
			}
			if bad != "" && len(c.failures) == 0 {
				t.Fatal("bad unit result accepted")
			}
		})
	}
}
