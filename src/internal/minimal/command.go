package minimal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/cli"
	"github.com/sori883/ai-dd/src/internal/install"
	"github.com/sori883/ai-dd/src/internal/kdr"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func encode(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	return append(raw, '\n'), err
}
func encodeSaved(saved kdr.Saved) ([]byte, error) {
	return encode(struct {
		kdr.Saved
		Content string `json:"content"`
	}{saved, string(saved.Raw)})
}
func (s Service) readDraft(file string) ([]byte, error) {
	if filepath.IsAbs(file) {
		relative, err := filepath.Rel(s.Root, file)
		if err != nil {
			return nil, err
		}
		file = relative
	}
	return okfmemory.ReadFile(s.Root, filepath.ToSlash(file))
}

// Execute runs one validated public operation; partial saves return output together with an error.
func (s Service) Execute(r cli.MinimalRequest) ([]byte, error) {
	store := s.store(r.Space)
	switch r.Command + "/" + r.Action {
	case "install/codex":
		result, err := install.Codex(s.Root, s.Binary)
		out, _ := encode(result)
		return out, err
	case "kdr/template":
		return okfmemory.ReadFile(s.Root, "aidlc/templates/kdr.md")
	case "kdr/create", "intent/create":
		raw, err := s.readDraft(r.File)
		if err != nil {
			return nil, err
		}
		if r.Command == "intent" {
			doc, err := kdr.Parse(raw, "")
			if err != nil {
				return nil, err
			}
			if doc.String("title") != r.Target {
				return nil, invalid("Intent name must match draft title")
			}
		}
		saved, err := store.Create(raw, r.Actor)
		out, _ := encodeSaved(saved)
		return out, err
	case "kdr/list", "intent/list":
		list, err := store.List()
		if err != nil {
			return nil, err
		}
		rows := []map[string]string{}
		for _, saved := range list {
			doc, err := kdr.Parse(saved.Raw, saved.ID)
			if err != nil {
				return nil, err
			}
			rows = append(rows, map[string]string{"id": saved.ID, "title": doc.String("title"), "path": saved.Path})
		}
		return encode(rows)
	case "kdr/show", "kdr/check":
		saved, err := store.Read(r.Target)
		if err != nil {
			return nil, err
		}
		if !r.Raw {
			if _, err := kdr.Parse(saved.Raw, r.Target); err != nil {
				return nil, err
			}
		}
		if r.Action == "check" {
			if err := s.checkBookkeeping(store, r.Target); err != nil {
				return nil, err
			}
		}
		return encodeSaved(saved)
	case "session/inspect":
		state, err := s.Inspect(r.Session)
		if err != nil {
			return nil, err
		}
		return encode(state)
	case "session/bind", "intent/switch":
		id := r.Target
		if r.Command == "intent" {
			if r.IntentID != nil {
				id = *r.IntentID
			} else {
				var err error
				id, err = store.Resolve(r.Target)
				if err != nil {
					return nil, err
				}
			}
		}
		return s.withSession(r.Session, func(state *Session) ([]byte, error) {
			if state.Tool != "" && !r.Recover {
				return nil, invalid("tool is running; poll or explicitly recover after checking the process")
			}
			if state.Intent != "" && (state.Intent != id || state.Space != r.Space) && (state.Dirty || r.Recover) {
				return nil, invalid("record the current Intent before switching")
			}
			saved, err := store.Read(id)
			if err != nil {
				return nil, err
			}
			if _, err := kdr.Parse(saved.Raw, id); err != nil {
				return nil, err
			}
			if err := s.checkBookkeeping(store, id); err != nil {
				return nil, err
			}
			rules, hash, err := s.rules(r.Space)
			if err != nil {
				return nil, err
			}
			if state.Intent != id || state.Space != r.Space {
				state.Dirty = true
			}
			if r.Recover {
				state.Tool = ""
				state.Dirty = true
			}
			state.Intent = id
			state.Space = r.Space
			state.RuleHash = hash
			state.RuleTurn = state.Turn
			if err := s.save(r.Session, *state); err != nil {
				return nil, err
			}
			return encode(map[string]any{"id": id, "hash": saved.Hash, "kdr": string(saved.Raw), "rules": rules, "rules_hash": hash, "draft": s.draftPath(r.Session)})
		})
	case "kdr/update", "kdr/repair":
		return s.withSession(r.Session, func(state *Session) ([]byte, error) {
			if state.Tool != "" {
				return nil, invalid("tool is still running")
			}
			if r.Action == "update" && (state.Intent != r.Target || state.Space != r.Space) {
				return nil, invalid("update must match the selected Intent")
			}
			if r.Action == "update" {
				_, hash, err := s.rules(r.Space)
				if err != nil {
					return nil, err
				}
				if state.RuleHash == "" || state.RuleHash != hash || state.RuleTurn != state.Turn {
					return nil, invalid("reread required Rules before recording")
				}
			}
			raw, err := s.readDraft(r.File)
			if err != nil {
				return nil, err
			}
			var saved kdr.Saved
			if r.Action == "repair" {
				saved, err = store.Repair(r.Target, raw, r.Expect, r.Actor)
			} else {
				saved, err = store.Update(r.Target, raw, r.Expect, r.Actor)
			}
			out, _ := encodeSaved(saved)
			if err != nil {
				return out, err
			}
			if r.Action == "update" {
				state.Dirty = false
				if err := s.save(r.Session, *state); err != nil {
					return out, fmt.Errorf("KDR saved; session unrecorded flag update failed: %w", err)
				}
			}
			return out, nil
		})
	case "memory/rules":
		text, hash, err := s.rules(r.Space)
		if err != nil {
			return nil, err
		}
		return []byte("Rules SHA-256: " + hash + "\n" + text), nil
	case "memory/search":
		if err := store.ValidateRoot(); err != nil {
			return nil, err
		}
		docs, err := okfmemory.Search(store.Bundle, r.Target, r.IntentID)
		if err != nil {
			return nil, err
		}
		rows := []map[string]string{}
		for _, doc := range docs {
			rows = append(rows, map[string]string{"concept_id": doc.ID, "intent_id": doc.String("intent_id"), "title": doc.String("title"), "description": doc.String("description"), "path": doc.ID + ".md"})
		}
		return encode(rows)
	case "memory/show":
		if err := store.ValidateRoot(); err != nil {
			return nil, err
		}
		doc, err := okfmemory.Read(store.Bundle, r.Target)
		if err != nil {
			return nil, err
		}
		raw, err := okfmemory.ReadFile(store.Bundle, doc.ID+".md")
		if err != nil {
			return nil, err
		}
		return encode(map[string]string{"concept_id": doc.ID, "content": string(raw), "hash": kdr.Hash(raw)})
	case "memory/check":
		if err := store.ValidateRoot(); err != nil {
			return nil, err
		}
		if err := okfmemory.Validate(store.Bundle); err != nil {
			return nil, err
		}
		return []byte("OKF validation passed\n"), nil
	case "memory/create", "memory/update":
		return s.memoryWrite(r)
	}
	return nil, invalid("unsupported minimal operation")
}
func (s Service) checkBookkeeping(store kdr.Store, id string) error {
	index, err := okfmemory.ReadFile(store.Bundle, "kdr/index.md")
	if err != nil {
		return err
	}
	log, err := okfmemory.ReadFile(store.Bundle, "log.md")
	if err != nil {
		return err
	}
	if !strings.Contains(string(index), "("+id+".md)") || !strings.Contains(string(log), "`kdr/"+id+"`") {
		return invalid("KDR index/log missing; use repair")
	}
	return nil
}
func (s Service) memoryWrite(r cli.MinimalRequest) ([]byte, error) {
	if r.Target == "kdr" || strings.HasPrefix(r.Target, "kdr/") {
		return nil, invalid("use KDR operations for kdr Concepts")
	}
	store := s.store(r.Space)
	if err := store.ValidateRoot(); err != nil {
		return nil, err
	}
	name, err := okfmemory.ConceptPath(r.Target)
	if err != nil {
		return nil, err
	}
	raw, err := s.readDraft(r.File)
	if err != nil {
		return nil, err
	}
	doc, err := okfmemory.Parse(raw)
	if err != nil {
		return nil, err
	}
	if doc.String("type") == "KDR" {
		return nil, invalid("use KDR operations for KDR type")
	}
	doc.ID = r.Target
	if strings.TrimSpace(r.Actor) == "" {
		return nil, invalid("actor required")
	}
	release, err := kdr.Lock(s.Root, "bundle-"+r.Space)
	if err != nil {
		return nil, err
	}
	defer release()
	current, readErr := okfmemory.ReadFile(store.Bundle, name)
	if r.Action == "create" {
		if readErr == nil {
			return nil, invalid("Concept exists")
		}
		if !os.IsNotExist(readErr) {
			return nil, readErr
		}
	} else {
		if readErr != nil {
			return nil, readErr
		}
		if kdr.Hash(current) != r.Expect {
			return nil, invalid("Concept hash conflict")
		}
		old, err := okfmemory.Parse(current)
		if err != nil {
			return nil, err
		}
		for key := range old.Metadata {
			if _, ok := doc.Metadata[key]; !ok {
				return nil, invalid("metadata omitted: " + key)
			}
		}
		if strings.TrimSpace(old.Body) == strings.TrimSpace(doc.Body) {
			return nil, invalid("Concept body unchanged")
		}
	}
	doc.Metadata["generated"] = map[string]any{"by": r.Actor, "at": time.Now().UTC().Format(time.RFC3339Nano)}
	encoded, err := doc.Bytes()
	if err != nil {
		return nil, err
	}
	if err := okfmemory.WriteFile(store.Bundle, name, encoded); err != nil {
		return nil, err
	}
	out, _ := encode(map[string]string{"concept_id": doc.ID, "hash": kdr.Hash(encoded), "path": name})
	if err := okfmemory.Bookkeeping(store.Bundle, doc, strings.Title(r.Action), time.Now()); err != nil {
		return out, fmt.Errorf("Concept saved; bookkeeping failed: %w", err)
	}
	return out, nil
}
