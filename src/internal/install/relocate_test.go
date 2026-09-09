package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"os"
	"path/filepath"
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
	root, oldRoot, oldBinary := relocateFixture(t)
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
	result, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(shellQuote(oldBinary) + " __minimal-hook --project-dir " + shellQuote(oldRoot))
	new, _ := json.Marshal(shellQuote("/new/aidlc") + " __minimal-hook --project-dir " + shellQuote(root))
	want := bytes.ReplaceAll(raw, old, new)
	if !bytes.Equal(got, want) || len(result.Paths) != 3 {
		t.Fatalf("references not relocated: paths=%v\n%s", result.Paths, got)
	}
	skill, err := os.ReadFile(filepath.Join(root, ".agents/skills/aidlc/SKILL.md"))
	if err != nil || !strings.Contains(string(skill), shellQuote("/new/aidlc")) {
		t.Fatal("skill not updated", err)
	}
	again, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
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
		{"unknown product", func(b []byte) []byte {
			return bytes.Replace(b, []byte("__minimal-hook"), []byte("__minimal-hook --unknown"), 1)
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
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint("conflict=", conflict), func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			hooks := filepath.Join(root, ".codex/hooks.json")
			original, err := os.ReadFile(hooks)
			if err != nil {
				t.Fatal(err)
			}
			result, err := relocate(root, "/new/aidlc", oldRoot, oldBinary, func(r, p string, b []byte) error {
				if strings.HasSuffix(p, "hooks.json") {
					return errors.New("injected save failure")
				}
				if err := filestore.WriteFile(r, p, b); err != nil {
					return err
				}
				if conflict {
					return os.WriteFile(hooks, append(original, ' '), 0644)
				}
				return nil
			})
			if err == nil || len(result.Paths) != 2 || len(result.Pending) != 1 || result.Pending[0] != ".codex/hooks.json" {
				t.Fatalf("partial result %+v %v", result, err)
			}
			if conflict && !strings.Contains(err.Error(), "concurrent asset change") {
				t.Fatal(err)
			}
			retry, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary)
			if err != nil || len(retry.Paths) != 1 {
				t.Fatalf("retry %+v %v", retry, err)
			}
		})
	}
}
func TestRelocateRejectsSymlinkAndEditedSkill(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint("symlink=", symlink), func(t *testing.T) {
			root, oldRoot, oldBinary := relocateFixture(t)
			p := filepath.Join(root, ".agents/skills/aidlc/SKILL.md")
			before, err := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if err != nil {
				t.Fatal(err)
			}
			if symlink {
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/unavailable/old", p); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(p, []byte("edited"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Relocate(root, "/new/aidlc", oldRoot, oldBinary); err == nil {
				t.Fatal("accepted unknown skill")
			}
			after, _ := os.ReadFile(filepath.Join(root, ".codex/hooks.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("hooks changed")
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
