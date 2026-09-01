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
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestRunIntentSwitchExplicit(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"intent", "switch", "build-auth", "--project-dir=project path"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			SwitchIntent: func(target, explicitDir string) (workspace.IntentSelection, error) {
				calls++
				if target != "build-auth" || explicitDir != "project path" {
					t.Errorf("switch callback(%q, %q), want raw target and project path", target, explicitDir)
				}
				return workspace.IntentSelection{SpaceName: "team", DirName: "240901-build-auth"}, nil
			},
		},
	)
	if code != 0 || calls != 1 {
		t.Errorf("exit/calls = %d/%d, want 0/1", code, calls)
	}
	const want = "Active intent → 240901-build-auth (space: team)\n"
	if stdout.String() != want || stderr.Len() != 0 {
		t.Errorf("stdout/stderr = %q/%q, want %q/empty", stdout.String(), stderr.String(), want)
	}
}

func TestRunIntentSwitchBareTarget(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"intent", "Build-Auth"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			SwitchIntent: func(target, explicitDir string) (workspace.IntentSelection, error) {
				calls++
				if target != "Build-Auth" || explicitDir != "" {
					t.Errorf("switch callback(%q, %q), want unchanged bare target", target, explicitDir)
				}
				return workspace.IntentSelection{SpaceName: "default", DirName: "Build-Auth"}, nil
			},
		},
	)
	if code != 0 || calls != 1 {
		t.Errorf("exit/calls = %d/%d, want 0/1", code, calls)
	}
	if stdout.String() != "Active intent → Build-Auth (space: default)\n" || stderr.Len() != 0 {
		t.Errorf("stdout/stderr = %q/%q, want exact success output", stdout.String(), stderr.String())
	}
}

func TestRunIntentSwitchRequiresTarget(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Errorf("cli.Run() panicked for missing target: %v", recovered)
		}
	}()
	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"intent", "switch"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
				calls++
				return workspace.IntentSelection{}, nil
			},
		},
	)
	if code != 1 || calls != 0 || stdout.Len() != 0 {
		t.Errorf("exit/calls/stdout = %d/%d/%q, want 1/0/empty", code, calls, stdout.String())
	}
	assertSpaceErrorJSON(t, stderr.String())
}

func TestRunIntentSwitchRejectsRawHelpAndEmpty(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"intent", "switch", ""},
		{"intent", "switch", "help"},
		{"intent", "switch", "-h"},
		{"intent", ""},
	} {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				cli.Dependencies{
					SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
						calls++
						return workspace.IntentSelection{SpaceName: "default", DirName: "unexpected"}, nil
					},
				},
			)
			if code != 1 || calls != 0 || stdout.Len() != 0 {
				t.Errorf("exit/calls/stdout = %d/%d/%q, want 1/0/empty", code, calls, stdout.String())
			}
			assertSpaceErrorJSON(t, stderr.String())
		})
	}
}

func TestRunIntentSwitchBareExtraTargetIsSyntaxError(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	prepares := 0
	code := cli.Run(
		[]string{"intent", "build-auth", "extra"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		cli.Dependencies{
			SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
				calls++
				return workspace.IntentSelection{}, nil
			},
			PrepareOutput: func() { prepares++ },
		},
	)
	if code != 1 || calls != 0 || prepares != 1 || stdout.Len() != 0 {
		t.Errorf(
			"exit/calls/prepares/stdout = %d/%d/%d/%q, want 1/0/1/empty",
			code,
			calls,
			prepares,
			stdout.String(),
		)
	}
	assertSpaceErrorJSON(t, stderr.String())
}

func TestRunHelpIncludesIntentSwitch(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"help"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{})
	if code != 0 || stderr.Len() != 0 {
		t.Errorf("exit/stderr = %d/%q, want 0/empty", code, stderr.String())
	}
	for _, text := range []string{
		"aidlc intent switch <target> [--project-dir <path>]",
		"aidlc intent <target> [--project-dir <path>]",
		"intent switch Select an existing intent",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("help is missing %q: %q", text, stdout.String())
		}
	}
}

func TestRunIntentSwitchProjectDirPositions(t *testing.T) {
	t.Parallel()

	commands := [][]string{
		{"intent", "switch", "target"},
		{"intent", "target"},
	}
	for _, command := range commands {
		for _, equals := range []bool{false, true} {
			for position := range len(command) + 1 {
				name := fmt.Sprintf("%s/equals=%t/position=%d", strings.Join(command, " "), equals, position)
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					flag := []string{"--project-dir", "project path"}
					if equals {
						flag = []string{"--project-dir=project path"}
					}
					args := append(slices.Clone(command[:position]), flag...)
					args = append(args, command[position:]...)
					var stdout, stderr bytes.Buffer
					calls := 0
					code := cli.Run(args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
						SwitchIntent: func(target, explicitDir string) (workspace.IntentSelection, error) {
							calls++
							if target != "target" || explicitDir != "project path" {
								t.Errorf("callback(%q, %q), want target and project path", target, explicitDir)
							}
							return workspace.IntentSelection{SpaceName: "team", DirName: "record"}, nil
						},
					})
					if code != 0 || calls != 1 || stderr.Len() != 0 {
						t.Errorf("exit/calls/stderr = %d/%d/%q, want 0/1/empty", code, calls, stderr.String())
					}
				})
			}
		}
	}
}

func TestRunIntentSwitchStrictArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "explicit extra", args: []string{"intent", "switch", "target", "extra"}},
		{name: "bare extra", args: []string{"intent", "target", "extra"}},
		{name: "unknown flag explicit", args: []string{"intent", "switch", "target", "--force"}},
		{name: "unknown flag bare", args: []string{"intent", "target", "--force"}},
		{name: "json explicit", args: []string{"--json", "intent", "switch", "target"}},
		{name: "json bare", args: []string{"intent", "--json", "target"}},
		{name: "json equals", args: []string{"intent", "target", "--json=false"}},
		{name: "missing project", args: []string{"intent", "switch", "target", "--project-dir"}},
		{name: "empty project", args: []string{"intent", "target", "--project-dir="}},
		{name: "duplicate project", args: []string{"--project-dir=one", "intent", "target", "--project-dir=two"}},
		{name: "flag as split project", args: []string{"intent", "target", "--project-dir", "--force"}},
		{name: "dash split project", args: []string{"intent", "target", "--project-dir", "-dir"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			prepares := 0
			code := cli.Run(tt.args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
					calls++
					return workspace.IntentSelection{}, nil
				},
				PrepareOutput: func() { prepares++ },
			})
			if code != 1 || calls != 0 || prepares != 1 || stdout.Len() != 0 {
				t.Errorf(
					"exit/calls/prepares/stdout = %d/%d/%d/%q, want 1/0/1/empty",
					code,
					calls,
					prepares,
					stdout.String(),
				)
			}
			assertSpaceErrorJSON(t, stderr.String())
		})
	}
}

func TestRunIntentSwitchReservedVerbBoundary(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"list", "switch", "create", "archive", "rename", "show", "birth"} {
		t.Run("explicit "+target, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				[]string{"intent", "switch", target},
				&stdout,
				&stderr,
				buildinfo.Info{},
				cli.Dependencies{
					SwitchIntent: func(got, _ string) (workspace.IntentSelection, error) {
						calls++
						if got != target {
							t.Errorf("target = %q, want %q", got, target)
						}
						return workspace.IntentSelection{SpaceName: "default", DirName: target}, nil
					},
				},
			)
			if code != 0 || calls != 1 || stderr.Len() != 0 {
				t.Errorf("exit/calls/stderr = %d/%d/%q, want 0/1/empty", code, calls, stderr.String())
			}
		})
	}

	for _, target := range []string{"create", "archive", "rename", "show", "birth", "help"} {
		t.Run("bare reserved "+target, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			prepares := 0
			code := cli.Run(
				[]string{"intent", target},
				&stdout,
				&stderr,
				buildinfo.Info{},
				cli.Dependencies{
					SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
						calls++
						return workspace.IntentSelection{}, nil
					},
					PrepareOutput: func() { prepares++ },
				},
			)
			if code != 2 || calls != 0 || prepares != 0 || stdout.Len() != 0 {
				t.Errorf("exit/calls/prepares/stdout = %d/%d/%d/%q, want 2/0/0/empty", code, calls, prepares, stdout.String())
			}
		})
	}

	for _, target := range []string{"List", "Switch", "Help"} {
		t.Run("case variant "+target, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run([]string{"intent", target}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				SwitchIntent: func(got, _ string) (workspace.IntentSelection, error) {
					calls++
					return workspace.IntentSelection{SpaceName: "default", DirName: got}, nil
				},
			})
			if code != 0 || calls != 1 || stderr.Len() != 0 {
				t.Errorf("exit/calls/stderr = %d/%d/%q, want 0/1/empty", code, calls, stderr.String())
			}
		})
	}
}

func TestRunIntentSwitchFailuresAndOutputPreparation(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected switch failure")
	tests := []struct {
		name       string
		args       []string
		switchErr  error
		stdout     io.Writer
		wantEvents []string
		wantError  string
	}{
		{
			name: "success", args: []string{"intent", "target"},
			wantEvents: []string{"prepare", "switch", "stdout"},
		},
		{
			name: "syntax", args: []string{"intent", "switch"},
			wantEvents: []string{"prepare", "stderr"}, wantError: "requires exactly one target",
		},
		{
			name: "callback", args: []string{"intent", "target"}, switchErr: cause,
			wantEvents: []string{"prepare", "switch", "stderr"}, wantError: cause.Error(),
		},
		{
			name: "short stdout", args: []string{"intent", "target"}, stdout: shortOutputWriter{},
			wantEvents: []string{"prepare", "switch", "stderr"}, wantError: io.ErrShortWrite.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := []string{}
			stdout := tt.stdout
			if stdout == nil {
				stdout = outputEventWriter{stream: "stdout", events: &events}
			}
			stderr := outputEventWriter{stream: "stderr", events: &events}
			code := cli.Run(tt.args, stdout, stderr, buildinfo.Info{}, cli.Dependencies{
				SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
					events = append(events, "switch")
					return workspace.IntentSelection{SpaceName: "default", DirName: "target"}, tt.switchErr
				},
				PrepareOutput: func() { events = append(events, "prepare") },
			})
			wantCode := 0
			if tt.wantError != "" {
				wantCode = 1
			}
			if code != wantCode || !slices.Equal(events, tt.wantEvents) {
				t.Errorf("exit/events = %d/%q, want %d/%q", code, events, wantCode, tt.wantEvents)
			}
		})
	}
}

func TestRunIntentSwitchOutputFailures(t *testing.T) {
	t.Parallel()

	message := "quoted \"error\"\nwith newline & 日本語"
	tests := []struct {
		name      string
		cause     error
		stdout    io.Writer
		stderr    io.Writer
		wantError string
	}{
		{name: "callback", cause: errors.New(message), stdout: &bytes.Buffer{}, wantError: message},
		{name: "stdout", stdout: errorWriter{err: errors.New(message)}, wantError: "write stdout: " + message},
		{name: "partial stdout", stdout: &partialListWriter{err: errors.New(message)}, wantError: "write stdout: " + message},
		{name: "short stdout", stdout: shortOutputWriter{}, wantError: "write stdout: " + io.ErrShortWrite.Error()},
		{name: "stderr unavailable", cause: errors.New(message), stdout: &bytes.Buffer{}, stderr: errorWriter{err: io.ErrClosedPipe}},
		{name: "both unavailable", stdout: errorWriter{err: io.ErrClosedPipe}, stderr: errorWriter{err: io.ErrClosedPipe}},
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
				[]string{"intent", "target"},
				tt.stdout,
				errorOutput,
				buildinfo.Info{},
				cli.Dependencies{
					SwitchIntent: func(string, string) (workspace.IntentSelection, error) {
						calls++
						return workspace.IntentSelection{SpaceName: "default", DirName: "target"}, tt.cause
					},
				},
			)
			if code != 1 || calls != 1 {
				t.Errorf("exit/calls = %d/%d, want 1/1", code, calls)
			}
			if tt.wantError != "" {
				if got := assertSpaceErrorJSON(t, stderr.String()); got != tt.wantError {
					t.Errorf("JSON error = %q, want %q", got, tt.wantError)
				}
			}
			if partial, ok := tt.stdout.(*partialListWriter); ok && partial.output.Len() == 0 {
				t.Error("partial stdout was unexpectedly rolled back")
			}
			if output, ok := tt.stdout.(*bytes.Buffer); ok && output.Len() != 0 {
				t.Errorf("callback failure wrote stdout: %q", output.String())
			}
		})
	}
}
