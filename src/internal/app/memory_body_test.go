package app

import (
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bodyString(s string) *string { return &s }
func bodyRequest(file string) cli.CommandRequest {
	return cli.CommandRequest{Command: "memory", Action: "create", Target: "codekb/note", Space: "default", BodyFile: file, Actor: "process:test", Metadata: okfmemory.MetadataInput{Type: bodyString("Note"), Title: bodyString("Memory"), Description: bodyString("Example"), ExtraJSON: bodyString(`{"custom":"keep"}`)}}
}

func TestMemoryCommandForwarding(t *testing.T) {
	s, _ := setup(t)
	bundle := filepath.Join(s.Root, "aidlc/spaces/team/knowledge")
	if err := os.MkdirAll(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(s.Root, "body.md")
	if err := os.WriteFile(draft, []byte("First.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 32)
	r := bodyRequest(draft)
	r.Space, r.Target, r.ProjectDir = "team", "notes/forwarded", s.Root
	r.Metadata.IntentID = &id
	out, err := s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	doc, err := okfmemory.Read(bundle, r.Target)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Body != "First.\n" || doc.String("intent_id") != id || doc.String("title") != "Memory" || doc.Metadata["generated"].(map[string]any)["by"] != r.Actor {
		t.Fatalf("create forwarding: %+v", doc)
	}
	if err := os.WriteFile(draft, []byte("Second.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r.Action, r.Expect, r.Actor = "update", result["hash"], "human:editor"
	r.Metadata = okfmemory.MetadataInput{Title: bodyString("Updated")}
	if _, err := s.Execute(r); err != nil {
		t.Fatal(err)
	}
	doc, err = okfmemory.Read(bundle, r.Target)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Body != "Second.\n" || doc.String("title") != "Updated" || doc.Metadata["generated"].(map[string]any)["by"] != r.Actor {
		t.Fatalf("update forwarding: %+v", doc)
	}
	out, err = s.Execute(cli.CommandRequest{Command: "memory", Action: "search", Space: r.Space, IntentID: &id})
	if err != nil || !strings.Contains(string(out), "notes/forwarded") {
		t.Fatalf("intent search: %s %v", out, err)
	}
	if _, err := s.Execute(r); err == nil {
		t.Fatal("stale expect accepted")
	}
}
