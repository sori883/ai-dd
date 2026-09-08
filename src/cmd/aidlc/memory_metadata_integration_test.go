//go:build integration

package main

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryMetadataCommand(t *testing.T) {
	binary := buildMinimalBinary(t)
	root := t.TempDir()
	runMinimalProcess(t, root, "git", "init", "-q")
	runMinimalCLI(t, binary, root, nil, "install", "codex", "--project-dir", root)
	unrelated := t.TempDir()
	if out := runMinimalCLI(t, binary, unrelated, nil, "memory", "create", "--help"); !strings.Contains(string(out), "generated.at") {
		t.Fatal("help missing metadata")
	}
	nested := filepath.Join(root, "drafts")
	writeMinimalFixture(t, filepath.Join(nested, "body.md"), "# First body\n")
	for _, tc := range []struct{ id, kind string }{{"knowledge/example", "Design"}, {"ADR/example", "ADR"}, {"rules/example", "Rule"}} {
		args := []string{"memory", "create", tc.id, "--space", "default", "--project-dir", root, "--body-file", "body.md", "--actor", "process:codex", "--type", tc.kind, "--title", "Example", "--description", "Current behavior", "--tag", "lookup", "--metadata-json", `{"extension":{"keep":true}}`}
		if tc.kind != "Rule" {
			args = append(args, "--intent-id", strings.Repeat("a", 32))
		}
		runMinimalCLI(t, binary, nested, nil, args...)
		show := runMinimalCLI(t, binary, root, nil, "memory", "show", tc.id, "--space", "default")
		var before struct{ Content, Hash string }
		if err := json.Unmarshal(show, &before); err != nil {
			t.Fatal(err)
		}
		doc, err := okfmemory.Parse([]byte(before.Content))
		if err != nil {
			t.Fatal(err)
		}
		if doc.String("type") != tc.kind || doc.Body != "# First body\n" || doc.Metadata["generated"] == nil || doc.Metadata["extension"] == nil {
			t.Fatalf("create %s", before.Content)
		}
		if tc.kind == "Rule" && doc.String("intent_id") != "" {
			t.Fatal("Rule received implicit Intent")
		}
		writeMinimalFixture(t, filepath.Join(nested, "body.md"), "# Second body\n")
		runMinimalCLI(t, binary, nested, nil, "memory", "update", tc.id, "--space", "default", "--project-dir", root, "--body-file", "body.md", "--actor", "human:editor", "--expect", before.Hash)
		raw, err := os.ReadFile(filepath.Join(root, "aidlc/spaces/default/knowledge", tc.id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		updated, err := okfmemory.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if updated.String("type") != tc.kind || updated.String("title") != "Example" || updated.Metadata["extension"] == nil || updated.Body != "# Second body\n" {
			t.Fatalf("update %s", raw)
		}
		if got := updated.Metadata["generated"].(map[string]any)["by"]; got != "human:editor" {
			t.Fatal(got)
		}
		cmd := exec.Command(binary, "memory", "update", tc.id, "--space", "default", "--project-dir", root, "--body-file", filepath.Join(nested, "body.md"), "--actor", "human:editor", "--expect", before.Hash)
		if err := cmd.Run(); err == nil {
			t.Fatal("stale hash accepted")
		}
		writeMinimalFixture(t, filepath.Join(nested, "body.md"), "# First body\n")
	}
	out := runMinimalCLI(t, binary, root, nil, "memory", "search", "lookup", "--space", "default", "--intent-id", strings.Repeat("a", 32))
	var found []map[string]string
	if err := json.Unmarshal(out, &found); err != nil || len(found) != 2 {
		t.Fatalf("search %s %v", out, err)
	}
	runMinimalCLI(t, binary, root, nil, "memory", "check", "--space", "default")
}
