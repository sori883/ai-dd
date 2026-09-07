//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/minimal"
)

type journeyTestResult struct {
	Exit                 int
	Output, Source, Test string
}

func verifyJourneyEvents(events []journeyObservation, testCommand string) error {
	type progress struct {
		started, bound, updated, clean bool
		id, latest                     string
	}
	sessions := map[string]*progress{}
	pending := map[string]journeyObservation{}
	denied := false
	var red *journeyTestResult
	green := false
	canonical := ""
	lastRecord := ""
	for _, e := range events {
		session := e.Input.Session
		if sessions[session] == nil {
			sessions[session] = &progress{}
		}
		p := sessions[session]
		key := session + "/" + e.Input.ID
		switch e.Input.Event {
		case "SessionStart":
			p.started = true
		case "PreToolUse":
			specific, _ := e.Output["hookSpecificOutput"].(map[string]any)
			if specific["permissionDecision"] == "deny" {
				if e.Before.Intent == "" && e.After.Tool == "" {
					denied = true
				}
				continue
			}
			if e.Input.ID == "" {
				return fmt.Errorf("missing Pre ID")
			}
			pending[key] = e
		case "PostToolUse":
			pre, ok := pending[key]
			if !ok {
				return fmt.Errorf("unmatched Post %s", key)
			}
			delete(pending, key)
			retainedKey := session + "/" + pre.Before.Tool
			retained, retainedOK := pending[retainedKey]
			if retainedOK && retained.Input.Tool == "apply_patch" && strings.Contains(pre.Input.Input.Command, " session bind ") && strings.Contains(pre.Input.Input.Command, " --recover") && e.After.Tool == "" && e.After.Dirty && e.After.Intent == pre.Before.Intent && e.After.Space == pre.Before.Space {
				delete(pending, retainedKey)
			}
			if pre.Before.Intent == "" && e.After.Intent != "" && e.After.RuleHash != "" {
				if !p.started {
					return fmt.Errorf("bind without SessionStart")
				}
				if lastRecord != "" && e.Documents[e.After.Intent] != lastRecord {
					return fmt.Errorf("resume content differs from recorded content")
				}
				p.bound = true
				p.id = e.After.Intent
				p.latest = e.Documents[p.id]
				if p.latest == "" {
					return fmt.Errorf("bind missing canonical bytes")
				}
				if canonical == "" {
					canonical = p.id
				} else if canonical != p.id {
					return fmt.Errorf("resumed another ID")
				}
			}
			if pre.Before.Dirty && !e.After.Dirty && e.After.Intent != "" {
				id := e.After.Intent
				if !p.bound || id != p.id || pre.Documents[id] == "" || journeyBody(pre.Documents[id]) == journeyBody(e.Documents[id]) {
					return fmt.Errorf("record without changed canonical body")
				}
				p.updated = true
				p.latest = e.Documents[id]
				lastRecord = p.latest
			}
			if pre.Input.Input.Command == testCommand {
				if pre.After.Tool != e.Input.ID || e.Before.Tool != e.Input.ID || e.After.Tool != "" || !e.After.Dirty {
					return fmt.Errorf("test has no terminal slot pair")
				}
				var result journeyTestResult
				if json.Unmarshal([]byte(e.Response), &result) != nil {
					return fmt.Errorf("missing process test result")
				}
				run, pass, fail := false, false, false
				for _, line := range strings.Split(result.Output, "\n") {
					var event struct{ Action, Test string }
					if json.Unmarshal([]byte(line), &event) == nil && event.Test == "TestAdd" {
						switch event.Action {
						case "run":
							run = true
						case "pass":
							pass = true
						case "fail":
							fail = true
						}
					}
				}
				if !run || result.Source == "" || result.Test == "" {
					return fmt.Errorf("no executed TestAdd")
				}
				if result.Exit == 1 && fail && !pass && red == nil {
					copy := result
					red = &copy
				} else if result.Exit == 0 && pass && !fail && red != nil && result.Source != red.Source && result.Test == red.Test {
					green = true
				} else {
					return fmt.Errorf("test not RED then changed implementation GREEN")
				}
			}
		case "Stop":
			if p.bound && p.updated && !e.Before.Dirty && e.Before.Tool == "" && e.Before.Intent == p.id && e.Documents[p.id] == p.latest {
				p.clean = true
			}
		}
	}
	complete := 0
	for _, p := range sessions {
		if p.started && p.bound && p.updated && p.clean {
			complete++
		}
	}
	if complete != 2 || !denied || red == nil || !green || len(pending) != 0 {
		return fmt.Errorf("incomplete journey: sessions=%d denied=%v RED=%v GREEN=%v pending=%d", complete, denied, red != nil, green, len(pending))
	}
	return nil
}

func journeyGoodEvidence() []journeyObservation {
	var events []journeyObservation
	doc := "initial record"
	events = append(events, journeyObservation{Input: minimal.HookInput{Event: "PreToolUse", Session: "one", ID: "deny"}, Output: map[string]any{"hookSpecificOutput": map[string]any{"permissionDecision": "deny"}}})
	for _, session := range []string{"one", "two"} {
		state := minimal.Session{Dirty: true}
		add := func(kind, id string, before, after minimal.Session, response string) {
			in := minimal.HookInput{Event: kind, Session: session, ID: id}
			if strings.HasPrefix(id, "test-") {
				in.Input.Command = "fixture-go-test"
			}
			events = append(events, journeyObservation{Input: in, Before: before, After: after, Documents: map[string]string{"same-id": doc}, Response: response})
		}
		add("SessionStart", "", state, state, "")
		add("PreToolUse", "bind", state, state, "")
		bound := state
		bound.Intent = "same-id"
		bound.Space = "default"
		bound.RuleHash = "rules"
		add("PostToolUse", "bind", state, bound, "")
		state = bound
		if session == "one" {
			for _, exit := range []int{1, 0} {
				id := fmt.Sprintf("test-%d", exit)
				running := state
				running.Tool = id
				add("PreToolUse", id, state, running, "")
				action, source := "fail", "return 0"
				if exit == 0 {
					action = "pass"
					source = "return a+b"
				}
				result := journeyTestResult{Exit: exit, Source: source, Test: "func TestAdd(t *testing.T) { if Add(2,3)!=5 { t.Fatal() } }", Output: "{\"Action\":\"run\",\"Test\":\"TestAdd\"}\n{\"Action\":\"" + action + "\",\"Test\":\"TestAdd\"}\n"}
				raw, _ := json.Marshal(result)
				add("PostToolUse", id, running, state, string(raw))
			}
		}
		add("PreToolUse", "update", state, state, "")
		doc += " recorded " + session
		clean := state
		clean.Dirty = false
		add("PostToolUse", "update", state, clean, "")
		add("Stop", "", clean, clean, "")
	}
	return events
}
func TestMinimalJourneyEvidence(t *testing.T) {
	if err := verifyJourneyEvents(journeyGoodEvidence(), "fixture-go-test"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"red", "green", "no_tests", "wrong_exit", "unchanged_source", "different_test", "second_bind", "second_update", "second_stop", "other_id", "unmatched_post"} {
		t.Run(name, func(t *testing.T) {
			events := journeyGoodEvidence()
			for i := range events {
				e := &events[i]
				switch name {
				case "red":
					if e.Input.ID == "test-1" {
						e.Input.Event = "ignored"
					}
				case "green":
					if e.Input.ID == "test-0" {
						e.Input.Event = "ignored"
					}
				case "no_tests", "wrong_exit", "unchanged_source", "different_test":
					if e.Input.Event == "PostToolUse" && e.Input.ID == "test-0" {
						var result journeyTestResult
						json.Unmarshal([]byte(e.Response), &result)
						switch name {
						case "no_tests":
							result.Output = "? journey [no test files]"
						case "wrong_exit":
							result.Exit = 1
						case "unchanged_source":
							result.Source = "return 0"
						case "different_test":
							result.Test = "different test"
						}
						raw, _ := json.Marshal(result)
						e.Response = string(raw)
					}
				case "second_bind":
					if e.Input.Session == "two" && e.Input.ID == "bind" {
						e.Input.Event = "ignored"
					}
				case "second_update":
					if e.Input.Session == "two" && e.Input.ID == "update" {
						e.Input.Event = "ignored"
					}
				case "second_stop":
					if e.Input.Session == "two" && e.Input.Event == "Stop" {
						e.Input.Event = "ignored"
					}
				case "other_id":
					if e.Input.Session == "two" {
						e.After.Intent = "other-id"
					}
				case "unmatched_post":
					if e.Input.ID == "test-0" && e.Input.Event == "PostToolUse" {
						e.Input.ID = "wrong"
					}
				}
			}
			if verifyJourneyEvents(events, "fixture-go-test") == nil {
				t.Fatal("accepted invalid evidence")
			}
		})
	}
}

func TestMinimalJourneyRunner(t *testing.T) {
	root := t.TempDir()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(root, "go.mod"), "module journey\n\ngo 1.26\n")
	writeMinimalFixture(t, filepath.Join(root, "add_test.go"), "package journey\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(2,3)!=5 {t.Fatal(\"want 5\")} }\n")
	for _, tc := range []struct {
		name, source string
		exit         int
	}{{"red", "package journey\nfunc Add(a,b int)int{return 0}\n", 1}, {"green", "package journey\nfunc Add(a,b int)int{return a+b}\n", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			writeMinimalFixture(t, filepath.Join(root, "add.go"), tc.source)
			cmd := exec.CommandContext(t.Context(), helper, "-test.run=^TestMinimalJourneyGoTest$", "--", root)
			raw, runErr := cmd.Output()
			if (runErr != nil) != (tc.exit != 0) {
				t.Fatalf("runner result: %v %s", runErr, raw)
			}
			var result journeyTestResult
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if result.Exit != tc.exit || result.Source != tc.source || !strings.Contains(result.Output, `"Test":"TestAdd"`) {
				t.Fatalf("not actual test result: %s", raw)
			}
		})
	}
}

func TestMinimalJourneyRejectsMetadataOnlyRecord(t *testing.T) {
	events := journeyGoodEvidence()
	for i := range events {
		e := &events[i]
		if e.Input.Session != "two" {
			continue
		}
		if e.Input.ID == "update" && e.Input.Event == "PreToolUse" {
			e.Documents = map[string]string{"same-id": "---\ntitle: Before\n---\nsame body\n"}
		}
		if (e.Input.ID == "update" && e.Input.Event == "PostToolUse") || e.Input.Event == "Stop" {
			e.Documents = map[string]string{"same-id": "---\ntitle: After\n---\nsame body\n"}
		}
	}
	if verifyJourneyEvents(events, "fixture-go-test") == nil {
		t.Fatal("metadata-only change counted as content recording")
	}
}

func journeyBody(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if strings.HasPrefix(normalized, "---\n") {
		if at := strings.Index(normalized[4:], "\n---\n"); at >= 0 {
			return normalized[4+at+5:]
		}
	}
	return normalized
}

func TestMinimalJourneyRelayDiagnostics(t *testing.T) {
	root, evidence := t.TempDir(), t.TempDir()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(evidence, "phase"), "cli-fault")
	input := minimal.HookInput{Event: "SessionStart", Session: "fault-session"}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), helper, "-test.run=^TestMinimalJourneyRelay$", "--", filepath.Join(root, "missing-aidlc"), root, evidence)
	cmd.Stdin = strings.NewReader(string(raw))
	output, runErr := cmd.CombinedOutput()
	if runErr == nil || !strings.Contains(string(output), "no such file") {
		t.Fatalf("missing CLI diagnostic not forwarded: %v %s", runErr, output)
	}
	files, err := filepath.Glob(filepath.Join(evidence, "journey-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("missing raw failure event: %v %v", files, err)
	}
	saved, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		journeyObservation
		Raw json.RawMessage
	}
	if err := json.Unmarshal(saved, &record); err != nil {
		t.Fatal(err)
	}
	if record.Error == "" || record.Phase != "cli-fault" || record.Before != record.After || !record.After.Dirty || string(record.Raw) != string(raw) {
		t.Fatalf("fault observer lost raw/state: %s", saved)
	}
}

func TestMinimalJourneyRejectsDifferentResumeContent(t *testing.T) {
	events := journeyGoodEvidence()
	for i := range events {
		e := &events[i]
		if e.Input.Session == "two" && e.Input.ID == "bind" && e.Input.Event == "PostToolUse" {
			e.Documents = map[string]string{"same-id": "unrelated replacement"}
		}
	}
	if verifyJourneyEvents(events, "fixture-go-test") == nil {
		t.Fatal("resume did not read previous recorded content")
	}
}

func TestMinimalJourneyAcceptsExplicitEditRecovery(t *testing.T) {
	events := journeyGoodEvidence()
	at := 0
	for i, e := range events {
		if e.Input.Session == "one" && e.Input.Event == "PostToolUse" && e.Input.ID == "bind" {
			at = i + 1
			break
		}
	}
	state := events[at-1].After
	running := state
	running.Tool = "failed-edit"
	failed := journeyObservation{Input: minimal.HookInput{Event: "PreToolUse", Session: "one", ID: "failed-edit", Tool: "apply_patch"}, Before: state, After: running, Documents: events[at-1].Documents}
	failed.Input.Input.Command = journeyFailedPatch
	recoverPre := journeyObservation{Input: minimal.HookInput{Event: "PreToolUse", Session: "one", ID: "recover", Tool: "Bash"}, Before: running, After: running, Documents: failed.Documents}
	recoverPre.Input.Input.Command = "aidlc session bind same-id --space default --session one --recover"
	recoverPost := journeyObservation{Input: minimal.HookInput{Event: "PostToolUse", Session: "one", ID: "recover", Tool: "Bash"}, Before: state, After: state, Documents: failed.Documents}
	tail := append([]journeyObservation{}, events[at:]...)
	events = append(events[:at], failed, recoverPre, recoverPost)
	events = append(events, tail...)
	if err := verifyJourneyEvents(events, "fixture-go-test"); err != nil {
		t.Fatal(err)
	}
}
