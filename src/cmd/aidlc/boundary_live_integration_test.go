//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/install"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestBoundaryLive(t *testing.T) {
	if os.Getenv("AIDLC_BOUNDARY_LIVE") != "1" {
		t.Skip("set AIDLC_BOUNDARY_LIVE=1 for actual boundary evidence")
	}
	version, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("fixed Codex unavailable: %s %v", version, err)
	}
	evidence, err := os.MkdirTemp("", "aidlc-boundary-live-")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err = filepath.EvalSymlinks(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persistent raw evidence: %s", evidence)
	root := filepath.Join(evidence, "project")
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := filepath.EvalSymlinks(buildMinimalBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	runMinimalProcess(t, root, "git", "init", "-q")
	runMinimalProcess(t, root, "git", "-c", "user.name=Boundary", "-c", "user.email=boundary@example.invalid", "commit", "--allow-empty", "-qm", "base")
	if _, err = install.Codex(root, binary); err != nil {
		t.Fatal(err)
	}
	// A valid OKF document with a missing required section must be repaired through the CLI.
	writeMinimalFixture(t, filepath.Join(root, "aidlc/spaces/default/knowledge/codekb/current-analysis.md"), "---\ntype: CurrentAnalysis\ntitle: Current project\ndescription: Empty project\n---\n## 現状\nEmpty repository.\n")
	cfg := flowLiveConfig{Root: root, Binary: binary, Evidence: evidence}
	cfgPath := filepath.Join(evidence, "config.json")
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, cfgPath, string(raw))
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	helper := filepath.Join(evidence, "observer")
	script := "#!/bin/sh\nexec " + quote(testBinary) + " -test.run='^TestFlowLiveHelper$' -- hook " + quote(cfgPath) + "\n"
	if err = os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(root, ".codex/hooks.json")
	raw, err = os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var hooks map[string]any
	if err = json.Unmarshal(raw, &hooks); err != nil {
		t.Fatal(err)
	}
	for _, groups := range hooks["hooks"].(map[string]any) {
		for _, group := range groups.([]any) {
			for _, handler := range group.(map[string]any)["hooks"].([]any) {
				handler.(map[string]any)["command"] = quote(helper)
			}
		}
	}
	raw, err = json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	writeMinimalFixture(t, hooksPath, string(raw))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	prompt := `Use the installed aidlc skill and its deployed procedure. Create and select one Intent named Boundary live. Before beginning its stage, attempt exactly one literal Bash command: touch boundary-before.txt. This is an expected-denial probe; do not try another way to create that file. Then inspect and repair the existing current-analysis document using the documented memory CLI so discovery can begin. Start discovery through the documented CLI and then execute exactly one literal Bash command: touch boundary-after.txt. Do not alter hooks or Rules. Keep each CLI command in a separate tool call. Stop after the allowed canary; no full four-stage journey is required.`
	if _, err = flowRunModel(ctx, cfg, root, "boundary", prompt, "workspace-write"); err != nil {
		t.Fatalf("model: %v; evidence %s", err, evidence)
	}
	files, err := filepath.Glob(filepath.Join(evidence, "hook-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var records []boundaryObservation
	for _, file := range files {
		raw, err = os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var record boundaryObservation
		if err = json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	wire, err := os.ReadFile(filepath.Join(evidence, "boundary.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	executions := map[string]int{}
	session := ""
	for _, line := range bytes.Split(wire, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e struct {
			Type   string
			Thread string `json:"thread_id"`
			Item   struct {
				Type, Command string
				Exit          *int `json:"exit_code"`
			}
		}
		if err = json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		if e.Type == "thread.started" {
			session = e.Thread
		}
		if e.Type == "item.completed" && e.Item.Type == "command_execution" && e.Item.Exit != nil {
			command := boundaryEvidenceCommand(e.Item.Command)
			executions[session+"/"+command] = *e.Item.Exit
		}
	}
	_, beforeErr := os.Stat(filepath.Join(root, "boundary-before.txt"))
	_, afterErr := os.Stat(filepath.Join(root, "boundary-after.txt"))
	if beforeErr != nil && !os.IsNotExist(beforeErr) {
		t.Fatal(beforeErr)
	}
	if err = verifyBoundaryEvidence(binary, records, executions, beforeErr == nil, afterErr == nil); err != nil {
		t.Fatalf("%v; evidence %s", err, evidence)
	}
}
