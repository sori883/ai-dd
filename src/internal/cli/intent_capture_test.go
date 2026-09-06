package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestCodexStageInternalCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var action, projectDir string
	code := cli.Run([]string{"__codex-stage", "decision", "--project-dir", "/tmp/project"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		CodexStage: func(gotAction, gotDir string) ([]byte, error) {
			action, projectDir = gotAction, gotDir
			return []byte(`{"kind":"codex-stage","action":"decision"}`), nil
		},
	})
	if code != 0 || action != "decision" || projectDir != "/tmp/project" {
		t.Fatalf("hidden command result = code %d action %q dir %q; stdout=%q stderr=%q", code, action, projectDir, stdout.String(), stderr.String())
	}
	if stdout.String() != "{\"kind\":\"codex-stage\",\"action\":\"decision\"}\n" || stderr.Len() != 0 {
		t.Fatalf("hidden command output = %q/%q", stdout.String(), stderr.String())
	}

	for _, args := range [][]string{
		{"__codex-stage"},
		{"__codex-stage", "unknown"},
		{"__codex-stage", "decision", "--force"},
		{"__codex-stage", "decision", "--project-dir"},
		{"__codex-stage", "decision", `{"decision":"q1"}`},
	} {
		stdout.Reset()
		stderr.Reset()
		calls := 0
		if got := cli.Run(args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
			CodexStage: func(string, string) ([]byte, error) { calls++; return []byte(`{"kind":"ok"}`), nil },
		}); got != 2 || stdout.Len() != 0 || stderr.Len() == 0 || calls != 0 {
			t.Errorf("invalid hidden args %v = code %d calls %d stdout=%q stderr=%q", args, got, calls, stdout.String(), stderr.String())
		}
	}
}

func TestIntentCaptureCommandAdapter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prepareCalls := 0
	callbackCalls := 0
	if code := cli.Run([]string{"__codex-stage", "run-sensors"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		PrepareOutput: func() { prepareCalls++ },
		CodexStage: func(action, explicitDir string) ([]byte, error) {
			callbackCalls++
			if action != "run-sensors" || explicitDir != "" {
				t.Fatalf("CodexStage(%q, %q), want run-sensors/empty", action, explicitDir)
			}
			return []byte(`{"kind":"sensor-results","results":[]}`), nil
		},
	}); code != 0 {
		t.Fatalf("hidden stage adapter exit = %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if prepareCalls != 1 || callbackCalls != 1 {
		t.Fatalf("prepare/callback calls = %d/%d, want 1/1", prepareCalls, callbackCalls)
	}
}

func TestCodexStageInputFailureIsInternalError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	prepareCalls := 0
	callbackCalls := 0
	code := cli.Run([]string{"__codex-stage", "decision"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		PrepareOutput: func() { prepareCalls++ },
		CodexStageInput: func() ([]byte, error) {
			return nil, errors.New("codex stage input exceeds 65536 bytes")
		},
		CodexStageWithInput: func(string, string, []byte) ([]byte, error) {
			callbackCalls++
			return []byte(`{"kind":"decision"}`), nil
		},
	})
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "exceeds") {
		t.Fatalf("oversized hidden input = code %d callback %d stdout=%q stderr=%q; want exit 1/internal stderr", code, callbackCalls, stdout.String(), stderr.String())
	}
	if prepareCalls != 1 || callbackCalls != 0 {
		t.Fatalf("prepare/callback calls = %d/%d, want 1/0", prepareCalls, callbackCalls)
	}
}

func TestReportGrammarRemainsFourResults(t *testing.T) {
	valid := []string{"awaiting-approval", "rejected", "revised", "approved"}
	for _, result := range valid {
		var stdout, stderr bytes.Buffer
		calls := 0
		if code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", result}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
			Report: func(_, gotResult, _, _, _ string) ([]byte, error) {
				calls++
				if gotResult != result {
					t.Errorf("report result = %q, want %q", gotResult, result)
				}
				if result == "approved" {
					return []byte(`{"kind":"done","reason":"ok"}`), nil
				}
				return []byte(`{"kind":"print","message":"ok"}`), nil
			},
		}); code != 0 || calls != 1 || stderr.Len() != 0 {
			t.Errorf("valid report result %q = code %d calls %d stdout=%q stderr=%q", result, code, calls, stdout.String(), stderr.String())
		}
	}
	for _, result := range []string{"complete", "skipped", "pending", "approved-now"} {
		var stdout, stderr bytes.Buffer
		calls := 0
		if code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", result}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
			Report: func(string, string, string, string, string) ([]byte, error) {
				calls++
				return []byte(`{"kind":"print","message":"bad"}`), nil
			},
		}); code != 2 || stdout.Len() != 0 || calls != 0 || !strings.Contains(stderr.String(), "unknown report result") {
			t.Errorf("invalid report result %q = code %d calls %d stdout=%q stderr=%q", result, code, calls, stdout.String(), stderr.String())
		}
	}
}
