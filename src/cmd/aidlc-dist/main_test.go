package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commandArgs(o options) []string {
	return []string{"--input-dir", o.InputDir, "--output-dir", o.OutputDir, "--version", o.Version, "--commit", o.Commit, "--go-version", o.GoVersion}
}
func TestDistCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"long help", []string{"--help"}}, {"short help", []string{"-h"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			if code := run(tc.args, &out, &errs); code != 0 || !strings.Contains(out.String(), "--targets") || errs.Len() != 0 {
				t.Fatalf("help code/output: %d %q %q", code, out.String(), errs.String())
			}
		})
	}
	for _, tc := range []struct {
		name  string
		extra []string
		count int
	}{
		{"default six", nil, 8}, {"native subset", []string{"--targets", "linux/arm64,windows/amd64"}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := archiveFixture(t)
			var out, errs bytes.Buffer
			if code := run(append(commandArgs(o), tc.extra...), &out, &errs); code != 0 || errs.Len() != 0 {
				t.Fatalf("run=%d %s", code, errs.String())
			}
			files, err := os.ReadDir(o.OutputDir)
			if err != nil || len(files) != tc.count || !strings.Contains(out.String(), o.OutputDir) {
				t.Fatalf("output=%q files=%v error=%v", out.String(), files, err)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		extra []string
		empty bool
	}{
		{"missing args", nil, true}, {"unknown flag", []string{"--publish"}, false}, {"positional", []string{"extra"}, false}, {"empty targets", []string{"--targets="}, false}, {"duplicate target", []string{"--targets", "linux/amd64,linux/amd64"}, false}, {"unknown target", []string{"--targets", "plan9/amd64"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := archiveFixture(t)
			args := append(commandArgs(o), tc.extra...)
			if tc.empty {
				args = nil
			}
			var out, errs bytes.Buffer
			if code := run(args, &out, &errs); code != 2 || out.Len() != 0 || errs.Len() == 0 {
				t.Fatalf("invalid code/output: %d %q %q", code, out.String(), errs.String())
			}
			if _, err := os.Stat(o.OutputDir); !os.IsNotExist(err) {
				t.Fatal("invalid CLI created output", err)
			}
		})
	}
	t.Run("operational failure", func(t *testing.T) {
		o := archiveFixture(t)
		if err := os.Mkdir(o.OutputDir, 0700); err != nil {
			t.Fatal(err)
		}
		var out, errs bytes.Buffer
		if code := run(commandArgs(o), &out, &errs); code != 1 || out.Len() != 0 || errs.Len() == 0 {
			t.Fatalf("existing output result: %d %q %q", code, out.String(), errs.String())
		}
		files, err := os.ReadDir(filepath.Clean(o.OutputDir))
		if err != nil || len(files) != 0 {
			t.Fatal("changed existing output", err)
		}
	})
}
func TestProductCLI(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"--product", "bad"}, &out, io.Discard)
	if code != 2 {
		t.Fatal(code)
	}
}
