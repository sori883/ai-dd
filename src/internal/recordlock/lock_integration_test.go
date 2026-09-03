//go:build integration

package recordlock

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordLockIntegrationSerializesSameIdentityAcrossProcesses(t *testing.T) {
	if os.Getenv("AIDLC_RECORDLOCK_HELPER") == "1" {
		runRecordLockHelper(t)
		return
	}
	t.Parallel()
	project := t.TempDir()
	firstAcquired := filepath.Join(t.TempDir(), "first-acquired")
	secondAcquired := filepath.Join(t.TempDir(), "second-acquired")
	firstRelease := filepath.Join(t.TempDir(), "first-release")
	secondRelease := filepath.Join(t.TempDir(), "second-release")

	first := startRecordLockHelper(t, project, firstAcquired, firstRelease)
	waitForPath(t, firstAcquired)
	second := startRecordLockHelper(t, project, secondAcquired, secondRelease)
	assertPathAbsent(t, secondAcquired, 150*time.Millisecond)
	if err := os.WriteFile(firstRelease, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForPath(t, secondAcquired)
	if err := os.WriteFile(secondRelease, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first helper error = %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second helper error = %v", err)
	}
}

func TestRecordLockIntegrationSeparatesDifferentIdentities(t *testing.T) {
	if os.Getenv("AIDLC_RECORDLOCK_HELPER") == "1" {
		runRecordLockHelper(t)
		return
	}
	t.Parallel()
	project := t.TempDir()
	firstAcquired := filepath.Join(t.TempDir(), "first-acquired")
	secondAcquired := filepath.Join(t.TempDir(), "second-acquired")
	firstRelease := filepath.Join(t.TempDir(), "first-release")
	secondRelease := filepath.Join(t.TempDir(), "second-release")
	first := startRecordLockHelperWithIntent(t, project, "build", firstAcquired, firstRelease)
	second := startRecordLockHelperWithIntent(t, project, "test", secondAcquired, secondRelease)
	waitForPath(t, firstAcquired)
	waitForPath(t, secondAcquired)
	if err := os.WriteFile(firstRelease, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondRelease, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatalf("first helper error = %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second helper error = %v", err)
	}
}

func startRecordLockHelper(t *testing.T, project, acquired, release string) *recordLockProcess {
	return startRecordLockHelperWithIntent(t, project, "build", acquired, release)
}

func startRecordLockHelperWithIntent(t *testing.T, project, intent, acquired, release string) *recordLockProcess {
	t.Helper()
	command := os.Args[0]
	cmd := exec.Command(command, "-test.run", "^TestRecordLockIntegrationSerializesSameIdentityAcrossProcesses$", "-test.v")
	cmd.Env = append(os.Environ(),
		"AIDLC_RECORDLOCK_HELPER=1",
		"AIDLC_RECORDLOCK_PROJECT="+project,
		"AIDLC_RECORDLOCK_INTENT="+intent,
		"AIDLC_RECORDLOCK_ACQUIRED="+acquired,
		"AIDLC_RECORDLOCK_RELEASE="+release,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return &recordLockProcess{wait: cmd.Wait}
}

type recordLockProcess struct {
	wait func() error
}

func (p *recordLockProcess) Wait() error { return p.wait() }

func runRecordLockHelper(t *testing.T) {
	project := os.Getenv("AIDLC_RECORDLOCK_PROJECT")
	intent := os.Getenv("AIDLC_RECORDLOCK_INTENT")
	acquired := os.Getenv("AIDLC_RECORDLOCK_ACQUIRED")
	release := os.Getenv("AIDLC_RECORDLOCK_RELEASE")
	identity, err := NewIdentity(project, "default", intent)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := Acquire(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := guard.Release(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.WriteFile(acquired, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(release); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func assertPathAbsent(t *testing.T, path string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("path %s appeared while lock was held: %v", path, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
