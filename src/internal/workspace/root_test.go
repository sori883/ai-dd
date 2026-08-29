package workspace

import (
	"path/filepath"
	"testing"
)

func TestResolveRoot(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	workingDir := filepath.Join(baseDir, "work", "project")
	explicitDir := filepath.Join(baseDir, "explicit")
	uncleanExplicitDir := explicitDir + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".."
	aidlcProjectDir := filepath.Join(baseDir, "aidlc-env")
	claudeProjectDir := filepath.Join(baseDir, "claude-env")

	tests := []struct {
		name     string
		input    RootInput
		expected string
	}{
		{
			name: "explicit directory has highest precedence",
			input: RootInput{
				ExplicitDir:      explicitDir,
				AIDLCProjectDir:  aidlcProjectDir,
				ClaudeProjectDir: claudeProjectDir,
				WorkingDir:       workingDir,
			},
			expected: explicitDir,
		},
		{
			name: "absolute explicit directory is cleaned",
			input: RootInput{
				ExplicitDir: uncleanExplicitDir,
				WorkingDir:  workingDir,
			},
			expected: explicitDir,
		},
		{
			name: "aidlc environment precedes claude environment",
			input: RootInput{
				AIDLCProjectDir:  aidlcProjectDir,
				ClaudeProjectDir: claudeProjectDir,
				WorkingDir:       workingDir,
			},
			expected: aidlcProjectDir,
		},
		{
			name: "claude environment precedes working directory",
			input: RootInput{
				ClaudeProjectDir: claudeProjectDir,
				WorkingDir:       workingDir,
			},
			expected: claudeProjectDir,
		},
		{
			name: "working directory is the fallback",
			input: RootInput{
				WorkingDir: filepath.Join(workingDir, "."),
			},
			expected: workingDir,
		},
		{
			name: "relative explicit directory resolves from working directory",
			input: RootInput{
				ExplicitDir: "../selected",
				WorkingDir:  workingDir,
			},
			expected: filepath.Join(baseDir, "work", "selected"),
		},
		{
			name: "relative aidlc environment resolves from working directory",
			input: RootInput{
				AIDLCProjectDir: "./nested/../selected",
				WorkingDir:      workingDir,
			},
			expected: filepath.Join(workingDir, "selected"),
		},
		{
			name: "relative claude environment resolves from working directory",
			input: RootInput{
				ClaudeProjectDir: "./nested/../selected",
				WorkingDir:       workingDir,
			},
			expected: filepath.Join(workingDir, "selected"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ResolveRoot(tt.input); got != tt.expected {
				t.Errorf("ResolveRoot() = %q, want %q", got, tt.expected)
			}
		})
	}
}
