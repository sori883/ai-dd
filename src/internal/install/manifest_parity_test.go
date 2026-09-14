package install

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/workflow"
)

func TestCodexManifestParity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project's root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	binaries := codex.Binaries{AIDLC: "/opt/aidlc's binary", OKF: "/knowledge/okf", Natural: "/words/natural"}
	result, err := CodexFrom(root, binaries, core.Files, codex.Files)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := codex.DistributionFrom(root, binaries, core.Files, codex.Files)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]byte{}
	for _, a := range assets {
		expected[a.Path] = a.Data
	}
	control := filepath.Join(filepath.Dir(root), "mode-control")
	if err := os.WriteFile(control, nil, 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(control)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		actual, err := entry.Info()
		if err != nil {
			return err
		}
		want, ok := expected[relative]
		if !ok || !bytes.Equal(raw, want) {
			t.Errorf("unexpected deployed bytes: %s", relative)
		}
		if !actual.Mode().IsRegular() || actual.Mode().Perm() != info.Mode().Perm() {
			t.Errorf("wrong deployed mode: %s: %v", relative, actual.Mode())
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	if len(paths) != len(expected) || !reflect.DeepEqual(paths, result.Paths) {
		t.Fatalf("saved/returned paths differ: %v", result.Paths)
	}
	definition, err := workflow.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range definition.Procedures["integration"].Outputs {
		if ref.Role == "current_analysis" && ref.Path != "${knowledge_root}/codekb/current-analysis.md" {
			t.Errorf("analysis output: %s", ref.Path)
		}
		if ref.Role == "architecture" && ref.Path != "${knowledge_root}/codekb/architecture.md" {
			t.Errorf("architecture output: %s", ref.Path)
		}
	}
	if !strings.Contains(string(expected["aidlc/templates/adr.md"]), "type: adr") {
		t.Error("wrong ADR type")
	}
	knowledge := filepath.Join(root, "aidlc/spaces/default/knowledge")
	if !strings.Contains(string(expected["aidlc/spaces/default/knowledge/index.md"]), "(codekb/index.md)") {
		t.Error("missing CodeKB link")
	}
	entries, err := os.ReadDir(knowledge)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "ADR" || entry.Name() == "knowledge" {
			t.Errorf("obsolete directory %s", entry.Name())
		}
	}
}

func TestCodexManifestPreflight(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"file", "directory", "symlink", "parent file"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			relative := "aidlc/workflow/stages/tdd.md"
			if kind == "parent file" {
				relative = "aidlc/workflow"
			}
			target := filepath.Join(root, relative)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "directory":
				if err := os.Mkdir(target, 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(t.TempDir(), target); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Codex(root, "/opt/aidlc")
			if err == nil || len(result.Paths) != 0 {
				t.Fatalf("preflight wrote files: %+v, %v", result, err)
			}
			if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
				t.Fatalf("wrote before late collision: %v", err)
			}
			if kind == "file" || kind == "parent file" {
				raw, err := os.ReadFile(target)
				if err != nil || string(raw) != "keep" {
					t.Fatalf("changed existing content: %s, %v", raw, err)
				}
				info, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0600 {
					t.Fatalf("changed existing mode: %v", info.Mode())
				}
			}
		})
	}
	for _, kind := range []string{"late skill", "nested parent link"} {
		t.Run(kind, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			target := filepath.Join(root, ".agents/skills/verification-before-completion/SKILL.md")
			if kind == "nested parent link" {
				target = filepath.Dir(target)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatal(err)
			}
			if kind == "late skill" {
				if err := os.WriteFile(target, []byte("user asset"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(outside, target); err != nil {
				t.Fatal(err)
			}
			result, err := Codex(root, "/opt/aidlc")
			if err == nil || len(result.Paths) != 0 {
				t.Fatalf("preflight %+v %v", result, err)
			}
			for _, absent := range []string{".codex", "aidlc", ".agents/skills/aidlc"} {
				if _, err := os.Lstat(filepath.Join(root, absent)); !os.IsNotExist(err) {
					t.Fatal("partial install", absent, err)
				}
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("wrote outside root", err)
			}
			if kind == "late skill" {
				raw, err := os.ReadFile(target)
				if err != nil || string(raw) != "user asset" {
					t.Fatal("changed collision", err)
				}
			}
		})
	}

}
