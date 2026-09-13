package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/projectroot"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const usage = `aidlc-install — Codex向け新規導入
使い方: aidlc-install codex --release-version VERSION [--project-dir ROOT] [--release-dir DIR]
指定したGitHub Releaseの3 runtimeと同版資材を検査してaidlc/bin/VERSIONへ配置します。
--release-dirは公開前候補／オフライン入力。通信と同じ検査を通します。
既存fileは上書きしません。失敗時はPathsとPendingを確認し、部分配置を成功扱いしないでください。
移転: 同じ版で --relocate --from-project-dir OLD_ROOT --from-binary OLD_AIDLC。3 runtimeのbytesを照合して既知の参照だけ更新します。
版表示: aidlc-install --version（導入対象は--release-versionで指定）。
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return 1
		}
		return 0
	}
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		info := buildinfo.Current()
		if _, err := fmt.Fprintf(stdout, "aidlc-install %s (commit %s)\n", info.Version, info.Commit); err != nil {
			return 1
		}
		return 0
	}
	if args[0] != "codex" {
		fmt.Fprintln(stderr, "unknown target; use aidlc-install codex --help")
		return 2
	}
	seen := map[string]bool{}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--") {
			name := strings.SplitN(arg, "=", 2)[0]
			if seen[name] {
				fmt.Fprintln(stderr, "duplicate flag", name)
				return 2
			}
			seen[name] = true
		}
	}
	var o install.ReleaseOptions
	f := flag.NewFlagSet("aidlc-install", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&o.Root, "project-dir", "", "")
	f.StringVar(&o.Version, "release-version", "", "")
	f.StringVar(&o.Directory, "release-dir", "", "")
	f.BoolVar(&o.Relocate, "relocate", false, "")
	f.StringVar(&o.FromProjectDir, "from-project-dir", "", "")
	f.StringVar(&o.FromBinary, "from-binary", "", "")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			io.WriteString(stdout, usage)
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	if (!o.Relocate && (o.FromProjectDir != "" || o.FromBinary != "")) || (o.Relocate && (o.FromProjectDir == "" || o.FromBinary == "")) {
		fmt.Fprintln(stderr, "--relocate requires --from-project-dir and --from-binary")
		return 2
	}
	if f.NArg() != 0 || o.Version == "" {
		fmt.Fprintln(stderr, "--release-version required; positional arguments unsupported")
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	o.Root, err = projectroot.Resolve(o.Root, cwd, true)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := install.InstallRelease(ctx, o)
	if len(result.Paths) > 0 || len(result.Pending) > 0 || err == nil {
		raw, e := install.EncodeReleaseResult(result)
		if e != nil {
			return 1
		}
		if _, e = fmt.Fprintln(stdout, string(raw)); e != nil {
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "aidlc-install:", err)
		if errors.Is(err, fs.ErrInvalid) || errors.Is(err, fs.ErrExist) {
			return 2
		}
		return 1
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
