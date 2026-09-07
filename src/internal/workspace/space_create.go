package workspace

import (
	"bytes"
	"errors"
	"fmt"
	core "github.com/sori883/ai-dd/src/core/minimal"
	"github.com/sori883/ai-dd/src/internal/okf"
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
	rule, err := readDefaultRule(root)
	if err != nil {
		return err
	}
	return fs.WalkDir(core.Files, "knowledge", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		destination := filepath.Join(targetPath, filepath.FromSlash(path))
		if entry.IsDir() {
			return root.Mkdir(destination, 0777)
		}
		data, err := core.Files.ReadFile(path)
		if err != nil {
			return err
		}
		if path == "knowledge/rules/rule.md" {
			data = rule
		}
		return writeSpaceFile(destination, string(data), func(name string, flags int, mode fs.FileMode) (io.WriteCloser, error) {
			return root.OpenFile(name, flags, mode)
		})
	})
}

func readDefaultRule(root *os.Root) ([]byte, error) {
	path := "aidlc/spaces/default/knowledge/rules/rule.md"
	parts := strings.Split(path, "/")
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if errors.Is(err, fs.ErrNotExist) {
			return core.Files.ReadFile("knowledge/rules/rule.md")
		}
		if err != nil {
			return nil, err
		}
		if i >= 2 && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("default Rule symlink: %w", fs.ErrInvalid)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("default Rule is not regular: %w", fs.ErrInvalid)
		}
	}
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16*1024 {
		return nil, fmt.Errorf("default Rule exceeds 16 KiB: %w", fs.ErrInvalid)
	}
	concept, err := okf.ParseConcept(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid default Rule: %w", err)
	}
	if concept.Type != "Rule" {
		return nil, fmt.Errorf("default Rule type must be Rule: %w", fs.ErrInvalid)
	}
	return data, nil
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
	return workspaceSlug(raw, 48)
}

func workspaceSlug(raw string, maxLength int) string {
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
	if len(name) > maxLength {
		name = name[:maxLength]
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
