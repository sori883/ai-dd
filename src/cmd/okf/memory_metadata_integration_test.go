//go:build integration

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func TestMemoryMetadataCommand(t *testing.T) {
	f := newMemoryFixture(t)
	root := f.root
	nested := filepath.Join(root, "drafts")
	writeMemoryFixture(t, filepath.Join(nested, "body.md"), "# First body\n")
	for _, tc := range []struct{ id, kind string }{{"codekb/example", "Design"}, {"rules/example", "Rule"}} {
		args := []string{"create", tc.id, "--space", "default", "--project-dir", root, "--body-file", "body.md", "--actor", "process:codex", "--type", tc.kind, "--title", "Example", "--description", "Current behavior", "--tag", "lookup", "--metadata-json", `{"extension":{"keep":true}}`}
		if tc.kind != "Rule" {
			args = append(args, "--intent-id", strings.Repeat("a", 32))
		}
		f.okAt(nested, args...)
		show := f.ok("show", tc.id, "--space", "default")
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
		if tc.kind == "Rule" {
			continue
		}
		writeMemoryFixture(t, filepath.Join(nested, "body.md"), "# Second body\n")
		f.okAt(nested, "update", tc.id, "--space", "default", "--project-dir", root, "--body-file", "body.md", "--actor", "human:editor", "--expect", before.Hash)
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
		writeMemoryFixture(t, filepath.Join(nested, "body.md"), "# Third body\n")
		f.rejectCode(2, "Concept hash conflict", "update", tc.id, "--space", "default", "--body-file", filepath.Join(nested, "body.md"), "--actor", "human:editor", "--expect", before.Hash)
		if !bytes.Equal(raw, memoryRead(t, filepath.Join(root, "aidlc/spaces/default/knowledge", tc.id+".md"))) {
			t.Fatal("stale update changed saved Concept")
		}
		writeMemoryFixture(t, filepath.Join(nested, "body.md"), "# First body\n")
	}
	out := f.ok("search", "lookup", "--space", "default", "--intent-id", strings.Repeat("a", 32))
	var found []map[string]string
	if err := json.Unmarshal(out, &found); err != nil || len(found) != 1 {
		t.Fatalf("search %s %v", out, err)
	}
	f.ok("check", "--space", "default")
}

func TestMemorySaveRecovery(t *testing.T) {
	f := newMemoryFixture(t)
	body := filepath.Join(f.root, "body.md")
	writeMemoryFixture(t, body, "Before\n")
	create := []string{"create", "codekb/save", "--space", "default", "--body-file", body, "--actor", "process:test", "--type", "Design", "--title", "Save", "--description", "Recovery"}
	f.ok(create...)
	concept := filepath.Join(f.root, "aidlc/spaces/default/knowledge/codekb/save.md")
	original := memoryRead(t, concept)
	hash := sha256.Sum256(original)
	update := []string{"update", "codekb/save", "--space", "default", "--body-file", body, "--actor", "process:test", "--expect", fmt.Sprintf("%x", hash)}
	writeMemoryFixture(t, body, "After\n")
	restore := memoryReadOnly(t, filepath.Dir(concept))
	f.rejectCode(1, "permission denied", update...)
	if !bytes.Equal(original, memoryRead(t, concept)) {
		t.Fatal("Knowledge pre-save failure changed document")
	}
	restore()
	f.ok("show", "codekb/save", "--space", "default")
	f.ok(update...)
	// A real directory collision makes bookkeeping fail after the Concept commit.
	index := filepath.Join(filepath.Dir(concept), "index.md")
	indexBytes := memoryRead(t, index)
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(index, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(index); os.WriteFile(index, indexBytes, 0644) })
	create[1] = "codekb/partial"
	result := f.run(create...)
	if result.code != 2 || !json.Valid(result.out) || !strings.Contains(string(result.stderr), "Concept saved; bookkeeping failed: not regular") {
		t.Fatalf("partial save hidden: %+v", result)
	}
	partial := filepath.Join(filepath.Dir(concept), "partial.md")
	saved := memoryRead(t, partial)
	var response struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(result.out, &response); err != nil {
		t.Fatal(err)
	}
	if response.Hash != fmt.Sprintf("%x", sha256.Sum256(saved)) {
		t.Fatal("partial saved hash missing")
	}
	f.ok("show", "codekb/partial", "--space", "default")
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, indexBytes, 0644); err != nil {
		t.Fatal(err)
	}
	f.ok("update", "codekb/partial", "--space", "default", "--body-file", body, "--actor", "process:test", "--expect", response.Hash, "--description", "Recovered bookkeeping")
	f.ok("check", "--space", "default")
	if !bytes.Contains(memoryRead(t, index), []byte("partial.md")) {
		t.Fatal("index not recovered")
	}
}

type memoryFixture struct {
	t            *testing.T
	root, binary string
}
type memoryResult struct {
	out, stderr []byte
	code        int
}

func newMemoryFixture(t *testing.T) memoryFixture {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "okf")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if out, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "aidlc/spaces/default/knowledge"), 0700); err != nil {
		t.Fatal(err)
	}
	return memoryFixture{t, root, binary}
}
func (f memoryFixture) runAt(cwd string, args ...string) memoryResult {
	f.t.Helper()
	// Every invocation executes the public okf binary directly.
	hasRoot := false
	for _, a := range args {
		if a == "--project-dir" {
			hasRoot = true
		}
	}
	if !hasRoot {
		args = append(args, "--project-dir", f.root)
	}
	cmd := exec.CommandContext(f.t.Context(), f.binary, args...)
	cmd.Dir = cwd
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	err := cmd.Run()
	code := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			f.t.Fatal(err)
		}
		code = e.ExitCode()
	}
	return memoryResult{out.Bytes(), errout.Bytes(), code}
}
func (f memoryFixture) run(args ...string) memoryResult { return f.runAt(f.root, args...) }
func (f memoryFixture) okAt(cwd string, args ...string) []byte {
	f.t.Helper()
	r := f.runAt(cwd, args...)
	if r.code != 0 || len(r.stderr) != 0 {
		f.t.Fatalf("%v: %+v", args, r)
	}
	return r.out
}
func (f memoryFixture) ok(args ...string) []byte { f.t.Helper(); return f.okAt(f.root, args...) }
func (f memoryFixture) rejectCode(code int, message string, args ...string) {
	f.t.Helper()
	r := f.run(args...)
	if r.code != code || len(r.out) != 0 || !strings.Contains(string(r.stderr), message) {
		f.t.Fatalf("%v: %+v", args, r)
	}
}
func writeMemoryFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func memoryRead(t *testing.T, p string) []byte {
	t.Helper()
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func memoryReadOnly(t *testing.T, p string) func() {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if err := os.Chmod(p, info.Mode().Perm()); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(restore)
	if err := os.Chmod(p, 0500); err != nil {
		t.Fatal(err)
	}
	return restore
}
