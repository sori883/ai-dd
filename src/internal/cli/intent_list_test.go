package cli_test

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/workspace"
)

func TestRunIntentListHumanAliases(t *testing.T) {
	t.Parallel()

	dirName := "240901-build-auth"
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "bare", args: []string{"intent"}},
		{name: "list", args: []string{"intent", "list"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(tt.args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				ListIntents: func(explicitDir string) (workspace.IntentListing, error) {
					calls++
					if explicitDir != "" {
						t.Errorf("explicitDir = %q, want empty", explicitDir)
					}
					return workspace.IntentListing{
						SpaceName: "beta",
						Intents: []workspace.Intent{
							{Slug: "build-auth", Status: "construction", Repos: []string{}, DirName: &dirName, Active: true},
							{Slug: "registry-only", Status: "planning", Repos: []string{}},
						},
					}, nil
				},
			})
			if code != 0 || calls != 1 {
				t.Errorf("exit=%d calls=%d, want 0 and 1", code, calls)
			}
			const want = "Intents in space \"beta\":\n" +
				"* 240901-build-auth  [construction]\n" +
				"  registry-only  [planning]\n"
			if got := stdout.String(); got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRunIntentListHumanEmptyAndNoActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		intents []workspace.Intent
		want    string
	}{
		{
			name: "empty",
			want: "No intents in space \"beta\" yet. Start one by describing what to build: " +
				"/aidlc \"build the auth service\"\n",
		},
		{
			name:    "no active",
			intents: []workspace.Intent{{Slug: "planned", Status: "planning", Repos: []string{}}},
			want: "Intents in space \"beta\":\n  planned  [planning]\n\n" +
				"(no active intent — switch with /aidlc intent <name>)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{"intent"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				ListIntents: func(string) (workspace.IntentListing, error) {
					return workspace.IntentListing{SpaceName: "beta", Intents: tt.intents}, nil
				},
			})
			if code != 0 {
				t.Errorf("exit=%d, want 0; stderr=%q", code, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunIntentListJSONContract(t *testing.T) {
	t.Parallel()

	dirName := "240901-build-auth"
	scope := "internal-only"
	tests := []struct {
		name    string
		intents []workspace.Intent
		want    string
	}{
		{
			name: "active directory",
			intents: []workspace.Intent{
				{UUID: "one", Slug: "build-auth", Status: "construction", Scope: &scope, Repos: []string{}, DirName: &dirName, Active: true},
				{UUID: "two", Slug: "registry-only", Status: "planning", Repos: []string{"api"}},
			},
			want: `{"active":"240901-build-auth","space":"beta","intents":[` +
				`{"uuid":"one","slug":"build-auth","status":"construction","repos":[],"dirName":"240901-build-auth","active":true},` +
				`{"uuid":"two","slug":"registry-only","status":"planning","repos":["api"],"dirName":null,"active":false}]}` + "\n",
		},
		{
			name:    "no active",
			intents: []workspace.Intent{{UUID: "one", Slug: "planned", Status: "planning", Repos: []string{}}},
			want:    `{"active":null,"space":"beta","intents":[{"uuid":"one","slug":"planned","status":"planning","repos":[],"dirName":null,"active":false}]}` + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := cli.Run([]string{"intent", "list", "--json"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				ListIntents: func(string) (workspace.IntentListing, error) {
					return workspace.IntentListing{SpaceName: "beta", Intents: tt.intents}, nil
				},
			})
			if code != 0 {
				t.Errorf("exit=%d, want 0; stderr=%q", code, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout=%q, want %q", got, tt.want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr=%q, want empty", got)
			}
		})
	}
}

func TestRunIntentListStrictArgumentsAndLazyCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantCode    int
		wantCalls   int
		wantPrepare int
		wantDir     string
	}{
		{name: "bare", args: []string{"intent"}, wantCalls: 1, wantPrepare: 1},
		{name: "JSON before command", args: []string{"--json", "intent", "list"}, wantCalls: 1, wantPrepare: 1},
		{name: "JSON between command words", args: []string{"intent", "--json", "list"}, wantCalls: 1, wantPrepare: 1},
		{name: "JSON after command", args: []string{"intent", "list", "--json"}, wantCalls: 1, wantPrepare: 1},
		{name: "project split", args: []string{"intent", "list", "--project-dir", "project path"}, wantCalls: 1, wantPrepare: 1, wantDir: "project path"},
		{name: "project equals with dash", args: []string{"intent", "list", "--project-dir=-project"}, wantCalls: 1, wantPrepare: 1, wantDir: "-project"},
		{name: "bare JSON separate value remains unknown", args: []string{"intent", "--json", "false"}, wantCode: 2},
		{name: "list JSON separate value is extra", args: []string{"intent", "list", "--json", "false"}, wantCode: 1, wantPrepare: 1},
		{name: "JSON equals is unknown flag", args: []string{"intent", "--json=false"}, wantCode: 1, wantPrepare: 1},
		{name: "duplicate JSON", args: []string{"intent", "list", "--json", "--json"}, wantCode: 1, wantPrepare: 1},
		{name: "unknown flag", args: []string{"intent", "list", "--force"}, wantCode: 1, wantPrepare: 1},
		{name: "project split rejects flag value", args: []string{"intent", "list", "--project-dir", "--force"}, wantCode: 1, wantPrepare: 1},
		{name: "duplicate project", args: []string{"intent", "list", "--project-dir=one", "--project-dir=two"}, wantCode: 1, wantPrepare: 1},
		{name: "extra positional", args: []string{"intent", "list", "extra"}, wantCode: 1, wantPrepare: 1},
		{name: "create remains unknown", args: []string{"intent", "create"}, wantCode: 2},
		{name: "switch remains unknown", args: []string{"intent", "switch"}, wantCode: 2},
		{name: "target remains unknown", args: []string{"intent", "target"}, wantCode: 2},
		{name: "dedicated help remains unknown", args: []string{"intent", "help"}, wantCode: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			prepares := 0
			code := cli.Run(tt.args, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
				ListIntents: func(explicitDir string) (workspace.IntentListing, error) {
					calls++
					if explicitDir != tt.wantDir {
						t.Errorf("explicitDir=%q, want %q", explicitDir, tt.wantDir)
					}
					return workspace.IntentListing{SpaceName: "default", Intents: []workspace.Intent{}}, nil
				},
				PrepareOutput: func() { prepares++ },
			})
			if code != tt.wantCode || calls != tt.wantCalls || prepares != tt.wantPrepare {
				t.Errorf(
					"exit/calls/prepares=%d/%d/%d, want %d/%d/%d; stderr=%q",
					code, calls, prepares, tt.wantCode, tt.wantCalls, tt.wantPrepare, stderr.String(),
				)
			}
		})
	}
}

func TestRunIntentListFailuresAndShortWrites(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "human", args: []string{"intent"}},
		{name: "JSON", args: []string{"intent", "list", "--json"}},
	} {
		t.Run(tt.name+" short write", func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := cli.Run(tt.args, shortOutputWriter{}, &stderr, buildinfo.Info{}, cli.Dependencies{
				ListIntents: func(string) (workspace.IntentListing, error) {
					return workspace.IntentListing{SpaceName: "default", Intents: []workspace.Intent{}}, nil
				},
			})
			if code != 1 {
				t.Errorf("exit=%d, want 1", code)
			}
			want := "write stdout: " + io.ErrShortWrite.Error()
			if got := assertSpaceErrorJSON(t, stderr.String()); got != want {
				t.Errorf("JSON error=%q, want %q", got, want)
			}
		})
	}

	t.Run("query error", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("injected intent query failure")
		var stdout, stderr bytes.Buffer
		code := cli.Run([]string{"intent"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{
			ListIntents: func(string) (workspace.IntentListing, error) {
				return workspace.IntentListing{}, cause
			},
		})
		if code != 1 || stdout.Len() != 0 {
			t.Errorf("exit=%d stdout=%q, want 1 and empty", code, stdout.String())
		}
		if got := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(got, cause.Error()) {
			t.Errorf("JSON error=%q, want query cause", got)
		}
	})
}

func TestRunHelpIncludesIntentList(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"help"}, &stdout, &stderr, buildinfo.Info{}, cli.Dependencies{})
	if code != 0 || stderr.Len() != 0 {
		t.Errorf("exit=%d stderr=%q, want 0 and empty", code, stderr.String())
	}
	for _, text := range []string{
		"aidlc intent list [--json] [--project-dir <path>]",
		"aidlc intent [--json] [--project-dir <path>]",
		"intent list   List intents (intent is an alias)",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("help is missing %q: %q", text, stdout.String())
		}
	}
}

func TestRunIntentListOutputPreparationOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		queryErr   error
		wantCode   int
		wantEvents []string
	}{
		{name: "bare human", args: []string{"intent"}, wantEvents: []string{"prepare", "list", "stdout"}},
		{name: "list JSON", args: []string{"intent", "list", "--json"}, wantEvents: []string{"prepare", "list", "stdout"}},
		{name: "syntax error", args: []string{"intent", "list", "--json=false"}, wantCode: 1, wantEvents: []string{"prepare", "stderr"}},
		{name: "query error", args: []string{"intent"}, queryErr: errors.New("query failure"), wantCode: 1, wantEvents: []string{"prepare", "list", "stderr"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			events := []string{}
			code := cli.Run(
				tt.args,
				outputEventWriter{stream: "stdout", events: &events},
				outputEventWriter{stream: "stderr", events: &events},
				buildinfo.Info{},
				cli.Dependencies{
					ListIntents: func(string) (workspace.IntentListing, error) {
						events = append(events, "list")
						return workspace.IntentListing{SpaceName: "default", Intents: []workspace.Intent{}}, tt.queryErr
					},
					PrepareOutput: func() { events = append(events, "prepare") },
				},
			)
			if code != tt.wantCode {
				t.Errorf("exit=%d, want %d", code, tt.wantCode)
			}
			if !slices.Equal(events, tt.wantEvents) {
				t.Errorf("events=%q, want %q", events, tt.wantEvents)
			}
		})
	}
}
