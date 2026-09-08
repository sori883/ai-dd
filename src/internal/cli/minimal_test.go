package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"testing"
)

func TestMinimalPublicCommands(t *testing.T) {
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
			code := Run(tc.args, &out, &errout, buildinfo.Info{}, Dependencies{Minimal: func(r MinimalRequest) ([]byte, error) {
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
func TestMinimalRejectsInvalid(t *testing.T) {
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
			code := Run(tc.args, &out, &errout, buildinfo.Info{}, Dependencies{Minimal: func(MinimalRequest) ([]byte, error) { calls++; return nil, nil }})
			if code != 2 || calls != 0 {
				t.Fatalf("code=%d calls=%d err=%s", code, calls, &errout)
			}
		})
	}
}
