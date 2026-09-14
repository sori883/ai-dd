package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
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
		[]string{"space", "create", "Team Alpha", "--project-dir=project"},
		&stdout,
		&stderr,
		buildinfo.Info{}, runDependencies(

			func(rawName, explicitDir string) (string, error) {
				calls++
				if rawName != "Team Alpha" || explicitDir != "project" {
					t.Errorf("callback(%q, %q), want (Team Alpha, project)", rawName, explicitDir)
				}
				return "team-alpha", nil
			},
			nil,
			nil,
			nil))

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
				buildinfo.Info{}, runDependencies(

					func(string, string) (string, error) {
						calls++
						return "ignored-on-error", errors.New(message)
					},
					nil,
					nil,
					nil))

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
		buildinfo.Info{}, runDependencies(

			func(string, string) (string, error) {
				calls++
				return "team", nil
			},
			nil,
			nil,
			nil))

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
		{name: "help mixed with name", args: []string{"space", "create", "help", "extra"}},
		{name: "extra name", args: []string{"space", "create", "team", "extra"}},
		{name: "unknown flag after name", args: []string{"space", "create", "team", "--force"}},
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
				buildinfo.Info{}, runDependencies(

					func(string, string) (string, error) {
						calls++
						return "must-not-create", nil
					},
					nil,
					nil,
					nil))

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

func TestRunSpaceCreateShortStdoutWrite(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := cli.Run(
		[]string{"space", "create", "team"},
		shortOutputWriter{},
		&stderr,
		buildinfo.Info{}, runDependencies(

			func(string, string) (string, error) { return "team", nil },
			nil,
			nil,
			nil))

	if code != 1 {
		t.Errorf("short stdout write exit = %d, want 1", code)
	}
	if message := assertSpaceErrorJSON(t, stderr.String()); !strings.Contains(message, "short write") {
		t.Errorf("JSON error = %q, want short-write cause", message)
	}
}

func TestRunSpaceOutputPreparation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		createErr    error
		wantCode     int
		wantPrepare  bool
		wantCallback bool
	}{
		{
			name:        "success",
			args:        []string{"space", "create", "team"},
			wantPrepare: true, wantCallback: true,
		},
		{
			name:        "callback failure",
			args:        []string{"space", "create", "team"},
			createErr:   errors.New("creation failed"),
			wantCode:    1,
			wantPrepare: true, wantCallback: true,
		},
		{
			name:        "missing name",
			args:        []string{"space", "create"},
			wantCode:    1,
			wantPrepare: true, wantCallback: false,
		},
		{name: "help", args: []string{"help"}, wantPrepare: false, wantCallback: false},
		{name: "version", args: []string{"version"}, wantPrepare: false, wantCallback: false},
		{
			name:        "unknown command",
			args:        []string{"unknown"},
			wantCode:    2,
			wantPrepare: false, wantCallback: false,
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
				buildinfo.Info{}, runDependencies(

					func(string, string) (string, error) {
						events = append(events, "create")
						return "team", tt.createErr
					},
					nil,
					nil,
					func() { events = append(events, "prepare") }))

			if code != tt.wantCode {
				t.Errorf("exit = %d, want %d", code, tt.wantCode)
			}
			assertOutputPreparation(t, events, "create", tt.wantPrepare, tt.wantCallback)
		})
	}
}

type outputEventWriter struct {
	stream string
	events *[]string
}

func (w outputEventWriter) Write(p []byte) (int, error) {
	*w.events = append(*w.events, w.stream)
	return len(p), nil
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

func assertOutputPreparation(t *testing.T, events []string, callback string, prepared, called bool) {
	t.Helper()
	count := 0
	for _, event := range events {
		if event == "prepare" {
			count++
		}
	}
	if prepared {
		if count != 1 || len(events) == 0 || events[0] != "prepare" {
			t.Fatalf("preparation must precede callback/output once: %v", events)
		}
	} else if count != 0 {
		t.Fatalf("unexpected preparation: %v", events)
	}
	if slices.Contains(events, callback) != called {
		t.Fatalf("callback presence: %v", events)
	}
}
