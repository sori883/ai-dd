// Package install deploys embedded assets into a fresh project without replacement.
package install

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	core "github.com/sori883/ai-dd/src/core/minimal"
	coreworkflow "github.com/sori883/ai-dd/src/core/workflow"
	codex "github.com/sori883/ai-dd/src/harness/codex/minimal"
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
	assets := map[string][]byte{}
	for _, source := range []struct {
		files  fs.FS
		prefix string
	}{{core.Files, ""}, {codex.Files, ""}, {coreworkflow.Files, "aidlc/workflow/"}} {
		err = fs.WalkDir(source.files, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			data, readErr := fs.ReadFile(source.files, path)
			if readErr != nil {
				return readErr
			}
			destination := ""
			switch {
			case source.prefix != "":
				destination = source.prefix + path
			case path == "adr-template.md":
				destination = "aidlc/templates/adr.md"
			case strings.HasPrefix(path, "knowledge/"):
				destination = "aidlc/spaces/default/" + path
			case path == "SKILL.md":
				destination = ".agents/skills/aidlc/" + path
			case path == "aidlc-cli/SKILL.md":
				destination = ".agents/skills/aidlc-cli/SKILL.md"
			case strings.HasPrefix(path, "agents/"):
				destination = ".codex/" + path
			}
			if destination != "" {
				assets[destination] = []byte(strings.ReplaceAll(string(data), "@@BINARY@@", shellQuote(binary)))
			}
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	hooks := map[string]any{}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"} {
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellQuote(binary) + " __minimal-hook --project-dir " + shellQuote(root), "timeout": 10}}}
		if event == "SessionStart" {
			group["hooks"].([]any)[0].(map[string]any)["additionalContextLimit"] = 8192
		}
		if event == "PreToolUse" || event == "PostToolUse" {
			group["matcher"] = assignmentMatcher
		}
		hooks[event] = []any{group}
	}
	assets[".codex/hooks.json"], err = json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		return result, err
	}
	var paths []string
	for path := range assets {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	project, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer project.Close()
	for _, path := range paths {
		if err := checkDestination(project, path); err != nil {
			return result, err
		}
	}
	for _, path := range paths {
		if err := project.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return result, fmt.Errorf("create parent for %s: %w", path, err)
		}
		file, err := project.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return result, fmt.Errorf("create %s: %w", path, err)
		}
		_, writeErr := file.Write(assets[path])
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
