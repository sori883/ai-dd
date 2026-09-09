package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
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
	Event           string          `json:"hook_event_name"`
	Tool            string          `json:"tool_name"`
	Input           json.RawMessage `json:"tool_input"`
	Response        json.RawMessage `json:"tool_response"`
	Session         string          `json:"session_id"`
	Turn            string          `json:"turn_id"`
	Call            string          `json:"tool_use_id"`
	Agent           string          `json:"agent_id"`
	CWD             string          `json:"cwd"`
	Transcript      string          `json:"transcript_path"`
	AgentTranscript string          `json:"agent_transcript_path"`
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
	if args[2] == "deny" && input.Event == "PreToolUse" && agentProbeSpawnTool(input.Tool) && spawn.Agent == "probe_worker" {
		response = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Expected G0 probe denial. Do not retry or substitute another agent."}}`
	}
	exit := 0
	fault := input.Event == "PreToolUse" && agentProbeSpawnTool(input.Tool) && spawn.Agent == "probe_worker"
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

type agentProbeCall struct{ ID, Name, Input, Output, Session string }
type agentProbeEvidence struct {
	Complete                       bool
	Records                        []agentProbeRecord
	Calls                          []agentProbeCall
	Control                        *agentProbeEvidence
	Processes                      map[string]json.RawMessage
	Children                       map[string]agentProbeChildMetadata
	ExpectedNonce, ExpectedCommand string
	ProcessAfterStop               bool
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
	if !e.Complete {
		return agentProbeCall{}, false
	}
	var spawn agentProbeCall
	count := 0
	seen := map[string]bool{}
	for _, call := range e.Calls {
		key := call.Session + "\x00" + call.ID
		if call.Session == "" || call.ID == "" || seen[key] {
			return spawn, false
		}
		seen[key] = true
		if call.Name == "spawn_agent" {
			spawn = call
			count++
		}
		// Opaque calls stay raw. Only a direct spawn plus its native hook
		// evidence is evaluated; no inner calls are synthesized.
		if call.Name == "" {
			return spawn, false
		}
	}
	if count != 1 || spawn.Output == "" {
		return spawn, false
	}
	var input struct {
		Agent string `json:"agent_type"`
	}
	if json.Unmarshal([]byte(spawn.Input), &input) != nil || input.Agent != "probe_worker" {
		return spawn, false
	}
	return spawn, true
}

// Normalize only a JSON object, a function-call JSON string, or the two
// input_text blocks recorded by the fixed CLI. Output itself remains raw.
// This never interprets JavaScript or invents an inner code-mode call.
func agentProbeResultObject(raw string) json.RawMessage {
	data := []byte(raw)
	if len(data) == 0 {
		return nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) == nil && object != nil {
		return data
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		if json.Unmarshal([]byte(text), &object) == nil && object != nil {
			return []byte(text)
		}
		return nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(data, &blocks) != nil || len(blocks) != 2 || blocks[0].Type != "input_text" || blocks[1].Type != "input_text" {
		return nil
	}
	if strings.HasPrefix(blocks[0].Text, "Script completed\n") {
		if json.Unmarshal([]byte(blocks[1].Text), &object) == nil && object != nil {
			return []byte(blocks[1].Text)
		}
	}
	reason := "Expected G0 probe denial. Do not retry or substitute another agent."
	if strings.HasPrefix(blocks[0].Text, "Script failed\n") && blocks[1].Text == "Script error:\nCommand blocked by PreToolUse hook: "+reason {
		encoded, _ := json.Marshal(map[string]string{"error": reason})
		return encoded
	}
	return nil
}

func agentProbeSameInput(raw json.RawMessage, input string) bool {
	var left, right any
	return json.Unmarshal(raw, &left) == nil && json.Unmarshal([]byte(input), &right) == nil && reflect.DeepEqual(left, right)
}

func agentProbeDenied(e agentProbeEvidence) bool {
	call, ok := agentProbeSpawn(e)
	if !ok || len(e.Processes) != 0 {
		return false
	}
	var output struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(agentProbeResultObject(call.Output), &output) != nil || output.Error != "Expected G0 probe denial. Do not retry or substitute another agent." {
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
		if input.Event == "PreToolUse" && agentProbeSpawnTool(input.Tool) {
			if input.Call != call.ID || input.Session != call.Session || record.Exit != 0 || !agentProbeSameInput(input.Input, call.Input) {
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
		Task  string `json:"task_name"`
	}
	if json.Unmarshal(agentProbeResultObject(call.Output), &output) == nil && output.Task != "" && output.Agent == "" {
		return agentProbeObservedAllowed(e, call, output.Task)
	}
	if json.Unmarshal(agentProbeResultObject(call.Output), &output) != nil || output.Agent == "" || output.Agent == call.Session {
		return false
	}
	pre, start := 0, 0
	for _, record := range e.Records {
		var input agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &input) != nil {
			return false
		}
		if input.Event == "PreToolUse" && agentProbeSpawnTool(input.Tool) {
			if input.Call != call.ID || input.Session != call.Session || record.Exit != 0 || record.Response != "{}" || !agentProbeSameInput(input.Input, call.Input) {
				return false
			}
			pre++
		}
		if input.Event == "SubagentStart" {
			if input.Agent != output.Agent || input.Session != call.Session {
				return false
			}
			start++
		}
	}
	return pre == 1 && start == 1 && agentProbeHasMarker(e, output.Agent)
}

func agentProbeHasMarker(e agentProbeEvidence, agent string) bool {
	if e.ExpectedNonce == "" || e.ExpectedCommand == "" || !strings.Contains(e.ExpectedCommand, " "+e.ExpectedNonce+" 15000") || len(e.Processes) != 1 {
		return false
	}
	raw, ok := e.Processes["process-"+e.ExpectedNonce+".json"]
	if !ok {
		return false
	}
	var process agentProbeProcessState
	if json.Unmarshal(raw, &process) != nil || process.Nonce != e.ExpectedNonce || process.PID <= 0 || process.StartedAt.IsZero() || process.EndedAt.Before(process.StartedAt) || process.ObservedAt != process.EndedAt {
		return false
	}
	matches := 0
	for _, record := range e.Records {
		var marker agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &marker) != nil {
			return false
		}
		if marker.Event != "PostToolUse" || marker.Tool != "Bash" {
			continue
		}
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(marker.Input, &input) != nil || input.Command != e.ExpectedCommand {
			continue
		}
		if marker.Session != agent || marker.Call == "" || record.Exit != 0 || !agentProbeExitZero(string(marker.Response)) {
			return false
		}
		matched := false
		for _, call := range e.Calls {
			if call.ID != marker.Call || call.Session != agent {
				continue
			}
			var command struct {
				Command string `json:"cmd"`
			}
			if call.Name != "exec_command" || json.Unmarshal([]byte(call.Input), &command) != nil || command.Command != e.ExpectedCommand || !agentProbeExitZero(call.Output) {
				return false
			}
			if matched {
				return false
			}
			matched = true
		}
		if !matched {
			return false
		}
		matches++
	}
	return matches == 1
}

func agentProbeExitZero(raw string) bool {
	var result struct {
		Exit *int `json:"exit_code"`
	}
	return json.Unmarshal(agentProbeResultObject(raw), &result) == nil && result.Exit != nil && *result.Exit == 0
}

func agentProbeSpawnTool(name string) bool {
	return name == "spawn_agent" || name == "collaborationspawn_agent"
}

// This association is a G0 observation for the pinned transcript format, not a
// stable runtime API or a guarantee about reusing task names.
type agentProbeChildMetadata struct {
	ID         string `json:"id"`
	AgentPath  string `json:"agent_path"`
	Parent     string `json:"parent_thread_id"`
	ForkedFrom string `json:"forked_from_id"`
}

func agentProbeObservedAllowed(e agentProbeEvidence, call agentProbeCall, task string) bool {
	pre, post, start := 0, 0, 0
	child := ""
	for _, record := range e.Records {
		var input agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &input) != nil {
			return false
		}
		if agentProbeSpawnTool(input.Tool) {
			if input.Session != call.Session || input.Call != call.ID || record.Exit != 0 || !agentProbeSameInput(input.Input, call.Input) {
				return false
			}
			switch input.Event {
			case "PreToolUse":
				if record.Response != "{}" {
					return false
				}
				pre++
			case "PostToolUse":
				var result struct {
					Task string `json:"task_name"`
				}
				if json.Unmarshal(agentProbeResultObject(string(input.Response)), &result) != nil || result.Task != task {
					return false
				}
				post++
			}
		}
		if input.Event == "SubagentStart" {
			meta, ok := e.Children[input.Agent]
			if !ok || input.Session != call.Session || input.Agent == "" || meta.ID != input.Agent || meta.Parent != call.Session || meta.ForkedFrom != call.Session || meta.AgentPath != task {
				return false
			}
			child = input.Agent
			start++
		}
	}
	return pre == 1 && post == 1 && start == 1 && agentProbeObservedMarker(e, call.Session, child)
}

func agentProbeObservedMarker(e agentProbeEvidence, parent, child string) bool {
	if e.ExpectedNonce == "" || !strings.Contains(e.ExpectedCommand, " "+e.ExpectedNonce+" 15000") || len(e.Processes) != 1 {
		return false
	}
	raw, ok := e.Processes["process-"+e.ExpectedNonce+".json"]
	if !ok {
		return false
	}
	var process agentProbeProcessState
	if json.Unmarshal(raw, &process) != nil || process.Nonce != e.ExpectedNonce || process.PID <= 0 || process.StartedAt.IsZero() || process.EndedAt.Before(process.StartedAt) || process.ObservedAt != process.EndedAt {
		return false
	}
	var pre, post []agentProbeInput
	for _, record := range e.Records {
		var input agentProbeInput
		if json.Unmarshal([]byte(record.Raw), &input) != nil {
			return false
		}
		if input.Tool != "Bash" {
			continue
		}
		var command struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(input.Input, &command) != nil || command.Command != e.ExpectedCommand {
			continue
		}
		if input.Session != parent || input.Agent != child || input.Call == "" || input.Turn == "" || record.Exit != 0 {
			return false
		}
		if input.Event == "PreToolUse" {
			pre = append(pre, input)
		}
		if input.Event == "PostToolUse" {
			post = append(post, input)
		}
	}
	if len(pre) != 1 || len(post) != 1 || pre[0].Call != post[0].Call || pre[0].Turn != post[0].Turn {
		return false
	}
	var response string
	// Empty Bash output plus the helper's side effect proves execution here;
	// neither the Post event nor this empty response proves exit 0 or child stop.
	return json.Unmarshal(post[0].Response, &response) == nil && response == ""
}
