package okfcli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"strings"
	"testing"
)

func TestOKFCommand(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		valid bool
	}{
		{"search", []string{"search", "hello", "--space", "notes", "--project-dir", "/project"}, true},
		{"rules", []string{"rules", "--space", "default"}, true},
		{"missing space", []string{"search"}, false},
		{"old prefix", []string{"memory", "search", "--space", "default"}, false},
		{"hook", []string{"__hook", "--project-dir", "/project"}, false},
		{"installer", []string{"install", "codex"}, false},
		{"duplicate", []string{"search", "--space", "a", "--space", "b"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseCommand(tc.args)
			if (err == nil) != tc.valid {
				t.Fatalf("request %+v, error %v, valid %v", r, err, tc.valid)
			}
			if tc.valid && r.Space == "" {
				t.Fatal("lost Space")
			}
		})
	}
}
func TestOKFHelp(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		t.Run(arg, func(t *testing.T) {
			var out, err bytes.Buffer
			code := Run([]string{arg}, &out, &err, buildinfo.Info{Version: "v0.1.1", Commit: "abc"}, Dependencies{})
			if code != 0 || !strings.Contains(out.String(), "okf ") {
				t.Fatalf("%d %s %s", code, &out, &err)
			}
		})
	}
}
