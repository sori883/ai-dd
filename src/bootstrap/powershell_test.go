package bootstrap

import (
	"archive/zip"
	"bytes"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
			for _, invocation := range []string{"file", "scriptblock"} {
				for _, mode := range []string{"valid", "exit", "checksum", "missing", "version", "duplicate", "symlink"} {
					t.Run(invocation+"/"+mode, func(t *testing.T) {
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
						script, err := filepath.Abs("install.ps1")
						if err != nil {
							t.Fatal(err)
						}
						sentinel := filepath.Join(base, "sentinel")
						cmd := powerShellBootstrapCommand(shell, script, version, project, invocation, sentinel)
						if cmd.Env == nil {
							cmd.Env = os.Environ()
						}
						cmd.Env = append(cmd.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "BOOTSTRAP_HELPER=1", "BOOTSTRAP_FIXTURE="+fixture, "BOOTSTRAP_RESULT="+result, "BOOTSTRAP_CALLS="+calls)
						want := 0
						if mode == "exit" {
							want = 17
							cmd.Env = append(cmd.Env, "BOOTSTRAP_EXIT=17")
						} else if mode != "valid" {
							want = 1
						}
						out, err := cmd.CombinedOutput()
						gotCode := 0
						if err != nil {
							e, ok := err.(*exec.ExitError)
							if !ok {
								t.Fatal(err)
							}
							gotCode = e.ExitCode()
						}
						if gotCode != want {
							t.Fatalf("exit=%d want=%d: %s", gotCode, want, out)
						}
						if invocation == "scriptblock" {
							got, err := os.ReadFile(sentinel)
							if err != nil || string(got) != strconv.Itoa(want) {
								t.Fatalf("caller did not resume with LASTEXITCODE=%d: %q %v", want, got, err)
							}
						}
						if mode != "valid" && mode != "exit" {
							if _, e := os.Stat(result); !os.IsNotExist(e) {
								t.Fatal("invalid installer executed")
							}
							return
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
			}
		})
	}
}
