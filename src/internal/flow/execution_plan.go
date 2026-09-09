package flow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExecutionStep identifies one run, including repeated runs of the same stage.
type ExecutionStep struct {
	Reopens string `json:"reopens,omitempty"`
	ID      string `json:"id"`
	Stage   string `json:"stage"`
	Status  string `json:"status"`
}

type StageOmission struct {
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

type PlanVersion struct {
	Approval     *Approval       `json:"approval"`
	Revision     uint64          `json:"revision"`
	Reason       string          `json:"reason"`
	Steps        []ExecutionStep `json:"steps"`
	Omitted      []StageOmission `json:"omitted"`
	ReopenStepID string          `json:"reopen_step_id,omitempty"`
}

// ExecutionPlan keeps proposed changes separate from the effective execution order.
type ExecutionPlan struct {
	Allocated map[string]string `json:"allocated"`
	Revision  uint64            `json:"revision"`
	Approved  *PlanVersion      `json:"approved"`
	Draft     *PlanVersion      `json:"draft"`
	Bootstrap []ExecutionStep   `json:"bootstrap,omitempty"`
	NextID    uint64            `json:"next_id"`
}

func stepNumber(id string) (uint64, error) {
	if !strings.HasPrefix(id, "s") {
		return 0, invalid("invalid step id")
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(id, "s"), 10, 64)
	if err != nil || n == 0 || id != fmt.Sprintf("s%02d", n) {
		return 0, invalid("invalid step id")
	}
	return n, nil
}

func executionSteps(st State) []ExecutionStep {
	if st.ExecutionPlan.Approved != nil {
		return st.ExecutionPlan.Approved.Steps
	}
	return st.ExecutionPlan.Bootstrap
}

func validateExecutionPlan(st State) error {
	if p := st.ExecutionPlan.Approved; p != nil {
		a := p.Approval
		if a == nil || a.Status != "approved" || a.Target != PlanHash(*p) || a.PlanHash != PlanHash(*p) || a.PlanRevision != p.Revision || a.Session == "" || a.Turn == "" || strings.TrimSpace(a.Quote) == "" || len(a.PromptHash) != 64 || a.DefinitionHash != st.DefinitionHash {
			return invalid("approved plan evidence required")
		}
	}

	p := st.ExecutionPlan
	if p.NextID < 3 {
		return invalid("invalid next step id")
	}
	validate := func(steps []ExecutionStep, omitted []StageOmission, bootstrap bool) error {
		if len(steps) < 2 || steps[0].Stage != "initialization" || (steps[0].ID != "s01" && steps[0].Reopens == "") {
			return invalid("mandatory prefix required")
		}
		mandatory := false
		for i := 1; i < len(steps); i++ {
			step := steps[i]
			if step.Stage == "discovery" && (step.ID == "s02" || step.Reopens != "") {
				mandatory = true
				break
			}
			if step.Stage != "initialization" || step.Reopens == "" {
				return invalid("invalid mandatory prefix insertion")
			}
			origin := false
			for _, earlier := range steps[:i] {
				if earlier.ID == step.Reopens && earlier.Stage == "initialization" && earlier.Status == "completed" {
					origin = true
				}
			}
			if !origin && p.Allocated[step.Reopens] == "initialization" {
				origin = true
			}
			if !origin {
				return invalid("invalid initialization replay origin")
			}
		}
		if !mandatory {
			return invalid("mandatory discovery required")
		}
		if bootstrap && len(steps) != 2 {
			return invalid("unapproved optional execution")
		}
		seen, stages := map[string]bool{}, map[string]bool{}
		pending := false
		for _, step := range steps {
			n, err := stepNumber(step.ID)
			if err != nil || n >= p.NextID || seen[step.ID] || !supportedStage(step.Stage) {
				return invalid("invalid execution step")
			}
			if step.Reopens != "" {
				origin, err := stepNumber(step.Reopens)
				if err != nil || origin >= n || p.Allocated[step.Reopens] != step.Stage {
					return invalid("invalid execution reopen origin")
				}
			}
			seen[step.ID] = true
			stages[step.Stage] = true
			switch step.Status {
			case "completed":
				if pending {
					return invalid("completed steps must form a prefix")
				}
			case "pending":
				pending = true
			case "active", "awaiting_approval":
				if pending {
					return invalid("execution out of order")
				}
				pending = true
			default:
				return invalid("invalid execution status")
			}
		}
		if bootstrap {
			return nil
		}
		omittedStages := map[string]bool{}
		for _, o := range omitted {
			optional := o.Stage == "architecture-analysis" || o.Stage == "planning" || o.Stage == "tdd" || o.Stage == "integration"
			if !optional || strings.TrimSpace(o.Reason) == "" || stages[o.Stage] || omittedStages[o.Stage] {
				return invalid("invalid stage omission")
			}
			omittedStages[o.Stage] = true
		}
		for _, stage := range []string{"architecture-analysis", "planning", "tdd", "integration"} {
			if !stages[stage] && !omittedStages[stage] {
				return invalid("stage choice missing")
			}
		}
		return nil
	}
	if p.Approved == nil {
		if p.Revision != 0 {
			return invalid("unapproved plan revision")
		}
		if err := validate(p.Bootstrap, nil, true); err != nil {
			return err
		}
	} else {
		if p.Revision == 0 || p.Approved.Revision != p.Revision || len(p.Bootstrap) != 0 || strings.TrimSpace(p.Approved.Reason) == "" {
			return invalid("invalid approved plan")
		}
		if err := validate(p.Approved.Steps, p.Approved.Omitted, false); err != nil {
			return err
		}
	}
	if p.Draft != nil {
		if p.Revision == ^uint64(0) || p.Draft.Revision != p.Revision+1 || strings.TrimSpace(p.Draft.Reason) == "" {
			return invalid("invalid draft revision")
		}
		if err := validate(p.Draft.Steps, p.Draft.Omitted, false); err != nil {
			return err
		}
		current := map[string]string{}
		for _, step := range executionSteps(st) {
			current[step.ID] = step.Stage
		}
		for _, step := range p.Draft.Steps {
			if stage, ok := current[step.ID]; ok && stage != step.Stage {
				return invalid("step stage changed")
			}
		}
	}
	steps := executionSteps(st)
	current := steps[len(steps)-1]
	for _, step := range steps {
		if step.Status != "completed" {
			current = step
			break
		}
	}
	if st.CurrentStepID != current.ID || st.Stage != current.Stage {
		return invalid("current step mismatch")
	}
	return nil
}

func currentExecution(st *State) *ExecutionStep {
	steps := st.ExecutionPlan.Bootstrap
	if st.ExecutionPlan.Approved != nil {
		steps = st.ExecutionPlan.Approved.Steps
	}
	for i := range steps {
		if steps[i].ID == st.CurrentStepID {
			return &steps[i]
		}
	}
	return nil
}

type PlanStepInput struct {
	ID    string `json:"id,omitempty"`
	Stage string `json:"stage"`
}

type PlanRequest struct {
	Reason       string          `json:"reason"`
	Steps        []PlanStepInput `json:"steps"`
	Omitted      []StageOmission `json:"omitted"`
	ReopenStepID string          `json:"reopen_step_id,omitempty"`
}

// ProposePlan stores a candidate without changing the approved execution order.
func (s Store) ProposePlan(id string, expect uint64, request PlanRequest) (State, error) {
	return s.change(id, expect, func(st *State) error {
		for _, unit := range st.Config.Units {
			if unit.Status == "running" || unit.Status == "needs_confirmation" {
				return invalid("confirm worker termination before changing plan")
			}
		}
		if st.ExecutionPlan.Revision == ^uint64(0) {
			return invalid("plan revision exhausted")
		}
		if st.ExecutionPlan.Allocated == nil {
			st.ExecutionPlan.Allocated = map[string]string{}
		}
		known := map[string]ExecutionStep{}
		current := executionSteps(*st)
		for _, step := range current {
			known[step.ID] = step
			st.ExecutionPlan.Allocated[step.ID] = step.Stage
		}
		if st.ExecutionPlan.Draft != nil {
			for _, step := range st.ExecutionPlan.Draft.Steps {
				known[step.ID] = step
				st.ExecutionPlan.Allocated[step.ID] = step.Stage
			}
		}
		draft := PlanVersion{Revision: st.ExecutionPlan.Revision + 1, Reason: request.Reason, Omitted: append([]StageOmission{}, request.Omitted...), ReopenStepID: request.ReopenStepID}
		for _, input := range request.Steps {
			step := ExecutionStep{Stage: input.Stage, Status: "pending"}
			if input.ID != "" {
				previous, ok := known[input.ID]
				if !ok || previous.Stage != input.Stage {
					return invalid("unknown or changed execution id")
				}
				step = previous
			} else {
				if st.ExecutionPlan.NextID == ^uint64(0) {
					return invalid("step id exhausted")
				}
				step.ID = fmt.Sprintf("s%02d", st.ExecutionPlan.NextID)
				if request.ReopenStepID != "" && step.Stage == known[request.ReopenStepID].Stage {
					step.Reopens = request.ReopenStepID
				}
				st.ExecutionPlan.NextID++
				st.ExecutionPlan.Allocated[step.ID] = step.Stage
			}
			if request.ReopenStepID != "" && step.Status != "completed" {
				step.Status = "pending"
			}
			draft.Steps = append(draft.Steps, step)
		}
		for i, step := range current {
			if step.Status != "completed" {
				break
			}
			if len(draft.Steps) <= i || draft.Steps[i] != step {
				return invalid("completed execution prefix must be preserved")
			}
		}
		a, err := newApproval(*st, PlanHash(draft), draft.Revision, PlanHash(draft))
		if err != nil {
			return err
		}
		draft.Approval = a
		st.ExecutionPlan.Draft = &draft
		st.Approval = nil
		st.Review = Gate{}
		st.Sensor = Gate{}
		if currentExecution(st).Status == "awaiting_approval" {
			currentExecution(st).Status = "active"
		}
		return validateExecutionPlan(*st)
	})
}

// rejectDraft discards only the proposal; allocated run identities stay reserved.
// Callers must verify the corresponding human decision before persisting it.
func rejectDraft(st *State) error {
	if st.ExecutionPlan.Draft == nil {
		return invalid("no proposed plan")
	}
	st.ExecutionPlan.Draft = nil
	return nil
}

// PlanHash commits to plan content, excluding mutable execution progress.
func PlanHash(plan PlanVersion) string {
	steps := make([]ExecutionStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, ExecutionStep{ID: step.ID, Stage: step.Stage, Reopens: step.Reopens})
	}
	content := struct {
		Revision uint64
		Steps    []ExecutionStep
		Request  PlanRequest
	}{Revision: plan.Revision, Steps: steps, Request: PlanRequest{Reason: plan.Reason, Omitted: plan.Omitted, ReopenStepID: plan.ReopenStepID}}
	raw, _ := json.Marshal(content)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// precedingStep resolves the nearest earlier execution of a stage. Future and
// current executions cannot provide accepted input versions.
func precedingStep(st State, stage string) string {
	found := ""
	for _, step := range executionSteps(st) {
		if step.ID == st.CurrentStepID {
			break
		}
		if step.Stage == stage && step.Status == "completed" {
			found = step.ID
		}
	}
	return found
}

func executionStage(st State, id string) string {
	for _, step := range executionSteps(st) {
		if step.ID == id {
			return step.Stage
		}
	}
	return ""
}
