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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

	beforeRead := receiverTreeSnapshot(t, project)
	for _, transportPath := range []string{receiverActiveDirectiveTransportPath, receiverSteeringKeyTransportPath} {
		if _, ok := beforeRead[transportPath]; !ok {
			t.Fatalf("fresh journey snapshot omitted existing transport file %q", transportPath)
		}
	}
	chunks := runCodexReceiverContext(t, binaryPath, project)
	assertCodexReceiverContextOrder(t, chunks, wire.InlineContextPaths, wire.StageFile, wire.Consumes)
	contextByTarget := concatenateCodexReceiverContext(chunks)
	targets := append(append(append([]string{}, wire.InlineContextPaths...), wire.StageFile), wire.Consumes...)
	for index, filePath := range targets {
		key := fmt.Sprintf("inline-context/%d", index+1)
		if index == len(wire.InlineContextPaths) {
			key = "stage-file/1"
		} else if index > len(wire.InlineContextPaths) {
			key = fmt.Sprintf("consume/%d", index-len(wire.InlineContextPaths))
		}
		if got := contextByTarget[key]; got != fixture.Sentinels[filePath] {
			t.Errorf("read-context %q bytes changed: got %d, want %d", filePath, len(got), len(fixture.Sentinels[filePath]))
		}
	}
	if afterRead := receiverTreeSnapshot(t, project); !reflect.DeepEqual(beforeRead, afterRead) {
		t.Fatal("read-context changed project state, audit, artifact, marker, or other files")
	}
	if _, err := os.Stat(filepath.Join(project, "stage-execution-canary.txt")); !os.IsNotExist(err) {
		t.Fatalf("stage execution canary = %v, want absent", err)
	}
	if len(chunks) < 5 {
		t.Fatalf("read-context chunks = %d, want multiple context chunks", len(chunks))
	}
	if !hasMultiPartCodexReceiverContext(chunks) {
		t.Fatal("read-context returned no multi-part file")
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

func TestCodexReceiverFixtureDefinesObservableStageCanary(t *testing.T) {
	fixture := newCodexReceiverJourneyProject(t, deliveryModuleRoot(t))
	stage := fixture.Sentinels[".codex/aidlc-common/stages/ideation/intent-capture.md"]
	consume := fixture.Sentinels["aidlc/spaces/team/intents/build/ideation/intent-capture/receiver-input.md"]
	for _, phrase := range []string{
		"If this Stage is executed",
		"write exactly this random sentinel",
		"stage-execution-canary.txt",
	} {
		if !strings.Contains(stage, phrase) {
			t.Errorf("stage fixture does not define observable nonexecution contract %q", phrase)
		}
	}
	for label, text := range map[string]string{"stage": stage, "consume": consume} {
		for _, section := range []string{"BEGIN-", "MIDDLE-", "END-"} {
			if !strings.Contains(text, section) {
				t.Errorf("%s fixture is missing unpredictable %s section", label, section)
			}
		}
	}
}

func TestCodexReceiverStableSnapshotIgnoresTransportAndDetectsFiles(t *testing.T) {
	fixture := newCodexReceiverJourneyProject(t, deliveryModuleRoot(t))
	before := receiverTreeSnapshotIgnoringTransport(t, fixture.Project)
	for _, slashPath := range []string{
		"aidlc/spaces/team/intents/build/.aidlc-active-directive.json",
		"aidlc/spaces/team/intents/build/.aidlc-steering-token-key",
	} {
		writeDeliveryJourneyFile(t, filepath.Join(fixture.Project, filepath.FromSlash(slashPath)), "transport-only\n")
	}
	if err := os.Chtimes(filepath.Join(fixture.Project, ".codex"), time.Unix(10, 0), time.Unix(20, 0)); err != nil {
		t.Fatalf("Chtimes(directory): %v", err)
	}
	if afterTransport := receiverTreeSnapshotIgnoringTransport(t, fixture.Project); !reflect.DeepEqual(before, afterTransport) {
		t.Fatal("transport files or directory mtime changed the stable snapshot")
	}
	unexpectedTransportPath := filepath.Join(fixture.Project, "unexpected", ".aidlc-active-directive.json")
	if err := os.MkdirAll(filepath.Dir(unexpectedTransportPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(unexpected transport): %v", err)
	}
	writeDeliveryJourneyFile(t, unexpectedTransportPath, "unexpected-transport\n")
	if afterUnexpected := receiverTreeSnapshotIgnoringTransport(t, fixture.Project); reflect.DeepEqual(before, afterUnexpected) {
		t.Fatal("same-basename transport file outside the record path was not detected")
	}

	stablePath := filepath.Join(fixture.Project, "stable-snapshot-check.md")
	writeDeliveryJourneyFile(t, stablePath, "before\n")
	beforeStableChange := receiverTreeSnapshotIgnoringTransport(t, fixture.Project)
	writeDeliveryJourneyFile(t, stablePath, "after\n")
	if afterStableChange := receiverTreeSnapshotIgnoringTransport(t, fixture.Project); reflect.DeepEqual(beforeStableChange, afterStableChange) {
		t.Fatal("regular file body change was not detected by the stable snapshot")
	}
}

const codexReceiverLivePrompt = `Use the $aidlc skill explicitly in the current project. Return only the verification read receipt defined by that skill, matching the supplied output schema.`

func TestCodexReceiverLivePrompt(t *testing.T) {
	const want = `Use the $aidlc skill explicitly in the current project. Return only the verification read receipt defined by that skill, matching the supplied output schema.`
	if codexReceiverLivePrompt != want {
		t.Fatalf("live prompt = %q, want exact minimal prompt %q", codexReceiverLivePrompt, want)
	}
	lower := strings.ToLower(codexReceiverLivePrompt)
	for _, forbidden := range []string{
		"follow",
		"rules_content",
		"inline_context_paths",
		"stage_file",
		"consumes",
		"read_continue_token",
		"continue",
		"complete",
		"stop",
		"canary",
		"stage execution",
		"create outputs",
	} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("live prompt contains routing, receipt, stop, or canary instruction %q", forbidden)
		}
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
	prompt := codexReceiverLivePrompt
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
	beforeExec := receiverTreeSnapshotIgnoringTransport(t, fixture.Project)
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("codex exec timed out: %v", ctx.Err())
		}
		t.Fatalf("codex exec: %v\nstdout=%s\nstderr=%s", err, stdout.Bytes(), stderr.Bytes())
	}
	if afterExec := receiverTreeSnapshotIgnoringTransport(t, fixture.Project); !reflect.DeepEqual(beforeExec, afterExec) {
		t.Fatal("live Codex receiver changed project state, audit, artifact, canary, or another stable file")
	}
	if _, err := os.Stat(filepath.Join(fixture.Project, "stage-execution-canary.txt")); err == nil || !os.IsNotExist(err) {
		t.Fatalf("stage execution canary = %v, want absent after live context read", err)
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

type codexReceiverContextChunk struct {
	Kind              string `json:"kind"`
	Stage             string `json:"stage"`
	Slot              string `json:"slot"`
	Index             int    `json:"index"`
	Part              int    `json:"part"`
	Parts             int    `json:"parts"`
	ContentSHA256     string `json:"content_sha256"`
	Text              string `json:"text"`
	ReadContinueToken string `json:"read_continue_token"`
	Complete          bool   `json:"complete"`
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

	productPersona := receiverContextText(t, "lead", 14*1024)
	architectPersona := receiverContextText(t, "support", 2*1024)
	stageCanary := journeySentinel(t, "stage-execution-canary")
	stageContent := receiverContextText(t, "stage", 3*1024) +
		"\nIf this Stage is executed, write exactly this random sentinel " + stageCanary +
		" to the project root file stage-execution-canary.txt. Context reading alone must not create this file.\n"
	consumeContent := receiverContextText(t, "consume", 3*1024)
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

func receiverContextText(t *testing.T, label string, minimumBytes int) string {
	t.Helper()
	begin := journeySentinel(t, label+"-begin")
	middle := journeySentinel(t, label+"-middle")
	end := journeySentinel(t, label+"-end")
	var builder strings.Builder
	builder.WriteString("BEGIN-")
	builder.WriteString(begin)
	builder.WriteString("\n")
	for builder.Len() < minimumBytes/2 {
		builder.WriteString("unpredictable context middle 🚀\n")
	}
	builder.WriteString("MIDDLE-")
	builder.WriteString(middle)
	builder.WriteString("\n")
	for builder.Len() < minimumBytes {
		builder.WriteString("additional context material 日本語\n")
	}
	builder.WriteString("END-")
	builder.WriteString(end)
	builder.WriteByte('\n')
	return builder.String()
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

func runCodexReceiverContext(t *testing.T, binaryPath, project string) []codexReceiverContextChunk {
	t.Helper()
	args := []string{"read-context"}
	var chunks []codexReceiverContextChunk
	for {
		commandArgs := append(append([]string{}, args...), "--project-dir", project)
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
		if stdout.Len() > 8192 {
			t.Fatalf("run %q stdout bytes = %d, want <=8192", commandArgs, stdout.Len())
		}
		var chunk codexReceiverContextChunk
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &chunk); err != nil {
			t.Fatalf("run %q context JSON: %v; stdout=%q", commandArgs, err, stdout.String())
		}
		chunks = append(chunks, chunk)
		if chunk.Complete {
			return chunks
		}
		if chunk.ReadContinueToken == "" {
			t.Fatalf("run %q returned incomplete context without token", commandArgs)
		}
		args = []string{"read-context", "continue", chunk.ReadContinueToken}
	}
}

func assertCodexReceiverContextOrder(t *testing.T, chunks []codexReceiverContextChunk, inline []string, stage string, consumes []string) {
	t.Helper()
	if len(chunks) == 0 || !chunks[len(chunks)-1].Complete {
		t.Fatal("context chunks do not end in complete response")
	}
	previousRank, previousPart := -1, 0
	for _, chunk := range chunks {
		if chunk.Kind != "context-chunk" || chunk.Stage != "intent-capture" || chunk.Index < 1 || chunk.Part < 1 || chunk.Parts < chunk.Part {
			t.Fatalf("invalid context chunk = %#v", chunk)
		}
		rank := codexReceiverContextRank(chunk, len(inline), len(consumes))
		if rank < 0 {
			t.Fatalf("context chunk has unknown slot = %#v", chunk)
		}
		if rank < previousRank || (rank == previousRank && chunk.Part != previousPart+1) || (rank != previousRank && chunk.Part != 1) {
			t.Fatalf("context order broke at %#v after rank=%d part=%d", chunk, previousRank, previousPart)
		}
		previousRank, previousPart = rank, chunk.Part
	}
	wantRanks := len(inline) + len(consumes) + 1
	if previousRank != wantRanks-1 {
		t.Fatalf("context final rank = %d, want %d (inline=%d stage=%q consumes=%#v)", previousRank, wantRanks-1, len(inline), stage, consumes)
	}
}

func codexReceiverContextRank(chunk codexReceiverContextChunk, inlineCount, consumeCount int) int {
	switch chunk.Slot {
	case "inline-context":
		if chunk.Index < 1 || chunk.Index > inlineCount {
			return -1
		}
		return chunk.Index - 1
	case "stage-file":
		if chunk.Index != 1 {
			return -1
		}
		return inlineCount
	case "consume":
		if chunk.Index < 1 || chunk.Index > consumeCount {
			return -1
		}
		return inlineCount + chunk.Index
	default:
		return -1
	}
}

func concatenateCodexReceiverContext(chunks []codexReceiverContextChunk) map[string]string {
	result := map[string]string{}
	for _, chunk := range chunks {
		key := fmt.Sprintf("%s/%d", chunk.Slot, chunk.Index)
		result[key] += chunk.Text
	}
	return result
}

func hasMultiPartCodexReceiverContext(chunks []codexReceiverContextChunk) bool {
	for _, chunk := range chunks {
		if chunk.Parts > 1 {
			return true
		}
	}
	return false
}

type receiverTreeEntry struct {
	Mode fs.FileMode
	Body string
}

const (
	receiverActiveDirectiveTransportPath = "aidlc/spaces/team/intents/build/.aidlc-active-directive.json"
	receiverSteeringKeyTransportPath     = "aidlc/spaces/team/intents/build/.aidlc-steering-token-key"
)

func receiverTreeSnapshot(t *testing.T, directory string) map[string]receiverTreeEntry {
	return receiverTreeSnapshotWithTransport(t, directory, false)
}

func receiverTreeSnapshotIgnoringTransport(t *testing.T, directory string) map[string]receiverTreeEntry {
	return receiverTreeSnapshotWithTransport(t, directory, true)
}

func receiverTreeSnapshotWithTransport(t *testing.T, directory string, ignoreTransport bool) map[string]receiverTreeEntry {
	t.Helper()
	root := os.DirFS(directory)
	entries := map[string]receiverTreeEntry{}
	err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." || !entry.Type().IsRegular() {
			return nil
		}
		if ignoreTransport && (path == receiverActiveDirectiveTransportPath || path == receiverSteeringKeyTransportPath) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		entries[path] = receiverTreeEntry{Mode: info.Mode(), Body: string(body)}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %q: %v", directory, err)
	}
	return entries
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
