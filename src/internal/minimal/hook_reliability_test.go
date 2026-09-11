package minimal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
)

func TestHookRecoveryGuidance(t *testing.T) {
	for _, event := range []string{"PreToolUse", "Stop"} {
		t.Run(event, func(t *testing.T) {
			s := Service{Root: t.TempDir(), Binary: "/opt/aidlc"}
			initial := Session{Space: "selected-space", Intent: "selected-intent", Turn: "turn", Tool: "running"}
			if err := s.save("session", initial); err != nil {
				t.Fatal(err)
			}
			in := HookInput{Event: event, Session: "session", Turn: "turn", Tool: "Bash", ID: "next"}
			in.Input.Command = "echo next"
			out, err := s.Hook(in)
			if err != nil {
				t.Fatal(err)
			}
			if event == "PreToolUse" && !deny(out) || event == "Stop" && out["decision"] != "block" {
				t.Fatal("busy operation did not expose recovery guidance", out)
			}
			raw, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"success or failure", "same Space, Intent and session", "main AI", "Poll a running Bash process to terminal", "does not release worker assignments", "session bind 'selected-intent' --space 'selected-space' --session 'session' --recover"} {
				if !strings.Contains(string(raw), text) {
					t.Errorf("actual %s response lacks %q: %s", event, text, raw)
				}
			}
			if strings.Contains(string(raw), "Only after an edit tool returned a failure") || strings.Contains(string(raw), "only the failed tool slot") {
				t.Fatal("recovery guidance still limits recovery to failed edits")
			}
			after, err := s.Inspect("session")
			if err != nil || after != initial {
				t.Fatal("guidance changed session", after, err)
			}
		})
	}
}

func TestHookSessionContention(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release bool
	}{{"released", true}, {"timeout", false}} {
		t.Run(tc.name, func(t *testing.T) {
			s := Service{Root: t.TempDir()}
			if err := s.save("session", Session{Turn: "turn", Tool: "tool"}); err != nil {
				t.Fatal(err)
			}
			unlock, err := filestore.Lock(s.Root, "session-session")
			if err != nil {
				t.Fatal(err)
			}
			held := true
			t.Cleanup(func() {
				if held {
					if err := unlock(); err != nil {
						t.Error(err)
					}
				}
			})
			now := time.Unix(1, 0)
			waited := time.Duration(0)
			s.hookClock = &hookClock{now: func() time.Time { return now }, wait: func(d time.Duration) {
				waited += d
				now = now.Add(d)
				if tc.release && held {
					if err := unlock(); err != nil {
						t.Fatal(err)
					}
					held = false
				}
			}}
			out, err := s.Hook(HookInput{Event: "PostToolUse", Session: "session", Turn: "later-turn", Tool: "Bash", ID: "tool"})
			if err != nil {
				t.Fatal(err)
			}
			state, err := s.Inspect("session")
			if err != nil {
				t.Fatal(err)
			}
			if tc.release {
				if len(out) != 0 || state.Tool != "" || waited == 0 {
					t.Fatalf("delivered Post lost after transient contention: out=%+v Tool=%q waited=%v", out, state.Tool, waited)
				}
			} else {
				if out["continue"] != false || state.Tool != "tool" || waited != 2*time.Second {
					t.Fatalf("timeout must preserve Tool and consume bounded budget: %+v %+v %v", out, state, waited)
				}
				if _, err := os.Stat(filepath.Join(s.Root, "aidlc/.runtime/locks/session-session")); err != nil {
					t.Fatal("lock stolen", err)
				}
			}
		})
	}
	t.Run("non_contention", func(t *testing.T) {
		s := Service{Root: filepath.Join(t.TempDir(), "missing")}
		s.hookClock = &hookClock{now: time.Now, wait: func(time.Duration) { t.Fatal("retried non-contention error") }}
		out, err := s.Hook(HookInput{Event: "PostToolUse", Session: "session", ID: "tool"})
		if err != nil || out["continue"] != false || !strings.Contains(out["stopReason"].(string), "no such file") {
			t.Fatalf("non-contention error lost: %+v %v", out, err)
		}
	})
}

func TestHookTerminalPersistence(t *testing.T) {
	for _, tc := range []struct{ name, event, id, session, want string }{{"matching", "PostToolUse", "tool", "session", ""}, {"duplicate", "PostToolUse", "tool", "session", ""}, {"old_id", "PostToolUse", "old", "session", "tool"}, {"other_session", "PostToolUse", "tool", "other", "tool"}, {"stop_without_terminal", "Stop", "", "session", "tool"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := Service{Root: t.TempDir()}
			initial := Session{Space: "default", Intent: "intent", Turn: "new-turn", Tool: "tool", RuleTurn: "old-turn", RuleHash: "hash"}
			if err := s.save("session", initial); err != nil {
				t.Fatal(err)
			}
			if err := s.save("other", Session{Tool: "new-tool"}); err != nil {
				t.Fatal(err)
			}
			out, err := s.Hook(HookInput{Event: tc.event, Session: tc.session, Turn: "original-turn", Tool: "Bash", ID: tc.id})
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "duplicate" {
				out, err = s.Hook(HookInput{Event: tc.event, Session: tc.session, Tool: "Bash", ID: tc.id})
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.Inspect("session")
			if err != nil {
				t.Fatal(err)
			}
			initial.Tool = tc.want
			if !reflect.DeepEqual(got, initial) {
				t.Fatalf("terminal changed wrong fields: %+v want %+v", got, initial)
			}
			other, err := s.Inspect("other")
			if err != nil || other.Tool != "new-tool" {
				t.Fatalf("other session changed: %+v %v", other, err)
			}
			if tc.event == "Stop" && out["decision"] != "block" {
				t.Fatal("missing terminal was silently recovered", out)
			}
		})
	}
	t.Run("callback_failure_and_retry", func(t *testing.T) {
		s := Service{Root: t.TempDir()}
		if err := s.save("session", Session{Tool: "tool"}); err != nil {
			t.Fatal(err)
		}
		cause := errors.New("injected save failure")
		_, err := s.withHookSession("session", func(state *Session) ([]byte, error) { state.Tool = ""; return nil, cause })
		if !errors.Is(err, cause) {
			t.Fatal("save error lost", err)
		}
		state, err := s.Inspect("session")
		if err != nil || state.Tool != "tool" {
			t.Fatalf("failed callback persisted: %+v %v", state, err)
		}
		out, err := s.Hook(HookInput{Event: "PostToolUse", Session: "session", Tool: "Bash", ID: "tool"})
		if err != nil || len(out) != 0 {
			t.Fatal("retry failed", out, err)
		}
		state, err = s.Inspect("session")
		if err != nil || state.Tool != "" {
			t.Fatal("retry did not clear", state, err)
		}
	})
	t.Run("release_failure", func(t *testing.T) {
		s := Service{Root: t.TempDir()}
		_, err := s.withHookSession("session", func(*Session) ([]byte, error) {
			return nil, os.WriteFile(filepath.Join(s.Root, "aidlc/.runtime/locks/session-session/held"), []byte("fixture"), 0600)
		})
		if err == nil || !strings.Contains(err.Error(), "release hook session lock") {
			t.Fatalf("lock release failure not diagnosed: %v", err)
		}
	})
}
