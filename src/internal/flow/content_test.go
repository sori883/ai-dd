package flow

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/core"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
)

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
