//go:build integration

package main

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
)

type releaseExpectation struct{ Version, Commit, GoVersion string }

func TestReleaseCandidateMetadataValidation(t *testing.T) {
	t.Run("native version output", func(t *testing.T) {
		e := releaseExpectation{"v0.1.2", strings.Repeat("a", 40), "go1.26.4"}
		for _, product := range release.Products {
			for _, change := range []string{"valid", "product", "version", "commit", "prefix", "suffix", "missing newline"} {
				if product == "natural-japanese-go" && change == "commit" {
					continue
				}
				t.Run(product+"/"+change, func(t *testing.T) {
					output := product + " v0.1.2 (commit " + strings.Repeat("a", 40) + ")\n"
					if product == "natural-japanese-go" {
						output = "natural-japanese-go v0.1.2\n"
					}
					switch change {
					case "product":
						output = strings.Replace(output, product, "wrong-product", 1)
					case "version":
						output = strings.Replace(output, "v0.1.2", "v0.1.20", 1)
					case "commit":
						output = strings.Replace(output, strings.Repeat("a", 40), strings.Repeat("b", 40), 1)
					case "prefix":
						output = "extra\n" + output
					case "suffix":
						output += "extra\n"
					case "missing newline":
						output = strings.TrimSuffix(output, "\n")
					}
					err := validateNativeVersion([]byte(output), product, e)
					if change == "valid" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err == nil {
						t.Fatal("incorrect native output accepted", change)
					}
				})
			}
		}
	})
	t.Run("binary build identity", func(t *testing.T) {
		e := releaseExpectation{"v0.1.2", strings.Repeat("a", 40), "go1.26.4"}
		for _, change := range []string{"valid", "path", "go", "os", "arch", "cgo", "trimpath", "missing cgo", "missing trimpath"} {
			t.Run(change, func(t *testing.T) {
				info := debug.BuildInfo{Path: "github.com/sori883/ai-dd/src/cmd/aidlc", GoVersion: e.GoVersion, Settings: []debug.BuildSetting{{Key: "GOOS", Value: "linux"}, {Key: "GOARCH", Value: "amd64"}, {Key: "CGO_ENABLED", Value: "0"}, {Key: "-trimpath", Value: "true"}}}
				switch change {
				case "path":
					info.Path = "github.com/sori883/ai-dd/src/cmd/okf"
				case "go":
					info.GoVersion = "go1.26.5"
				case "os":
					info.Settings[0].Value = "windows"
				case "arch":
					info.Settings[1].Value = "arm64"
				case "cgo":
					info.Settings[2].Value = "1"
				case "trimpath":
					info.Settings[3].Value = "false"
				case "missing cgo":
					info.Settings = append(info.Settings[:2], info.Settings[3])
				case "missing trimpath":
					info.Settings = info.Settings[:3]
				}
				err := validateBinaryBuild(&info, "aidlc", "linux/amd64", e)
				if change == "valid" {
					if err != nil {
						t.Fatal("valid trimpath binary without ldflags rejected", err)
					}
				} else if err == nil {
					t.Fatal("invalid build accepted", change)
				}
			})
		}
	})
	o := releaseFixture(t)
	if err := packageRelease(o); err != nil {
		t.Fatal(err)
	}
	original := mustRead(t, filepath.Join(o.OutputDir, release.BundleName(o.Version, "linux/amd64")))
	for _, mode := range []string{"valid", "commit", "version", "toolchain", "target", "schema", "mode", "license", "source"} {
		t.Run(mode, func(t *testing.T) {
			entries, err := release.Unpack(original, false, release.MaxArchiveBytes)
			if err != nil {
				t.Fatal(err)
			}
			var m release.BundleManifest
			json.Unmarshal(entries["manifest.json"], &m)
			modes := release.BundlePaths("linux/amd64")
			switch mode {
			case "commit":
				m.SourceCommit = strings.Repeat("b", 40)
			case "version":
				m.Version = "v9"
			case "toolchain":
				m.GoVersion = "go1.26.5"
			case "target":
				m.Target = "linux/arm64"
			case "schema":
				m.SchemaVersion = 1
			case "mode":
				modes["okf"] = 0644
			case "license":
				entries["LICENSES/okf/yaml-NOTICE.txt"] = []byte("changed")
			case "source":
				entries["core/adr-template.md"] = []byte("changed")
			}
			for i := range m.Files {
				f := &m.Files[i]
				f.Size = int64(len(entries[f.Path]))
				f.SHA256 = release.Hash(entries[f.Path])
				f.Mode = modes[f.Path]
			}
			entries["manifest.json"], _ = json.Marshal(m)
			raw, err := release.ArchiveModes(entries, modes, false)
			if err != nil {
				t.Fatal(err)
			}
			err = validateCandidateBundle(raw, releaseExpectation{o.Version, o.Commit, o.GoVersion}, "linux/amd64", release.Hash(raw))
			if mode == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("invalid candidate accepted", mode)
			}
		})
	}
}

func validateCandidateBundle(raw []byte, e releaseExpectation, target, digest string) error {
	m, entries, err := release.ValidateBundleArchive(raw, e.Version, target, digest)
	if err != nil {
		return err
	}
	if e.Version == "" || e.Commit == "" || e.GoVersion == "" || m.SourceCommit != e.Commit || m.GoVersion != e.GoVersion {
		return fmt.Errorf("candidate build identity differs")
	}
	for _, product := range release.Products {
		if err := validateCandidateLicenses(product, raw, strings.HasPrefix(target, "windows/")); err != nil {
			return err
		}
	}
	for p, b := range entries {
		var want []byte
		var err error
		switch {
		case strings.HasPrefix(p, "core/"):
			want, err = core.Files.ReadFile(strings.TrimPrefix(p, "core/"))
		case strings.HasPrefix(p, "codex/"):
			want, err = codex.Files.ReadFile(strings.TrimPrefix(p, "codex/"))
		case p == "LICENSES/PRODUCT.txt":
			want, err = os.ReadFile("../../../LICENSE")
		default:
			continue
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(want, b) {
			return fmt.Errorf("candidate source differs: %s", p)
		}
	}
	return nil
}

func selectBundleNative(sums map[string]string, version, target string) (string, error) {
	name := release.BundleName(version, target)
	if !release.ValidVersion(version) || !slices.Contains(release.Targets, target) || !release.ValidHash(sums[name]) {
		return "", fmt.Errorf("missing or unsupported native bundle")
	}
	return name, nil
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

func verifyReleaseCandidate(t *testing.T, dir string, e releaseExpectation) {
	t.Helper()
	sums, err := release.ParseBundleSums(mustRead(t, filepath.Join(dir, "SHA256SUMS")), e.Version)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range release.Targets {
		name, err := selectBundleNative(sums, e.Version, target)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateCandidateBundle(mustRead(t, filepath.Join(dir, name)), e, target, sums[name]); err != nil {
			t.Fatal(err)
		}
		entries, err := release.Unpack(mustRead(t, filepath.Join(dir, name)), strings.HasPrefix(target, "windows/"), release.MaxArchiveBytes)
		if err != nil {
			t.Fatal(err)
		}
		for _, product := range release.Products {
			binary := product
			if strings.HasPrefix(target, "windows/") {
				binary += ".exe"
			}
			info, err := buildinfo.Read(bytes.NewReader(entries[binary]))
			if err != nil {
				t.Fatal(binary, err)
			}
			if err := validateBinaryBuild(info, product, target, e); err != nil {
				t.Fatal(product, target, err)
			}
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 7 {
		t.Fatal("release requires seven assets", len(files), err)
	}
}

func TestReleaseCandidateNative(t *testing.T) {
	dir, e := releaseInputs(t)
	verifyReleaseCandidate(t, dir, e)
	target := runtime.GOOS + "/" + runtime.GOARCH
	base := t.TempDir()
	bins := map[string]string{}
	installedLicenses := map[string][]byte{}
	sums, err := release.ParseBundleSums(mustRead(t, filepath.Join(dir, "SHA256SUMS")), e.Version)
	if err != nil {
		t.Fatal(err)
	}
	name, err := selectBundleNative(sums, e.Version, target)
	if err != nil {
		t.Fatal(err)
	}
	_, entries, err := release.ValidateBundleArchive(mustRead(t, filepath.Join(dir, name)), e.Version, target, sums[name])
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range release.Products {
		name := product
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if product == "aidlc" || product == "okf" || product == "natural-japanese-go" {
			prefix := "LICENSES/" + product + "/"
			for p, data := range entries {
				if strings.HasPrefix(p, prefix) {
					installedLicenses[filepath.Join("aidlc/bin", e.Version, "licenses", product, strings.TrimPrefix(p, prefix))] = data
				}
			}
		}
		binary := filepath.Join(base, name)
		if err := os.WriteFile(binary, entries[name], 0755); err != nil {
			t.Fatal(err)
		}
		bins[product] = binary
	}
	t.Setenv("PATH", t.TempDir())
	for _, product := range release.Products {
		out := distributionOK(t, base, bins[product], "--version")
		if err := validateNativeVersion(out, product, e); err != nil {
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
	if len(result.Paths) != len(expected)+3+len(installedLicenses) {
		t.Fatal("installed count", len(result.Paths))
	}
	for _, a := range expected {
		if !bytes.Equal(mustRead(t, filepath.Join(root, a.Path)), a.Data) {
			t.Fatal("source mismatch", a.Path)
		}
	}
	for name, want := range installedLicenses {
		if got := mustRead(t, filepath.Join(root, name)); !bytes.Equal(got, want) {
			t.Fatal("installed license differs", name)
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
	for name, want := range installedLicenses {
		if got := mustRead(t, filepath.Join(moved, name)); !bytes.Equal(got, want) {
			t.Fatal("relocated license differs", name)
		}
	}
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
	t.Logf("same seven-asset candidate: five native CLIs, fresh install, role references, preservation and same-version relocation on %s; no live Codex execution", target)
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

func validateBinaryBuild(info *debug.BuildInfo, product, target string, e releaseExpectation) error {
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if info.GoVersion != e.GoVersion || settings["GOOS"]+"/"+settings["GOARCH"] != target {
		return fmt.Errorf("binary toolchain/target differs")
	}
	if info.Path != "github.com/sori883/ai-dd/src/cmd/"+product || settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" {
		return fmt.Errorf("binary product/build settings differ")
	}
	return nil
}

func validateNativeVersion(raw []byte, product string, e releaseExpectation) error {
	want := product + " " + e.Version
	if product != "natural-japanese-go" {
		want += " (commit " + e.Commit + ")"
	}
	if string(raw) != want+"\n" {
		return fmt.Errorf("native version output differs for %s", product)
	}
	return nil
}
