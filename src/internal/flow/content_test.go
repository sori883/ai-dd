package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/core"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
)

func TestProcedureBoundaryComposedWorkflow(t *testing.T) {
	got, err := coreworkflow.Render(core.Files, map[string]string{"skill-root": ".agents/skills"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../install/testdata/codex-assets-sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline []struct {
		Path   string
		SHA256 string
	}
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, asset := range baseline {
		if !strings.HasPrefix(asset.Path, "aidlc/workflow/") {
			continue
		}
		count++
		name := strings.TrimPrefix(asset.Path, "aidlc/workflow/")
		hash := sha256.Sum256(got[name])
		if hex.EncodeToString(hash[:]) != asset.SHA256 {
			t.Errorf("completed workflow bytes changed: %s", name)
		}
	}
	if count != 7 {
		t.Fatalf("checked %d assets, want 7", count)
	}
}

func TestProcedureBoundarySharedOperationPropagation(t *testing.T) {
	files := fstest.MapFS{}
	if err := fs.WalkDir(core.Files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(core.Files, name)
		if err == nil {
			files[name] = &fstest.MapFile{Data: data}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	files["skills/shared/workflow-operations.md"] = &fstest.MapFile{Data: []byte("変更した共通工程操作。\n")}
	got, err := coreworkflow.Render(files, map[string]string{"skill-root": ".agents/skills"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for name, data := range got {
		if strings.HasPrefix(name, "stages/") && strings.Contains(string(data), "変更した共通工程操作。") {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("common operation reached %d stages, want 6", count)
	}
	delete(files, "skills/shared/workflow-operations.md")
	if _, err := coreworkflow.Render(files, map[string]string{"skill-root": ".agents/skills"}); err == nil {
		t.Fatal("missing operation accepted")
	}
}
