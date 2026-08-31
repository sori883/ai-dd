package cli_test

import (
	"bytes"
	"encoding/json"
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

func TestRunSpaceListHuman(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"space", "list"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		nil,
		func(explicitDir string) ([]workspace.Space, error) {
			calls++
			if explicitDir != "" {
				t.Errorf("explicitDir = %q, want empty", explicitDir)
			}
			return []workspace.Space{
				{Name: "alpha"},
				{Name: "default"},
				{Name: "zeta", Active: true},
			}, nil
		},
		nil,
		nil,
	)
	if code != 0 || calls != 1 {
		t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
	}
	const want = "Spaces:\n  alpha\n  default\n* zeta\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}
}

func TestRunSpaceListJSON(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"space", "list", "--json"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		nil,
		func(string) ([]workspace.Space, error) {
			calls++
			return []workspace.Space{
				{Name: "alpha"},
				{Name: "default"},
				{Name: "zeta", Active: true},
			}, nil
		},
		nil,
		nil,
	)
	if code != 0 || calls != 1 {
		t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
	}
	const want = "{\"active\":\"zeta\",\"spaces\":[{\"name\":\"alpha\",\"active\":false}," +
		"{\"name\":\"default\",\"active\":false},{\"name\":\"zeta\",\"active\":true}]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}
}

func TestRunSpaceBareAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "human", args: []string{"space"}, want: "Spaces:\n* default\n"},
		{
			name: "JSON",
			args: []string{"space", "--json"},
			want: "{\"active\":\"default\",\"spaces\":[{\"name\":\"default\",\"active\":true}]}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					calls++
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 0 || calls != 1 {
				t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRunSpaceListExtraArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "name", args: []string{"space", "list", "team"}},
		{name: "empty", args: []string{"space", "list", ""}},
		{name: "JSON separate value", args: []string{"space", "list", "--json", "false"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					calls++
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 1 || calls != 0 {
				t.Errorf("exit=%d calls=%d, want 1, 0", code, calls)
			}
			if got := stdout.String(); got != "" {
				t.Errorf("stdout = %q, want empty", got)
			}
			if got := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(got, "argument") {
				t.Errorf("JSON error = %q, want an argument diagnostic", got)
			}
		})
	}
}

func TestRunSpaceListDuplicateJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"--json", "space", "--json", "list"}},
		{name: "bare", args: []string{"space", "--json", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					calls++
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 1 || calls != 0 {
				t.Errorf("exit=%d calls=%d, want 1, 0", code, calls)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if got := assertSpaceErrorJSON(t, stderr.String()); got != "duplicate --json" {
				t.Errorf("JSON error = %q, want duplicate --json", got)
			}
		})
	}
}

func TestRunSpaceListShortStdoutWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "human", args: []string{"space", "list"}},
		{name: "JSON", args: []string{"space", "list", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				shortOutputWriter{},
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 1 {
				t.Errorf("short stdout write exit=%d, want 1", code)
			}
			want := "write stdout: " + io.ErrShortWrite.Error()
			if got := assertSpaceErrorJSON(t, stderr.String()); got != want {
				t.Errorf("JSON error = %q, want %q", got, want)
			}
		})
	}
}

func TestRunSpaceListOutputPreparation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		readErr    error
		wantCode   int
		wantEvents []string
	}{
		{
			name:       "list human",
			args:       []string{"space", "list"},
			wantEvents: []string{"prepare", "list", "stdout"},
		},
		{
			name:       "list JSON",
			args:       []string{"space", "--json", "list"},
			wantEvents: []string{"prepare", "list", "stdout"},
		},
		{
			name:       "bare human",
			args:       []string{"space"},
			wantEvents: []string{"prepare", "list", "stdout"},
		},
		{
			name:       "bare JSON",
			args:       []string{"--json", "space"},
			wantEvents: []string{"prepare", "list", "stdout"},
		},
		{
			name:       "list invalid flag",
			args:       []string{"space", "list", "--json=false"},
			wantCode:   1,
			wantEvents: []string{"prepare", "stderr"},
		},
		{
			name:       "list extra positional",
			args:       []string{"space", "list", "extra"},
			wantCode:   1,
			wantEvents: []string{"prepare", "stderr"},
		},
		{
			name:       "bare invalid flag",
			args:       []string{"space", "--help"},
			wantCode:   1,
			wantEvents: []string{"prepare", "stderr"},
		},
		{
			name:       "reader error",
			args:       []string{"space", "list"},
			readErr:    errors.New("read failure"),
			wantCode:   1,
			wantEvents: []string{"prepare", "list", "stderr"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var events []string
			code := cli.Run(
				tt.args,
				outputEventWriter{stream: "stdout", events: &events},
				outputEventWriter{stream: "stderr", events: &events},
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					events = append(events, "list")
					return []workspace.Space{{Name: "default", Active: true}}, tt.readErr
				},
				nil,
				func() { events = append(events, "prepare") },
			)
			if code != tt.wantCode {
				t.Errorf("exit=%d, want %d", code, tt.wantCode)
			}
			if !slices.Equal(events, tt.wantEvents) {
				t.Errorf("events=%v, want %v", events, tt.wantEvents)
			}
		})
	}
}

func TestRunHelpIncludesSpaceList(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run(
		[]string{"help"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		nil,
		nil,
		nil,
		nil,
	)
	if code != 0 || stderr.Len() != 0 {
		t.Errorf("exit=%d stderr=%q, want 0 and empty", code, stderr.String())
	}
	for _, syntax := range []string{
		"aidlc space list [--json] [--project-dir <path>]",
		"aidlc space [--json] [--project-dir <path>]",
	} {
		if !strings.Contains(stdout.String(), syntax) {
			t.Errorf("help is missing %q: %q", syntax, stdout.String())
		}
	}
}

func TestRunSpaceListFlagPositions(t *testing.T) {
	t.Parallel()

	const projectDir = "relative project/with spaces"
	commands := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"space", "list"}},
		{name: "bare", args: []string{"space"}},
	}
	for _, command := range commands {
		for jsonPosition := range len(command.args) + 1 {
			for projectPosition := range len(command.args) + 2 {
				for _, equalsForm := range []bool{false, true} {
					name := fmt.Sprintf(
						"%s/json=%d/project=%d/equals=%t",
						command.name,
						jsonPosition,
						projectPosition,
						equalsForm,
					)
					t.Run(name, func(t *testing.T) {
						t.Parallel()

						args := slices.Insert(slices.Clone(command.args), jsonPosition, "--json")
						flag := []string{"--project-dir", projectDir}
						if equalsForm {
							flag = []string{"--project-dir=" + projectDir}
						}
						args = slices.Insert(args, projectPosition, flag...)
						var stdout, stderr bytes.Buffer
						calls := 0
						code := cli.Run(
							args,
							&stdout,
							&stderr,
							buildinfo.Info{},
							nil,
							func(explicitDir string) ([]workspace.Space, error) {
								calls++
								if explicitDir != projectDir {
									t.Errorf("explicitDir=%q, want %q", explicitDir, projectDir)
								}
								return []workspace.Space{{Name: "default", Active: true}}, nil
							},
							nil,
							nil,
						)
						if code != 0 || calls != 1 {
							t.Errorf(
								"args=%q exit=%d calls=%d, want 0, 1",
								args,
								code,
								calls,
							)
						}
						const want = "{\"active\":\"default\",\"spaces\":[{\"name\":\"default\",\"active\":true}]}\n"
						if got := stdout.String(); got != want {
							t.Errorf("stdout=%q, want %q", got, want)
						}
						if got := stderr.String(); got != "" {
							t.Errorf("stderr=%q, want empty", got)
						}
					})
				}
			}
		}
	}
}

func TestRunSpaceListUnknownSubcommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "separate JSON value after bare", args: []string{"space", "--json", "false"}},
		{name: "unknown", args: []string{"space", "unknown"}},
		{name: "bare help positional", args: []string{"space", "help"}},
		{name: "list is not the command", args: []string{"other", "space", "list", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				nil,
				nil,
				func() { t.Error("unknown subcommand ran the space output hook") },
			)
			if code != 2 || stdout.Len() != 0 {
				t.Errorf("exit=%d stdout=%q, want 2 and empty", code, stdout.String())
			}
			want := fmt.Sprintf("aidlc: unknown arguments: %q\n\n", strings.Join(tt.args, " ")) + wantHelp
			if got := stderr.String(); got != want {
				t.Errorf("stderr=%q, want original unknown-command diagnostic %q", got, want)
			}
		})
	}
}

func TestRunSpaceListInvalidFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags []string
	}{
		{name: "unknown", flags: []string{"--force"}},
		{name: "help", flags: []string{"--help"}},
		{name: "short help", flags: []string{"-h"}},
		{name: "end marker", flags: []string{"--"}},
		{name: "JSON true value", flags: []string{"--json=true"}},
		{name: "JSON false value", flags: []string{"--json=false"}},
		{name: "JSON empty value", flags: []string{"--json="}},
		{name: "missing project", flags: []string{"--project-dir"}},
		{name: "empty split project", flags: []string{"--project-dir", ""}},
		{name: "empty equals project", flags: []string{"--project-dir="}},
		{name: "duplicate project", flags: []string{"--project-dir=one", "--project-dir", "two"}},
		{name: "flag in project value", flags: []string{"--project-dir", "--force"}},
		{name: "split dash project", flags: []string{"--project-dir", "-dir"}},
	}
	for _, bare := range []bool{false, true} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("bare=%t/%s", bare, tt.name), func(t *testing.T) {
				t.Parallel()

				args := []string{"space"}
				if !bare {
					args = append(args, "list")
				}
				args = append(args, tt.flags...)
				var stdout, stderr bytes.Buffer
				code := cli.Run(
					args,
					&stdout,
					&stderr,
					buildinfo.Info{},
					nil,
					func(string) ([]workspace.Space, error) {
						t.Error("invalid flags invoked the list callback")
						return nil, nil
					},
					nil,
					nil,
				)
				if code != 1 || stdout.Len() != 0 {
					t.Errorf("exit=%d stdout=%q, want 1 and empty", code, stdout.String())
				}
				assertSpaceErrorJSON(t, stderr.String())
			})
		}
	}
}

func TestRunSpaceListProjectDirLiteral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		flags []string
		want  string
	}{
		{name: "equals dash", flags: []string{"--project-dir=-dir"}, want: "-dir"},
		{name: "relative dash", flags: []string{"--project-dir", "./-dir"}, want: "./-dir"},
		{name: "whitespace", flags: []string{"--project-dir", "   "}, want: "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				append([]string{"space", "list"}, tt.flags...),
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(explicitDir string) ([]workspace.Space, error) {
					calls++
					if explicitDir != tt.want {
						t.Errorf("explicitDir=%q, want %q", explicitDir, tt.want)
					}
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 0 || calls != 1 {
				t.Errorf("exit=%d calls=%d, want 0, 1", code, calls)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr=%q, want empty", stderr.String())
			}
		})
	}
}

func TestRunSpaceListJSONPreservesRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		spaces     []workspace.Space
		wantActive string
	}{
		{
			name:       "unknown cursor leaves all rows inactive",
			spaces:     []workspace.Space{{Name: "alpha"}, {Name: "default"}},
			wantActive: "default",
		},
		{
			name:       "escaped names",
			spaces:     []workspace.Space{{Name: "Team \"A\"\\line\n<&>\u2028日本語", Active: true}, {Name: "default"}},
			wantActive: "Team \"A\"\\line\n<&>\u2028日本語",
		},
		{
			name:       "first active row without sorting or rewriting",
			spaces:     []workspace.Space{{Name: "zeta", Active: true}, {Name: "alpha", Active: true}},
			wantActive: "zeta",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			spaces := slices.Clone(tt.spaces)
			code := cli.Run(
				[]string{"space", "list", "--json"},
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) { return spaces, nil },
				nil,
				nil,
			)
			if code != 0 || stderr.Len() != 0 {
				t.Errorf("exit=%d stderr=%q, want 0 and empty", code, stderr.String())
			}
			if strings.Count(stdout.String(), "\n") != 1 || !strings.HasSuffix(stdout.String(), "\n") {
				t.Errorf("stdout=%q, want one LF-terminated JSON line", stdout.String())
			}
			var got struct {
				Active string            `json:"active"`
				Spaces []workspace.Space `json:"spaces"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Active != tt.wantActive || !slices.Equal(got.Spaces, tt.spaces) {
				t.Errorf("JSON=%+v, want active=%q and unchanged rows", got, tt.wantActive)
			}
			if !slices.Equal(spaces, tt.spaces) {
				t.Error("list rendering mutated the callback's rows")
			}
		})
	}
}

func TestRunSpaceListReadFailure(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"human", "JSON"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			args := []string{"space", "list"}
			if format == "JSON" {
				args = append(args, "--json")
			}
			cause := errors.New("read \"project\"\nfailed & 日本語")
			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					calls++
					return []workspace.Space{{Name: "discarded", Active: true}}, cause
				},
				nil,
				nil,
			)
			if code != 1 || calls != 1 {
				t.Errorf("exit=%d calls=%d, want 1 each", code, calls)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout=%q, want no listing on read error", stdout.String())
			}
			if got := assertSpaceErrorJSON(t, stderr.String()); got != cause.Error() {
				t.Errorf("JSON error=%q, want %q", got, cause.Error())
			}
		})
	}
}

func TestRunSpaceListPartialStdoutFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{name: "human", args: []string{"space", "list"}, wantPrefix: "Spa"},
		{name: "JSON", args: []string{"space", "list", "--json"}, wantPrefix: "{\"a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := &partialListWriter{err: errors.New("output closed")}
			var stderr bytes.Buffer
			code := cli.Run(
				tt.args,
				stdout,
				&stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					return []workspace.Space{{Name: "default", Active: true}}, nil
				},
				nil,
				nil,
			)
			if code != 1 {
				t.Errorf("exit=%d, want 1", code)
			}
			if got := stdout.output.String(); got != tt.wantPrefix {
				t.Errorf("stdout prefix=%q, want retained %q", got, tt.wantPrefix)
			}
			if got := assertSpaceErrorJSON(t, stderr.String()); got != "write stdout: output closed" {
				t.Errorf("JSON error=%q, want stdout write failure", got)
			}
		})
	}
}

func TestRunSpaceListStderrFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		readErr     error
		failStdout  bool
		shortStderr bool
		wantCalls   int
	}{
		{name: "syntax", args: []string{"space", "list", "--json=false"}},
		{name: "short stderr", args: []string{"space", "--json=false"}, shortStderr: true},
		{
			name:      "reader",
			args:      []string{"space", "list"},
			readErr:   errors.New("read failed"),
			wantCalls: 1,
		},
		{
			name:       "both streams",
			args:       []string{"space", "list", "--json"},
			failStdout: true,
			wantCalls:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			var stdout io.Writer = &output
			if tt.failStdout {
				stdout = errorWriter{err: errors.New("stdout unavailable")}
			}
			var stderr io.Writer = errorWriter{err: errors.New("stderr unavailable")}
			if tt.shortStderr {
				stderr = shortOutputWriter{}
			}
			calls := 0
			code := cli.Run(
				tt.args,
				stdout,
				stderr,
				buildinfo.Info{},
				nil,
				func(string) ([]workspace.Space, error) {
					calls++
					return []workspace.Space{{Name: "default", Active: true}}, tt.readErr
				},
				nil,
				nil,
			)
			if code != 1 || calls != tt.wantCalls {
				t.Errorf(
					"exit=%d calls=%d, want 1 and %d",
					code,
					calls,
					tt.wantCalls,
				)
			}
			if output.Len() != 0 {
				t.Errorf("stdout=%q, want empty", output.String())
			}
		})
	}
}

type partialListWriter struct {
	output bytes.Buffer
	err    error
}

func (w *partialListWriter) Write(p []byte) (int, error) {
	n, _ := w.output.Write(p[:min(3, len(p))])
	return n, w.err
}
