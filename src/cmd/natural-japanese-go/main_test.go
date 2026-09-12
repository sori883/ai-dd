package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand(t *testing.T) {
	for _, tt := range []struct {
		name     string
		args     []string
		input    string
		code     int
		contains string
	}{
		{"help", []string{"--help"}, "", 0, "終了コード"},
		{"version", []string{"--version"}, "", 0, "natural-japanese-go"},
		{"rules", []string{"--list-rules", "--json"}, "", 0, "low_specificity"},
		{"stdin", []string{"-", "--json"}, "猫が走る。", 0, "schema_version"},
		{"missing", nil, "", 2, ""}, {"flag", []string{"--semantic", "-"}, "", 2, ""},
		{"genre", []string{"--genre", "unknown", "-"}, "", 2, ""},
		{"unreadable", []string{"/does-not-exist"}, "", 1, ""},
		{"invalid utf8", []string{"-"}, string([]byte{255}), 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			code := run(tt.args, strings.NewReader(tt.input), &out, &errout)
			if code != tt.code {
				t.Fatalf("exit=%d want=%d stderr=%s", code, tt.code, &errout)
			}
			if !strings.Contains(out.String(), tt.contains) {
				t.Fatalf("stdout=%s want %s", &out, tt.contains)
			}
		})
	}
}
func TestCommandFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "text.md")
	raw := []byte("猫が走る。")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"--json", path}, strings.NewReader(""), &out, io.Discard); code != 0 {
		t.Fatal(code)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, after) {
		t.Fatal("modified input")
	}
	if code := run([]string{dir}, strings.NewReader(""), io.Discard, io.Discard); code != 1 {
		t.Fatal("accepted directory", code)
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestCommandOutputError(t *testing.T) {
	if code := run([]string{"--help"}, strings.NewReader(""), failedWriter{}, io.Discard); code != 1 {
		t.Fatal(code)
	}
}
func TestCommandAnalysis(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--json", "-"}, strings.NewReader("非常に重要。"), &out, io.Discard); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "forbidden_phrase") {
		t.Fatal(out.String())
	}
	baseline := filepath.Join(t.TempDir(), "previous.json")
	if err := os.WriteFile(baseline, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"-", "--baseline", baseline, "--json"}, strings.NewReader("非常に重要。"), &out, io.Discard); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(out.String(), "persisting") {
		t.Fatal(out.String())
	}
}
func TestCommandBaselineError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"--baseline", path, "-"}, strings.NewReader("猫。"), io.Discard, &stderr); code != 1 || stderr.Len() == 0 {
		t.Fatal(code, stderr.String())
	}
}
