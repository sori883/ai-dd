package main

import (
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/flow"
	"strings"
	"testing"
)

type flowProofTest struct {
	Name, TestHash, SourceHash string
	Exit                       int
	Ran                        bool
}
type flowProofJob struct {
	CodeHash                                                   string
	Kind, Unit, Root, Session, Target, Status, Summary, Commit string
	Start, End                                                 int64
	Tests                                                      []flowProofTest
}
type flowCLIProof struct {
	Session, Command, ItemID, Output string
	Exit                             int
	Request                          flow.ReviewRequest
	Before                           uint64
}
type flowProof struct {
	Binary        string
	CLI           []flowCLIProof
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

	accepted := map[string]bool{}
	staleRejected := false
	seenItems := map[string]bool{}
	for _, execution := range p.CLI {
		request, _, ok := flowReviewCommand(p.Binary, execution.Command)
		if !ok || request.Target != id || execution.ItemID == "" || execution.Session == "" {
			return fail()
		}
		key := execution.Session + "/" + execution.ItemID
		if seenItems[key] {
			return fail()
		}
		seenItems[key] = true
		matched := false
		for _, job := range p.Jobs {
			r := execution.Request
			if job.Kind == "review" && r.Action == "accept" && r.Session == job.Session && r.Root == job.Root && r.Target == job.Target && r.Status == job.Status && r.Summary == job.Summary {
				matched = true
			}
		}
		if !matched {
			return fail()
		}
		if execution.Exit == 2 && strings.TrimSpace(execution.Output) == "aidlc: unassigned or stale review: invalid argument" {
			staleRejected = true
			continue
		}
		if execution.Exit != 0 {
			return fail()
		}
		var saved flow.State
		if json.Unmarshal([]byte(execution.Output), &saved) != nil || saved.ID != id || saved.Revision != execution.Before+1 || saved.Review.Target != execution.Request.Target || saved.Review.Status != execution.Request.Status || saved.Review.Summary != execution.Request.Summary {
			return fail()
		}
		accepted[execution.Request.Session+"/"+execution.Request.Target] = true
	}
	if !staleRejected {
		return fail()
	}
	reviewFail, reviewPass := false, false
	workers := map[string]flowProofJob{}
	for _, j := range p.Jobs {
		if j.Kind == "review" {
			if j.Root == "" || j.Session == "" || j.Target == "" || j.Summary == "" || len(j.Commit) != 40 || j.CodeHash == "" {
				return fail()
			}
			if accepted[j.Session+"/"+j.Target] && j.Status == "fail" {
				reviewFail = true
			}
			if reviewFail && accepted[j.Session+"/"+j.Target] && j.Status == "pass" {
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
	p := flowProof{Binary: "/bin/aidlc", Actions: []string{"intent create", "intent configure", "intent advance", "intent advance", "intent advance", "intent advance", "unit claim", "unit claim", "unit claim", "unit result", "unit result", "unit result", "unit integrate", "unit integrate", "unit integrate"}, StaleRejected: true, Sessions: []string{"one", "two"}}
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

	for i := 0; i < 2; i++ {
		p.Jobs[i].Commit = strings.Repeat("c", 40)
		p.Jobs[i].CodeHash = "fixed-code"
	}
	for _, job := range p.Jobs[:2] {
		request := flow.ReviewRequest{Action: "accept", Session: job.Session, Root: job.Root, Target: job.Target, Status: job.Status, Summary: job.Summary}
		raw, _ := json.Marshal(flow.State{ID: "same", Revision: 2, Review: flow.Gate{Target: job.Target, Status: job.Status, Summary: job.Summary}})
		p.CLI = append(p.CLI, flowCLIProof{Session: "one", Command: "/bin/aidlc intent review same --space default --expect 1 --file /draft", ItemID: job.Target, Request: request, Before: 1, Output: string(raw)})
	}
	stale := p.CLI[1]
	stale.ItemID = "stale"
	stale.Exit = 2
	stale.Output = "aidlc: unassigned or stale review: invalid argument\n"
	p.CLI = append(p.CLI, stale)
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

func TestFlowCommandRejectsInventedReviewEvidence(t *testing.T) {
	for _, change := range []func(*flowProof){func(p *flowProof) { p.CLI = nil }, func(p *flowProof) { p.Jobs[0].CodeHash = "" }, func(p *flowProof) { p.CLI[2].Command = "echo unassigned or stale review" }, func(p *flowProof) { p.CLI[2].Exit = 0 }, func(p *flowProof) { p.CLI[0].Request.Summary = "fabricated" }, func(p *flowProof) { p.CLI[1].Request.Root = "other" }, func(p *flowProof) { p.CLI[1].Request.Target = "other" }} {
		p := validFlowProof()
		change(&p)
		if verifyFlowProof(p) == nil {
			t.Fatal("invented review evidence accepted")
		}
	}
}

func flowReviewCommand(binary, command string) (cli.MinimalRequest, string, bool) {
	args, ok := flowShellWords(command)
	if !ok {
		return cli.MinimalRequest{}, "", false
	}
	if len(args) == 3 && (args[0] == "/bin/zsh" || args[0] == "/bin/bash") && args[1] == "-lc" {
		args, ok = flowShellWords(args[2])
		if !ok {
			return cli.MinimalRequest{}, "", false
		}
	}
	if len(args) < 2 || args[0] != binary {
		return cli.MinimalRequest{}, "", false
	}
	r, err := cli.ParseMinimal(args[1:])
	return r, strings.Join(args, "\x00"), err == nil && r.Command == "intent" && r.Action == "review"
}

func flowShellWords(command string) ([]string, bool) {
	var words []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	started := false
	for _, r := range command {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '$' || r == '`' || r == '\n' || r == '\r' {
			return nil, false
		}
		if r == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if strings.ContainsRune(";|&<>()", r) {
			return nil, false
		}
		if r == ' ' || r == '\t' {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 || escaped {
		return nil, false
	}
	if started {
		words = append(words, word.String())
	}
	return words, true
}

type flowReviewObservation struct {
	Session, Command string
	Before           uint64
	Request          flow.ReviewRequest
}

func flowExecutions(raw []byte, observations []flowReviewObservation, binary string) ([]flowCLIProof, error) {
	queues := map[string][]flowReviewObservation{}
	for _, o := range observations {
		_, key, ok := flowReviewCommand(binary, o.Command)
		if ok {
			queues[o.Session+"/"+key] = append(queues[o.Session+"/"+key], o)
		}
	}
	var out []flowCLIProof
	session := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Session string `json:"thread_id"`
			Item    struct {
				ID, Type, Command string
				Exit              *int   `json:"exit_code"`
				Output            string `json:"aggregated_output"`
			} `json:"item"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		if event.Type == "thread.started" {
			session = event.Session
		}
		if event.Type != "item.completed" || event.Item.Type != "command_execution" || event.Item.Exit == nil {
			continue
		}
		_, key, ok := flowReviewCommand(binary, event.Item.Command)
		if !ok {
			continue
		}
		key = session + "/" + key
		queue := queues[key]
		if len(queue) == 0 {
			return nil, fmt.Errorf("CLI review transport has no matching Pre request")
		}
		observation := queue[0]
		queues[key] = queue[1:]
		if observation.Request.Action != "accept" {
			continue
		}
		out = append(out, flowCLIProof{Session: session, Command: event.Item.Command, ItemID: event.Item.ID, Exit: *event.Item.Exit, Output: event.Item.Output, Before: observation.Before, Request: observation.Request})
	}
	return out, nil
}
func TestFlowCommandTransportReviewEvidence(t *testing.T) {
	r := flow.ReviewRequest{Action: "accept", Session: "reviewer", Root: "/review", Target: "target", Status: "pass", Summary: "actual report"}
	command := "/bin/aidlc intent review same --space default --expect 1 --file /draft"
	wire := `{"type":"thread.started","thread_id":"coordinator"}` + "\n" + `{"type":"item.completed","item":{"id":"item-1","type":"command_execution","command":"/bin/zsh -lc '/bin/aidlc intent review same --space default --expect 1 --file /draft'","exit_code":2,"aggregated_output":"aidlc: unassigned or stale review: invalid argument\n"}}` + "\n" + `{"type":"item.completed","item":{"id":"echo","type":"command_execution","command":"/bin/zsh -lc 'echo unassigned or stale review'","exit_code":0,"aggregated_output":"unassigned or stale review"}}`
	got, err := flowExecutions([]byte(wire), []flowReviewObservation{{Session: "coordinator", Command: command, Before: 1, Request: r}}, "/bin/aidlc")
	if err != nil || len(got) != 1 || got[0].Exit != 2 || got[0].Request.Target != "target" || got[0].ItemID != "item-1" {
		t.Fatalf("transport proof %+v %v", got, err)
	}
}
