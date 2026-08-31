//go:build unix

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func TestMainSpaceList(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	teamDir := filepath.Join(
		project,
		"aidlc",
		"spaces",
		"team",
	)
	if err := os.MkdirAll(teamDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cursor := filepath.Join(project, "aidlc", "active-space")
	if err := os.WriteFile(cursor, []byte("team\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := mainTreeSnapshot(t, project)
	cmd := mainProcess(
		t,
		"space",
		"list",
		"--project-dir",
		project,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	state := runMainProcess(t, cmd)
	if got := state.ExitCode(); got != 0 {
		t.Errorf(
			"main exit=%d (%s), want 0; stderr=%q",
			got,
			state,
			stderr.String(),
		)
	}
	if got := stdout.String(); got != "Spaces:\n  default\n* team\n" {
		t.Errorf("stdout=%q, want project space listing", got)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("stderr=%q, want empty", got)
	}
	if after := mainTreeSnapshot(t, project); !maps.Equal(before, after) {
		t.Error("space list changed the project")
	}
}

func TestMainSpaceListClosedPipes(t *testing.T) {
	t.Parallel()

	commands := []struct {
		name string
		args []string
	}{
		{name: "list human", args: []string{"space", "list"}},
		{name: "list JSON", args: []string{"space", "list", "--json"}},
		{name: "bare human", args: []string{"space"}},
		{name: "bare JSON", args: []string{"space", "--json"}},
	}
	failures := []struct {
		name        string
		closeStdout bool
		closeStderr bool
		invalidFlag bool
		missingRoot bool
	}{
		{name: "stdout", closeStdout: true},
		{name: "stderr syntax", closeStderr: true, invalidFlag: true},
		{name: "stderr root error", closeStderr: true, missingRoot: true},
		{name: "both", closeStdout: true, closeStderr: true},
	}
	for _, command := range commands {
		for _, failure := range failures {
			t.Run(command.name+"/"+failure.name, func(t *testing.T) {
				t.Parallel()

				project := t.TempDir()
				if err := os.WriteFile(filepath.Join(project, "keep.txt"), []byte("unchanged\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				before := mainTreeSnapshot(t, project)
				projectDir := project
				if failure.missingRoot {
					projectDir = filepath.Join(project, "missing")
				}
				args := append(slices.Clone(command.args), "--project-dir", projectDir)
				if failure.invalidFlag {
					args = append(args, "--json=false")
				}
				cmd := mainProcess(t, args...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if failure.closeStdout {
					cmd.Stdout = closedPipeWriter(t)
				}
				if failure.closeStderr {
					cmd.Stderr = closedPipeWriter(t)
				}
				state := runMainProcess(t, cmd)
				if got := state.ExitCode(); got != 1 {
					t.Errorf("main exit=%d (%s), want 1", got, state)
				}
				if stdout.Len() != 0 {
					t.Errorf("stdout=%q, want empty for an unread pipe or early error", stdout.String())
				}
				if failure.closeStderr {
					if stderr.Len() != 0 {
						t.Errorf("closed stderr=%q, want empty", stderr.String())
					}
				} else if message := mainErrorJSON(t, stderr.String()); !strings.Contains(message, "write stdout:") {
					t.Errorf("JSON error=%q, want stdout failure", message)
				}
				if after := mainTreeSnapshot(t, project); !maps.Equal(before, after) {
					t.Error("failed list output changed the project")
				}
			})
		}
	}
}

func TestMainSpaceCreateClosedPipes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		closeStdout bool
		closeStderr bool
		wantCreated bool
	}{
		{
			name:        "stdout",
			args:        []string{"space", "create", "Pipe Target"},
			closeStdout: true,
			wantCreated: true,
		},
		{
			name:        "stderr missing name",
			args:        []string{"space", "create"},
			closeStderr: true,
		},
		{
			name:        "stderr invalid flag",
			args:        []string{"space", "create", "Pipe Target", "--force"},
			closeStderr: true,
		},
		{
			name:        "both",
			args:        []string{"space", "create", "Pipe Target"},
			closeStdout: true,
			closeStderr: true,
			wantCreated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			args := append(slices.Clone(tt.args), "--project-dir", project)
			cmd := mainProcess(t, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if tt.closeStdout {
				cmd.Stdout = closedPipeWriter(t)
			}
			if tt.closeStderr {
				cmd.Stderr = closedPipeWriter(t)
			}
			state := runMainProcess(t, cmd)
			if got := state.ExitCode(); got != 1 {
				t.Errorf("main exit = %d (%s), want 1", got, state)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if tt.closeStderr {
				if stderr.Len() != 0 {
					t.Errorf("closed stderr = %q, want empty", stderr.String())
				}
			} else if message := mainErrorJSON(t, stderr.String()); !strings.Contains(message, "write stdout:") {
				t.Errorf("stderr JSON error = %q, want stdout error", message)
			}
			if tt.wantCreated {
				assertSpaceRetainedAfterOutputFailure(t, project, args)
			} else if entries, err := os.ReadDir(project); err != nil || len(entries) != 0 {
				t.Errorf("project entries = %v, error = %v, want empty project", entries, err)
			}
		})
	}
}

func TestMainRootCommandsKeepSIGPIPE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		closeStderr bool
	}{
		{name: "no arguments"},
		{name: "help", args: []string{"help"}},
		{name: "help flag", args: []string{"--help"}},
		{name: "version", args: []string{"version"}},
		{name: "version flag", args: []string{"--version"}},
		{name: "unknown", args: []string{"unknown"}, closeStderr: true},
		{name: "unknown space subcommand", args: []string{"space", "unknown"}, closeStderr: true},
		{name: "bare JSON separate value", args: []string{"space", "--json", "false"}, closeStderr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := mainProcess(t, tt.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if tt.closeStderr {
				cmd.Stderr = closedPipeWriter(t)
			} else {
				cmd.Stdout = closedPipeWriter(t)
			}
			state := runMainProcess(t, cmd)
			status, ok := state.Sys().(syscall.WaitStatus)
			if !ok {
				t.Fatalf("process status has unexpected type %T", state.Sys())
			}
			if !status.Signaled() || status.Signal() != syscall.SIGPIPE {
				t.Errorf("main state = %s, want original SIGPIPE behavior", state)
			}
		})
	}
}

// TestMainProcessHelper runs only in an isolated child, so real main owns signals and exit.
func TestMainProcessHelper(t *testing.T) {
	if os.Getenv("AIDLC_TEST_MAIN_PROCESS") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator == -1 {
		t.Fatal("main subprocess arguments are missing --")
	}
	os.Args = append([]string{os.Args[0]}, os.Args[separator+1:]...)
	main()
}

func mainProcess(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()

	helperArgs := append([]string{"-test.run=^TestMainProcessHelper$", "--"}, args...)
	cmd := exec.Command(os.Args[0], helperArgs...)
	cmd.Dir = t.TempDir()
	// Give child coverage its own directory so runtime diagnostics do not pollute command stderr.
	cmd.Env = append(os.Environ(), "AIDLC_TEST_MAIN_PROCESS=1", "GOCOVERDIR="+t.TempDir())
	return cmd
}

func runMainProcess(t *testing.T, cmd *exec.Cmd) *os.ProcessState {
	t.Helper()

	if err := cmd.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			t.Fatalf("main process error = %v, want success or exit error", err)
		}
	}
	return cmd.ProcessState
}

func closedPipeWriter(t *testing.T) *os.File {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("close pipe writer: %v", err)
		}
	})
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return writer
}

func mainErrorJSON(t *testing.T, output string) string {
	t.Helper()

	var message map[string]string
	if err := json.Unmarshal([]byte(output), &message); err != nil {
		t.Fatalf("stderr = %q, want JSON error: %v", output, err)
	}
	if len(message) != 1 || message["error"] == "" {
		t.Errorf("stderr JSON = %v, want one nonempty error field", message)
	}
	if strings.Count(output, "\n") != 1 || !strings.HasSuffix(output, "\n") {
		t.Errorf("stderr = %q, want exactly one newline-terminated JSON line", output)
	}
	return message["error"]
}

func assertSpaceRetainedAfterOutputFailure(t *testing.T, project string, args []string) {
	t.Helper()

	target := filepath.Join(
		project,
		"aidlc",
		"spaces",
		"pipe-target",
	)
	before := mainTreeSnapshot(t, target)
	directories := []string{".", "memory", "memory/phases", "memory/templates", "intents", "codekb", "knowledge"}
	files := map[string]string{
		"memory/org.md":             "# Organization defaults\n",
		"memory/team.md":            "# Team practices\n",
		"memory/project.md":         "# Project overrides\n",
		"memory/templates/.gitkeep": "",
		"codekb/.gitkeep":           "",
		"knowledge/.gitkeep":        "",
	}
	if len(before) != len(directories)+len(files) {
		t.Errorf("retained space has %d entries, want 7 directories and 6 files", len(before))
	}
	for _, path := range directories {
		if entry, ok := before[path]; !ok || !entry.mode.IsDir() {
			t.Errorf("retained space is missing directory %q", path)
		}
	}
	for path, wantBody := range files {
		entry, ok := before[path]
		validFile := ok && entry.mode.IsRegular() && entry.body == wantBody
		if !validFile {
			t.Errorf(
				"retained file %q = %+v, want body %q",
				path,
				entry,
				wantBody,
			)
		}
	}

	cmd := mainProcess(t, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	state := runMainProcess(t, cmd)
	if state.ExitCode() != 1 || stdout.Len() != 0 {
		t.Errorf("retry state = %s, stdout = %q; want exit 1 and no success output", state, stdout.String())
	}
	if message := mainErrorJSON(t, stderr.String()); !strings.Contains(message, syscall.EEXIST.Error()) {
		t.Errorf("retry error = %q, want existing-target error", message)
	}
	if after := mainTreeSnapshot(t, target); !maps.Equal(before, after) {
		t.Error("retry modified the completed space after output failure")
	}
}

type mainTreeEntry struct {
	mode    fs.FileMode
	modTime int64
	body    string
}

func mainTreeSnapshot(t *testing.T, directory string) map[string]mainTreeEntry {
	t.Helper()

	entries := map[string]mainTreeEntry{}
	root := os.DirFS(directory)
	err := fs.WalkDir(root, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var body []byte
		if info.Mode().IsRegular() {
			body, err = fs.ReadFile(root, path)
			if err != nil {
				return err
			}
		}
		entries[path] = mainTreeEntry{mode: info.Mode(), modTime: info.ModTime().UnixNano(), body: string(body)}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %q: %v", directory, err)
	}
	return entries
}
