//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/okf"
)

func TestKnowledgeSearchFreshNonLiveJourney(t *testing.T) {
	moduleRoot := deliveryModuleRoot(t)
	binary := buildIntentCaptureBinary(t, moduleRoot)
	fixture := newCodexReceiverJourneyProject(t, moduleRoot)
	bundle := filepath.Join(fixture.Project, "aidlc", "spaces", "team", "knowledge", "okf")
	if err := os.MkdirAll(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	for i := range 6 {
		writeDeliveryJourneyFile(t, filepath.Join(bundle, fmt.Sprintf("%d.md", i)), fmt.Sprintf("---\ntype: Guide\ntags: [topic]\ntitle: Concept %d\n---\nSECRET_BODY_%d\n", i, i))
	}
	writeDeliveryJourneyFile(t, filepath.Join(bundle, "bad.md"), "invalid")
	search := func(extra ...string) okf.SearchResult {
		t.Helper()
		args := append([]string{"knowledge", "search", "--project-dir", fixture.Project}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = fixture.Project
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("search %v: %v %s", extra, err, &stderr)
		}
		if stderr.Len() != 0 || strings.Contains(stdout.String(), "SECRET_BODY") || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
			t.Fatalf("invalid output %s %s", &stdout, &stderr)
		}
		var result okf.SearchResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := search("--tag", "topic")
	if len(first.Results) != 4 || len(first.Warnings) != 1 {
		t.Fatalf("first %+v", first)
	}
	second := search("--query", "5")
	if len(second.Results) != 1 || second.Results[0].ConceptID != "5" {
		t.Fatalf("second %+v", second)
	}
	selected := filepath.Join(fixture.Project, filepath.FromSlash(second.Results[0].Path))
	text, err := os.ReadFile(selected)
	if err != nil || !strings.Contains(string(text), "SECRET_BODY_5") {
		t.Fatalf("selected body %s %v", text, err)
	}
	writeDeliveryJourneyFile(t, selected, "---\ntype: Revised\n---\nUPDATED_SECRET\n")
	if got := search("--type", "Revised"); len(got.Results) != 1 {
		t.Fatalf("fresh search %+v", got)
	}
	next := runCodexReceiverDirective(t, binary, fixture.Project, "next")
	for next.Kind == "load-steering" {
		next = runCodexReceiverDirective(t, binary, fixture.Project, "continue", next.ContinueToken)
	}
	if next.Kind != "run-stage" || !strings.Contains(string(next.Raw), "aidlc knowledge search") || strings.Contains(string(next.Raw), "SECRET_BODY") {
		t.Fatalf("directive %s", next.Raw)
	}
}
