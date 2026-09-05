package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/delivery"
)

func TestRunNextPublishesOneDirective(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var prepareCalls, callbackCalls int
	dependencies := cli.Dependencies{
		PrepareOutput: func() { prepareCalls++ },
		NextDelivery: func(explicitDir string) ([]byte, error) {
			callbackCalls++
			if explicitDir != "/tmp/project" {
				t.Errorf("NextDelivery explicitDir = %q, want /tmp/project", explicitDir)
			}
			return []byte(`{"kind":"run-stage"}`), nil
		},
	}

	if got := cli.Run([]string{"next", "--project-dir", "/tmp/project"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(next) exit = %d, want 0", got)
	}
	if stdout.String() != "{\"kind\":\"run-stage\"}\n" {
		t.Errorf("stdout = %q, want one newline-terminated directive", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
	if prepareCalls != 1 || callbackCalls != 1 {
		t.Errorf("PrepareOutput/NextDelivery calls = %d/%d, want 1/1", prepareCalls, callbackCalls)
	}
}

func TestRunContinuePublishesOneDirective(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var callbackCalls int
	dependencies := cli.Dependencies{
		ContinueDelivery: func(token, explicitDir string) ([]byte, error) {
			callbackCalls++
			if token != "token" || explicitDir != "/tmp/project" {
				t.Errorf("ContinueDelivery args = token %q, dir %q", token, explicitDir)
			}
			return []byte(`{"kind":"load-steering"}`), nil
		},
	}

	if got := cli.Run([]string{"continue", "token", "--project-dir", "/tmp/project"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(continue) exit = %d, want 0", got)
	}
	if stdout.String() != "{\"kind\":\"load-steering\"}\n" {
		t.Errorf("stdout = %q, want one newline-terminated directive", stdout.String())
	}
	if callbackCalls != 1 {
		t.Errorf("ContinueDelivery calls = %d, want 1", callbackCalls)
	}
}

func TestRunContinueWorkflowErrorIsTerminalDirective(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies := cli.Dependencies{
		ContinueDelivery: func(string, string) ([]byte, error) {
			return nil, &delivery.WorkflowError{Message: "continuation is stale"}
		},
	}

	if got := cli.Run([]string{"continue", "token"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(continue workflow error) exit = %d, want 0", got)
	}
	if stdout.String() != "{\"kind\":\"error\",\"message\":\"continuation is stale\"}\n" {
		t.Errorf("stdout = %q, want typed terminal error", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunContinueTreatsDashPrefixedTokenAsOpaque(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var callbackToken string
	dependencies := cli.Dependencies{
		ContinueDelivery: func(token, explicitDir string) ([]byte, error) {
			callbackToken = token
			if explicitDir != "" {
				t.Errorf("ContinueDelivery explicitDir = %q, want empty", explicitDir)
			}
			return nil, &delivery.WorkflowError{Message: "continuation token is invalid"}
		},
	}

	if got := cli.Run([]string{"continue", "-tampered"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(continue dash-prefixed token) exit = %d, want 0", got)
	}
	if callbackToken != "-tampered" {
		t.Errorf("ContinueDelivery token = %q, want -tampered", callbackToken)
	}
	if stdout.String() != "{\"kind\":\"error\",\"message\":\"continuation token is invalid\"}\n" {
		t.Errorf("stdout = %q, want typed terminal error", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunContinueTreatsReservedProjectFlagTokenAsOpaque(t *testing.T) {
	for _, test := range []struct {
		name      string
		args      []string
		wantToken string
	}{
		{name: "equals flag spelling", args: []string{"continue", "--project-dir=opaque"}, wantToken: "--project-dir=opaque"},
		{name: "split flag spelling", args: []string{"continue", "--project-dir"}, wantToken: "--project-dir"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			var callbackToken string
			dependencies := cli.Dependencies{
				ContinueDelivery: func(token, explicitDir string) ([]byte, error) {
					callbackToken = token
					if explicitDir != "" {
						t.Errorf("ContinueDelivery explicitDir = %q, want empty", explicitDir)
					}
					return nil, &delivery.WorkflowError{Message: "continuation token is invalid"}
				},
			}
			if got := cli.Run(test.args, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0", test.args, got)
			}
			if callbackToken != test.wantToken {
				t.Errorf("ContinueDelivery token = %q, want %q", callbackToken, test.wantToken)
			}
			if stdout.String() != "{\"kind\":\"error\",\"message\":\"continuation token is invalid\"}\n" {
				t.Errorf("stdout = %q, want typed workflow error", stdout.String())
			}
			if stderr.String() != "" {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunDeliverySyntaxAndInternalErrorsHaveNoStdout(t *testing.T) {
	tests := []struct {
		name string
		args []string
		deps cli.Dependencies
		code int
	}{
		{name: "next positional", args: []string{"next", "extra"}, code: 2},
		{name: "continue missing token", args: []string{"continue"}, code: 2},
		{name: "continue too many tokens", args: []string{"continue", "one", "two"}, code: 2},
		{
			name: "internal callback error",
			args: []string{"next"},
			deps: cli.Dependencies{NextDelivery: func(string) ([]byte, error) { return nil, errors.New("disk failure") }},
			code: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if got := cli.Run(test.args, &stdout, &stderr, buildinfo.Info{}, test.deps); got != test.code {
				t.Fatalf("Run() exit = %d, want %d", got, test.code)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Error("stderr is empty, want diagnostic")
			}
		})
	}
}

type shortDeliveryWriter struct {
	data bytes.Buffer
}

func (writer *shortDeliveryWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	written := len(value) / 2
	if written == 0 {
		written = 1
	}
	_, _ = writer.data.Write(value[:written])
	return written, nil
}

func TestRunDeliveryShortStdoutWriteFails(t *testing.T) {
	stdout := &shortDeliveryWriter{}
	var stderr bytes.Buffer
	dependencies := cli.Dependencies{
		NextDelivery: func(string) ([]byte, error) { return []byte(`{"kind":"run-stage"}`), nil },
	}

	if got := cli.Run([]string{"next"}, stdout, &stderr, buildinfo.Info{}, dependencies); got != 1 {
		t.Fatalf("Run(short stdout) exit = %d, want 1", got)
	}
	if stdout.data.Len() == 0 {
		t.Error("stdout has no partial write, want short-write evidence")
	}
	if stderr.Len() == 0 {
		t.Error("stderr is empty, want short-write diagnostic")
	}
}

func TestRunReadContextPublishesBoundedJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var callbackCalls int
	dependencies := cli.Dependencies{
		ReadContext: func(explicitDir string) ([]byte, error) {
			callbackCalls++
			if explicitDir != "/tmp/project" {
				t.Errorf("ReadContext explicitDir = %q, want /tmp/project", explicitDir)
			}
			return []byte(`{"kind":"context-chunk","complete":true}`), nil
		},
	}
	if got := cli.Run([]string{"read-context", "--project-dir", "/tmp/project"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(read-context) exit = %d, want 0", got)
	}
	if stdout.String() != "{\"kind\":\"context-chunk\",\"complete\":true}\n" {
		t.Errorf("stdout = %q, want canonical JSON line", stdout.String())
	}
	if stderr.Len() != 0 || callbackCalls != 1 {
		t.Errorf("stderr/callbacks = %q/%d, want empty/1", stderr.String(), callbackCalls)
	}
}

func TestRunReadContextContinueKeepsOpaqueToken(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var callbackToken string
	dependencies := cli.Dependencies{
		ContinueContext: func(token, explicitDir string) ([]byte, error) {
			callbackToken = token
			if explicitDir != "/tmp/project" {
				t.Errorf("ContinueContext explicitDir = %q, want /tmp/project", explicitDir)
			}
			return []byte(`{"kind":"context-chunk","complete":true}`), nil
		},
	}
	if got := cli.Run([]string{"read-context", "continue", "opaque-token", "--project-dir", "/tmp/project"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 0 {
		t.Fatalf("Run(read-context continue) exit = %d, want 0", got)
	}
	if callbackToken != "opaque-token" {
		t.Errorf("ContinueContext token = %q, want opaque-token", callbackToken)
	}
	if stdout.String() != "{\"kind\":\"context-chunk\",\"complete\":true}\n" || stderr.Len() != 0 {
		t.Errorf("stdout/stderr = %q/%q, want JSON/empty", stdout.String(), stderr.String())
	}
}

func TestRunReadContextRejectsCallerSelectedPathOrSlot(t *testing.T) {
	for _, args := range [][]string{
		{"read-context", "path"},
		{"read-context", "--slot", "stage-file"},
		{"read-context", "continue", "token", "--part", "2"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			callbackCalls := 0
			dependencies := cli.Dependencies{
				ReadContext: func(string) ([]byte, error) {
					callbackCalls++
					return []byte(`{"kind":"context-chunk"}`), nil
				},
				ContinueContext: func(string, string) ([]byte, error) {
					callbackCalls++
					return []byte(`{"kind":"context-chunk"}`), nil
				},
			}
			if got := cli.Run(args, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 2 {
				t.Fatalf("Run(%v) exit = %d, want 2", args, got)
			}
			if stdout.Len() != 0 || stderr.Len() == 0 || callbackCalls != 0 {
				t.Errorf("stdout/stderr/callbacks = %q/%q/%d, want empty/diagnostic/0", stdout.String(), stderr.String(), callbackCalls)
			}
		})
	}
}

func TestRunReadContextRuntimeErrorHasNoStdout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies := cli.Dependencies{
		ReadContext: func(string) ([]byte, error) { return nil, errors.New("unsafe context") },
	}
	if got := cli.Run([]string{"read-context"}, &stdout, &stderr, buildinfo.Info{}, dependencies); got != 1 {
		t.Fatalf("Run(read-context error) exit = %d, want 1", got)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Errorf("stdout/stderr = %q/%q, want empty/diagnostic", stdout.String(), stderr.String())
	}
}
