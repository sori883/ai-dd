// Command aidlc provides the AI-DLC command-line entry point.
package main

import (
	"os"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, buildinfo.Current()))
}
