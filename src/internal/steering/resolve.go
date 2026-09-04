package steering

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/graph"
)

// RuleSource identifies the filesystem root used by a resolved rule path.
type RuleSource uint8

const (
	// RuleSourceUnknown is the zero value for an unresolved rule source.
	RuleSourceUnknown RuleSource = iota
	// RuleSourceProject identifies a path read from the project filesystem.
	RuleSourceProject
	// RuleSourceMemory identifies a path read from the active Space's Memory filesystem.
	RuleSourceMemory
)

// RulePath pairs a display path with the filesystem-relative path to read.
type RulePath struct {
	Path     string
	ReadPath string
	Source   RuleSource
}

// RulePaths contains the active Memory directory and ordered resolved rules.
type RulePaths struct {
	MemoryDir string
	Entries   []RulePath
}

// ResolveRulePaths maps graph rule references to active-Space display paths
// and their project or Memory filesystem roots.
func ResolveRulePaths(projectDir, activeSpace, rulesDir string, refs []graph.Rule) (RulePaths, error) {
	if err := validateResolveInputs(projectDir, activeSpace, rulesDir, refs); err != nil {
		return RulePaths{}, err
	}

	memoryDir := filepath.Join(projectDir, "aidlc", "spaces", activeSpace, "memory")
	if rulesDir != "" {
		if filepath.IsAbs(rulesDir) {
			memoryDir = rulesDir
		} else {
			memoryDir = filepath.Join(projectDir, rulesDir)
		}
	}

	entries := make([]RulePath, 0, len(refs))
	for index, ref := range refs {
		marker := "/memory/"
		if markerIndex := strings.Index(ref.Path, marker); markerIndex >= 0 {
			subpath := ref.Path[markerIndex+len(marker):]
			entry := RulePath{
				Path:     filepath.ToSlash(filepath.Join("aidlc", "spaces", activeSpace, "memory", filepath.FromSlash(subpath))),
				ReadPath: subpath,
				Source:   RuleSourceMemory,
			}
			if err := validateRulePathEntry(entry); err != nil {
				return RulePaths{}, fmt.Errorf("invalid generated rule at index %d: %w", index, err)
			}
			entries = append(entries, entry)
			continue
		}
		entry := RulePath{
			Path:     ref.Path,
			ReadPath: ref.Path,
			Source:   RuleSourceProject,
		}
		if err := validateRulePathEntry(entry); err != nil {
			return RulePaths{}, fmt.Errorf("invalid generated rule at index %d: %w", index, err)
		}
		entries = append(entries, entry)
	}
	return RulePaths{MemoryDir: memoryDir, Entries: entries}, nil
}

func validateResolveInputs(projectDir, activeSpace, rulesDir string, refs []graph.Rule) error {
	if err := validateProjectDir(projectDir); err != nil {
		return fmt.Errorf("invalid project directory: %w", err)
	}
	if err := validateActiveSpace(activeSpace); err != nil {
		return fmt.Errorf("invalid active Space %q: %w", activeSpace, err)
	}
	if err := validateRulesDir(rulesDir); err != nil {
		return fmt.Errorf("invalid rules directory %q: %w", rulesDir, err)
	}
	for index, ref := range refs {
		if err := validateReferencePath(ref.Path); err != nil {
			return fmt.Errorf("invalid rule reference at index %d: %w", index, err)
		}
	}
	return nil
}

func validateProjectDir(projectDir string) error {
	if projectDir == "" || !utf8.ValidString(projectDir) || strings.IndexByte(projectDir, 0) >= 0 {
		return fmt.Errorf("project directory must be a valid absolute path: %w", fs.ErrInvalid)
	}
	if !filepath.IsAbs(projectDir) {
		return fmt.Errorf("project directory must be a valid absolute path: %w", fs.ErrInvalid)
	}
	return nil
}

func validateActiveSpace(activeSpace string) error {
	if activeSpace == "" || !utf8.ValidString(activeSpace) || strings.IndexByte(activeSpace, 0) >= 0 {
		return fmt.Errorf("active Space must be one path component: %w", fs.ErrInvalid)
	}
	if strings.ContainsAny(activeSpace, `/\\`) || activeSpace == "." || activeSpace == ".." {
		return fmt.Errorf("active Space must be one path component: %w", fs.ErrInvalid)
	}
	if !filepath.IsLocal(filepath.FromSlash(activeSpace)) {
		return fmt.Errorf("active Space must be local: %w", fs.ErrInvalid)
	}
	return nil
}

func validateRulesDir(rulesDir string) error {
	if rulesDir == "" {
		return nil
	}
	if !utf8.ValidString(rulesDir) || strings.IndexByte(rulesDir, 0) >= 0 {
		return fmt.Errorf("rules directory is not safe: %w", fs.ErrInvalid)
	}
	native := filepath.FromSlash(rulesDir)
	if !filepath.IsAbs(native) {
		if filepath.VolumeName(native) != "" {
			return fmt.Errorf("rules directory is drive-relative: %w", fs.ErrInvalid)
		}
		if filepath.Separator == '\\' && strings.HasPrefix(native, string(filepath.Separator)) {
			return fmt.Errorf("rules directory is drive-less rooted: %w", fs.ErrInvalid)
		}
	}
	return nil
}

func validateReferencePath(rulePath string) error {
	if !utf8.ValidString(rulePath) || strings.IndexByte(rulePath, 0) >= 0 {
		return fmt.Errorf("invalid rule path %q: %w", rulePath, fs.ErrInvalid)
	}
	if err := validateRulePath(rulePath); err != nil {
		return err
	}
	native := filepath.FromSlash(rulePath)
	if filepath.IsAbs(native) || !filepath.IsLocal(native) {
		return fmt.Errorf("invalid rule path %q: %w", rulePath, fs.ErrInvalid)
	}
	return nil
}

// ReadResolvedRules reads resolved rules from their borrowed filesystem roots.
// The roots remain owned and managed by the caller.
func ReadResolvedRules(projectFS, memoryFS fs.FS, entries []RulePath) ([]RuleContent, error) {
	for index, entry := range entries {
		if err := validateRulePathEntry(entry); err != nil {
			return nil, fmt.Errorf("invalid resolved rule at index %d: %w", index, err)
		}
	}

	selected := make([]RulePath, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry.Path]; ok {
			continue
		}
		seen[entry.Path] = struct{}{}
		selected = append(selected, entry)
	}

	projectUsed, memoryUsed := false, false
	for _, entry := range selected {
		switch entry.Source {
		case RuleSourceProject:
			projectUsed = true
		case RuleSourceMemory:
			memoryUsed = true
		}
	}
	if projectUsed && isNilFS(projectFS) {
		return nil, fmt.Errorf("read resolved rules: nil project filesystem: %w", fs.ErrInvalid)
	}
	if memoryUsed && isNilFS(memoryFS) {
		return nil, fmt.Errorf("read resolved rules: nil memory filesystem: %w", fs.ErrInvalid)
	}

	rules := make([]RuleContent, 0, len(selected))
	for _, entry := range selected {
		readFS := projectFS
		if entry.Source == RuleSourceMemory {
			readFS = memoryFS
		}
		contents, err := ReadRules(readFS, []string{entry.ReadPath})
		if err != nil {
			return nil, fmt.Errorf("read resolved rule %q: %w", entry.Path, err)
		}
		if len(contents) == 0 {
			continue
		}
		rules = append(rules, RuleContent{Path: entry.Path, Text: contents[0].Text})
	}
	return rules, nil
}

func validateRulePathEntry(entry RulePath) error {
	if err := validateReferencePath(entry.Path); err != nil {
		return fmt.Errorf("display path: %w", err)
	}
	if err := validateReferencePath(entry.ReadPath); err != nil {
		return fmt.Errorf("read path: %w", err)
	}
	switch entry.Source {
	case RuleSourceProject, RuleSourceMemory:
		return nil
	default:
		return fmt.Errorf("unknown source %d: %w", entry.Source, fs.ErrInvalid)
	}
}
