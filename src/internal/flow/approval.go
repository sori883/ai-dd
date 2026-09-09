package flow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

type Approval struct {
	RequestID      string `json:"request_id"`
	Target         string `json:"target"`
	StepID         string `json:"step_id"`
	PlanRevision   uint64 `json:"plan_revision"`
	PlanHash       string `json:"plan_hash"`
	DefinitionHash string `json:"definition_hash"`
	Status         string `json:"status"`
	Session        string `json:"session,omitempty"`
	Turn           string `json:"turn,omitempty"`
	Quote          string `json:"quote,omitempty"`
	PromptHash     string `json:"prompt_hash,omitempty"`
	At             string `json:"at"`
}
type ApprovalDecision struct {
	RequestID string `json:"request_id"`
	Target    string `json:"target"`
	Decision  string `json:"decision"`
	Session   string `json:"session"`
	Turn      string `json:"turn"`
	Quote     string `json:"quote"`
}

func (s Store) DecidePlan(id string, expect uint64, r ApprovalDecision) (State, error) {
	return s.changePlanDecision(id, expect, r, func(st *State) error {
		draft := st.ExecutionPlan.Draft
		if draft == nil {
			return invalid("no pending plan")
		}
		if err := s.decideApproval(*st, draft.Approval, r); err != nil {
			return err
		}
		if r.Decision == "reject" {
			st.Approval = nil
			st.Review = Gate{}
			st.Sensor = Gate{}
			return rejectDraft(st)
		}
		if draft.Approval.Target != PlanHash(*draft) {
			return invalid("plan target changed")
		}
		for i := range draft.Steps {
			for _, old := range executionSteps(*st) {
				if old.ID == draft.Steps[i].ID {
					draft.Steps[i].Status = old.Status
				}
			}
		}
		if draft.ReopenStepID != "" {
			if err := s.recordPlanReopen(st, *draft); err != nil {
				return err
			}
		}
		preserve := st.Stage == "discovery" && st.ExecutionPlan.Approved == nil
		if preserve && st.Approval != nil && s.resultTarget(*st) != nil {
			preserve = false
		}
		if !preserve {
			st.Entry = nil
			st.Config.Units = nil
			st.Config.TestResults = nil
			st.Config.DirectCommit = ""
			st.Approval = nil
			st.Review = Gate{}
			st.Sensor = Gate{}
			for i := range draft.Steps {
				if draft.Steps[i].Status != "completed" {
					draft.Steps[i].Status = "pending"
				}
			}
		}
		st.PendingReopen = nil
		st.ExecutionPlan.Approved = draft
		st.ExecutionPlan.Revision = draft.Revision
		st.ExecutionPlan.Draft = nil
		st.ExecutionPlan.Bootstrap = nil
		for _, step := range draft.Steps {
			if step.Status != "completed" {
				st.CurrentStepID = step.ID
				st.Stage = step.Stage
				st.Status = "active"
				break
			}
		}
		return nil
	})
}
func (s Store) Decide(id string, expect uint64, r ApprovalDecision) (State, error) {
	return s.change(id, expect, func(st *State) error {
		if err := s.resultTarget(*st); err != nil {
			return err
		}
		if err := s.decideApproval(*st, st.Approval, r); err != nil {
			return err
		}
		if r.Decision == "reject" {
			st.Review = Gate{}
			st.Sensor = Gate{}
			currentExecution(st).Status = "active"
		}
		return nil
	})
}
func evidencePlan(st State) (uint64, string) {
	if st.Stage == "discovery" && st.ExecutionPlan.Approved == nil && st.ExecutionPlan.Draft != nil {
		return st.ExecutionPlan.Draft.Revision, PlanHash(*st.ExecutionPlan.Draft)
	}
	if p := st.ExecutionPlan.Approved; p != nil {
		return p.Revision, PlanHash(*p)
	}
	return 0, ""
}
func (s Store) resultTarget(st State) error {
	a := st.Approval
	if a == nil {
		return invalid("result approval required")
	}
	revision, hash := evidencePlan(st)
	gate, err := s.checkState(st)
	if err != nil {
		return err
	}
	if gate.Status != "pass" || gate.Target != a.Target || st.Review.Status != "pass" || st.Review.Target != gate.Target || st.Review.StepID != st.CurrentStepID || a.StepID != st.CurrentStepID || a.PlanRevision != revision || a.PlanHash != hash || a.DefinitionHash != st.DefinitionHash {
		return invalid("result approval target changed")
	}
	return nil
}

type approvalSource struct {
	Session  string   `json:"session"`
	Turn     string   `json:"turn"`
	Prompt   string   `json:"prompt"`
	Requests []string `json:"requests"`
	Seen     []string `json:"seen"`
}

func (s Store) approvalSourcePath(id string) string {
	return "aidlc/.runtime/flow/approvals/" + s.Space + "-" + id + ".json"
}
func (s Store) CaptureApproval(id, session, previousTurn, turn, prompt string) error {
	release, err := filestore.Lock(s.Root, "flow-"+s.Space)
	if err != nil {
		return err
	}
	defer release()
	st, err := s.Read(id)
	if err != nil {
		return err
	}
	if session == "" || turn == "" || strings.TrimSpace(prompt) == "" {
		return nil
	}
	if len(prompt) > 64*1024 || !utf8.ValidString(prompt) {
		return invalid("approval prompt exceeds limit")
	}
	source := approvalSource{}
	raw, err := filestore.ReadFile(s.Root, s.approvalSourcePath(id))
	if err == nil {
		if err = json.Unmarshal(raw, &source); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	key := filestore.Hash([]byte(session + "\x00" + turn))
	for _, seen := range source.Seen {
		if seen == key {
			if source.Session == session && source.Turn == turn && source.Prompt == prompt {
				return nil
			}
			return invalid("replayed approval turn")
		}
	}
	source.Seen = append(source.Seen, key)
	source.Session = session
	source.Turn = turn
	source.Prompt = prompt
	source.Requests = nil
	for _, a := range pendingApprovals(st) {
		source.Requests = append(source.Requests, a.RequestID)
	}
	raw, err = json.Marshal(source)
	if err != nil {
		return err
	}
	if len(raw) > filestore.MaxBytes {
		return invalid("approval source exceeds 256 KiB")
	}
	return filestore.WriteFile(s.Root, s.approvalSourcePath(id), raw)
}
func pendingApprovals(st State) []*Approval {
	out := []*Approval{}
	if st.Approval != nil && st.Approval.Status == "pending" {
		out = append(out, st.Approval)
	}
	if d := st.ExecutionPlan.Draft; d != nil && d.Approval != nil && d.Approval.Status == "pending" {
		out = append(out, d.Approval)
	}
	return out
}
func newApproval(st State, target string, revision uint64, planHash string) (*Approval, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	return &Approval{RequestID: hex.EncodeToString(id), Target: target, StepID: st.CurrentStepID, PlanRevision: revision, PlanHash: planHash, DefinitionHash: st.DefinitionHash, Status: "pending", At: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
func (s Store) decideApproval(st State, a *Approval, r ApprovalDecision) error {
	if a == nil || a.Status != "pending" || a.RequestID != r.RequestID || a.Target != r.Target || a.StepID != st.CurrentStepID || a.DefinitionHash != st.DefinitionHash {
		return invalid("approval request or target mismatch")
	}
	if r.Decision != "approve" && r.Decision != "reject" {
		return invalid("invalid approval decision")
	}
	if r.Session == "" || r.Turn == "" || strings.TrimSpace(r.Quote) == "" || len(r.Quote) > 8192 || !utf8.ValidString(r.Quote) {
		return invalid("invalid approval source")
	}
	raw, err := filestore.ReadFile(s.Root, s.approvalSourcePath(st.ID))
	if err != nil {
		return err
	}
	var source approvalSource
	if err = json.Unmarshal(raw, &source); err != nil {
		return err
	}
	found := false
	for _, id := range source.Requests {
		found = found || id == a.RequestID
	}
	if !found || source.Session != r.Session || source.Turn != r.Turn || !strings.Contains(source.Prompt, r.Quote) {
		return invalid("answer was not captured for this request")
	}
	a.Status = "approved"
	if r.Decision == "reject" {
		a.Status = "rejected"
	}
	a.Session = r.Session
	a.Turn = r.Turn
	a.Quote = r.Quote
	a.PromptHash = filestore.Hash([]byte(source.Prompt))
	a.At = time.Now().UTC().Format(time.RFC3339Nano)
	return nil
}
func (s Store) Finish(id string, expect uint64) (State, error) {
	return s.change(id, expect, func(st *State) error {
		if st.Status != "active" || st.Approval == nil || st.Approval.Status != "approved" {
			return invalid("human result approval required")
		}
		if st.ExecutionPlan.Draft != nil {
			return invalid("plan approval required")
		}
		if st.Stage == "discovery" && st.ExecutionPlan.Approved == nil {
			return invalid("stage choices require plan approval")
		}
		gate, c, err := s.checkStateSnapshot(*st)
		if err != nil {
			return err
		}
		revision, hash := evidencePlan(*st)
		if gate.Status != "pass" || st.Review.Status != "pass" || st.Review.StepID != st.CurrentStepID || st.Review.Target != gate.Target || st.Approval.StepID != st.CurrentStepID || st.Approval.PlanRevision != revision || st.Approval.PlanHash != hash || st.Approval.DefinitionHash != st.DefinitionHash || gate.Target != st.Approval.Target {
			return invalid("result changed")
		}
		if st.Accepted == nil {
			st.Accepted = map[string]StageAcceptance{}
		}
		st.Accepted[st.CurrentStepID] = StageAcceptance{StepID: st.CurrentStepID, Stage: st.Stage, ReviewTarget: gate.Target, Outputs: append([]FileVersion{}, c.proof...)}
		currentExecution(st).Status = "completed"
		next := ""
		for _, step := range executionSteps(*st) {
			if step.Status != "completed" {
				next = step.ID
				st.Stage = step.Stage
				break
			}
		}
		if next == "" {
			st.Status = "completed"
		} else {
			st.CurrentStepID = next
		}
		st.Config.Units = nil
		st.Config.TestResults = nil
		st.Config.DirectCommit = ""
		st.Entry = nil
		st.Sensor = Gate{}
		st.Review = Gate{}
		st.Approval = nil
		return nil
	})
}

func validateApproval(a *Approval, st State) error {
	if a == nil {
		return nil
	}
	if !validID(a.RequestID) || !validHash(a.Target) || !validHash(a.DefinitionHash) || !validAllocatedStep(st, a.StepID) {
		return invalid("invalid approval target")
	}
	if a.PlanRevision == 0 && a.PlanHash != "" || a.PlanRevision > 0 && !validHash(a.PlanHash) {
		return invalid("invalid approval plan binding")
	}
	if _, err := time.Parse(time.RFC3339Nano, a.At); err != nil {
		return invalid("invalid approval timestamp")
	}
	switch a.Status {
	case "pending":
		if a.Session != "" || a.Turn != "" || a.Quote != "" || a.PromptHash != "" {
			return invalid("pending approval contains answer")
		}
	case "approved", "rejected":
		if a.Session == "" || a.Turn == "" || strings.TrimSpace(a.Quote) == "" || len(a.Quote) > 8192 || !utf8.ValidString(a.Quote) || !validHash(a.PromptHash) {
			return invalid("invalid approval answer")
		}
	default:
		return invalid("invalid approval status")
	}
	return nil
}
func validHash(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

func validAllocatedStep(st State, id string) bool {
	if executionStage(st, id) != "" {
		return true
	}
	return st.ExecutionPlan.Allocated[id] != ""
}
