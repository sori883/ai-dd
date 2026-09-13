package projectroot

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolve locates the installed manager independently of Git layout.
func Resolve(explicit, cwd string, install bool) (string, error) {
	canonical := func(p string) (string, error) {
		p, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		p, err = filepath.EvalSymlinks(p)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("project root must be a directory")
		}
		return p, nil
	}
	if explicit != "" {
		return canonical(explicit)
	}
	root, err := canonical(cwd)
	if err != nil {
		return "", err
	}
	var candidates []string
	for p := root; ; p = filepath.Dir(p) {
		installed, err := installedRoot(p)
		if err != nil {
			return "", err
		}
		if installed {
			candidates = append(candidates, p)
		}

		if filepath.Dir(p) == p {
			break
		}
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple installed projects; specify --project-dir")
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if install {
		return root, nil
	}
	return "", fmt.Errorf("project is not installed; run aidlc-install codex or specify --project-dir")
}

// Check path components before descending: an ancestor may legitimately contain
// a standalone aidlc binary. Only missing paths and non-directory components
// are non-candidates; permission and other filesystem errors remain visible.
func installedRoot(root string) (bool, error) {
	path := root
	for _, component := range []string{"aidlc", "workflow", "stage-graph.json"} {
		path = filepath.Join(path, component)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if component == "stage-graph.json" {
			return info.Mode().IsRegular(), nil
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return false, nil
}
