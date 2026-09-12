// Package install deploys embedded assets into a fresh project without replacement.
package install

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	codex "github.com/sori883/ai-dd/src/harness/codex"
)

// Result names each file saved before success or a partial failure.
type Result struct{ Paths []string }

// Codex installs initial assets. Existing target files are never replaced.
func Codex(root, binary string) (result Result, err error) {
	if !filepath.IsAbs(binary) {
		return result, fmt.Errorf("binary must be absolute: %w", fs.ErrInvalid)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return result, err
	}
	assets, err := codex.Distribution(root, binary)
	if err != nil {
		return result, err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer project.Close()
	for _, asset := range assets {
		path := asset.Path
		if err := checkDestination(project, path); err != nil {
			return result, err
		}
	}
	for _, asset := range assets {
		path := asset.Path
		if err := project.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return result, fmt.Errorf("create parent for %s: %w", path, err)
		}
		file, err := project.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return result, fmt.Errorf("create %s: %w", path, err)
		}
		_, writeErr := file.Write(asset.Data)
		closeErr := file.Close()
		result.Paths = append(result.Paths, path)
		if writeErr != nil {
			return result, fmt.Errorf("save %s: %w", path, writeErr)
		}
		if closeErr != nil {
			return result, fmt.Errorf("close %s: %w", path, closeErr)
		}
	}
	return result, nil
}

func checkDestination(root *os.Root, path string) error {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := range parts {
		prefix := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(prefix)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || i == len(parts)-1 {
			return fmt.Errorf("existing destination %s: %w", prefix, fs.ErrExist)
		}
		if !info.IsDir() {
			return fmt.Errorf("invalid parent %s: %w", prefix, fs.ErrInvalid)
		}
	}
	return nil
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

const assignmentMatcher = "^(Bash|apply_patch|spawn_agent|collaborationspawn_agent|followup_task|collaborationfollowup_task|send_message|collaborationsend_message|interrupt_agent|collaborationinterrupt_agent)$"
