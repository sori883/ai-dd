//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var fixtureBinary struct {
	once       sync.Once
	dir, aidlc string
	err        error
}

func buildAIDLCBinary(t *testing.T) string {
	t.Helper()
	fixtureBinary.once.Do(func() {
		fixtureBinary.dir, fixtureBinary.err = os.MkdirTemp("", "aidlc-test-binaries-")
		if fixtureBinary.err != nil {
			return
		}
		directory, err := filepath.EvalSymlinks(fixtureBinary.dir)
		if err != nil {
			fixtureBinary.err = err
			return
		}
		fixtureBinary.dir = directory

		suffix := ""
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, product := range []string{"aidlc", "okf"} {
			target := filepath.Join(fixtureBinary.dir, product+suffix)
			out, err := exec.CommandContext(ctx, "go", "build", "-o", target, "../"+product).CombinedOutput()
			if err != nil {
				fixtureBinary.err = fmt.Errorf("build %s: %w: %s", product, err, out)
				return
			}
		}
		fixtureBinary.aidlc = filepath.Join(fixtureBinary.dir, "aidlc"+suffix)
	})
	if fixtureBinary.err != nil {
		t.Fatal(fixtureBinary.err)
	}
	return fixtureBinary.aidlc
}

func TestMain(m *testing.M) {
	code := m.Run()
	if fixtureBinary.dir != "" {
		if err := os.RemoveAll(fixtureBinary.dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}
