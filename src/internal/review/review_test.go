package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestIntentCaptureReviewRequest(t *testing.T) {
	request, err := NewRequest(RequestInput{
		Stage:            "intent-capture",
		Reviewer:         "aidlc-product-lead-agent",
		Iteration:        1,
		ArtifactPath:     "intent-statement.md",
		ArtifactSnapshot: []byte("# Intent\n\n"),
		RequestSource:    "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if request.RequestFingerprint == "" || request.ArtifactFingerprint == "" {
		t.Fatal("review request fingerprints are empty")
	}
	if request.AppendixOffset != int64(len(request.ArtifactSnapshot)) {
		t.Errorf("AppendixOffset = %d, want %d", request.AppendixOffset, len(request.ArtifactSnapshot))
	}
	if request.PriorLength != 0 || request.PriorDigest != "none" || request.ReviewChallenge != "" {
		t.Errorf("first request prior binding = digest %q length %d challenge %q, want none/0/no challenge", request.PriorDigest, request.PriorLength, request.ReviewChallenge)
	}
}

func TestReviewRequestFingerprintBindsDeclaredArtifacts(t *testing.T) {
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: []byte("intent-v1\n"),
		ArtifactSnapshots: map[string][]byte{
			"intent-capture-questions.md": []byte("questions-v1\n"),
			"intent-statement.md":         []byte("intent-v1\n"),
			"stakeholder-map.md":          []byte("stakeholders-v1\n"),
		},
		RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest(): %v", err)
	}
	changed, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: []byte("intent-v1\n"),
		ArtifactSnapshots: map[string][]byte{
			"intent-capture-questions.md": []byte("questions-v1\n"),
			"intent-statement.md":         []byte("intent-v1\n"),
			"stakeholder-map.md":          []byte("stakeholders-v2\n"),
		},
		RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest(changed): %v", err)
	}
	if request.ArtifactFingerprint == changed.ArtifactFingerprint {
		t.Fatal("ArtifactFingerprint did not change when a declared artifact changed")
	}
	if request.RequestFingerprint == changed.RequestFingerprint {
		t.Fatal("RequestFingerprint did not change when a declared artifact changed")
	}
}

func TestIntentCaptureReviewCompletion(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before,
		RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	after := append(append([]byte(nil), before...), []byte(canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1))...)
	completion, err := NewCompletion(CompletionInput{
		Request:       request,
		Verdict:       "READY",
		ArtifactAfter: after,
	})
	if err != nil {
		t.Fatalf("NewCompletion() error = %v", err)
	}
	if err := ValidateCompletion(request, completion); err != nil {
		t.Fatalf("ValidateCompletion() error = %v", err)
	}
	if completion.PostFingerprint == request.ArtifactFingerprint {
		t.Fatal("post fingerprint did not change after review appendix")
	}
}

func TestReviewRejectsSubstringAuthorityFields(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	valid := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	for name, appendix := range map[string]string{
		"verdict suffix":   strings.Replace(valid, "**Verdict:** READY", "**Verdict:** READY-ish", 1),
		"reviewer suffix":  strings.Replace(valid, "**Reviewer:** aidlc-product-lead-agent", "**Reviewer:** aidlc-product-lead-agent-extra", 1),
		"iteration prefix": strings.Replace(valid, "**Iteration:** 1", "**Iteration:** 10", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), before...), appendix...)})
			if !errors.Is(err, ErrMalformedAppendix) {
				t.Fatalf("NewCompletion() error = %v, want ErrMalformedAppendix", err)
			}
		})
	}
}

func TestReviewRequiresCanonicalTerminalStructure(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	short := "## Review\n**Verdict:** READY\n**Reviewer:** aidlc-product-lead-agent\n**Iteration:** 1\n"
	_, err = NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), before...), short...)})
	if !errors.Is(err, ErrMalformedAppendix) {
		t.Fatalf("NewCompletion() error = %v, want ErrMalformedAppendix", err)
	}
}

func TestReviewRequiresAuthorityBlockAndUniqueFindingsTable(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before,
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	valid := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	header := "| ID | Severity | Location | Finding | Required action | Status |\n"
	cases := map[string]string{
		"authority after summary":   valid + "\n**Reviewer:** aidlc-product-lead-agent\n",
		"extra h3 before findings":  strings.Replace(valid, "### Findings\n", "### Extra\n\n### Findings\n", 1),
		"duplicate findings header": strings.Replace(valid, header, header+header, 1),
	}
	for name, appendix := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), before...), []byte(appendix)...)})
			if !errors.Is(err, ErrMalformedAppendix) {
				t.Fatalf("NewCompletion() error = %v, want ErrMalformedAppendix", err)
			}
		})
	}
}

func TestReviewRejectsHiddenOrNestedAuthority(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	valid := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	cases := map[string]string{
		"multiline comment":        strings.Replace(valid, "**Date:** 2026-09-06T00:00:00Z\n", "<!--\n**Date:** 2026-09-06T00:00:00Z\n-->\n", 1),
		"tilde fence":              strings.Replace(valid, "**Date:** 2026-09-06T00:00:00Z\n", "~~~\n**Date:** 2026-09-06T00:00:00Z\n~~~\n", 1),
		"long fence shorter close": strings.Replace(valid, "**Date:** 2026-09-06T00:00:00Z\n", "````\n**Date:** hidden\n```\n**Date:** 2026-09-06T00:00:00Z\n````\n```\n```\n", 1),
		"list container":           strings.Replace(valid, "**Reviewer:** aidlc-product-lead-agent\n", "- **Reviewer:** aidlc-product-lead-agent\n", 1),
		"blockquote container":     strings.Replace(valid, "**Reviewer:** aidlc-product-lead-agent\n", "> **Reviewer:** aidlc-product-lead-agent\n", 1),
		"table container":          strings.Replace(valid, "**Reviewer:** aidlc-product-lead-agent\n", "| **Reviewer:** aidlc-product-lead-agent |\n", 1),
	}
	for name, appendix := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), before...), appendix...)})
			if !errors.Is(err, ErrMalformedAppendix) {
				t.Fatalf("NewCompletion() error = %v, want ErrMalformedAppendix", err)
			}
		})
	}
}

func TestReviewRejectsRenderedHeadingEscapes(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	valid := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	for name, heading := range map[string]string{
		"raw h1":    "<h1>Escape</h1>",
		"raw h2":    "<h2>Escape</h2>",
		"setext h1": "Escape\n====",
		"setext h2": "Escape\n----",
	} {
		t.Run(name, func(t *testing.T) {
			appendix := strings.Replace(valid, "The artifact is ready for implementation.", heading, 1)
			_, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), before...), appendix...)})
			if !errors.Is(err, ErrMalformedAppendix) {
				t.Fatalf("NewCompletion() error = %v, want ErrMalformedAppendix", err)
			}
		})
	}
}

func TestReviewRequestRejectsExistingReviewAppendix(t *testing.T) {
	prefix := []byte("# Intent\n\n")
	prior := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	snapshot := append(append([]byte(nil), prefix...), []byte(prior)...)
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: snapshot, RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v, want revision request", err)
	}
	if request.AppendixOffset != int64(len(prefix)) {
		t.Fatalf("AppendixOffset = %d, want %d", request.AppendixOffset, len(prefix))
	}
	if request.PriorLength != int64(len(prior)) {
		t.Fatalf("PriorLength = %d, want %d", request.PriorLength, len(prior))
	}
	digest := sha256.Sum256([]byte(prior))
	if request.PriorDigest != "sha256:"+hex.EncodeToString(digest[:]) {
		t.Fatalf("PriorDigest = %q, want canonical appendix digest", request.PriorDigest)
	}
	if !regexp.MustCompile(`^review:[0-9a-f]{32}$`).MatchString(request.ReviewChallenge) {
		t.Fatalf("ReviewChallenge = %q, want review:<32 lowercase hex>", request.ReviewChallenge)
	}
}

func TestReviewRevisionUsesExistingAppendixIterationAndRejectsMultipleTerminals(t *testing.T) {
	prefix := []byte("# Intent\n\n")
	prior := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	snapshot := append(append([]byte(nil), prefix...), []byte(prior)...)
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 2,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: snapshot,
	})
	if err != nil {
		t.Fatalf("NewRequest(iteration 2 over iteration 1 appendix): %v", err)
	}
	if request.Iteration != 2 || request.ReviewChallenge == "" || request.PriorLength == 0 {
		t.Fatalf("request = %#v, want fresh iteration 2 challenge over prior appendix", request)
	}
	duplicate := append(append([]byte(nil), snapshot...), []byte(prior)...)
	if _, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 2,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: duplicate,
	}); !errors.Is(err, ErrMalformedAppendix) {
		t.Fatalf("NewRequest(duplicate terminal) error = %v, want ErrMalformedAppendix", err)
	}
}

func TestReviewChallengeRandomFailureFailsClosed(t *testing.T) {
	prefix := []byte("# Intent\n\n")
	prior := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	oldReader := reviewChallengeRandom
	reviewChallengeRandom = errorReader{err: errors.New("entropy unavailable")}
	defer func() { reviewChallengeRandom = oldReader }()
	_, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 2,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: append(append([]byte(nil), prefix...), []byte(prior)...),
	})
	if err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("NewRequest(entropy failure) error = %v, want fail-closed entropy error", err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestReviewRevisionCompletionRequiresFreshChallenge(t *testing.T) {
	prefix := []byte("# Intent\n\n")
	prior := canonicalReviewAppendix("READY", "aidlc-product-lead-agent", 1)
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: append(append([]byte(nil), prefix...), []byte(prior)...), RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	oldReplay := append(append([]byte(nil), prefix...), []byte(prior)...)
	if _, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: oldReplay}); !errors.Is(err, ErrArtifactChanged) {
		t.Fatalf("NewCompletion(old replay) error = %v, want ErrArtifactChanged", err)
	}
	fresh := canonicalReviewAppendixWithChallenge("READY", "aidlc-product-lead-agent", 1, request.ReviewChallenge)
	completion, err := NewCompletion(CompletionInput{Request: request, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), prefix...), []byte(fresh)...)})
	if err != nil {
		t.Fatalf("NewCompletion(fresh challenge) error = %v", err)
	}
	if completion.PostFingerprint == "" || !strings.HasPrefix(completion.PostFingerprint, "sha256:") {
		t.Fatalf("PostFingerprint = %q, want sha256 manifest fingerprint", completion.PostFingerprint)
	}
}

func TestReviewRevisionCompletionAllowsRemovedPriorAppendix(t *testing.T) {
	prefix := []byte("# Intent\n\n")
	prior := canonicalReviewAppendix("NOT-READY", "aidlc-product-lead-agent", 1)
	original, err := NewRequest(RequestInput{Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1, ArtifactPath: "intent-statement.md", ArtifactSnapshot: append(append([]byte(nil), prefix...), []byte(prior)...), RequestSource: "ignored"})
	if err != nil {
		t.Fatalf("NewRequest(): %v", err)
	}
	rehydrated := Request{
		Stage: original.Stage, Reviewer: original.Reviewer, Iteration: original.Iteration,
		ArtifactPath: original.ArtifactPath, ArtifactSnapshot: prefix,
		ArtifactSnapshots:   map[string][]byte{original.ArtifactPath: prefix},
		ArtifactFingerprint: ArtifactSnapshotsRequestFingerprint(map[string][]byte{original.ArtifactPath: prefix}, original.ArtifactPath, int64(len(prefix))),
		PriorDigest:         original.PriorDigest, PriorLength: original.PriorLength, ReviewChallenge: original.ReviewChallenge,
		AppendixOffset: int64(len(prefix)), PriorAppendixRemoved: true,
	}
	rehydrated.RequestFingerprint = rehydrated.ArtifactFingerprint
	fresh := canonicalReviewAppendixWithChallenge("READY", rehydrated.Reviewer, rehydrated.Iteration, rehydrated.ReviewChallenge)
	completion, err := NewCompletion(CompletionInput{Request: rehydrated, Verdict: "READY", ArtifactAfter: append(append([]byte(nil), prefix...), []byte(fresh)...)})
	if err != nil {
		t.Fatalf("NewCompletion(removed prior appendix): %v", err)
	}
	if err := ValidateCompletion(rehydrated, completion); err != nil {
		t.Fatalf("ValidateCompletion(): %v", err)
	}
}

func canonicalReviewAppendixWithChallenge(verdict, reviewer string, iteration int, challenge string) string {
	return "## Review\n" +
		"**Verdict:** " + verdict + "\n" +
		"**Reviewer:** " + reviewer + "\n" +
		"**Date:** 2026-09-06T00:00:00Z\n" +
		"**Iteration:** " + fmt.Sprint(iteration) + "\n" +
		"**Request Challenge:** " + challenge + "\n\n" +
		"### Findings\n\n" +
		"| ID | Severity | Location | Finding | Required action | Status |\n" +
		"|---|---|---|---|---|---|\n\n" +
		"### Summary\n\n" +
		"The artifact is ready for implementation.\n"
}

func canonicalReviewAppendix(verdict, reviewer string, iteration int) string {
	return "## Review\n" +
		"**Verdict:** " + verdict + "\n" +
		"**Reviewer:** " + reviewer + "\n" +
		"**Date:** 2026-09-06T00:00:00Z\n" +
		"**Iteration:** " + fmt.Sprint(iteration) + "\n\n" +
		"### Findings\n\n" +
		"| ID | Severity | Location | Finding | Required action | Status |\n" +
		"|---|---|---|---|---|---|\n\n" +
		"### Summary\n\n" +
		"The artifact is ready for implementation.\n"
}

func TestIntentCaptureReviewRejectsChangedOrMalformedAppendix(t *testing.T) {
	before := []byte("# Intent\n\n")
	request, err := NewRequest(RequestInput{
		Stage: "intent-capture", Reviewer: "aidlc-product-lead-agent", Iteration: 1,
		ArtifactPath: "intent-statement.md", ArtifactSnapshot: before,
		RequestSource: "directive-v1",
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	cases := []struct {
		name    string
		after   []byte
		verdict string
		want    error
	}{
		{name: "changed prefix", after: []byte("# Replaced\n## Review\n**Verdict:** READY\n**Reviewer:** aidlc-product-lead-agent\n**Iteration:** 1\n"), verdict: "READY", want: ErrArtifactChanged},
		{name: "missing terminal", after: append(append([]byte(nil), before...), []byte("draft\n")...), verdict: "READY", want: ErrMalformedAppendix},
		{name: "wrong reviewer", after: append(append([]byte(nil), before...), []byte("## Review\n**Verdict:** READY\n**Reviewer:** other\n**Iteration:** 1\n")...), verdict: "READY", want: ErrMalformedAppendix},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			completion, err := NewCompletion(CompletionInput{Request: request, Verdict: tt.verdict, ArtifactAfter: tt.after})
			if err == nil {
				err = ValidateCompletion(request, completion)
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("review validation error = %v, want %v", err, tt.want)
			}
		})
	}
	if bytes.Equal(before, request.ArtifactSnapshot) == false {
		t.Fatal("NewRequest mutated artifact snapshot")
	}
}
