package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"testing"
)

func TestCommandPublicCommands(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"install", []string{"install", "codex", "--project-dir", "/tmp/project"}},
		{"intent_show", []string{"intent", "show", "abc", "--space", "main"}},
		{"intent_check", []string{"intent", "check", "abc", "--space", "main"}},
		{"intent_create", []string{"intent", "create", "Search", "--space", "main"}},
		{"intent_switch", []string{"intent", "switch", "Search", "--space", "main", "--session", "session"}},
		{"memory_id", []string{"memory", "search", "--space", "main", "--intent-id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			calls := 0
			code := Run(tc.args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(r CommandRequest) ([]byte, error) {
				calls++
				if r.Command != tc.args[0] {
					t.Errorf("request = %+v", r)
				}
				return []byte("done\n"), nil
			}})
			if code != 0 || calls != 1 || out.String() != "done\n" {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), errout.String())
			}
		})
	}
}
func TestCommandRejectsInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"actor_missing", []string{"kdr", "create", "--space", "main", "--file", "draft"}},
		{"space_missing", []string{"memory", "search", "query"}},
		{"empty_id", []string{"memory", "search", "--space", "main", "--intent-id", ""}},
		{"duplicate", []string{"memory", "rules", "--space", "main", "--space", "other"}},
		{"unknown", []string{"kdr", "template", "--space", "main", "--force"}},
		{"mixed_name_id", []string{"intent", "switch", "name", "--id", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--space", "main", "--session", "s"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			calls := 0
			code := Run(tc.args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { calls++; return nil, nil }})
			if code != 2 || calls != 0 {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, &errout)
			}
		})
	}
}

func TestHookCommandDispatch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		args      []string
		wantCode  int
		wantCalls int
	}{
		{name: "current hook", args: []string{"__hook", "--project-dir", "/tmp/project"}, wantCalls: 1},
		{name: "retired hook", args: []string{"__minimal-hook", "--project-dir", "/tmp/project"}, wantCode: 2},
		{name: "unknown flag", args: []string{"__hook", "--project-dir", "/tmp/project", "--unknown"}, wantCode: 2},
		{name: "missing project", args: []string{"__hook"}, wantCode: 2},
		{name: "extra argument", args: []string{"__hook", "--project-dir", "/tmp/project", "extra"}, wantCode: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errout bytes.Buffer
			calls := 0
			deps := Dependencies{Execute: func(r CommandRequest) ([]byte, error) {
				calls++
				if r.Command != "__hook" || r.ProjectDir != "/tmp/project" || r.Action != "" {
					t.Errorf("request = %+v", r)
				}
				return []byte("hook result\n"), nil
			}}
			code := Run(tc.args, &out, &errout, buildinfo.Info{}, deps)
			if code != tc.wantCode || calls != tc.wantCalls {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, &errout)
			}
			if tc.wantCalls == 1 && out.String() != "hook result\n" {
				t.Fatalf("output=%q", out.String())
			}
			_, err := ParseCommand(tc.args)
			if (err == nil) != (tc.wantCode == 0) {
				t.Fatalf("parse error=%v", err)
			}
		})
	}
}
