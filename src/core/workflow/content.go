package workflow

import (
	"io/fs"
	"strings"

	"github.com/sori883/ai-dd/src/core"
)

// Render produces completed workflow files from common sources and host path text.
// Callers must propagate errors before saving any deployment file.
func Render(files fs.FS, host map[string]string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	err := fs.WalkDir(files, "workflow", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := core.RenderContent(files, name, host)
		if err != nil {
			return err
		}
		result[strings.TrimSuffix(strings.TrimPrefix(name, "workflow/"), ".tmpl")] = data
		return nil
	})
	return result, err
}
