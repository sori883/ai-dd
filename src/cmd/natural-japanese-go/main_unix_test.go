//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandFIFO(t *testing.T) {
	if path := os.Getenv("NATURAL_JAPANESE_FIFO_TEST"); path != "" {
		args := []string{path}
		if os.Getenv("NATURAL_JAPANESE_FIFO_BASELINE") == "1" {
			args = []string{"--baseline", path, "-"}
		}
		os.Exit(run(args, strings.NewReader("猫。"), os.Stdout, os.Stderr))
	}
	for _, baseline := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "baseline"}[baseline], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pipe")
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			// Bound a blocked FIFO read while allowing process startup under
			// the race detector. Both inputs must be rejected before analysis.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCommandFIFO$")
			cmd.Env = append(os.Environ(), "NATURAL_JAPANESE_FIFO_TEST="+path)
			if baseline {
				cmd.Env = append(cmd.Env, "NATURAL_JAPANESE_FIFO_BASELINE=1")
			}
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			err := cmd.Run()
			if ctx.Err() != nil {
				t.Fatalf("command did not reject FIFO within 30 seconds: stderr=%s", &stderr)
			}
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || out.Len() != 0 {
				t.Fatalf("err=%v stdout=%s stderr=%s", err, &out, &stderr)
			}
		})
	}
}
