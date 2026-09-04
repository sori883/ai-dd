//go:build integration

package steering

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadOrCreateContinuationKeyCreatesRecordKey(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "project")
	recordPath := filepath.Join(root, "record")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("create record directory: %v", err)
	}

	statePath := filepath.Join(recordPath, "aidlc-state.md")
	if err := os.WriteFile(statePath, []byte("# state\n"), 0o644); err != nil {
		t.Fatalf("create state file: %v", err)
	}
	stateInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if !stateInfo.Mode().IsRegular() {
		t.Fatalf("state file mode = %v, want regular file", stateInfo.Mode())
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
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read record key file: %v", err)
	}
	wantKeyBytes := base64.RawURLEncoding.EncodeToString(key) + "\n"
	if string(keyBytes) != wantKeyBytes {
		t.Errorf("record key file = %q, want %q", string(keyBytes), wantKeyBytes)
	}

	projectKeyPath := filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key")
	if _, err := os.Stat(projectKeyPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("project session key stat error = %v, want os.ErrNotExist", err)
	}

	claims := ContinuationClaims{
		Version:       1,
		Stage:         "stage",
		Scope:         "scope",
		NextPart:      1,
		Bundle:        "bundle",
		DirectiveHash: "directive-hash",
		RouteHash:     "route-hash",
		StateAware:    false,
		Gate:          GateTrue,
	}
	token, err := EncodeContinuationToken(key, claims)
	if err != nil {
		t.Fatalf("EncodeContinuationToken() error = %v", err)
	}
	gotClaims, err := DecodeContinuationToken(key, token)
	if err != nil {
		t.Fatalf("DecodeContinuationToken() error = %v", err)
	}
	wantClaims := claims
	swarmSettled := false
	wantClaims.SwarmSettled = &swarmSettled
	if !reflect.DeepEqual(gotClaims, wantClaims) {
		t.Errorf("DecodeContinuationToken() = %#v, want %#v", gotClaims, wantClaims)
	}

	if _, err := projectRoot.Stat("."); err != nil {
		t.Errorf("project root unavailable after call: %v", err)
	}
	if _, err := recordRoot.Stat("aidlc-state.md"); err != nil {
		t.Errorf("record root unavailable after call: %v", err)
	}
}

func TestReadOrCreateContinuationKeyUsesSessionFallback(t *testing.T) {
	tests := []struct {
		name       string
		withRecord bool
	}{
		{name: "without active record", withRecord: false},
		{name: "record state absent", withRecord: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			projectPath := filepath.Join(root, "project")
			recordPath := filepath.Join(root, "record")
			if err := os.MkdirAll(projectPath, 0o755); err != nil {
				t.Fatalf("create project directory: %v", err)
			}

			projectRoot, err := os.OpenRoot(projectPath)
			if err != nil {
				t.Fatalf("open project root: %v", err)
			}
			defer projectRoot.Close()

			var recordRoot *os.Root
			if test.withRecord {
				if err := os.MkdirAll(recordPath, 0o755); err != nil {
					t.Fatalf("create record directory: %v", err)
				}
				recordRoot, err = os.OpenRoot(recordPath)
				if err != nil {
					t.Fatalf("open record root: %v", err)
				}
				defer recordRoot.Close()
			}

			key, err := ReadOrCreateContinuationKey(projectRoot, recordRoot)
			if err != nil {
				t.Fatalf("ReadOrCreateContinuationKey() error = %v", err)
			}
			if len(key) != 32 {
				t.Fatalf("ReadOrCreateContinuationKey() key length = %d, want 32", len(key))
			}

			projectKeyPath := filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key")
			keyBytes, err := os.ReadFile(projectKeyPath)
			if err != nil {
				t.Fatalf("read project session key file: %v", err)
			}
			wantKeyBytes := base64.RawURLEncoding.EncodeToString(key) + "\n"
			if string(keyBytes) != wantKeyBytes {
				t.Errorf("project session key file = %q, want %q", string(keyBytes), wantKeyBytes)
			}

			recordKeyPath := filepath.Join(recordPath, ".aidlc-steering-token-key")
			if _, err := os.Stat(recordKeyPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("record key stat error = %v, want os.ErrNotExist", err)
			}

			if _, err := projectRoot.Stat("."); err != nil {
				t.Errorf("project root unavailable after call: %v", err)
			}
			if recordRoot != nil {
				if _, err := recordRoot.Stat("."); err != nil {
					t.Errorf("record root unavailable after call: %v", err)
				}
			}
		})
	}
}

func TestReadOrCreateContinuationKeyIntegrationRejectsUnsafeLeaf(t *testing.T) {
	tests := []struct {
		name        string
		useRecord   bool
		nilProject  bool
		sessionLeaf bool
		setup       func(*unsafeContinuationKeyFixture)
	}{
		{
			name:       "nil project root",
			useRecord:  true,
			nilProject: true,
		},
		{
			name:      "state directory",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				f.replaceState(t)
				if err := os.Mkdir(f.statePath, 0o755); err != nil {
					t.Fatalf("create state directory: %v", err)
				}
			},
		},
		{
			name:      "state in-root symlink",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				f.replaceState(t)
				stateTarget := filepath.Join(f.recordPath, "state-target.md")
				if err := os.WriteFile(stateTarget, []byte("# target\n"), 0o640); err != nil {
					t.Fatalf("create in-root state target: %v", err)
				}
				if err := os.Symlink("state-target.md", f.statePath); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name:      "state root-out symlink",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				f.replaceState(t)
				if err := os.Symlink(f.outsidePath, f.statePath); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name:      "record key directory",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				if err := os.Mkdir(f.recordKeyPath, 0o755); err != nil {
					t.Fatalf("create record key directory: %v", err)
				}
			},
		},
		{
			name:      "record key absolute root-out symlink",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				if err := os.Symlink(f.outsidePath, f.recordKeyPath); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name:      "record key relative root-out symlink",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				if err := os.Symlink(filepath.Join("..", filepath.Base(f.outsidePath)), f.recordKeyPath); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name:      "record key in-root relative symlink",
			useRecord: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				f.createInRootRecordKeySymlink(t)
			},
		},
		{
			name:        "session key root-out symlink",
			sessionLeaf: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				if err := os.MkdirAll(filepath.Dir(f.sessionKeyPath), 0o755); err != nil {
					t.Fatalf("create session directory: %v", err)
				}
				if err := os.Symlink(f.outsidePath, f.sessionKeyPath); err != nil {
					t.Skipf("symlink unsupported: %v", err)
				}
			},
		},
		{
			name:        "session key in-root relative symlink",
			sessionLeaf: true,
			setup: func(f *unsafeContinuationKeyFixture) {
				f.createInRootSessionKeySymlink(t)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newUnsafeContinuationKeyFixture(t)
			if test.setup != nil {
				test.setup(fixture)
			}

			var projectRoot *os.Root
			if !test.nilProject {
				projectRoot = fixture.projectRoot
			}
			var recordRoot *os.Root
			if test.useRecord {
				recordRoot = fixture.recordRoot
			}
			key, err := callReadOrCreateContinuationKey(projectRoot, recordRoot)
			if err == nil {
				t.Fatal("ReadOrCreateContinuationKey() error = nil, want unsafe leaf error")
			}
			if !errors.Is(err, ErrInvalidContinuationKeyFile) {
				t.Errorf("ReadOrCreateContinuationKey() error = %v, want ErrInvalidContinuationKeyFile", err)
			}
			if len(key) != 0 {
				t.Errorf("ReadOrCreateContinuationKey() key length = %d, want zero", len(key))
			}

			unrelatedBytes, err := os.ReadFile(fixture.unrelatedPath)
			if err != nil {
				t.Fatalf("read unrelated sentinel: %v", err)
			}
			if !bytes.Equal(unrelatedBytes, fixture.unrelatedBytes) {
				t.Errorf("unrelated sentinel = %q, want %q", unrelatedBytes, fixture.unrelatedBytes)
			}
			unrelatedInfo, err := os.Stat(fixture.unrelatedPath)
			if err != nil {
				t.Fatalf("stat unrelated sentinel: %v", err)
			}
			if got := unrelatedInfo.Mode().Perm(); got != fixture.unrelatedMode {
				t.Errorf("unrelated sentinel mode = %o, want %o", got, fixture.unrelatedMode)
			}
			outsideBytes, err := os.ReadFile(fixture.outsidePath)
			if err != nil {
				t.Fatalf("read outside sentinel: %v", err)
			}
			if !bytes.Equal(outsideBytes, fixture.outsideBytes) {
				t.Errorf("outside sentinel = %q, want %q", outsideBytes, fixture.outsideBytes)
			}
			outsideInfo, err := os.Stat(fixture.outsidePath)
			if err != nil {
				t.Fatalf("stat outside sentinel: %v", err)
			}
			if got := outsideInfo.Mode().Perm(); got != fixture.outsideMode {
				t.Errorf("outside sentinel mode = %o, want %o", got, fixture.outsideMode)
			}
			if fixture.targetPath != "" {
				targetBytes, err := os.ReadFile(fixture.targetPath)
				if err != nil {
					t.Fatalf("read in-root target: %v", err)
				}
				if !bytes.Equal(targetBytes, fixture.targetBytes) {
					t.Errorf("in-root target = %q, want %q", targetBytes, fixture.targetBytes)
				}
				targetInfo, err := os.Stat(fixture.targetPath)
				if err != nil {
					t.Fatalf("stat in-root target: %v", err)
				}
				if got := targetInfo.Mode().Perm(); got != fixture.targetMode {
					t.Errorf("in-root target mode = %o, want %o", got, fixture.targetMode)
				}
				leafInfo, err := os.Lstat(fixture.targetLeafPath)
				if err != nil {
					t.Fatalf("lstat in-root key leaf: %v", err)
				}
				if leafInfo.Mode()&os.ModeSymlink == 0 {
					t.Errorf("in-root key leaf mode = %v, want symlink", leafInfo.Mode())
				}
			}

			if test.sessionLeaf {
				keyInfo, err := os.Lstat(fixture.sessionKeyPath)
				if err != nil {
					t.Fatalf("lstat session key leaf: %v", err)
				}
				if keyInfo.Mode()&os.ModeSymlink == 0 {
					t.Errorf("session key leaf mode = %v, want symlink", keyInfo.Mode())
				}
			} else if _, err := os.Lstat(fixture.projectSessionKeyPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("project session key stat error = %v, want os.ErrNotExist", err)
			}
			if !test.useRecord {
				if _, err := os.Lstat(fixture.recordKeyPath); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("record key stat error = %v, want os.ErrNotExist", err)
				}
			}
			if _, err := fixture.projectRoot.Stat("."); err != nil {
				t.Errorf("project root unavailable after call: %v", err)
			}
			if _, err := fixture.recordRoot.Stat("."); err != nil {
				t.Errorf("record root unavailable after call: %v", err)
			}
		})
	}
}

type unsafeContinuationKeyFixture struct {
	projectRoot           *os.Root
	recordRoot            *os.Root
	projectPath           string
	recordPath            string
	statePath             string
	recordKeyPath         string
	projectSessionKeyPath string
	sessionKeyPath        string
	targetPath            string
	targetLeafPath        string
	targetBytes           []byte
	targetMode            os.FileMode
	outsidePath           string
	outsideBytes          []byte
	outsideMode           os.FileMode
	unrelatedPath         string
	unrelatedBytes        []byte
	unrelatedMode         os.FileMode
}

func newUnsafeContinuationKeyFixture(t *testing.T) *unsafeContinuationKeyFixture {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "project")
	recordPath := filepath.Join(root, "record")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	if err := os.MkdirAll(recordPath, 0o755); err != nil {
		t.Fatalf("create record directory: %v", err)
	}
	statePath := filepath.Join(recordPath, "aidlc-state.md")
	if err := os.WriteFile(statePath, []byte("# state\n"), 0o644); err != nil {
		t.Fatalf("create state file: %v", err)
	}
	outsidePath := filepath.Join(root, "outside-target")
	outsideBytes := []byte("outside sentinel\n")
	if err := os.WriteFile(outsidePath, outsideBytes, 0o640); err != nil {
		t.Fatalf("create outside sentinel: %v", err)
	}
	outsideInfo, err := os.Stat(outsidePath)
	if err != nil {
		t.Fatalf("stat outside sentinel: %v", err)
	}
	unrelatedPath := filepath.Join(recordPath, "unrelated.txt")
	unrelatedBytes := []byte("unrelated sentinel\n")
	if err := os.WriteFile(unrelatedPath, unrelatedBytes, 0o640); err != nil {
		t.Fatalf("create unrelated sentinel: %v", err)
	}
	unrelatedInfo, err := os.Stat(unrelatedPath)
	if err != nil {
		t.Fatalf("stat unrelated sentinel: %v", err)
	}
	projectRoot, err := os.OpenRoot(projectPath)
	if err != nil {
		t.Fatalf("open project root: %v", err)
	}
	recordRoot, err := os.OpenRoot(recordPath)
	if err != nil {
		_ = projectRoot.Close()
		t.Fatalf("open record root: %v", err)
	}
	t.Cleanup(func() {
		_ = projectRoot.Close()
		_ = recordRoot.Close()
	})
	return &unsafeContinuationKeyFixture{
		projectRoot:           projectRoot,
		recordRoot:            recordRoot,
		projectPath:           projectPath,
		recordPath:            recordPath,
		statePath:             statePath,
		recordKeyPath:         filepath.Join(recordPath, ".aidlc-steering-token-key"),
		projectSessionKeyPath: filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key"),
		sessionKeyPath:        filepath.Join(projectPath, "aidlc", ".aidlc-sessions", ".aidlc-steering-token-key"),
		outsidePath:           outsidePath,
		outsideBytes:          outsideBytes,
		outsideMode:           outsideInfo.Mode().Perm(),
		unrelatedPath:         unrelatedPath,
		unrelatedBytes:        unrelatedBytes,
		unrelatedMode:         unrelatedInfo.Mode().Perm(),
	}
}

func (f *unsafeContinuationKeyFixture) replaceState(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.statePath); err != nil {
		t.Fatalf("remove regular state file: %v", err)
	}
}

func (f *unsafeContinuationKeyFixture) createInRootRecordKeySymlink(t *testing.T) {
	t.Helper()
	f.targetPath = filepath.Join(f.recordPath, "record-target-key")
	f.targetLeafPath = f.recordKeyPath
	f.targetBytes = []byte(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x73}, 32)) + "\n")
	if err := os.WriteFile(f.targetPath, f.targetBytes, 0o640); err != nil {
		t.Fatalf("create in-root record key target: %v", err)
	}
	targetInfo, err := os.Stat(f.targetPath)
	if err != nil {
		t.Fatalf("stat in-root record key target: %v", err)
	}
	f.targetMode = targetInfo.Mode().Perm()
	if err := os.Symlink(filepath.Base(f.targetPath), f.recordKeyPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
}

func (f *unsafeContinuationKeyFixture) createInRootSessionKeySymlink(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f.sessionKeyPath), 0o755); err != nil {
		t.Fatalf("create session directory: %v", err)
	}
	f.targetPath = filepath.Join(filepath.Dir(f.sessionKeyPath), "session-target-key")
	f.targetLeafPath = f.sessionKeyPath
	f.targetBytes = []byte(base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x74}, 32)) + "\n")
	if err := os.WriteFile(f.targetPath, f.targetBytes, 0o640); err != nil {
		t.Fatalf("create in-root session key target: %v", err)
	}
	targetInfo, err := os.Stat(f.targetPath)
	if err != nil {
		t.Fatalf("stat in-root session key target: %v", err)
	}
	f.targetMode = targetInfo.Mode().Perm()
	if err := os.Symlink(filepath.Base(f.targetPath), f.sessionKeyPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
}

func callReadOrCreateContinuationKey(projectRoot, recordRoot *os.Root) (key []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			key = nil
			err = fmt.Errorf("ReadOrCreateContinuationKey panic: %v", recovered)
		}
	}()
	return ReadOrCreateContinuationKey(projectRoot, recordRoot)
}
