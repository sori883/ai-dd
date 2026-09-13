//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/harness/codex"
)

type releaseExpectation struct{ Version, Commit, GoVersion string }

func TestReleaseCandidateMetadataValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		change    func(*manifest, *releaseExpectation)
		wantError bool
	}{
		{"matching six targets", func(*manifest, *releaseExpectation) {}, false},
		{"missing expected version", func(_ *manifest, e *releaseExpectation) { e.Version = "" }, true},
		{"missing expected commit", func(_ *manifest, e *releaseExpectation) { e.Commit = "" }, true},
		{"missing expected go version", func(_ *manifest, e *releaseExpectation) { e.GoVersion = "" }, true},
		{"version mismatch", func(m *manifest, _ *releaseExpectation) { m.Version = "other" }, true},
		{"commit mismatch", func(m *manifest, _ *releaseExpectation) { m.SourceCommit = strings.Repeat("b", 40) }, true},
		{"go version mismatch", func(m *manifest, _ *releaseExpectation) { m.GoVersion = "go1.26.5" }, true},
		{"schema mismatch", func(m *manifest, _ *releaseExpectation) { m.SchemaVersion = 2 }, true},
		{"missing target", func(m *manifest, _ *releaseExpectation) { m.Artifacts = m.Artifacts[:5] }, true},
		{"extra target", func(m *manifest, _ *releaseExpectation) {
			m.Artifacts = append(m.Artifacts, artifact{Target: "freebsd/amd64"})
		}, true},
		{"duplicate target", func(m *manifest, _ *releaseExpectation) { m.Artifacts[5] = m.Artifacts[0] }, true},
		{"unknown target", func(m *manifest, _ *releaseExpectation) { m.Artifacts[5].Target = "freebsd/amd64" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, e := releaseMetadataFixture()
			tt.change(&m, &e)
			if err := validateReleaseMetadata(m, e); (err != nil) != tt.wantError {
				t.Fatalf("metadata validation error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func releaseMetadataFixture() (manifest, releaseExpectation) {
	e := releaseExpectation{"v0.1.0", strings.Repeat("a", 40), "go1.26.4"}
	m := manifest{SchemaVersion: 1, Version: e.Version, SourceCommit: e.Commit, GoVersion: e.GoVersion}
	for _, target := range fixtureTargets {
		m.Artifacts = append(m.Artifacts, artifact{Target: target})
	}
	return m, e
}

func validateReleaseMetadata(m manifest, e releaseExpectation) error {
	if e.Version == "" || e.Commit == "" || e.GoVersion == "" {
		return fmt.Errorf("expected version, commit and Go version are required")
	}
	if m.SchemaVersion != 1 || m.Version != e.Version || m.SourceCommit != e.Commit || m.GoVersion != e.GoVersion {
		return fmt.Errorf("candidate metadata differs from expected release: %+v", e)
	}
	if len(m.Artifacts) != len(fixtureTargets) {
		return fmt.Errorf("expected six targets, got %d", len(m.Artifacts))
	}
	remaining := make(map[string]bool, len(fixtureTargets))
	for _, target := range fixtureTargets {
		remaining[target] = true
	}
	for _, a := range m.Artifacts {
		if !remaining[a.Target] {
			return fmt.Errorf("unknown or duplicate target %q", a.Target)
		}
		delete(remaining, a.Target)
	}
	return nil
}

func TestReleaseCandidateNativeSelection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, target string
		artifacts    []artifact
		want         string
		wantError    bool
	}{
		{"select matching target", "linux/arm64", []artifact{{Target: "linux/amd64", Binary: "aidlc", Archive: "amd64.tar.gz"}, {Target: "linux/arm64", Binary: "aidlc", Archive: "arm64.tar.gz"}}, "arm64.tar.gz", false},
		{"select windows", "windows/amd64", []artifact{{Target: "windows/amd64", Binary: "aidlc.exe", Archive: "windows.zip"}}, "windows.zip", false},
		{"missing native", "linux/arm64", []artifact{{Target: "linux/amd64", Binary: "aidlc", Archive: "amd64.tar.gz"}}, "", true},
		{"duplicate native", "linux/amd64", []artifact{{Target: "linux/amd64", Binary: "aidlc", Archive: "one.tar.gz"}, {Target: "linux/amd64", Binary: "aidlc", Archive: "two.tar.gz"}}, "", true},
		{"unknown native", "freebsd/amd64", []artifact{{Target: "freebsd/amd64", Binary: "aidlc", Archive: "one.tar.gz"}}, "", true},
		{"binary mismatch", "linux/amd64", []artifact{{Target: "linux/amd64", Binary: "aidlc.exe", Archive: "one.tar.gz"}}, "", true},
		{"format mismatch", "windows/amd64", []artifact{{Target: "windows/amd64", Binary: "aidlc.exe", Archive: "one.tar.gz"}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectReleaseNative(tt.artifacts, tt.target)
			if (err != nil) != tt.wantError {
				t.Fatalf("native selection error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && got.Archive != tt.want {
				t.Fatalf("selected archive = %q, want %q", got.Archive, tt.want)
			}
		})
	}
}

func selectReleaseNative(artifacts []artifact, target string) (artifact, error) {
	known := false
	for _, candidate := range fixtureTargets {
		if target == candidate {
			known = true
		}
	}
	if !known {
		return artifact{}, fmt.Errorf("unsupported native target %q", target)
	}
	var selected artifact
	count := 0
	for _, a := range artifacts {
		if a.Target == target {
			selected = a
			count++
		}
	}
	if count != 1 {
		return artifact{}, fmt.Errorf("expected one native archive for %s, got %d", target, count)
	}
	binary, suffix := "aidlc", ".tar.gz"
	if strings.HasPrefix(target, "windows/") {
		binary, suffix = "aidlc.exe", ".zip"
	}
	if selected.Binary != binary || !strings.HasSuffix(selected.Archive, suffix) {
		return artifact{}, fmt.Errorf("native archive format mismatch: %+v", selected)
	}
	return selected, nil
}

// Release entry points consume candidates built by the package job; they never rebuild.
func TestReleaseCandidateMetadata(t *testing.T) {
	dir, e := releaseInputs(t)
	verifyReleaseCandidate(t, dir, e)
}

func releaseInputs(t *testing.T) (string, releaseExpectation) {
	t.Helper()
	dir := os.Getenv("AIDLC_DIST_DIR")
	e := releaseExpectation{os.Getenv("AIDLC_RELEASE_VERSION"), os.Getenv("AIDLC_RELEASE_COMMIT"), os.Getenv("AIDLC_RELEASE_GO_VERSION")}
	if dir == "" && e == (releaseExpectation{}) {
		t.Skip("set release candidate directory and expected metadata")
	}
	if dir == "" || e.Version == "" || e.Commit == "" || e.GoVersion == "" {
		t.Fatal("AIDLC_DIST_DIR and all AIDLC_RELEASE_* expectations are required")
	}
	return dir, e
}

func verifyReleaseCandidate(t *testing.T, dir string, e releaseExpectation) manifest {
	t.Helper()
	var m manifest
	if err := json.Unmarshal(mustRead(t, filepath.Join(dir, "manifest.json")), &m); err != nil {
		t.Fatal(err)
	}
	if err := validateReleaseMetadata(m, e); err != nil {
		t.Fatal(err)
	}
	return verifyDistribution(t, dir)
}

func TestReleaseCandidateNative(t *testing.T) {
	dir, e := releaseInputs(t)
	m := verifyReleaseCandidate(t, dir, e)
	target := runtime.GOOS + "/" + runtime.GOARCH
	a, err := selectReleaseNative(m.Artifacts, target)
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	binary := filepath.Join(base, a.Binary)
	payload := distributionPayload(t, a, mustRead(t, filepath.Join(dir, a.Archive)))
	if err := os.WriteFile(binary, payload, 0755); err != nil {
		t.Fatal(err)
	}
	binary = fixtureBinaryPath(t, binary)
	// Keep OS environment variables, but make Git unavailable to every candidate command.
	t.Setenv("PATH", t.TempDir())
	if got := strings.TrimSpace(string(distributionOK(t, base, binary, "version"))); got != "aidlc "+e.Version+" (commit "+e.Commit+")" {
		t.Fatalf("candidate version mismatch: %q", got)
	}
	if got := distributionOK(t, base, binary, "--help"); !bytes.Contains(got, []byte("Usage:")) {
		t.Fatal("candidate help missing")
	}
	root := releaseProjectDirectory(t, filepath.Join(base, "project"))
	var installed struct{ Paths []string }
	if err := json.Unmarshal(distributionOK(t, root, binary, "install", "codex", "--project-dir", root), &installed); err != nil {
		t.Fatal(err)
	}
	expected, err := codex.Distribution(root, binary)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, asset := range expected {
		paths = append(paths, asset.Path)
		if got := mustRead(t, filepath.Join(root, filepath.FromSlash(asset.Path))); !bytes.Equal(got, asset.Data) {
			t.Fatalf("candidate embedded asset differs from checked-out source: %s", asset.Path)
		}
	}
	if !reflect.DeepEqual(installed.Paths, paths) {
		t.Fatalf("installed paths differ: got %v, want %v", installed.Paths, paths)
	}
	before := snapshotFixture(t, root, paths)
	if _, err := distributionCommand(t, root, binary, "install", "codex", "--project-dir", root); err == nil {
		t.Fatal("candidate reinstall accepted existing files")
	}
	if !reflect.DeepEqual(before, snapshotFixture(t, root, paths)) {
		t.Fatal("candidate reinstall modified existing files")
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("candidate installation created .git: %v", err)
	}
	t.Logf("executed package candidate %s on %s; verified version/help/install/assets/preservation, not live AI hook execution", a.Archive, target)
}

func TestReleaseCandidateProjectDirectory(t *testing.T) {
	t.Parallel()
	root := releaseProjectDirectory(t, filepath.Join(t.TempDir(), "project"))
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || !filepath.IsAbs(root) {
		t.Fatalf("project must be an absolute directory: %q, %v", root, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatalf("release candidate project must have no .git: %v", err)
	}
}

func releaseProjectDirectory(t *testing.T, root string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	actual, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return actual
}
