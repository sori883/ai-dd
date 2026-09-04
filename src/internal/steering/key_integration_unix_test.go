//go:build integration && (darwin || linux)

package steering

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOrCreateContinuationKeyIntegrationUses0600(t *testing.T) {
	tests := []struct {
		name      string
		withState bool
	}{
		{name: "active record", withState: true},
		{name: "session fallback", withState: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			projectPath := filepath.Join(root, "project")
			recordPath := filepath.Join(root, "record")
			if err := os.MkdirAll(projectPath, 0o755); err != nil {
				t.Fatalf("create project directory: %v", err)
			}
			if err := os.MkdirAll(recordPath, 0o755); err != nil {
				t.Fatalf("create record directory: %v", err)
			}
			if test.withState {
				if err := os.WriteFile(filepath.Join(recordPath, "aidlc-state.md"), []byte("# state\n"), 0o644); err != nil {
					t.Fatalf("create state file: %v", err)
				}
			}

			projectRoot, err := os.OpenRoot(projectPath)
			if err != nil {
				t.Fatalf("open project root: %v", err)
			}
			defer projectRoot.Close()
			recordRoot, err := os.OpenRoot(recordPath)
			if err != nil {
				t.Fatalf("open record root: %v", err)
			}
			defer recordRoot.Close()

			key, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
			if err != nil {
				t.Fatalf("ReadOrCreateContinuationKey() error = %v", err)
			}
			if len(key) != 32 {
				t.Fatalf("ReadOrCreateContinuationKey() key length = %d, want 32", len(key))
			}

			keyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
			if !test.withState {
				keyPath = filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key")
			}
			keyInfo, err := os.Stat(keyPath)
			if err != nil {
				t.Fatalf("stat continuation key: %v", err)
			}
			if got := keyInfo.Mode().Perm(); got != 0o600 {
				t.Errorf("continuation key mode = %o, want 600", got)
			}
			if _, err := projectRoot.Stat("."); err != nil {
				t.Errorf("project root unavailable after call: %v", err)
			}
			if _, err := recordRoot.Stat("."); err != nil {
				t.Errorf("record root unavailable after call: %v", err)
			}
		})
	}
}
