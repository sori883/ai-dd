//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	codex "github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/app"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type memoryLiveRecord struct {
	Raw, Output    json.RawMessage
	Document, Body []byte
	Bound          bool
}

func memoryLiveArgs(binary, command string) ([]string, bool) {
	args, ok := flowShellWords(command)
	if !ok {
		return nil, false
	}
	if len(args) == 3 && (args[0] == "/bin/zsh" || args[0] == "/bin/bash") && args[1] == "-lc" {
		args, ok = flowShellWords(args[2])
	}
	return args, ok && (len(args) > 1 && args[0] == binary ||
		len(args) == 2 && args[0] == "cat" && args[1] == ".agents/skills/aidlc-okf/SKILL.md")
}
func verifyMemoryLive(binary string, transport []byte, records []memoryLiveRecord) error {
	fail := func() error {
		return fmt.Errorf("missing actual skill read, unselected help and body-only create/update evidence")
	}
	type execution struct {
		exit   int
		output string
	}
	executions := map[string]execution{}
	session := ""
	for _, line := range bytes.Split(transport, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct {
			Type    string
			Session string `json:"thread_id"`
			Item    struct {
				Type, Command string
				Exit          *int   `json:"exit_code"`
				Output        string `json:"aggregated_output"`
			}
		}
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Type == "thread.started" {
			session = event.Session
		}
		if event.Type != "item.completed" || event.Item.Type != "command_execution" || event.Item.Exit == nil {
			continue
		}
		args, ok := memoryLiveArgs(binary, event.Item.Command)
		if !ok {
			continue
		}
		executions[session+"/"+strings.Join(args, "\x00")] = execution{*event.Item.Exit, event.Item.Output}
	}
	template, err := codex.Files.ReadFile("aidlc-okf/SKILL.md")
	if err != nil {
		return err
	}
	expectedSkill := strings.ReplaceAll(string(template), "@@BINARY@@", "'"+strings.ReplaceAll(binary, "'", "'\"'\"'")+"'")
	skillRead := map[string]bool{}
	pending := map[string]memoryLiveRecord{}
	helped, created, updated := false, false, false
	var previous okfmemory.Document
	previousHash := ""
	for _, record := range records {
		var input app.HookInput
		if json.Unmarshal(record.Raw, &input) != nil {
			return fail()
		}
		args, ok := memoryLiveArgs(binary, input.Input.Command)
		if input.Tool != "Bash" || !ok {
			continue
		}
		execution, ran := executions[input.Session+"/"+strings.Join(args, "\x00")]
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
			if input.ID == "" || decision.Specific.Decision == "deny" {
				continue
			}
			if help, ok := cli.Help(args[1:]); ok && !record.Bound && ran && execution.exit == 0 && strings.TrimSpace(execution.output) == strings.TrimSpace(help) {
				helped = true
			}
			pending[key] = record
			continue
		}
		if input.Event != "PostToolUse" {
			continue
		}
		if args[0] == "cat" {
			pre, ok := pending[key]
			if !ok || !pre.Bound || !ran || execution.exit != 0 || execution.output != expectedSkill {
				return fail()
			}
			var before app.HookInput
			if json.Unmarshal(pre.Raw, &before) != nil {
				return fail()
			}
			preArgs, ok := memoryLiveArgs(binary, before.Input.Command)
			if !ok || !reflect.DeepEqual(preArgs, args) {
				return fail()
			}
			delete(pending, key)
			skillRead[input.Session] = true
			continue
		}
		r, err := cli.ParseCommand(args[1:])
		if err != nil || r.Command != "memory" || (r.Action != "create" && r.Action != "update") {
			continue
		}
		if r.Target != "codekb/live-note" {
			continue
		}
		pre, ok := pending[key]
		if !ok || !pre.Bound || !ran || execution.exit != 0 || !helped || !skillRead[input.Session] {
			return fail()
		}
		var before app.HookInput
		if json.Unmarshal(pre.Raw, &before) != nil {
			return fail()
		}
		preArgs, ok := memoryLiveArgs(binary, before.Input.Command)
		if !ok || !reflect.DeepEqual(preArgs, args) {
			return fail()
		}
		delete(pending, key)
		var result struct{ Hash string }
		if json.Unmarshal([]byte(execution.output), &result) != nil || result.Hash != filestore.Hash(record.Document) {
			return fail()
		}
		doc, err := okfmemory.Parse(record.Document)
		if err != nil || doc.Body != string(pre.Body) || !bytes.Equal(pre.Body, record.Body) {
			return fail()
		}
		if strings.HasPrefix(doc.Body, "---") || doc.String("title") != "Arithmetic" {
			return fail()
		}
		generated, ok := doc.Metadata["generated"].(map[string]any)
		if !ok || generated["by"] != r.Actor || generated["at"] == nil {
			return fail()
		}
		if r.Action == "create" {
			if created || !strings.Contains(doc.Body, "FIRST-BODY") {
				return fail()
			}
			created = true
		} else {
			if !created || r.Expect != previousHash || !strings.Contains(doc.Body, "SECOND-BODY") || doc.Body == previous.Body {
				return fail()
			}
			for _, key := range []string{"type", "title", "description", "tags", "audience"} {
				if !reflect.DeepEqual(doc.Metadata[key], previous.Metadata[key]) {
					return fail()
				}
			}
			updated = true
		}
		previous, previousHash = doc, result.Hash
	}
	if !created || !updated {
		return fail()
	}
	return nil
}

func TestMemoryMetadataCommandEvidence(t *testing.T) {
	binary := "/bin/aidlc"
	help := binary + " memory create --help"
	skillCommand := "cat .agents/skills/aidlc-okf/SKILL.md"
	template, err := codex.Files.ReadFile("aidlc-okf/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	skill := strings.ReplaceAll(string(template), "@@BINARY@@", "'/bin/aidlc'")
	create := binary + " memory create codekb/live-note --space default --body-file body.md --actor process:codex --type Design --title Arithmetic --description Current"
	update := binary + " memory update codekb/live-note --space default --body-file body.md --actor process:codex --expect first"
	document := func(body string) []byte {
		return []byte("---\ntype: Design\ntitle: Arithmetic\ndescription: Current\ntags: [arithmetic]\naudience: maintainers\ngenerated: {by: 'process:codex', at: '2026-09-08T00:00:00Z'}\n---\n" + body)
	}
	first, second := document("FIRST-BODY\n"), document("SECOND-BODY\n")
	update = strings.Replace(update, "--expect first", "--expect "+filestore.Hash(first), 1)
	for _, mode := range []string{"valid", "missing skill", "denied skill", "mismatched skill", "failed skill", "missing skill post", "late skill", "missing update", "bound help", "wrong document", "wrong body", "denied create", "self report"} {
		t.Run(mode, func(t *testing.T) {
			var records []memoryLiveRecord
			add := func(event, id, command string, doc, body []byte, bound bool) {
				in := app.HookInput{Event: event, Session: "session", Turn: "turn", ID: id, Tool: "Bash"}
				in.Input.Command = command
				raw, _ := json.Marshal(in)
				records = append(records, memoryLiveRecord{Raw: raw, Output: json.RawMessage(`{}`), Document: doc, Body: body, Bound: bound})
			}
			add("PreToolUse", "help", help, nil, nil, mode == "bound help")
			addSkill := func() {
				if mode == "missing skill" {
					return
				}
				add("PreToolUse", "skill", skillCommand, nil, nil, true)
				if mode == "denied skill" {
					records[len(records)-1].Output = json.RawMessage(`{"hookSpecificOutput":{"permissionDecision":"deny"}}`)
				}
				if mode != "missing skill post" {
					add("PostToolUse", "skill", skillCommand, nil, nil, true)
				}
			}
			if mode != "late skill" {
				addSkill()
			}
			add("PreToolUse", "create", create, nil, []byte("FIRST-BODY\n"), true)
			if mode == "denied create" {
				records[len(records)-1].Output = json.RawMessage(`{"hookSpecificOutput":{"permissionDecision":"deny"}}`)
			}
			saved := first
			if mode == "wrong document" {
				saved = second
			}
			add("PostToolUse", "create", create, saved, []byte("FIRST-BODY\n"), true)
			if mode == "late skill" {
				addSkill()
			}
			if mode != "missing update" {
				add("PreToolUse", "update", update, first, []byte("SECOND-BODY\n"), true)
				body := []byte("SECOND-BODY\n")
				if mode == "wrong body" {
					body = []byte("frontmatter injected")
				}
				add("PostToolUse", "update", update, second, body, true)
			}
			var wire bytes.Buffer
			write := func(value any) { raw, _ := json.Marshal(value); wire.Write(raw); wire.WriteByte('\n') }
			write(map[string]any{"type": "thread.started", "thread_id": "session"})
			expected, _ := cli.Help([]string{"memory", "create", "--help"})
			skillOutput := skill
			if mode == "mismatched skill" {
				skillOutput = "other skill"
			}
			for _, item := range []struct{ cmd, out string }{{help, expected}, {skillCommand, skillOutput}, {create, `{"hash":"` + filestore.Hash(first) + `"}`}, {update, `{"hash":"` + filestore.Hash(second) + `"}`}} {
				if mode == "missing skill" && item.cmd == skillCommand {
					continue
				}
				exitCode := 0
				if mode == "failed skill" && item.cmd == skillCommand {
					exitCode = 1
				}
				write(map[string]any{"type": "item.completed", "item": map[string]any{"type": "command_execution", "command": item.cmd, "exit_code": exitCode, "aggregated_output": item.out}})
			}
			if mode == "self report" {
				wire.Reset()
				write(map[string]any{"type": "item.completed", "item": map[string]any{"type": "agent_message", "text": "done"}})
			}
			err := verifyMemoryLive(binary, wire.Bytes(), records)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("evidence result = %v", err)
			}
		})
	}
}

// TestMemoryMetadataLiveHelper relays the actual product hook and records fixture snapshots.
func TestMemoryMetadataLiveHelper(t *testing.T) {
	args := flowLiveArgs()
	if len(args) != 1 {
		t.Skip("live hook subprocess only")
	}
	var cfg flowLiveConfig
	raw, err := os.ReadFile(args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cfg.Binary, "__hook", "--project-dir", cfg.Root)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var hook app.HookInput
	if err := json.Unmarshal(input, &hook); err != nil {
		t.Fatal(err)
	}
	session, err := (app.Service{Root: cfg.Root, Binary: cfg.Binary}).Inspect(hook.Session)
	if err != nil {
		t.Fatal(err)
	}
	record := memoryLiveRecord{Raw: input, Output: output, Bound: session.Intent != ""}
	record.Document, _ = filestore.ReadFile(cfg.Root, "aidlc/spaces/default/knowledge/codekb/live-note.md")
	if args, ok := memoryLiveArgs(cfg.Binary, hook.Input.Command); ok {
		if request, err := cli.ParseCommand(args[1:]); err == nil && request.BodyFile != "" {
			name := request.BodyFile
			if filepath.IsAbs(name) {
				name, err = filepath.Rel(cfg.Root, name)
			}
			if err == nil {
				record.Body, _ = filestore.ReadFile(cfg.Root, filepath.ToSlash(name))
			}
		}
	}
	if err := flowLiveSave(cfg.Evidence, "memory-hook", record); err != nil {
		t.Fatal(err)
	}
	fmt.Print(string(output))
	os.Exit(0)
}

func TestMemoryMetadataLive(t *testing.T) {
	if os.Getenv("AIDLC_MEMORY_LIVE") != "1" {
		t.Skip("set AIDLC_MEMORY_LIVE=1 for actual model evidence")
	}
	version, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex unavailable: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-memory-live-")
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
	writeAIDLCFixture(t, filepath.Join(root, "arithmetic.go"), "package arithmetic\nfunc Add(a,b int)int{return a+b}\n")
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
	prompt := `Use the installed aidlc skill. Before selecting or creating an Intent, read memory create help through the installed binary. Then follow the skill to select an Intent, read its project Rules, explicitly read .agents/skills/aidlc/SKILL.md and .agents/skills/aidlc-cli/SKILL.md, then run exactly cat .agents/skills/aidlc-okf/SKILL.md in its own tool call before any memory create/update, and obtain the current deployed procedure. Record the current behavior of arithmetic.go in Concept codekb/live-note: title Arithmetic, type Design, tag arithmetic, extension audience=maintainers. The first body must include FIRST-BODY and describe Add. Then read update help, revise only the body to include SECOND-BODY instead and add a concrete example; preserve its metadata. Use actor process:codex. Inspect the saved document afterward. Use one literal CLI command per tool call so its result can be observed. Do not edit the product hooks or Rules. Stop after the Knowledge update; this task does not require the full implementation journey.`
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
	if err := verifyMemoryLive(binary, transport, records); err != nil {
		t.Fatalf("%v; evidence %s", err, evidence)
	}
	final, err := filestore.ReadFile(root, "aidlc/spaces/default/knowledge/codekb/live-note.md")
	if err != nil {
		t.Fatal(err)
	}
	var last []byte
	for _, record := range records {
		if len(record.Document) > 0 {
			last = record.Document
		}
	}
	if !bytes.Equal(final, last) {
		t.Fatal("final document differs from observed saved bytes")
	}
}
