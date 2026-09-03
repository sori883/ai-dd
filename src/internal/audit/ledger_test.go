package audit

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestAppendCreatesCanonicalShardAndEscapesLineTerminators(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	const cloneID = "abcdef123456"
	intentPath := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(intentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte(cloneID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	timestamp := time.Date(2026, time.September, 3, 4, 5, 6, 0, time.UTC)
	events := []Event{{
		Event:     "STAGE_STARTED",
		Timestamp: time.Date(2099, time.January, 2, 3, 4, 5, 999_999_999, time.UTC),
		Fields: map[string]string{
			"Message": "first\r\nsecond\nthird\u2028fourth\u2029",
			"Stage":   "build",
		},
	}}
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.now = func() time.Time { return timestamp.Add(987_654_321 * time.Nanosecond) }
	if err := appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, events, &ops); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	shardPath := filepath.Join("audit", shardNameForHost(cloneID))
	data, err := recordRoot.ReadFile(shardPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "# AI-DLC Audit Log\n\n" +
		"## Stage Start\n" +
		"**Timestamp**: 2026-09-03T04:05:06Z\n" +
		"**Event**: STAGE_STARTED\n" +
		"**Message**: first\\nsecond\\nthird\\nfourth\\n\n" +
		"**Stage**: build\n\n" +
		"---\n"
	if string(data) != want {
		t.Errorf("audit bytes = %q, want %q", data, want)
	}
	if _, err := projectRoot.Stat("aidlc"); err != nil {
		t.Errorf("project Root unusable after Append(): %v", err)
	}
	if _, err := recordRoot.Stat("audit"); err != nil {
		t.Errorf("record Root unusable after Append(): %v", err)
	}
}

func TestAppendPrevalidatesBatchBeforeFilesystemMutation(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	err = Append(context.Background(), guard, projectRoot, recordRoot, []Event{
		{Event: "STAGE_STARTED"},
		{Event: "NOT_ALLOWED"},
	})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("Append() error = %v, want ErrInvalidEvent", err)
	}
	if _, statErr := recordRoot.Lstat("audit"); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("invalid batch touched audit directory: %v", statErr)
	}
}

func TestAppendRequiresHeldMatchingGuard(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Release(); err != nil {
		t.Fatal(err)
	}
	if err := Append(context.Background(), guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}); !errors.Is(err, ErrGuardNotHeld) {
		t.Errorf("released guard error = %v, want ErrGuardNotHeld", err)
	}
	otherIdentity, err := recordlock.NewIdentity(projectDir, "default", "other")
	if err != nil {
		t.Fatal(err)
	}
	otherGuard, err := recordlock.Acquire(context.Background(), otherIdentity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = otherGuard.Release() })
	if err := AppendForIdentity(context.Background(), identity, otherGuard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}); !errors.Is(err, ErrGuardIdentity) {
		t.Errorf("different identity error = %v, want ErrGuardIdentity", err)
	}
}

func TestShardNameNormalizesHostAndCloneID(t *testing.T) {
	t.Parallel()

	if got := normalizeHost("Build.Machine_01"); got != "build-machine-01" {
		t.Errorf("normalizeHost() = %q", got)
	}
	if got := shardNameForHost("abcdef123456"); !strings.HasSuffix(got, "-abcdef123456.md") {
		t.Errorf("shardNameForHost() = %q, want clone suffix", got)
	}
}

func TestValidateEventAllowlistAndFieldBoundary(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"HUMAN_TURN", "STAGE_AWAITING_APPROVAL", "GATE_APPROVED", "GATE_REJECTED",
		"STAGE_REVISING", "STAGE_COMPLETED", "PHASE_COMPLETED", "PHASE_VERIFIED",
		"PHASE_STARTED", "STAGE_STARTED", "WORKFLOW_COMPLETED",
	}
	for _, eventType := range allowed {
		if got, err := validateEvent(Event{Event: eventType}); err != nil || got != eventType {
			t.Errorf("validateEvent(%q) = (%q, %v), want accepted", eventType, got, err)
		}
	}
	if _, err := validateEvent(Event{Event: "WORKFLOW_STARTED"}); !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("unknown event error = %v, want ErrInvalidEvent", err)
	}
	for _, key := range []string{"Timestamp", "Event", "bad:**", "1st"} {
		if _, err := validateEvent(Event{Event: "STAGE_STARTED", Fields: map[string]string{key: "value"}}); !errors.Is(err, ErrInvalidField) {
			t.Errorf("field %q error = %v, want ErrInvalidField", key, err)
		}
	}
}

func TestFormatTimestampUsesUTCAndFixedSeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{
			name: "zero milliseconds",
			when: time.Date(2026, 9, 3, 4, 5, 6, 0, time.FixedZone("JST", 9*60*60)),
			want: "2026-09-02T19:05:06Z",
		},
		{
			name: "truncate nanoseconds",
			when: time.Date(2026, 9, 3, 4, 5, 6, 789_999_999, time.FixedZone("JST", 9*60*60)),
			want: "2026-09-02T19:05:06Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTimestamp(tt.when); got != tt.want {
				t.Errorf("formatTimestamp() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendGeneratesAndConvergesOnTwelveHexCloneID(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.random = bytes.NewReader([]byte{0xab, 0xcd, 0xef, 0x12, 0x34, 0x56})
	ops.hostname = func() (string, error) { return "Host.Name", nil }
	if err := appendForIdentityWithOps(
		context.Background(), identity, guard, projectRoot, recordRoot,
		[]Event{{Event: "STAGE_STARTED", Timestamp: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)}}, &ops,
	); err != nil {
		t.Fatalf("appendForIdentityWithOps() error = %v", err)
	}
	cloneBytes, err := os.ReadFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(cloneBytes) != "abcdef123456\n" {
		t.Errorf("clone id bytes = %q, want %q", cloneBytes, "abcdef123456\\n")
	}
	if _, err := recordRoot.Stat(filepath.Join(auditDirectory, "host-name-abcdef123456.md")); err != nil {
		t.Errorf("generated shard missing: %v", err)
	}

	// A subsequent append must read the persisted token rather than minting a
	// second shard, even when its entropy source differs.
	ops.random = bytes.NewReader([]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc})
	if err := appendForIdentityWithOps(
		context.Background(), identity, guard, projectRoot, recordRoot,
		[]Event{{Event: "STAGE_COMPLETED", Timestamp: time.Date(2026, 9, 3, 0, 0, 1, 0, time.UTC)}}, &ops,
	); err != nil {
		t.Fatalf("second append error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(recordDir, auditDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "host-name-abcdef123456.md" {
		t.Errorf("audit shards = %+v, want one persisted clone shard", entries)
	}
}

func TestParseCloneIDRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "abcdef12345", "abcdef1234567", "ABCDEF123456", "abcdef12345g", "abcdef123456\n\n", "abcdef123456\r\n"} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if _, err := parseCloneID([]byte(raw)); !errors.Is(err, ErrInvalidCloneID) {
				t.Errorf("parseCloneID(%q) error = %v, want ErrInvalidCloneID", raw, err)
			}
		})
	}
}

func TestAppendRejectsSymlinkAndNonRegularAuditTargets(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(projectDir, recordDir string) error
	}{
		{
			name: "clone id symlink",
			setup: func(projectDir, recordDir string) error {
				if err := os.MkdirAll(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(projectDir, "target"), []byte("abcdef123456\n"), 0o600); err != nil {
					return err
				}
				return os.Symlink("../target", filepath.Join(projectDir, cloneIDDirectory, cloneIDFile))
			},
		},
		{
			name: "audit directory file",
			setup: func(projectDir, recordDir string) error {
				if err := os.MkdirAll(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(recordDir, auditDirectory), nil, 0o600)
			},
		},
		{
			name: "audit directory symlink",
			setup: func(projectDir, recordDir string) error {
				if err := os.MkdirAll(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
					return err
				}
				outside := filepath.Join(t.TempDir(), "outside-audit")
				if err := os.Mkdir(outside, 0o700); err != nil {
					return err
				}
				return os.Symlink(outside, filepath.Join(recordDir, auditDirectory))
			},
		},
		{
			name: "shard symlink",
			setup: func(projectDir, recordDir string) error {
				if err := os.MkdirAll(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
					return err
				}
				if err := os.Mkdir(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
					return err
				}
				outside := filepath.Join(t.TempDir(), "outside-shard")
				if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
					return err
				}
				return os.Symlink(outside, filepath.Join(recordDir, auditDirectory, shardNameForHost("abcdef123456")))
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projectDir := t.TempDir()
			recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
			if err := os.MkdirAll(recordDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := tc.setup(projectDir, recordDir); err != nil {
				if strings.Contains(tc.name, "symlink") && errors.Is(err, fs.ErrPermission) {
					t.Skipf("symlink unavailable: %v", err)
				}
				t.Fatal(err)
			}
			projectRoot, err := os.OpenRoot(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = projectRoot.Close() })
			recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = recordRoot.Close() })
			identity, err := recordlock.NewIdentity(projectDir, "default", "build")
			if err != nil {
				t.Fatal(err)
			}
			guard, err := recordlock.Acquire(context.Background(), identity)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = guard.Release() })
			if err := Append(context.Background(), guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}); !errors.Is(err, ErrInvalidRoot) {
				t.Errorf("Append() error = %v, want ErrInvalidRoot", err)
			}
		})
	}
}

func TestAppendReportsOpenWriteShortAndCloseFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*ledgerOps, error)
		want      error
	}{
		{
			name: "open",
			configure: func(ops *ledgerOps, cause error) {
				ops.recordOpen = func(string, int, fs.FileMode) (*os.File, error) { return nil, cause }
			},
			want: nil,
		},
		{
			name: "no progress",
			configure: func(ops *ledgerOps, cause error) {
				ops.write = func(*os.File, []byte) (int, error) { return 0, nil }
			},
			want: ErrNoWriteProgress,
		},
		{
			name: "write",
			configure: func(ops *ledgerOps, cause error) {
				ops.write = func(*os.File, []byte) (int, error) { return 0, cause }
			},
			want: nil,
		},
		{
			name: "close",
			configure: func(ops *ledgerOps, cause error) {
				actualClose := ops.close
				ops.close = func(file *os.File) error {
					_ = actualClose(file)
					return cause
				}
			},
			want: nil,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projectDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
			if err := os.MkdirAll(recordDir, 0o700); err != nil {
				t.Fatal(err)
			}
			projectRoot, err := os.OpenRoot(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = projectRoot.Close() })
			recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = recordRoot.Close() })
			identity, err := recordlock.NewIdentity(projectDir, "default", "build")
			if err != nil {
				t.Fatal(err)
			}
			guard, err := recordlock.Acquire(context.Background(), identity)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = guard.Release() })
			cause := errors.New(tc.name + " failure")
			ops := systemLedgerOps(projectRoot, recordRoot)
			tc.configure(&ops, cause)
			err = appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED", Timestamp: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)}}, &ops)
			if tc.want == nil {
				if !errors.Is(err, cause) {
					t.Errorf("append error = %v, want injected cause", err)
				}
			} else if !errors.Is(err, tc.want) {
				t.Errorf("append error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAppendRejectsRecordRootsOutsideExpectedIntent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	intentDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(intentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	siblingDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "other")
	if err := os.MkdirAll(siblingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })

	externalDir := t.TempDir()
	externalRoot, err := os.OpenRoot(externalDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = externalRoot.Close() })
	cases := []struct {
		name string
		root *os.Root
	}{
		{
			name: "sibling intent",
			root: mustOpenRoot(t, projectRoot, filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "other")),
		},
		{name: "external root", root: externalRoot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Append(context.Background(), guard, projectRoot, tc.root, []Event{{Event: "STAGE_STARTED"}}); !errors.Is(err, ErrInvalidRoot) {
				t.Errorf("Append() error = %v, want ErrInvalidRoot", err)
			}
			if _, statErr := tc.root.Lstat(auditDirectory); !errors.Is(statErr, fs.ErrNotExist) {
				t.Errorf("rejected record root was mutated: %v", statErr)
			}
		})
	}
}

func TestRenderEventsUsesPrivateClockAndSecondPrecision(t *testing.T) {
	t.Parallel()

	clock := time.Date(2026, 9, 3, 4, 5, 6, 987_654_321, time.FixedZone("JST", 9*60*60))
	callerTimestamp := time.Date(2099, 1, 2, 3, 4, 5, 999_999_999, time.UTC)
	rendered, err := renderEvents([]Event{{
		Event:     "STAGE_STARTED",
		Timestamp: callerTimestamp,
	}}, func() time.Time { return clock })
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(rendered[0].block), "\n## Stage Start\n**Timestamp**: 2026-09-02T19:05:06Z\n**Event**: STAGE_STARTED\n\n---\n"; got != want {
		t.Errorf("rendered event = %q, want %q", got, want)
	}
}

func TestAppendRejectsShardAndParentReplacementAroundWrite(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		replace func(t *testing.T, recordDir, shardPath string) error
	}{
		{
			name: "shard",
			replace: func(t *testing.T, recordDir, shardPath string) error {
				if err := os.Rename(shardPath, shardPath+".opened"); err != nil {
					return err
				}
				return os.WriteFile(shardPath, []byte("replacement\n"), 0o600)
			},
		},
		{
			name: "parent",
			replace: func(t *testing.T, recordDir, shardPath string) error {
				parent := filepath.Dir(shardPath)
				if err := os.Rename(parent, parent+".opened"); err != nil {
					return err
				}
				return os.Mkdir(parent, 0o700)
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projectDir := t.TempDir()
			recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
			if err := os.MkdirAll(recordDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			projectRoot, err := os.OpenRoot(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = projectRoot.Close() })
			recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = recordRoot.Close() })
			identity, err := recordlock.NewIdentity(projectDir, "default", "build")
			if err != nil {
				t.Fatal(err)
			}
			guard, err := recordlock.Acquire(context.Background(), identity)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = guard.Release() })
			shardPath := filepath.Join(recordDir, auditDirectory, shardName("host", "abcdef123456"))
			ops := systemLedgerOps(projectRoot, recordRoot)
			ops.hostname = func() (string, error) { return "host", nil }
			ops.now = func() time.Time { return time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) }
			actualWrite := ops.write
			replaced := false
			ops.write = func(file *os.File, data []byte) (int, error) {
				if !replaced {
					replaced = true
					if err := tc.replace(t, recordDir, shardPath); err != nil {
						return 0, err
					}
				}
				return actualWrite(file, data)
			}
			err = appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}, &ops)
			if !errors.Is(err, ErrInvalidRoot) {
				t.Errorf("append error = %v, want ErrInvalidRoot", err)
			}
			if tc.name == "shard" {
				data, readErr := os.ReadFile(shardPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(data) != "replacement\n" {
					t.Errorf("replacement shard = %q, want attacker bytes preserved", data)
				}
			}
		})
	}
}

func TestAppendLeaseSerializesCopiesAndReleaseWaitsForFilesystemWork(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.hostname = func() (string, error) { return "host", nil }
	ops.now = func() time.Time { return time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) }
	actualWrite := ops.write
	writeStarted := make(chan struct{})
	allowWrite := make(chan struct{})
	ops.write = func(file *os.File, data []byte) (int, error) {
		select {
		case <-writeStarted:
		default:
			close(writeStarted)
		}
		<-allowWrite
		return actualWrite(file, data)
	}
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}, &ops)
	}()
	select {
	case <-writeStarted:
	case <-time.After(time.Second):
		t.Fatal("append did not reach the filesystem write")
	}
	blockedCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if secondErr := appendForIdentityWithOps(blockedCtx, identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_COMPLETED"}}, &ops); !errors.Is(secondErr, context.DeadlineExceeded) {
		t.Errorf("concurrent append error = %v, want context deadline", secondErr)
	}
	releaseDone := make(chan error, 1)
	go func() { releaseDone <- guard.Release() }()
	select {
	case err := <-releaseDone:
		t.Fatalf("Release() completed during append: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(allowWrite)
	if err := <-firstDone; err != nil {
		t.Fatalf("first append error = %v", err)
	}
	select {
	case err := <-releaseDone:
		if err != nil {
			t.Fatalf("Release() after append = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Release() did not finish after append")
	}
}

func TestAppendRejectsReplacementBeforeDescriptorOpen(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shard := shardName("host", "abcdef123456")
	shardPath := filepath.Join(recordDir, auditDirectory, shard)
	if err := os.WriteFile(shardPath, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Release() })
	actualLstat := recordRoot.Lstat
	swapped := false
	ops := systemLedgerOps(projectRoot, recordRoot)
	ops.hostname = func() (string, error) { return "host", nil }
	ops.now = func() time.Time { return time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC) }
	ops.recordLstat = func(name string) (fs.FileInfo, error) {
		info, err := actualLstat(name)
		if name != filepath.Join(auditDirectory, shard) || swapped || err != nil {
			return info, err
		}
		swapped = true
		if err := os.Rename(shardPath, shardPath+".original"); err != nil {
			return nil, err
		}
		if err := os.WriteFile(shardPath, []byte("replacement\n"), 0o600); err != nil {
			return nil, err
		}
		return info, nil
	}
	err = appendForIdentityWithOps(context.Background(), identity, guard, projectRoot, recordRoot, []Event{{Event: "STAGE_STARTED"}}, &ops)
	if !errors.Is(err, ErrInvalidRoot) {
		t.Errorf("append error = %v, want ErrInvalidRoot", err)
	}
	data, err := os.ReadFile(shardPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "replacement\n" {
		t.Errorf("replacement shard = %q, want preserved replacement bytes", data)
	}
}

func mustOpenRoot(t *testing.T, parent *os.Root, name string) *os.Root {
	t.Helper()
	root, err := parent.OpenRoot(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}
