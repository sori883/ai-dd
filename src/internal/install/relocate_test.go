package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func relocateFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Codex(root, "/missing/old aidlc"); err != nil {
		t.Fatal(err)
	}
	return root, root, "/missing/old aidlc"
}
func TestRelocateReferences(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	old := codex.Binaries{AIDLC: "/missing/old aidlc", OKF: "/knowledge/old okf", Natural: "/words/old natural"}
	updated := codex.Binaries{AIDLC: "/new/aidlc", OKF: "/new-knowledge/okf", Natural: "/new-words/natural"}
	if _, err := CodexFrom(root, old, core.Files, codex.Files); err != nil {
		t.Fatal(err)
	}
	oldRoot := root
	p := filepath.Join(root, ".codex/hooks.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte(`"hooks": {`), []byte(`"custom" : {"x": 1}, "hooks": {"CustomEvent": [{"hooks":[{"type":"command","command":"echo custom"}]}],`), 1)
	if err := os.WriteFile(p, raw, 0644); err != nil {
		t.Fatal(err)
	}
	// Supply a nonexistent old root as a string by adapting only the known installed references.
	oldRoot = "/does/not/exist/source"
	raw = bytes.ReplaceAll(raw, []byte(shellQuote(root)), []byte(shellQuote(oldRoot)))
	if err := os.WriteFile(p, raw, 0644); err != nil {
		t.Fatal(err)
	}
	result, err := RelocateFrom(root, oldRoot, updated, old, core.Files, codex.Files)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	oldCommand, _ := json.Marshal(shellQuote(old.AIDLC) + " __hook --project-dir " + shellQuote(oldRoot) + " --okf-binary " + shellQuote(old.OKF))
	newCommand, _ := json.Marshal(shellQuote(updated.AIDLC) + " __hook --project-dir " + shellQuote(root) + " --okf-binary " + shellQuote(updated.OKF))
	want := bytes.ReplaceAll(raw, oldCommand, newCommand)
	if !bytes.Equal(got, want) || len(result.Paths) != 6 {
		t.Fatalf("references not relocated: paths=%v\n%s", result.Paths, got)
	}
	skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil || !strings.Contains(string(skill), shellQuote("/new/aidlc")) {
		t.Fatal("skill not updated", err)
	}
	for name, binary := range map[string]string{"aidlc": updated.AIDLC, "okf-agent-memory": updated.OKF, "natural-japanese-go": updated.Natural} {
		body, err := os.ReadFile(filepath.Join(root, ".agents/skills", name, "SKILL.md"))
		if err != nil || !bytes.Contains(body, []byte(shellQuote(binary))) {
			t.Fatalf("unmoved %s: %s %v", name, body, err)
		}
	}
	again, err := RelocateFrom(root, oldRoot, updated, old, core.Files, codex.Files)
	if err != nil || len(again.Paths) != 0 {
		t.Fatalf("retry %+v %v", again, err)
	}
}
func TestRelocateRejectsBeforeSaving(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"duplicate", func(b []byte) []byte { return append([]byte(`{"hooks":{},`), b[1:]...) }},
		{"invalid utf8", func(b []byte) []byte { return append(b, 255) }},
		{"trailing", func(b []byte) []byte { return append(b, []byte(` {}`)...) }},
		{"wrong timeout", func(b []byte) []byte { return bytes.Replace(b, []byte(`"timeout": 10`), []byte(`"timeout": 1`), 1) }},
		{"extra unknown product", func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"hooks": {`), []byte(`"hooks": {"CustomEvent": [{"hooks":[{"type":"command","command":"aidlc __hook --unknown"}]}],`), 1)
		}},
		{"unknown product", func(b []byte) []byte {
			return bytes.Replace(b, []byte("__hook"), []byte("__hook --unknown"), 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			p := filepath.Join(root, ".codex/hooks.json")
			raw, _ := os.ReadFile(p)
			raw = tc.mutate(raw)
			os.WriteFile(p, raw, 0644)
			skillPath := filepath.Join(root, ".agents/skills/aidlc/SKILL.md")
			before, _ := os.ReadFile(skillPath)
			if _, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary); err == nil {
				t.Fatal("accepted invalid hooks")
			}
			after, _ := os.ReadFile(skillPath)
			if !bytes.Equal(before, after) {
				t.Fatal("saved before validating all files")
			}
		})
	}
}

func TestRelocatePartialAndConcurrentRetry(t *testing.T) {
	paths := []string{".agents/skills/aidlc/SKILL.md", ".agents/skills/aidlc-cli/SKILL.md", ".agents/skills/okf-agent-memory/SKILL.md", ".agents/skills/natural-japanese-go/SKILL.md", ".agents/skills/natural-japanese-go/references/cli.md", ".codex/hooks.json"}
	for _, tc := range []struct {
		name, fail string
		conflict   bool
	}{
		{name: "early skill", fail: paths[1]}, {name: "hooks", fail: paths[5]}, {name: "concurrent hooks", fail: paths[5], conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			before := map[string][]byte{}
			for _, path := range paths {
				raw, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				before[path] = raw
			}
			user := filepath.Join(root, "user.md")
			if err := os.WriteFile(user, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := relocate(root, "/new/aidlc", oldRoot, oldBinary, func(r, path string, data []byte) error {
				if path == tc.fail {
					return errors.New("injected save failure")
				}
				if err := filestore.WriteFile(r, path, data); err != nil {
					return err
				}
				if tc.conflict {
					return os.WriteFile(filepath.Join(root, paths[5]), append(append([]byte{}, before[paths[5]]...), ' '), 0644)
				}
				return nil
			})
			if err == nil {
				t.Fatal("partial save succeeded")
			}
			if tc.conflict && !strings.Contains(err.Error(), "concurrent asset change") {
				t.Fatal(err)
			}
			saved, pending := []string{}, []string{}
			failed := false
			for _, path := range paths {
				failed = failed || path == tc.fail
				raw, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				if failed {
					pending = append(pending, path)
					want := before[path]
					if tc.conflict && path == paths[5] {
						want = append(append([]byte{}, want...), ' ')
					}
					if !bytes.Equal(raw, want) {
						t.Errorf("pending file changed: %s", path)
					}
				} else {
					saved = append(saved, path)
					if bytes.Equal(raw, before[path]) {
						t.Errorf("saved file unchanged: %s", path)
					}
				}
			}
			if !reflect.DeepEqual(result.Paths, saved) || !reflect.DeepEqual(result.Pending, pending) {
				t.Fatalf("partial result %+v want %v / %v", result, saved, pending)
			}
			retry, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
			if err != nil || !reflect.DeepEqual(retry.Paths, pending) || len(retry.Pending) != 0 {
				t.Fatalf("retry %+v %v", retry, err)
			}
			again, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
			if err != nil || len(again.Paths) != 0 || len(again.Pending) != 0 {
				t.Fatalf("again %+v %v", again, err)
			}
			raw, err := os.ReadFile(user)
			if err != nil || string(raw) != "keep" {
				t.Fatal("changed user file", err)
			}
		})
	}
}

func TestRelocateRejectsSymlinkAndEditedSkill(t *testing.T) {
	for _, mode := range []string{"edited", "missing", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			path := filepath.Join(root, ".agents/skills/natural-japanese-go/references/cli.md")
			before, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "edited" {
				err = os.WriteFile(path, []byte("edited"), 0644)
			} else {
				err = os.Remove(path)
				if err == nil && mode == "symlink" {
					err = os.Symlink("/unavailable/old", path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
			if err == nil || len(result.Paths) != 0 {
				t.Fatalf("accepted unknown skill: %+v %v", result, err)
			}
			after, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("hooks changed", err)
			}
			if mode == "edited" {
				raw, err := os.ReadFile(path)
				if err != nil || string(raw) != "edited" {
					t.Fatal("user edit changed", err)
				}
			}
		})
	}
}

func TestRelocateSameReferencesAndLock(t *testing.T) {
	root, oldRoot, oldBinary := relocateFixture(t)
	same, err := Relocate(root, oldBinary, oldRoot, oldBinary)
	if err != nil || len(same.Paths) != 0 || len(same.Pending) != 0 {
		t.Fatalf("same references %+v %v", same, err)
	}
	release, err := filestore.Lock(root, "install-relocate")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if _, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary); err == nil {
		t.Fatal("ignored relocation lock")
	}
	after, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("changed locked asset")
	}
}
