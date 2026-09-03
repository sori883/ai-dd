package workspace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestUUIDV7TimestampVersionAndVariant(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 1, 12, 34, 56, 789_000_000, time.UTC)
	uuid, err := uuidV7(now, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}))
	if err != nil {
		t.Fatal(err)
	}
	compact := strings.ReplaceAll(uuid, "-", "")
	if len(compact) != 32 {
		t.Fatalf("uuidV7() = %q, want 32 hexadecimal digits", uuid)
	}
	if got, want := compact[:12], fmt.Sprintf("%012x", now.UnixMilli()); got != want {
		t.Errorf("timestamp = %q, want %q", got, want)
	}
	if compact[12] != '7' {
		t.Errorf("version nibble = %q, want 7", compact[12])
	}
	if !strings.ContainsRune("89ab", rune(compact[16])) {
		t.Errorf("variant nibble = %q, want RFC 4122 variant", compact[16])
	}
}

func TestUUIDV7EntropyFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("entropy unavailable")
	uuid, err := uuidV7(time.UnixMilli(0), errorReader{err: cause})
	if uuid != "" || !errors.Is(err, cause) {
		t.Errorf("uuidV7() = (%q, %v), want empty UUID and entropy cause", uuid, err)
	}
}

func TestUUIDV7TightLoopUniqueness(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	seen := make(map[string]struct{}, 1_000)
	for range 1_000 {
		uuid, err := uuidV7(now, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if _, duplicate := seen[uuid]; duplicate {
			t.Fatalf("uuidV7() repeated %q within one millisecond", uuid)
		}
		seen[uuid] = struct{}{}
	}
}

func TestIntentSlugUsesReferenceNormalizationAndLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "existing", raw: "build-auth", want: "build-auth"},
		{name: "dotted capital", raw: "AİB", want: "ai-b"},
		{name: "empty", raw: "!? 東京", want: "intent"},
		{name: "numeric start", raw: "2026 Platform", want: "intent-2026-platform"},
		{name: "twenty five", raw: strings.Repeat("a", 25), want: strings.Repeat("a", 24)},
		{
			name: "trim separator after truncation",
			raw:  strings.Repeat("a", 23) + "-b",
			want: strings.Repeat("a", 23),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := intentSlug(tt.raw); got != tt.want {
				t.Errorf("intentSlug(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeIntentLabelRejectsReservedNames(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"help", "LIST", " switch! ", "create", "archive", "rename", "show", "birth",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			slug, err := normalizeIntentLabel(raw)
			if slug != "" || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf(
					"normalizeIntentLabel(%q) = (%q, %v), want empty slug and fs.ErrInvalid",
					raw,
					slug,
					err,
				)
			}
		})
	}
}

func TestIntentDirBaseUsesUTCDate(t *testing.T) {
	t.Parallel()

	plusFourteen := time.FixedZone("UTC+14", 14*60*60)
	now := time.Date(2026, time.September, 1, 0, 30, 0, 0, plusFourteen)
	base, err := intentDirBase("Build Auth", now)
	if err != nil {
		t.Fatal(err)
	}
	if base != "260831-build-auth" {
		t.Errorf("intentDirBase() = %q, want UTC date prefix %q", base, "260831-build-auth")
	}
}

func TestResolveIntentDirNameUsesBoundedNumericSuffixes(t *testing.T) {
	t.Parallel()

	t.Run("first free suffix", func(t *testing.T) {
		t.Parallel()

		occupied := map[string]bool{"260901-build-auth": true, "260901-build-auth-2": true}
		got, err := resolveIntentDirName(
			"260901-build-auth",
			func(name string) (fs.FileInfo, error) {
				if occupied[name] {
					return nil, nil
				}
				return nil, fs.ErrNotExist
			},
		)
		if err != nil || got != "260901-build-auth-3" {
			t.Errorf("resolveIntentDirName() = (%q, %v), want suffix -3", got, err)
		}
	})

	t.Run("stat error", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("injected stat failure")
		got, err := resolveIntentDirName(
			"260901-build-auth",
			func(name string) (fs.FileInfo, error) {
				if name == "260901-build-auth" {
					return nil, nil
				}
				return nil, cause
			},
		)
		if got != "" || !errors.Is(err, cause) {
			t.Errorf("resolveIntentDirName() = (%q, %v), want stat cause", got, err)
		}
	})

	t.Run("suffix exhaustion", func(t *testing.T) {
		t.Parallel()

		calls := 0
		got, err := resolveIntentDirName(
			"260901-build-auth",
			func(name string) (fs.FileInfo, error) {
				calls++
				want := "260901-build-auth"
				if calls > 1 {
					want += "-" + strconv.Itoa(calls)
				}
				if name != want {
					t.Errorf("candidate %d = %q, want %q", calls, name, want)
				}
				return nil, nil
			},
		)
		if got != "" || !errors.Is(err, fs.ErrExist) {
			t.Errorf("resolveIntentDirName() = (%q, %v), want bounded fs.ErrExist", got, err)
		}
		if calls != 999 {
			t.Errorf("candidate checks = %d, want base through -999", calls)
		}
	})
}

func TestCreateIntentRecordCreatesExactStubExclusively(t *testing.T) {
	t.Parallel()

	steps := []string{}
	ops := intentRecordOps{
		mkdir: func(name string, mode fs.FileMode) error {
			steps = append(steps, fmt.Sprintf("mkdir %s %04o", name, mode))
			return nil
		},
		openFile: func(name string, flags int, mode fs.FileMode) (*os.File, error) {
			steps = append(steps, fmt.Sprintf("open %s %d %04o", name, flags, mode))
			return nil, nil
		},
		write: func(_ *os.File, content string) (int, error) {
			steps = append(steps, "write "+content)
			return len(content), nil
		},
		close: func(*os.File) error {
			steps = append(steps, "close")
			return nil
		},
	}
	if err := createIntentRecord("260901-build-auth", ops); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"mkdir 260901-build-auth 0777",
		fmt.Sprintf(
			"open %s %d 0666",
			filepath.Join("260901-build-auth", "aidlc-state.md"),
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		),
		"write " + intentStateStub,
		"close",
	}
	if !slices.Equal(steps, want) {
		t.Errorf("steps = %q, want %q", steps, want)
	}
}

func TestCreateIntentRunsLockedTransactionAndReturnsCommittedMetadata(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	now := time.Date(2026, time.September, 1, 1, 2, 3, 4, time.UTC)
	scope := "repository"
	steps := []string{}
	projectRoot := new(os.Root)
	intentsRoot := new(os.Root)
	result, err := createIntent(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{
			SpaceName: "team", Label: "Build Auth", Scope: &scope, Repos: []string{"api", "web"},
		},
		intentCreateOps{
			acquireLock: func(pathContext context.Context, path string) (workspaceLockReceipt, error) {
				steps = append(steps, "acquire "+path)
				if pathContext.Err() != nil {
					t.Errorf("lock context unexpectedly canceled: %v", pathContext.Err())
				}
				return workspaceLockReceipt{path: "lock", token: "token"}, nil
			},
			releaseLock: func(workspaceLockReceipt) error {
				steps = append(steps, "release")
				return nil
			},
			openProject: func(path string) (*os.Root, error) {
				steps = append(steps, "open project "+path)
				return projectRoot, nil
			},
			openChild: func(_ *os.Root, path string) (*os.Root, error) {
				steps = append(steps, "open child "+path)
				return intentsRoot, nil
			},
			closeRoot: func(root *os.Root) error {
				if root == intentsRoot {
					steps = append(steps, "close child")
				} else {
					steps = append(steps, "close project")
				}
				return nil
			},
			readRegistry: func(*os.Root) ([]json.RawMessage, error) {
				steps = append(steps, "read registry")
				return []json.RawMessage{}, nil
			},
			now: func() time.Time { return now },
			uuid: func(got time.Time) (string, error) {
				steps = append(steps, "uuid")
				if !got.Equal(now) {
					t.Errorf("uuid time = %v, want %v", got, now)
				}
				return "0199-aaaa", nil
			},
			resolveDir: func(_ *os.Root, base string) (string, error) {
				steps = append(steps, "resolve "+base)
				return "260901-build-auth", nil
			},
			createRecord: func(_ *os.Root, dirName string) error {
				steps = append(steps, "create "+dirName)
				return nil
			},
			writeRegistry: func(
				_ *os.Root,
				rows []json.RawMessage,
				entry intentRegistryEntry,
			) error {
				steps = append(steps, "write registry")
				if len(rows) != 0 {
					t.Errorf("existing rows = %d, want zero", len(rows))
				}
				want := intentRegistryEntry{
					UUID: "0199-aaaa", Slug: "build-auth", DirName: "260901-build-auth",
					Scope: &scope, Repos: []string{"api", "web"}, Status: "in-flight",
				}
				if !reflect.DeepEqual(entry, want) {
					t.Errorf("registry entry = %+v, want %+v", entry, want)
				}
				return nil
			},
			activeSpace: func(root *os.Root) string {
				steps = append(steps, "read active-space")
				if root != projectRoot {
					t.Error("active-space read used a root other than the project root")
				}
				return "default"
			},
			completeActiveSpace: func(_ *os.Root, name string) error {
				steps = append(steps, "complete active-space")
				if name != "default" {
					t.Errorf("active-space completion = %q, want shared fallback default", name)
				}
				return nil
			},
			saveActiveIntent: func(*os.Root, string) error {
				steps = append(steps, "save active-intent")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantResult := CreatedIntent{
		UUID: "0199-aaaa", Slug: "build-auth", DirName: "260901-build-auth",
		RecordDir: filepath.Join(project, "aidlc", "spaces", "team", "intents", "260901-build-auth"),
		SpaceName: "team",
	}
	if result != wantResult {
		t.Errorf("createIntent() result = %+v, want %+v", result, wantResult)
	}
	wantSteps := []string{
		"acquire " + project,
		"open project " + project,
		"open child " + filepath.Join("aidlc", "spaces", "team", "intents"),
		"read registry",
		"uuid",
		"resolve 260901-build-auth",
		"create 260901-build-auth",
		"write registry",
		"read active-space",
		"complete active-space",
		"save active-intent",
		"close child",
		"close project",
		"release",
	}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want locked transaction order %q", steps, wantSteps)
	}
}

func TestCreateIntentCallbackRunsBeforeWorkspaceLockRelease(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	lockHeld := false
	steps := []string{}
	ops := successfulIntentCreateOps()
	ops.acquireLock = func(context.Context, string) (workspaceLockReceipt, error) {
		steps = append(steps, "acquire")
		lockHeld = true
		return workspaceLockReceipt{path: "lock", token: "token"}, nil
	}
	ops.releaseLock = func(workspaceLockReceipt) error {
		steps = append(steps, "release")
		lockHeld = false
		return nil
	}
	created, err := createIntentWithCallback(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
		ops,
		func(CreatedIntent) error {
			steps = append(steps, "additional work")
			if !lockHeld {
				t.Error("additional work ran after the workspace lock was released")
			}
			return nil
		},
	)
	if err != nil || created == (CreatedIntent{}) {
		t.Errorf("createIntentWithCallback() = (%+v, %v), want committed result", created, err)
	}
	wantSteps := []string{"acquire", "additional work", "release"}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want callback inside outer lock %q", steps, wantSteps)
	}
}

func TestCreateIntentWithInitializer(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	projectRoot := new(os.Root)
	intentsRoot := new(os.Root)
	recordRoot := new(os.Root)
	lockHeld := false
	steps := []string{}
	ops := successfulIntentCreateOps()
	ops.acquireLock = func(context.Context, string) (workspaceLockReceipt, error) {
		steps = append(steps, "acquire")
		lockHeld = true
		return workspaceLockReceipt{path: "lock", token: "token"}, nil
	}
	ops.releaseLock = func(workspaceLockReceipt) error {
		steps = append(steps, "release")
		lockHeld = false
		return nil
	}
	ops.openProject = func(path string) (*os.Root, error) {
		steps = append(steps, "open project "+path)
		return projectRoot, nil
	}
	ops.openChild = func(parent *os.Root, path string) (*os.Root, error) {
		switch {
		case parent == projectRoot:
			steps = append(steps, "open intents "+path)
			return intentsRoot, nil
		case parent == intentsRoot:
			steps = append(steps, "open record "+path)
			return recordRoot, nil
		default:
			t.Fatalf("openChild() received unexpected parent %p and path %q", parent, path)
			return nil, nil
		}
	}
	ops.readRegistry = func(*os.Root) ([]json.RawMessage, error) {
		steps = append(steps, "read registry")
		return []json.RawMessage{}, nil
	}
	ops.uuid = func(time.Time) (string, error) {
		steps = append(steps, "uuid")
		return "0199-aaaa", nil
	}
	ops.resolveDir = func(*os.Root, string) (string, error) {
		steps = append(steps, "resolve 260901-build-auth")
		return "260901-build-auth", nil
	}
	ops.createRecord = func(*os.Root, string) error {
		steps = append(steps, "create 260901-build-auth")
		return nil
	}
	ops.writeRegistry = func(*os.Root, []json.RawMessage, intentRegistryEntry) error {
		steps = append(steps, "write registry")
		return nil
	}
	ops.activeSpace = func(*os.Root) string {
		steps = append(steps, "read active-space")
		return "default"
	}
	ops.completeActiveSpace = func(*os.Root, string) error {
		steps = append(steps, "complete active-space")
		return nil
	}
	ops.saveActiveIntent = func(*os.Root, string) error {
		steps = append(steps, "save active-intent")
		return nil
	}
	ops.closeRoot = func(root *os.Root) error {
		switch root {
		case recordRoot:
			steps = append(steps, "close record")
		case intentsRoot:
			steps = append(steps, "close intents")
		case projectRoot:
			steps = append(steps, "close project")
		default:
			t.Fatalf("closeRoot() received unexpected root %p", root)
		}
		return nil
	}
	created, err := createIntentWithInitializer(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
		ops,
		func(gotProject, gotRecord *os.Root, got CreatedIntent) error {
			steps = append(steps, "initialize")
			if !lockHeld {
				t.Error("initializer ran after the workspace lock was released")
			}
			if gotProject != projectRoot || gotRecord != recordRoot {
				t.Errorf("initializer roots = (%p, %p), want (%p, %p)", gotProject, gotRecord, projectRoot, recordRoot)
			}
			if got.DirName != "260901-build-auth" {
				t.Errorf("initializer CreatedIntent = %+v, want committed identity", got)
			}
			return nil
		},
	)
	if err != nil || created == (CreatedIntent{}) {
		t.Errorf("createIntentWithInitializer() = (%+v, %v), want committed result", created, err)
	}
	wantSteps := []string{
		"acquire",
		"open project " + project,
		"open intents " + filepath.Join("aidlc", "spaces", "team", "intents"),
		"read registry",
		"uuid",
		"resolve 260901-build-auth",
		"create 260901-build-auth",
		"write registry",
		"read active-space",
		"complete active-space",
		"save active-intent",
		"open record 260901-build-auth",
		"initialize",
		"close record",
		"close intents",
		"close project",
		"release",
	}
	if !slices.Equal(steps, wantSteps) {
		t.Errorf("steps = %q, want initializer order %q", steps, wantSteps)
	}
}

func TestCreateIntentWithInitializerRunsAfterCursorFailure(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	cursorCause := errors.New("active cursor failed")
	initializerCause := errors.New("initializer failed")
	closeCause := errors.New("root close failed")
	releaseCause := errors.New("lock release failed")
	initialized := false
	ops := successfulIntentCreateOps()
	ops.completeActiveSpace = func(*os.Root, string) error {
		return cursorCause
	}
	ops.saveActiveIntent = func(*os.Root, string) error {
		t.Fatal("saveActiveIntent ran after the first cursor operation failed")
		return nil
	}
	ops.closeRoot = func(*os.Root) error { return closeCause }
	ops.releaseLock = func(workspaceLockReceipt) error { return releaseCause }
	created, err := createIntentWithInitializer(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
		ops,
		func(projectRoot, recordRoot *os.Root, got CreatedIntent) error {
			initialized = true
			if projectRoot == nil || recordRoot == nil {
				t.Error("initializer received a nil root")
			}
			if got == (CreatedIntent{}) {
				t.Error("initializer received a zero CreatedIntent")
			}
			return initializerCause
		},
	)
	if created == (CreatedIntent{}) || !initialized {
		t.Errorf("createIntentWithInitializer() = (%+v, %v), want committed result and initializer call", created, err)
	}
	for _, cause := range []error{cursorCause, initializerCause, closeCause, releaseCause} {
		if !errors.Is(err, cause) {
			t.Errorf("error %v lost cause %v", err, cause)
		}
	}
}

func TestCreateIntentWithInitializerDoesNotRunBeforeRegistryCommit(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeCause := errors.New("registry commit failed")
	initialized := false
	ops := successfulIntentCreateOps()
	ops.writeRegistry = func(*os.Root, []json.RawMessage, intentRegistryEntry) error {
		return writeCause
	}
	created, err := createIntentWithInitializer(
		context.Background(),
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
		ops,
		func(*os.Root, *os.Root, CreatedIntent) error {
			initialized = true
			return nil
		},
	)
	if created != (CreatedIntent{}) {
		t.Errorf("pre-commit result = %+v, want zero", created)
	}
	if initialized {
		t.Error("initializer ran before registry commit")
	}
	if !errors.Is(err, writeCause) {
		t.Errorf("createIntentWithInitializer() error = %v, want registry cause", err)
	}
}

func TestCreateIntentLockedDoesNotManageWorkspaceLock(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	prepared, err := prepareIntentCreate(
		RootInput{ExplicitDir: project},
		IntentCreateInput{SpaceName: "team", Label: "Build Auth"},
	)
	if err != nil {
		t.Fatal(err)
	}
	ops := successfulIntentCreateOps()
	ops.acquireLock = func(context.Context, string) (workspaceLockReceipt, error) {
		t.Fatal("caller-held createIntentLocked reacquired the workspace lock")
		return workspaceLockReceipt{}, nil
	}
	ops.releaseLock = func(workspaceLockReceipt) error {
		t.Fatal("caller-held createIntentLocked released the workspace lock")
		return nil
	}
	created, committed, err := createIntentLocked(prepared, ops.lockedOperations())
	if err != nil || !committed || created == (CreatedIntent{}) {
		t.Errorf(
			"createIntentLocked() = (%+v, %t, %v), want committed result without lock management",
			created,
			committed,
			err,
		)
	}
}

func TestCreateIntentCommitBoundaryControlsReturnedResult(t *testing.T) {
	t.Parallel()

	input := IntentCreateInput{SpaceName: "team", Label: "Build Auth"}
	t.Run("pre-commit registry and release errors return zero", func(t *testing.T) {
		t.Parallel()

		project := t.TempDir()
		writeCause := errors.New("registry write failed")
		releaseCause := errors.New("release failed")
		ops := successfulIntentCreateOps()
		ops.writeRegistry = func(*os.Root, []json.RawMessage, intentRegistryEntry) error {
			return writeCause
		}
		ops.releaseLock = func(workspaceLockReceipt) error { return releaseCause }
		created, err := createIntent(
			context.Background(),
			RootInput{ExplicitDir: project},
			input,
			ops,
		)
		if created != (CreatedIntent{}) {
			t.Errorf("pre-commit result = %+v, want zero", created)
		}
		if !errors.Is(err, writeCause) || !errors.Is(err, releaseCause) {
			t.Errorf("pre-commit error %v lost registry/release causes", err)
		}
	})

	t.Run("post-commit cursor close and release errors retain result", func(t *testing.T) {
		t.Parallel()

		project := t.TempDir()
		cursorCause := errors.New("cursor failed")
		closeCause := errors.New("close failed")
		releaseCause := errors.New("release failed")
		ops := successfulIntentCreateOps()
		ops.completeActiveSpace = func(*os.Root, string) error { return cursorCause }
		ops.closeRoot = func(*os.Root) error { return closeCause }
		ops.releaseLock = func(workspaceLockReceipt) error { return releaseCause }
		created, err := createIntent(
			context.Background(),
			RootInput{ExplicitDir: project},
			input,
			ops,
		)
		if created == (CreatedIntent{}) || created.DirName != "260901-build-auth" {
			t.Errorf("post-commit result = %+v, want committed identity", created)
		}
		for _, cause := range []error{cursorCause, closeCause, releaseCause} {
			if !errors.Is(err, cause) {
				t.Errorf("post-commit error %v lost cause %v", err, cause)
			}
		}
	})
}

func successfulIntentCreateOps() intentCreateOps {
	return intentCreateOps{
		acquireLock: func(context.Context, string) (workspaceLockReceipt, error) {
			return workspaceLockReceipt{path: "lock", token: "token"}, nil
		},
		releaseLock: func(workspaceLockReceipt) error { return nil },
		openProject: func(string) (*os.Root, error) { return new(os.Root), nil },
		openChild:   func(*os.Root, string) (*os.Root, error) { return new(os.Root), nil },
		closeRoot:   func(*os.Root) error { return nil },
		readRegistry: func(*os.Root) ([]json.RawMessage, error) {
			return []json.RawMessage{}, nil
		},
		now: func() time.Time {
			return time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
		},
		uuid:         func(time.Time) (string, error) { return "0199-aaaa", nil },
		resolveDir:   func(*os.Root, string) (string, error) { return "260901-build-auth", nil },
		createRecord: func(*os.Root, string) error { return nil },
		writeRegistry: func(*os.Root, []json.RawMessage, intentRegistryEntry) error {
			return nil
		},
		activeSpace:         func(*os.Root) string { return "default" },
		completeActiveSpace: func(*os.Root, string) error { return nil },
		saveActiveIntent:    func(*os.Root, string) error { return nil },
	}
}

func TestCreateIntentRecordPreservesFailureCausesAndPartialDirectory(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"mkdir", "open", "write", "short write", "close", "write and close"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()

			cause := errors.New("injected " + stage + " failure")
			closeCause := errors.New("injected close failure")
			created := false
			closed := false
			expected := cause
			if stage == "close" {
				expected = closeCause
			}
			ops := intentRecordOps{
				mkdir: func(string, fs.FileMode) error {
					if stage == "mkdir" {
						return cause
					}
					created = true
					return nil
				},
				openFile: func(string, int, fs.FileMode) (*os.File, error) {
					if stage == "open" {
						return nil, cause
					}
					return nil, nil
				},
				write: func(_ *os.File, content string) (int, error) {
					switch stage {
					case "write", "write and close":
						return 0, cause
					case "short write":
						expected = io.ErrShortWrite
						return len(content) - 1, nil
					default:
						return len(content), nil
					}
				},
				close: func(*os.File) error {
					closed = true
					if stage == "close" || stage == "write and close" {
						return closeCause
					}
					return nil
				},
			}
			err := createIntentRecord("260901-build-auth", ops)
			if !errors.Is(err, expected) {
				t.Errorf("error %v lost cause %v", err, expected)
			}
			if stage == "close" || stage == "write and close" {
				if !errors.Is(err, closeCause) {
					t.Errorf("error %v lost close cause %v", err, closeCause)
				}
			}
			if created != (stage != "mkdir") {
				t.Errorf("directory created = %t, want %t", created, stage != "mkdir")
			}
			wantClosed := stage != "mkdir" && stage != "open"
			if closed != wantClosed {
				t.Errorf("file closed = %t, want %t", closed, wantClosed)
			}
		})
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

var _ io.Reader = errorReader{}
