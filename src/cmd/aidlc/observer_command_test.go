//go:build integration && diagnostic

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// Shared by observers so forwarding uses the same role paths as deployed hooks.
func observerHookCommand(ctx context.Context, binary, root string) *exec.Cmd {
	suffix := ""
	if strings.HasSuffix(binary, ".exe") {
		suffix = ".exe"
	}
	return exec.CommandContext(ctx, binary, "__hook", "--project-dir", root, "--okf-binary", filepath.Join(filepath.Dir(binary), "okf"+suffix))
}
