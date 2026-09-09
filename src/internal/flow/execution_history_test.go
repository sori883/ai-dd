package flow

import (
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"strings"
	"testing"
)

func TestExecutionPlanReopenHistoryRecords(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("history")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	if st.HistoryHead == "" {
		t.Fatal("plan change has no committed history head")
	}
	first := st.HistoryHead
	if err = s.CaptureApproval(st.ID, "user", "", "A", "approve plan"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
	if err != nil {
		t.Fatal(err)
	}
	records, err := s.History(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 || records[0].Previous != first || records[0].State.ExecutionPlan.Approved == nil {
		t.Fatal("history lost plan/approval order")
	}
}
func TestExecutionPlanReopenHistoryStateFailure(t *testing.T) {
	s := executionFixture(t)
	st, err := s.Create("retry approval")
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "", "A", "approve"); err != nil {
		t.Fatal(err)
	}
	decision := planDecision(st, "user", "A")
	injected := errors.New("state write failed")
	s.write = func(root, name string, raw []byte) error {
		if strings.HasSuffix(name, "/state.json") {
			return injected
		}
		return filestore.WriteFile(root, name, raw)
	}
	if _, err = s.DecidePlan(st.ID, st.Revision, decision); !errors.Is(err, injected) {
		t.Fatalf("write failure: %v", err)
	}
	current, err := s.Read(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.HistoryHead != st.HistoryHead || current.ExecutionPlan.Approved != nil {
		t.Fatal("partial write published approval")
	}
	s.write = nil
	current, err = s.DecidePlan(st.ID, st.Revision, decision)
	if err != nil {
		t.Fatalf("answer consumed before state commit: %v", err)
	}
	records, err := s.History(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].State.Revision != current.Revision {
		t.Fatal("history head not committed with state")
	}
}

func TestExecutionPlanReopenHistoryInvalidRecord(t *testing.T) {
	for _, mode := range []string{"approval status", "request target", "history previous", "state status"} {
		t.Run(mode, func(t *testing.T) {
			s := executionFixture(t)
			st, err := s.Create("history validation")
			if err != nil {
				t.Fatal(err)
			}
			st, err = s.ProposePlan(st.ID, st.Revision, initialPlanRequest())
			if err != nil {
				t.Fatal(err)
			}
			records, err := s.History(st.ID)
			if err != nil {
				t.Fatal(err)
			}
			record := records[0]
			switch mode {
			case "approval status":
				record.State.ExecutionPlan.Draft.Approval.Status = "invented"
			case "request target":
				record.State.ExecutionPlan.Draft.Approval.Target = "wrong"
			case "history previous":
				record.Previous = "../escape"
			case "state status":
				record.State.Status = "invented"
			}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			head := filestore.Hash(raw)
			if err = filestore.WriteFile(s.Root, s.historyPath(st.ID, head), raw); err != nil {
				t.Fatal(err)
			}
			st.HistoryHead = head
			if err = s.persist(st); err != nil {
				t.Fatal(err)
			}
			if _, err = s.History(st.ID); err == nil {
				t.Fatal("invalid history value accepted")
			}
		})
	}
}

func completedDiscovery(t *testing.T) (Store, State) {
	t.Helper()
	s, st := discoveryApprovalFixture(t)
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve both"); err != nil {
		t.Fatal(err)
	}
	pd := planDecision(st, "user", "A")
	a := st.Approval
	var err error
	st, err = s.DecidePlan(st.ID, st.Revision, pd)
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Decide(st.ID, st.Revision, ApprovalDecision{RequestID: a.RequestID, Target: a.Target, Decision: "approve", Session: "user", Turn: "A", Quote: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Finish(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}
func TestExecutionPlanReopenHistoryProposal(t *testing.T) {
	s, st := completedDiscovery(t)
	old := st.ExecutionPlan.Approved.Steps[1]
	var err error
	st, err = s.Reopen(st.ID, st.Revision, "s02", "clarify requirements")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "completed" || st.CurrentStepID != "s02" || st.ExecutionPlan.Draft == nil {
		t.Fatal("unapproved reopen changed current progress")
	}
	if err = s.CaptureApproval(st.ID, "user", "A", "B", "approve reopen"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "B"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "active" || st.CurrentStepID != "s03" || st.ExecutionPlan.Approved.Steps[1] != old || st.ExecutionPlan.Approved.Steps[2].Status != "pending" || st.Accepted["s02"].StepID != "s02" {
		t.Fatal("reopen lost old execution or did not allocate new run")
	}
	raw, err := readWorkLog(s.Root, "aidlc/spaces/default/knowledge/log/"+st.ID+"-work-log.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "clarify requirements") || !strings.Contains(string(raw), "s02") {
		t.Fatal("reopen reason/step missing from log")
	}
}

func TestExecutionPlanReopenHistoryEarlyInitialization(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	// Adopt the first plan before rerunning initialization; discovery is unfinished.
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve plan"); err != nil {
		t.Fatal(err)
	}
	var err error
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Reopen(st.ID, st.Revision, "s01", "recheck initialization")
	if err != nil {
		t.Fatal(err)
	}
	if st.CurrentStepID != "s02" {
		t.Fatal("unapproved replay changed current execution")
	}
	if err = s.CaptureApproval(st.ID, "user", "A", "B", "approve replay"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "B"))
	if err != nil {
		t.Fatal(err)
	}
	steps := executionSteps(st)
	if len(steps) != 3 || steps[0].ID != "s01" || steps[1].Stage != "initialization" || steps[1].ID == "s01" || steps[2].ID != "s02" || st.CurrentStepID != steps[1].ID {
		t.Fatalf("invalid mandatory replay order: %+v", steps)
	}
	st, err = s.Begin(st.ID, st.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if st.Stage != "initialization" {
		t.Fatal("discovery began before replay initialization")
	}
}

func TestExecutionPlanReopenHistoryLogRetry(t *testing.T) {
	s, st := completedDiscovery(t)
	var err error
	st, err = s.Reopen(st.ID, st.Revision, "s02", "revisit")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "A", "B", "approve replay"); err != nil {
		t.Fatal(err)
	}
	decision := planDecision(st, "user", "B")
	injected := errors.New("final state failure")
	s.write = func(root, name string, raw []byte) error {
		if strings.HasSuffix(name, "/state.json") {
			var candidate State
			if err := json.Unmarshal(raw, &candidate); err != nil {
				return err
			}
			if candidate.ExecutionPlan.Revision == 2 && candidate.PendingReopen == nil {
				return injected
			}
		}
		return filestore.WriteFile(root, name, raw)
	}
	if _, err = s.DecidePlan(st.ID, st.Revision, decision); !errors.Is(err, injected) {
		t.Fatalf("injection: %v", err)
	}
	current, err := s.Read(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.PendingReopen == nil || current.ExecutionPlan.Revision != 1 {
		t.Fatal("durable pending boundary missing")
	}
	name := "aidlc/spaces/default/knowledge/log/" + st.ID + "-work-log.md"
	before, err := readWorkLog(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	s.write = nil
	current, err = s.DecidePlan(st.ID, st.Revision, decision)
	if err != nil {
		t.Fatalf("same approval retry failed: %v", err)
	}
	after, err := readWorkLog(s.Root, name)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || strings.Count(string(after), "## Reopen") != 1 || current.PendingReopen != nil {
		t.Fatal("retry duplicated or rewrote log")
	}
}

func TestExecutionPlanReopenHistoryCurrentReplacement(t *testing.T) {
	s, st := discoveryApprovalFixture(t)
	if err := s.CaptureApproval(st.ID, "user", "", "A", "approve plan"); err != nil {
		t.Fatal(err)
	}
	var err error
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "A"))
	if err != nil {
		t.Fatal(err)
	}
	st, err = s.Reopen(st.ID, st.Revision, "s02", "restart unfinished discovery")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CaptureApproval(st.ID, "user", "A", "B", "approve replacement"); err != nil {
		t.Fatal(err)
	}
	st, err = s.DecidePlan(st.ID, st.Revision, planDecision(st, "user", "B"))
	if err != nil {
		t.Fatal(err)
	}
	steps := st.ExecutionPlan.Approved.Steps
	if len(steps) != 2 || steps[1].ID == "s02" || steps[1].Reopens != "s02" || steps[1].Stage != "discovery" || steps[1].Status != "pending" || st.Entry != nil || st.Approval != nil || st.Review.Status != "" {
		t.Fatalf("unfinished execution not replaced: %+v", steps)
	}
	records, err := s.History(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range records {
		if record.State.ExecutionPlan.Approved == nil {
			continue
		}
		for _, old := range record.State.ExecutionPlan.Approved.Steps {
			found = found || old.ID == "s02"
		}
	}
	if !found {
		t.Fatal("old unfinished execution missing from history")
	}
}

func TestExecutionPlanReopenHistoryPendingBinding(t *testing.T) {
	s, st := executionAt(t, "tdd")
	if err := s.persist(st); err != nil {
		t.Fatal(err)
	}
	st, err := s.Reopen(st.ID, st.Revision, st.CurrentStepID, "retry")
	if err != nil {
		t.Fatal(err)
	}
	d := st.ExecutionPlan.Draft
	p := PendingReopen{Revision: st.Revision, From: st.Stage, To: st.Stage, Reason: d.Reason, At: "2026-09-09T00:00:00Z", LogHash: strings.Repeat("a", 64), LogAfterHash: strings.Repeat("b", 64), StepID: d.ReopenStepID, PlanRevision: d.Revision, PlanHash: PlanHash(*d), HadLog: true}
	st.PendingReopen = &p
	if err = s.persist(st); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"step", "revision", "hash", "reason"} {
		t.Run(field, func(t *testing.T) {
			bad := p
			switch field {
			case "step":
				bad.StepID = "s01"
			case "revision":
				bad.PlanRevision++
			case "hash":
				bad.PlanHash = strings.Repeat("c", 64)
			case "reason":
				bad.Reason = "other"
			}
			candidate := st
			candidate.PendingReopen = &bad
			if err := s.persist(candidate); err == nil {
				t.Fatal("pending detached from requested plan accepted")
			}
		})
	}
}
