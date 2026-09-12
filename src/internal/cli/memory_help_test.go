package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"strings"
	"testing"
)

func TestMemoryHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}, {"memory", "--help"}, {"memory", "help"}, {"help", "memory"}, {"help", "memory", "create"}, {"memory", "create", "--help"}, {"memory", "update", "help"}, {"memory", "help", "update"}, {"intent", "review", "--help"}, {"unit", "claim", "--help"}, {"space", "switch", "--help"}, {"session", "bind", "--help"}, {"install", "codex", "--help"}} {
		var out, errout bytes.Buffer
		calls := 0
		code := Run(args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { calls++; return nil, nil }, PrepareOutput: func() { calls++ }})
		if code != 0 || out.Len() == 0 || errout.Len() != 0 || calls != 0 {
			t.Errorf("help %v exit=%d calls=%d out=%s err=%s", args, code, calls, &out, &errout)
		}
	}
	for _, args := range [][]string{{"help", "unknown"}, {"memory", "unknown", "--help"}, {"memory", "create", "name", "--help"}, {"memory", "create", "--help", "--body-file", "secret"}, {"__hook", "--help"}} {
		var out, errout bytes.Buffer
		calls := 0
		code := Run(args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { calls++; return nil, nil }})
		if code != 2 || calls != 0 {
			t.Errorf("invalid help %v code=%d calls=%d", args, code, calls)
		}
	}
	for _, action := range []string{"create", "update"} {
		var out, errout bytes.Buffer
		Run([]string{"memory", action, "--help"}, &out, &errout, buildinfo.Info{}, Dependencies{})
		for _, want := range []string{"--body-file", "--type", "--title", "--description", "--actor", "--tag", "--status", "draft", "stable", "deprecated", "自由", "generated.at", "UTC", "--intent-id", "--sources-json", "--verified-json", "--metadata-json", "producer/version", "human:id", "process:id"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s help lacks %s", action, want)
			}
		}
	}
	for _, tc := range []struct {
		group, action string
		words         []string
	}{{"intent", "reopen", []string{"discovery", "planning", "tdd", "integration", "completed", "waiting", "paused"}}, {"intent", "review", []string{"pass", "fail", "assign", "accept"}}, {"unit", "claim", []string{"pending", "running", "needs_confirmation", "reported", "integrated"}}} {
		var out, errout bytes.Buffer
		Run([]string{tc.group, tc.action, "--help"}, &out, &errout, buildinfo.Info{}, Dependencies{})
		for _, want := range tc.words {
			if !strings.Contains(out.String(), want) {
				t.Errorf("help lacks %s", want)
			}
		}
	}
	var out, errout bytes.Buffer
	Run([]string{"memory", "update"}, &out, &errout, buildinfo.Info{}, Dependencies{})
	if !strings.Contains(errout.String(), "aidlc memory update --help") {
		t.Fatalf("parser error lacks help: %s", &errout)
	}
}

func TestMemoryHelpUnitConfirmVerification(t *testing.T) {
	var out, errout bytes.Buffer
	code := Run([]string{"unit", "confirm", "--help"}, &out, &errout, buildinfo.Info{}, Dependencies{})
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &errout)
	}
	for _, want := range []string{"confirmはunit/session/root/run_id/verification_sha256", "登録run", "現在のroot内容", "64桁"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("confirm help lacks %q: %s", want, &out)
		}
	}
}
