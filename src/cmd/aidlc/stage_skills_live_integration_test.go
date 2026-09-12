//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	codex "github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/naturaljapanese"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func verifyStageSkillsEvidence(binary string, transport []byte, records []memoryLiveRecord) error {
	fail := func() error {
		return fmt.Errorf("missing actual skill read, CLI exit/stdout or paired Pre/Post evidence")
	}
	type execution struct {
		exit   int
		output string
	}
	runs := map[string]execution{}
	session := ""
	normalize := func(command string) string {
		args, ok := flowShellWords(command)
		if !ok {
			return ""
		}
		if len(args) == 3 && (args[0] == "/bin/zsh" || args[0] == "/bin/bash") && args[1] == "-lc" {
			args, ok = flowShellWords(args[2])
			if !ok {
				return ""
			}
		}
		return strings.Join(args, "\x00")
	}
	for _, line := range bytes.Split(transport, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev struct {
			Type    string
			Session string `json:"thread_id"`
			Item    struct {
				Type, Command string
				Exit          *int   `json:"exit_code"`
				Output        string `json:"aggregated_output"`
			}
		}
		if json.Unmarshal(line, &ev) != nil {
			return fail()
		}
		if ev.Type == "thread.started" {
			session = ev.Session
		}
		if ev.Type == "item.completed" && ev.Item.Type == "command_execution" && ev.Item.Exit != nil {
			runs[session+"/"+normalize(ev.Item.Command)] = execution{*ev.Item.Exit, ev.Item.Output}
		}
	}
	commands := []string{"cat\x00.agents/skills/aidlc-grilling/SKILL.md", "cat\x00.agents/skills/natural-japanese-go/SKILL.md", binary + "\x00--json\x00text.md"}
	pending := map[string]string{}
	seen := map[string]map[string]bool{}
	for _, record := range records {
		var input app.HookInput
		if json.Unmarshal(record.Raw, &input) != nil {
			return fail()
		}
		command := normalize(input.Input.Command)
		if input.Tool != "Bash" {
			continue
		}
		index := -1
		for i, c := range commands {
			if command == c {
				index = i
			}
		}
		if index < 0 {
			continue
		}
		var decision struct {
			Specific struct {
				Decision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal(record.Output, &decision) != nil {
			return fail()
		}
		key := input.Session + "/" + input.ID
		if input.Event == "PreToolUse" {
			if input.ID != "" && record.Bound && decision.Specific.Decision != "deny" {
				pending[key] = command
			}
			continue
		}
		if input.Event != "PostToolUse" {
			continue
		}
		run, ok := runs[input.Session+"/"+command]
		if pending[key] != command || !record.Bound || !ok || run.exit != 0 {
			return fail()
		}
		delete(pending, key)
		if seen[input.Session] == nil {
			seen[input.Session] = map[string]bool{}
		}
		if index < 2 {
			name := "stage-skills/aidlc-grilling/SKILL.md"
			if index == 1 {
				name = "stage-skills/natural-japanese-go/SKILL.md"
			}
			expected, err := codex.Files.ReadFile(name)
			if err != nil {
				return err
			}
			if run.output != string(expected) {
				return fail()
			}
		} else {
			if !seen[input.Session][commands[0]] || !seen[input.Session][commands[1]] {
				return fail()
			}
			var report naturaljapanese.Report
			if json.Unmarshal([]byte(run.output), &report) != nil || report.SchemaVersion != 1 || report.Engine != "Kagome v2.11.0" || report.Dictionary != "UniDic v1.2.6" || report.File != "text.md" || len(report.Findings) == 0 || report.Findings[0].Category != "forbidden_phrase" {
				return fail()
			}
		}
		seen[input.Session][command] = true
	}
	for _, session := range seen {
		if session[commands[0]] && session[commands[1]] && session[commands[2]] {
			return nil
		}
	}
	return fail()
}

func TestStageSkillsEvidence(t *testing.T) {
	for _, mode := range []string{"valid", "no post", "no transport", "failed exit", "wrong stdout", "denied", "unbound"} {
		t.Run(mode, func(t *testing.T) {
			var records []memoryLiveRecord
			transport := []byte("{\"type\":\"thread.started\",\"thread_id\":\"s\"}\n")
			commands := []string{"cat .agents/skills/aidlc-grilling/SKILL.md", "cat .agents/skills/natural-japanese-go/SKILL.md", "/opt/natural-japanese-go --json text.md"}
			for i, cmd := range commands {
				out := stageEvidenceOutput(t, i)
				if mode == "wrong stdout" {
					out = "claimed success"
				}
				exit := 0
				if mode == "failed exit" {
					exit = 1
				}
				event := map[string]any{"type": "item.completed", "item": map[string]any{"type": "command_execution", "command": cmd, "exit_code": exit, "aggregated_output": out}}
				line, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				transport = append(transport, append(line, '\n')...)
				for _, ev := range []string{"PreToolUse", "PostToolUse"} {
					if mode == "no post" && ev == "PostToolUse" {
						continue
					}
					raw := fmt.Sprintf(`{"hook_event_name":%q,"session_id":"s","tool_name":"Bash","tool_use_id":%q,"tool_input":{"command":%q}}`, ev, fmt.Sprint(i), cmd)
					decision := json.RawMessage(`{}`)
					if mode == "denied" {
						decision = json.RawMessage(`{"hookSpecificOutput":{"permissionDecision":"deny"}}`)
					}
					records = append(records, memoryLiveRecord{Raw: json.RawMessage(raw), Output: decision, Bound: mode != "unbound"})
				}
			}
			if mode == "no transport" {
				transport = nil
			}
			err := verifyStageSkillsEvidence("/opt/natural-japanese-go", transport, records)
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}
func stageEvidenceOutput(t *testing.T, i int) string {
	t.Helper()
	if i == 2 {
		return `{"schema_version":1,"engine":"Kagome v2.11.0","dictionary":"UniDic v1.2.6","file":"text.md","stats":{},"findings":[{"line":1,"category":"forbidden_phrase","excerpt":"非常に重要","severity":"warn","detail":"test"}]}`
	}
	name := "stage-skills/aidlc-grilling/SKILL.md"
	if i == 1 {
		name = "stage-skills/natural-japanese-go/SKILL.md"
	}
	raw, err := codex.Files.ReadFile(name)
	if err != nil {
		return "not installed"
	}
	return string(raw)
}

// Uses the existing explicit fixture trust helper; this is not normal-user trust evidence.
func TestStageSkillsLive(t *testing.T) {
	if os.Getenv("AIDLC_STAGE_SKILLS_LIVE") != "1" {
		t.Skip("set AIDLC_STAGE_SKILLS_LIVE=1 for actual model evidence")
	}
	naturalBinary := os.Getenv("AIDLC_NATURAL_JAPANESE_BINARY")
	if !filepath.IsAbs(naturalBinary) {
		t.Fatal("AIDLC_NATURAL_JAPANESE_BINARY must name an absolute built Go CLI")
	}
	version, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex unavailable: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-stage-skills-live-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent raw evidence: %s", evidence)
	root := filepath.Join(evidence, "project")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := filepath.EvalSymlinks(buildAIDLCBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	runFixtureProcess(t, root, "git", "init", "-q")
	writeAIDLCFixture(t, filepath.Join(root, "text.md"), "非常に重要。\n")
	if _, err := install.Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	cfg := flowLiveConfig{Root: root, Binary: binary, Evidence: evidence}
	cfgPath := filepath.Join(evidence, "config.json")
	config, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, config, 0600); err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	helper := filepath.Join(evidence, "memory-hook")
	script := "#!/bin/sh\nexec " + quote(testBinary) + " -test.run='^TestMemoryMetadataLiveHelper$' -- " + quote(cfgPath) + "\n"
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(root, ".codex/hooks.json")
	hooksRaw, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
		t.Fatal(err)
	}
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, group := range groups.([]any) {
			for _, handler := range group.(map[string]any)["hooks"].([]any) {
				handler.(map[string]any)["command"] = quote(helper)
			}
		}
	}
	hooksRaw, _ = json.Marshal(hooks)
	if err := os.WriteFile(hooksPath, hooksRaw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	prompt := `Use the installed aidlc skill and existing hooks. Select/create an Intent, read its project Rules, and follow the required initialization so a normal tool command is permitted in a begun stage. Do not alter hooks or Rules. Then run these three commands in separate tool calls, in this order: cat .agents/skills/aidlc-grilling/SKILL.md ; cat .agents/skills/natural-japanese-go/SKILL.md ; ` + quote(naturalBinary) + ` --json text.md . Do not combine the commands with semicolons. Inspect their output. Stop after this limited fixture, without claiming the full Intent completed.`

	if _, err := flowRunModel(ctx, cfg, root, "memory", prompt, "workspace-write"); err != nil {
		t.Fatalf("model failed: %v; evidence %s", err, evidence)
	}
	files, err := filepath.Glob(filepath.Join(evidence, "memory-hook-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var records []memoryLiveRecord
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var record memoryLiveRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	transport, err := os.ReadFile(filepath.Join(evidence, "memory.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyStageSkillsEvidence(naturalBinary, transport, records); err != nil {
		t.Fatalf("%v; evidence %s", err, evidence)
	}
}
