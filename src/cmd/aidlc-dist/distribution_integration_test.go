//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func distributionCommand(t *testing.T, dir, command string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	t.Logf("%s %v: %v\n%s", command, args, err, out)
	return out, err
}
func distributionOK(t *testing.T, dir, command string, args ...string) []byte {
	t.Helper()
	out, err := distributionCommand(t, dir, command, args...)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	return out
}

func writeFixture(t *testing.T, root, name string, raw []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0644); err != nil {
		t.Fatal(err)
	}
}
func snapshotFixture(t *testing.T, root string, names []string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, name := range names {
		m[name] = string(mustRead(t, filepath.Join(root, filepath.FromSlash(name))))
	}
	return m
}
func fixtureGitRoot(t *testing.T, root string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	distributionOK(t, root, "git", "init", "-q")
	actual, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return actual
}

const customHook = `"Notification": [
      {"hooks":[{"type":"command","command":"echo user-hook","timeout":3}]}
    ]`

func withCustomHook(t *testing.T, raw []byte) []byte {
	t.Helper()
	ending := "\n  }\n}"
	if !bytes.HasSuffix(raw, []byte(ending)) {
		t.Fatal("unexpected fixture hook layout")
	}
	return []byte(strings.TrimSuffix(string(raw), ending) + ",\n    " + customHook + ending)
}
