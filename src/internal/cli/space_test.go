package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sori883/ai-dd/src/internal/buildinfo"
	"github.com/sori883/ai-dd/src/internal/cli"
)

func TestRunSpaceCreate(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"space", "create", "Team Alpha"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		func(rawName, explicitDir string) (string, error) {
			calls++
			if rawName != "Team Alpha" || explicitDir != "" {
				t.Errorf("callback(%q, %q), want (Team Alpha, empty)", rawName, explicitDir)
			}
			return "team-alpha", nil
		},
	)
	if calls != 1 || code != 0 {
		t.Errorf("callback calls = %d, exit code = %d; want 1, 0", calls, code)
	}
	if got := stdout.String(); got != "Space created: team-alpha\n" {
		t.Errorf("stdout = %q, want single success line", got)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}
}

func TestRunSpaceCreateProjectDirPositions(t *testing.T) {
	t.Parallel()

	const projectDir = "relative project/with spaces"
	for _, equalsForm := range []bool{false, true} {
		for position := range 4 {
			t.Run(fmt.Sprintf("equals=%t position=%d", equalsForm, position), func(t *testing.T) {
				t.Parallel()

				command := []string{"space", "create", "Team Alpha"}
				flag := []string{"--project-dir", projectDir}
				if equalsForm {
					flag = []string{"--project-dir=" + projectDir}
				}
				args := append([]string{}, command[:position]...)
				args = append(args, flag...)
				args = append(args, command[position:]...)
				var stdout, stderr bytes.Buffer
				calls := 0
				code := cli.Run(
					args,
					&stdout,
					&stderr,
					buildinfo.Info{},
					func(rawName, explicitDir string) (string, error) {
						calls++
						if rawName != "Team Alpha" || explicitDir != projectDir {
							t.Errorf(
								"callback(%q, %q), want (Team Alpha, %q)",
								rawName,
								explicitDir,
								projectDir,
							)
						}
						return "team-alpha", nil
					},
				)
				if code != 0 || calls != 1 || stdout.String() != "Space created: team-alpha\n" || stderr.Len() != 0 {
					t.Errorf(
						"exit=%d calls=%d stdout=%q stderr=%q",
						code,
						calls,
						stdout.String(),
						stderr.String(),
					)
				}
			})
		}
	}
}

func TestRunSpaceCreateFailureJSON(t *testing.T) {
	t.Parallel()

	for _, message := range []string{"permission denied", "quoted \"error\"\nwith newline & 日本語"} {
		t.Run(message, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				[]string{"space", "create", "team"},
				&stdout,
				&stderr,
				buildinfo.Info{},
				func(string, string) (string, error) {
					calls++
					return "ignored-on-error", errors.New(message)
				},
			)
			if calls != 1 || code != 1 || stdout.Len() != 0 {
				t.Errorf(
					"calls=%d exit=%d stdout=%q; want 1, 1, empty",
					calls,
					code,
					stdout.String(),
				)
			}
			if got := assertSpaceErrorJSON(t, stderr.String()); got != message {
				t.Errorf("JSON error = %q, want %q", got, message)
			}
		})
	}
}

func TestRunSpaceCreateStdoutFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	calls := 0
	code := cli.Run(
		[]string{"space", "create", "team"},
		errorWriter{err: errors.New("broken pipe")},
		&stderr,
		buildinfo.Info{},
		func(string, string) (string, error) {
			calls++
			return "team", nil
		},
	)
	if code != 1 || calls != 1 {
		t.Errorf("exit=%d calls=%d, want 1 each", code, calls)
	}
	if message := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(message, "broken pipe") {
		t.Errorf("JSON error = %q, want stdout error cause", message)
	}
}

func TestRunSpaceCreateInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing name", args: []string{"space", "create"}},
		{name: "empty name", args: []string{"space", "create", ""}},
		{name: "help name", args: []string{"space", "create", "help"}},
		{name: "short help name", args: []string{"space", "create", "-h"}},
		{name: "extra name", args: []string{"space", "create", "team", "extra"}},
		{name: "unknown flag after name", args: []string{"space", "create", "team", "--force"}},
		{name: "unknown flag before command", args: []string{"--force", "space", "create", "team"}},
		{name: "unknown flag between commands", args: []string{"space", "--force", "create", "team"}},
		{name: "unknown short flag", args: []string{"space", "create", "team", "-x"}},
		{name: "unknown flag equals", args: []string{"space", "create", "team", "--name=other"}},
		{name: "missing project dir", args: []string{"space", "create", "team", "--project-dir"}},
		{name: "empty project dir", args: []string{"space", "create", "team", "--project-dir", ""}},
		{name: "empty equals project dir", args: []string{"--project-dir=", "space", "create", "team"}},
		{
			name: "duplicate project dir",
			args: []string{"--project-dir", "first", "space", "create", "team", "--project-dir", "second"},
		},
		{
			name: "duplicate mixed project dir",
			args: []string{"--project-dir=first", "space", "create", "team", "--project-dir", "second"},
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
				func(string, string) (string, error) {
					calls++
					return "must-not-create", nil
				},
			)
			if code != 1 || calls != 0 || stdout.Len() != 0 {
				t.Errorf(
					"exit=%d calls=%d stdout=%q, want 1, 0, empty",
					code,
					calls,
					stdout.String(),
				)
			}
			assertSpaceErrorJSON(t, stderr.String())
		})
	}
}

func TestRunSpaceCreateDashProjectDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		flag        []string
		wantDir     string
		wantFailure bool
	}{
		{name: "flag is not a split path", flag: []string{"--project-dir", "--force"}, wantFailure: true},
		{name: "equals dash path", flag: []string{"--project-dir=-dir"}, wantDir: "-dir"},
		{name: "dot slash dash path", flag: []string{"--project-dir", "./-dir"}, wantDir: "./-dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := append([]string{"space", "create", "team"}, tt.flag...)
			var stdout, stderr bytes.Buffer
			calls := 0
			code := cli.Run(
				args,
				&stdout,
				&stderr,
				buildinfo.Info{},
				func(_ string, explicitDir string) (string, error) {
					calls++
					if explicitDir != tt.wantDir {
						t.Errorf("project dir = %q, want %q", explicitDir, tt.wantDir)
					}
					return "team", nil
				},
			)
			if tt.wantFailure {
				if code != 1 || calls != 0 || stdout.Len() != 0 {
					t.Errorf(
						"exit=%d calls=%d stdout=%q, want 1, 0, empty",
						code,
						calls,
						stdout.String(),
					)
				}
				assertSpaceErrorJSON(t, stderr.String())
			} else if code != 0 || calls != 1 || stderr.Len() != 0 {
				t.Errorf(
					"exit=%d calls=%d stderr=%q, want 0, 1, empty",
					code,
					calls,
					stderr.String(),
				)
			}
		})
	}
}

func TestRunHelpIncludesSpaceCreate(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := cli.Run(
		[]string{"help"},
		&stdout,
		&stderr,
		buildinfo.Info{},
		func(string, string) (string, error) {
			t.Error("help called create callback")
			return "", nil
		},
	)
	if code != 0 || !strings.Contains(stdout.String(), "aidlc space create <name> [--project-dir <path>]") {
		t.Errorf("exit=%d stdout=%q, want help with creation syntax", code, stdout.String())
	}
}

func TestRunSpaceCreateShortStdoutWrite(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := cli.Run(
		[]string{"space", "create", "team"},
		shortOutputWriter{},
		&stderr,
		buildinfo.Info{},
		func(string, string) (string, error) { return "team", nil },
	)
	if code != 1 {
		t.Errorf("short stdout write exit = %d, want 1", code)
	}
	if message := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(message, "short write") {
		t.Errorf("JSON error = %q, want short-write cause", message)
	}
}

func TestRunSpaceCreateStderrFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := cli.Run(
		[]string{"space", "create", "team"},
		&stdout,
		errorWriter{err: errors.New("stderr unavailable")},
		buildinfo.Info{},
		func(string, string) (string, error) { return "", errors.New("creation failed") },
	)
	if code != 1 || stdout.Len() != 0 {
		t.Errorf("exit=%d stdout=%q, want 1 and empty", code, stdout.String())
	}
}

type shortOutputWriter struct{}

func (shortOutputWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func assertSpaceErrorJSON(t *testing.T, output string) string {
	t.Helper()

	if strings.Count(output, "\n") != 1 || !strings.HasSuffix(output, "\n") {
		t.Errorf("stderr = %q, want exactly one JSON line", output)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("stderr is not a JSON error object: %q: %v", output, err)
	}
	if len(payload) != 1 || payload["error"] == "" {
		t.Errorf("stderr JSON = %v, want one nonempty error field", payload)
	}
	return payload["error"]
}
