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
}
