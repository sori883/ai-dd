package app

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestExecutionPlanReviewReopenPublicPlan(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(map[bool]string{false: "log only", true: "late initialization"}[extra], func(t *testing.T) {
			s, st := setup(t)
			r := flow.PlanRequest{Reason: "reinitialize", ReopenStepID: "s01", Omitted: st.ExecutionPlan.Approved.Omitted}
			for _, step := range st.ExecutionPlan.Approved.Steps {
				r.Steps = append(r.Steps, flow.PlanStepInput{ID: step.ID, Stage: step.Stage})
			}
			if extra {
				r.Steps = append(r.Steps, flow.PlanStepInput{Stage: "initialization"})
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(s.Root, "plan.json")
			if err = os.WriteFile(p, raw, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = s.Execute(cli.CommandRequest{Command: "intent", Action: "plan", Space: "default", Target: st.ID, Expect: strconv.FormatUint(st.Revision, 10), File: p})
			if err == nil {
				t.Fatal("public PLAN accepted invalid reopen")
			}
		})
	}
}
