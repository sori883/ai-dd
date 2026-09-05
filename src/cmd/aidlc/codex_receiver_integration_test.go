//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const codexReceiverJourneyGraphJSON = `[
  {"slug":"workspace-scaffold","number":"0.1","name":"Workspace Scaffold","phase":"initialization","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]},
  {"slug":"intent-capture","number":"1.1","name":"Intent Capture","phase":"ideation","execution":"ALWAYS","lead_agent":"aidlc-product-agent","support_agents":["aidlc-architect-agent"],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[{"artifact":"receiver-input","required":true}],"rules_in_context":[{"path":"receiver-rule-a.md","scope":"project"},{"path":"receiver-rule-b.md","scope":"project"}],"requires_stage":[]},
  {"slug":"next-stage","number":"1.2","name":"Next Stage","phase":"ideation","execution":"ALWAYS","lead_agent":"orchestrator","support_agents":[],"mode":"inline","scopes":["classic"],"enabled":true,"produces":[],"consumes":[],"requires_stage":[]}
]`

func TestCodexReceiverFreshPlacementJourney(t *testing.T) {
	moduleRoot := deliveryModuleRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "aidlc")
	build := exec.Command("go", "build", "-o", binaryPath, "./src/cmd/aidlc")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build aidlc: %v\n%s", err, output)
	}

	fixture := newCodexReceiverJourneyProject(t, moduleRoot)
	project := fixture.Project
	next := runCodexReceiverDirective(t, binaryPath, project, "next")
	if next.Kind != "load-steering" {
		t.Fatalf("next kind = %q, want load-steering", next.Kind)
	}
	if next.Parts < 2 {
		t.Fatalf("next parts = %d, want at least two rule chunks", next.Parts)
	}

	var receivedRules []codexReceiverRule
	wantPart := 1
	for next.Kind == "load-steering" {
		if next.Part != wantPart {
			t.Fatalf("load-steering part = %d, want part %d", next.Part, wantPart)
		}
		if next.ContinueToken == "" {
			t.Fatalf("load-steering part %d has empty continuation token", next.Part)
		}
		receivedRules = append(receivedRules, next.RulesContent...)
		wantPart++
		next = runCodexReceiverDirective(t, binaryPath, project, "continue", next.ContinueToken)
	}
	if next.Kind != "run-stage" {
		t.Fatalf("final directive kind = %q, want run-stage", next.Kind)
	}
	if len(receivedRules) == 0 {
		t.Fatal("received no rules_content before run-stage")
	}

	var wire codexReceiverRunStageWire
	if err := json.Unmarshal(next.Raw, &wire); err != nil {
		t.Fatalf("decode run-stage directive: %v", err)
	}
	wantInline := []string{
		".codex/agents/aidlc-product-agent.md",
		".codex/agents/aidlc-architect-agent.md",
	}
	if !equalStrings(wire.InlineContextPaths, wantInline) {
		t.Fatalf("inline_context_paths = %#v, want %#v", wire.InlineContextPaths, wantInline)
	}
	wantStageFile := ".codex/aidlc-common/stages/ideation/intent-capture.md"
	if wire.StageFile != wantStageFile {
		t.Fatalf("stage_file = %q, want %q", wire.StageFile, wantStageFile)
	}
	wantConsumes := []string{
		"aidlc/spaces/team/intents/build/ideation/intent-capture/receiver-input.md",
	}
	if !equalStrings(wire.Consumes, wantConsumes) {
		t.Fatalf("consumes = %#v, want %#v", wire.Consumes, wantConsumes)
	}

	readOrder := append([]string{}, wire.InlineContextPaths...)
	readOrder = append(readOrder, wire.StageFile)
	readOrder = append(readOrder, wire.Consumes...)
	wantSentinels := []string{
		fixture.Sentinels[wantInline[0]],
		fixture.Sentinels[wantInline[1]],
		fixture.Sentinels[wantStageFile],
		fixture.Sentinels[wantConsumes[0]],
	}
	if len(readOrder) != len(wantSentinels) {
		t.Fatalf("receiver read order = %#v, want four declared files", readOrder)
	}
	for index, filePath := range readOrder {
		if got := readJourneyFile(t, project, filePath); got != wantSentinels[index] {
			t.Errorf("receiver read %q = %q, want sentinel %q", filePath, got, wantSentinels[index])
		}
	}
	wantRulePaths := []string{"receiver-rule-a.md", "receiver-rule-b.md"}
	if len(receivedRules) != len(wantRulePaths) {
		t.Fatalf("rules_content entries = %d, want two complete rule documents", len(receivedRules))
	}
	for index, rule := range receivedRules {
		if rule.Path != wantRulePaths[index] {
			t.Errorf("rules_content[%d].path = %q, want %q", index, rule.Path, wantRulePaths[index])
		}
		if rule.Text != fixture.RuleDocuments[index].Text {
			t.Errorf("rules_content[%d].text changed bytes: got %d, want %d", index, len(rule.Text), len(fixture.RuleDocuments[index].Text))
		}
	}
}

func TestCodexReceiverRuleFixtureUsesCompactOrderedSentinels(t *testing.T) {
	fixture := newCodexReceiverJourneyProject(t, deliveryModuleRoot(t))
	if len(fixture.RuleDocuments) != 2 {
		t.Fatalf("rule documents = %d, want two", len(fixture.RuleDocuments))
	}
	if len(fixture.RuleSentinels) != len(fixture.RuleDocuments) {
		t.Fatalf("rule sentinels = %d, want one per rule document", len(fixture.RuleSentinels))
	}
	var totalBytes int
	var lastSentinels []string
	for index, rule := range fixture.RuleDocuments {
		if len(rule.Text) < 11*1024 || len(rule.Text) >= 12*1024 {
			t.Errorf("rule %d size = %d bytes, want [11, 12) KiB", index, len(rule.Text))
		}
		lines := strings.Split(strings.TrimRight(rule.Text, "\n"), "\n")
		if len(lines) == 0 || strings.TrimSpace(lines[len(lines)-1]) == "" {
			t.Fatalf("rule %d has no final non-empty line", index)
		}
		lastSentinel := lines[len(lines)-1]
		if strings.Count(rule.Text, lastSentinel) != 1 {
			t.Errorf("rule %d final sentinel occurs %d times, want once", index, strings.Count(rule.Text, lastSentinel))
		}
		if fixture.RuleSentinels[index] != lastSentinel {
			t.Errorf("rule %d expected sentinel = %q, want final line %q", index, fixture.RuleSentinels[index], lastSentinel)
		}
		lastSentinels = append(lastSentinels, lastSentinel)
		totalBytes += len(rule.Text)
	}
	if totalBytes <= 20*1024 {
		t.Errorf("rule fixture size = %d bytes, want more than 20 KiB", totalBytes)
	}
	if lastSentinels[0] == lastSentinels[1] {
		t.Error("rule sentinels are not distinct")
	}
}

func TestCodexReceiverLiveCommandIsRepositorySafeAndUsesReceiptFile(t *testing.T) {
	data, err := os.ReadFile("codex_receiver_integration_test.go")
	if err != nil {
		t.Fatalf("ReadFile(integration source): %v", err)
	}
	source := string(data)
	start := strings.LastIndex(source, "func TestCodexReceiverReadsDeliveredContext")
	end := strings.LastIndex(source, "type codexReceiverRule")
	if start < 0 || end <= start {
		t.Fatal("could not isolate live command implementation")
	}
	implementation := source[start:end]
	for _, phrase := range []string{
		`"--skip-git-repo-check"`,
		`"--sandbox", "workspace-write"`,
		`"-c", "approval_policy=\"never\""`,
		`"--output-last-message", receiptPath`,
	} {
		if !strings.Contains(implementation, phrase) {
			t.Errorf("live command implementation does not contain independent argument %q", phrase)
		}
	}
}

func TestCodexReceiverReadsDeliveredContext(t *testing.T) {
	if os.Getenv("AIDLC_CODEX_EXEC_LIVE") != "1" {
		t.Skip("set AIDLC_CODEX_EXEC_LIVE=1 to run the live Codex receipt")
	}

	moduleRoot := deliveryModuleRoot(t)
	fixture := newCodexReceiverJourneyProject(t, moduleRoot)
	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "aidlc")
	build := exec.Command("go", "build", "-o", binaryPath, "./src/cmd/aidlc")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build aidlc: %v\n%s", err, output)
	}

	schemaPath := filepath.Join(t.TempDir(), "receiver-receipt-schema.json")
	const schema = `{"type":"object","additionalProperties":false,"required":["rules","inline_context","stage_file","consumes"],"properties":{"rules":{"type":"array","items":{"type":"string"}},"inline_context":{"type":"array","items":{"type":"string"}},"stage_file":{"type":"string"},"consumes":{"type":"array","items":{"type":"string"}}}}`
	if err := os.WriteFile(schemaPath, []byte(schema), 0o600); err != nil {
		t.Fatalf("WriteFile(output schema): %v", err)
	}

	pathEnv := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathEnv)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	prompt := `Use the $aidlc skill explicitly now in the current project. Follow its receiver instructions exactly: obtain the delivery through the PATH aidlc command, preserve every delivered rules_content item in order, and continue with each opaque token until run-stage. Do not use search, globbing, directory listings, guessed files, or any source outside paths declared by the directive. For run-stage, read every declared inline_context_paths file completely first and wait for all of them, then read stage_file completely, then read every existing consumes file completely in declared order. Do not execute a stage or create outputs. Return exactly one JSON object matching the supplied output schema: rules is an array with one exact value equal to the last non-empty line of each delivered rules_content item in order; inline_context is the exact full text of each inline context file in read order; stage_file is the exact full text of that file; consumes is the exact full text of each consumed file in read order. Do not return path names, hashes, explanations, or markdown fences.`
	receiptPath := filepath.Join(t.TempDir(), "receiver-last-message.json")
	commandArgs := []string{
		"exec",
		"--ephemeral",
		"--skip-git-repo-check",
		"--sandbox", "workspace-write",
		"-c", "approval_policy=\"never\"",
		"--output-schema", schemaPath,
		"--output-last-message", receiptPath,
		prompt,
	}
	command := exec.CommandContext(ctx, "codex", commandArgs...)
	command.Dir = fixture.Project
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("codex exec timed out: %v", ctx.Err())
		}
		t.Fatalf("codex exec: %v\nstdout=%s\nstderr=%s", err, stdout.Bytes(), stderr.Bytes())
	}

	receiptBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("ReadFile(Codex last message): %v\nstdout=%s\nstderr=%s", err, stdout.Bytes(), stderr.Bytes())
	}
	var receipt codexReceiverLiveReceipt
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(receiptBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		t.Fatalf("decode Codex receipt: %v; receipt=%q; stdout=%q; stderr=%q", err, receiptBytes, stdout.String(), stderr.String())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("Codex receipt has trailing JSON: %v", err)
	}

	wantRules := append([]string(nil), fixture.RuleSentinels...)
	if !equalStrings(receipt.Rules, wantRules) {
		t.Fatalf("Codex rule sentinel receipt changed bytes/order: got %#v, want %#v", receipt.Rules, wantRules)
	}
	wantInline := []string{
		fixture.Sentinels[".codex/agents/aidlc-product-agent.md"],
		fixture.Sentinels[".codex/agents/aidlc-architect-agent.md"],
	}
	if !equalStrings(receipt.InlineContext, wantInline) {
		t.Fatalf("Codex inline_context = %#v, want %#v", receipt.InlineContext, wantInline)
	}
	if receipt.StageFile != fixture.Sentinels[".codex/aidlc-common/stages/ideation/intent-capture.md"] {
		t.Fatalf("Codex stage_file receipt = %q, want sentinel", receipt.StageFile)
	}
	wantConsumes := []string{
		fixture.Sentinels["aidlc/spaces/team/intents/build/ideation/intent-capture/receiver-input.md"],
	}
	if !equalStrings(receipt.Consumes, wantConsumes) {
		t.Fatalf("Codex consumes receipt = %#v, want %#v", receipt.Consumes, wantConsumes)
	}
}

type codexReceiverRule struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type codexReceiverDirective struct {
	Kind          string              `json:"kind"`
	Part          int                 `json:"part"`
	Parts         int                 `json:"parts"`
	RulesContent  []codexReceiverRule `json:"rules_content"`
	ContinueToken string              `json:"continue_token"`
	Raw           []byte              `json:"-"`
}

type codexReceiverLiveReceipt struct {
	Rules         []string `json:"rules"`
	InlineContext []string `json:"inline_context"`
	StageFile     string   `json:"stage_file"`
	Consumes      []string `json:"consumes"`
}

type codexReceiverRunStageWire struct {
	Kind               string   `json:"kind"`
	InlineContextPaths []string `json:"inline_context_paths"`
	StageFile          string   `json:"stage_file"`
	Consumes           []string `json:"consumes"`
	ContextWarnings    []string `json:"context_warnings"`
}

type codexReceiverJourneyFixture struct {
	Project       string
	Sentinels     map[string]string
	RuleDocuments []codexReceiverRule
	RuleSentinels []string
}

func newCodexReceiverJourneyProject(t *testing.T, moduleRoot string) codexReceiverJourneyFixture {
	t.Helper()
	project := newDeliveryJourneyProject(t)
	dataDir := filepath.Join(project, ".codex", "tools", "data")
	writeDeliveryJourneyFile(t, filepath.Join(dataDir, "stage-graph.json"), codexReceiverJourneyGraphJSON)

	skillSourcePath := filepath.Join(moduleRoot, "src", "harness", "codex", "skills", "aidlc", "SKILL.md")
	skill, err := os.ReadFile(skillSourcePath)
	if err != nil {
		t.Fatalf("ReadFile(source skill): %v", err)
	}
	skillTargetPath := filepath.Join(project, ".agents", "skills", "aidlc", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillTargetPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(skill target): %v", err)
	}
	if err := os.WriteFile(skillTargetPath, skill, 0o600); err != nil {
		t.Fatalf("WriteFile(skill target): %v", err)
	}
	placed, err := os.ReadFile(skillTargetPath)
	if err != nil {
		t.Fatalf("ReadFile(skill target): %v", err)
	}
	if !bytes.Equal(placed, skill) {
		t.Fatal("placed skill is not byte-identical to source skill")
	}

	productPersona := journeySentinel(t, "lead")
	architectPersona := journeySentinel(t, "support")
	stageContent := journeySentinel(t, "stage")
	consumeContent := journeySentinel(t, "consume")
	sentinels := map[string]string{
		".codex/agents/aidlc-product-agent.md":                                      productPersona,
		".codex/agents/aidlc-architect-agent.md":                                    architectPersona,
		".codex/aidlc-common/stages/ideation/intent-capture.md":                     stageContent,
		"aidlc/spaces/team/intents/build/ideation/intent-capture/receiver-input.md": consumeContent,
	}
	for relative, content := range map[string]string{
		filepath.Join(".codex", "agents", "aidlc-product-agent.md"):                                                     productPersona,
		filepath.Join(".codex", "agents", "aidlc-architect-agent.md"):                                                   architectPersona,
		filepath.Join(".codex", "aidlc-common", "stages", "ideation", "intent-capture.md"):                              stageContent,
		filepath.Join("aidlc", "spaces", "team", "intents", "build", "ideation", "intent-capture", "receiver-input.md"): consumeContent,
	} {
		absolute := filepath.Join(project, relative)
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(absolute), err)
		}
		writeDeliveryJourneyFile(t, absolute, content)
	}
	ruleAText, ruleASentinel := receiverRuleText(t, "rule-a")
	ruleBText, ruleBSentinel := receiverRuleText(t, "rule-b")
	ruleDocuments := []codexReceiverRule{
		{Path: "receiver-rule-a.md", Text: ruleAText},
		{Path: "receiver-rule-b.md", Text: ruleBText},
	}
	for _, rule := range ruleDocuments {
		writeDeliveryJourneyFile(t, filepath.Join(project, rule.Path), rule.Text)
	}
	return codexReceiverJourneyFixture{
		Project:       project,
		Sentinels:     sentinels,
		RuleDocuments: ruleDocuments,
		RuleSentinels: []string{ruleASentinel, ruleBSentinel},
	}
}

func receiverRuleText(t *testing.T, label string) (string, string) {
	t.Helper()
	sentinel := journeySentinel(t, label)
	const targetBytes = 11*1024 + 512
	const paddingLine = "receiver fixture padding.\n"
	var builder strings.Builder
	for builder.Len()+len(paddingLine)+len(sentinel)+1 <= targetBytes {
		builder.WriteString(paddingLine)
	}
	builder.WriteString(sentinel)
	builder.WriteByte('\n')
	return builder.String(), sentinel
}

func runCodexReceiverDirective(t *testing.T, binaryPath, project string, args ...string) codexReceiverDirective {
	t.Helper()
	commandArgs := append([]string{}, args...)
	commandArgs = append(commandArgs, "--project-dir", project)
	command := exec.Command(binaryPath, commandArgs...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run %q: %v\nstdout=%s\nstderr=%s", commandArgs, err, stdout.Bytes(), stderr.Bytes())
	}
	if stdout.Len() == 0 || bytes.Count(stdout.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("run %q stdout = %q, want one JSON line", commandArgs, stdout.String())
	}
	var directive codexReceiverDirective
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &directive); err != nil {
		t.Fatalf("run %q JSON: %v; stdout=%q", commandArgs, err, stdout.String())
	}
	directive.Raw = append([]byte(nil), bytes.TrimSpace(stdout.Bytes())...)
	return directive
}

func journeySentinel(t *testing.T, label string) string {
	t.Helper()
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("crypto/rand %s sentinel: %v", label, err)
	}
	return fmt.Sprintf("receiver-%s-%s", label, hex.EncodeToString(buffer))
}

func readJourneyFile(t *testing.T, project, slashPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(slashPath)))
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", slashPath, err)
	}
	return string(content)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
