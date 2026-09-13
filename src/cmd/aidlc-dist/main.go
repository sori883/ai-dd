// Command aidlc-dist packages locally built aidlc binaries. It never publishes them.
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"io"
	"os"
	"strings"
)

const usage = `使い方: aidlc-dist --input-dir DIR --output-dir NEW_DIR --version VERSION --commit SHA --go-version GO_VERSION [--targets OS/ARCH,...] --license-dir DIR

開発者向けにローカルで配布候補を梱包します。アップロードや公開は行いません。
--license-dir  同じbuildの独自MIT・Go・YAML等の許諾入力。5製品と資材を6一式archive＋SHA256SUMSの7件へ梱包
--input-dir    ビルド済みの PRODUCT-OS-ARCH を置いたディレクトリ（Windowsは末尾に .exe）
--output-dir   出力先の新しいディレクトリ（既存のディレクトリは指定不可）
--version      配布候補の版名。例: dev-abcdef0
--commit       ソースのコミットID（小文字の16進数40桁）
--go-version   ビルドに使用したGoの版。例: go1.26.4
--targets      Must contain all six darwin/linux/windows x amd64/arm64 targets; default is all six.
終了コード: 0は成功またはヘルプ表示、2は引数の不正、1はファイル操作や出力の失敗。
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		info := buildinfo.Current()
		if _, err := fmt.Fprintf(stdout, "aidlc-dist %s (commit %s)\n", info.Version, info.Commit); err != nil {
			return 1
		}
		return 0
	}
	var o options
	fs := flag.NewFlagSet("aidlc-dist", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o.Product = "all"
	fs.StringVar(&o.LicenseDir, "license-dir", "", "")
	fs.StringVar(&o.InputDir, "input-dir", "", "")
	fs.StringVar(&o.OutputDir, "output-dir", "", "")
	fs.StringVar(&o.Version, "version", "", "")
	fs.StringVar(&o.Commit, "commit", "", "")
	fs.StringVar(&o.GoVersion, "go-version", "", "")
	targets := fs.String("targets", strings.Join(supportedTargets, ","), "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := io.WriteString(stdout, usage); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "positional arguments are not supported")
		return 2
	}
	o.Targets = strings.Split(*targets, ",")
	if err := packageArchives(o); err != nil {
		fmt.Fprintln(stderr, "aidlc-dist:", err)
		if errors.Is(err, errInvalidInput) {
			return 2
		}
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "Created distribution candidate: %s\n", o.OutputDir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
