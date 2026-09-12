//go:build integration

package main

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/assignment"
	"github.com/sori883/ai-dd/src/internal/flow"
)

func TestAssignmentJourney(t *testing.T) {
	f := operationsNew(t)
	st := f.tdd()
	worker := f.worktree()
	st = f.unit(st, "claim", flow.UnitRequest{Unit: "a", Session: "worker-a", Root: worker})
	var records []assignment.Reservation
	if err := json.Unmarshal(f.ok("assignment", "list"), &records); err != nil || len(records) != 1 {
		t.Fatalf("reservations: %+v %v", records, err)
	}
	v := records[0]
	call := func(input app.HookInput) map[string]any {
		t.Helper()
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		output := runAIDLCCLI(t, f.binary, f.root, raw, "__hook", "--project-dir", f.root)
		var out map[string]any
		if err := json.Unmarshal(output, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	denied := func(out map[string]any) bool {
		specific, _ := out["hookSpecificOutput"].(map[string]any)
		return specific["permissionDecision"] == "deny"
	}
	call(app.HookInput{Event: "UserPromptSubmit", Session: "coordinator", Turn: "turn"})
	f.bind(st, "coordinator")
	pre := app.HookInput{Event: "PreToolUse", Session: "coordinator", Turn: "turn", ID: "spawn", Tool: "collaborationspawn_agent"}
	pre.Input.AgentType = "aidlc-worker"
	pre.Input.TaskName = v.TaskName
	if out := call(pre); denied(out) {
		t.Fatalf("reserved spawn rejected: %+v", out)
	}
	follow := app.HookInput{Event: "PreToolUse", Session: "coordinator", Turn: "turn", ID: "follow", Tool: "collaborationfollowup_task"}
	follow.Input.Target = v.TaskName
	if !denied(call(follow)) {
		t.Fatal("missing post allowed followup")
	}
	post := pre
	post.Event = "PostToolUse"
	post.Response, _ = json.Marshal(`{"task_name":"/root/` + v.TaskName + `"}`)
	call(post)
	if denied(call(follow)) {
		t.Fatal("bound followup denied")
	}
	var runtime flow.UnitRequest
	if err := json.Unmarshal(operationsRead(t, f.root+"/aidlc/.runtime/flow/units/default/"+st.ID+"/a.json"), &runtime); err != nil {
		t.Fatal(err)
	}
	runtime.VerificationSHA256 = "HEAD"
	st = f.unit(st, "result", runtime)
	request := flow.AssignmentRequest{RegistryEpoch: v.RegistryEpoch, RequestID: "competing", StepID: st.CurrentStepID, Agent: "aidlc-worker", Root: worker, Session: "worker-b"}
	conflict := f.run("assignment", "reserve", st.ID, "--space", "default", "--session", "coordinator", "--expect", strconv.FormatUint(st.Revision, 10), "--file", f.request(request))
	if conflict.code == 0 {
		t.Fatal("reported root was freed")
	}
	if err := json.Unmarshal(f.ok("assignment", "show", v.ID), &v); err != nil {
		t.Fatal(err)
	}
	release := assignment.ReleaseRequest{RegistryEpoch: v.RegistryEpoch, RequestID: "release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "synthetic fixture: no actual child process, results collected"}
	f.ok("assignment", "release", v.ID, "--session", "coordinator", "--expect", strconv.FormatUint(v.EntryRevision, 10), "--file", f.request(release))
	if !denied(call(follow)) {
		t.Fatal("released child accepted followup")
	}
	request.RequestID = "new-reservation"
	f.ok("assignment", "reserve", st.ID, "--space", "default", "--session", "coordinator", "--expect", strconv.FormatUint(st.Revision, 10), "--file", f.request(request))
}
