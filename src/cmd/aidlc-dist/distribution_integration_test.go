//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestDistributionArchives verifies candidates built outside this test, without executing them.
func TestDistributionArchives(t *testing.T) {
	dir := os.Getenv("AIDLC_DIST_DIR")
	if dir == "" {
		t.Skip("set AIDLC_DIST_DIR to the six-target candidate directory")
	}
	m := verifyDistribution(t, dir)
	if len(m.Artifacts) != 6 {
		t.Fatalf("six targets required, got %d", len(m.Artifacts))
	}
	t.Logf("verified six cross-built archives on %s/%s; foreign binaries were not executed", runtime.GOOS, runtime.GOARCH)
}

func verifyDistribution(t *testing.T, dir string) manifest {
	return verifyProductDistribution(t, dir, "aidlc")
}
func verifyProductDistribution(t *testing.T, dir, product string) manifest {
	t.Helper()
	name, sums := release.MetadataNames(product)
	raw := mustRead(t, filepath.Join(dir, name))
	var old manifest
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	m, _, err := release.ValidateManifest(raw, mustRead(t, filepath.Join(dir, sums)), product, old.Version, "linux/amd64")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range m.Artifacts {
		if _, err := release.ValidateBinary(mustRead(t, filepath.Join(dir, a.Archive)), a); err != nil {
			t.Fatal(err)
		}
	}
	return old
}

func distributionPayload(t *testing.T, a artifact, raw []byte) []byte {
	t.Helper()
	entries, err := release.Unpack(raw, strings.HasSuffix(a.Archive, ".zip"), release.MaxArchiveBytes)
	if err != nil {
		t.Fatal(err)
	}
	return entries[a.Binary]
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
func candidateBinary(t *testing.T, source, base, version, commit string) string {
	t.Helper()
	input := filepath.Join(base, "input")
	if err := os.MkdirAll(input, 0700); err != nil {
		t.Fatal(err)
	}
	target := runtime.GOOS + "/" + runtime.GOARCH
	name := "aidlc-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	flags := "-X github.com/sori883/ai-dd/src/internal/buildinfo.Version=" + version + " -X github.com/sori883/ai-dd/src/internal/buildinfo.Commit=" + commit
	distributionOK(t, source, "go", "build", "-trimpath", "-ldflags", flags, "-o", filepath.Join(input, name), "./src/cmd/aidlc")
	o := options{InputDir: input, OutputDir: filepath.Join(base, "archives"), Version: version, Commit: commit, GoVersion: runtime.Version(), Targets: []string{target}}
	var stdout, stderr bytes.Buffer
	if code := run(append(commandArgs(o), "--targets", target), &stdout, &stderr); code != 0 {
		t.Fatalf("packaging code %d: %s", code, stderr.String())
	}
	m := verifyDistribution(t, o.OutputDir)
	extracted := filepath.Join(base, "extracted")
	if err := os.Mkdir(extracted, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(extracted, m.Artifacts[0].Binary)
	payload := distributionPayload(t, m.Artifacts[0], mustRead(t, filepath.Join(o.OutputDir, m.Artifacts[0].Archive)))
	if err := os.WriteFile(binary, payload, 0755); err != nil {
		t.Fatal(err)
	}
	got := string(distributionOK(t, base, binary, "version"))
	if strings.TrimSpace(got) != "aidlc "+version+" (commit "+commit+")" {
		t.Fatalf("version mismatch: %q", got)
	}
	if got := distributionOK(t, base, binary, "--help"); !bytes.Contains(got, []byte("Usage:")) {
		t.Fatal("help missing")
	}
	return fixtureBinaryPath(t, binary)
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

func TestNaturalJapaneseDistributionArchives(t *testing.T) {
	dir := os.Getenv("AIDLC_NATURAL_DIST_DIR")
	if dir == "" {
		t.Skip("set AIDLC_NATURAL_DIST_DIR")
	}
	m := verifyProductDistribution(t, dir, "natural-japanese-go")
	if len(m.Artifacts) != 6 {
		t.Fatal("six targets required")
	}
}
func naturalDistributionPayload(t *testing.T, a artifact, raw []byte) []byte {
	return distributionPayload(t, a, raw)
}

func TestNaturalJapaneseDistributionJourney(t *testing.T) { TestReleaseCandidateNative(t) }
