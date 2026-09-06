package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/okf"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func knowledgeSearcher(getwd func() (string, error), getenv func(string) string, now func() time.Time) func(okf.SearchOptions, string) (okf.SearchResult, error) {
	return func(options okf.SearchOptions, explicitDir string) (okf.SearchResult, error) {
		workingDir, err := getwd()
		if err != nil {
			return okf.SearchResult{}, fmt.Errorf("read working directory: %w", err)
		}
		projectPath := workspace.ResolveRoot(workspace.RootInput{ExplicitDir: explicitDir, WorkingDir: workingDir, AIDLCProjectDir: getenv("AIDLC_PROJECT_DIR"), ClaudeProjectDir: getenv("CLAUDE_PROJECT_DIR")})
		project, err := os.OpenRoot(projectPath)
		if err != nil {
			return okf.SearchResult{}, fmt.Errorf("open project: %w", err)
		}
		defer project.Close()
		space := workspace.ActiveSpace(project.FS())
		isComponent := fs.ValidPath(space) && space != "." && !strings.ContainsAny(space, "/\\\x00")
		if !isComponent || !utf8.ValidString(space) {
			return okf.SearchResult{}, fmt.Errorf("invalid active space %q", space)
		}
		prefix := path.Join("aidlc", "spaces", space, "knowledge", "okf")
		info, err := project.Lstat(filepath.FromSlash(prefix))
		if err != nil {
			return okf.SearchResult{}, fmt.Errorf("inspect okf root: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return okf.SearchResult{}, fmt.Errorf("okf root is not a non-symlink directory")
		}
		root, err := project.OpenRoot(filepath.FromSlash(prefix))
		if err != nil {
			return okf.SearchResult{}, fmt.Errorf("open okf root: %w", err)
		}
		defer root.Close()
		bundle, err := okf.ScanBundle(root.FS(), prefix)
		if err != nil {
			return okf.SearchResult{}, err
		}
		return okf.Search(bundle, options, now())
	}
}
