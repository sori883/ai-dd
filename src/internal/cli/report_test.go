package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/delivery"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
)

func TestRunReportGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStage  string
		wantResult string
		wantInput  string
		wantReason string
		wantDir    string
	}{
		{
			name:       "awaiting approval with project directory",
			args:       []string{"report", "--stage", "intent-capture", "--result", "awaiting-approval", "--project-dir", "/tmp/project"},
			wantStage:  "intent-capture",
			wantResult: "awaiting-approval",
			wantDir:    "/tmp/project",
		},
		{
			name:       "rejected preserves exact values",
			args:       []string{"report", "--result", "rejected", "--reason", "  keep this  ", "--user-input", "Request Changes", "--stage", "intent-capture"},
			wantStage:  "intent-capture",
			wantResult: "rejected",
			wantInput:  "Request Changes",
			wantReason: "  keep this  ",
		},
		{
			name:       "revised accepts optional flags in any order",
			args:       []string{"report", "--project-dir=/tmp/project", "--stage=intent-capture", "--result=revised"},
			wantStage:  "intent-capture",
			wantResult: "revised",
			wantDir:    "/tmp/project",
		},
		{
			name:       "approved accepts exact choice as opaque value",
			args:       []string{"report", "--stage", "intent-capture", "--user-input", "Accept as-is", "--result", "approved", "--reason", "unused"},
			wantStage:  "intent-capture",
			wantResult: "approved",
			wantInput:  "Accept as-is",
			wantReason: "unused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			var calls int
			dependencies := cli.Dependencies{
				Report: func(stage, result, userInput, reason, explicitDir string) ([]byte, error) {
					calls++
					if stage != tt.wantStage || result != tt.wantResult || userInput != tt.wantInput || reason != tt.wantReason || explicitDir != tt.wantDir {
						t.Errorf("Report args = (%q, %q, %q, %q, %q), want (%q, %q, %q, %q, %q)", stage, result, userInput, reason, explicitDir, tt.wantStage, tt.wantResult, tt.wantInput, tt.wantReason, tt.wantDir)
					}
					if result == "approved" {
						return []byte(`{"kind":"done","reason":"ok"}`), nil
					}
					return []byte(`{"kind":"print","message":"ok"}`), nil
				},
			}

			if got := cli.Run(tt.args, &stdout, &stderr, buildinfo.Info{}, dependencies); got != tt.wantCode {
				t.Fatalf("Run(%v) exit = %d, want %d; stdout=%q stderr=%q", tt.args, got, tt.wantCode, stdout.String(), stderr.String())
			}
			if calls != 1 {
				t.Fatalf("Report calls = %d, want 1", calls)
			}
			wantWire := "{\"kind\":\"print\",\"message\":\"ok\"}\n"
			if tt.wantResult == "approved" {
				wantWire = "{\"kind\":\"done\",\"reason\":\"ok\"}\n"
			}
			if stdout.String() != wantWire || stderr.Len() != 0 {
				t.Errorf("stdout/stderr = %q/%q, want directive/empty", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunReportPreparesOutputExactlyOnceForValidCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	prepareCalls := 0
	reportCalls := 0
	code := cli.Run(
		[]string{"report", "--stage", "intent-capture", "--result", "awaiting-approval"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			Report: func(string, string, string, string, string) ([]byte, error) {
				reportCalls++
				return []byte(`{"kind":"print","message":"ok"}`), nil
			},
			PrepareOutput: func() { prepareCalls++ },
		},
	)
	if code != 0 {
		t.Fatalf("Run(report) exit = %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if prepareCalls != 1 {
		t.Errorf("PrepareOutput calls = %d, want 1", prepareCalls)
	}
	if reportCalls != 1 {
		t.Errorf("Report calls = %d, want 1", reportCalls)
	}
}

func TestRunReportRejectsInvalidGrammarWithoutCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing stage", args: []string{"report", "--result", "approved"}},
		{name: "missing result", args: []string{"report", "--stage", "intent-capture"}},
		{name: "unknown result", args: []string{"report", "--stage", "intent-capture", "--result", "complete"}},
		{name: "unknown alias", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--status", "done"}},
		{name: "unknown flag", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--force"}},
		{name: "duplicate stage", args: []string{"report", "--stage", "one", "--stage", "two", "--result", "approved"}},
		{name: "duplicate result", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--result", "revised"}},
		{name: "duplicate optional flag", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--reason", "one", "--reason", "two"}},
		{name: "missing stage value", args: []string{"report", "--stage", "--result", "approved"}},
		{name: "missing result value", args: []string{"report", "--stage", "intent-capture", "--result"}},
		{name: "missing optional value", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--user-input"}},
		{name: "empty equals value", args: []string{"report", "--stage=", "--result", "approved"}},
		{name: "positional argument", args: []string{"report", "intent-capture", "--result", "approved"}},
		{name: "positional after flags", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "extra"}},
		{name: "duplicate project directory", args: []string{"report", "--project-dir", "one", "--stage", "intent-capture", "--result", "approved", "--project-dir=two"}},
		{name: "project directory missing value", args: []string{"report", "--stage", "intent-capture", "--result", "approved", "--project-dir"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			prepareCalls := 0
			code := cli.Run(tt.args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				Report: func(string, string, string, string, string) ([]byte, error) {
					calls++
					return []byte(`{"kind":"print","message":"unexpected"}`), nil
				},
				PrepareOutput: func() { prepareCalls++ },
			})
			if code != 2 {
				t.Errorf("Run(%v) exit = %d, want 2; stdout=%q stderr=%q", tt.args, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 || !strings.Contains(stderr.String(), "aidlc:") {
				t.Errorf("stderr = %q, want syntax diagnostic", stderr.String())
			}
			if calls != 0 {
				t.Errorf("Report calls = %d, want 0", calls)
			}
			if prepareCalls != 1 {
				t.Errorf("PrepareOutput calls = %d, want 1", prepareCalls)
			}
		})
	}
}

func TestRunReportCallbackErrorIsInternal(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	wantErr := errors.New("report callback failed")
	code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", "approved"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		Report: func(string, string, string, string, string) ([]byte, error) { return nil, wantErr },
	})
	if code != 1 {
		t.Fatalf("Run(report callback error) exit = %d, want 1", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), wantErr.Error()) {
		t.Errorf("stdout/stderr = %q/%q, want empty/diagnostic", stdout.String(), stderr.String())
	}
}

func TestRunReportWorkflowErrorIsTerminalDirective(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", "approved"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		Report: func(string, string, string, string, string) ([]byte, error) {
			return nil, &delivery.WorkflowError{Message: "stage is not ready"}
		},
	})
	if code != 0 {
		t.Fatalf("Run(report workflow error) exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "{\"kind\":\"error\",\"message\":\"stage is not ready\"}\n" {
		t.Errorf("stdout = %q, want terminal error directive", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReportOrchestratorWorkflowErrorIsTerminalDirective(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", "rejected"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
		Report: func(string, string, string, string, string) ([]byte, error) {
			return nil, orchestrator.NewWorkflowError("approval choice is stale", orchestrator.ErrStaleHumanTurn)
		},
	})
	if code != 0 {
		t.Fatalf("Run(report orchestrator workflow error) exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "{\"kind\":\"error\",\"message\":\"approval choice is stale\"}\n" {
		t.Errorf("stdout = %q, want terminal error directive", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReportPreservesCanonicalSuccessWire(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result string
		wire   string
	}{
		{
			name:   "awaiting approval",
			result: "awaiting-approval",
			wire:   `{"kind":"print","message":"Recorded awaiting-approval for \"intent-capture\"."}`,
		},
		{
			name:   "rejected",
			result: "rejected",
			wire:   `{"kind":"print","message":"Recorded rejected for \"intent-capture\"."}`,
		},
		{
			name:   "revised",
			result: "revised",
			wire:   `{"kind":"print","message":"Recorded revised for \"intent-capture\"."}`,
		},
		{
			name:   "approved",
			result: "approved",
			wire:   `{"kind":"done","reason":"Committed approve for \"intent-capture\". State advanced; run next to continue."}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", tt.result}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				Report: func(string, string, string, string, string) ([]byte, error) {
					return []byte(tt.wire), nil
				},
			})
			if code != 0 {
				t.Fatalf("Run(report %s) exit = %d; stdout=%q stderr=%q", tt.result, code, stdout.String(), stderr.String())
			}
			if got := strings.TrimSuffix(stdout.String(), "\n"); got != tt.wire {
				t.Errorf("stdout wire = %q, want %q", got, tt.wire)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

type shortReportWriter struct {
	data bytes.Buffer
}

func (writer *shortReportWriter) Write(value []byte) (int, error) {
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

func TestRunReportShortWriteAndInvalidWireAreInternal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wire []byte
		out  io.Writer
	}{
		{name: "short write", wire: []byte(`{"kind":"print","message":"ok"}`), out: &shortReportWriter{}},
		{name: "invalid wire", wire: []byte("not json"), out: &bytes.Buffer{}},
		{name: "null wire", wire: []byte("null"), out: &bytes.Buffer{}},
		{name: "wrong result wire", wire: []byte(`{"kind":"done","reason":"unexpected"}`), out: &bytes.Buffer{}},
		{name: "noncanonical whitespace", wire: []byte(` {"kind":"print","message":"ok"} `), out: &bytes.Buffer{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			code := cli.Run([]string{"report", "--stage", "intent-capture", "--result", "revised"}, tt.out, &stderr, buildinfo.Info{}, cli.Dependencies{
				Report: func(string, string, string, string, string) ([]byte, error) { return tt.wire, nil },
			})
			if code != 1 {
				t.Errorf("Run(report %s) exit = %d, want 1; stderr=%q", tt.name, code, stderr.String())
			}
			if stderr.Len() == 0 {
				t.Error("stderr is empty, want internal diagnostic")
			}
		})
	}
}
