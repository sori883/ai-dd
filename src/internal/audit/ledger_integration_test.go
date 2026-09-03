//go:build integration

package audit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/recordlock"
)

func TestAuditIntegrationConcurrentAppendUsesOneCloneShard(t *testing.T) {
	if os.Getenv("AIDLC_AUDIT_HELPER") == "1" {
		runAuditAppendHelper(t)
		return
	}
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile), []byte("abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recordDir := t.TempDir()
	const helperCount = 4
	processes := make([]*auditProcess, 0, helperCount)
	for index := range helperCount {
		processes = append(processes, startAuditHelper(t, projectDir, recordDir, index))
	}
	for index, process := range processes {
		if err := process.Wait(); err != nil {
			t.Fatalf("helper %d error = %v", index, err)
		}
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	shardPath := filepath.Join(recordDir, auditDirectory, shardName(host, "abcdef123456"))
	data, err := os.ReadFile(shardPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, "# AI-DLC Audit Log\n") != 1 {
		t.Errorf("header count = %d, want one", strings.Count(content, "# AI-DLC Audit Log\n"))
	}
	for index := range helperCount {
		if got := strings.Count(content, fmt.Sprintf("**Worker**: %d\n", index)); got != 1 {
			t.Errorf("worker %d count = %d, want one; content=%q", index, got, content)
		}
	}
}

func TestAuditIntegrationConcurrentFirstCloneIDGenerationConverges(t *testing.T) {
	if os.Getenv("AIDLC_AUDIT_HELPER") == "1" {
		runAuditAppendHelper(t)
		return
	}
	projectDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectDir, cloneIDDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	recordDir := t.TempDir()
	first := startAuditHelper(t, projectDir, recordDir, 0)
	second := startAuditHelper(t, projectDir, recordDir, 1)
	if err := first.Wait(); err != nil {
		t.Fatalf("first helper error = %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second helper error = %v", err)
	}
	cloneBytes, err := os.ReadFile(filepath.Join(projectDir, cloneIDDirectory, cloneIDFile))
	if err != nil {
		t.Fatal(err)
	}
	cloneID, err := parseCloneID(cloneBytes)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(recordDir, auditDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != shardNameForHost(cloneID) {
		t.Errorf("audit shards = %+v, want one converged clone shard", entries)
	}
}

type auditProcess struct {
	wait func() error
}

func (p *auditProcess) Wait() error { return p.wait() }

func startAuditHelper(t *testing.T, projectDir, recordDir string, index int) *auditProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "^TestAuditIntegrationConcurrentAppendUsesOneCloneShard$", "-test.v")
	cmd.Env = append(os.Environ(),
		"AIDLC_AUDIT_HELPER=1",
		"AIDLC_AUDIT_PROJECT="+projectDir,
		"AIDLC_AUDIT_RECORD="+recordDir,
		fmt.Sprintf("AIDLC_AUDIT_WORKER=%d", index),
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return &auditProcess{wait: cmd.Wait}
}

func runAuditAppendHelper(t *testing.T) {
	projectDir := os.Getenv("AIDLC_AUDIT_PROJECT")
	recordDir := os.Getenv("AIDLC_AUDIT_RECORD")
	worker := os.Getenv("AIDLC_AUDIT_WORKER")
	identity, err := recordlock.NewIdentity(projectDir, "default", "build")
	if err != nil {
		t.Fatal(err)
	}
	guard, err := recordlock.Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := guard.Release(); err != nil {
			t.Error(err)
		}
	}()
	projectRoot, err := os.OpenRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = projectRoot.Close() }()
	recordRoot, err := os.OpenRoot(recordDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = recordRoot.Close() }()
	event := Event{
		Event:     "STAGE_STARTED",
		Timestamp: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC).Add(time.Duration(worker[0]-'0') * time.Second),
		Fields:    map[string]string{"Worker": worker},
	}
	if err := Append(context.Background(), guard, projectRoot, recordRoot, []Event{event}); err != nil {
		t.Fatal(err)
	}
}
