package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/naturaljapanese"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

var version = "dev"
var categories = naturaljapanese.Categories

const usage = `natural-japanese-go [--json] [--genre essay|tech|business] [--baseline PREVIOUS.json] FILE
FILE はUTF-8通常ファイル、- は標準入力。flagはFILEの前後で指定できます。
--list-rules [--json] 検査カテゴリを表示
--help 利用方法 / --version 版
severity: info, warn, critical。短文は統計検査の最低量に届かない場合があります。
実験的検査・reading-load・semanticは対象外です。検出なしは内容の正しさを保証しません。
終了コード: 0 正常（指摘ありを含む）、1 入出力・解析・baselineエラー、2 引数不正
`

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
func run(args []string, in io.Reader, out, errout io.Writer) int {
	fs := flag.NewFlagSet("natural-japanese-go", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var jsonOutput, help, showVersion, list bool
	var genre, baseline string
	fs.BoolVar(&jsonOutput, "json", false, "")
	fs.BoolVar(&help, "help", false, "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&list, "list-rules", false, "")
	fs.StringVar(&genre, "genre", "", "")
	fs.StringVar(&baseline, "baseline", "", "")
	// Move positional input behind flags while keeping each flag value attached.
	var flags, files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if (a == "--genre" || a == "-genre" || a == "--baseline" || a == "-baseline") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else {
			files = append(files, a)
		}
	}
	if err := fs.Parse(flags); err != nil {
		fmt.Fprintf(errout, "引数が不正です: %v（--helpで利用方法を確認）\n", err)
		return 2
	}
	if genre != "" && genre != "essay" && genre != "tech" && genre != "business" {
		fmt.Fprintf(errout, "genre %q は使用できません。essay、tech、businessを指定してください。\n", genre)
		return 2
	}
	modes := 0
	for _, v := range []bool{help, showVersion, list} {
		if v {
			modes++
		}
	}
	if modes > 1 || (modes > 0 && (len(files) > 0 || genre != "" || baseline != "")) || (modes == 0 && len(files) != 1) {
		fmt.Fprintln(errout, "引数の組合せが不正です。検査にはFILEを1つ指定してください。利用方法は--helpで確認できます。")
		return 2
	}
	write := func(text string) int {
		if _, err := io.WriteString(out, text); err != nil {
			fmt.Fprintf(errout, "出力に失敗しました: %v\n", err)
			return 1
		}
		return 0
	}
	if help {
		return write(usage)
	}
	if showVersion {
		return write("natural-japanese-go " + version + "\n")
	}
	if list {
		if jsonOutput {
			if err := json.NewEncoder(out).Encode(categories); err != nil {
				fmt.Fprintf(errout, "出力に失敗しました: %v\n", err)
				return 1
			}
			return 0
		}
		return write(usage + strings.Join(categories, "\n") + "\n")
	}
	var raw []byte
	var err error
	if files[0] == "-" {
		raw, err = io.ReadAll(in)
	} else {
		raw, err = readRegularFile(files[0])
	}
	if err != nil {
		fmt.Fprintf(errout, "入力 %q の読込みに失敗しました: %v\n", files[0], err)
		return 1
	}
	if !utf8.Valid(raw) {
		fmt.Fprintf(errout, "入力 %q は有効なUTF-8ではありません。\n", files[0])
		return 1
	}

	report, err := naturaljapanese.Check(string(raw), files[0], genre)
	if err != nil {
		fmt.Fprintf(errout, "解析に失敗しました: %v\n", err)
		return 1
	}
	if baseline != "" {
		var previous []byte
		previous, err = readRegularFile(baseline)
		if err == nil {
			report, err = naturaljapanese.Compare(report, previous)
		}
	}
	if err != nil {
		fmt.Fprintf(errout, "baseline %q の処理に失敗しました: %v\n", baseline, err)
		return 1
	}
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(report); err != nil {
			fmt.Fprintf(errout, "出力に失敗しました: %v\n", err)
			return 1
		}
		return 0
	}
	var result strings.Builder
	fmt.Fprintf(&result, "検査結果: %d件（検出なしは内容の正しさを保証しません）\n", len(report.Findings))
	for _, f := range report.Findings {
		fmt.Fprintf(&result, "%d: [%s] %s %s\n  %s\n", f.Line, f.Severity, f.Category, f.Excerpt, f.Detail)
	}
	if report.Comparison != nil {
		fmt.Fprintf(&result, "比較: 新規%d 継続%d 解消%d\n", report.Comparison["new"], report.Comparison["persisting"], report.Comparison["resolved"])
	}
	return write(result.String())
}

// readRegularFile rejects special files before opening them, then verifies the opened file.
func readRegularFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: 通常ファイルではありません", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("%s: 通常ファイルではありません", path)
	}
	var raw []byte
	if err == nil {
		raw, err = io.ReadAll(f)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return raw, err
}
