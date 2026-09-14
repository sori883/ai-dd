package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestRunSpaceSwitch(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"space", "switch", "Team Alpha", "--project-dir=project path"},
		&stdout,
		&stderr,
		buildinfo.Info{}, runDependencies(

			nil,
			nil,
			func(rawName, explicitDir string) (string, error) {
				calls++
				if rawName != "Team Alpha" || explicitDir != "project path" {
					t.Errorf("switch callback(%q, %q), want raw name and path", rawName, explicitDir)
				}
				return "team-alpha", nil
			},
			nil))

	if code != 0 || calls != 1 {
		t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
	}
	if stdout.String() != "Active space → team-alpha\n" || stderr.Len() != 0 {
		t.Errorf("stdout=%q stderr=%q, want exact success line and empty stderr", stdout.String(), stderr.String())
	}
}

func TestRunSpaceSwitchInvalidRawName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "-h"} {
		t.Run("raw "+name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				[]string{"space", "switch", name},
				&stdout,
				&stderr,
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					func(string, string) (string, error) {
						t.Error("invalid raw name reached switch callback")
						return "intent", nil
					},
					nil))

			if code != 1 || stdout.Len() != 0 {
				t.Errorf("exit=%d stdout=%q, want 1 and empty", code, stdout.String())
			}
			assertSpaceErrorJSON(t, stderr.String())
		})
	}
}

func TestRunSpaceSwitchShortStdoutWrite(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := cli.Run(
		[]string{"space", "switch", "team"},
		shortOutputWriter{},
		&stderr,
		buildinfo.Info{}, runDependencies(

			nil,
			nil,
			func(string, string) (string, error) { return "team", nil },
			nil))

	if code != 1 {
		t.Errorf("exit=%d, want 1 for short stdout write", code)
	}
	if message := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(message, "short write") {
		t.Errorf("JSON error=%q, want short-write cause", message)
	}
}

func TestRunSpaceSwitchInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing name", args: []string{"space", "switch"}},
		{name: "extra name", args: []string{"space", "switch", "team", "extra"}},
		{name: "force", args: []string{"space", "switch", "team", "--force"}},
		{name: "json", args: []string{"space", "switch", "team", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					func(string, string) (string, error) {
						t.Error("invalid arguments called switch callback")
						return "team", nil
					},
					nil))

			if code != 1 || stdout.Len() != 0 {
				t.Errorf("exit=%d stdout=%q, want 1 and empty", code, stdout.String())
			}
			assertSpaceErrorJSON(t, stderr.String())
		})
	}
}

func TestRunSpaceSwitchOutputPreparation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		cause        error
		code         int
		wantPrepare  bool
		wantCallback bool
	}{
		{name: "success", args: []string{"space", "switch", "team"}, wantPrepare: true, wantCallback: true},
		{
			name: "callback error", args: []string{"space", "switch", "team"}, cause: errors.New("save failure"), code: 1,
			wantPrepare: true, wantCallback: true,
		},
		{name: "missing name", args: []string{"space", "switch"}, code: 1, wantPrepare: true, wantCallback: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			steps := []string{}
			code := cli.Run(
				tt.args,
				outputEventWriter{stream: "stdout", events: &steps},
				outputEventWriter{stream: "stderr", events: &steps},
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					func(string, string) (string, error) {
						steps = append(steps, "switch")
						return "team", tt.cause
					},
					func() { steps = append(steps, "prepare") }))

			if code != tt.code {
				t.Fatalf("exit %d", code)
			}
			assertOutputPreparation(t, steps, "switch", tt.wantPrepare, tt.wantCallback)
		})
	}
}

func TestRunSpaceSwitchOutputFailures(t *testing.T) {
	t.Parallel()

	message := "quoted \"error\"\nwith newline & 日本語"
	tests := []struct {
		name     string
		cause    error
		stdout   io.Writer
		stderr   io.Writer
		wantJSON string
	}{
		{name: "callback", cause: errors.New(message), stdout: &bytes.Buffer{}, wantJSON: message},
		{
			name: "stdout", stdout: errorWriter{err: errors.New(message)},
			wantJSON: "write stdout: " + message,
		},
		{
			name: "partial stdout", stdout: &partialListWriter{err: errors.New(message)},
			wantJSON: "write stdout: " + message,
		},
		{
			name: "stderr unavailable", cause: errors.New(message), stdout: &bytes.Buffer{},
			stderr: errorWriter{err: io.ErrClosedPipe},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			errorOutput := tt.stderr
			if errorOutput == nil {
				errorOutput = &stderr
			}
			calls := 0
			code := cli.Run(
				[]string{"space", "switch", "team"},
				tt.stdout,
				errorOutput,
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					func(string, string) (string, error) {
						calls++
						return "ignored-on-error", tt.cause
					},
					nil))

			if code != 1 || calls != 1 {
				t.Errorf("exit=%d calls=%d, want 1 each", code, calls)
			}
			if tt.wantJSON != "" {
				if got := assertSpaceErrorJSON(t, stderr.String()); got != tt.wantJSON {
					t.Errorf("JSON error=%q, want %q", got, tt.wantJSON)
				}
			}
			if output, ok := tt.stdout.(*bytes.Buffer); ok && output.Len() != 0 {
				t.Errorf("callback failure wrote stdout: %q", output.String())
			}
		})
	}
}
