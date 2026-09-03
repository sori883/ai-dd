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
