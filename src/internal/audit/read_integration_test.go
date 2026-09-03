//go:build integration

package audit

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestReadEventsIntegrationReadsBoundedAuditShards(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	content := "# AI-DLC Audit Log\n\n## Human Turn\n" +
		"**Timestamp**: 2026-09-04T10:00:00Z\n" +
		"**Event**: HUMAN_TURN\n**Prompt**: inspect\n\n---\n"
	if err := os.WriteFile(filepath.Join(recordDir, auditDirectory, "a.md"), []byte(content), 0o600); err != nil {
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

	records, err := ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ReadEvents() records = %d, want 1", len(records))
	}
	wantTime := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	got := records[0]
	if got.Event != "HUMAN_TURN" || !got.Timestamp.Equal(wantTime) || got.Shard != "audit/a.md" || got.Position != 0 || got.Fields["Prompt"] != "inspect" {
		t.Fatalf("ReadEvents() record = %#v, want canonical audit record", got)
	}
	if _, err := projectRoot.Stat("."); err != nil {
		t.Fatalf("project root unusable after ReadEvents(): %v", err)
	}
	if _, err := recordRoot.Stat("."); err != nil {
		t.Fatalf("record root unusable after ReadEvents(): %v", err)
	}
}

func TestReadEventsIntegrationTreatsMissingAuditDirectoryAsEmpty(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatal(err)
	}
	projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
	defer func() { _ = projectRoot.Close() }()
	defer func() { _ = recordRoot.Close() }()
	defer func() { _ = guard.Release() }()

	records, err := ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
	if err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Fatalf("ReadEvents() records = %#v, want non-nil empty slice", records)
	}
}

func TestValidateRecordBindingIntegrationDoesNotReadAuditBody(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
	if err := os.MkdirAll(filepath.Join(recordDir, auditDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recordDir, auditDirectory, "broken.md"), []byte("not a canonical audit shard"), 0o600); err != nil {
		t.Fatal(err)
	}

	projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
	defer func() { _ = projectRoot.Close() }()
	defer func() { _ = recordRoot.Close() }()
	defer func() { _ = guard.Release() }()

	if err := ValidateRecordBinding(context.Background(), identity, guard, projectRoot, recordRoot); err != nil {
		t.Fatalf("ValidateRecordBinding() error = %v, want nil without reading audit body", err)
	}
}

func TestValidateRecordBindingIntegrationRejectsUnboundInputs(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		defer func() { _ = guard.Release() }()
		if err := ValidateRecordBinding(nil, identity, guard, projectRoot, recordRoot); !errors.Is(err, fs.ErrInvalid) {
			t.Fatalf("ValidateRecordBinding() error = %v, want fs.ErrInvalid", err)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		defer func() { _ = guard.Release() }()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := ValidateRecordBinding(ctx, identity, guard, projectRoot, recordRoot); !errors.Is(err, context.Canceled) {
			t.Fatalf("ValidateRecordBinding() error = %v, want context.Canceled", err)
		}
	})

	t.Run("nil guard", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		defer func() { _ = guard.Release() }()
		if err := ValidateRecordBinding(context.Background(), identity, nil, projectRoot, recordRoot); !errors.Is(err, ErrGuardNotHeld) {
			t.Fatalf("ValidateRecordBinding() error = %v, want ErrGuardNotHeld", err)
		}
	})

	t.Run("released guard", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		if err := guard.Release(); err != nil {
			t.Fatal(err)
		}
		if err := ValidateRecordBinding(context.Background(), identity, guard, projectRoot, recordRoot); !errors.Is(err, ErrGuardNotHeld) {
			t.Fatalf("ValidateRecordBinding() error = %v, want ErrGuardNotHeld", err)
		}
	})

	t.Run("wrong guard", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		defer func() { _ = guard.Release() }()
		otherIdentity, err := recordlock.NewIdentity(projectDir, "default", "other")
		if err != nil {
			t.Fatal(err)
		}
		otherGuard, err := recordlock.Acquire(context.Background(), otherIdentity)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = otherGuard.Release() }()
		if err := ValidateRecordBinding(context.Background(), identity, otherGuard, projectRoot, recordRoot); !errors.Is(err, ErrGuardIdentity) {
			t.Fatalf("ValidateRecordBinding() error = %v, want ErrGuardIdentity", err)
		}
	})

	t.Run("wrong record root", func(t *testing.T) {
		projectDir := t.TempDir()
		recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(recordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
		defer func() { _ = projectRoot.Close() }()
		defer func() { _ = recordRoot.Close() }()
		defer func() { _ = guard.Release() }()
		otherProject := t.TempDir()
		otherRecordDir := filepath.Join(otherProject, cloneIDDirectory, "spaces", "default", "intents", "build")
		if err := os.MkdirAll(otherRecordDir, 0o700); err != nil {
			t.Fatal(err)
		}
		otherProjectRoot, err := os.OpenRoot(otherProject)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = otherProjectRoot.Close() }()
		otherRecordRoot, err := otherProjectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = otherRecordRoot.Close() }()
		if err := ValidateRecordBinding(context.Background(), identity, guard, projectRoot, otherRecordRoot); !errors.Is(err, ErrInvalidRoot) {
			t.Fatalf("ValidateRecordBinding() error = %v, want ErrInvalidRoot", err)
		}
	})
}

func TestReadEventsIntegrationRejectsNonregularShard(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(string) error
		needsSymlink bool
	}{
		{
			name: "directory",
			setup: func(auditDir string) error {
				return os.Mkdir(filepath.Join(auditDir, "bad.md"), 0o700)
			},
		},
		{
			name:         "symlink",
			needsSymlink: true,
			setup: func(auditDir string) error {
				if err := os.WriteFile(filepath.Join(auditDir, "target.md"), []byte("# Audit\n"), 0o600); err != nil {
					return err
				}
				return os.Symlink("target.md", filepath.Join(auditDir, "bad.md"))
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			recordDir := filepath.Join(projectDir, cloneIDDirectory, "spaces", "default", "intents", "build")
			auditDir := filepath.Join(recordDir, auditDirectory)
			if err := os.MkdirAll(auditDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := test.setup(auditDir); err != nil {
				if test.needsSymlink && runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission) {
					t.Skipf("Windows symlink permissions: %v", err)
				}
				t.Fatal(err)
			}
			projectRoot, recordRoot, identity, guard := openAuditReadRoots(t, projectDir, recordDir)
			defer func() { _ = projectRoot.Close() }()
			defer func() { _ = recordRoot.Close() }()
			defer func() { _ = guard.Release() }()

			if _, err := ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot); !errors.Is(err, ErrInvalidRoot) {
				t.Fatalf("ReadEvents() error = %v, want ErrInvalidRoot", err)
			}
		})
	}
}

func openAuditReadRoots(t *testing.T, projectDir, recordDir string) (*os.Root, *os.Root, recordlock.Identity, *recordlock.Guard) {
	t.Helper()
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	recordRoot, err := projectRoot.OpenRoot(filepath.Join(cloneIDDirectory, "spaces", "default", "intents", "build"))
	if err != nil {
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		_ = recordRoot.Close()
		_ = projectRoot.Close()
		t.Fatal(err)
	}
	return projectRoot, recordRoot, identity, guard
}
