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
	st, err := s.Create("検索できる記録")
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.TrimSuffix(s.path(st.ID), "state.json") + "work-log.md"
	if err := filestore.WriteFile(s.Root, legacy, []byte("legacy untouched")); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	for _, reason := range []string{"first reason", "second reason"} {
		st, err = s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: reason})
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
	for _, existing := range []bool{false, true} {
		points := []string{"base", "pending", "log", "final"}
		if existing {
			points = points[1:]
		}
		for _, point := range points {
			t.Run(map[bool]string{false: "fresh", true: "existing"}[existing]+"/"+point, func(t *testing.T) {
				s := flowStore(t)
				st, err := s.Create("recover")
				if err != nil {
					t.Fatal(err)
				}
				if existing {
					seedWorkLog(t, s, st, "old history\n")
				}
				writes := 0
				s.write = func(root, name string, raw []byte) error {
					writes++
					n := map[string]int{"base": 1, "pending": 2, "log": 3, "final": 4}[point]
					if existing {
						n--
					}
					if writes == n {
						return errors.New("injected " + point)
					}
					return filestore.WriteFile(root, name, raw)
				}
				request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "recover reason"}
				if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
					t.Fatal("partial write returned success")
				}
				current, err := s.Read(st.ID)
				if err != nil || current.Revision != st.Revision {
					t.Fatal("partial write advanced state", err)
				}
				name := workLogPath(s, st.ID)
				before, readErr := filestore.ReadFile(s.Root, name)
				if point != "base" {
					if readErr != nil {
						t.Fatal(readErr)
					}
					if _, err = okfmemory.Parse(before); err != nil {
						t.Fatal("partial write poisoned OKF", err)
					}
					if _, err = okfmemory.Search(filepath.Join(s.Root, "aidlc/spaces", s.Space, "knowledge"), "", nil); err != nil {
						t.Fatal("partial write poisoned search", err)
					}
				}
				if point == "log" || point == "final" {
					pendingJSON, err := json.Marshal(current.PendingReopen)
					if err != nil {
						t.Fatal(err)
					}
					var pending map[string]any
					if err = json.Unmarshal(pendingJSON, &pending); err != nil {
						t.Fatal(err)
					}
					afterHash, _ := pending["log_after_hash"].(string)
					if len(afterHash) != 64 {
						t.Fatal("missing completed document hash")
					}
					different := request
					different.Reason = "different"
					if _, err = s.Transition(st.ID, st.Revision, different); err == nil {
						t.Fatal("different request accepted")
					}
				}
				s.write = nil
				result, err := s.Transition(st.ID, st.Revision, request)
				if err != nil {
					t.Fatal("same request recovery failed", err)
				}
				after, err := filestore.ReadFile(s.Root, name)
				if err != nil {
					t.Fatal(err)
				}
				doc, err := okfmemory.Parse(after)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(doc.Body, "recover reason") != 1 || result.Revision != st.Revision+1 {
					t.Fatalf("recovery duplicated or advanced incorrectly: %s", after)
				}
				if point == "final" && string(before) != string(after) {
					t.Fatal("final retry rewrote completed document")
				}
				if current.PendingReopen != nil {
					generated := doc.Metadata["generated"].(map[string]any)
					if generated["at"] != current.PendingReopen.At {
						t.Fatal("retry changed timestamp")
					}
				}
			})
		}
	}
}

func TestOKFWorkLogRecoveryPendingValidation(t *testing.T) {
	for _, field := range []string{"log_hash", "log_after_hash"} {
		for _, value := range []any{nil, "bad"} {
			t.Run(field+"/"+map[bool]string{true: "missing", false: "bad"}[value == nil], func(t *testing.T) {
				s := flowStore(t)
				st, err := s.Create("pending validation")
				if err != nil {
					t.Fatal(err)
				}
				raw, err := filestore.ReadFile(s.Root, s.path(st.ID))
				if err != nil {
					t.Fatal(err)
				}
				var state map[string]any
				if err = json.Unmarshal(raw, &state); err != nil {
					t.Fatal(err)
				}
				pending := map[string]any{"revision": st.Revision, "from": st.Stage, "to": "discovery", "reason": "retry", "at": "2020-01-01T00:00:00Z", "log_hash": strings.Repeat("a", 64), "log_after_hash": strings.Repeat("b", 64), "had_log": true}
				if value == nil {
					delete(pending, field)
				} else {
					pending[field] = value
				}
				state["pending_reopen"] = pending
				raw, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err = filestore.WriteFile(s.Root, s.path(st.ID), raw); err != nil {
					t.Fatal(err)
				}
				if _, err = s.Read(st.ID); err == nil {
					t.Fatal("invalid pending hash accepted")
				}
			})
		}
	}
}

func TestOKFWorkLogRecoveryRejects(t *testing.T) {
	for _, mode := range []string{"malformed", "wrong type", "wrong intent", "directory", "symlink", "oversize", "metadata overflow", "missing pending", "changed pending"} {
		t.Run(mode, func(t *testing.T) {
			s := flowStore(t)
			st, err := s.Create("reject")
			if err != nil {
				t.Fatal(err)
			}
			name := workLogPath(s, st.ID)
			original := seedWorkLog(t, s, st, "history\n")
			request := TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"}
			switch mode {
			case "malformed":
				original = []byte("plain markdown")
			case "wrong type":
				original = []byte(strings.Replace(string(original), "type: work-log", "type: adr", 1))
			case "wrong intent":
				original = []byte(strings.Replace(string(original), st.ID, strings.Repeat("f", 32), 1))
			case "oversize":
				original = []byte(strings.Repeat("a", filestore.MaxBytes+1))
			case "metadata overflow":
				doc, err := okfmemory.Parse(original)
				if err != nil {
					t.Fatal(err)
				}
				doc.Body = strings.Repeat("a", filestore.MaxBytes-len(original)+len(doc.Body)-10)
				original, err = doc.Bytes()
				if err != nil {
					t.Fatal(err)
				}
			case "missing pending", "changed pending":
				s.write = func(root, path string, raw []byte) error {
					if strings.HasSuffix(path, "work-log.md") {
						return errors.New("log failure")
					}
					return filestore.WriteFile(root, path, raw)
				}
				if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
					t.Fatal("expected save failure")
				}
				s.write = nil
				original = append(original, []byte("changed")...)
			}
			if err = filestore.WriteFile(s.Root, name, original); err != nil {
				t.Fatal(err)
			}
			if mode == "directory" || mode == "symlink" || mode == "missing pending" {
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
			if _, err = s.Transition(st.ID, st.Revision, request); err == nil {
				t.Fatal("invalid log accepted")
			}
			after, err := filestore.ReadFile(s.Root, s.path(st.ID))
			if err != nil || string(after) != string(before) {
				t.Fatal("rejection changed state", err)
			}
			if mode != "directory" && mode != "missing pending" {
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
	st, err := s.Create("special file")
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
		_, err := s.Transition(st.ID, st.Revision, TransitionRequest{Action: "reopen", Stage: "discovery", Reason: "retry"})
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
