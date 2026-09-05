package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestRecordHumanTurnUsesIdentityBoundAppendWithoutPromptFields(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "aidlc", ".aidlc-clone-id"), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(clone id): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "default", "intents", "build")))
	if err != nil {
		t.Fatalf("OpenRoot(record): %v", err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity(): %v", err)
	}

	if err := RecordHumanTurn(context.Background(), identity, projectRoot, recordRoot); err != nil {
		t.Fatalf("RecordHumanTurn() error = %v", err)
	}
	var records []AuditRecord
	if err := recordlock.With(context.Background(), identity, func(guard *recordlock.Guard) error {
		var err error
		records, err = ReadEvents(context.Background(), identity, guard, projectRoot, recordRoot)
		return err
	}); err != nil {
		t.Fatalf("ReadEvents() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one HUMAN_TURN", records)
	}
	if records[0].Event != "HUMAN_TURN" {
		t.Errorf("event = %q, want HUMAN_TURN", records[0].Event)
	}
	if len(records[0].Fields) != 0 {
		t.Errorf("HUMAN_TURN fields = %#v, want no prompt or choice fields", records[0].Fields)
	}
}

func TestRecordHumanTurnRejectsNilContextBeforeMutation(t *testing.T) {
	projectDir := t.TempDir()
	recordDir := filepath.Join(projectDir, "aidlc", "spaces", "default", "intents", "build")
	if err := os.MkdirAll(recordDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(record): %v", err)
	}
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatalf("OpenRoot(project): %v", err)
	}
	t.Cleanup(func() { _ = projectRoot.Close() })
	recordRoot, err := projectRoot.OpenRoot(filepath.ToSlash(filepath.Join("aidlc", "spaces", "default", "intents", "build")))
	if err != nil {
		t.Fatalf("OpenRoot(record): %v", err)
	}
	t.Cleanup(func() { _ = recordRoot.Close() })
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatalf("NewIdentity(): %v", err)
	}
	if err := RecordHumanTurn(nil, identity, projectRoot, recordRoot); err == nil {
		t.Fatal("RecordHumanTurn(nil context) error = nil, want error")
	}
	if _, err := recordRoot.Lstat("audit"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("audit after nil context = %v, want absent", err)
	}
}
