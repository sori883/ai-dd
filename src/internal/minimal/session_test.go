package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/flow"
	"github.com/sori883/ai-dd/src/internal/install"
)

func setup(t *testing.T) (Service, flow.State) {
	t.Helper()
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root, Binary: "/opt/aidlc"}
	store := flow.Store{Root: root, Space: "default"}
	saved, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	return s, saved
}
func hook(t *testing.T, s Service, event, tool, id, command string, active bool) map[string]any {
	t.Helper()
	in := HookInput{Event: event, Session: "session", Turn: "turn", Tool: tool, ID: id, Active: active}
	in.Input.Command = command
	out, err := s.Hook(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func deny(out map[string]any) bool {
	specific, _ := out["hookSpecificOutput"].(map[string]any)
	return specific["permissionDecision"] == "deny"
}
func bind(t *testing.T, s Service, id string) {
	t.Helper()
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: id, Space: "default", Session: "session"}); err != nil {
		t.Fatal(err)
	}
}
func TestRulesFullTextAndLimit(t *testing.T) {
	s, _ := setup(t)
	raw, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "rules", Space: "default"})
	if err != nil || !strings.Contains(string(raw), "作業の合意") {
		t.Fatalf("rules = %s, %v", raw, err)
	}
	path := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	if err := os.WriteFile(path, []byte("---\ntype: Rule\n---\n"+strings.Repeat("x", 17000)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "rules", Space: "default"}); err == nil {
		t.Fatal("truncated oversized Rule")
	}
}
func TestSessionStartLoadsPlacedSkillOnly(t *testing.T) {
	s, _ := setup(t)
	placed := filepath.Join(s.Root, ".agents/skills/aidlc/SKILL.md")
	if err := os.WriteFile(placed, []byte("PLACED BOOTSTRAP: use the named Intent."), 0600); err != nil {
		t.Fatal(err)
	}
	out := hook(t, s, "SessionStart", "", "", "", false)
	specific, _ := out["hookSpecificOutput"].(map[string]any)
	context, _ := specific["additionalContext"].(string)
	if !strings.Contains(context, "PLACED BOOTSTRAP") {
		t.Fatalf("did not deliver deployed skill: %+v", out)
	}
	state, err := s.Inspect("session")
	if err != nil || state.RuleHash != "" || state.RuleTurn != "" {
		t.Fatalf("SessionStart marked Rules ready: %+v, %v", state, err)
	}
	if err := os.Remove(placed); err != nil {
		t.Fatal(err)
	}
	if out := hook(t, s, "SessionStart", "", "", "", false); out["continue"] != false {
		t.Fatalf("missing skill did not stop bootstrap: %+v", out)
	}
	if err := os.WriteFile(placed, []byte(strings.Repeat("x", 4097)), 0600); err != nil {
		t.Fatal(err)
	}
	if out := hook(t, s, "SessionStart", "", "", "", false); out["continue"] != false {
		t.Fatalf("oversized skill was silently truncated: %+v", out)
	}
}

func TestSessionMemoryWritesAndSearchPreserveSelection(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	before, err := os.ReadFile(filepath.Join(s.Root, "aidlc/.runtime/flow/sessions/session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	draft := s.draftPath("session")
	if err := os.MkdirAll(filepath.Dir(draft), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, []byte("---\ntype: Note\ntitle: Memory\ndescription: Retained metadata\ncustom: keep\n---\nFirst body.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	request := cli.MinimalRequest{Command: "memory", Action: "create", Target: "knowledge/note", Space: "default", File: draft, Actor: "process:test"}
	output, err := s.Execute(request)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, []byte(strings.Replace(string(raw), "First body.", "Second body.", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	request.Action = "update"
	request.Expect = result["hash"]
	if _, err := s.Execute(request); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(request); err == nil {
		t.Fatal("accepted stale memory hash")
	}
	id := saved.ID
	if _, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "search", Space: "default", IntentID: &id}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(s.Root, "aidlc/.runtime/flow/sessions/session.txt"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("knowledge operation changed session: %v", err)
	}
}

func TestSessionMemoryBoundaries(t *testing.T) {
	s, _ := setup(t)
	draft := filepath.Join(s.Root, "draft.md")
	body := "---\ntype: Note\ntitle: Memory\ndescription: Example\ncustom: keep\n---\nFirst.\n"
	if err := os.WriteFile(draft, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r := cli.MinimalRequest{Command: "memory", Action: "create", Target: "knowledge/note", Space: "default", File: draft, Actor: "process:test"}
	output, err := s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/note.md")
	raw, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	dropped := strings.Replace(string(raw), "custom: keep\n", "", 1)
	dropped = strings.Replace(dropped, "First.", "Second.", 1)
	if err := os.WriteFile(draft, []byte(dropped), 0600); err != nil {
		t.Fatal(err)
	}
	r.Action = "update"
	r.Expect = result["hash"]
	if _, err := s.Execute(r); err == nil {
		t.Fatal("memory update dropped unknown metadata")
	}
	r.Action = "create"
	other := filepath.Join(s.Root, "aidlc/spaces/other/knowledge")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	out, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "search", Space: "other", Target: "Memory"})
	if err != nil || strings.TrimSpace(string(out)) != "[]" {
		t.Fatalf("cross-Space search = %s, %v", out, err)
	}
	index := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/index.md")
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(index, 0755); err != nil {
		t.Fatal(err)
	}
	r.Target = "knowledge/partial"
	partial, err := s.Execute(r)
	if err == nil || !json.Valid(partial) {
		t.Fatalf("partial write was hidden: %s, %v", partial, err)
	}
	if _, err := os.ReadFile(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/partial.md")); err != nil {
		t.Fatal("partial saved Concept was lost", err)
	}
}

func TestSessionMemoryShowOriginalHash(t *testing.T) {
	s, _ := setup(t)
	raw := []byte("---\ntype: Knowledge\ntitle: 'Original quoting'\n---\nExact bytes.\n")
	path := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/knowledge/exact.md")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "show", Target: "knowledge/exact", Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Content, Hash string }
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("show must expose original bytes and hash: %s", out)
	}
	if result.Content != string(raw) || result.Hash != filestore.Hash(raw) {
		t.Fatalf("show changed bytes or hash: %s", out)
	}
}
