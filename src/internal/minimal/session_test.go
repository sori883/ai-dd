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
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func setup(t *testing.T) (Service, flow.State) {
	t.Helper()
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	deployProcedureFixture(t, root)
	s := Service{Root: root, Binary: "/opt/aidlc"}
	store := flow.Store{Root: root, Space: "default"}
	saved, err := store.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	saved = executionFixtureState(t, store, saved, "discovery")
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
	if err := os.WriteFile(draft, []byte("First body.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	request := bodyRequest(draft)
	output, err := s.Execute(request)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	_, err = os.ReadFile(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, []byte("Second body.\n"), 0600); err != nil {
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
	body := "First.\n"
	if err := os.WriteFile(draft, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r := bodyRequest(draft)
	output, err := s.Execute(r)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/note.md")
	_, err = os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	dropped := "Second.\n"
	if err := os.WriteFile(draft, []byte(dropped), 0600); err != nil {
		t.Fatal(err)
	}
	r.Action = "update"
	r.Expect = result["hash"]
	r.Metadata = okfmemory.MetadataInput{}
	if _, err := s.Execute(r); err != nil {
		t.Fatal(err)
	}
	doc, err := okfmemory.Read(filepath.Dir(filepath.Dir(savedPath)), "codekb/note")
	if err != nil || doc.String("custom") != "keep" {
		t.Fatal("memory update dropped unknown metadata", err)
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
	index := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/index.md")
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(index, 0755); err != nil {
		t.Fatal(err)
	}
	r.Target = "codekb/partial"
	r.Metadata = bodyRequest(draft).Metadata
	partial, err := s.Execute(r)
	if err == nil || !json.Valid(partial) {
		t.Fatalf("partial write was hidden: %s, %v", partial, err)
	}
	if _, err := os.ReadFile(filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/partial.md")); err != nil {
		t.Fatal("partial saved Concept was lost", err)
	}
}

func TestSessionMemoryShowOriginalHash(t *testing.T) {
	s, _ := setup(t)
	raw := []byte("---\ntype: Knowledge\ntitle: 'Original quoting'\n---\nExact bytes.\n")
	path := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/codekb/exact.md")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := s.Execute(cli.MinimalRequest{Command: "memory", Action: "show", Target: "codekb/exact", Space: "default"})
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

func TestHookBootstrapBinaryIdentity(t *testing.T) {
	s, _ := setup(t)
	real := filepath.Join(t.TempDir(), "aidlc")
	if err := os.WriteFile(real, []byte("binary"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "aidlc")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "aidlc")
	if err := os.WriteFile(other, []byte("binary"), 0700); err != nil {
		t.Fatal(err)
	}
	s.Binary = real
	for _, tc := range []struct {
		name, binary string
		denied       bool
	}{{"same", real, false}, {"alias", alias, false}, {"different", other, true}, {"basename", "aidlc", true}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, args := range []string{" intent list --space default", " intent create Work --space default"} {
				out := hook(t, s, "PreToolUse", "Bash", "tool", "'"+tc.binary+"'"+args, false)
				if deny(out) != tc.denied {
					t.Fatalf("denied=%v want %v: %+v", deny(out), tc.denied, out)
				}
			}
		})
	}
}
