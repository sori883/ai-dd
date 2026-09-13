package okfcli

import (
	"errors"
	"fmt"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"io"
	"io/fs"
)

func Run(args []string, stdout, stderr io.Writer, info buildinfo.Info, deps Dependencies) int {
	write := func(text string) int {
		if _, err := io.WriteString(stdout, text); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if text, ok := Help(args); ok {
		return write(text)
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		return write(fmt.Sprintf("okf %s (commit %s)\n", info.Version, info.Commit))
	}
	r, err := ParseCommand(args)
	if err != nil {
		fmt.Fprintln(stderr, "okf:", err)
		if len(args) > 0 {
			if _, ok := Help([]string{args[0], "--help"}); ok {
				fmt.Fprintf(stderr, "使い方: okf %s --help\n", args[0])
			}
		}
		return 2
	}
	if deps.Execute == nil {
		fmt.Fprintln(stderr, "okf: command runtime unavailable")
		return 1
	}
	out, err := deps.Execute(r)
	if len(out) > 0 {
		if _, e := stdout.Write(out); e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "okf:", err)
		if errors.Is(err, fs.ErrInvalid) || errors.Is(err, fs.ErrExist) {
			return 2
		}
		return 1
	}
	return 0
}
