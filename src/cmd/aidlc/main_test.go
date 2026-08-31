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
