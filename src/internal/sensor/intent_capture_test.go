package sensor

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClaimSources(t *testing.T) {
	input := Input{
		Stage:              "intent-capture",
		ArtifactPath:       "intent-statement.md",
		Content:            []byte("# Intent\n## Initial Scope Signal\n- [scope] Workflow-selected scope: `classic`.\n## Sources\n- [desc] Initial description: \"A project\"\n## Assumptions & Open Questions\n- None.\n"),
		ProjectDescription: "A project",
		Scope:              "classic",
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	assertSensorInvocation(t, invocation, "claim-sources")
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("claim-sources terminal status = %q, want %q", invocation.TerminalResult().Status, StatusCompleted)
	}
	if len(invocation.TerminalResult().Findings) != 0 {
		t.Fatalf("claim-sources valid fixture produced findings: %#v", invocation.TerminalResult().Findings)
	}
}

func TestClaimSourcesRequiresRegisteredSourcesAndFilledQuestions(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [unregistered] This claim has no source.\n## Assumptions & Open Questions\n- None.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n## Q1\n[Answer]: TBD\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
	if len(invocation.TerminalResult().Findings) == 0 {
		t.Fatal("claim-sources missing source registration finding")
	}
}

func TestClaimSourcesRequiresAssumptionConfirmation(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [desc] A claim.\n## Assumptions & Open Questions\n- [assumption] Users accept local storage.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
	if len(invocation.TerminalResult().Findings) == 0 {
		t.Fatal("claim-sources missing assumption confirmation finding")
	}
}

func TestClaimSourcesRequiresAssumptionTag(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [desc] A claim.\n## Assumptions & Open Questions\n- [desc] Users accept local storage.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n## Assumption Confirmation\n[Answer]: A. Accept assumptions\n- [assumption] Users accept local storage.\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
}

func TestClaimSourcesRequiresFilledQuestionAnswer(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [Q1] A question-grounded claim.\n## Assumptions & Open Questions\n- None.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n## Q1\n[Answer]: Unanswered\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
}

func TestClaimSourcesValidatesMemorySourceAgainstAuthority(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [memory:rule-1] Use the established source links.\n## Assumptions & Open Questions\n- None.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n- [memory:rule-1] `aidlc/spaces/team/memory/team.md#Rules`: \"Use the established source links.\"\n"),
		ActiveSpace:  "team",
		MemorySources: map[string][]string{
			"aidlc/spaces/team/memory/team.md#Rules": {"Use the established source links."},
		},
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("claim-sources status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
	}
}

func TestClaimSourcesRejectsPastedDescriptionGrounding(t *testing.T) {
	input := Input{
		Stage:                 "intent-capture",
		ArtifactPath:          "intent-statement.md",
		PastedDocumentPresent: true,
		Content:               []byte("## Problem\n- [desc] A claim.\n## Assumptions & Open Questions\n- None.\n"),
		Questions:             []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
}

func TestClaimSourcesExcludesReviewAppendix(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Problem\n- [desc] A claim.\n## Assumptions & Open Questions\n- None.\n## Review\n- [not-a-source] reviewer prose may be untagged.\n"),
		Questions:    []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("claim-sources status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
	}
}

func TestClaimSourcesRejectsReferenceLinkTag(t *testing.T) {
	input := Input{
		Stage:              "intent-capture",
		ArtifactPath:       "intent-statement.md",
		ProjectDescription: "A project",
		Scope:              "classic",
		Content:            []byte("## Problem\n- [Q1][Q1] A linked reference, not a source tag.\n[Q1]: https://example.test/question\n## Assumptions & Open Questions\n- None.\n"),
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n## Q1\n[Answer]: A. Yes\n"),
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("reference-link source status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusFailed)
	}
}

func TestClaimSourcesExcludesQuestionsAndHandlesTablesAndTildeFences(t *testing.T) {
	main := []byte("## Problem\n\n| Claim | Source |\n| --- | --- |\n| [desc] visible claim | source |\n\n~~~markdown\n- [unknown] hidden example\n~~~\n\n## Assumptions & Open Questions\n- None.\n")
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "ideation/intent-capture/intent-statement.md",
		Content:      main,
		Deliverables: [][]byte{main, []byte("## Problem\n- [unknown] scaffold must be ignored\n")},
		OutputFiles: []Deliverable{
			{Path: "ideation/intent-capture/intent-statement.md", Content: main},
			{Path: "ideation/intent-capture/intent-capture-questions.md", Content: []byte("## Questions\n- [unknown] scaffold must be ignored\n")},
			{Path: "ideation/intent-capture/intent-capture-timestamp.md", Content: []byte("[unknown] scaffold must be ignored")},
		},
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n"),
		ProjectDescription: "A project",
		Scope:              "classic",
	}
	invocation := RunAll(context.Background(), input, nil)[0]
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("visible claim status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
	}
}

func TestClaimSourcesRejectsRulesOnlyInCodeOrComment(t *testing.T) {
	memory := []byte("## Rules\n- visible rule\n````markdown\n- hidden code rule\n```\n- still hidden code rule\n````\n- <!-- hidden inline rule -->\n<!--\n- hidden multiline rule\n-->\n")
	sources := parseMemorySources("aidlc/spaces/team/memory/team.md", memory)
	if got := sources["aidlc/spaces/team/memory/team.md#Rules"]; len(got) != 1 || got[0] != "visible rule" {
		t.Fatalf("parseMemorySources() = %#v, want only visible rule", sources)
	}
	input := Input{
		Stage:              "intent-capture",
		ArtifactPath:       "intent-statement.md",
		Content:            []byte("## Problem\n- [memory:rule-1] hidden code rule\n## Assumptions & Open Questions\n- None.\n"),
		ProjectDescription: "A project",
		Scope:              "classic",
		ActiveSpace:        "team",
		MemorySources:      sources,
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n- [memory:rule-1] `aidlc/spaces/team/memory/team.md#Rules`: \"hidden code rule\"\n"),
	}
	if got := RunAll(context.Background(), input, nil)[0].TerminalResult().Status; got != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q for hidden memory rule", got, StatusFailed)
	}
}

func TestVisibleMarkdownLinesKeepsLongerFenceClosedOnlyByMatchingLength(t *testing.T) {
	lines := visibleMarkdownLines("````markdown\n- hidden [Q1]\n```\n- still hidden [Q1]\n````\n- visible [Q1]\n")
	if strings.TrimSpace(lines[1]) != "" || strings.TrimSpace(lines[3]) != "" {
		t.Fatalf("visibleMarkdownLines() exposed fenced lines: %#v", lines)
	}
	if strings.TrimSpace(lines[5]) != "- visible [Q1]" {
		t.Fatalf("visibleMarkdownLines() after matching close = %#v", lines)
	}
}

func TestClaimSourcesRejectsNestedReferenceLinkLabelsAndHiddenTags(t *testing.T) {
	input := Input{
		Stage:              "intent-capture",
		ArtifactPath:       "intent-statement.md",
		ProjectDescription: "A project",
		Scope:              "classic",
		Content:            []byte("## Problem\n> [Q1] linked claim\n> [Q1]: https://example.test/question\n- <!-- [desc] --> comment-only source\n````markdown\n- [Q1] hidden source\n```\n- [Q1] still hidden source\n````\n## Assumptions & Open Questions\n- None.\n"),
		Questions:          []byte("## Sources\n- [desc] Initial description: \"A project\"\n- [scope] Workflow-selected scope: `classic`.\n## Q1\n[Answer]: A. Yes\n"),
	}
	claims, _ := visibleClaimBlocks(string(input.Content))
	for _, claim := range claims {
		if strings.Contains(claim.Text, "hidden source") {
			t.Fatalf("visibleClaimBlocks() exposed hidden claim: %#v", claims)
		}
		if strings.Contains(claim.Text, "comment-only") && len(visibleSourceTags(claim.Text, claim.ReferenceLabels)) != 0 {
			t.Fatalf("visibleClaimBlocks() retained a comment-only source tag: %#v", claim)
		}
	}
	if got := RunAll(context.Background(), input, nil)[0].TerminalResult().Status; got != StatusFailed {
		t.Fatalf("claim-sources status = %q, want %q for nested reference/comment-only claims", got, StatusFailed)
	}
}

func TestRequiredSections(t *testing.T) {
	input := Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("# Intent\n## Outcome\n## Assumptions & Open Questions\n")}
	invocation := RunAll(context.Background(), input, nil)[1]
	assertSensorInvocation(t, invocation, "required-sections")
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("required-sections terminal status = %q, want %q", invocation.TerminalResult().Status, StatusCompleted)
	}
}

func TestRequiredSectionsRejectsHeadingsOnlyInHiddenMarkdown(t *testing.T) {
	cases := map[string]string{
		"backtick fence": "```markdown\n## Hidden one\n## Hidden two\n```\n",
		"tilde fence":    "~~~markdown\n## Hidden one\n## Hidden two\n~~~\n",
		"longer fence":   "````markdown\n## Hidden one\n```\n## Hidden two\n````\n",
		"html comment":   "<!--\n## Hidden one\n## Hidden two\n-->\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			input := Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte(content)}
			invocation := RunAll(context.Background(), input, nil)[1]
			if invocation.TerminalResult().Status != StatusFailed {
				t.Fatalf("required-sections status = %q, findings = %#v, want hidden headings rejected", invocation.TerminalResult().Status, invocation.TerminalResult().Findings)
			}
		})
	}
}

func TestRequiredSectionsRejectsInvalidUTF8(t *testing.T) {
	input := Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("## Outcome\n\xff\n## Assumptions & Open Questions\n")}
	invocation := RunAll(context.Background(), input, nil)[1]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("required-sections status = %q, want %q", invocation.TerminalResult().Status, StatusFailed)
	}
}

func TestRequiredSectionsAppliesTeamThenFrameworkTemplate(t *testing.T) {
	t.Run("team template wins", func(t *testing.T) {
		input := Input{
			Stage:        "intent-capture",
			ArtifactPath: "ideation/intent-capture/intent-statement.md",
			OutputFiles:  []Deliverable{{Path: "ideation/intent-capture/intent-statement.md", Content: []byte("## Overview\n## Team Notes\n")}},
			TeamTemplates: map[string][]byte{
				"intent-statement": []byte("## Overview\n## Team Notes\n"),
			},
			FrameworkTemplates: map[string][]byte{
				"intent-statement": []byte("## Overview\n## Framework Notes\n"),
			},
		}
		invocation := RunAll(context.Background(), input, nil)[1]
		if invocation.TerminalResult().Status != StatusCompleted {
			t.Fatalf("team-template status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
		}
	})
	t.Run("template mismatch fails closed", func(t *testing.T) {
		input := Input{
			Stage:        "intent-capture",
			ArtifactPath: "ideation/intent-capture/intent-statement.md",
			Content:      []byte("## Overview\n## Wrong\n"),
			TeamTemplates: map[string][]byte{
				"intent-statement": []byte("## Overview\n## Team Notes\n"),
			},
		}
		invocation := RunAll(context.Background(), input, nil)[1]
		if invocation.TerminalResult().Status != StatusFailed {
			t.Fatalf("template-mismatch status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusFailed)
		}
	})
}

func TestUpstreamCoverage(t *testing.T) {
	invocation := RunAll(context.Background(), Input{Stage: "intent-capture"}, nil)[2]
	assertSensorInvocation(t, invocation, "upstream-coverage")
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("upstream-coverage terminal status = %q, want %q", invocation.TerminalResult().Status, StatusCompleted)
	}
	if invocation.TerminalResult().Detail == "" {
		t.Fatal("upstream-coverage terminal detail is empty")
	}
}

func TestUpstreamCoverageRecognizesProducerDirectory(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "intent-statement.md",
		Content:      []byte("## Provenance\n- [scope] construction/nfr-requirements/ supplied the upstream contract.\n"),
		Consumes:     []string{"requirements:nfr-requirements"},
	}
	invocation := RunAll(context.Background(), input, nil)[2]
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("upstream-coverage status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
	}
}

func TestUpstreamCoverageExcludesScaffoldingAndIgnoresCase(t *testing.T) {
	input := Input{
		Stage:        "intent-capture",
		ArtifactPath: "ideation/intent-capture/intent-statement.md",
		Content:      nil,
		OutputFiles: []Deliverable{
			{Path: "ideation/intent-capture/intent-capture-questions.md", Content: []byte("nfr-requirements")},
			{Path: "ideation/intent-capture/intent-statement.md", Content: []byte("The NFR-REQUIREMENTS contract is covered here.\n")},
		},
		Questions: []byte("nfr-requirements"),
		Consumes:  []string{"nfr-requirements:reverse-engineering"},
	}
	invocation := RunAll(context.Background(), input, nil)[2]
	if invocation.TerminalResult().Status != StatusCompleted {
		t.Fatalf("case-insensitive coverage status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusCompleted)
	}

	input.OutputFiles = []Deliverable{{Path: "ideation/intent-capture/intent-capture-questions.md", Content: []byte("NFR-REQUIREMENTS")}}
	invocation = RunAll(context.Background(), input, nil)[2]
	if invocation.TerminalResult().Status != StatusFailed {
		t.Fatalf("scaffolding-only coverage status = %q, findings = %#v, want %q", invocation.TerminalResult().Status, invocation.TerminalResult().Findings, StatusFailed)
	}
}

func TestSensorReceiptRecording(t *testing.T) {
	input := Input{Stage: "intent-capture", ArtifactPath: "intent-statement.md", Content: []byte("# Intent\n")}
	checks := map[string]CheckFunc{
		"claim-sources": func(Input) (CheckResult, error) {
			return CheckResult{}, errors.New("sensor unavailable")
		},
	}
	invocations := RunAll(context.Background(), input, checks)
	if len(invocations) != 3 {
		t.Fatalf("RunAll() returned %d invocations, want all three", len(invocations))
	}
	if invocations[0].TerminalResult().Status != StatusCompleted || invocations[0].Error() == nil || invocations[0].TerminalResult().Note == "" {
		t.Fatalf("failed sensor outcome = %#v, want passed terminal with script-error note and local error", invocations[0])
	}
	if got := invocations[0].FireResult().FireID(); len(got) != 8 || strings.Trim(got, "0123456789abcdef") != "" {
		t.Fatalf("fire id = %q, want eight lowercase hex characters", got)
	}
}

func TestIntentCaptureAdvisorySensorOutcomesDoNotAuthorizeOrBlockGate(t *testing.T) {
	checks := map[string]CheckFunc{
		"claim-sources":     func(Input) (CheckResult, error) { return CheckResult{}, errors.New("failed") },
		"required-sections": func(Input) (CheckResult, error) { return CheckResult{}, nil },
		"upstream-coverage": func(Input) (CheckResult, error) { return CheckResult{}, nil },
	}
	invocations := RunAll(context.Background(), Input{Stage: "intent-capture"}, checks)
	if AdvisoryOutcomesAuthorizeGate(invocations) {
		t.Fatal("advisory outcomes authorized gate")
	}
	if AdvisoryOutcomesBlockGate(invocations) {
		t.Fatal("advisory outcomes blocked gate")
	}
	if invocations[1].TerminalResult().Status == StatusCompleted {
		t.Fatal("missing terminal result was displayed as pass")
	}
}

func assertSensorInvocation(t *testing.T, invocation Invocation, name string) {
	t.Helper()
	if invocation.FireResult().Sensor != name || invocation.FireResult().Status != StatusFired {
		t.Fatalf("fire result = %#v, want %s/%s", invocation.FireResult(), name, StatusFired)
	}
	if invocation.TerminalResult().Sensor != name || !invocation.TerminalResult().Terminal {
		t.Fatalf("terminal result = %#v, want terminal %s", invocation.TerminalResult(), name)
	}
	if invocation.FireResult().FireID() == "" || invocation.TerminalResult().FireID() != invocation.FireResult().FireID() {
		t.Fatal("sensor fire identity was not preserved across terminal result")
	}
}
