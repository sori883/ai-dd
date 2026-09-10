// Command aidlc-dist packages locally built aidlc binaries. It never publishes them.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `Usage: aidlc-dist --input-dir DIR --output-dir NEW_DIR --version VERSION --commit SHA --go-version GO_VERSION [--targets OS/ARCH,...]

Developer-only local packaging; no upload or release is performed.
--input-dir    Built aidlc-OS-ARCH binaries (.exe for Windows)
--output-dir   A directory that does not exist yet
--version      Safe candidate version, for example dev-abcdef0
--commit       40 lowercase hexadecimal source commit digits
--go-version   Go toolchain version, for example go1.26.4
--targets      Nonempty subset of darwin/linux/windows x amd64/arm64; defaults to all six
Exit codes: 0 success/help, 2 invalid arguments, 1 filesystem or output failure.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	var o options
	fs := flag.NewFlagSet("aidlc-dist", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
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
