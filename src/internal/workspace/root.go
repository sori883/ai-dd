// Package workspace resolves the locations used by AI-DLC workspace data.
package workspace

import "path/filepath"

// RootInput contains the project-directory candidates in precedence order.
// WorkingDir must be an absolute path supplied by the process boundary.
type RootInput struct {
	ExplicitDir      string
	AIDLCProjectDir  string
	ClaudeProjectDir string
	WorkingDir       string
}

// ResolveRoot selects and normalizes the project root without accessing the filesystem.
func ResolveRoot(input RootInput) string {
	candidate := input.WorkingDir

	switch {
	case input.ExplicitDir != "":
		candidate = input.ExplicitDir
	case input.AIDLCProjectDir != "":
		candidate = input.AIDLCProjectDir
	case input.ClaudeProjectDir != "":
		candidate = input.ClaudeProjectDir
	}

	if filepath.IsAbs(candidate) {
		return filepath.Clean(candidate)
	}

	return filepath.Clean(filepath.Join(input.WorkingDir, candidate))
}
