package minimal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func TestAssignmentContract(t *testing.T) {
	s := Service{Root: t.TempDir(), Binary: "/opt/aidlc"}
	if err := os.WriteFile(filepath.Join(s.Root, "init.json"), []byte(`{"request_id":"init","human_confirmed":true,"reason":"human confirmed work stopped"}`), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := s.Execute(cli.MinimalRequest{Command: "assignment", Action: "init", File: "init.json"})
	if err != nil {
		t.Fatal(err)
	}
	var r assignment.Registry
	if err := json.Unmarshal(raw, &r); err != nil || r.Epoch == "" {
		t.Fatalf("init output: %s %v", raw, err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "assignment", Action: "list"}); err != nil {
		t.Fatal(err)
	}
	// A forged owner argument must be denied even before normal selected-Intent checks.
	out := hook(t, s, "PreToolUse", "Bash", "tool", "/opt/aidlc assignment release id --session another --expect 1 --file release.json", false)
	if !deny(out) {
		t.Fatal("forged release owner accepted")
	}
	out = hook(t, s, "PreToolUse", "Bash", "tool", "/opt/aidlc assignment list", false)
	if deny(out) {
		t.Fatal("registry diagnostics blocked without selection")
	}
}

func TestAssignmentContractOwnerBoundary(t *testing.T) {
	for _, name := range []string{"release", "reserve", "reserve-space", "reserve-intent", "release-root", "claim", "reassign", "claim-space", "claim-intent", "claim-root", "claim-unknown", "claim-duplicate", "claim-missing", "owner-release"} {
		t.Run(name, func(t *testing.T) {
			s, st := agentFixture(t)
			store := flow.Store{Root: s.Root, Space: "default"}
			st, err := store.Begin(st.ID, st.Revision)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.CheckWork(st.ID); err != nil {
				t.Fatal(err)
			}
			bind(t, s, st.ID)
			if deny(hook(t, s, "PreToolUse", "Bash", "control", "true", false)) {
				t.Fatal("ordinary Bash control denied")
			}
			hook(t, s, "PostToolUse", "Bash", "control", "", false)
			reg, err := (assignment.Store{Root: s.Root}).Read()
			if err != nil {
				t.Fatal(err)
			}
			worker := t.TempDir()
			if out, err := exec.Command("git", "-C", worker, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
			owner := "another"
			if name == "owner-release" || name == "release-root" {
				owner = "session"
			}
			reservation, err := (assignment.Store{Root: s.Root}).Reserve(assignment.ReserveRequest{RegistryEpoch: reg.Epoch, RequestID: "reserve", CoordinatorSession: owner, Session: "worker", Space: "default", IntentID: st.ID, StepID: st.CurrentStepID, DefinitionHash: st.DefinitionHash, Root: worker, Agent: "aidlc-worker", SourceRevision: st.Revision})
			if err != nil {
				t.Fatal(err)
			}
			draft := `{"registry_epoch":"` + reg.Epoch + `","request_id":"release","previous_run_stopped":true,"no_more_requests":true,"reason":"collected"}`
			command := fmt.Sprintf("/opt/aidlc assignment release %s --session %s --expect 1 --file request.json", reservation.ID, owner)
			if strings.HasPrefix(name, "reserve") {
				draft = `{"registry_epoch":"` + reg.Epoch + `","request_id":"next","step_id":"` + st.CurrentStepID + `","agent":"aidlc-worker","root":"` + worker + `","session":"worker"}`
				command = fmt.Sprintf("/opt/aidlc assignment reserve %s --space default --session another --expect %d --file request.json", st.ID, st.Revision)
				if name != "reserve" {
					command = strings.Replace(command, "--session another", "--session session", 1)
				}
			}
			if strings.HasPrefix(name, "claim") || name == "reassign" {
				action := "claim"
				if name == "reassign" {
					action = "reassign"
				}
				draft = `{"coordinator_session":"another"}`
				command = fmt.Sprintf("/opt/aidlc unit %s %s --space default --expect %d --file request.json", action, st.ID, st.Revision)
				if name != "claim" && name != "reassign" {
					draft = `{"coordinator_session":"session"}`
				}
			}
			if strings.HasSuffix(name, "-space") {
				command = strings.Replace(command, "--space default", "--space other", 1)
			}
			if strings.HasSuffix(name, "-intent") {
				command = strings.Replace(command, st.ID, strings.Repeat("a", 32), 1)
			}
			if strings.HasSuffix(name, "-root") {
				command += " --project-dir /other/root"
			}
			if name == "claim-unknown" {
				draft = `{"coordinator_session":"session","unknown":true}`
			}
			if name == "claim-duplicate" {
				draft = `{"coordinator_session":"session","coordinator_session":"session"}`
			}
			if name != "claim-missing" {
				if err := os.WriteFile(filepath.Join(s.Root, "request.json"), []byte(draft), 0600); err != nil {
					t.Fatal(err)
				}
			}
			request, err := cli.ParseMinimal(strings.Fields(command)[1:])
			if err != nil {
				t.Fatal(err)
			}
			out := hook(t, s, "PreToolUse", "Bash", "managed", command, false)
			var executionErr error
			if !deny(out) {
				_, executionErr = s.Execute(request)
				hook(t, s, "PostToolUse", "Bash", "managed", "", false)
			}
			if name == "owner-release" {
				if deny(out) || executionErr != nil {
					t.Fatalf("owner release blocked: %+v %v", out, executionErr)
				}
			} else if !deny(out) {
				t.Fatalf("foreign or invalid managed command reached Execute: %s (execution error %v)", name, executionErr)
			}
			got, err := (assignment.Store{Root: s.Root}).Read()
			if err != nil {
				t.Fatal(err)
			}
			want := "reserved"
			if name == "owner-release" {
				want = "released"
			}
			if got.Reservations[0].Status != want {
				t.Fatalf("reservation status %s want %s", got.Reservations[0].Status, want)
			}
		})
	}
}
