package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type deployedAsset struct {
	Path   string      `json:"path"`
	SHA256 string      `json:"sha256"`
	Mode   fs.FileMode `json:"mode"`
}

// The fixture was captured from the installer at f8d9eb0d83143144bbcb2fc5b6a9dc5db80acf94,
// before introducing Manifest.Render. Only the temporary root in hooks is normalized.
func TestCodexManifestParity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project's root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	result, err := Codex(root, "/opt/aidlc's binary")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	// Replace only the encoded root token; preserve hook JSON formatting and all other bytes.
	rootToken, err := json.Marshal(shellQuote(root))
	if err != nil {
		t.Fatal(err)
	}
	fixedToken, err := json.Marshal(shellQuote("/fixed/project"))
	if err != nil {
		t.Fatal(err)
	}
	// A control file observes the caller's umask without changing process-wide state.
	controlPath := filepath.Join(filepath.Dir(root), "mode-control")
	if err := os.WriteFile(controlPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	controlInfo, err := os.Stat(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []deployedAsset
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		path, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if filepath.ToSlash(path) == ".codex/hooks.json" {
			raw = bytes.ReplaceAll(raw, rootToken[1:len(rootToken)-1], fixedToken[1:len(fixedToken)-1])
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			t.Errorf("deployed asset %q is not regular: %v", path, info.Mode())
		}
		sum := sha256.Sum256(raw)
		got = append(got, deployedAsset{filepath.ToSlash(path), hex.EncodeToString(sum[:]), info.Mode()})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(a, b deployedAsset) int { return strings.Compare(a.Path, b.Path) })
	var paths []string
	for _, asset := range got {
		paths = append(paths, asset.Path)
	}
	if !reflect.DeepEqual(paths, result.Paths) {
		t.Fatalf("returned paths differ from saved files: %v", result.Paths)
	}
	raw, err := os.ReadFile("testdata/codex-assets-sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []deployedAsset
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for i := range want {
		want[i].Mode = controlInfo.Mode().Perm()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Codex deployment changed\ngot: %+v\nwant: %+v", got, want)
	}
	t.Logf("verified %d deployed files, bytes and regular modes", len(got))
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
}
