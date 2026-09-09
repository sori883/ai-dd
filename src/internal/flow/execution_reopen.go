package flow

import (
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"strings"
	"time"
)

func (s Store) Reopen(id string, expect uint64, stepID, reason string) (State, error) {
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if st.ExecutionPlan.Approved == nil || strings.TrimSpace(reason) == "" {
		return State{}, invalid("approved plan and reopen reason required")
	}
	target := -1
	steps := executionSteps(st)
	for i, step := range steps {
		if step.ID == stepID {
			target = i
		}
	}
	if target < 0 {
		return State{}, invalid("known execution required for reopen")
	}
	request := PlanRequest{Reason: reason, ReopenStepID: stepID, Omitted: st.ExecutionPlan.Approved.Omitted}
	for _, step := range steps {
		if step.Status == "completed" {
			request.Steps = append(request.Steps, PlanStepInput{ID: step.ID, Stage: step.Stage})
		}
	}
	for _, step := range steps[target:] {
		if step.Status == "completed" {
			request.Steps = append(request.Steps, PlanStepInput{Stage: step.Stage})
		} else if step.ID == stepID {
			request.Steps = append(request.Steps, PlanStepInput{Stage: step.Stage})
		} else {
			request.Steps = append(request.Steps, PlanStepInput{ID: step.ID, Stage: step.Stage})
		}
	}
	return s.ProposePlan(id, expect, request)
}
func (s Store) recordPlanReopen(st *State, plan PlanVersion) error {
	name := "aidlc/spaces/" + s.Space + "/knowledge/log/" + st.ID + "-work-log.md"
	raw, readErr := readWorkLog(s.Root, name)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	write := s.write
	if write == nil {
		write = filestore.WriteFile
	}
	if st.PendingReopen == nil {
		pending := &PendingReopen{Revision: st.Revision, From: st.Stage, To: executionStage(*st, plan.ReopenStepID), Reason: plan.Reason, At: time.Now().UTC().Format(time.RFC3339Nano), HadLog: true, StepID: plan.ReopenStepID, PlanRevision: plan.Revision, PlanHash: PlanHash(plan)}
		var err error
		if os.IsNotExist(readErr) {
			raw, err = workLogBytes(*st, pending.At)
			if err != nil {
				return err
			}
		}
		pending.LogHash = logHash(raw)
		after, err := appendWorkLog(*st, raw, *pending)
		if err != nil {
			return err
		}
		pending.LogAfterHash = logHash(after)
		if os.IsNotExist(readErr) {
			if err = write(s.Root, name, raw); err != nil {
				return err
			}
			readErr = nil
		}
		base, err := s.Read(st.ID)
		if err != nil {
			return err
		}
		base.PendingReopen = pending
		if err = s.persist(base); err != nil {
			return err
		}
		st.PendingReopen = pending
	}
	pending := st.PendingReopen
	if pending.PlanRevision != plan.Revision || pending.PlanHash != PlanHash(plan) || pending.StepID != plan.ReopenStepID || pending.Reason != plan.Reason {
		return invalid("different reopen pending")
	}
	if readErr != nil {
		return invalid("work-log missing; restore recorded version")
	}
	if logHash(raw) == pending.LogAfterHash {
		return nil
	}
	if logHash(raw) != pending.LogHash {
		return invalid("work-log changed; restore recorded version")
	}
	after, err := appendWorkLog(*st, raw, *pending)
	if err != nil {
		return err
	}
	if logHash(after) != pending.LogAfterHash {
		return invalid("work-log after hash mismatch")
	}
	return write(s.Root, name, after)
}

func (s Store) changePlanDecision(id string, expect uint64, r ApprovalDecision, fn func(*State) error) (State, error) {
	if err := s.check(); err != nil {
		return State{}, err
	}
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return State{}, err
	}
	defer release()
	st, err := s.Read(id)
	if err != nil {
		return State{}, err
	}
	if st.Revision != expect || expect == ^uint64(0) {
		return State{}, invalid("revision conflict")
	}
	if err = s.verifyHistoryHead(st); err != nil {
		return State{}, err
	}
	if st.PendingReopen == nil {
		if err = s.guardReassignment(st, nil); err != nil {
			return State{}, err
		}
	} else {
		d := st.ExecutionPlan.Draft
		if d == nil || d.Approval == nil || r.Decision != "approve" || r.RequestID != d.Approval.RequestID || r.Target != d.Approval.Target || st.PendingReopen.PlanHash != PlanHash(*d) {
			return State{}, invalid("different reopen decision pending")
		}
	}
	if err = fn(&st); err != nil {
		return State{}, err
	}
	st.Revision++
	if err = s.commit(&st); err != nil {
		return State{}, err
	}
	return st, nil
}
