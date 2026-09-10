//go:build integration

package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	t.Helper()
	raw := mustRead(t, filepath.Join(dir, "manifest.json"))
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != 1 || len(m.Artifacts) == 0 {
		t.Fatalf("invalid manifest: %+v", m)
	}
	sums := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(mustRead(t, filepath.Join(dir, "SHA256SUMS"))), "\n"), "\n") {
		hash, name, ok := strings.Cut(line, "  ")
		if !ok || filepath.Base(name) != name || sums[name] != "" {
			t.Fatalf("invalid checksum line %q", line)
		}
		if hash != digest(mustRead(t, filepath.Join(dir, name))) {
			t.Fatalf("checksum mismatch: %s", name)
		}
		sums[name] = hash
	}
	if sums["manifest.json"] != digest(raw) || len(sums) != len(m.Artifacts)+1 {
		t.Fatal("checksum list does not match manifest")
	}
	seen := map[string]bool{}
	for _, a := range m.Artifacts {
		if seen[a.Target] {
			t.Fatal("duplicate target", a.Target)
		}
		seen[a.Target] = true
		valid := false
		for _, target := range fixtureTargets {
			if a.Target == target {
				valid = true
			}
		}
		if !valid {
			t.Fatal("unknown target", a.Target)
		}
		binary, suffix := "aidlc", ".tar.gz"
		if strings.HasPrefix(a.Target, "windows/") {
			binary, suffix = "aidlc.exe", ".zip"
		}
		expected := "aidlc_" + m.Version + "_" + strings.ReplaceAll(a.Target, "/", "_") + suffix
		if a.Binary != binary || a.Archive != expected || filepath.Base(expected) != expected {
			t.Fatalf("unexpected archive names: %+v", a)
		}
		compressed := mustRead(t, filepath.Join(dir, a.Archive))
		if a.ArchiveSize != int64(len(compressed)) || a.ArchiveSHA256 != digest(compressed) || sums[a.Archive] != a.ArchiveSHA256 {
			t.Fatal("archive metadata mismatch", a.Target)
		}
		payload := distributionPayload(t, a, compressed)
		if a.BinarySize != int64(len(payload)) || a.BinarySHA256 != digest(payload) || len(payload) == 0 {
			t.Fatal("binary metadata mismatch", a.Target)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != len(m.Artifacts)+2 {
		t.Fatalf("unexpected output files: %v %v", files, err)
	}
	return m
}
func distributionPayload(t *testing.T, a artifact, raw []byte) []byte {
	t.Helper()
	if strings.HasSuffix(a.Archive, ".zip") {
		z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if len(z.File) != 1 || z.File[0].Name != a.Binary || !z.File[0].Mode().IsRegular() {
			t.Fatal("unexpected zip content")
		}
		r, err := z.File[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		payload, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return payload
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != a.Binary || h.Typeflag != tar.TypeReg || h.Mode != 0755 {
		t.Fatal("unexpected tar content", h)
	}
	payload, err := io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatal("extra tar content", err)
	}
	return payload
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
	return binary
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
func TestDistributionJourney(t *testing.T) {
	t.Logf("native distribution journey: %s/%s %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
	source, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(distributionOK(t, source, "git", "rev-parse", "HEAD")))
	base := t.TempDir()
	oldBinary := candidateBinary(t, source, filepath.Join(base, "old"), "dev-fixture-old", commit)
	newBinary := candidateBinary(t, source, filepath.Join(base, "new"), "dev-fixture-new", commit)
	root := fixtureGitRoot(t, filepath.Join(base, "project"))
	var installed struct{ Paths []string }
	if err := json.Unmarshal(distributionOK(t, root, oldBinary, "install", "codex", "--project-dir", root), &installed); err != nil {
		t.Fatal(err)
	}
	beforeInstall := snapshotFixture(t, root, installed.Paths)
	if _, err := distributionCommand(t, root, oldBinary, "install", "codex", "--project-dir", root); err == nil {
		t.Fatal("reinstall replaced existing files")
	}
	if !reflect.DeepEqual(beforeInstall, snapshotFixture(t, root, installed.Paths)) {
		t.Fatal("failed reinstall modified files")
	}
	users := map[string]string{
		"AGENTS.md": "User-owned instructions\n", ".codex/config.toml": "# user config\n", ".codex/agents/user.toml": "# user agent\n",
		"aidlc/spaces/default/knowledge/codekb/user.md": "User knowledge\n", "aidlc/spaces/default/knowledge/adr/user.md": "User decision\n",
		"aidlc/spaces/other/knowledge/user.md": "Other Space\n", "aidlc/spaces/default/intents/fixture/state.json": "{\"fixture_state\":true}\n",
		"aidlc/spaces/default/intents/fixture/history.jsonl": "{\"fixture_history\":true}\n", "aidlc/.runtime/assignments/registry.json": "{\"fixture_runtime\":true}\n",
	}
	rule := "aidlc/spaces/default/knowledge/rules/rule.md"
	users[rule] = string(mustRead(t, filepath.Join(root, rule))) + "\nUser-specific Rule\n"
	names := []string{}
	for name, raw := range users {
		writeFixture(t, root, name, []byte(raw))
		names = append(names, name)
	}
	writeFixture(t, root, ".codex/hooks.json", withCustomHook(t, mustRead(t, filepath.Join(root, ".codex/hooks.json"))))
	preserved := snapshotFixture(t, root, names)
	original := snapshotFixture(t, root, installed.Paths)
	backup := filepath.Join(base, "backup")
	for name, raw := range original {
		writeFixture(t, backup, name, []byte(raw))
	}
	if !reflect.DeepEqual(original, snapshotFixture(t, backup, installed.Paths)) {
		t.Fatal("backup not verified")
	}
	// Unknown skill edits require a decision; the fixture explicitly restores its saved bytes.
	skill := ".agents/skills/aidlc/SKILL.md"
	writeFixture(t, root, skill, []byte(original[skill]+"\nUnknown local edit\n"))
	changed := snapshotFixture(t, root, installed.Paths)
	if _, err := distributionCommand(t, root, newBinary, "install", "codex", "--relocate", "--project-dir", root, "--from-project-dir", root, "--from-binary", oldBinary); err == nil {
		t.Fatal("unknown skill edit was accepted")
	}
	if !reflect.DeepEqual(changed, snapshotFixture(t, root, installed.Paths)) {
		t.Fatal("rejected relocation changed files")
	}
	writeFixture(t, root, skill, []byte(original[skill]))
	stage := fixtureGitRoot(t, filepath.Join(base, "stage"))
	var candidate struct{ Paths []string }
	if err := json.Unmarshal(distributionOK(t, stage, newBinary, "install", "codex", "--project-dir", stage), &candidate); err != nil {
		t.Fatal(err)
	}
	// Explicitly selected product files only; seed Space data is never applied to an existing project.
	for _, name := range candidate.Paths {
		if strings.HasPrefix(name, "aidlc/spaces/") {
			continue
		}
		raw := mustRead(t, filepath.Join(stage, filepath.FromSlash(name)))
		if name == ".codex/hooks.json" {
			raw = withCustomHook(t, raw)
		}
		writeFixture(t, root, name, raw)
	}
	distributionOK(t, root, newBinary, "install", "codex", "--relocate", "--project-dir", root, "--from-project-dir", stage, "--from-binary", newBinary)
	hooks := mustRead(t, filepath.Join(root, ".codex/hooks.json"))
	if !bytes.Contains(hooks, []byte(customHook)) || bytes.Contains(hooks, []byte(stage)) {
		t.Fatal("custom hook or staging reference not preserved/corrected")
	}
	var parsed struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(hooks, &parsed); err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	expectedCommand := quote(newBinary) + " __minimal-hook --project-dir " + quote(root)
	for event, groups := range parsed.Hooks {
		if event == "Notification" {
			continue
		}
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if handler.Command != expectedCommand {
					t.Fatalf("incorrect product reference: %q want %q", handler.Command, expectedCommand)
				}
			}
		}
	}
	if !reflect.DeepEqual(preserved, snapshotFixture(t, root, names)) {
		t.Fatal("manual switch changed user data")
	}
	if string(mustRead(t, filepath.Join(root, skill))) == original[skill] {
		t.Fatal("versioned binary reference did not change")
	}
	for name := range original {
		if strings.HasPrefix(name, "aidlc/spaces/") {
			continue
		}
		writeFixture(t, root, name, mustRead(t, filepath.Join(backup, filepath.FromSlash(name))))
	}
	if !reflect.DeepEqual(original, snapshotFixture(t, root, installed.Paths)) || !reflect.DeepEqual(preserved, snapshotFixture(t, root, names)) {
		t.Fatal("rollback did not restore exact bytes")
	}
	got := string(distributionOK(t, root, oldBinary, "version"))
	if !strings.Contains(got, "dev-fixture-old") {
		t.Fatal("old binary unavailable after rollback")
	}
	t.Log("manual reference switch and byte restoration passed; no Codex hook execution or unknown-version upgrade compatibility claimed")
}
