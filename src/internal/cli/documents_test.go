package cli

import (
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"strings"
	"testing"
)

func TestIntentDocumentsGrammar(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, tail := range [][]string{{}, {"--expect", "1", "--file", "docs.json"}} {
		args := append([]string{"intent", "documents", id, "--space", "default"}, tail...)
		if _, err := ParseCommand(args); err != nil {
			t.Fatalf("valid documents: %v", err)
		}
	}
	for _, tail := range [][]string{{"--expect", "1"}, {"--file", "docs.json"}, {"--raw"}} {
		args := append([]string{"intent", "documents", id, "--space", "default"}, tail...)
		if _, err := ParseCommand(args); err == nil {
			t.Fatal("incomplete/unknown flags accepted")
		}
	}
	if text, ok := Help([]string{"intent", "documents", "--help"}); !ok || !strings.Contains(text, "metadata") {
		t.Fatal("documents help missing")
	}
}

func TestIntentDocumentsLowercaseHelp(t *testing.T) {
	for _, action := range []string{"create", "update"} {
		text, ok := okfcli.Help([]string{action, "--help"})
		if !ok || !strings.Contains(text, "Design / adr / Rule") || strings.Contains(text, "Design / ADR / Rule") {
			t.Fatalf("%s help has incorrect adr type example", action)
		}
	}
}

func TestSplitRuntimeHelp(t *testing.T) {
	text, ok := Help([]string{"intent", "documents", "--help"})
	if !ok || !strings.Contains(text, "okf create --intent-id") || strings.Contains(text, "memory create") || !strings.Contains(text, "aidlc/bin/VERSION/okf") {
		t.Fatalf("incorrect knowledge CLI guidance: %s", text)
	}
}
