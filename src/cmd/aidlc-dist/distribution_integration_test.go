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

// TestDistributionArchives verifies candidates built outside this test, without executing them.
func TestDistributionArchives(t *testing.T) {
	dir, e := releaseInputs(t)
	verifyReleaseCandidate(t, dir, e)
	t.Log("verified same six bundled archives; foreign binaries were not executed")
}

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
func fixtureBinaryPath(t *testing.T, binary string) string {
	t.Helper()
	actual, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	return actual
}

func TestDistributionBinaryPath(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(realDir, "aidlc")
	if err := os.WriteFile(binary, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	if got := fixtureBinaryPath(t, filepath.Join(alias, "aidlc")); got != want {
		t.Fatalf("binary reference = %q, want installed physical path %q", got, want)
	}
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

// TestDistributionJourney validates a manual procedure, not an automatic updater
// or compatibility with an unknown future version. Both builds use this source.
// Same-version relocation is exercised against the one verified release candidate.
func TestDistributionJourney(t *testing.T) { TestReleaseCandidateNative(t) }

func TestNaturalJapaneseDistributionArchives(t *testing.T) { TestReleaseCandidateMetadata(t) }

func TestNaturalJapaneseDistributionJourney(t *testing.T) { TestReleaseCandidateNative(t) }
