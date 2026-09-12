// Package app connects Space documents to one conversation's temporary state.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

// Service locates one worktree and its fixed executable.
type Service struct {
	Root, Binary string
	hookClock    *hookClock
	writeSession func(string, string, []byte) error
}

// Private clock declaration permits deterministic hook wait tests.
type hookClock struct {
	now  func() time.Time
	wait func(time.Duration)
}

// Session is temporary conversation state; it is not the canonical work record.
type Session struct {
	Space, Intent, Turn, Tool, RuleTurn, RuleHash string
}

// HookInput contains only policy fields from the observed Codex hook protocol.
type HookInput struct {
	AgentID   string          `json:"agent_id"`
	AgentType string          `json:"agent_type"`
	Response  json.RawMessage `json:"tool_response"`
	Prompt    string          `json:"prompt"`
	Event     string          `json:"hook_event_name"`
	Session   string          `json:"session_id"`
	Turn      string          `json:"turn_id"`
	Tool      string          `json:"tool_name"`
	ID        string          `json:"tool_use_id"`
	Active    bool            `json:"stop_hook_active"`
	Input     struct {
		Command   string `json:"command"`
		AgentType string `json:"agent_type"`
		TaskName  string `json:"task_name"`
		Target    string `json:"target"`
	} `json:"tool_input"`
}

func invalid(message string) error { return fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
func sessionPath(session string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`).MatchString(session) {
		return "", invalid("invalid session ID")
	}
	return "aidlc/.runtime/flow/sessions/" + session + ".txt", nil
}

// Inspect reads state without creating it; an unseen session remains unrecorded.
func (s Service) Inspect(session string) (Session, error) {
	name, err := sessionPath(session)
	if err != nil {
		return Session{}, err
	}
	raw, err := okfmemory.ReadFile(s.Root, name)
	if os.IsNotExist(err) {
		return Session{}, nil
	}
	if err != nil {
		return Session{}, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Session{}, invalid("invalid session text")
		}
		if _, exists := values[key]; exists {
			return Session{}, invalid("duplicate session field")
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return Session{}, invalid("invalid session encoding")
		}
		values[key] = decoded
	}
	if len(values) != 6 {
		return Session{}, invalid("invalid session fields")
	}
	for _, key := range []string{"space", "intent", "turn", "tool", "rule_turn", "rule_hash"} {
		if _, ok := values[key]; !ok {
			return Session{}, invalid("missing session field")
		}
	}
	return Session{Space: values["space"], Intent: values["intent"], Turn: values["turn"], Tool: values["tool"], RuleTurn: values["rule_turn"], RuleHash: values["rule_hash"]}, nil
}
func (s Service) save(session string, state Session) error {
	name, err := sessionPath(session)
	if err != nil {
		return err
	}
	var out strings.Builder
	for _, field := range []struct{ key, value string }{{"space", state.Space}, {"intent", state.Intent}, {"turn", state.Turn}, {"tool", state.Tool}, {"rule_turn", state.RuleTurn}, {"rule_hash", state.RuleHash}} {
		fmt.Fprintf(&out, "%s=%s\n", field.key, url.QueryEscape(field.value))
	}
	write := s.writeSession
	if write == nil {
		write = okfmemory.WriteFile
	}
	return write(s.Root, name, []byte(out.String()))
}
func (s Service) store(space string) bundleStore {
	return bundleStore{Root: s.Root, Space: space, Bundle: filepath.Join(s.Root, "aidlc/spaces", space, "knowledge")}
}
func (s Service) withSession(session string, fn func(*Session) ([]byte, error)) ([]byte, error) {
	if _, err := sessionPath(session); err != nil {
		return nil, err
	}
	release, err := filestore.Lock(s.Root, "session-"+session)
	if err != nil {
		return nil, err
	}
	defer release()
	state, err := s.Inspect(session)
	if err != nil {
		return nil, err
	}
	return fn(&state)
}

func (s Service) hookTiming() hookClock {
	if s.hookClock != nil {
		return *s.hookClock
	}
	return hookClock{now: time.Now, wait: time.Sleep}
}

// Hook delivery has a bounded retry budget; interactive CLI locking stays
// immediate. An existing lock is never removed or replaced by the waiter.
func (s Service) withHookSession(session string, fn func(*Session) ([]byte, error)) (result []byte, resultErr error) {
	if _, err := sessionPath(session); err != nil {
		return nil, err
	}
	clock := s.hookTiming()
	deadline := clock.now().Add(2 * time.Second)
	for {
		release, err := filestore.Lock(s.Root, "session-"+session)
		if err == nil {
			defer func() {
				if err := release(); err != nil {
					resultErr = errors.Join(resultErr, fmt.Errorf("release hook session lock: %w", err))
				}
			}()
			state, err := s.Inspect(session)
			if err != nil {
				return nil, err
			}
			return fn(&state)
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		remaining := deadline.Sub(clock.now())
		if remaining <= 0 {
			return nil, fmt.Errorf("hook session lock retry deadline exceeded: %w", err)
		}
		clock.wait(min(20*time.Millisecond, remaining))
	}
}
func (s Service) rules(space string) (string, string, error) {
	store := s.store(space)
	// A list read validates the caller's project/Space path before any Rule access.
	if err := store.ValidateRoot(); err != nil {
		return "", "", err
	}
	entry, err := okfmemory.Read(store.Bundle, "rules/entry")
	if err != nil {
		return "", "", err
	}
	if entry.String("type") != "Rule" {
		return "", "", invalid("entry must have type Rule")
	}
	entryRaw, err := okfmemory.ReadFile(store.Bundle, "rules/entry.md")
	if err != nil {
		return "", "", err
	}
	all := string(entryRaw)
	fingerprint := "rules/entry.md\x00" + all
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).FindAllStringSubmatch(entry.Body, -1)
	if len(links) == 0 {
		return "", "", invalid("Rule entry has no explicit links")
	}
	for _, link := range links {
		target := link[1]
		if strings.ContainsAny(target, "#?:\\") {
			return "", "", invalid("Rule link must name a local Concept")
		}
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Join("rules", target)
		}
		if !strings.HasSuffix(target, ".md") {
			return "", "", invalid("Rule link must name a Markdown file")
		}
		doc, err := okfmemory.Read(store.Bundle, strings.TrimSuffix(target, ".md"))
		if err != nil {
			return "", "", err
		}
		if doc.String("type") != "Rule" {
			return "", "", invalid("required Concept must have type Rule")
		}
		raw, err := okfmemory.ReadFile(store.Bundle, target)
		if err != nil {
			return "", "", err
		}
		all += "\n" + string(raw)
		fingerprint += target + "\x00" + string(raw)
	}
	if len(all) > 16*1024 {
		return "", "", invalid("required Rules exceed 16 KiB; reorganize without truncation")
	}
	return all, filestore.Hash([]byte(fingerprint)), nil
}
func (s Service) draftPath(session string) string {
	return filepath.Join(s.Root, "aidlc/.runtime/drafts", session+".md")
}
func (s Service) sameDraft(file, session string) bool {
	if filepath.Clean(file) != file {
		return false
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(s.Root, file)
	}
	return file == s.draftPath(session)
}
