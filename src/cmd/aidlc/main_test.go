package main

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestSpaceCreatorRootInput(t *testing.T) {
	t.Parallel()

	wantInput := workspace.RootInput{
		ExplicitDir:      "explicit path",
		AIDLCProjectDir:  "aidlc path",
		ClaudeProjectDir: "claude path",
		WorkingDir:       "working directory",
	}
	getwdCalls := 0
	createCalls := 0
	envKeys := []string{}
	callback := spaceCreator(
		func() (string, error) {
			getwdCalls++
			return wantInput.WorkingDir, nil
		},
		func(key string) string {
			envKeys = append(envKeys, key)
			switch key {
			case "AIDLC_PROJECT_DIR":
				return wantInput.AIDLCProjectDir
			case "CLAUDE_PROJECT_DIR":
				return wantInput.ClaudeProjectDir
			default:
				t.Errorf("unexpected environment lookup %q", key)
				return ""
			}
		},
		func(input workspace.RootInput, rawName string) (string, error) {
			createCalls++
			if input != wantInput || rawName != "Team Alpha" {
				t.Errorf(
					"create(%+v, %q), want (%+v, Team Alpha)",
					input,
					rawName,
					wantInput,
				)
			}
			return "team-alpha", nil
		},
	)
	if getwdCalls != 0 || len(envKeys) != 0 || createCalls != 0 {
		t.Fatal("constructing the callback read process inputs or created data")
	}
	got, err := callback("Team Alpha", wantInput.ExplicitDir)
	if got != "team-alpha" || err != nil {
		t.Errorf("callback() = (%q, %v), want (team-alpha, nil)", got, err)
	}
	if getwdCalls != 1 || createCalls != 1 {
		t.Errorf("getwd/create calls = %d/%d, want 1 each", getwdCalls, createCalls)
	}
	if want := []string{"AIDLC_PROJECT_DIR", "CLAUDE_PROJECT_DIR"}; !slices.Equal(envKeys, want) {
		t.Errorf("environment keys = %q, want %q", envKeys, want)
	}
}

func TestSpaceCreatorWorkingDirectoryFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected working directory failure")
	callback := spaceCreator(
		func() (string, error) { return "partial path", cause },
		func(string) string {
			t.Error("environment read after cwd failure")
			return ""
		},
		func(workspace.RootInput, string) (string, error) {
			t.Error("space creation called after cwd failure")
			return "", nil
		},
	)
	got, err := callback("team", "explicit path")
	if got != "" || !errors.Is(err, cause) {
		t.Errorf("callback() = (%q, %v), want empty name and cwd cause", got, err)
	}
}

func TestSpaceCreatorLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "no arguments"},
		{name: "help", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
		{name: "version", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "unknown command", args: []string{"unknown"}, code: 2},
		{name: "missing name", args: []string{"space", "create"}, code: 1},
		{name: "unknown flag", args: []string{"space", "create", "team", "--force"}, code: 1},
		{name: "missing path", args: []string{"space", "create", "team", "--project-dir"}, code: 1},
		{
			name: "duplicate flag",
			args: []string{"space", "create", "team", "--project-dir=one", "--project-dir=two"},
			code: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callback := spaceCreator(
				func() (string, error) {
					t.Error("cwd read without a valid create invocation")
					return "", errors.New("must not read cwd")
				},
				func(string) string {
					t.Error("environment read without a valid create invocation")
					return ""
				},
				func(workspace.RootInput, string) (string, error) {
					t.Error("filesystem creation invoked for help/version/syntax error")
					return "", nil
				},
			)
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				callback,
				nil,
				nil,
				nil,
			)
			if code != tt.code {
				t.Errorf(
					"exit=%d, want %d; stdout=%q stderr=%q",
					code,
					tt.code,
					stdout.String(),
					stderr.String(),
				)
			}
		})
	}
}

func TestSpaceCreatorCreationFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected filesystem creation failure")
	callback := spaceCreator(
		func() (string, error) { return "working", nil },
		func(string) string { return "" },
		func(workspace.RootInput, string) (string, error) { return "", cause },
	)
	got, err := callback("team", "project")
	if got != "" || !errors.Is(err, cause) {
		t.Errorf("callback() = (%q, %v), want empty name and creation cause", got, err)
	}
}

func TestSpaceListerRootInput(t *testing.T) {
	t.Parallel()

	getwdCalls := 0
	var envKeys []string
	readCalls := 0
	want := []workspace.Space{{Name: "team", Active: true}, {Name: "default"}}
	callback := spaceLister(
		func() (string, error) {
			getwdCalls++
			return "working directory", nil
		},
		func(key string) string {
			envKeys = append(envKeys, key)
			return map[string]string{
				"AIDLC_PROJECT_DIR":  "aidlc directory",
				"CLAUDE_PROJECT_DIR": "claude directory",
			}[key]
		},
		func(input workspace.RootInput) ([]workspace.Space, error) {
			readCalls++
			wantInput := workspace.RootInput{
				ExplicitDir:      "explicit directory",
				AIDLCProjectDir:  "aidlc directory",
				ClaudeProjectDir: "claude directory",
				WorkingDir:       "working directory",
			}
			if input != wantInput {
				t.Errorf("RootInput=%+v, want %+v", input, wantInput)
			}
			return want, nil
		},
	)
	if getwdCalls != 0 || readCalls != 0 {
		t.Error("constructing the list callback read process inputs or workspace")
	}
	if len(envKeys) != 0 {
		t.Error("constructing the list callback read environment variables")
	}
	got, err := callback("explicit directory")
	if err != nil || !slices.Equal(got, want) {
		t.Errorf(
			"callback()=(%v, %v), want (%v, nil)",
			got,
			err,
			want,
		)
	}
	if getwdCalls != 1 || readCalls != 1 {
		t.Errorf("getwd calls=%d read calls=%d, want 1 each", getwdCalls, readCalls)
	}
	if wantKeys := []string{"AIDLC_PROJECT_DIR", "CLAUDE_PROJECT_DIR"}; !slices.Equal(envKeys, wantKeys) {
		t.Errorf("environment keys=%v, want %v", envKeys, wantKeys)
	}
}

func TestSpaceListerWorkingDirectoryFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected working directory failure")
	callback := spaceLister(
		func() (string, error) { return "partial path", cause },
		func(string) string {
			t.Error("environment read after cwd failure")
			return ""
		},
		func(workspace.RootInput) ([]workspace.Space, error) {
			t.Error("workspace read after cwd failure")
			return nil, nil
		},
	)
	got, err := callback("explicit path")
	if got != nil || !errors.Is(err, cause) {
		t.Errorf("callback()=(%v, %v), want nil and cwd cause", got, err)
	}
}

func TestSpaceListerReadFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("injected workspace read failure")
	callback := spaceLister(
		func() (string, error) { return "working directory", nil },
		func(string) string { return "" },
		func(workspace.RootInput) ([]workspace.Space, error) { return nil, cause },
	)
	got, err := callback("explicit path")
	if got != nil || !errors.Is(err, cause) {
		t.Errorf("callback()=(%v, %v), want nil and read cause", got, err)
	}
}

func TestSpaceListerLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "no arguments"},
		{name: "help", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
		{name: "version", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "unknown command", args: []string{"unknown"}, code: 2},
		{name: "unknown subcommand", args: []string{"space", "unknown"}, code: 2},
		{name: "bare separate JSON value", args: []string{"space", "--json", "false"}, code: 2},
		{name: "extra positional", args: []string{"space", "list", "extra"}, code: 1},
		{name: "help after list", args: []string{"space", "list", "--help"}, code: 1},
		{name: "duplicate JSON", args: []string{"space", "list", "--json", "--json"}, code: 1},
		{name: "bare JSON equals", args: []string{"space", "--json=false"}, code: 1},
		{name: "missing project", args: []string{"space", "list", "--project-dir"}, code: 1},
		{
			name: "duplicate project",
			args: []string{"space", "list", "--project-dir=one", "--project-dir=two"},
			code: 1,
		},
		{name: "create command", args: []string{"space", "create", "team"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callback := spaceLister(
				func() (string, error) {
					t.Error("cwd read without a valid list invocation")
					return "", errors.New("must not read cwd")
				},
				func(string) string {
					t.Error("environment read without a valid list invocation")
					return ""
				},
				func(workspace.RootInput) ([]workspace.Space, error) {
					t.Error("workspace read without a valid list invocation")
					return nil, nil
				},
			)
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				func(string, string) (string, error) { return "team", nil },
				callback,
				nil,
				nil,
			)
			if code != tt.code {
				t.Errorf(
					"exit=%d, want %d; stderr=%q",
					code,
					tt.code,
					stderr.String(),
				)
			}
		})
	}
}

func TestSpaceSwitcherRootInput(t *testing.T) {
	t.Parallel()

	want := workspace.RootInput{
		ExplicitDir: "explicit", AIDLCProjectDir: "aidlc env", ClaudeProjectDir: "claude env", WorkingDir: "cwd",
	}
	steps := []string{}
	callback := spaceSwitcher(
		func() (string, error) {
			steps = append(steps, "cwd")
			return want.WorkingDir, nil
		},
		func(key string) string {
			steps = append(steps, key)
			return map[string]string{
				"AIDLC_PROJECT_DIR":  want.AIDLCProjectDir,
				"CLAUDE_PROJECT_DIR": want.ClaudeProjectDir,
			}[key]
		},
		func(input workspace.RootInput, raw string) (string, error) {
			steps = append(steps, "switch")
			if input != want || raw != "Team Alpha" {
				t.Errorf("switch received (%+v, %q), want all root inputs and raw name", input, raw)
			}
			return "team-alpha", nil
		},
	)
	if len(steps) != 0 {
		t.Fatalf("constructing callback performed work: %q", steps)
	}
	name, err := callback("Team Alpha", want.ExplicitDir)
	if name != "team-alpha" || err != nil {
		t.Errorf("callback() = (%q, %v), want (team-alpha, nil)", name, err)
	}
	if !slices.Equal(steps, []string{"cwd", "AIDLC_PROJECT_DIR", "CLAUDE_PROJECT_DIR", "switch"}) {
		t.Errorf("callback steps = %q, want lazy process inputs and one switch", steps)
	}
}

func TestSpaceSwitcherFailures(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"cwd", "switch"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("injected " + stage + " failure")
			callback := spaceSwitcher(
				func() (string, error) {
					if stage == "cwd" {
						return "partial cwd", cause
					}
					return "cwd", nil
				},
				func(string) string {
					if stage == "cwd" {
						t.Error("environment read after cwd failure")
					}
					return ""
				},
				func(workspace.RootInput, string) (string, error) {
					if stage == "cwd" {
						t.Error("switch attempted after cwd failure")
					}
					return "", cause
				},
			)
			name, err := callback("team", "project")
			if name != "" || !errors.Is(err, cause) {
				t.Errorf(
					"callback() = (%q, %v), want empty and cause %v",
					name,
					err,
					cause,
				)
			}
		})
	}
}

func TestSpaceSwitcherLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "no arguments"},
		{name: "help", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
		{name: "version", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "unknown", args: []string{"unknown"}, code: 2},
		{name: "bare switch is not implemented", args: []string{"space", "team"}, code: 2},
		{name: "bare json separate value", args: []string{"space", "--json", "false"}, code: 2},
		{name: "missing name", args: []string{"space", "switch"}, code: 1},
		{name: "empty name", args: []string{"space", "switch", ""}, code: 1},
		{name: "raw help", args: []string{"space", "switch", "help"}, code: 1},
		{name: "raw short help", args: []string{"space", "switch", "-h"}, code: 1},
		{name: "unknown flag", args: []string{"space", "switch", "team", "--json"}, code: 1},
		{name: "extra name", args: []string{"space", "switch", "team", "extra"}, code: 1},
		{name: "missing project", args: []string{"space", "switch", "team", "--project-dir"}, code: 1},
		{
			name: "duplicate project",
			args: []string{"space", "switch", "team", "--project-dir=one", "--project-dir=two"}, code: 1,
		},
		{name: "create", args: []string{"space", "create", "team"}},
		{name: "list", args: []string{"space", "list"}},
		{name: "bare list", args: []string{"space"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			callback := spaceSwitcher(
				func() (string, error) {
					t.Error("cwd read without a valid switch")
					return "", errors.New("unexpected cwd read")
				},
				func(string) string {
					t.Error("environment read without a valid switch")
					return ""
				},
				func(workspace.RootInput, string) (string, error) {
					t.Error("workspace called without a valid switch")
					return "", nil
				},
			)
			var stdout, stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				func(string, string) (string, error) { return "team", nil },
				func(string) ([]workspace.Space, error) { return []workspace.Space{}, nil },
				callback,
				nil,
			)
			if code != tt.code {
				t.Errorf(
					"exit=%d, want %d; stderr=%q",
					code,
					tt.code,
					stderr.String(),
				)
			}
		})
	}
}
