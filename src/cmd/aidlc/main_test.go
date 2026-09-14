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

func mainDependencies(
	createSpace func(string, string) (string, error),
	listSpaces func(string) ([]workspace.Space, error),
	switchSpace func(string, string) (string, error),
	prepareOutput func(),
) cli.Dependencies {
	return cli.Dependencies{
		CreateSpace:   createSpace,
		ListSpaces:    listSpaces,
		SwitchSpace:   switchSpace,
		PrepareOutput: prepareOutput,
	}
}

func TestSpaceCreatorRootInput(t *testing.T) {
	t.Parallel()

	wantInput := workspace.RootInput{
		ExplicitDir:      "explicit path",
		AIDLCProjectDir:  "aidlc path",
		ClaudeProjectDir: "claude path",
		WorkingDir:       "working directory",
	}
	callback := spaceCreator(
		func() (string, error) {
			return wantInput.WorkingDir, nil
		},
		func(key string) string {
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
	got, err := callback("Team Alpha", wantInput.ExplicitDir)
	if got != "team-alpha" || err != nil {
		t.Errorf("callback() = (%q, %v), want (team-alpha, nil)", got, err)
	}
}

func TestSpaceCreatorLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "help", args: []string{"help"}},
		{name: "version", args: []string{"version"}},
		{name: "missing name", args: []string{"space", "create"}, code: 1},
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
				buildinfo.Info{}, mainDependencies(

					callback,
					nil,
					nil,
					nil))

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

func TestSpaceListerRootInput(t *testing.T) {
	t.Parallel()

	want := []workspace.Space{{Name: "team", Active: true}, {Name: "default"}}
	callback := spaceLister(
		func() (string, error) {
			return "working directory", nil
		},
		func(key string) string {
			return map[string]string{
				"AIDLC_PROJECT_DIR":  "aidlc directory",
				"CLAUDE_PROJECT_DIR": "claude directory",
			}[key]
		},
		func(input workspace.RootInput) ([]workspace.Space, error) {
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
	got, err := callback("explicit directory")
	if err != nil || !slices.Equal(got, want) {
		t.Errorf(
			"callback()=(%v, %v), want (%v, nil)",
			got,
			err,
			want,
		)
	}
}

func TestSpaceListerLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "extra positional", args: []string{"space", "list", "extra"}, code: 1},
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
				buildinfo.Info{}, mainDependencies(

					func(string, string) (string, error) { return "team", nil },
					callback,
					nil,
					nil))

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
	callback := spaceSwitcher(
		func() (string, error) {
			return want.WorkingDir, nil
		},
		func(key string) string {
			return map[string]string{
				"AIDLC_PROJECT_DIR":  want.AIDLCProjectDir,
				"CLAUDE_PROJECT_DIR": want.ClaudeProjectDir,
			}[key]
		},
		func(input workspace.RootInput, raw string) (string, error) {
			if input != want || raw != "Team Alpha" {
				t.Errorf("switch received (%+v, %q), want all root inputs and raw name", input, raw)
			}
			return "team-alpha", nil
		},
	)
	name, err := callback("Team Alpha", want.ExplicitDir)
	if name != "team-alpha" || err != nil {
		t.Errorf("callback() = (%q, %v), want (team-alpha, nil)", name, err)
	}
}

func TestSpaceSwitcherLazyCLIInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "missing name", args: []string{"space", "switch"}, code: 1},
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
				buildinfo.Info{}, mainDependencies(

					func(string, string) (string, error) { return "team", nil },
					func(string) ([]workspace.Space, error) { return []workspace.Space{}, nil },
					callback,
					nil))

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

func TestSpaceAdapterErrors(t *testing.T) {
	t.Run("cwd", func(t *testing.T) {
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
	})
	t.Run("create", func(t *testing.T) {
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
	})
	t.Run("read", func(t *testing.T) {
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
	})
	t.Run("switch", func(t *testing.T) {
		cause := errors.New("workspace failure")
		call := spaceSwitcher(func() (string, error) { return "cwd", nil }, func(string) string { return "" }, func(workspace.RootInput, string) (string, error) { return "", cause })
		if got, err := call("team", "project"); got != "" || !errors.Is(err, cause) {
			t.Fatalf("%q %v", got, err)
		}
	})
}
