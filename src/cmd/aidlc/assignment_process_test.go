package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

type assignmentProcessRecord struct {
	Nonce, Peer, Cwd                           string
	PID                                        int
	StartedAt, ReadyAt, WorkStartedAt, EndedAt time.Time
	Error                                      string
}

func assignmentRunProcess(dir, nonce, peer string, wait, work time.Duration) (result error) {
	valid := regexp.MustCompile(`^[a-z0-9]{1,64}$`)
	if !valid.MatchString(nonce) || !valid.MatchString(peer) || nonce == peer || wait <= 0 || wait > 90*time.Second || work <= 0 || work > 15*time.Second {
		return errors.New("invalid bounded process request")
	}
	file, err := os.OpenFile(filepath.Join(dir, nonce+".process.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		file.Close()
		return err
	}
	record := assignmentProcessRecord{Nonce: nonce, Peer: peer, Cwd: cwd, PID: os.Getpid(), StartedAt: time.Now().UTC()}
	defer func() {
		record.EndedAt = time.Now().UTC()
		if result != nil {
			record.Error = result.Error()
		}
		result = errors.Join(result, json.NewEncoder(file).Encode(record), file.Close())
	}()
	record.ReadyAt = time.Now().UTC()
	if err := os.WriteFile(filepath.Join(dir, nonce+".ready"), []byte(nonce), 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, err := os.ReadFile(filepath.Join(dir, peer+".ready"))
		if err == nil && string(raw) == peer {
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	record.WorkStartedAt = time.Now().UTC()
	time.Sleep(work)
	return nil
}

// TestAssignmentProcess runs only when an isolated live child invokes its marker.
func TestAssignmentProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "assignment-process" {
			continue
		}
		args := os.Args[i+1:]
		if len(args) != 5 {
			os.Exit(64)
		}
		wait, err := strconv.Atoi(args[3])
		if err != nil {
			os.Exit(64)
		}
		work, err := strconv.Atoi(args[4])
		if err != nil {
			os.Exit(64)
		}
		if err := assignmentRunProcess(args[0], args[1], args[2], time.Duration(wait)*time.Millisecond, time.Duration(work)*time.Millisecond); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func assignmentReadProcess(t *testing.T, dir, nonce string) assignmentProcessRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, nonce+".process.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r assignmentProcessRecord
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestAssignmentProcessRendezvous(t *testing.T) {
	t.Run("finite missing peer", func(t *testing.T) {
		dir := t.TempDir()
		start := time.Now()
		err := assignmentRunProcess(dir, "a", "b", 30*time.Millisecond, 30*time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("missing peer did not time out: %v", err)
		}
		r := assignmentReadProcess(t, dir, "a")
		if time.Since(start) > time.Second || r.ReadyAt.IsZero() || !r.WorkStartedAt.IsZero() || r.EndedAt.IsZero() || r.Error == "" {
			t.Fatalf("invalid timeout record: %+v", r)
		}
	})
	t.Run("actual peer and duplicate nonce", func(t *testing.T) {
		dir := t.TempDir()
		done := make(chan error, 2)
		go func() { done <- assignmentRunProcess(dir, "a", "b", time.Second, 50*time.Millisecond) }()
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(filepath.Join(dir, "a.ready")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("first peer never became ready")
			}
			time.Sleep(time.Millisecond)
		}
		if err := assignmentRunProcess(dir, "a", "b", time.Second, 50*time.Millisecond); !errors.Is(err, os.ErrExist) {
			t.Fatalf("duplicate nonce accepted: %v", err)
		}
		go func() { done <- assignmentRunProcess(dir, "b", "a", time.Second, 50*time.Millisecond) }()
		for range 2 {
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		}
		a, b := assignmentReadProcess(t, dir, "a"), assignmentReadProcess(t, dir, "b")
		if a.Nonce != "a" || b.Nonce != "b" || a.Cwd == "" || a.PID == 0 || a.WorkStartedAt.IsZero() || b.WorkStartedAt.IsZero() || !a.WorkStartedAt.Before(b.EndedAt) || !b.WorkStartedAt.Before(a.EndedAt) {
			t.Fatalf("no actual work overlap: %+v %+v", a, b)
		}
	})
}
