//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/minimal"
)

func relocationSelected(binary, id string, records []memoryLiveRecord) string {
	pending := map[string]string{}
	for _, r := range records {
		var in minimal.HookInput
		if json.Unmarshal(r.Raw, &in) != nil {
			continue
		}
		args, ok := memoryLiveArgs(binary, in.Input.Command)
		if !ok {
			continue
		}
		request, err := cli.ParseMinimal(args[1:])
		if err != nil || request.Command != "intent" || request.Action != "switch" || request.IntentID == nil || *request.IntentID != id || request.Session != in.Session {
			continue
		}
		var decision struct {
			Specific struct {
				Decision string `json:"permissionDecision"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal(r.Output, &decision) != nil || decision.Specific.Decision == "deny" {
			continue
		}
		key := in.Session + "/" + in.ID
		if in.Event == "PreToolUse" && in.ID != "" {
			pending[key] = in.Input.Command
		}
		if in.Event == "PostToolUse" && r.Bound && pending[key] == in.Input.Command {
			return in.Session
		}
	}
	return ""
}
func TestRelocationCommandSelectionEvidence(t *testing.T) {
	binary, id := "/new/aidlc", strings.Repeat("a", 32)
	cmd := binary + " intent switch --id " + id + " --space default --session s"
	record := func(event string) memoryLiveRecord {
		in := minimal.HookInput{Event: event, Session: "s", ID: "call", Tool: "Bash"}
		in.Input.Command = cmd
		raw, _ := json.Marshal(in)
		return memoryLiveRecord{Raw: raw, Output: json.RawMessage(`{}`), Bound: true}
	}
	records := []memoryLiveRecord{record("PreToolUse"), record("PostToolUse")}
	if relocationSelected(binary, id, records) != "s" {
		t.Fatal("valid selected evidence missing")
	}
	if relocationSelected(binary, strings.Repeat("b", 32), records) != "" || relocationSelected(binary, id, records[:1]) != "" {
		t.Fatal("false selection evidence")
	}
}
func TestRelocationLive(t *testing.T) {
	if os.Getenv("AIDLC_RELOCATION_LIVE") != "1" {
		t.Skip("set AIDLC_RELOCATION_LIVE=1 for relocated actual model evidence")
	}
	version, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex unavailable: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-relocation-live-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent raw evidence: %s", evidence)
	source := filepath.Join(evidence, "source")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := filepath.EvalSymlinks(buildMinimalBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	f := operationsFixture{t, binary, source}
	f.git("init", "-q")
	f.ok("install", "codex", "--project-dir", source)
	writeMinimalFixture(t, filepath.Join(source, "arithmetic.go"), "package arithmetic\nfunc Add(a,b int)int{return a+b}\n")
	st := f.create("Relocated arithmetic knowledge")
	f.commit("source knowledge work")
	clone := filepath.Join(evidence, "clone")
	f.git("clone", "-q", source, clone)
	g := operationsFixture{t, relocationBinary(t, binary), clone}
	original := relocationSnapshot(t, source)
	g.ok("install", "codex", "--relocate", "--project-dir", clone, "--from-project-dir", source, "--from-binary", binary)
	hooksPath := filepath.Join(clone, ".codex/hooks.json")
	relocated := operationsRead(t, hooksPath)
	if err := os.WriteFile(filepath.Join(evidence, "relocated-hooks.json"), relocated, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := flowLiveConfig{Root: clone, Binary: g.binary, Evidence: evidence}
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
	helper := filepath.Join(evidence, "relocation-hook")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nexec "+quote(testBinary)+" -test.run='^TestMemoryMetadataLiveHelper$' -- "+quote(cfgPath)+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(relocated, &hooks); err != nil {
		t.Fatal(err)
	}
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, group := range groups.([]any) {
			for _, handler := range group.(map[string]any)["hooks"].([]any) {
				h := handler.(map[string]any)
				args, ok := flowShellWords(h["command"].(string))
				if !ok || len(args) != 4 || args[0] != g.binary || args[1] != "__minimal-hook" || args[3] != clone {
					t.Fatal("relocated handler mismatch")
				}
				h["command"] = quote(helper)
			}
		}
	}
	relay, _ := json.Marshal(hooks)
	if err := os.WriteFile(hooksPath, relay, 0600); err != nil {
		t.Fatal(err)
	}
	prompt := fmt.Sprintf(`Use the installed aidlc skill. Before selecting an Intent read memory create help through the installed binary. Then select the existing Intent by ID %s in default Space; do not create another Intent. Follow the skill to read Rules and deployed procedure. Record arithmetic.go in knowledge/live-note with title Arithmetic, type Design, tag arithmetic and extension audience=maintainers. First body must include FIRST-BODY and describe Add. Read update help, then replace FIRST-BODY with SECOND-BODY and add an example, preserving metadata. Use actor process:codex. Inspect the saved document and stop. Use one literal CLI per tool call. Do not modify product hooks or Rules.`, st.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	if _, err := flowRunModel(ctx, cfg, clone, "relocation", prompt, "workspace-write"); err != nil {
		t.Fatalf("model failed: %v; evidence %s", err, evidence)
	}
	files, err := filepath.Glob(filepath.Join(evidence, "memory-hook-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var records []memoryLiveRecord
	for _, p := range files {
		var r memoryLiveRecord
		if err := json.Unmarshal(operationsRead(t, p), &r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	transport := operationsRead(t, filepath.Join(evidence, "relocation.jsonl"))
	if err := verifyMemoryLive(g.binary, transport, records); err != nil {
		t.Fatal(err)
	}
	session := relocationSelected(g.binary, st.ID, records)
	if session == "" || g.session(session).Intent != st.ID {
		t.Fatal("same Intent not bound in actual session")
	}
	final := operationsRead(t, filepath.Join(clone, "aidlc/spaces/default/knowledge/knowledge/live-note.md"))
	var last []byte
	for _, r := range records {
		if len(r.Document) > 0 {
			last = r.Document
		}
	}
	if !bytes.Equal(final, last) {
		t.Fatal("final document differs from recorded bytes")
	}
	if !reflect.DeepEqual(original, relocationSnapshot(t, source)) {
		t.Fatal("source changed during model operation")
	}
}
