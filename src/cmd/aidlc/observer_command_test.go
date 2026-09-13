package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Shared by observers so forwarding uses the same role paths as deployed hooks.
func observerHookCommand(ctx context.Context, binary, root string) *exec.Cmd {
	suffix := ""
	if strings.HasSuffix(binary, ".exe") {
		suffix = ".exe"
	}
	return exec.CommandContext(ctx, binary, "__hook", "--project-dir", root, "--okf-binary", filepath.Join(filepath.Dir(binary), "okf"+suffix))
}
func TestObserverBinaryArgs(t *testing.T) {
	for _, binary := range []string{"/project/aidlc/bin/v0.1.1/aidlc", "/tools/aidlc.exe"} {
		t.Run(binary, func(t *testing.T) {
			suffix := ""
			if strings.HasSuffix(binary, ".exe") {
				suffix = ".exe"
			}
			want := []string{binary, "__hook", "--project-dir", "/project", "--okf-binary", filepath.Join(filepath.Dir(binary), "okf"+suffix)}
			if got := observerHookCommand(t.Context(), binary, "/project").Args; !reflect.DeepEqual(got, want) {
				t.Fatalf("observer forwarded %q, want %q", got, want)
			}
		})
	}
}
