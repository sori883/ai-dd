//go:build integration

package main

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGitIndependentJourney(t *testing.T) {
	binary := buildMinimalBinary(t)
	for _, mode := range []string{"no-git", "detect-git"} {
		t.Run(mode, func(t *testing.T) {
			productPath := t.TempDir()
			marker := filepath.Join(productPath, "called")
			if mode == "detect-git" {
				stub := filepath.Join(productPath, "git")
				if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf called > '"+marker+"'\nexit 99\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("AIDLC_TEST_PRODUCT_PATH", productPath)
			for _, units := range []bool{false, true} {
				name := "direct"
				if units {
					name = "units"
				}
				t.Run(name, func(t *testing.T) {
					root := t.TempDir()
					runGitIndependentBoundaryJourney(t, binary, root, units)
					if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
						t.Fatal("fixture acquired Git marker")
					}
				})
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("product invoked Git")
			}
		})
	}
}
func runGitIndependentUnits(t *testing.T, binary, root string, st *flow.State, config flow.Config) flow.Config {
	t.Helper()
	config.Units = []flow.Unit{{ID: "a", StepID: st.CurrentStepID, Bolt: "one", Status: "pending", Scope: []string{"add.go"}, Tests: config.Tests}, {ID: "b", StepID: st.CurrentStepID, Bolt: "two", Status: "pending", DependsOn: []string{"a"}, Scope: []string{"add.go"}, Tests: config.Tests}}
	request := func(value any) string {
		raw, err := marshalFlowRequest(value)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, "aidlc/.runtime/request.json")
		writeMinimalFixture(t, p, string(raw))
		return p
	}
	update := func(command, action string, value any) {
		args := []string{command, action, st.ID, "--space", "default", "--expect", strconv.FormatUint(st.Revision, 10), "--file", request(value)}
		raw := runMinimalCLI(t, binary, root, nil, args...)
		if err := json.Unmarshal(raw, st); err != nil {
			t.Fatal(err)
		}
	}
	update("intent", "configure", config)
	var registry assignment.Registry
	raw := runMinimalCLI(t, binary, root, nil, "assignment", "init", "--file", request(assignment.InitRequest{RequestID: "init", HumanConfirmed: true, Reason: "synthetic fixture: no prior workers"}))
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	for _, unit := range []string{"a", "b"} {
		claim := flow.UnitRequest{StepID: st.CurrentStepID, Unit: unit, Root: root, Session: "worker-" + unit, CoordinatorSession: "main", RegistryEpoch: registry.Epoch, RequestID: "claim-" + unit}
		update("unit", "claim", claim)
		var records assignment.Registry
		if err := json.Unmarshal(runMinimalCLI(t, binary, root, nil, "assignment", "list"), &records); err != nil {
			t.Fatal(err)
		}
		var reservation assignment.Reservation
		for _, v := range records.Reservations {
			if v.Unit == unit {
				reservation = v
			}
		}
		if unit == "b" {
			writeMinimalFixture(t, filepath.Join(root, "add.go"), "package add\n// Unit B adjusts shared implementation after A is integrated.\nfunc Add(a,b int)int{return b+a}\n")
		}
		var view flow.VerificationView
		if err := json.Unmarshal(runMinimalCLI(t, binary, root, nil, "intent", "hash", st.ID, "--space", "default", "--unit", unit, "--root", root), &view); err != nil {
			t.Fatal(err)
		}
		output := runMinimalProcess(t, root, "go", "test", "-count=1", "-run", "^TestAdd$")
		log := "aidlc/evidence/" + unit + ".txt"
		writeMinimalFixture(t, filepath.Join(root, log), string(output))
		doc := map[string]any{"step_id": st.CurrentStepID, "stage": "tdd", "verification_scope": "unit", "verification_sha256": view.SHA256, "unit_id": unit, "run_id": reservation.RunID, "runs": []map[string]any{{"unit_id": unit, "command": config.Tests[0], "exit_code": 0, "output_path": log}}}
		raw, _ := json.Marshal(doc)
		result := "aidlc/evidence/unit-" + unit + ".json"
		writeMinimalFixture(t, filepath.Join(root, result), string(raw))
		current := st.Config
		current.TestResults = append(current.TestResults, result)
		update("intent", "configure", current)
		update("unit", "result", flow.UnitRequest{StepID: st.CurrentStepID, Unit: unit, Root: root, Session: claim.Session, RunID: reservation.RunID, VerificationSHA256: view.SHA256})
		update("unit", "integrate", flow.UnitRequest{StepID: st.CurrentStepID, Unit: unit})
		runMinimalCLI(t, binary, root, nil, "assignment", "release", reservation.ID, "--session", "main", "--expect", strconv.FormatUint(reservation.EntryRevision, 10), "--file", request(assignment.ReleaseRequest{RegistryEpoch: registry.Epoch, RequestID: "release-" + unit, PreviousRunStopped: true, NoMoreRequests: true, Reason: "synthetic synchronous worker collected"}))
		if st.Config.Units[0].Status != "integrated" {
			t.Fatal("Unit A lost integrated status")
		}
	}
	return st.Config
}
