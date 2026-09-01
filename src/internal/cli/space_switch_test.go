package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
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

	for _, name := range []string{"", "help", "-h"} {
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

func TestRunHelpIncludesSpaceSwitch(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run(
		[]string{"--help"},
		&stdout,
		&stderr,
		buildinfo.Info{}, runDependencies(

			nil,
			nil,
			nil,
			nil))

	if code != 0 || stderr.Len() != 0 {
		t.Errorf("exit=%d stderr=%q, want 0 and empty", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "aidlc space switch <name> [--project-dir <path>]") {
		t.Errorf("help does not include switch syntax: %q", stdout.String())
	}
}

func TestRunSpaceSwitchProjectDirPositions(t *testing.T) {
	t.Parallel()

	for _, equals := range []bool{false, true} {
		for position := range 4 {
			t.Run(fmt.Sprintf("equals=%t position=%d", equals, position), func(t *testing.T) {
				t.Parallel()
				command := []string{"space", "switch", "Help"}
				flag := []string{"--project-dir", "project path"}
				if equals {
					flag = []string{"--project-dir=project path"}
				}
				args := append(slices.Clone(command[:position]), flag...)
				args = append(args, command[position:]...)
				var stdout, stderr bytes.Buffer
				calls := 0
				code := cli.Run(
					args,
					&stdout,
					&stderr,
					buildinfo.Info{}, runDependencies(

						nil,
						nil,
						func(name, dir string) (string, error) {
							calls++
							if name != "Help" || dir != "project path" {
								t.Errorf("callback(%q, %q), want unchanged name and path", name, dir)
							}
							return "help", nil
						},
						nil))

				if code != 0 || calls != 1 {
					t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
				}
				if stdout.String() != "Active space → help\n" || stderr.Len() != 0 {
					t.Errorf("stdout=%q stderr=%q, want normalized output and empty stderr", stdout.String(), stderr.String())
				}
			})
		}
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
		{name: "unknown before command", args: []string{"--force", "space", "switch", "team"}},
		{name: "unknown between commands", args: []string{"space", "--force", "switch", "team"}},
		{name: "unknown short", args: []string{"space", "switch", "team", "-x"}},
		{name: "unknown equals", args: []string{"space", "switch", "team", "--name=other"}},
		{name: "end marker", args: []string{"space", "switch", "--", "team"}},
		{name: "json", args: []string{"space", "switch", "team", "--json"}},
		{name: "json before", args: []string{"--json", "space", "switch", "team"}},
		{name: "json middle", args: []string{"space", "--json", "switch", "team"}},
		{name: "json true", args: []string{"space", "switch", "team", "--json=true"}},
		{name: "json false", args: []string{"space", "switch", "team", "--json=false"}},
		{name: "missing project", args: []string{"space", "switch", "team", "--project-dir"}},
		{name: "empty project", args: []string{"space", "switch", "team", "--project-dir", ""}},
		{name: "empty equals project", args: []string{"space", "switch", "team", "--project-dir="}},
		{
			name: "duplicate project",
			args: []string{"--project-dir=one", "space", "switch", "team", "--project-dir", "two"},
		},
		{name: "flag as split path", args: []string{"space", "switch", "team", "--project-dir", "--force"}},
		{name: "dash as split path", args: []string{"space", "switch", "team", "--project-dir", "-dir"}},
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

func TestRunSpaceSwitchDashPath(t *testing.T) {
	t.Parallel()

	for _, flag := range [][]string{{"--project-dir=-dir"}, {"--project-dir", "./-dir"}} {
		t.Run(strings.Join(flag, " "), func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				append([]string{"space", "switch", "team"}, flag...),
				&stdout,
				&stderr,
				buildinfo.Info{}, runDependencies(

					nil,
					nil,
					func(_ string, dir string) (string, error) {
						want := "-dir"
						if len(flag) == 2 {
							want = "./-dir"
						}
						if dir != want {
							t.Errorf("project dir=%q, want %q", dir, want)
						}
						return "team", nil
					},
					nil))

			if code != 0 || stderr.Len() != 0 {
				t.Errorf("exit=%d stderr=%q, want 0 and empty", code, stderr.String())
			}
		})
	}
}

func TestRunSpaceSwitchOutputPreparation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		cause error
		code  int
		steps []string
	}{
		{name: "success", args: []string{"space", "switch", "team"}, steps: []string{"prepare", "switch", "stdout"}},
		{
			name: "callback error", args: []string{"space", "switch", "team"}, cause: errors.New("save failure"), code: 1,
			steps: []string{"prepare", "switch", "stderr"},
		},
		{name: "missing name", args: []string{"space", "switch"}, code: 1, steps: []string{"prepare", "stderr"}},
		{
			name: "invalid flag", args: []string{"--json", "space", "switch", "team"}, code: 1,
			steps: []string{"prepare", "stderr"},
		},
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

			if code != tt.code || !slices.Equal(steps, tt.steps) {
				t.Errorf(
					"exit=%d steps=%q; want exit=%d steps=%q",
					code,
					steps,
					tt.code,
					tt.steps,
				)
			}
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
		{name: "both unavailable", stdout: errorWriter{err: io.ErrClosedPipe}, stderr: errorWriter{err: io.ErrClosedPipe}},
		{name: "short stderr", cause: errors.New(message), stdout: &bytes.Buffer{}, stderr: shortOutputWriter{}},
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
			if partial, ok := tt.stdout.(*partialListWriter); ok && partial.output.Len() == 0 {
				t.Error("partial stdout unexpectedly rolled back")
			}
			if output, ok := tt.stdout.(*bytes.Buffer); ok && output.Len() != 0 {
				t.Errorf("callback failure wrote stdout: %q", output.String())
			}
		})
	}
}
