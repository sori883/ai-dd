// Package kdr maintains one OKF record for each named Intent.
package kdr

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

// Store restricts KDR operations to one validated Space bundle.
type Store struct{ Root, Bundle, Space string }

// Saved reports canonical bytes and their hash, including after partial bookkeeping failure.
type Saved struct {
	ID, Path, Hash string
	Raw            []byte
}

var headings = []string{"目的と完成条件", "参照する設計・ルール", "不明点と進め方", "判断と結果", "検証・レビュー", "残件と再開"}

// Parse requires a KDR and six substantive sections while retaining unknown metadata.
func Parse(raw []byte, id string) (okfmemory.Document, error) {
	doc, err := okfmemory.Parse(raw)
	if err != nil {
		return doc, err
	}
	if doc.String("type") != "KDR" || strings.TrimSpace(doc.String("title")) == "" || strings.TrimSpace(doc.String("description")) == "" {
		return doc, invalid("KDR type, title and description are required")
	}
	if id != "" && (!okfmemory.ValidID(id) || doc.String("intent_id") != id) {
		return doc, invalid("KDR intent_id must match its filename")
	}
	sections := sections(doc.Body)
	for _, heading := range headings {
		text := sections[heading]
		if strings.TrimSpace(text) == "" || regexp.MustCompile(`<[^>]+>`).MatchString(text) {
			return doc, invalid("empty or placeholder section: " + heading)
		}
	}
	return doc, nil
}
func sections(body string) map[string]string {
	out := map[string]string{}
	heading := ""
	body = regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllString(body, "")
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			heading = strings.TrimPrefix(line, "## ")
			if _, ok := out[heading]; ok {
				return nil
			}
			out[heading] = ""
			continue
		}
		if heading != "" {
			out[heading] += line + "\n"
		}
	}
	return out
}
func invalid(message string) error { return fmt.Errorf("%s: %w", message, fs.ErrInvalid) }

// Hash identifies exact persisted bytes for compare-and-save operations.
func Hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func (s Store) check() error {
	if !filepath.IsAbs(s.Root) || !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(s.Space) {
		return invalid("invalid project or Space")
	}
	expected := filepath.Join(s.Root, "aidlc", "spaces", s.Space, "knowledge")
	if filepath.Clean(s.Bundle) != expected {
		return invalid("bundle does not match Space")
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	parts := []string{"aidlc", "spaces", s.Space, "knowledge"}
	for i := range parts {
		info, err := root.Lstat(filepath.Join(parts[:i+1]...))
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return invalid("bundle path is not a real directory")
		}
	}
	return nil
}

// Read returns raw canonical bytes even when document validation would fail.
func (s Store) Read(id string) (Saved, error) {
	if err := s.check(); err != nil {
		return Saved{}, err
	}
	if !okfmemory.ValidID(id) {
		return Saved{}, invalid("invalid intent_id")
	}
	result := Saved{ID: id, Path: filepath.Join(s.Bundle, "kdr", id+".md")}
	raw, err := okfmemory.ReadFile(s.Bundle, "kdr/"+id+".md")
	if err != nil {
		return result, err
	}
	result.Raw = raw
	result.Hash = Hash(raw)
	return result, nil
}

// Create assigns a fresh identity and saves body before derived bookkeeping.
func (s Store) Create(raw []byte, actor string) (Saved, error) {
	if err := s.check(); err != nil {
		return Saved{}, err
	}
	release, err := Lock(s.Root, "bundle-"+s.Space)
	if err != nil {
		return Saved{}, err
	}
	defer release()
	doc, err := Parse(raw, "")
	if err != nil {
		return Saved{}, err
	}
	if doc.String("intent_id") != "" {
		return Saved{}, invalid("new draft must not supply intent_id")
	}
	id := fmt.Sprintf("%x", randomID())
	doc.Metadata["intent_id"] = id
	doc.ID = "kdr/" + id
	if err := setActor(&doc, actor); err != nil {
		return Saved{}, err
	}
	encoded, err := doc.Bytes()
	if err != nil {
		return Saved{}, err
	}
	result := Saved{ID: id, Path: filepath.Join(s.Bundle, "kdr", id+".md"), Hash: Hash(encoded), Raw: encoded}
	if _, err := os.Lstat(result.Path); !os.IsNotExist(err) {
		return result, fmt.Errorf("new KDR path already exists: %w", fs.ErrExist)
	}
	return s.save(result, doc, "Creation")
}

// Update is called under the session lock by the runtime. This method acquires
// the Space lock second and performs CAS before writing any KDR data.
func (s Store) Update(id string, raw []byte, expect, actor string) (Saved, error) {
	return s.change(id, raw, expect, actor, false)
}

// Repair restores the same identity; a valid existing body is preserved.
func (s Store) Repair(id string, raw []byte, expect, actor string) (Saved, error) {
	return s.change(id, raw, expect, actor, true)
}
func (s Store) change(id string, raw []byte, expect, actor string, repair bool) (Saved, error) {
	if err := s.check(); err != nil {
		return Saved{}, err
	}
	if !okfmemory.ValidID(id) {
		return Saved{}, invalid("invalid intent_id")
	}
	release, err := Lock(s.Root, "bundle-"+s.Space)
	if err != nil {
		return Saved{}, err
	}
	defer release()
	current, readErr := s.Read(id)
	if readErr != nil && !os.IsNotExist(readErr) {
		return current, readErr
	}
	if os.IsNotExist(readErr) {
		if !repair || expect != "missing" {
			return current, invalid("KDR missing; use explicit repair")
		}
	} else if expect != current.Hash {
		return current, invalid("KDR hash conflict; read current bytes before retry")
	}
	next, err := Parse(raw, id)
	if err != nil {
		return current, err
	}
	next.ID = "kdr/" + id
	old, oldErr := Parse(current.Raw, id)
	if repair && oldErr == nil {
		old.ID = next.ID
		if err := okfmemory.Bookkeeping(s.Bundle, old, "Repair", time.Now()); err != nil {
			return current, fmt.Errorf("repair bookkeeping: %w", err)
		}
		return current, nil
	}
	if !repair {
		if oldErr != nil {
			return current, invalid("current KDR is invalid; use repair")
		}
		for key := range old.Metadata {
			if _, ok := next.Metadata[key]; !ok {
				return current, invalid("metadata omitted: " + key)
			}
		}
		before, after := sections(old.Body), sections(next.Body)
		changed := false
		for _, heading := range headings[3:] {
			if strings.Join(strings.Fields(before[heading]), "") != strings.Join(strings.Fields(after[heading]), "") {
				changed = true
			}
		}
		if !changed {
			return current, invalid("record update requires meaningful result, verification or resume content")
		}
	}
	if err := setActor(&next, actor); err != nil {
		return current, err
	}
	encoded, err := next.Bytes()
	if err != nil {
		return current, err
	}
	result := Saved{ID: id, Path: current.Path, Hash: Hash(encoded), Raw: encoded}
	action := "Update"
	if repair {
		action = "Repair"
	}
	return s.save(result, next, action)
}
func (s Store) save(result Saved, doc okfmemory.Document, action string) (Saved, error) {
	if err := okfmemory.WriteFile(s.Bundle, "kdr/"+result.ID+".md", result.Raw); err != nil {
		return result, fmt.Errorf("KDR save (inspect current bytes): %w", err)
	}
	if err := okfmemory.Bookkeeping(s.Bundle, doc, action, time.Now()); err != nil {
		return result, fmt.Errorf("KDR saved at %s hash %s; bookkeeping failed: %w", result.Path, result.Hash, err)
	}
	return result, nil
}
func setActor(doc *okfmemory.Document, actor string) error {
	if strings.TrimSpace(actor) == "" {
		return invalid("actor is required")
	}
	doc.Metadata["generated"] = map[string]any{"by": actor, "at": time.Now().UTC().Format(time.RFC3339Nano)}
	return nil
}
func randomID() []byte { raw := make([]byte, 16); _, _ = rand.Read(raw); return raw }

// List returns validated canonical KDR documents from this Space.
func (s Store) List() ([]Saved, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	docs, err := okfmemory.Search(s.Bundle, "", nil)
	if err != nil {
		return nil, err
	}
	out := []Saved{}
	for _, doc := range docs {
		if !strings.HasPrefix(doc.ID, "kdr/") {
			continue
		}
		id := strings.TrimPrefix(doc.ID, "kdr/")
		saved, err := s.Read(id)
		if err != nil {
			return nil, err
		}
		if _, err := Parse(saved.Raw, id); err != nil {
			return nil, err
		}
		out = append(out, saved)
	}
	return out, nil
}

// Resolve requires exactly one title match and never changes a session selection.
func (s Store) Resolve(name string) (string, error) {
	list, err := s.List()
	if err != nil {
		return "", err
	}
	var ids []string
	for _, saved := range list {
		doc, err := Parse(saved.Raw, saved.ID)
		if err != nil {
			return "", err
		}
		if doc.String("title") == name {
			ids = append(ids, saved.ID)
		}
	}
	if len(ids) != 1 {
		return "", invalid(fmt.Sprintf("name %q matched %d KDRs; candidates: %s", name, len(ids), strings.Join(ids, ", ")))
	}
	return ids[0], nil
}

// Lock claims one runtime directory. It never steals or deletes another owner's
// lock. After a crash, inspect stopped processes before removing that lock.
func Lock(project, key string) (func() error, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(key) {
		return nil, invalid("invalid lock key")
	}
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"aidlc", "aidlc/.runtime", "aidlc/.runtime/locks"} {
		info, err := root.Lstat(name)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			root.Close()
			return nil, invalid("runtime path is not a directory")
		}
		if err != nil && !os.IsNotExist(err) {
			root.Close()
			return nil, err
		}
		if os.IsNotExist(err) {
			if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
				root.Close()
				return nil, err
			}
		}
	}
	ignore, err := root.OpenFile("aidlc/.runtime/.gitignore", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, writeErr := ignore.WriteString("*\n")
		closeErr := ignore.Close()
		if writeErr != nil {
			root.Close()
			return nil, writeErr
		}
		if closeErr != nil {
			root.Close()
			return nil, closeErr
		}
	} else if !os.IsExist(err) {
		root.Close()
		return nil, err
	}
	name := "aidlc/.runtime/locks/" + key
	if err := root.Mkdir(name, 0700); err != nil {
		root.Close()
		return nil, fmt.Errorf("lock %s unavailable; inspect active process before recovery: %w", key, err)
	}
	return func() error {
		err := root.Remove(name)
		closeErr := root.Close()
		if err != nil {
			return err
		}
		return closeErr
	}, nil
}

// ValidateRoot checks the Space boundary without reading or mutating its Concepts.
func (s Store) ValidateRoot() error { return s.check() }
