//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

func relocationSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = filestore.Hash(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func relocationBinary(t *testing.T, old string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "aidlc")
	if err := os.WriteFile(p, operationsRead(t, old), 0755); err != nil {
		t.Fatal(err)
	}
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestRelocationCommand(t *testing.T) {
	f := operationsNew(t)
	canonicalBinary, canonicalErr := filepath.EvalSymlinks(f.binary)
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	f.binary = canonicalBinary
	writeAIDLCFixture(t, filepath.Join(f.root, "user-note.txt"), "keep user bytes\n")
	hookPath := filepath.Join(f.root, ".codex/hooks.json")
	hooks := operationsRead(t, hookPath)
	hooks = bytes.Replace(hooks, []byte(`"hooks": {`), []byte(`"custom": {"keep": true}, "hooks": {"CustomEvent":[{"hooks":[{"type":"command","command":"echo unchanged"}]}],`), 1)
	if err := os.WriteFile(hookPath, hooks, 0644); err != nil {
		t.Fatal(err)
	}
	original := relocationSnapshot(t, f.root)
	clone := filepath.Join(t.TempDir(), "clone")
	if err := os.CopyFS(clone, os.DirFS(f.root)); err != nil {
		t.Fatal(err)
	}
	clone, err := filepath.EvalSymlinks(clone)
	if err != nil {
		t.Fatal(err)
	}
	g := operationsFixture{t, relocationBinary(t, f.binary), clone}
	before := relocationSnapshot(t, clone)
	g.ok("install", "codex", "--relocate", "--project-dir", clone, "--from-project-dir", f.root, "--from-binary", f.binary)
	after := relocationSnapshot(t, clone)
	const runtimeIgnore = "aidlc/.runtime/.gitignore"
	if _, existed := before[runtimeIgnore]; !existed {
		got := operationsRead(t, filepath.Join(clone, runtimeIgnore))
		if string(got) != "*\n" {
			t.Fatalf("unexpected runtime ignore bytes: %q", got)
		}
		before[runtimeIgnore] = filestore.Hash([]byte("*\n"))
	}
	for p, hash := range before {
		if p == ".codex/hooks.json" || p == ".agents/skills/aidlc/SKILL.md" || p == ".agents/skills/aidlc-cli/SKILL.md" || p == ".agents/skills/okf-agent-memory/SKILL.md" {
			continue
		}
		if after[p] != hash {
			t.Fatalf("changed unrelated asset %s", p)
		}
	}
	if len(after) != len(before) {
		t.Fatal("relocation added or removed assets")
	}
	changedHooks := operationsRead(t, filepath.Join(clone, ".codex/hooks.json"))
	if !bytes.Contains(changedHooks, []byte(`"custom": {"keep": true}`)) || bytes.Contains(changedHooks, []byte(f.root)) {
		t.Fatal("old refs or custom bytes corrupted")
	}
	// Execute the actual relocated handler command; it must initialize only clone runtime.
	var config struct {
		Hooks map[string][]struct{ Hooks []struct{ Command string } }
	}
	if err := json.Unmarshal(changedHooks, &config); err != nil {
		t.Fatal(err)
	}
	args, ok := flowShellWords(config.Hooks["UserPromptSubmit"][0].Hooks[0].Command)
	if !ok || args[0] != g.binary {
		t.Fatal("bad relocated command")
	}
	runAIDLCCLI(t, args[0], clone, []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"new-coordinator","turn_id":"turn"}`), args[1:]...)
	if g.session("new-coordinator").Turn != "turn" {
		t.Fatal("relocated hook did not initialize destination runtime")
	}
	if !reflect.DeepEqual(original, relocationSnapshot(t, f.root)) {
		t.Fatal("source project changed")
	}
}
