package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// Raw is a string to preserve the exact input bytes, including unknown fields.
// Exit is the hook's selected exit status; the caller must separately observe it.
type agentProbeRecord struct {
	Raw        string    `json:"raw"`
	Response   string    `json:"response"`
	Exit       int       `json:"exit"`
	ObservedAt time.Time `json:"observed_at"`
}

type agentProbeInput struct {
	Event    string          `json:"hook_event_name"`
	Tool     string          `json:"tool_name"`
	Input    json.RawMessage `json:"tool_input"`
	Response json.RawMessage `json:"tool_response"`
	Session  string          `json:"session_id"`
	Turn     string          `json:"turn_id"`
	Call     string          `json:"tool_use_id"`
	Agent    string          `json:"agent_id"`
	CWD      string          `json:"cwd"`
}

func TestAgentHookProbeHelper(t *testing.T) {
	split := -1
	for i, arg := range os.Args {
		if arg == "--" {
			split = i
			break
		}
	}
	if split < 0 {
		t.Skip("hook subprocess only")
	}
	args := os.Args[split+1:]
	if len(args) != 3 || args[0] != "agent-hook" {
		fmt.Fprintln(os.Stderr, "invalid probe arguments")
		os.Exit(64)
	}
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, (2<<20)+1))
	var input agentProbeInput
	if err != nil || len(raw) > 2<<20 || json.Unmarshal(raw, &input) != nil {
		fmt.Fprintln(os.Stderr, "invalid hook input")
		os.Exit(65)
	}
	response := "{}"
	var spawn struct {
		Agent string `json:"agent_type"`
	}
	_ = json.Unmarshal(input.Input, &spawn)
	if args[2] == "deny" && input.Event == "PreToolUse" && input.Tool == "spawn_agent" && spawn.Agent == "probe_worker" {
		response = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Expected G0 probe denial. Do not retry or substitute another agent."}}`
	}
	exit := 0
	fault := input.Event == "PreToolUse" && input.Tool == "spawn_agent" && spawn.Agent == "probe_worker"
	if fault {
		switch args[2] {
		case "missing", "timeout":
			response = ""
		case "nonzero":
			response = ""
			exit = 2
		case "save-failure":
			response = ""
			exit = 74
		}
	}
	record := agentProbeRecord{Raw: string(raw), Response: response, Exit: exit, ObservedAt: time.Now().UTC()}
	data, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(74)
	}
	file, err := os.CreateTemp(args[1], "event-*.json")
	if err == nil {
		_, err = file.Write(data)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "save evidence: %v\n", err)
		os.Exit(74)
	}
	if fault && args[2] == "save-failure" {
		// An actual failed save is intentional; the normal record above preserves
		// fault intent independently so a missing record is not mistaken for denial.
		_, err := os.CreateTemp(args[1]+"/intentionally-absent", "event-*.json")
		fmt.Fprintf(os.Stderr, "intentional save evidence failure: %v\n", err)
	}
	if fault && args[2] == "timeout" {
		time.Sleep(3 * time.Second)
	}
	if response != "" {
		fmt.Fprintln(os.Stdout, strings.TrimSpace(response))
	}
	os.Exit(exit)
}

type agentProbeCall struct{ ID, Name, Input, Output string }
type agentProbeEvidence struct {
	Complete                 bool
	Records                  []agentProbeRecord
	Calls                    []agentProbeCall
	Control                  *agentProbeEvidence
	ProcessAfterStop         bool
	MarkerAgent, MarkerNonce string
	MarkerCommand            string
	MarkerEvent              agentProbeRecord
}
type agentProbeResult struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func agentProbeEvaluate(e agentProbeEvidence) map[string]agentProbeResult {
	results := map[string]agentProbeResult{}
	reasons := []string{
		"拒否の実tool結果と、子開始・試験印を伴う許可対照の完全な証拠が必要",
		"固定版の構造化した要求rootと実子rootの対応は未確定。親cwdやpromptから推測しない",
		"追加依頼・再開・割込み・closeの実tool結果とPre/Postの照合は未確定",
		"Stopと子に対応する外部process終端は未確定。返答・Unit報告・spawn Postを停止と扱わない",
		"Unitあり/なし、別path・別調整rootの同一worktree対応は未確定",
		"hook故障時の実tool結果と子実行の照合は未確定",
	}
	for i, reason := range reasons {
		results[fmt.Sprintf("G0-%d", i+1)] = agentProbeResult{"inconclusive", reason}
	}
	if agentProbeDenied(e) && e.Control != nil && agentProbeAllowed(*e.Control) {
		results["G0-1"] = agentProbeResult{"pass", "独立した拒否caseの実tool拒否・子開始なしと、同じ担当の許可caseの実子開始・試験印を照合"}
	}
	return results
}

func agentProbeSpawn(e agentProbeEvidence) (agentProbeCall, bool) {
	if !e.Complete || len(e.Calls) != 1 {
		return agentProbeCall{}, false
	}
	call := e.Calls[0]
	if call.ID == "" || call.Name != "spawn_agent" || call.Output == "" {
		return call, false
	}
	var input struct {
		Agent string `json:"agent_type"`
	}
	if json.Unmarshal([]byte(call.Input), &input) != nil || input.Agent != "probe_worker" {
		return call, false
	}
	return call, true
}

func agentProbeDenied(e agentProbeEvidence) bool {
	call, ok := agentProbeSpawn(e)
	if !ok || e.MarkerAgent != "" || e.MarkerNonce != "" {
		return false
	}
	var output struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(call.Output), &output) != nil || output.Error != "Expected G0 probe denial. Do not retry or substitute another agent." {
		return false
	}
	pre := 0
	for _, record := range e.Records {
		var input agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &input) != nil {
			return false
		}
		if input.Event == "SubagentStart" {
			return false
		}
		if input.Event == "PreToolUse" && input.Tool == "spawn_agent" {
			if input.Call != call.ID || input.Session == "" || record.Exit != 0 {
				return false
			}
			var response struct {
				Hook struct {
					Decision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if json.Unmarshal([]byte(record.Response), &response) != nil || response.Hook.Decision != "deny" {
				return false
			}
			pre++
		}
	}
	return pre == 1
}

func agentProbeAllowed(e agentProbeEvidence) bool {
	call, ok := agentProbeSpawn(e)
	if !ok {
		return false
	}
	var output struct {
		Agent string `json:"agent_id"`
	}
	if json.Unmarshal([]byte(call.Output), &output) != nil || output.Agent == "" || e.MarkerAgent != output.Agent || e.MarkerNonce == "" {
		return false
	}
	var marker agentProbeInput
	if json.Unmarshal([]byte(e.MarkerEvent.Raw), &marker) != nil || marker.Event != "PostToolUse" || marker.Session != output.Agent || marker.Call == "" || marker.Tool != "Bash" {
		return false
	}
	var command struct {
		Command string `json:"command"`
	}
	var result struct {
		Exit *int `json:"exit_code"`
	}
	if json.Unmarshal(marker.Input, &command) != nil || command.Command != e.MarkerCommand || !strings.Contains(command.Command, e.MarkerNonce) || json.Unmarshal(marker.Response, &result) != nil || result.Exit == nil || *result.Exit != 0 {
		return false
	}
	pre, start := 0, 0
	for _, record := range e.Records {
		var input agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &input) != nil {
			return false
		}
		if input.Event == "PreToolUse" && input.Tool == "spawn_agent" {
			if input.Call != call.ID || input.Session == "" || record.Exit != 0 || record.Response != "{}" {
				return false
			}
			pre++
		}
		if input.Event == "SubagentStart" {
			if input.Agent != output.Agent {
				return false
			}
			start++
		}
	}
	return pre == 1 && start == 1
}
