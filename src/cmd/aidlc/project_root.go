package main

import "github.com/sori883/ai-dd/src/internal/projectroot"

func resolveProjectRoot(explicit, cwd string, install bool) (string, error) {
	return projectroot.Resolve(explicit, cwd, install)
}
