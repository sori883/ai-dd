package flow

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func workLogPath(s Store, id string) string {
	return "aidlc/spaces/" + s.Space + "/knowledge/log/" + id + "-work-log.md"
}

func TestOKFWorkLogDocument(t *testing.T) {
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "検索できる記録")
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
	if err := filestore.WriteFile(s.Root, legacy, []byte("legacy untouched")); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	for _, reason := range []string{"first reason", "second reason"} {
		st, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: reason})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := filestore.ReadFile(s.Root, workLogPath(s, st.ID))
		if err != nil {
			t.Fatalf("knowledge work-log missing: %v", err)
		}
		doc, err := okfmemory.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if doc.String("type") != "work-log" || doc.String("intent_id") != st.ID || !strings.Contains(doc.String("title"), st.Name) || doc.String("description") == "" || doc.String("status") != "stable" {
			t.Fatalf("metadata: %#v", doc.Metadata)
		}
		generated, ok := doc.Metadata["generated"].(map[string]any)
		if !ok || generated["by"] != "process:aidlc" {
			t.Fatalf("generated: %#v", generated)
		}
		at, err := time.Parse(time.RFC3339Nano, generated["at"].(string))
		if err != nil || at.Before(started) || at.After(time.Now()) || !strings.HasSuffix(generated["at"].(string), "Z") {
			t.Fatalf("generated time: %#v %v", generated, err)
		}
		if strings.Count(doc.Body, reason) != 1 {
			t.Fatalf("missing or duplicate reason: %s", doc.Body)
		}
		if reason == "first reason" {
			doc.Metadata["custom"] = "preserved"
			raw, err = doc.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if err = filestore.WriteFile(s.Root, workLogPath(s, st.ID), raw); err != nil {
				t.Fatal(err)
			}
		} else if doc.String("custom") != "preserved" || strings.Count(doc.Body, "first reason") != 1 {
			t.Fatalf("history lost: %#v", doc)
		}
	}
	bundle := filepath.Join(s.Root, "aidlc/spaces", s.Space, "knowledge")
	matches, err := okfmemory.Search(bundle, "work-log", &st.ID)
	if err != nil || len(matches) != 1 || matches[0].ID != "log/"+st.ID+"-work-log" {
		t.Fatalf("search: %#v %v", matches, err)
	}
	other := strings.Repeat("f", 32)
	matches, err = okfmemory.Search(bundle, "work-log", &other)
	if err != nil || len(matches) != 0 {
		t.Fatalf("intent filter: %#v %v", matches, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Root, legacy))
	if err != nil || string(raw) != "legacy untouched" {
		t.Fatal("legacy log changed", err)
	}
}

func seedWorkLog(t *testing.T, s Store, st State, body string) []byte {
	t.Helper()
	doc := okfmemory.Document{Metadata: map[string]any{"type": "work-log", "intent_id": st.ID, "title": st.Name, "description": "reopen history", "tags": []string{"work-log"}, "generated": map[string]any{"by": "process:aidlc", "at": "2020-01-01T00:00:00Z"}}, Body: body}
	raw, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err = filestore.WriteFile(s.Root, workLogPath(s, st.ID), raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOKFWorkLogRecovery(t *testing.T) {
	for _, point := range []string{"fresh/base", "existing/pending", "existing/log"} {
		t.Run(point, func(t *testing.T) {
			s := flowStore(t)
			st, err := createExecutionFixture(t, s, "recover")
			if err != nil {
				t.Fatal(err)
			}
			existing := strings.HasPrefix(point, "existing/")
			var original []byte
			if existing {
				original = seedWorkLog(t, s, st, "old history\n")
			}
			request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "recover reason"}
			if point == "existing/log" {
				request.Reason = "line one\n## forged marker"
			}
			st = prepareReopenFixture(t, s, st, request)
			injected := errors.New("injected " + point)
			s.write = func(root, name string, raw []byte) error {
				if (point == "fresh/base" || point == "existing/log") && name == workLogPath(s, st.ID) {
					return injected
				}
				if point == "existing/pending" && name == s.path(st.ID) {
					var candidate State
					if err := json.Unmarshal(raw, &candidate); err != nil {
						return err
					}
					if candidate.PendingReopen != nil {
						return injected
					}
				}
				return filestore.WriteFile(root, name, raw)
			}
			if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, request); !errors.Is(err, injected) {
				t.Fatalf("save point: %v", err)
			}
			current, err := s.Read(st.ID)
			if err != nil || current.Revision != st.Revision {
				t.Fatal("partial write advanced state", err)
			}
			if existing {
				before, err := filestore.ReadFile(s.Root, workLogPath(s, st.ID))
				if err != nil || string(before) != string(original) {
					t.Fatal("partial write changed old log", err)
				}
			}
			s.write = nil
			if point == "existing/log" {
				if current.PendingReopen == nil {
					t.Fatal("pending missing")
				}
				if _, err = saveExecutionFixture(t, s, current, current.Revision); err == nil {
					t.Fatal("pending allowed configure")
				}
				if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "pause", Reason: "other"}); err == nil {
					t.Fatal("pending allowed pause")
				}
				if err = s.CheckWork(st.ID); err == nil {
					t.Fatal("pending allowed work")
				}
				different := request
				different.Reason = "different"
				if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, different); err == nil {
					t.Fatal("different request accepted")
				}
			}
			result, err := transitionExecutionFixture(t, s, st.ID, st.Revision, request)
			if err != nil {
				t.Fatal("same request recovery failed", err)
			}
			after, err := filestore.ReadFile(s.Root, workLogPath(s, st.ID))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := okfmemory.Parse(after)
			if err != nil {
				t.Fatal(err)
			}
			if result.Revision != st.Revision+1 || result.PendingReopen != nil {
				t.Fatal("recovery revision/pending")
			}
			reason := "recover reason"
			if point == "existing/log" {
				reason = "forged marker"
			}
			if strings.Count(doc.Body, reason) != 1 || strings.Contains(doc.Body, "\n## forged marker") {
				t.Fatal("reason duplicated or unescaped")
			}
			if existing && !strings.Contains(doc.Body, "old history\n") {
				t.Fatal("old body lost")
			}
			if current.PendingReopen != nil && doc.Metadata["generated"].(map[string]any)["at"] != current.PendingReopen.At {
				t.Fatal("retry changed timestamp")
			}
			matches, err := okfmemory.Search(filepath.Join(s.Root, "aidlc/spaces", s.Space, "knowledge"), "work-log", &st.ID)
			if err != nil || len(matches) != 1 {
				t.Fatal("recovered log not searchable", err)
			}
			if point == "existing/log" {
				if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, request); err == nil {
					t.Fatal("old expect accepted")
				}
			}
		})
	}
}

func TestOKFWorkLogRecoveryRejects(t *testing.T) {
	for _, mode := range []string{"malformed", "wrong type", "wrong intent", "directory", "symlink", "oversize", "before log changed", "after log deleted"} {
		t.Run(mode, func(t *testing.T) {
			s := flowStore(t)
			st, err := createExecutionFixture(t, s, "reject")
			if err != nil {
				t.Fatal(err)
			}
			name := workLogPath(s, st.ID)
			original := seedWorkLog(t, s, st, "history\n")
			request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"}
			st = prepareReopenFixture(t, s, st, request)
			switch mode {
			case "malformed":
				original = []byte("plain markdown")
			case "wrong type":
				original = []byte(strings.Replace(string(original), "type: work-log", "type: adr", 1))
			case "wrong intent":
				original = []byte(strings.Replace(string(original), st.ID, strings.Repeat("f", 32), 1))
			case "oversize":
				original = []byte(strings.Repeat("a", filestore.MaxBytes+1))
			case "before log changed", "after log deleted":
				injected := errors.New("injected log boundary")
				s.write = func(root, path string, raw []byte) error {
					if mode == "before log changed" && path == name {
						return injected
					}
					if mode == "after log deleted" && path == s.path(st.ID) {
						var candidate State
						if err := json.Unmarshal(raw, &candidate); err != nil {
							return err
						}
						if candidate.PendingReopen == nil {
							return injected
						}
					}
					return filestore.WriteFile(root, path, raw)
				}
				if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, request); !errors.Is(err, injected) {
					t.Fatalf("save boundary: %v", err)
				}
				pending, err := s.Read(st.ID)
				if err != nil || pending.PendingReopen == nil {
					t.Fatal("missing durable pending", err)
				}
				if mode == "after log deleted" {
					completed, err := filestore.ReadFile(s.Root, name)
					if err != nil || filestore.Hash(completed) != pending.PendingReopen.LogAfterHash {
						t.Fatal("final failure did not retain completed log", err)
					}
				}
				s.write = nil
				original = append(original, []byte("changed")...)
			}
			if err = filestore.WriteFile(s.Root, name, original); err != nil {
				t.Fatal(err)
			}
			if mode == "directory" || mode == "symlink" || mode == "after log deleted" {
				if err = os.Remove(filepath.Join(s.Root, name)); err != nil {
					t.Fatal(err)
				}
				if mode == "directory" {
					if err = os.Mkdir(filepath.Join(s.Root, name), 0755); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "symlink" {
					target := filepath.Join(t.TempDir(), "outside.md")
					if err = os.WriteFile(target, original, 0644); err != nil {
						t.Fatal(err)
					}
					if err = os.Symlink(target, filepath.Join(s.Root, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			before, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = transitionExecutionFixture(t, s, st.ID, st.Revision, request); err == nil {
				t.Fatal("invalid log accepted")
			}
			after, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil || string(after) != string(before) {
				t.Fatal("rejection changed state", err)
			}
			if mode == "after log deleted" {
				if _, err := os.Lstat(filepath.Join(s.Root, name)); !os.IsNotExist(err) {
					t.Fatal("deleted log recreated", err)
				}
			}
			if mode != "directory" && mode != "after log deleted" {
				after, err = os.ReadFile(filepath.Join(s.Root, name))
				if err != nil || string(after) != string(original) {
					t.Fatal("rejection changed log", err)
				}
			}
		})
	}
}

func TestOKFWorkLogRecoveryFIFO(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named-pipe fixture uses POSIX mkfifo")
	}
	s := flowStore(t)
	st, err := createExecutionFixture(t, s, "special file")
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(s.Root, workLogPath(s, st.ID))
	if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("mkfifo", name).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, out)
	}
	done := make(chan error, 1)
	go func() {
		_, err := transitionExecutionFixture(t, s, st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("fifo accepted")
		}
	case <-time.After(250 * time.Millisecond):
		// Release the old blocking reader before returning the failing assertion.
		writer, err := os.OpenFile(name, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		<-done
		t.Fatal("reopen blocked opening a fifo")
	}
}
