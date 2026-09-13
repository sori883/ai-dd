//go:build integration

package bootstrap

import (
	"bytes"
	"encoding/json"
	"github.com/sori883/ai-dd/src/core"
	"github.com/sori883/ai-dd/src/harness/codex"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Only HTTP acquisition is replaced. The installer and runtime bytes are the
// unchanged release candidate transferred from the packaging job.
func TestBootstrapCandidateNative(t *testing.T) {
	dir, version := os.Getenv("AIDLC_DIST_DIR"), os.Getenv("AIDLC_RELEASE_VERSION")
	if dir == "" && version == "" {
		t.Skip("set candidate directory and version")
	}
	if dir == "" || !release.ValidVersion(version) {
		t.Fatal("candidate inputs required")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	shells := []string{"sh"}
	script := "install.sh"
	curl := "curl"
	if runtime.GOOS == "windows" {
		shells = []string{"powershell.exe", "pwsh.exe"}
		script = "install.ps1"
		curl = "curl.exe"
	}
	script, err = filepath.Abs(script)
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "project space 日本語")
			bin := filepath.Join(base, "bin")
			os.Mkdir(root, 0700)
			os.Mkdir(bin, 0700)
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, curl), helper, 0700); err != nil {
				t.Fatal(err)
			}
			calls := filepath.Join(base, "calls")
			args := []string{script, version, root}
			if runtime.GOOS == "windows" {
				args = append([]string{"-NoProfile", "-NonInteractive", "-File"}, args...)
			}
			cmd := exec.Command(shell, args...)
			sentinel := filepath.Join(base, "sentinel")
			if runtime.GOOS == "windows" {
				cmd = powerShellBootstrapCommand(shell, script, version, root, "scriptblock", sentinel)
			}
			if cmd.Env == nil {
				cmd.Env = os.Environ()
			}
			cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "BOOTSTRAP_HELPER=1", "BOOTSTRAP_FIXTURE="+dir, "BOOTSTRAP_CALLS="+calls, "BOOTSTRAP_VERSION="+version)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("native bootstrap failed: %v %s", err, out)
			}
			if runtime.GOOS == "windows" {
				got, err := os.ReadFile(sentinel)
				if err != nil || string(got) != "0" {
					t.Fatal("public caller did not resume successfully", string(got), err)
				}
			}
			var result struct{ Paths []string }
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal("installer output", err, string(out))
			}
			name := release.BundleName(version, runtime.GOOS+"/"+runtime.GOARCH)
			got, err := os.ReadFile(calls)
			if err != nil || string(got) != "SHA256SUMS\n"+name+"\n" {
				t.Fatal("bootstrap downloaded other assets", string(got), err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			_, entries, err := release.ValidateBundleArchive(raw, version, runtime.GOOS+"/"+runtime.GOARCH, release.Hash(raw))
			if err != nil {
				t.Fatal(err)
			}
			suffix := ""
			if runtime.GOOS == "windows" {
				suffix = ".exe"
			}
			binary := func(product string) string { return filepath.Join(root, "aidlc/bin", version, product+suffix) }
			assets, err := codex.DistributionFrom(root, codex.Binaries{AIDLC: binary("aidlc"), OKF: binary("okf"), Natural: binary("natural-japanese-go")}, core.Files, codex.Files)
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string][]byte{}
			for _, a := range assets {
				expected[a.Path] = a.Data
			}
			for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
				expected[filepath.Join("aidlc/bin", version, product+suffix)] = entries[product+suffix]
				prefix := "LICENSES/" + product + "/"
				for p, b := range entries {
					if strings.HasPrefix(p, prefix) {
						expected[filepath.Join("aidlc/bin", version, "licenses", product, strings.TrimPrefix(p, prefix))] = b
					}
				}
			}
			if len(result.Paths) != len(expected) {
				t.Fatal("unexpected installed files", len(result.Paths), len(expected))
			}
			for p, want := range expected {
				got, err := os.ReadFile(filepath.Join(root, p))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("installed bytes differ", p, err)
				}
			}
			for _, product := range []string{"aidlc", "okf", "natural-japanese-go"} {
				out, err := exec.Command(binary(product), "--version").CombinedOutput()
				if err != nil || !bytes.Contains(out, []byte(version)) {
					t.Fatal(product, string(out), err)
				}
			}
			t.Log("same candidate bootstrap and exact installed sources/runtimes/licenses verified; only HTTP download was simulated")
		})
	}
}
