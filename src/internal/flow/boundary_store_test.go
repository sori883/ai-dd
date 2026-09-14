package flow

import (
	"strings"
	"testing"
)

func TestBoundaryStoreVersions(t *testing.T) {
	for _, mode := range []string{"valid", "stage", "input path", "hash", "source runtime", "duplicate input"} {
		t.Run(mode, func(t *testing.T) {
			s := flowStore(t)
			st := schemaPlanState(s)
			st.Entry = &StageEntry{Stage: st.Stage, StepID: st.CurrentStepID, Inputs: []FileVersion{{Path: "file", SHA256: strings.Repeat("a", 64)}}, Sources: []FileVersion{{Path: "source", SHA256: strings.Repeat("b", 64)}}}
			switch mode {
			case "stage":
				st.Entry.Stage = "planning"
			case "input path":
				st.Entry.Inputs[0].Path = "../escape"
			case "hash":
				st.Entry.Inputs[0].SHA256 = "bad"
			case "source runtime":
				st.Entry.Sources[0].Path = "aidlc/.runtime/x"
			case "duplicate input":
				st.Entry.Inputs = append(st.Entry.Inputs, st.Entry.Inputs[0])
			}
			if err := s.persist(st); (err == nil) != (mode == "valid") {
				t.Fatalf("entry %s: %v", mode, err)
			}
		})
	}
}
