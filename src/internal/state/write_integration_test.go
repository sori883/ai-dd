//go:build integration

package state

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

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

	recordDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(recordDir, stateFile), 0o700); err != nil {
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

	wantSidecar := []byte("new description")
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
	if !slices.Equal(gotSidecar, wantSidecar) {
		t.Errorf("sidecar bytes = %q, want committed bytes %q", gotSidecar, wantSidecar)
	}
}
