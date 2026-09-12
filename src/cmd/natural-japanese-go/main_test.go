package main

import (
	"bytes"
	"encoding/json"
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

func TestCommandBaselineExcerptRequired(t *testing.T) {
	for _, missing := range []bool{true, false} {
		var initial bytes.Buffer
		if code := run([]string{"--json", "-"}, strings.NewReader("非常に重要。"), &initial, io.Discard); code != 0 {
			t.Fatal(code)
		}
		var doc map[string]any
		if err := json.Unmarshal(initial.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
		f := doc["findings"].([]any)[0].(map[string]any)
		if missing {
			delete(f, "excerpt")
		} else {
			f["excerpt"] = ""
		}
		raw, _ := json.Marshal(doc)
		path := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		code := run([]string{"--baseline", path, "-"}, strings.NewReader("猫。"), &out, &stderr)
		if code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "excerpt") {
			t.Fatalf("missing=%v exit=%d stdout=%s stderr=%s", missing, code, &out, &stderr)
		}
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCommandDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		in      io.Reader
		out     io.Writer
		code    int
		message string
	}{
		{"missing", nil, strings.NewReader(""), nil, 2, "引数"},
		{"genre", []string{"--genre", "unknown", "-"}, strings.NewReader(""), nil, 2, "genre"},
		{"unknown flag", []string{"--semantic", "-"}, strings.NewReader(""), nil, 2, "引数"},
		{"missing file", []string{filepath.Join(t.TempDir(), "absent")}, strings.NewReader(""), nil, 1, "入力"},
		{"utf8", []string{"-"}, strings.NewReader(string([]byte{255})), nil, 1, "UTF-8"},
		{"read", []string{"-"}, failedReader{}, nil, 1, "入力"},
		{"baseline", []string{"--baseline", filepath.Join(t.TempDir(), "absent"), "-"}, strings.NewReader("猫。"), nil, 1, "baseline"},
		{"text write", []string{"--help"}, strings.NewReader(""), failedWriter{}, 1, "出力"},
		{"rules write", []string{"--json", "--list-rules"}, strings.NewReader(""), failedWriter{}, 1, "出力"},
		{"json write", []string{"--json", "-"}, strings.NewReader("猫。"), failedWriter{}, 1, "出力"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			out := tt.out
			if out == nil {
				out = &stdout
			}
			code := run(tt.args, tt.in, out, &stderr)
			if code != tt.code || !strings.Contains(stderr.String(), tt.message) || stdout.Len() != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s want=%q", code, &stdout, &stderr, tt.message)
			}
		})
	}
}
