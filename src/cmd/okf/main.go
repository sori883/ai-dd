package main

import (
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"io"
	"os"
)

func run(args []string, stdout, stderr io.Writer) int {
	return okfcli.Run(args, stdout, stderr, buildinfo.Current(), okfcli.Dependencies{Execute: executeCommand})
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
