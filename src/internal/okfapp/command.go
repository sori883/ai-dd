package okfapp

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okfcli"
	"github.com/sori883/ai-dd/src/internal/okfmemory"
)

func encode(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	return append(raw, '\n'), err
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
func (s Service) Execute(r okfcli.CommandRequest) ([]byte, error) {
	store := s.store(r.Space)
	switch r.Action {
	case "rules":
		text, hash, err := s.Rules(r.Space)
		if err != nil {
			return nil, err
		}
		return []byte("Rules SHA-256: " + hash + "\n" + text), nil
	case "search":
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
	case "show":
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
		return encode(map[string]string{"concept_id": doc.ID, "content": string(raw), "hash": filestore.Hash(raw)})
	case "check":
		if err := store.ValidateRoot(); err != nil {
			return nil, err
		}
		if err := okfmemory.Validate(store.Bundle); err != nil {
			return nil, err
		}
		return []byte("OKF validation passed\n"), nil
	case "create", "update":
		return s.memoryWrite(r)
	}
	return nil, invalid("unsupported operation")
}
func (s Service) memoryWrite(r okfcli.CommandRequest) ([]byte, error) {
	store := s.store(r.Space)
	if err := store.ValidateRoot(); err != nil {
		return nil, err
	}
	name, err := okfmemory.ConceptPath(r.Target)
	if err != nil {
		return nil, err
	}
	raw, err := s.readDraft(r.BodyFile)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.Actor) == "" {
		return nil, invalid("actor required")
	}
	release, err := filestore.Lock(s.Root, "bundle-"+r.Space)
	if err != nil {
		return nil, err
	}
	defer release()
	var previous *okfmemory.Document
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
		if filestore.Hash(current) != r.Expect {
			return nil, invalid("Concept hash conflict")
		}
		old, err := okfmemory.Parse(current)
		if err != nil {
			return nil, err
		}
		previous = &old
	}
	metadata := r.Metadata
	metadata.Actor = r.Actor
	now := time.Now().UTC()
	doc, err := okfmemory.BuildMetadata(previous, raw, metadata, now)
	if err != nil {
		return nil, err
	}
	doc.ID = r.Target

	encoded, err := doc.Bytes()
	if err != nil {
		return nil, err
	}
	if err := okfmemory.WriteFile(store.Bundle, name, encoded); err != nil {
		return nil, err
	}
	out, _ := encode(map[string]string{"concept_id": doc.ID, "hash": filestore.Hash(encoded), "path": name})
	if err := okfmemory.Bookkeeping(store.Bundle, doc, strings.Title(r.Action), now); err != nil {
		return out, fmt.Errorf("Concept saved; bookkeeping failed: %w", err)
	}
	return out, nil
}

type Service struct{ Root string }

func invalid(message string) error { return fmt.Errorf("%s: %w", message, fs.ErrInvalid) }
