package minimal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/kdr"
)

const testDraft = "---\ntype: KDR\ntitle: Work\ndescription: Test work\n---\n## 目的と完成条件\nProduce tested code.\n## 参照する設計・ルール\nRead rules/rule.\n## 不明点と進め方\nNo questions yet.\n## 判断と結果\nNot implemented: starting.\n## 検証・レビュー\nNot tested: starting.\n## 残件と再開\nWrite tests.\n"

func setup(t *testing.T) (Service, kdr.Saved) {
	t.Helper()
	root := t.TempDir()
	if _, err := install.Codex(root, "/opt/aidlc"); err != nil {
		t.Fatal(err)
	}
	s := Service{Root: root, Binary: "/opt/aidlc"}
	store := kdr.Store{Root: root, Space: "default", Bundle: filepath.Join(root, "aidlc/spaces/default/knowledge")}
	saved, err := store.Create([]byte(testDraft), "process:test")
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
func TestSessionBindAndRecord(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "SessionStart", "", "", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	current, err := s.Inspect("session")
	if err != nil || current.Intent != saved.ID || !current.Dirty || current.RuleHash == "" {
		t.Fatalf("bound = %+v, %v", current, err)
	}
	path := filepath.Join(s.Root, "aidlc/.runtime/drafts/session.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(saved.Raw), "Write tests.", "Tests passed; review remains.", 1)
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "update", Target: saved.ID, Space: "default", Session: "session", File: path, Actor: "process:test", Expect: saved.Hash}); err != nil {
		t.Fatal(err)
	}
	current, err = s.Inspect("session")
	if err != nil || current.Dirty {
		t.Fatalf("recorded = %+v, %v", current, err)
	}
	bind(t, s, saved.ID)
	current, _ = s.Inspect("session")
	if current.Dirty {
		t.Fatal("same bind dirtied recorded turn")
	}
}
func TestHookGuardsAndTerminal(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "SessionStart", "", "", "", false)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if !deny(hook(t, s, "PreToolUse", "Bash", "blocked", "touch code.go", false)) {
		t.Fatal("allowed unbound operation")
	}
	current, _ := s.Inspect("session")
	if current.Tool != "" {
		t.Fatal("denied Pre acquired slot")
	}
	bind(t, s, saved.ID)
	if deny(hook(t, s, "PreToolUse", "Bash", "running", "sleep 3", false)) {
		t.Fatal("denied bound operation")
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "other", "touch other", false)) {
		t.Fatal("allowed overlap")
	}
	hook(t, s, "PostToolUse", "Bash", "wrong", "sleep 3", false)
	current, _ = s.Inspect("session")
	if current.Tool != "running" {
		t.Fatal("unmatched Post released slot")
	}
	hook(t, s, "PostToolUse", "Bash", "running", "sleep 3", false)
	current, _ = s.Inspect("session")
	if current.Tool != "" || !current.Dirty {
		t.Fatalf("terminal = %+v", current)
	}
	if out := hook(t, s, "Stop", "", "", "", false); out["decision"] != "block" {
		t.Fatalf("first Stop = %+v", out)
	}
	if out := hook(t, s, "Stop", "", "", "", true); out["decision"] == "block" || out["systemMessage"] == nil {
		t.Fatalf("reentry = %+v", out)
	}
}
func TestRulesReloadAndExceptions(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	if deny(hook(t, s, "PreToolUse", "Bash", "read", "/opt/aidlc memory rules --space default", false)) {
		t.Fatal("denied diagnostic read")
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "compound", "/opt/aidlc memory rules --space default; touch code", false)) {
		t.Fatal("exempted compound command")
	}
	draft := filepath.Join(s.Root, "aidlc/.runtime/drafts/session.md")
	patch := "*** Begin Patch\n*** Add File: " + draft + "\n+draft\n*** End Patch"
	if deny(hook(t, s, "PreToolUse", "apply_patch", "draft", patch, false)) {
		t.Fatal("denied own draft")
	}
	bind(t, s, saved.ID)
	rule := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	raw, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rule, append(raw, []byte("Changed rule.\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	if !deny(hook(t, s, "PreToolUse", "Bash", "stale", "touch code", false)) {
		t.Fatal("accepted stale Rule")
	}
	bind(t, s, saved.ID)
	if deny(hook(t, s, "PreToolUse", "Bash", "fresh", "touch code", false)) {
		t.Fatal("denied refreshed Rule")
	}
}
func TestSessionReadOnlyInspect(t *testing.T) {
	s, _ := setup(t)
	state, err := s.Inspect("new")
	if err != nil || !state.Dirty {
		t.Fatalf("missing session = %+v, %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "aidlc/.runtime/sessions/new.txt")); !os.IsNotExist(err) {
		t.Fatalf("inspect created state: %v", err)
	}
	if _, err := s.Inspect("../bad"); err == nil {
		t.Fatal("accepted path traversal")
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
func TestSessionJSONOutput(t *testing.T) {
	s, saved := setup(t)
	out, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "show", Space: "default", Target: saved.ID})
	if err != nil || !json.Valid(out) {
		t.Fatalf("show = %s, %v", out, err)
	}
}

func TestRulesUpdateRequiresFreshRead(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	rule := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	raw, err := os.ReadFile(rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rule, append(raw, []byte("New requirement.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	draft := s.draftPath("session")
	if err := os.MkdirAll(filepath.Dir(draft), 0700); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(saved.Raw), "Write tests.", "Tests passed.", 1)
	if err := os.WriteFile(draft, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "update", Space: "default", Target: saved.ID, Session: "session", File: draft, Actor: "process:test", Expect: saved.Hash}); err == nil {
		t.Fatal("recorded despite changed unread Rule")
	}
}
func TestHookRejectsCanonicalPatch(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	patch := "*** Begin Patch\n*** Update File: " + saved.Path + "\n@@\n-old\n+new\n*** End Patch"
	if !deny(hook(t, s, "PreToolUse", "apply_patch", "direct", patch, false)) {
		t.Fatal("allowed direct KDR patch instead of CLI update")
	}
}
func TestSessionDirtySwitchAndRecovery(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	other, err := s.store("default").Create([]byte(strings.Replace(testDraft, "title: Work", "title: Other", 1)), "process:test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Space: "default", Target: other.ID, Session: "session"}); err == nil {
		t.Fatal("switched dirty Intent")
	}
	hook(t, s, "PreToolUse", "Bash", "running", "sleep 3", false)
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Space: "default", Target: saved.ID, Session: "session"}); err == nil {
		t.Fatal("bound while tool running")
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Space: "default", Target: saved.ID, Session: "session", Recover: true}); err != nil {
		t.Fatal(err)
	}
	state, err := s.Inspect("session")
	if err != nil || state.Tool != "" || !state.Dirty {
		t.Fatalf("recovery = %+v, %v", state, err)
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
	before, err := os.ReadFile(filepath.Join(s.Root, "aidlc/.runtime/sessions/session.txt"))
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
	after, err := os.ReadFile(filepath.Join(s.Root, "aidlc/.runtime/sessions/session.txt"))
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
	r.Target = "kdr/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := s.Execute(r); err == nil {
		t.Fatal("generic memory write accepted KDR directory")
	}
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

func TestSessionShowReadableKDR(t *testing.T) {
	s, saved := setup(t)
	raw, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "show", Target: saved.ID, Space: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Content string `json:"content"`
		Hash    string
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output.Content != string(saved.Raw) || output.Hash != saved.Hash {
		t.Fatalf("readable KDR missing: %s", raw)
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
	if result.Content != string(raw) || result.Hash != kdr.Hash(raw) {
		t.Fatalf("show changed bytes or hash: %s", out)
	}
}

func TestSessionRejectsCommentOnlyBookkeeping(t *testing.T) {
	for _, name := range []string{"index", "log"} {
		t.Run(name, func(t *testing.T) {
			s, saved := setup(t)
			store := s.store("default")
			path, raw := "kdr/index.md", "<!-- ("+saved.ID+".md) -->\n"
			if name == "log" {
				path = "log.md"
				raw = "<!-- `kdr/" + saved.ID + "` -->\n"
			}
			if err := os.WriteFile(filepath.Join(store.Bundle, path), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"check", "bind"} {
				command := "kdr"
				if action == "bind" {
					command = "session"
				}
				if _, err := s.Execute(cli.MinimalRequest{Command: command, Action: action, Target: saved.ID, Space: "default", Session: "session"}); err == nil {
					t.Errorf("%s accepted comment-only %s", action, name)
				}
			}
			if _, err := store.Repair(saved.ID, saved.Raw, saved.Hash, "process:test"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "check", Target: saved.ID, Space: "default"}); err != nil {
				t.Fatal(err)
			}
			bind(t, s, saved.ID)
		})
	}
}

func TestSessionBookkeepingActiveEntries(t *testing.T) {
	for _, name := range []string{"undated", "invalid_date", "unknown_action", "fenced", "open_comment"} {
		t.Run(name, func(t *testing.T) {
			s, saved := setup(t)
			store := s.store("default")
			if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "check", Target: saved.ID, Space: "default"}); err != nil {
				t.Fatalf("valid initial bookkeeping rejected: %v", err)
			}
			item := "- Update: `kdr/" + saved.ID + "`.\n"
			path, raw := "log.md", item
			switch name {
			case "invalid_date":
				raw = "## 2026-02-30\n" + item
			case "unknown_action":
				raw = "## 2026-09-08\n- Something: `kdr/" + saved.ID + "`.\n"
			case "fenced":
				raw = "```md\n## 2026-09-08\n" + item + "```\n"
			case "open_comment":
				path = "kdr/index.md"
				raw = "<!--\n- [Work](" + saved.ID + ".md): description\n"
			}
			if err := os.WriteFile(filepath.Join(store.Bundle, path), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "check", Target: saved.ID, Space: "default"}); err == nil {
				t.Fatal("accepted inactive/invalid bookkeeping")
			}
			if _, err := store.Repair(saved.ID, saved.Raw, saved.Hash, "process:test"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "check", Target: saved.ID, Space: "default"}); err != nil {
				t.Fatalf("same-ID repair did not restore bookkeeping: %v", err)
			}
		})
	}
}

func TestHookEditFailureRecovery(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	hook(t, s, "PreToolUse", "apply_patch", "failed-edit", "*** Begin Patch\n*** Update File: absent.go\n@@\n-no matching line\n+replacement\n*** End Patch", false)
	current, _ := s.Inspect("session")
	if current.Tool != "failed-edit" {
		t.Fatal("failed edit did not retain slot without Post")
	}
	base := "/opt/aidlc session bind " + saved.ID + " --space default --session session"
	if out := hook(t, s, "PreToolUse", "Bash", "recover", base+" --recover", false); deny(out) {
		t.Fatalf("same-session explicit recover blocked: %v", out)
	}
	for _, command := range []string{base, strings.Replace(base, "session session", "session other", 1) + " --recover", strings.Replace(base, "space default", "space other", 1) + " --recover", strings.Replace(base, saved.ID, strings.Repeat("a", 32), 1) + " --recover", base + " --recover; true", "echo unsafe"} {
		if out := hook(t, s, "PreToolUse", "Bash", "other", command, false); !deny(out) {
			t.Errorf("accepted while slot retained: %s", command)
		}
	}
	oldHash := current.RuleHash
	rulePath := filepath.Join(s.Root, "aidlc/spaces/default/knowledge/rules/rule.md")
	rule, err := os.ReadFile(rulePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulePath, append(rule, []byte("\nNew required verification note.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "session", Action: "bind", Target: saved.ID, Space: "default", Session: "session", Recover: true}); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Inspect("session")
	if current.Tool != "" || !current.Dirty || current.RuleHash == "" || current.RuleHash == oldHash || current.Intent != saved.ID {
		t.Fatalf("bad recovery: %+v", current)
	}
	hook(t, s, "PreToolUse", "Bash", "retry", "echo verified", false)
	hook(t, s, "PostToolUse", "Bash", "retry", "", false)
	raw := strings.Replace(string(saved.Raw), "Write tests.", "Edit failed, explicitly recovered, retry verified.", 1)
	file := filepath.Join(s.Root, "aidlc/.runtime/drafts/session.md")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(cli.MinimalRequest{Command: "kdr", Action: "update", Target: saved.ID, Space: "default", Session: "session", File: file, Actor: "process:test", Expect: saved.Hash}); err != nil {
		t.Fatal(err)
	}
	current, _ = s.Inspect("session")
	if current.Dirty {
		t.Fatal("successful recording remains dirty")
	}
}

func TestHookRecoveryDiagnostic(t *testing.T) {
	s, saved := setup(t)
	hook(t, s, "UserPromptSubmit", "", "", "", false)
	bind(t, s, saved.ID)
	hook(t, s, "PreToolUse", "Bash", "retained", "echo work", false)
	out := hook(t, s, "PreToolUse", "Bash", "blocked", "echo retry", false)
	raw, _ := json.Marshal(out)
	for _, want := range []string{"session bind", saved.ID, "--space", "default", "--session", "--recover"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("diagnostic lacks %s: %s", want, raw)
		}
	}
	stop := hook(t, s, "Stop", "", "", "", false)
	raw, _ = json.Marshal(stop)
	if !strings.Contains(string(raw), "unrecorded=true") {
		t.Fatalf("Stop omits current state: %s", raw)
	}
}
