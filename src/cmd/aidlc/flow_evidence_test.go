package main

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/flow"
	"testing"
)

type flowProofTest struct {
	Name, TestHash, SourceHash string
	Exit                       int
	Ran                        bool
}
type flowProofJob struct {
	Kind, Unit, Root, Session, Target, Status, Summary, Commit string
	Start, End                                                 int64
	Tests                                                      []flowProofTest
}
type flowProof struct {
	Actions       []string
	States        []flow.State
	Jobs          []flowProofJob
	StaleRejected bool
	Sessions      []string
}

func verifyFlowProof(p flowProof) error {
	fail := func() error { return fmt.Errorf("incomplete four-stage evidence") }
	counts := map[string]int{}
	for _, a := range p.Actions {
		counts[a]++
	}
	for action, minimum := range map[string]int{"intent create": 1, "intent configure": 1, "intent advance": 4, "unit claim": 3, "unit result": 3, "unit integrate": 3} {
		if counts[action] < minimum {
			return fail()
		}
	}
	stages := map[string]bool{}
	id := ""
	completed := false
	for _, st := range p.States {
		if id == "" {
			id = st.ID
		}
		if st.ID != id {
			return fail()
		}
		stages[st.Stage] = true
		completed = completed || st.Status == "completed"
	}
	if len(stages) != 4 || !completed || !p.StaleRejected || len(p.Sessions) < 2 || p.Sessions[0] == p.Sessions[1] {
		return fail()
	}
	reviewFail, reviewPass := false, false
	workers := map[string]flowProofJob{}
	for _, j := range p.Jobs {
		if j.Kind == "review" {
			if j.Root == "" || j.Session == "" || j.Target == "" || j.Summary == "" {
				return fail()
			}
			if j.Status == "fail" {
				reviewFail = true
			}
			if reviewFail && j.Status == "pass" {
				reviewPass = true
			}
			continue
		}
		if j.Kind != "worker" {
			continue
		}
		if j.Root == "" || j.Session == "" || j.Commit == "" || j.End <= j.Start {
			return fail()
		}
		red := map[string]flowProofTest{}
		pair := false
		for _, test := range j.Tests {
			if !test.Ran {
				continue
			}
			if test.Exit == 1 {
				red[test.Name] = test
			}
			if previous, ok := red[test.Name]; ok && test.Exit == 0 && test.TestHash != "" && previous.TestHash == test.TestHash && previous.SourceHash != test.SourceHash {
				pair = true
			}
		}
		if !pair {
			return fail()
		}
		workers[j.Unit] = j
	}
	a, b, c := workers["a"], workers["b"], workers["c"]
	if len(workers) != 3 || !reviewFail || !reviewPass || a.Root == b.Root || a.Session == b.Session || a.Start >= b.End || b.Start >= a.End || c.Start < a.End || c.Start < b.End {
		return fail()
	}
	return nil
}
func validFlowProof() flowProof {
	p := flowProof{Actions: []string{"intent create", "intent configure", "intent advance", "intent advance", "intent advance", "intent advance", "unit claim", "unit claim", "unit claim", "unit result", "unit result", "unit result", "unit integrate", "unit integrate", "unit integrate"}, StaleRejected: true, Sessions: []string{"one", "two"}}
	for _, stage := range []string{"discovery", "planning", "tdd", "integration"} {
		p.States = append(p.States, flow.State{ID: "same", Stage: stage, Status: "active"})
	}
	p.States = append(p.States, flow.State{ID: "same", Stage: "integration", Status: "completed"})
	p.Jobs = []flowProofJob{{Kind: "review", Root: "review-one", Session: "r1", Target: "hash", Status: "fail", Summary: "missing case"}, {Kind: "review", Root: "review-two", Session: "r2", Target: "hash2", Status: "pass", Summary: "fixed"}}
	for i, u := range []string{"a", "b", "c"} {
		start := int64(10)
		if i == 2 {
			start = 30
		}
		p.Jobs = append(p.Jobs, flowProofJob{Kind: "worker", Unit: u, Root: u, Session: u, Commit: u, Start: start, End: start + 10, Tests: []flowProofTest{{Name: "TestUnit", TestHash: "same", SourceHash: "red", Exit: 1, Ran: true}, {Name: "TestUnit", TestHash: "same", SourceHash: "green", Exit: 0, Ran: true}}})
	}
	return p
}
func TestFlowCommandEvidenceVerifier(t *testing.T) {
	if err := verifyFlowProof(validFlowProof()); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*flowProof){func(p *flowProof) { p.States = p.States[:4] }, func(p *flowProof) { p.StaleRejected = false }, func(p *flowProof) { p.Actions = nil }, func(p *flowProof) { p.Sessions = p.Sessions[:1] }, func(p *flowProof) { p.Jobs[0].Status = "pass" }, func(p *flowProof) { p.Jobs[2].Tests[0].Exit = 0 }, func(p *flowProof) { p.Jobs[2].Tests[1].Ran = false }, func(p *flowProof) { p.Jobs[2].Tests[1].TestHash = "changed" }, func(p *flowProof) { p.Jobs[3].Start = 21 }, func(p *flowProof) { p.Jobs[4].Start = 15 }} {
		p := validFlowProof()
		change(&p)
		if err := verifyFlowProof(p); err == nil {
			t.Fatal("incomplete live evidence accepted")
		}
	}
}
