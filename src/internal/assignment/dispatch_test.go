package assignment

import "testing"

func TestAssignmentDispatch(t *testing.T) {
	s, r, a, _ := registryFixture(t)
	v, err := s.Reserve(reserveRequest(r, a))
	if err != nil {
		t.Fatal(err)
	}
	req := DispatchRequest{Session: "main", Turn: "turn", ToolID: "spawn", TaskName: v.TaskName, Agent: v.Agent, Space: v.Space, IntentID: v.IntentID, StepID: v.StepID, DefinitionHash: v.DefinitionHash}
	first, err := s.PreSpawn(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "dispatch_pending" {
		t.Fatalf("status=%s", first.Status)
	}
	if _, err := s.PreSpawn(req); err != nil {
		t.Fatalf("same pre retry: %v", err)
	}
	different := req
	different.ToolID = "other"
	if _, err := s.PreSpawn(different); err == nil {
		t.Fatal("same name respawn allowed")
	}
	if _, err := s.CheckTarget(req.Session, req.TaskName, req.Space, req.IntentID, req.StepID, req.DefinitionHash); err == nil {
		t.Fatal("missing post allowed followup")
	}
	raw := []byte(`"{\"task_name\":\"/root/` + req.TaskName + `\"}"`)
	bound, err := s.PostSpawn(req.Session, req.ToolID, raw)
	if err != nil {
		t.Fatal(err)
	}
	if bound.Canonical != "/root/"+req.TaskName {
		t.Fatalf("canonical=%q", bound.Canonical)
	}
	for _, target := range []string{req.TaskName, bound.Canonical} {
		if _, err := s.CheckTarget(req.Session, target, req.Space, req.IntentID, req.StepID, req.DefinitionHash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CheckTarget(req.Session, req.TaskName, req.Space, req.IntentID, "new-step", req.DefinitionHash); err == nil {
		t.Fatal("old step followup allowed")
	}
	current, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	v = current.Reservations[0]
	if _, err := s.Release(v.ID, "main", v.EntryRevision, ReleaseRequest{RegistryEpoch: r.Epoch, RequestID: "release", PreviousRunStopped: true, NoMoreRequests: true, Reason: "collected and stopped"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckTarget(req.Session, req.TaskName, req.Space, req.IntentID, req.StepID, req.DefinitionHash); err == nil {
		t.Fatal("released followup allowed")
	}
	if _, err := s.PostSpawn(req.Session, req.ToolID, raw); err == nil {
		t.Fatal("late post revived released reservation")
	}
}
func TestAssignmentDispatchReadOnly(t *testing.T) {
	s, _, _, _ := registryFixture(t)
	req := DispatchRequest{Session: "main", Turn: "turn", ToolID: "one", TaskName: "review", Agent: "aidlc-reviewer", Space: "default", IntentID: "11111111111111111111111111111111", StepID: "s01", DefinitionHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := s.PreSpawn(req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostSpawn("main", "one", []byte(`{"unexpected":"/root/review"}`)); err == nil {
		t.Fatal("unknown response accepted")
	}
	if _, err := s.CheckTarget("main", "review", req.Space, req.IntentID, req.StepID, req.DefinitionHash); err == nil {
		t.Fatal("uncertain followup allowed")
	}
	req.TaskName = "review_new"
	req.ToolID = "two"
	if _, err := s.PreSpawn(req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostSpawn("main", "two", []byte(`{"task_name":"/root/review_new"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckTarget("other", "review_new", req.Space, req.IntentID, req.StepID, req.DefinitionHash); err == nil {
		t.Fatal("other parent accepted")
	}
}

func TestAssignmentDispatchUnknownPath(t *testing.T) {
	s, _, _, _ := registryFixture(t)
	req := DispatchRequest{Session: "main", Turn: "turn", ToolID: "spawn", TaskName: "review", Agent: "aidlc-reviewer", Space: "default", IntentID: "11111111111111111111111111111111", StepID: "s01", DefinitionHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if _, err := s.PreSpawn(req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PostSpawn("main", "spawn", []byte(`{"task_name":"/root/\u0001/review"}`)); err == nil {
		t.Fatal("unrecognized canonical namespace accepted")
	}
}
