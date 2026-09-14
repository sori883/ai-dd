//go:build integration && diagnostic

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type opaqueRow struct {
	Type    string `json:"type"`
	Payload struct {
		Type      string `json:"type"`
		Author    string `json:"author"`
		Recipient string `json:"recipient"`
		Content   []struct {
			Type      string `json:"type"`
			Text      string `json:"text"`
			Encrypted string `json:"encrypted_content"`
		} `json:"content"`
	} `json:"payload"`
}

type opaqueHook struct {
	Event     string `json:"hook_event_name"`
	Session   string `json:"session_id"`
	Turn      string `json:"turn_id"`
	ID        string `json:"tool_use_id"`
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
	Tool      string `json:"tool_name"`
	Input     struct {
		Target  string `json:"target"`
		Message string `json:"message"`
	} `json:"tool_input"`
}

func reliabilityOpaqueDelivery(pre, post reliabilityRecord, rows [][]byte, parent, child, agent string) (string, bool) {
	for _, snap := range []reliabilitySnapshot{pre.Before, pre.After, post.Before, post.After} {
		if snap.Missing || snap.Error != "" || len(snap.Data) == 0 || !bytes.Equal(snap.Data, pre.Before.Data) {
			return "", false
		}
	}
	var inputs [2]opaqueHook
	for i, r := range []reliabilityRecord{pre, post} {
		if r.Exit != 0 || r.Error != "" || json.Unmarshal(r.Raw, &inputs[i]) != nil {
			return "", false
		}
		var out struct {
			Continue *bool  `json:"continue"`
			Decision string `json:"decision"`
			Specific struct {
				Permission string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if !bytes.HasPrefix(bytes.TrimSpace(r.Stdout), []byte("{")) || json.Unmarshal(r.Stdout, &out) != nil || out.Continue != nil && !*out.Continue || out.Decision != "" && out.Decision != "allow" || out.Specific.Permission != "" && out.Specific.Permission != "allow" {
			return "", false
		}
		in := inputs[i]
		if in.Session == "" || in.Turn == "" || in.ID == "" || in.AgentID == "" || in.AgentID != agent || in.AgentType == "" || in.Input.Target != parent || in.Input.Message == "" || in.Tool != "send_message" && in.Tool != "collaborationsend_message" {
			return "", false
		}
	}
	if inputs[0].Event != "PreToolUse" || inputs[1].Event != "PostToolUse" {
		return "", false
	}
	inputs[1].Event = inputs[0].Event
	if inputs[0] != inputs[1] {
		return "", false
	}
	input := inputs[0]
	found, candidates, final := 0, 0, false
	for _, raw := range rows {
		var row opaqueRow
		if json.Unmarshal(raw, &row) != nil {
			return "", false
		}
		p := row.Payload
		if row.Type != "response_item" || p.Type != "agent_message" {
			continue
		}
		if p.Author == child && p.Recipient == parent && len(p.Content) > 0 && p.Content[0].Type == "input_text" && strings.HasPrefix(p.Content[0].Text, "Message Type: MESSAGE\n") {
			candidates++
		}
		if p.Author == child && p.Recipient == parent && len(p.Content) > 0 && p.Content[0].Type == "input_text" && strings.HasPrefix(p.Content[0].Text, "Message Type: FINAL_ANSWER\nTask name: "+parent+"\nSender: "+child+"\nPayload:\n") {
			final = true
		}
		if p.Author == child && p.Recipient == parent && len(p.Content) == 2 && p.Content[0].Type == "input_text" && p.Content[0].Text == "Message Type: MESSAGE\nTask name: "+parent+"\nSender: "+child+"\nPayload:\n" && p.Content[1].Type == "encrypted_content" && p.Content[1].Encrypted == input.Input.Message {
			if final {
				return "", false
			}
			found++
		}
	}
	if found != 1 || candidates != 1 || !final {
		return "", false
	}
	return reliabilityHash([]byte(input.Input.Message)), true
}

func opaqueFixture(t *testing.T) (reliabilityRecord, reliabilityRecord, [][]byte) {
	t.Helper()
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"session","turn_id":"turn","tool_use_id":"tool","agent_id":"agent","agent_type":"aidlc-stage-planner","tool_name":"send_message","tool_input":{"target":"/root","message":"synthetic-opaque"}}`)
	pre := reliabilityRecord{Raw: raw, Stdout: []byte(`{}`), Before: reliabilitySnapshot{Data: []byte("session bytes")}, After: reliabilitySnapshot{Data: []byte("session bytes")}}
	post := pre
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	input["hook_event_name"] = "PostToolUse"
	post.Raw, _ = json.Marshal(input)
	return pre, post, [][]byte{[]byte(`{"type":"response_item","payload":{"type":"agent_message","author":"/root/report","recipient":"/root","content":[{"type":"input_text","text":"Message Type: MESSAGE\nTask name: /root\nSender: /root/report\nPayload:\n"},{"type":"encrypted_content","encrypted_content":"synthetic-opaque"}]}}`), []byte(`{"type":"response_item","payload":{"type":"agent_message","author":"/root/report","recipient":"/root","content":[{"type":"input_text","text":"Message Type: FINAL_ANSWER\nTask name: /root\nSender: /root/report\nPayload:\ndone"}]}}`)}
}

func TestHookReliabilityOpaqueEvidence(t *testing.T) {
	t.Log("synthetic opaque checker fixture")
	pre, post, rows := opaqueFixture(t)
	if hash, ok := reliabilityOpaqueDelivery(pre, post, rows, "/root", "/root/report", "agent"); !ok || hash != reliabilityHash([]byte("synthetic-opaque")) {
		t.Fatal("valid opaque delivery rejected")
	}
	rows[0] = bytes.ReplaceAll(rows[0], []byte("synthetic-opaque"), []byte("different"))
	if _, ok := reliabilityOpaqueDelivery(pre, post, rows, "/root", "/root/report", "agent"); ok {
		t.Fatal("mismatched opaque receipt accepted")
	}
}
