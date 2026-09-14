package cli

import (
	"bytes"
	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"strings"
	"testing"
)

func TestMemoryHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}, {"intent", "review", "--help"}, {"unit", "claim", "--help"}, {"space", "switch", "--help"}, {"session", "bind", "--help"}} {
		var out, errout bytes.Buffer
		calls := 0
		code := Run(args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { calls++; return nil, nil }, PrepareOutput: func() { calls++ }})
		if code != 0 || out.Len() == 0 || errout.Len() != 0 || calls != 0 {
			t.Errorf("help %v exit=%d calls=%d out=%s err=%s", args, code, calls, &out, &errout)
		}
	}
	for _, args := range [][]string{{"help", "unknown"}, {"__hook", "--help"}} {
		var out, errout bytes.Buffer
		calls := 0
		code := Run(args, &out, &errout, buildinfo.Info{}, Dependencies{Execute: func(CommandRequest) ([]byte, error) { calls++; return nil, nil }})
		if code != 2 || calls != 0 {
			t.Errorf("invalid help %v code=%d calls=%d", args, code, calls)
		}
	}
	for _, action := range []string{"create", "update"} {
		var out, errout bytes.Buffer
		okfcli.Run([]string{action, "--help"}, &out, &errout, buildinfo.Info{}, okfcli.Dependencies{})
		for _, want := range []string{"--body-file", "--type", "--title", "--description", "--actor", "--tag", "--status", "--intent-id", "--sources-json", "--verified-json", "--metadata-json", "producer/version", "human:id", "process:id"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s help lacks %s", action, want)
			}
		}
	}
	var out, errout bytes.Buffer
	okfcli.Run([]string{"update"}, &out, &errout, buildinfo.Info{}, okfcli.Dependencies{})
	if !strings.Contains(errout.String(), "okf update --help") {
		t.Fatalf("parser error lacks help: %s", &errout)
	}
}

func TestMemoryHelpUnitConfirmVerification(t *testing.T) {
	var out, errout bytes.Buffer
	code := Run([]string{"unit", "confirm", "--help"}, &out, &errout, buildinfo.Info{}, Dependencies{})
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s", code, &errout)
	}
	for _, want := range []string{"confirmはunit/session/root/run_id/verification_sha256", "登録run", "現在のroot内容"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("confirm help lacks %q: %s", want, &out)
		}
	}
}
