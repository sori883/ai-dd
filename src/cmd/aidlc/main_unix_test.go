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

func TestMainSpaceSwitchClosedPipes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		hasClosedStdout bool
		hasClosedStderr bool
		isRejected      bool
		wantCode        int
	}{
		{name: "success", args: []string{"space", "switch", "Team Alpha"}},
		{
			name: "stdout", args: []string{"space", "switch", "Team Alpha"},
			hasClosedStdout: true, wantCode: 1,
		},
		{
			name: "stderr syntax", args: []string{"space", "switch", "help"},
			hasClosedStderr: true, isRejected: true, wantCode: 1,
		},
		{
			name: "stderr workspace failure", args: []string{"space", "switch", "unknown"},
			hasClosedStderr: true, isRejected: true, wantCode: 1,
		},
		{
			name: "both", args: []string{"space", "switch", "Team Alpha"},
			hasClosedStdout: true, hasClosedStderr: true, wantCode: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			if err := os.MkdirAll(filepath.Join(
				project,
				"aidlc",
				"spaces",
				"team-alpha",
			), 0o700); err != nil {
				t.Fatal(err)
			}
			cursor := filepath.Join(project, "aidlc", "active-space")
			if err := os.WriteFile(cursor, []byte("old bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			before := mainTreeSnapshot(t, project)
			args := append(slices.Clone(tt.args), "--project-dir", project)
			cmd := mainProcess(t, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if tt.hasClosedStdout {
				cmd.Stdout = closedPipeWriter(t)
			}
			if tt.hasClosedStderr {
				cmd.Stderr = closedPipeWriter(t)
			}
			state := runMainProcess(t, cmd)
			if code := state.ExitCode(); code != tt.wantCode {
				t.Errorf(
					"main exit=%d (%s), want %d",
					code,
					state,
					tt.wantCode,
				)
			}
			wantCursor := "team-alpha\n"
			if tt.isRejected {
				wantCursor = "old bytes"
				if !maps.Equal(before, mainTreeSnapshot(t, project)) {
					t.Error("rejected switch changed the project")
				}
			}
			data, err := os.ReadFile(cursor)
			if err != nil || string(data) != wantCursor {
				t.Errorf(
					"cursor=(%q, %v), want %q without output-error rollback",
					data,
					err,
					wantCursor,
				)
			}
			wantStdout := ""
			if tt.wantCode == 0 {
				wantStdout = "Active space → team-alpha\n"
			}
			if stdout.String() != wantStdout {
				t.Errorf("stdout=%q, want %q", stdout.String(), wantStdout)
			}
			if tt.hasClosedStdout && !tt.hasClosedStderr {
				if message := mainErrorJSON(t, stderr.String()); !strings.Contains(message, "write stdout:") {
					t.Errorf("JSON error=%q, want stdout failure", message)
				}
			} else if stderr.Len() != 0 {
				t.Errorf("stderr=%q, want empty for success or closed stderr", stderr.String())
			}
		})
	}
}

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
		{name: "unknown intent subcommand", args: []string{"intent", "create"}, closeStderr: true},
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
	directories := []string{".", "knowledge", "knowledge/design", "knowledge/ADR", "knowledge/knowledge", "knowledge/rules"}
	files := map[string]string{
		"knowledge/design/index.md":    "# Index\n",
		"knowledge/index.md":           "---\nokf_version: \"0.2\"\n---\n# Space knowledge\n\n- [必須ルール](rules/entry.md): 作業前に読む文書。\n- [共有知識](knowledge/index.md)\n- [設計](design/index.md)\n- [判断理由](ADR/index.md)\n",
		"knowledge/ADR/index.md":       "# Architecture Decision Records\n\n判断理由と採用・却下した選択肢を置く。現行の仕様と手順はKnowledgeを参照する。\n",
		"knowledge/knowledge/index.md": "# Index\n",
		"knowledge/rules/entry.md":     "---\ntype: Rule\ntitle: 必須ルールの入口\ndescription: 作業前に以下のリンク順で本文を読む。\n---\n# 必須ルール\n\n- [作業の合意](rule.md)\n",
		"knowledge/rules/rule.md":      "---\ntype: Rule\ntitle: 四段階の作業合意\ndescription: 目的を理解し、計画・TDD・統合検証を独立レビューで進める。\nstatus: stable\n---\n# 作業の合意\n\nIntentは一つの目的。discovery、planning、tdd、integrationの順に進む。\n各境界と完了には現在のSensorと独立reviewのpassが必要。未実施、fail、対象変更後の古いpassで進めない。\nDiscoveryでは目的、範囲、受入条件、現状、制約を理解する。実装計画を妨げる未確定事項を確認する。\n結果を左右する判断は質問し、必要な調査・試作で理解する。全疑問ゼロや最初からUnit分割を要求しない。\nPlanningでは実装と検証の手順を定める。分割する場合Unitの担当範囲・依存・検証・Boltを具体化する。\n調整役AIが独立workerとreviewerを起動する。共有stateのwriterは調整役一人。\nworkerは別worktreeで担当範囲を実装し成果commitを返す。依存の統合前や重複割当では開始しない。\nTDDでは実行可能な失敗を観測してから最小実装、成功確認、整理を繰り返す。テスト不在やskipを成功としない。\nIntegrationでは実成果を統合し全体の受入を検証する。別rootのread-only reviewerへ対象版を渡す。\nreview失敗は修正して再reviewする。対象コード・計画・成果物の変更で古い結果を使わない。\nKnowledgeは現行what/how、ADRはwhyと代替案・影響。必要なADRだけ作り、不要なら理由をreviewする。\n文書はOKF metadataを保持する。一般知識は命令権限を持たない。合格目的でRuleを変えない。\n質問待ちはwait、中断はpause、再開はresume。進行中Unitは実run確認後confirmし、自動再実行しない。\n記録は現在の状態と必要な知識に限る。毎操作の日誌、全操作audit、一律ADRを作らない。\n",
	}
	if len(before) != len(directories)+len(files) {
		t.Errorf("retained space has %d entries, want 6 directories and 6 files", len(before))
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
