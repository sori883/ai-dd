//go:build integration

package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestWriteStateIntegrationPersistsExactBytesAndKeepsRoot(t *testing.T) {
	t.Parallel()

	recordDir := t.TempDir()
	oldState := []byte(canonicalStateContent())
	replacement := []byte(strings.Replace(
		canonicalStateContent(),
		"- [-] intent-capture — EXECUTE",
		"- [?] intent-capture — EXECUTE",
		1,
	))
	replacement = append([]byte{0xef, 0xbb, 0xbf}, replacement...)
	statePath := filepath.Join(recordDir, stateFile)
	if err := os.WriteFile(statePath, oldState, 0o600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	if err := WriteState(root, replacement); err != nil {
		t.Fatal(err)
	}
	got, err := root.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, replacement) {
		t.Errorf("state bytes = %q, want exact replacement %q", got, replacement)
	}
	if slices.Equal(got, oldState) {
		t.Error("state bytes were not replaced")
	}
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != stateFile {
		t.Errorf("record entries = %q, want only %q", entries, stateFile)
	}
	if _, err := root.Stat(stateFile); err != nil {
		t.Errorf("Root was closed after WriteState: %v", err)
	}
}

func TestWriteStateIntegrationRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	recordDir := t.TempDir()
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	err = WriteState(root, []byte(canonicalStateContent()))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("WriteState() error = %v, want fs.ErrNotExist", err)
	}
	if _, err := root.ReadFile(stateFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("state after missing-target rejection = %v, want fs.ErrNotExist", err)
	}
	assertNoStateWriterTemps(t, recordDir)
	if _, err := root.Stat("."); err != nil {
		t.Errorf("Root was closed after missing-target rejection: %v", err)
	}
}

func TestWriteStateIntegrationRejectsNonRegularTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(string) (skip bool, err error)
		check func(*testing.T, string)
	}{
		{
			name: "directory",
			setup: func(recordDir string) (bool, error) {
				stateDir := filepath.Join(recordDir, stateFile)
				if err := os.Mkdir(stateDir, 0o700); err != nil {
					return false, err
				}
				return false, os.WriteFile(filepath.Join(stateDir, "sentinel"), []byte("keep directory"), 0o600)
			},
			check: func(t *testing.T, recordDir string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if !info.IsDir() {
					t.Fatalf("state mode = %v, want directory", info.Mode())
				}
				got, err := os.ReadFile(filepath.Join(recordDir, stateFile, "sentinel"))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, []byte("keep directory")) {
					t.Errorf("directory sentinel = %q, want unchanged bytes", got)
				}
			},
		},
		{
			name: "symlink",
			setup: func(recordDir string) (bool, error) {
				if err := os.WriteFile(filepath.Join(recordDir, "state-target"), []byte("keep link target"), 0o600); err != nil {
					return false, err
				}
				if err := os.Symlink("state-target", filepath.Join(recordDir, stateFile)); err != nil {
					return true, err
				}
				return false, nil
			},
			check: func(t *testing.T, recordDir string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("state mode = %v, want symlink", info.Mode())
				}
				linkTarget, err := os.Readlink(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if linkTarget != "state-target" {
					t.Errorf("symlink target = %q, want unchanged target", linkTarget)
				}
				got, err := os.ReadFile(filepath.Join(recordDir, "state-target"))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, []byte("keep link target")) {
					t.Errorf("symlink target bytes = %q, want unchanged bytes", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recordDir := t.TempDir()
			skipSetup, err := tt.setup(recordDir)
			if err != nil {
				if skipSetup {
					t.Skipf("symlink unavailable on this platform: %v", err)
				}
				t.Fatal(err)
			}
			root, err := os.OpenRoot(recordDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("Root.Close() error = %v", err)
				}
			})

			err = WriteState(root, []byte(canonicalStateContent()))
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("WriteState() error = %v, want fs.ErrInvalid", err)
			}
			tt.check(t, recordDir)
			assertNoStateWriterTemps(t, recordDir)
			if _, err := root.Stat(stateFile); err != nil {
				t.Errorf("Root was closed after nonregular target rejection: %v", err)
			}
		})
	}
}

func TestWriteStateIntegrationRejectsReadOnlyTarget(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide reliable read-only permission semantics for this test")
	}
	recordDir := t.TempDir()
	statePath := filepath.Join(recordDir, stateFile)
	oldState := []byte(canonicalStateContent())
	if err := os.WriteFile(statePath, oldState, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(statePath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(statePath, 0o600); err != nil {
			t.Errorf("restore state permissions: %v", err)
		}
	})
	probe, err := os.OpenFile(statePath, os.O_WRONLY, 0)
	if err == nil {
		if err := probe.Close(); err != nil {
			t.Fatalf("close writable probe: %v", err)
		}
		t.Skip("filesystem permits O_WRONLY on 0444 files; read-only barrier is not observable")
	}

	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	err = WriteState(root, []byte(canonicalStateContent()))
	if err == nil {
		t.Fatal("WriteState() error = nil, want read-only barrier error")
	}
	got, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, oldState) {
		t.Errorf("state bytes = %q, want unchanged bytes", got)
	}
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o444 {
		t.Errorf("state mode = %o, want 0444", info.Mode().Perm())
	}
	assertNoStateWriterTemps(t, recordDir)
	if _, err := root.Stat(stateFile); err != nil {
		t.Errorf("Root was closed after read-only target rejection: %v", err)
	}
}

func TestWriteInitialIntegrationPersistsExactPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		initialState   string
		initialSidecar string
		existingState  bool
		existingDesc   bool
	}{
		{
			name:           "replaces existing stub",
			initialState:   "canonical state\n",
			initialSidecar: "canonical description\n",
			existingState:  true,
			existingDesc:   true,
		},
		{
			name:           "creates missing files",
			initialState:   "state without stub",
			initialSidecar: "description without prior file",
		},
		{
			name:           "accepts empty payloads",
			initialState:   "",
			initialSidecar: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recordDir := t.TempDir()
			if tt.existingState {
				if err := os.WriteFile(filepath.Join(recordDir, stateFile), []byte("# stub\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.existingDesc {
				if err := os.WriteFile(filepath.Join(recordDir, projectDescriptionFile), []byte("\"old\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			root, err := os.OpenRoot(recordDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("Root.Close() error = %v", err)
				}
			})

			if err := WriteInitial(root, Initial{
				ProjectDescriptionJSON: tt.initialSidecar,
				StateContent:           tt.initialState,
			}); err != nil {
				t.Fatal(err)
			}

			gotState, err := root.ReadFile(stateFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotState) != tt.initialState {
				t.Errorf("state bytes = %q, want exact %q", gotState, tt.initialState)
			}
			gotSidecar, err := root.ReadFile(projectDescriptionFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotSidecar) != tt.initialSidecar {
				t.Errorf("sidecar bytes = %q, want exact %q", gotSidecar, tt.initialSidecar)
			}

			entries, err := os.ReadDir(recordDir)
			if err != nil {
				t.Fatal(err)
			}
			entryNames := make([]string, len(entries))
			for index, entry := range entries {
				entryNames[index] = entry.Name()
			}
			slices.Sort(entryNames)
			wantNames := []string{projectDescriptionFile, stateFile}
			slices.Sort(wantNames)
			if !slices.Equal(entryNames, wantNames) {
				t.Errorf("record entries = %q, want only target files %q", entryNames, wantNames)
			}

			if _, err := root.Stat(stateFile); err != nil {
				t.Errorf("Root remained usable after WriteInitial: %v", err)
			}
		})
	}
}

func TestWriteInitialIntegrationKeepsNonRegularStateAfterSidecar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(string) (skip bool, err error)
		check func(*testing.T, string)
	}{
		{
			name: "directory",
			setup: func(recordDir string) (bool, error) {
				stateDir := filepath.Join(recordDir, stateFile)
				if err := os.Mkdir(stateDir, 0o700); err != nil {
					return false, err
				}
				return false, os.WriteFile(filepath.Join(stateDir, "sentinel"), []byte("keep directory contents"), 0o600)
			},
			check: func(t *testing.T, recordDir string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if !info.IsDir() {
					t.Fatalf("state mode = %v, want directory", info.Mode())
				}
				got, err := os.ReadFile(filepath.Join(recordDir, stateFile, "sentinel"))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, []byte("keep directory contents")) {
					t.Errorf("directory sentinel = %q, want unchanged contents", got)
				}
			},
		},
		{
			name: "symlink",
			setup: func(recordDir string) (bool, error) {
				if err := os.WriteFile(filepath.Join(recordDir, "state-target"), []byte("keep symlink target"), 0o600); err != nil {
					return false, err
				}
				if err := os.Symlink("state-target", filepath.Join(recordDir, stateFile)); err != nil {
					return true, err
				}
				return false, nil
			},
			check: func(t *testing.T, recordDir string) {
				t.Helper()
				info, err := os.Lstat(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("state mode = %v, want symlink", info.Mode())
				}
				linkTarget, err := os.Readlink(filepath.Join(recordDir, stateFile))
				if err != nil {
					t.Fatal(err)
				}
				if linkTarget != "state-target" {
					t.Errorf("symlink target = %q, want unchanged %q", linkTarget, "state-target")
				}
				got, err := os.ReadFile(filepath.Join(recordDir, "state-target"))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, []byte("keep symlink target")) {
					t.Errorf("symlink target bytes = %q, want unchanged contents", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recordDir := t.TempDir()
			skipSetup, err := tt.setup(recordDir)
			if err != nil {
				if skipSetup {
					t.Skipf("symlink unavailable on this platform: %v", err)
				}
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(recordDir, projectDescriptionFile), []byte("\"old\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(recordDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("Root.Close() error = %v", err)
				}
			})

			err = WriteInitial(root, Initial{
				ProjectDescriptionJSON: "new description",
				StateContent:           "new state",
			})
			if err == nil {
				t.Fatal("WriteInitial() error = nil, want nonregular state error")
			}
			gotSidecar, err := root.ReadFile(projectDescriptionFile)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(gotSidecar, []byte("new description")) {
				t.Errorf("sidecar bytes = %q, want committed bytes %q", gotSidecar, "new description")
			}
			tt.check(t, recordDir)
			assertNoInitialWriterTemps(t, recordDir)
			if _, err := root.Stat(projectDescriptionFile); err != nil {
				t.Errorf("Root remained usable after nonregular state error: %v", err)
			}
		})
	}
}

func TestWriteInitialIntegrationKeepsReadOnlyStateAfterSidecar(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Windows does not provide reliable read-only permission semantics for this test")
	}
	recordDir := t.TempDir()
	statePath := filepath.Join(recordDir, stateFile)
	oldState := []byte("old state bytes")
	if err := os.WriteFile(statePath, oldState, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(statePath, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(statePath, 0o600); err != nil {
			t.Errorf("restore state permissions: %v", err)
		}
	})
	probe, err := os.OpenFile(statePath, os.O_WRONLY, 0)
	if err == nil {
		if err := probe.Close(); err != nil {
			t.Fatalf("close writable probe: %v", err)
		}
		t.Skip("filesystem permits O_WRONLY on 0444 files; read-only barrier is not observable")
	}
	if err := os.WriteFile(filepath.Join(recordDir, projectDescriptionFile), []byte("\"old\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("Root.Close() error = %v", err)
		}
	})

	err = WriteInitial(root, Initial{
		ProjectDescriptionJSON: "new description",
		StateContent:           "new state",
	})
	if err == nil {
		t.Fatal("WriteInitial() error = nil, want read-only state barrier error")
	}
	gotSidecar, err := root.ReadFile(projectDescriptionFile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotSidecar, []byte("new description")) {
		t.Errorf("sidecar bytes = %q, want committed bytes %q", gotSidecar, "new description")
	}
	gotState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotState, oldState) {
		t.Errorf("state bytes = %q, want unchanged bytes %q", gotState, oldState)
	}
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o444 {
		t.Errorf("state mode = %o, want 0444", info.Mode().Perm())
	}
	assertNoInitialWriterTemps(t, recordDir)
	if _, err := root.Stat(projectDescriptionFile); err != nil {
		t.Errorf("Root remained usable after read-only state error: %v", err)
	}
}

func assertNoInitialWriterTemps(t *testing.T, recordDir string) {
	t.Helper()
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+projectDescriptionFile+"-") ||
			strings.HasPrefix(entry.Name(), "."+stateFile+"-") {
			t.Errorf("writer temporary %q remains", entry.Name())
		}
	}
}

func assertNoStateWriterTemps(t *testing.T, recordDir string) {
	t.Helper()
	entries, err := os.ReadDir(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+stateFile+"-") {
			t.Errorf("state writer temporary %q remains", entry.Name())
		}
	}
}
