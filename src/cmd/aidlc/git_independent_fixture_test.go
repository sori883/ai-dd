package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGitIndependentFixture(t *testing.T) {
	env := gitIndependentEnvironment([]string{"PATH=/has/git", "OTHER=value"}, "/isolated")
	if strings.Join(env, "|") != "OTHER=value|PATH=/isolated" {
		t.Fatalf("product PATH not isolated: %v", env)
	}
	raw, err := gitIndependentResult("s04", "tdd", strings.Repeat("a", 64), []string{"a", "b"}, "test", "aidlc/evidence/output")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Scope string `json:"verification_scope"`
		SHA   string `json:"verification_sha256"`
		Runs  []struct {
			Unit    string `json:"unit_id"`
			Command string `json:"command"`
		}
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Scope != "intent" || len(doc.SHA) != 64 || len(doc.Runs) != 2 || doc.Runs[0].Unit != "a" || doc.Runs[1].Unit != "b" {
		t.Fatalf("invalid result %s", raw)
	}
	if _, err = gitIndependentResult("s04", "tdd", "bad", nil, "test", "aidlc/evidence/output"); err == nil {
		t.Fatal("accepted invalid SHA")
	}
}
