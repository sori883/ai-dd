package flow

import (
	"strings"
	"testing"
)

func TestStagePlannerProcedure(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "planner procedure")
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.Procedure(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, a := range view.Procedure.Agents {
		names = append(names, a.Role+":"+a.Agent)
	}
	want := "requirements:aidlc-requirements,research:aidlc-researcher,execution_planning:aidlc-stage-planner,independent_review:aidlc-reviewer"
	if strings.Join(names, ",") != want {
		t.Errorf("roles/order=%v", names)
	}
	body := view.Procedure.Text
	for _, want := range []string{"要件整理と調査の後", "aidlc-stage-planner", "PLAN.json", "ユーザー", "計画承認と成果承認"} {
		if !strings.Contains(body, want) {
			t.Errorf("procedure missing %s", want)
		}
	}
}
