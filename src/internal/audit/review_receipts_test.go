package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/review"
)

func TestReviewReceiptsUseCanonicalAuthorityFields(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "intent-statement.md")
	before := []byte("# Intent\n\n")
	if err := os.WriteFile(artifactPath, before, 0o600); err != nil {
		t.Fatalf("WriteFile(artifact): %v", err)
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "ignored-source",
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request); err != nil {
		t.Fatalf("RecordReviewRequested(): %v", err)
	}
	after := append(append([]byte(nil), before...), []byte(canonicalAuditReviewAppendix("READY"))...)
	completion, err := review.NewCompletion(review.CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: after})
	if err != nil {
		t.Fatalf("review.NewCompletion(): %v", err)
	}
	if err := os.WriteFile(artifactPath, after, 0o600); err != nil {
		t.Fatalf("WriteFile(reviewed artifact): %v", err)
	}
	if err := RecordReviewCompleted(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request, completion); err != nil {
		t.Fatalf("RecordReviewCompleted(): %v", err)
	}
	records := mustReadAuditRecords(t, fixture)
	if len(records) != 2 {
		t.Fatalf("records = %#v, want request and completion", records)
	}
	wantRequest := map[string]string{
		"Stage":                        "intent-capture",
		"Reviewer":                     "aidlc-product-lead-agent",
		"Iteration":                    "1",
		"Artifact Fingerprint":         request.ArtifactFingerprint,
		"Review Appendix Artifact":     request.ArtifactPath,
		"Review Appendix Offset":       fmt.Sprint(request.AppendixOffset),
		"Review Appendix Prior Digest": "none",
		"Review Appendix Prior Length": "0",
	}
	assertReviewFields(t, records[0].Fields, wantRequest)
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(records[0].Fields["Artifact Fingerprint"]) {
		t.Fatalf("request artifact fingerprint = %q, want sha256 manifest", records[0].Fields["Artifact Fingerprint"])
	}
	wantCompletion := map[string]string{
		"Stage":                        "intent-capture",
		"Reviewer":                     "aidlc-product-lead-agent",
		"Iteration":                    "1",
		"Verdict":                      "READY",
		"Request Fingerprint":          request.ArtifactFingerprint,
		"Artifact Fingerprint":         completion.PostFingerprint,
		"Review Appendix Artifact":     request.ArtifactPath,
		"Review Appendix Offset":       fmt.Sprint(completion.AppendixOffset),
		"Review Appendix Prior Digest": "none",
		"Review Appendix Prior Length": "0",
	}
	assertReviewFields(t, records[1].Fields, wantCompletion)
}

func TestReviewReceiptRejectsUnknownAuthorityFields(t *testing.T) {
	fields := map[string]string{
		"Stage":                        "intent-capture",
		"Reviewer":                     "aidlc-product-lead-agent",
		"Iteration":                    "1",
		"Artifact Fingerprint":         "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		"Review Appendix Artifact":     "intent-statement.md",
		"Review Appendix Offset":       "0",
		"Review Appendix Prior Digest": "none",
		"Review Appendix Prior Length": "0",
	}
	if !validCanonicalReviewReceiptFields(fields, false) {
		t.Fatal("validCanonicalReviewReceiptFields(request) = false for canonical fields")
	}
	fields["Artifact Snapshot"] = "sha256:forged"
	if validCanonicalReviewReceiptFields(fields, false) {
		t.Fatalf("validCanonicalReviewReceiptFields(request) = true for unknown authority field: %#v", fields)
	}
	delete(fields, "Artifact Snapshot")
	fields["Review Challenge"] = ""
	if validCanonicalReviewReceiptFields(fields, false) {
		t.Fatalf("validCanonicalReviewReceiptFields(request) = true for empty conditional challenge: %#v", fields)
	}
}

func TestReviewReceiptPriorBindingRequiresDigestLengthAndChallengeAgreement(t *testing.T) {
	base := map[string]string{
		"Stage":                        "intent-capture",
		"Reviewer":                     "aidlc-product-lead-agent",
		"Iteration":                    "1",
		"Artifact Fingerprint":         "sha256:" + strings.Repeat("0", 64),
		"Review Appendix Artifact":     "intent-statement.md",
		"Review Appendix Offset":       "0",
		"Review Appendix Prior Digest": "none",
		"Review Appendix Prior Length": "0",
	}
	if !validCanonicalReviewReceiptFields(base, false) {
		t.Fatal("zero-length initial binding is invalid")
	}
	for name, mutate := range map[string]func(map[string]string){
		"initial digest": func(fields map[string]string) {
			fields["Review Appendix Prior Digest"] = "sha256:" + strings.Repeat("1", 64)
		},
		"initial challenge": func(fields map[string]string) { fields["Review Challenge"] = "review:" + strings.Repeat("a", 32) },
		"revision none digest": func(fields map[string]string) {
			fields["Review Appendix Prior Length"] = "12"
		},
		"revision missing challenge": func(fields map[string]string) {
			fields["Review Appendix Prior Length"] = "12"
			fields["Review Appendix Prior Digest"] = "sha256:" + strings.Repeat("1", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			fields := make(map[string]string, len(base))
			for key, value := range base {
				fields[key] = value
			}
			mutate(fields)
			if validCanonicalReviewReceiptFields(fields, false) {
				t.Fatalf("validCanonicalReviewReceiptFields(%s) = true, want false: %#v", name, fields)
			}
		})
	}
	validRevision := make(map[string]string, len(base))
	for key, value := range base {
		validRevision[key] = value
	}
	validRevision["Review Appendix Prior Length"] = "12"
	validRevision["Review Appendix Prior Digest"] = "sha256:" + strings.Repeat("1", 64)
	validRevision["Review Challenge"] = "review:" + strings.Repeat("a", 32)
	if !validCanonicalReviewReceiptFields(validRevision, false) {
		t.Fatalf("valid revision binding rejected: %#v", validRevision)
	}
}

func assertReviewFields(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("review fields = %#v, want exactly %#v", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("review field %q = %q, want %q", key, got[key], value)
		}
	}
	for _, key := range []string{"Artifact Path", "Request Source", "Prior Digest", "Prior Length", "Post Fingerprint", "Artifact Set"} {
		if _, ok := got[key]; ok {
			t.Errorf("legacy review field %q is present: %#v", key, got)
		}
	}
}

func TestIntentCaptureReviewRequest(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "intent-statement.md")
	if err := os.WriteFile(artifactPath, []byte("# Intent\n\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifact): %v", err)
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: []byte("# Intent\n\n"), RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request); err != nil {
		t.Fatalf("RecordReviewRequested(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 1 || records[0].Event != "REVIEW_REQUESTED" {
		t.Fatalf("records = %#v, want one REVIEW_REQUESTED", records)
	}
	if records[0].Fields["Artifact Fingerprint"] != request.ArtifactFingerprint {
		t.Errorf("Artifact Fingerprint = %q, want %q", records[0].Fields["Artifact Fingerprint"], request.ArtifactFingerprint)
	}
}

func TestReviewRevisionReceiptBindsChallengeAndPriorAppendix(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "intent-statement.md")
	prior := []byte("# Intent\n\n" + canonicalAuditReviewAppendix("NOT-READY"))
	if err := os.WriteFile(artifactPath, prior, 0o600); err != nil {
		t.Fatalf("WriteFile(prior artifact): %v", err)
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: prior,
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	if request.ReviewChallenge == "" {
		t.Fatal("review.NewRequest() challenge is empty for revision")
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request); err != nil {
		t.Fatalf("RecordReviewRequested(): %v", err)
	}
	if request.PriorDigest == "none" || request.PriorLength == 0 {
		t.Fatalf("revision binding = digest %q length %d, want prior appendix", request.PriorDigest, request.PriorLength)
	}
	updated := append(append([]byte(nil), prior[:request.AppendixOffset]...), []byte(canonicalAuditReviewAppendixWithChallenge("READY", request.ReviewChallenge))...)
	completion, err := review.NewCompletion(review.CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: updated})
	if err != nil {
		t.Fatalf("review.NewCompletion(): %v", err)
	}
	if err := os.WriteFile(artifactPath, updated, 0o600); err != nil {
		t.Fatalf("WriteFile(updated artifact): %v", err)
	}
	if err := RecordReviewCompleted(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request, completion); err != nil {
		t.Fatalf("RecordReviewCompleted(): %v", err)
	}
	records := mustReadAuditRecords(t, fixture)
	if len(records) != 2 {
		t.Fatalf("records = %#v, want request and completion", records)
	}
	for index, record := range records {
		if record.Fields["Review Challenge"] != request.ReviewChallenge ||
			record.Fields["Review Appendix Prior Digest"] != request.PriorDigest ||
			record.Fields["Review Appendix Prior Length"] != fmt.Sprint(request.PriorLength) {
			t.Errorf("record %d revision fields = %#v, want challenge/prior binding", index, record.Fields)
		}
		for _, legacy := range []string{"Artifact Path", "Prior Digest", "Prior Length", "Request Source", "Artifact Set", "Post Fingerprint"} {
			if _, ok := record.Fields[legacy]; ok {
				t.Errorf("record %d contains legacy field %q: %#v", index, legacy, record.Fields)
			}
		}
	}
}

func TestReviewRevisionAllowsSameArtifactFingerprintAfterRejection(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "intent-statement.md")
	before := []byte("# Intent\n\n")
	if err := os.WriteFile(artifactPath, before, 0o600); err != nil {
		t.Fatalf("WriteFile(artifact): %v", err)
	}
	first, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before,
	})
	if err != nil {
		t.Fatalf("review.NewRequest(first): %v", err)
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, first); err != nil {
		t.Fatalf("RecordReviewRequested(first): %v", err)
	}
	after := append(append([]byte(nil), before...), []byte(canonicalAuditReviewAppendix("NOT-READY"))...)
	completion, err := review.NewCompletion(review.CompletionInput{Request: first, Verdict: "NOT-READY", ArtifactAfter: after})
	if err != nil {
		t.Fatalf("review.NewCompletion(first): %v", err)
	}
	if err := os.WriteFile(artifactPath, after, 0o600); err != nil {
		t.Fatalf("WriteFile(first appendix): %v", err)
	}
	if err := RecordReviewCompleted(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, first, completion); err != nil {
		t.Fatalf("RecordReviewCompleted(first): %v", err)
	}
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		return Append(context.Background(), guard, fixture.projectRoot, fixture.recordRoot, []Event{
			{Event: "GATE_REJECTED", Fields: map[string]string{"Stage": "intent-capture"}},
			{Event: "STAGE_REVISING", Fields: map[string]string{"Stage": "intent-capture"}},
		})
	}); err != nil {
		t.Fatalf("Append(revision anchor): %v", err)
	}
	second, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: after,
	})
	if err != nil {
		t.Fatalf("review.NewRequest(second): %v", err)
	}
	if second.ArtifactFingerprint != first.ArtifactFingerprint {
		t.Fatalf("revision artifact fingerprint = %q, want unchanged %q", second.ArtifactFingerprint, first.ArtifactFingerprint)
	}
	if second.ReviewChallenge == "" {
		t.Fatal("revision request challenge is empty")
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, second); err != nil {
		t.Fatalf("RecordReviewRequested(second): %v, want fresh revision request", err)
	}
}

func TestIntentCaptureReviewCompletion(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	before := []byte("# Intent\n\n")
	artifactPath := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build", "intent-statement.md")
	if err := os.WriteFile(artifactPath, before, 0o600); err != nil {
		t.Fatalf("WriteFile(artifact): %v", err)
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request); err != nil {
		t.Fatalf("RecordReviewRequested(): %v", err)
	}
	after := append(append([]byte(nil), before...), []byte(canonicalAuditReviewAppendix("READY"))...)
	completion, err := review.NewCompletion(review.CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: after})
	if err != nil {
		t.Fatalf("review.NewCompletion(): %v", err)
	}
	if err := os.WriteFile(artifactPath, after, 0o600); err != nil {
		t.Fatalf("WriteFile(reviewed artifact): %v", err)
	}
	if err := RecordReviewCompleted(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request, completion); err != nil {
		t.Fatalf("RecordReviewCompleted(): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if len(records) != 2 || records[1].Event != "REVIEW_COMPLETED" || records[1].Fields["Verdict"] != "READY" {
		t.Fatalf("records = %#v, want request then READY completion", records)
	}
}

func TestReviewReceiptRejectsChangedDeclaredArtifact(t *testing.T) {
	fixture := newHumanTurnWorkspaceFixture(t)
	recordDir := filepath.Join(fixture.project, "aidlc", "spaces", "team", "intents", "build")
	snapshots := map[string][]byte{
		"ideation/intent-capture/intent-capture-questions.md": []byte("questions-v1\n"),
		"ideation/intent-capture/intent-statement.md":         []byte("intent-v1\n"),
		"ideation/intent-capture/stakeholder-map.md":          []byte("stakeholders-v1\n"),
	}
	for name, content := range snapshots {
		fullPath := filepath.Join(recordDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s): %v", name, err)
		}
		if err := os.WriteFile(fullPath, content, 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	request, err := review.NewRequest(review.RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath:      "ideation/intent-capture/intent-statement.md",
		ArtifactSnapshot:  snapshots["ideation/intent-capture/intent-statement.md"],
		ArtifactSnapshots: snapshots, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("review.NewRequest(): %v", err)
	}
	if err := RecordReviewRequested(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request); err != nil {
		t.Fatalf("RecordReviewRequested(): %v", err)
	}
	after := append(append([]byte(nil), snapshots[request.ArtifactPath]...), []byte(canonicalAuditReviewAppendix("READY"))...)
	completion, err := review.NewCompletion(review.CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: after})
	if err != nil {
		t.Fatalf("review.NewCompletion(): %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, filepath.FromSlash(request.ArtifactPath)), after, 0o600); err != nil {
		t.Fatalf("WriteFile(reviewed artifact): %v", err)
	}
	if err := RecordReviewCompleted(context.Background(), fixture.identity, fixture.projectRoot, fixture.recordRoot, request, completion); err != nil {
		t.Fatalf("RecordReviewCompleted(): %v", err)
	}
	if err := ValidateReviewReceiptCurrent(fixture.recordRoot, mustReadAuditRecords(t, fixture), "intent-capture", "aidlc-product-lead-agent"); err != nil {
		t.Fatalf("ValidateReviewReceiptCurrent() before change: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, filepath.FromSlash("ideation/intent-capture/stakeholder-map.md")), []byte("stakeholders-v2\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed stakeholder map): %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var readErr error
		records, readErr = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return readErr
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	if err := ValidateReviewReceiptCurrent(fixture.recordRoot, records, "intent-capture", "aidlc-product-lead-agent"); err == nil {
		t.Fatal("ValidateReviewReceiptCurrent() error = nil after stakeholder-map change, want stale rejection")
	}
}

func mustReadAuditRecords(t *testing.T, fixture humanTurnWorkspaceFixture) []AuditRecord {
	t.Helper()
	var records []AuditRecord
	if err := recordlock.With(context.Background(), fixture.identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), fixture.identity, guard, fixture.projectRoot, fixture.recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents(): %v", err)
	}
	return records
}

func canonicalAuditReviewAppendix(verdict string) string {
	return "## Review\n" +
		"**Verdict:** " + verdict + "\n" +
		"**Reviewer:** aidlc-product-lead-agent\n" +
		"**Date:** 2026-09-06T00:00:00Z\n" +
		"**Iteration:** 1\n\n" +
		"### Findings\n\n" +
		"| ID | Severity | Location | Finding | Required action | Status |\n" +
		"|---|---|---|---|---|---|\n\n" +
		"### Summary\n\n" +
		"The artifact is ready for implementation.\n"
}

func canonicalAuditReviewAppendixWithChallenge(verdict, challenge string) string {
	return "## Review\n" +
		"**Verdict:** " + verdict + "\n" +
		"**Reviewer:** aidlc-product-lead-agent\n" +
		"**Date:** 2026-09-06T00:00:00Z\n" +
		"**Iteration:** 1\n" +
		"**Request Challenge:** " + challenge + "\n\n" +
		"### Findings\n\n" +
		"| ID | Severity | Location | Finding | Required action | Status |\n" +
		"|---|---|---|---|---|---|\n\n" +
		"### Summary\n\n" +
		"The revised artifact is ready for implementation.\n"
}
