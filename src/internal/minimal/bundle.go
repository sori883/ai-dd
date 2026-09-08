package minimal

import (
	"os"
	"path/filepath"
	"regexp"
)

type bundleStore struct{ Root, Space, Bundle string }

func (s bundleStore) ValidateRoot() error {
	if !filepath.IsAbs(s.Root) || !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(s.Space) {
		return invalid("invalid project or Space")
	}
	expected := filepath.Join(s.Root, "aidlc", "spaces", s.Space, "knowledge")
	if filepath.Clean(s.Bundle) != expected {
		return invalid("bundle does not match Space")
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	parts := []string{"aidlc", "spaces", s.Space, "knowledge"}
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return invalid("bundle path is not a real directory")
		}
	}
	return nil
}
