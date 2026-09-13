package bootstrap

import (
	"archive/zip"
	"bytes"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBootstrapPowerShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell 5.1 and 7 execute in Windows CI")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range []string{"powershell.exe", "pwsh.exe"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Fatal("required Windows verification shell missing", err)
			}
			for _, mode := range []string{"valid", "checksum", "missing", "version", "duplicate", "symlink"} {
				t.Run(mode, func(t *testing.T) {
					base := t.TempDir()
					fixture := filepath.Join(base, "fixture")
					bin := filepath.Join(base, "bin")
					project := filepath.Join(base, "project space 日本語")
					for _, p := range []string{fixture, bin, project} {
						os.Mkdir(p, 0700)
					}
					os.WriteFile(filepath.Join(bin, "curl.exe"), binary, 0700)
					raw, err := release.Archive(map[string][]byte{"aidlc-install.exe": binary}, "aidlc-install.exe", true)
					if err != nil {
						t.Fatal(err)
					}
					if mode == "duplicate" || mode == "symlink" {
						var b bytes.Buffer
						w := zip.NewWriter(&b)
						for i := 0; i < 2; i++ {
							h := &zip.FileHeader{Name: "aidlc-install.exe", Method: zip.Deflate}
							h.SetMode(0755)
							if mode == "symlink" {
								h.SetMode(os.ModeSymlink | 0755)
							}
							f, e := w.CreateHeader(h)
							if e != nil {
								t.Fatal(e)
							}
							f.Write(binary)
							if mode == "symlink" {
								break
							}
						}
						w.Close()
						raw = b.Bytes()
					}
					target := "windows/" + runtime.GOARCH
					name := release.BundleName("v0.1.2", target)
					digest := release.Hash(raw)
					if mode == "checksum" {
						digest = strings.Repeat("0", 64)
					}
					var sums []string
					for _, platform := range release.Targets {
						hash := strings.Repeat("a", 64)
						if platform == target {
							hash = digest
						}
						sums = append(sums, hash+"  "+release.BundleName("v0.1.2", platform))
					}
					os.WriteFile(filepath.Join(fixture, "SHA256SUMS"), []byte(strings.Join(sums, "\n")+"\n"), 0600)
					if mode != "missing" {
						os.WriteFile(filepath.Join(fixture, name), raw, 0600)
					}
					version := "v0.1.2"
					if mode == "version" {
						version = "../bad"
					}
					result := filepath.Join(base, "result")
					calls := filepath.Join(base, "calls")
					// The script is ASCII, so -File and the public completed-download scriptblock have identical bytes.
					cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", "install.ps1", version, project)
					cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "BOOTSTRAP_HELPER=1", "BOOTSTRAP_FIXTURE="+fixture, "BOOTSTRAP_RESULT="+result, "BOOTSTRAP_CALLS="+calls)
					out, err := cmd.CombinedOutput()
					if mode != "valid" {
						if err == nil {
							t.Fatal("invalid bootstrap succeeded", mode)
						}
						if _, e := os.Stat(result); !os.IsNotExist(e) {
							t.Fatal("invalid installer executed")
						}
						return
					}
					if err != nil {
						t.Fatalf("bootstrap failed: %v %s", err, out)
					}
					got, _ := os.ReadFile(result)
					args := strings.Split(string(got), "\n")
					if len(args) != 7 || args[0] != "codex" || args[1] != "--release-version" || args[2] != version || args[3] != "--project-dir" || !strings.EqualFold(args[4], project) || args[5] != "--release-dir" {
						t.Fatal("arguments changed", args)
					}
					if _, err := os.Stat(args[6]); !os.IsNotExist(err) {
						t.Fatal("temporary directory retained")
					}
					got, _ = os.ReadFile(calls)
					if string(got) != "SHA256SUMS\n"+name+"\n" {
						t.Fatal("downloads repeated", string(got))
					}
				})
			}
		})
	}
}
