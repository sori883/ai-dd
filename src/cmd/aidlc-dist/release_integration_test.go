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

	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
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
	for _, product := range release.Products {
		got := verifyProductDistribution(t, dir, product)
		if err := validateReleaseMetadata(got, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := release.ValidateData(mustRead(t, filepath.Join(dir, "aidlc-assets-manifest.json")), mustRead(t, filepath.Join(dir, "aidlc-assets-SHA256SUMS")), mustRead(t, filepath.Join(dir, "aidlc-assets_"+e.Version+".tar.gz")), e.Version, e.Commit); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 43 {
		t.Fatal("release requires 43 assets", len(files), err)
	}
	return m
}

func TestReleaseCandidateNative(t *testing.T) {
	dir, e := releaseInputs(t)
	verifyReleaseCandidate(t, dir, e)
	target := runtime.GOOS + "/" + runtime.GOARCH
	base := t.TempDir()
	bins := map[string]string{}
	for _, product := range release.Products {
		mn, sn := release.MetadataNames(product)
		_, a, err := release.ValidateManifest(mustRead(t, filepath.Join(dir, mn)), mustRead(t, filepath.Join(dir, sn)), product, e.Version, target)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := release.ValidateBinary(mustRead(t, filepath.Join(dir, a.Archive)), a)
		if err != nil {
			t.Fatal(err)
		}
		binary := filepath.Join(base, a.Binary)
		if err := os.WriteFile(binary, payload, 0755); err != nil {
			t.Fatal(err)
		}
		bins[product] = binary
	}
	t.Setenv("PATH", t.TempDir())
	for _, product := range release.Products {
		out := distributionOK(t, base, bins[product], "--version")
		if !bytes.Contains(out, []byte(e.Version)) {
			t.Fatalf("%s version: %s", product, out)
		}
		if len(distributionOK(t, base, bins[product], "--help")) == 0 {
			t.Fatal("missing help", product)
		}
	}
	root := releaseProjectDirectory(t, filepath.Join(base, "project"))
	var result struct{ Paths []string }
	installArgs := []string{"codex", "--release-version", e.Version, "--release-dir", dir, "--project-dir", root}
	if err := json.Unmarshal(distributionOK(t, root, bins["aidlc-install"], installArgs...), &result); err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	runtimePath := func(root, product string) string {
		return filepath.Join(root, "aidlc", "bin", e.Version, product+suffix)
	}
	b := codex.Binaries{AIDLC: runtimePath(root, "aidlc"), OKF: runtimePath(root, "okf"), Natural: runtimePath(root, "natural-japanese-go")}
	expected, err := codex.DistributionFrom(root, b, core.Files, codex.Files)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Paths) != len(expected)+3 {
		t.Fatal("installed count", len(result.Paths))
	}
	for _, a := range expected {
		if !bytes.Equal(mustRead(t, filepath.Join(root, a.Path)), a.Data) {
			t.Fatal("source mismatch", a.Path)
		}
	}
	before := snapshotFixture(t, root, result.Paths)
	if _, err := distributionCommand(t, root, bins["aidlc-install"], installArgs...); err == nil {
		t.Fatal("overwrite accepted")
	}
	if !reflect.DeepEqual(before, snapshotFixture(t, root, result.Paths)) {
		t.Fatal("reinstall changed files")
	}
	text := filepath.Join(root, "text.md")
	writeFixture(t, root, "text.md", []byte("非常に重要。\n"))
	report := distributionOK(t, root, b.Natural, "--json", text)
	if !bytes.Contains(report, []byte("forbidden_phrase")) {
		t.Fatal(string(report))
	}
	writeFixture(t, root, "previous.json", report)
	if out := distributionOK(t, root, b.Natural, "--json", "--baseline", filepath.Join(root, "previous.json"), text); !bytes.Contains(out, []byte("persisting")) {
		t.Fatal(string(out))
	}
	distributionOK(t, root, b.OKF, "rules", "--space", "default")
	distributionOK(t, root, b.AIDLC, "space", "list")
	distributionOK(t, root, b.AIDLC, "space", "create", "example")
	writeFixture(t, root, "body.md", []byte("Verified release candidate behavior.\n"))
	distributionOK(t, root, b.OKF, "create", "codekb/release", "--space", "default", "--body-file", "body.md", "--actor", "process:test", "--type", "Design", "--title", "Release", "--description", "Verified release behavior")
	if out := distributionOK(t, root, b.OKF, "search", "Release", "--space", "default"); !bytes.Contains(out, []byte("codekb/release")) {
		t.Fatal(string(out))
	}
	users := map[string]string{"AGENTS.md": "user instructions\n", ".codex/config.toml": "# user config\n", ".codex/agents/user.toml": "# user agent\n", "aidlc/spaces/default/knowledge/codekb/user.md": "user knowledge\n", "aidlc/spaces/other/knowledge/user.md": "other Space\n", "aidlc/.runtime/user.txt": "user state\n"}
	names := []string{}
	for name, raw := range users {
		writeFixture(t, root, name, []byte(raw))
		names = append(names, name)
	}
	writeFixture(t, root, ".codex/hooks.json", withCustomHook(t, mustRead(t, filepath.Join(root, ".codex/hooks.json"))))
	saved := snapshotFixture(t, root, names)
	moved := filepath.Join(base, "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	relocateArgs := []string{"codex", "--release-version", e.Version, "--release-dir", dir, "--project-dir", moved, "--relocate", "--from-project-dir", root, "--from-binary", b.AIDLC}
	skill := ".agents/skills/aidlc/SKILL.md"
	original := mustRead(t, filepath.Join(moved, skill))
	writeFixture(t, moved, skill, append(append([]byte{}, original...), []byte("unknown edit\n")...))
	changed := snapshotFixture(t, moved, result.Paths)
	if _, err := distributionCommand(t, moved, bins["aidlc-install"], relocateArgs...); err == nil {
		t.Fatal("unknown edit accepted")
	}
	if !reflect.DeepEqual(changed, snapshotFixture(t, moved, result.Paths)) {
		t.Fatal("failed relocation changed files")
	}
	writeFixture(t, moved, skill, original)
	distributionOK(t, moved, bins["aidlc-install"], relocateArgs...)
	hooks := mustRead(t, filepath.Join(moved, ".codex/hooks.json"))
	if !bytes.Contains(hooks, []byte(customHook)) || bytes.Contains(hooks, []byte(root)) {
		t.Fatal("custom hook/root changed")
	}
	if !reflect.DeepEqual(saved, snapshotFixture(t, moved, names)) {
		t.Fatal("user data changed")
	}
	if string(mustRead(t, filepath.Join(moved, "text.md"))) != "非常に重要。\n" {
		t.Fatal("input modified")
	}
	if _, err := os.Lstat(filepath.Join(moved, ".git")); !os.IsNotExist(err) {
		t.Fatal("created Git metadata")
	}
	t.Logf("same 43-asset candidate: five native CLIs, fresh install, role references, preservation and same-version relocation on %s; no live Codex execution", target)
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
