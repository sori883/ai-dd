package aidlc

import (
	"os"
	"strings"
	"testing"
)

func TestIntentCaptureReceiverOrder(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := string(data)
	start := strings.Index(body, "## Intent-capture execution contract")
	if start < 0 {
		t.Fatal("SKILL.md has no intent-capture execution contract")
	}
	body = body[start:]
	phrases := []string{
		"inline persona/knowledge",
		"base stage protocol",
		"reviewer/ensemble protocol",
		"question-rendering annex",
		"project-description.json",
		"stage file",
		"consumes",
		"DECISION_RECORDED",
		"fresh HUMAN_TURN",
		"QUESTION_ANSWERED",
		"SUMMARY_CONFIRMATION_RECORDED",
		"REVIEW_REQUESTED",
		"REVIEW_COMPLETED",
		"SENSOR_FIRED",
		"RULE_LEARNED",
		"Approve",
	}
	last := -1
	for _, phrase := range phrases {
		position := strings.Index(body, phrase)
		if position < 0 {
			t.Fatalf("intent-capture contract lacks %q", phrase)
		}
		if position <= last {
			t.Fatalf("intent-capture contract puts %q out of order", phrase)
		}
		last = position
	}
}

func TestIntentCaptureInlineArchitect(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.ToLower(string(data))
	for _, phrase := range []string{"architect: inline", "no architect subagent", "no contribution file", "product-lead"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("SKILL.md does not define %q", phrase)
		}
	}
	if strings.Contains(body, "architect subagent dispatch") || strings.Contains(body, "architect contribution file") {
		t.Fatal("SKILL.md authorizes an architect subagent or contribution file")
	}
}

func TestIntentCaptureHiddenCommandSequence(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := string(data)
	sequence := []string{
		"printf '%s' '<JSON decision data>' | aidlc __codex-stage decision --project-dir .",
		"UserPromptSubmit -> aidlc __codex-user-prompt-submit",
		"printf '%s' '<JSON answer data>' | aidlc __codex-stage answer --project-dir .",
		"aidlc __codex-stage summary --project-dir .",
		"UserPromptSubmit -> aidlc __codex-user-prompt-submit",
		"printf '%s' '<JSON summary answer data>' | aidlc __codex-stage answer --project-dir .",
		"write the three intent-capture artifacts",
		"aidlc __codex-stage review-request --project-dir .",
		"dispatch configured `aidlc-product-lead-agent`",
		"configured reviewer appends canonical `## Review` appendix",
		"printf '%s' '<JSON verdict data>' | aidlc __codex-stage review-complete --project-dir .",
		"aidlc __codex-stage run-sensors --project-dir .",
		"aidlc __codex-stage learnings-surface --project-dir .",
		"UserPromptSubmit -> aidlc __codex-user-prompt-submit",
		"printf '%s' '{\"selections\":[]}' | aidlc __codex-stage learnings-persist --project-dir .",
		"aidlc report --stage intent-capture --result awaiting-approval",
		"UserPromptSubmit -> aidlc __codex-user-prompt-submit",
		"aidlc report --stage intent-capture --result approved --user-input Approve",
		"aidlc next",
	}
	last := -1
	for _, phrase := range sequence {
		position := strings.Index(body[last+1:], phrase)
		if position >= 0 {
			position += last + 1
		}
		if position < 0 {
			t.Fatalf("SKILL.md hidden sequence lacks %q", phrase)
		}
		if position <= last {
			t.Fatalf("SKILL.md hidden sequence puts %q out of order", phrase)
		}
		last = position
	}
}

func TestIntentCaptureBridgeUsesStdinAndReviewerDispatch(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := string(data)
	normalized := strings.Join(strings.Fields(body), " ")
	for _, phrase := range []string{
		"JSON payload is supplied on stdin",
		"dispatch configured `aidlc-product-lead-agent`",
		"conductor does not append or mint the review",
		"fresh review, run the advisory sensors",
	} {
		if !strings.Contains(normalized, phrase) {
			t.Errorf("SKILL.md bridge contract lacks %q", phrase)
		}
	}
	if strings.Contains(body, "aidlc __codex-stage decision <JSON decision data>") || strings.Contains(body, "aidlc __codex-stage answer <JSON answer data>") {
		t.Fatal("SKILL.md documents JSON as a positional hidden-command argument")
	}
}

func TestIntentCaptureRevisionReviewDispatchBrief(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.ToLower(string(data))
	for _, phrase := range []string{
		"backend-derived iteration",
		"all three declared artifacts",
		"prior findings",
		"exactly one `**request challenge:**",
		"do not surface or persist learnings again",
		"anything to add for next time?",
		"nothing to add",
		"add a note",
	} {
		if !strings.Contains(body, phrase) {
			t.Errorf("SKILL.md revision contract lacks %q", phrase)
		}
	}

	data, err = os.ReadFile("../../agents/aidlc-product-lead-agent.toml")
	if err != nil {
		t.Fatalf("ReadFile(product-lead TOML): %v", err)
	}
	config := strings.ToLower(string(data))
	for _, phrase := range []string{"backend-derived iteration", "prior findings", "request challenge", "iteration 2"} {
		if !strings.Contains(config, phrase) {
			t.Errorf("product-lead TOML revision brief lacks %q", phrase)
		}
	}
}

func TestIntentCaptureReadOnlyBoundaryIsScoped(t *testing.T) {
	data, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md): %v", err)
	}
	body := strings.ToLower(string(data))
	if strings.Contains(body, "never performs stage execution") || strings.Contains(body, "never performs stage execution, creates outputs") {
		t.Fatal("SKILL.md describes the intent-capture receiver as universally read-only")
	}
	if !strings.Contains(body, "ordinary run-stage directives remain read-only") {
		t.Fatal("SKILL.md does not scope read-only behavior to ordinary run-stage directives")
	}
}

func TestQuestionRenderingAnnex(t *testing.T) {
	data, err := os.ReadFile("question-rendering.md")
	if err != nil {
		t.Fatalf("ReadFile(question-rendering.md): %v", err)
	}
	body := strings.ToLower(string(data))
	for _, phrase := range []string{"one question per turn", "exact answer", "looks correct", "fresh human_turn", "no path exploration", "project-description.json"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("question-rendering annex lacks %q", phrase)
		}
	}
}
