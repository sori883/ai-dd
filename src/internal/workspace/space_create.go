package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CreateSpace creates a new space without selecting it or replacing existing data.
// The selected project must already exist. It returns the normalized name on
// success and an empty name on error; existing targets report fs.ErrExist.
// An error, including a Close failure, may leave a partial or complete space.
// CreateSpace does not roll back, repair, or remove that target.
func CreateSpace(input RootInput, rawName string) (string, error) {
	return createSpace(
		input,
		rawName,
		os.OpenRoot,
		(*os.Root).Close,
		populateSpace,
	)
}

func createSpace(
	input RootInput,
	rawName string,
	openProject func(string) (*os.Root, error),
	closeProject func(*os.Root) error,
	populate func(*os.Root, string) error,
) (name string, err error) {
	name, err = normalizeSpaceName(rawName)
	if err != nil {
		return "", err
	}
	spacePath, err := localizeSpace(name)
	if err != nil {
		return "", err
	}
	projectPath := ResolveRoot(input)
	if !filepath.IsAbs(projectPath) {
		return "", fmt.Errorf("resolve project root %q: %w", projectPath, fs.ErrInvalid)
	}
	projectRoot, err := openProject(projectPath)
	if err != nil {
		return "", fmt.Errorf("open project root %q: %w", projectPath, err)
	}
	defer func() {
		if closeErr := closeProject(projectRoot); closeErr != nil {
			name = ""
			err = errors.Join(err, fmt.Errorf("close project root %q: %w", projectPath, closeErr))
		}
	}()
	spacesPath := filepath.Join("aidlc", "spaces")
	// Match the reference mkdir default; the process umask controls final permissions.
	if err := projectRoot.MkdirAll(spacesPath, 0o777); err != nil {
		return "", fmt.Errorf("create space parents %q: %w", spacesPath, err)
	}
	targetPath := filepath.Join(spacesPath, spacePath)
	if err := projectRoot.Mkdir(targetPath, 0o777); err != nil {
		return "", fmt.Errorf("create space %q: %w", targetPath, err)
	}
	if err := populate(projectRoot, targetPath); err != nil {
		return "", err
	}
	return name, nil
}

func populateSpace(root *os.Root, targetPath string) error {
	for _, relative := range []string{
		"memory", "memory/phases", "memory/templates", "intents", "codekb", "knowledge",
	} {
		name := filepath.Join(targetPath, filepath.FromSlash(relative))
		if err := root.Mkdir(name, 0o777); err != nil {
			return fmt.Errorf("create space directory %q: %w", name, err)
		}
	}
	orgContent, err := readDefaultOrganization(func(name string) (io.ReadCloser, error) {
		return root.Open(name)
	})
	if err != nil {
		return err
	}
	files := []struct {
		name    string
		content string
	}{
		{name: "memory/org.md", content: orgContent},
		{name: "memory/team.md", content: "# Team practices\n"},
		{name: "memory/project.md", content: "# Project overrides\n"},
		{name: "memory/templates/.gitkeep"},
		{name: "codekb/.gitkeep"},
		{name: "knowledge/.gitkeep"},
	}
	openFile := func(name string, flags int, mode fs.FileMode) (io.WriteCloser, error) {
		return root.OpenFile(name, flags, mode)
	}
	for _, file := range files {
		name := filepath.Join(targetPath, filepath.FromSlash(file.name))
		if err := writeSpaceFile(name, file.content, openFile); err != nil {
			return err
		}
	}
	return nil
}

func readDefaultOrganization(openFile func(string) (io.ReadCloser, error)) (content string, err error) {
	name := filepath.FromSlash("aidlc/spaces/default/memory/org.md")
	file, err := openFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "# Organization defaults\n", nil
	}
	if err != nil {
		return "", fmt.Errorf("open default organization %q: %w", name, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			content = ""
			err = errors.Join(err, fmt.Errorf("close default organization %q: %w", name, closeErr))
		}
	}()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read default organization %q: %w", name, err)
	}
	return string(data), nil
}

func writeSpaceFile(
	name string,
	content string,
	openFile func(string, int, fs.FileMode) (io.WriteCloser, error),
) (err error) {
	file, err := openFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("open new space file %q: %w", name, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close new space file %q: %w", name, closeErr))
		}
	}()
	n, err := io.WriteString(file, content)
	if err == nil && n != len(content) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("write new space file %q: %w", name, err)
	}
	return nil
}

func normalizeSpaceName(raw string) (string, error) {
	switch raw {
	case "", "help", "-h":
		return "", fmt.Errorf("invalid space name %q: %w", raw, fs.ErrInvalid)
	}
	name := spaceSlug(raw)
	switch name {
	case "help", "list", "switch", "create", "archive", "rename", "show", "birth":
		return "", fmt.Errorf("reserved space name %q: %w", name, fs.ErrInvalid)
	}
	return name, nil
}

// spaceSlug normalizes names without applying command-specific reserved names.
func spaceSlug(raw string) string {
	// JavaScript lowercasing expands U+0130; Go's simple lowercase mapping does not.
	raw = strings.ReplaceAll(raw, "İ", "i\u0307")
	var slug strings.Builder
	var hasSeparator bool
	for _, char := range strings.ToLower(raw) {
		isLetter := char >= 'a' && char <= 'z'
		isDigit := char >= '0' && char <= '9'
		if !isLetter && !isDigit {
			hasSeparator = slug.Len() > 0
			continue
		}
		if hasSeparator {
			slug.WriteByte('-')
		}
		slug.WriteRune(char)
		hasSeparator = false
	}
	name := slug.String()
	if len(name) > 48 {
		name = name[:48]
	}
	name = strings.TrimRight(name, "-")
	if name == "" {
		return "intent"
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = "intent-" + name
	}
	return name
}
