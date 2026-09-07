//go:build integration

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/minimal"
	"strings"
	"testing"
)

const journeyFailedPatch = "*** Begin Patch\n*** Update File: boundary-edit.txt\n@@\n-this-line-is-absent\n+recovered\n*** End Patch"
const journeyRetryPatch = "*** Begin Patch\n*** Update File: boundary-edit.txt\n@@\n-before\n+recovered\n*** End Patch"

func verifyJourneyEditRecovery(events []journeyObservation, transport []byte) error {
	outer := ""
	failure := false
	scanner := bufio.NewScanner(bytes.NewReader(transport))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		var row struct {
			Type    string
			Payload struct {
				Type, Name string
				CallID     string `json:"call_id"`
				Input      string
				Output     []struct{ Type, Text string }
			}
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return err
		}
		p := row.Payload
		if row.Type != "response_item" {
			continue
		}
		if p.Type == "custom_tool_call" && p.Name == "exec" && strings.TrimSpace(p.Input) == "text(await tools.apply_patch("+string(mustJourneyJSON(journeyFailedPatch))+"));" {
			outer = p.CallID
		}
		if p.Type == "custom_tool_call_output" && outer != "" && p.CallID == outer && len(p.Output) == 2 && p.Output[0].Type == "input_text" && strings.HasPrefix(p.Output[0].Text, "Script failed\n") && p.Output[1].Type == "input_text" && strings.HasPrefix(p.Output[1].Text, "Script error:\napply_patch verification failed:") {
			failure = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !failure {
		return fmt.Errorf("missing exact failed patch process response")
	}
	pending := map[string]journeyObservation{}
	session, id, failedID := "", "", ""
	recovered, retried, saved, clean := false, false, false, false
	for _, e := range events {
		key := e.Input.Session + "/" + e.Input.ID
		if e.Input.Event == "PreToolUse" {
			pending[key] = e
			if e.Input.Input.Command == journeyFailedPatch && e.After.Tool == e.Input.ID && e.After.Dirty && e.Files["boundary-edit.txt"] == "before\n" {
				session = e.Input.Session
				id = e.After.Intent
				failedID = e.Input.ID
			}
		}
		if e.Input.Event == "PostToolUse" {
			if failedID != "" && e.Input.Session == session && e.Input.ID == failedID {
				return fmt.Errorf("failed patch unexpectedly has Post")
			}
			pre, ok := pending[key]
			if !ok {
				continue
			}
			delete(pending, key)
			same := e.Input.Session == session && e.After.Intent == id && e.After.Space == pre.Before.Space
			if same && pre.Before.Tool == failedID && failedID != "" && strings.Contains(pre.Input.Input.Command, " session bind ") && strings.Contains(pre.Input.Input.Command, " --recover") && e.After.Tool == "" && e.After.Dirty && e.After.RuleHash != "" && e.Files["boundary-edit.txt"] == "before\n" {
				recovered = true
			}
			if same && recovered && pre.Input.Input.Command == journeyRetryPatch && pre.After.Tool == e.Input.ID && e.Before.Tool == e.Input.ID && e.After.Tool == "" && e.After.Dirty && e.Files["boundary-edit.txt"] == "recovered\n" {
				retried = true
			}
			if same && retried && pre.Before.Dirty && !e.After.Dirty && journeyBody(pre.Documents[id]) != journeyBody(e.Documents[id]) {
				saved = true
			}
		}
		if e.Input.Event == "Stop" && saved && e.Input.Session == session && e.Before.Intent == id && !e.Before.Dirty && e.Before.Tool == "" {
			clean = true
		}
	}
	if !recovered || !retried || !saved || !clean {
		return fmt.Errorf("incomplete edit recovery: recovered=%v retry=%v saved=%v clean=%v", recovered, retried, saved, clean)
	}
	return nil
}
func journeyEditFixture() ([]journeyObservation, []byte) {
	state := minimal.Session{Space: "default", Intent: "id", RuleHash: "rules", Dirty: true}
	running := state
	running.Tool = "failed"
	retry := state
	retry.Tool = "retry"
	clean := state
	clean.Dirty = false
	event := func(kind, id, cmd string, before, after minimal.Session, file, doc string) journeyObservation {
		input := minimal.HookInput{Event: kind, Session: "session", ID: id}
		input.Input.Command = cmd
		return journeyObservation{Input: input, Before: before, After: after, Files: map[string]string{"boundary-edit.txt": file}, Documents: map[string]string{"id": doc}}
	}
	events := []journeyObservation{
		event("PreToolUse", "failed", journeyFailedPatch, state, running, "before\n", "initial"),
		event("PreToolUse", "recover", "aidlc session bind id --space default --session session --recover", running, running, "before\n", "initial"),
		event("PostToolUse", "recover", "", state, state, "before\n", "initial"),
		event("PreToolUse", "retry", journeyRetryPatch, state, retry, "before\n", "initial"),
		event("PostToolUse", "retry", "", retry, state, "recovered\n", "initial"),
		event("PreToolUse", "update", "aidlc kdr update id", state, state, "recovered\n", "initial"),
		event("PostToolUse", "update", "", clean, clean, "recovered\n", "recorded recovery"),
		event("Stop", "", "", clean, clean, "recovered\n", "recorded recovery"),
	}
	call, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "outer", "input": "text(await tools.apply_patch(" + string(mustJourneyJSON(journeyFailedPatch)) + "));"}})
	output, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call_output", "call_id": "outer", "output": []map[string]string{{"type": "input_text", "text": "Script failed\n"}, {"type": "input_text", "text": "Script error:\napply_patch verification failed: missing expected line"}}}})
	return events, append(append(call, '\n'), output...)
}
func mustJourneyJSON(value string) []byte { raw, _ := json.Marshal(value); return raw }
func TestMinimalJourneyEditRecoveryEvidence(t *testing.T) {
	events, transport := journeyEditFixture()
	if err := verifyJourneyEditRecovery(events, transport); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"failure_response", "post_present", "recover", "wrong_session", "dirty_lost", "retry", "side_effect", "save", "stop"} {
		t.Run(name, func(t *testing.T) {
			events, transport := journeyEditFixture()
			switch name {
			case "failure_response":
				transport = nil
			case "post_present":
				extra := events[0]
				extra.Input.Event = "PostToolUse"
				events = append(events, extra)
			case "recover":
				events[2].Input.Event = "ignored"
			case "wrong_session":
				events[2].Input.Session = "other"
			case "dirty_lost":
				events[2].After.Dirty = false
			case "retry":
				events[4].Input.Event = "ignored"
			case "side_effect":
				events[4].Files["boundary-edit.txt"] = "before\n"
			case "save":
				events[6].Documents["id"] = "initial"
			case "stop":
				events[7].Input.Event = "ignored"
			}
			if verifyJourneyEditRecovery(events, transport) == nil {
				t.Fatal("accepted incomplete edit recovery")
			}
		})
	}
}
