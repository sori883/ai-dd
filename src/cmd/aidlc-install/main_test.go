package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallerCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{{"help", []string{"--help"}, 0}, {"version", []string{"--version"}, 0}, {"duplicate release", []string{"codex", "--release-version", "v1", "--release-version", "v2"}, 2}, {"missing release", []string{"codex"}, 2}, {"unknown host", []string{"claude", "--release-version", "v0.1.1"}, 2}, {"old install prefix", []string{"install", "codex"}, 2}, {"unknown URL", []string{"codex", "--release-version", "v0.1.1", "--url", "https://x"}, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			got := run(tc.args, &out, &err)
			if got != tc.code {
				t.Fatalf("exit %d want %d: %s", got, tc.code, &err)
			}
			if tc.code == 0 && !strings.Contains(out.String(), "aidlc-install") {
				t.Fatal("missing CLI identity")
			}
		})
	}
}
