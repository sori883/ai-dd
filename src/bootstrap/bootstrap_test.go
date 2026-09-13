package bootstrap

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/release"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("BOOTSTRAP_STREAM_HELPER") == "1" {
		fmt.Fprintln(os.Stdout, `{"Paths":[]}`)
		fmt.Fprintln(os.Stderr, "#< CLIXML progress")
		code, _ := strconv.Atoi(os.Getenv("BOOTSTRAP_EXIT"))
		os.Exit(code)
	}
	if os.Getenv("BOOTSTRAP_HELPER") == "1" {
		name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
		switch name {
		case "uname":
			if os.Getenv("BOOTSTRAP_ARCH_FAIL") == "1" {
				fmt.Println("unsupported")
				os.Exit(0)
			}
			if os.Args[1] == "-s" {
				fmt.Print("Linux\n")
			} else {
				fmt.Print("x86_64\n")
			}
			os.Exit(0)
		case "curl":
			args := os.Args[1:]
			if len(args) == 0 || args[0] != "--disable" {
				os.Exit(91)
			}
			var output, url string
			for i, a := range args {
				if a == "--output" {
					output = args[i+1]
				}
				if strings.HasPrefix(a, "https://") {
					url = a
				}
			}
			version := os.Getenv("BOOTSTRAP_VERSION")
			if version == "" {
				version = "v0.1.2"
			}
			if !strings.HasPrefix(url, "https://github.com/sori883/ai-dd/releases/download/"+version+"/") {
				os.Exit(92)
			}
			b, e := os.ReadFile(filepath.Join(os.Getenv("BOOTSTRAP_FIXTURE"), filepath.Base(url)))
			if e != nil {
				os.Exit(22)
			}
			if output == "-" {
				_, e = os.Stdout.Write(b)
			} else {
				e = os.WriteFile(output, b, 0600)
			}
			if e != nil {
				os.Exit(93)
			}
			f, _ := os.OpenFile(os.Getenv("BOOTSTRAP_CALLS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			fmt.Fprintln(f, filepath.Base(url))
			f.Close()
			os.Exit(0)
		case "aidlc-install":
			os.WriteFile(os.Getenv("BOOTSTRAP_RESULT"), []byte(strings.Join(os.Args[1:], "\n")), 0600)
			code, _ := strconv.Atoi(os.Getenv("BOOTSTRAP_EXIT"))
			os.Exit(code)
		}
	}
	os.Exit(m.Run())
}

func TestBootstrap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows scenarios use TestBootstrapPowerShell")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "checksum", "missing", "version", "duplicate", "symlink", "nested", "project", "architecture", "exit"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			fixture := filepath.Join(base, "fixture")
			bin := filepath.Join(base, "bin")
			project := filepath.Join(base, "project space 日本語")
			for _, p := range []string{fixture, bin, project} {
				os.Mkdir(p, 0700)
			}
			for _, name := range []string{"curl", "uname"} {
				if err := os.WriteFile(filepath.Join(bin, name), binary, 0700); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := release.Archive(map[string][]byte{"aidlc-install": binary}, "aidlc-install", false)
			if err != nil {
				t.Fatal(err)
			}
			// Duplicate/link fixtures are formed independently of the safe archive writer.
			if mode == "nested" {
				raw, err = release.Archive(map[string][]byte{"aidlc-install/evil": binary}, "aidlc-install/evil", false)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "duplicate" || mode == "symlink" {
				raw = unsafeInstaller(t, binary, mode)
			}
			name := "ai-dd_v0.1.2_linux_amd64.tar.gz"
			digest := release.Hash(raw)
			if mode == "checksum" {
				digest = strings.Repeat("0", 64)
			}
			var sums []string
			for _, target := range release.Targets {
				hash := strings.Repeat("a", 64)
				if target == "linux/amd64" {
					hash = digest
				}
				sums = append(sums, hash+"  "+release.BundleName("v0.1.2", target))
			}
			os.WriteFile(filepath.Join(fixture, "SHA256SUMS"), []byte(strings.Join(sums, "\n")+"\n"), 0600)
			if mode != "missing" {
				os.WriteFile(filepath.Join(fixture, name), raw, 0600)
			}
			version := "v0.1.2"
			if mode == "version" {
				version = "../bad"
			}
			project, err = filepath.EvalSymlinks(project)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "project" {
				project = filepath.Join(project, "missing")
			}
			result := filepath.Join(base, "result")
			calls := filepath.Join(base, "calls")
			cmd := exec.Command("sh", "install.sh", version, project)
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "BOOTSTRAP_HELPER=1", "BOOTSTRAP_FIXTURE="+fixture, "BOOTSTRAP_RESULT="+result, "BOOTSTRAP_CALLS="+calls)
			if mode == "architecture" {
				cmd.Env = append(cmd.Env, "BOOTSTRAP_ARCH_FAIL=1")
			}
			if mode == "exit" {
				cmd.Env = append(cmd.Env, "BOOTSTRAP_EXIT=17")
			}
			out, err := cmd.CombinedOutput()
			if mode != "valid" && mode != "exit" {
				if err == nil {
					t.Fatal("invalid bootstrap succeeded", mode)
				}
				if _, e := os.Stat(result); !os.IsNotExist(e) {
					t.Fatal("invalid installer executed")
				}
				return
			}
			if mode == "exit" {
				if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 17 {
					t.Fatal("installer exit code lost", err)
				}
				err = nil
			}
			if err != nil {
				t.Fatalf("bootstrap failed: %v %s", err, out)
			}
			got, _ := os.ReadFile(result)
			args := strings.Split(string(got), "\n")
			if len(args) != 7 || args[0] != "codex" || args[1] != "--release-version" || args[2] != version || args[3] != "--project-dir" || args[4] != project || args[5] != "--release-dir" {
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

func TestBootstrapOutputStreams(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{0, 17} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			cmd := exec.Command(exe)
			cmd.Env = append(os.Environ(), "BOOTSTRAP_STREAM_HELPER=1", "BOOTSTRAP_EXIT="+strconv.Itoa(code))
			stdout, stderr, err := bootstrapCommandOutput(cmd)
			if string(stdout) != "{\"Paths\":[]}\n" || !strings.HasPrefix(string(stderr), "#< CLIXML progress\n") || strings.Contains(string(stderr), `{"Paths":[]}`) {
				t.Fatalf("output streams mixed: stdout=%q stderr=%q", stdout, stderr)
			}
			if code == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != code {
				t.Fatal("failure status lost", err)
			}
		})
	}
}

func TestBootstrapProjectDirectory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project space 日本語")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	missing := filepath.Join(project, "missing")
	for _, tc := range []struct {
		name, actual, expected string
		want                   bool
	}{
		{"same", project, project, true}, {"alternate spelling", project + string(os.PathSeparator) + ".", project, true}, {"different", other, project, false}, {"missing actual", missing, project, false}, {"missing expected", project, missing, false}, {"both missing", missing, missing, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameProjectDirectory(tc.actual, tc.expected); got != tc.want {
				t.Fatalf("same directory=%v want=%v", got, tc.want)
			}
		})
	}
}
