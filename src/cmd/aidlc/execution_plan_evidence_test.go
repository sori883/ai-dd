package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

type executionApprovalEvidence struct {
	PendingStep, ApprovedStep, FinishedStep string
	PendingTarget, ApprovedTarget           string
	PendingTurn, AnswerTurn                 string
	Prompt, Quote                           string
	PlanRequest, ResultRequest              string
	CapturedRequests                        []string
	ApprovalExit, FinishExit                int
	HistoryHead                             string
}

func verifyExecutionApprovalEvidence(e executionApprovalEvidence) error {
	if e.PendingStep == "" || e.PendingStep != e.ApprovedStep || e.ApprovedStep != e.FinishedStep || e.PendingTarget == "" || e.PendingTarget != e.ApprovedTarget || e.AnswerTurn == "" || e.AnswerTurn == e.PendingTurn || e.Quote == "" || !strings.Contains(e.Prompt, e.Quote) || e.PlanRequest == "" || e.ResultRequest == "" || !slices.Contains(e.CapturedRequests, e.PlanRequest) || !slices.Contains(e.CapturedRequests, e.ResultRequest) || e.ApprovalExit != 0 || e.FinishExit != 0 || e.HistoryHead == "" {
		return fmt.Errorf("missing matching later-turn approval and finish evidence")
	}
	return nil
}
func TestExecutionPlanDistributionEvidence(t *testing.T) {
	good := executionApprovalEvidence{PendingStep: "s02", ApprovedStep: "s02", FinishedStep: "s02", PendingTarget: "hash", ApprovedTarget: "hash", PendingTurn: "A", AnswerTurn: "B", Prompt: "approve both", Quote: "approve", PlanRequest: "p", ResultRequest: "r", CapturedRequests: []string{"p", "r"}, HistoryHead: "head"}
	if err := verifyExecutionApprovalEvidence(good); err != nil {
		t.Fatalf("valid actual evidence rejected: %v", err)
	}
	for _, mode := range []string{"step", "target", "turn", "quote", "late request", "exit", "history"} {
		bad := good
		switch mode {
		case "step":
			bad.ApprovedStep = "s03"
		case "target":
			bad.ApprovedTarget = "other"
		case "turn":
			bad.AnswerTurn = "A"
		case "quote":
			bad.Quote = "fabricated"
		case "late request":
			bad.CapturedRequests = []string{"p"}
		case "exit":
			bad.FinishExit = 1
		case "history":
			bad.HistoryHead = ""
		}
		if verifyExecutionApprovalEvidence(bad) == nil {
			t.Errorf("invalid %s evidence accepted", mode)
		}
	}
}
