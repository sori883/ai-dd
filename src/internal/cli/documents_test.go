package cli

import (
	"strings"
	"testing"
)

func TestIntentDocumentsGrammar(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, tail := range [][]string{{}, {"--expect", "1", "--file", "docs.json"}} {
		args := append([]string{"intent", "documents", id, "--space", "default"}, tail...)
		if _, err := ParseMinimal(args); err != nil {
			t.Fatalf("valid documents: %v", err)
		}
	}
	for _, tail := range [][]string{{"--expect", "1"}, {"--file", "docs.json"}, {"--raw"}} {
		args := append([]string{"intent", "documents", id, "--space", "default"}, tail...)
		if _, err := ParseMinimal(args); err == nil {
			t.Fatal("incomplete/unknown flags accepted")
		}
	}
	if text, ok := Help([]string{"intent", "documents", "--help"}); !ok || !strings.Contains(text, "metadata") {
		t.Fatal("documents help missing")
	}
}
