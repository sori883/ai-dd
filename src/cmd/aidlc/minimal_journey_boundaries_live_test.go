//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/minimal"
)

const journeyLongCommand = "sleep 20; printf done > boundary-terminal"

func verifyJourneyBoundaryEvents(events []journeyObservation) error {
	pending := map[string]journeyObservation{}
	questionSession, id := "", ""
	question, answerDirty, answerFirst, answerAgain, answerDone := false, false, false, false, false
	ruleFault, cliFault, rebound, competed, terminal, recovered, resumed, resumeDone := false, false, false, false, false, false, false, false
	longID := ""
	answerClosed := false
	updates := map[string]int{}
	for _, e := range events {
		key := e.Input.Session + "/" + e.Input.ID
		specific, _ := e.Output["hookSpecificOutput"].(map[string]any)
		denied := specific["permissionDecision"] == "deny"
		clean := e.Input.Event == "Stop" && !e.Before.Dirty && e.Before.Tool == "" && e.Before.Intent == id && id != ""
		if e.Phase == "cli-fault" && e.Error != "" && strings.Contains(e.Error, "no such file") && e.Before == e.After && e.Before.Dirty && e.Input.Session == questionSession {
			cliFault = true
		}
		if e.Error != "" {
			continue
		}
		if e.Phase == "answer" && e.Input.Event == "UserPromptSubmit" && e.Input.Session == questionSession && !e.Before.Dirty && e.After.Dirty {
			answerDirty = true
		}
		if e.Input.Event == "PreToolUse" {
			if e.Phase == "rule-fault" && denied && e.After.Dirty && e.Files["aidlc/spaces/default/knowledge/rules/rule.md"] == "" && e.Input.Session == questionSession {
				ruleFault = true
			}
			if e.Phase == "recovery" && denied && strings.Contains(e.Input.Input.Command, " kdr update ") && e.Before.Tool == longID && longID != "" && e.Files["boundary-terminal"] == "" && e.After.Tool == longID {
				competed = true
			}
			if !denied && e.Error == "" {
				pending[key] = e
				if e.Phase == "answer" && answerFirst && !e.Before.Dirty && e.After.Dirty && e.After.Tool == e.Input.ID {
					answerAgain = true
				}
				if e.Phase == "recovery" && rebound && e.Input.Input.Command == journeyLongCommand && e.After.Tool == e.Input.ID && e.Files["boundary-terminal"] == "" {
					longID = e.Input.ID
				}
			}
		}
		if e.Input.Event == "PostToolUse" {
			pre, ok := pending[key]
			if !ok {
				return fmt.Errorf("unmatched boundary Post %s", key)
			}
			delete(pending, key)
			if pre.Before.Tool != "" && strings.Contains(pre.Input.Input.Command, " session bind ") && strings.Contains(pre.Input.Input.Command, " --recover") && e.After.Tool == "" && e.After.Dirty && e.After.Intent == pre.Before.Intent && e.After.Space == pre.Before.Space {
				delete(pending, e.Input.Session+"/"+pre.Before.Tool)
			}
			if e.Phase == "recovery" && e.After.RuleHash != "" && e.After.Intent == id && e.Files["aidlc/spaces/default/knowledge/rules/rule.md"] != "" {
				rebound = true
			}
			if e.Phase == "resumed" && pre.Before.Intent == "" && e.After.Intent == id && e.After.RuleHash != "" && e.Input.Session != questionSession && e.Documents[id] != "" {
				resumed = true
			}
			if pre.Before.Dirty && !e.After.Dirty && e.After.Intent != "" {
				currentID := e.After.Intent
				if pre.Documents[currentID] == "" || journeyBody(pre.Documents[currentID]) == journeyBody(e.Documents[currentID]) {
					return fmt.Errorf("no changed boundary record")
				}
				updates[e.Phase]++
				if e.Phase == "question" && strings.Contains(e.Documents[currentID], "precision-choice") {
					id = currentID
					questionSession = e.Input.Session
				}
				if e.Phase == "answer" && answerDirty && e.Input.Session == questionSession && currentID == id {
					if !answerFirst {
						answerFirst = true
					} else if answerAgain {
						answerDone = true
					}
				}
			}
			if e.Phase == "recovery" && competed && e.Input.ID == longID && e.Before.Tool == longID && e.After.Tool == "" && e.After.Dirty && e.Files["boundary-terminal"] == "done" {
				terminal = true
			}
		}
		if e.Phase == "answer" && clean && answerDone {
			answerClosed = true
		}
		if e.Phase == "question" && clean && updates["question"] > 0 {
			question = true
		}
		if e.Phase == "recovery" && clean && terminal && updates["recovery"] > 0 {
			recovered = true
		}
		if e.Phase == "resumed" && clean && resumed && updates["resumed"] > 0 {
			resumeDone = true
		}
	}
	if !question || !answerClosed || !ruleFault || !cliFault || !recovered || !resumeDone || len(pending) != 0 {
		return fmt.Errorf("incomplete boundaries: question=%v answer=%v rule=%v CLI=%v recovery=%v resume=%v pending=%d", question, answerDone, ruleFault, cliFault, recovered, resumeDone, len(pending))
	}
	return nil
}
func journeyBoundaryFixture() []journeyObservation {
	var events []journeyObservation
	phase, session, body := "question", "one", "initial precision-choice"
	state := minimal.Session{Intent: "same-id", Space: "default", RuleHash: "rules", Dirty: true}
	files := map[string]string{"aidlc/spaces/default/knowledge/rules/rule.md": "Rule"}
	emit := func(kind, id, command string, before, after minimal.Session, deny bool, err string) {
		in := minimal.HookInput{Event: kind, Session: session, ID: id}
		in.Input.Command = command
		copyFiles := map[string]string{}
		for k, v := range files {
			copyFiles[k] = v
		}
		output := map[string]any{}
		if deny {
			output["hookSpecificOutput"] = map[string]any{"permissionDecision": "deny"}
		}
		events = append(events, journeyObservation{Phase: phase, Input: in, Before: before, After: after, Output: output, Documents: map[string]string{"same-id": body}, Files: copyFiles, Error: err})
	}
	update := func(id string) {
		emit("PreToolUse", id, "", state, state, false, "")
		body += " recorded " + phase + id
		before := state
		state.Dirty = false
		emit("PostToolUse", id, "", before, state, false, "")
	}
	stop := func() { emit("Stop", "", "", state, state, false, "") }
	update("question")
	stop()
	phase = "answer"
	before := state
	state.Dirty = true
	emit("UserPromptSubmit", "", "", before, state, false, "")
	update("answer-first")
	before = state
	state.Dirty = true
	state.Tool = "followup"
	emit("PreToolUse", "followup", "printf changed > boundary-followup", before, state, false, "")
	before = state
	state.Tool = ""
	files["boundary-followup"] = "changed"
	emit("PostToolUse", "followup", "", before, state, false, "")
	update("answer-second")
	stop()
	phase = "rule-fault"
	state.Dirty = true
	delete(files, "aidlc/spaces/default/knowledge/rules/rule.md")
	emit("PreToolUse", "rule", "", state, state, true, "")
	phase = "cli-fault"
	emit("SessionStart", "", "", state, state, false, "fork/exec missing: no such file or directory")
	phase = "recovery"
	files["aidlc/spaces/default/knowledge/rules/rule.md"] = "Rule"
	state.RuleHash = ""
	emit("PreToolUse", "bind", "", state, state, false, "")
	before = state
	state.RuleHash = "rules"
	emit("PostToolUse", "bind", "", before, state, false, "")
	before = state
	state.Tool = "long"
	emit("PreToolUse", "long", journeyLongCommand, before, state, false, "")
	emit("PreToolUse", "update", "aidlc kdr update same-id", state, state, true, "")
	before = state
	state.Tool = ""
	files["boundary-terminal"] = "done"
	emit("PostToolUse", "long", "", before, state, false, "")
	update("recovery")
	stop()
	phase, session = "resumed", "two"
	state = minimal.Session{Dirty: true}
	emit("PreToolUse", "bind", "", state, state, false, "")
	before = state
	state = minimal.Session{Intent: "same-id", Space: "default", RuleHash: "rules", Dirty: true}
	emit("PostToolUse", "bind", "", before, state, false, "")
	update("resumed")
	stop()
	return events
}
func TestMinimalJourneyBoundaryEvidence(t *testing.T) {
	if err := verifyJourneyBoundaryEvents(journeyBoundaryFixture()); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"question", "answer", "rule-fault", "cli-fault", "recovery", "resumed"} {
		t.Run(phase, func(t *testing.T) {
			var broken []journeyObservation
			for _, event := range journeyBoundaryFixture() {
				if event.Phase != phase {
					broken = append(broken, event)
				}
			}
			if verifyJourneyBoundaryEvents(broken) == nil {
				t.Fatal("accepted missing live checkpoint")
			}
		})
	}
	for _, name := range []string{"wrong_question_session", "early_terminal", "wrong_resume_id", "no_competition"} {
		t.Run(name, func(t *testing.T) {
			events := journeyBoundaryFixture()
			for i := range events {
				e := &events[i]
				switch name {
				case "wrong_question_session":
					if e.Phase == "answer" {
						e.Input.Session = "other"
					}
				case "early_terminal":
					if e.Input.ID == "update" {
						e.Files = map[string]string{"boundary-terminal": "done"}
					}
				case "wrong_resume_id":
					if e.Phase == "resumed" {
						e.After.Intent = "other-id"
					}
				case "no_competition":
					if e.Input.ID == "update" {
						e.Input.Event = "ignored"
					}
				}
			}
			if verifyJourneyBoundaryEvents(events) == nil {
				t.Fatal("accepted invalid boundary evidence")
			}
		})
	}
}

func TestMinimalJourneyBoundariesLive(t *testing.T) {
	if os.Getenv("AIDLC_MINIMAL_JOURNEY_LIVE") != "1" {
		t.Skip("set AIDLC_MINIMAL_JOURNEY_LIVE=1 for model boundary checkpoints")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Fatal("requires fixed darwin/arm64")
	}
	if version := runMinimalProcess(t, ".", "codex", "--version"); strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed version required: %s", version)
	}
	evidence, err := os.MkdirTemp("", "aidlc-minimal-boundaries-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("preserved live boundary evidence: %s", evidence)
	root := filepath.Join(evidence, "repo")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	built := buildMinimalBinary(t)
	raw, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(evidence, "aidlc")
	if err := os.WriteFile(binary, raw, 0700); err != nil {
		t.Fatal(err)
	}
	runMinimalProcess(t, root, "git", "init", "--quiet")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(root, ".codex/hooks.json")
	config, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(config, &hooks); err != nil {
		t.Fatal(err)
	}
	relay := minimalProbeQuote(helper) + " -test.run='^TestMinimalJourneyRelay$' -- " + minimalProbeQuote(binary) + " " + minimalProbeQuote(root) + " " + minimalProbeQuote(evidence)
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, group := range groups.([]any) {
			for _, handler := range group.(map[string]any)["hooks"].([]any) {
				handler.(map[string]any)["command"] = relay
			}
		}
	}
	config, err = json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, config, 0600); err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(root, ".codex/config.toml"), "[features]\nhooks = true\n")
	load := func() []journeyObservation {
		t.Helper()
		paths, err := filepath.Glob(filepath.Join(evidence, "journey-*.json"))
		if err != nil {
			t.Fatal(err)
		}
		var events []journeyObservation
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var event journeyObservation
			if err := json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
		return events
	}
	run := func(phase, session, prompt string, allowFailure bool) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(evidence, "phase"), []byte(phase), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, phase+".prompt"), []byte(prompt), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
		defer cancel()
		args := []string{"exec", "--ignore-user-config", "--dangerously-bypass-hook-trust", "-s", "workspace-write", "-c", `approval_policy="never"`, "-m", "gpt-6-astra", "-c", `model_reasoning_effort="medium"`, "-c", minimalProbeTrustConfig(root), "-C", root}
		if session != "" {
			args = append(args, "resume", "--json", session, prompt)
		} else {
			args = append(args, "--json", prompt)
		}
		cmd := exec.CommandContext(ctx, "codex", args...)
		cmd.Dir = root
		cmd.WaitDelay = 5 * time.Second
		stdout, err := os.Create(filepath.Join(evidence, phase+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		stderr, err := os.Create(filepath.Join(evidence, phase+".stderr"))
		if err != nil {
			stdout.Close()
			t.Fatal(err)
		}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		runErr := cmd.Run()
		stdout.Close()
		stderr.Close()
		metadata, _ := json.Marshal(map[string]string{"phase": phase, "resume_session": session, "process_error": journeyError(runErr)})
		if err := os.WriteFile(filepath.Join(evidence, phase+".process.json"), metadata, 0600); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil || (runErr != nil && !allowFailure) {
			t.Fatalf("checkpoint %s failed: %v; evidence %s", phase, runErr, evidence)
		}
	}
	run("question", "", "Use the installed aidlc skill and default Space. Start one Intent named Precision decision. We must decide the precision-choice: integer-only or decimal arithmetic. Ask me which one before implementation, preserving the unresolved choice for the next turn. Do not choose on my behalf. Do not implement code yet.", false)
	session := ""
	for _, event := range load() {
		if event.Phase == "question" && event.Input.Event == "SessionStart" {
			session = event.Input.Session
			break
		}
	}
	if session == "" {
		t.Fatal("question checkpoint has no session ID")
	}
	run("answer", session, "Choose integer-only arithmetic. Use the installed workflow to record that answer. After recording it, perform exactly one further general Bash command: printf changed > boundary-followup . Then finish the turn through the installed workflow. This later command is an intentional check that subsequent work needs another record.", false)
	rulePath := filepath.Join(root, "aidlc/spaces/default/knowledge/rules/rule.md")
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rulePath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.WriteFile(rulePath, rule, 0644) })
	run("rule-fault", session, "A test operator removed a required Rule. Attempt to resume the existing Intent using the installed skill, then attempt the general command touch rule-forbidden once to observe the hook denial. Do not recreate the Rule or the canary by another method. Diagnose the missing Rule and stop; do not claim successful recording.", false)
	if _, err := os.Stat(filepath.Join(root, "rule-forbidden")); !os.IsNotExist(err) {
		t.Fatalf("Rule fault canary: %v", err)
	}
	if err := os.WriteFile(rulePath, rule, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(binary, binary+".offline"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := os.Stat(binary); os.IsNotExist(err) {
			os.Rename(binary+".offline", binary)
		}
	})
	run("cli-fault", session, "The test operator restored the Rule and temporarily removed the configured aidlc executable. Observe the hook or CLI process diagnostic. Stop on this fault, do not replace the executable, bypass the workflow, or claim that the outstanding record was saved.", true)
	if err := os.Rename(binary+".offline", binary); err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, filepath.Join(root, "boundary-edit.txt"), "before\n")
	run("recovery", session, "The original Rule and aidlc executable are restored. Resume the same Intent via the installed skill. First perform an intentional edit-failure diagnostic by calling apply_patch with this exact code-mode input: "+"text(await tools.apply_patch("+string(mustJourneyJSON(journeyFailedPatch))+"));"+" . After the verification error returns, use the installed skill to recover the same session and Intent without user intervention, then retry with this exact patch: "+journeyRetryPatch+" . Do not modify boundary-edit.txt by any other method. For an intentional concurrency diagnostic, first prepare a normal KDR update draft and expected hash using the installed skill. Then start Bash command `"+journeyLongCommand+"` with yield_time_ms=1. While it is pending, attempt exactly one KDR update using the installed workflow; expect rejection and do not retry until the process finishes. Poll the original session_id with write_stdin until terminal. Then record the fault recovery and concurrency result using the installed workflow. Do not create boundary-terminal by another command.", false)
	run("resumed", "", "This is a new conversation after an interruption. Use the installed aidlc skill in default Space to resume Precision decision from its existing KDR, inspect the saved decision and fault recovery evidence, and record the resumed findings. Do not create a second Intent.", false)
	transcriptPath := ""
	for _, event := range load() {
		if event.Phase == "recovery" && event.TranscriptPath != "" {
			transcriptPath = event.TranscriptPath
			break
		}
	}
	transcript, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("read edit failure transport: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "edit-failure-transcript.jsonl"), transcript, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyJourneyBoundaryEvents(load()); err != nil {
		t.Fatal(err)
	}
	if err := verifyJourneyEditRecovery(load(), transcript); err != nil {
		t.Fatal(err)
	}
}

func TestMinimalJourneyBoundaryRequiresAnswerStop(t *testing.T) {
	events := journeyBoundaryFixture()
	for i := range events {
		if events[i].Phase == "answer" && events[i].Input.Event == "Stop" {
			events[i].Input.Event = "ignored"
		}
	}
	if verifyJourneyBoundaryEvents(events) == nil {
		t.Fatal("accepted answer without clean Stop")
	}
}
