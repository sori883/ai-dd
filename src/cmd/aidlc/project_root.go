package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveProjectRoot locates the installed manager independently of Git layout.
func resolveProjectRoot(explicit, cwd string, install bool) (string, error) {
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
		info, err := os.Stat(filepath.Join(p, "aidlc/workflow/stage-graph.json"))
		if err == nil && info.Mode().IsRegular() {
			candidates = append(candidates, p)
		} else if err != nil && !os.IsNotExist(err) {
			return "", err
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
	return "", fmt.Errorf("project is not installed; run aidlc install codex or specify --project-dir")
}
