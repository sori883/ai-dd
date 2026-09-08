package okfmemory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"io"
	"io/fs"
	"strings"
	"time"
	"unicode/utf8"
)

// MetadataInput distinguishes omitted metadata from explicit replacements.
type MetadataInput struct {
	Type, Title, Description, Status, Resource, StaleAfter *string
	IntentID, SourcesJSON, VerifiedJSON, ExtraJSON         *string
	Tags                                                   []string
	ClearTags                                              bool
	Actor                                                  string
}

// BuildMetadata builds or patches one body-only Concept.
func BuildMetadata(old *Document, body []byte, input MetadataInput, now time.Time) (Document, error) {
	fail := func(message string) (Document, error) {
		return Document{}, fmt.Errorf("%s: %w", message, fs.ErrInvalid)
	}
	if len(body) > MaxBytes || !utf8.Valid(body) {
		return fail("body must be UTF-8 within 256 KiB")
	}
	prefix := strings.TrimPrefix(string(body), "\ufeff")
	if prefix == "---" || strings.HasPrefix(prefix, "---\n") || strings.HasPrefix(prefix, "---\r\n") {
		return fail("--body-file must not contain leading frontmatter")
	}
	if strings.TrimSpace(input.Actor) == "" {
		return fail("actor required")
	}
	metadata := map[string]any{}
	if old != nil {
		raw, err := yaml.Marshal(old.Metadata)
		if err != nil {
			return Document{}, err
		}
		if err := yaml.Unmarshal(raw, &metadata); err != nil {
			return Document{}, err
		}
	} else {
		metadata["status"] = "stable"
		metadata["tags"] = []string{}
	}
	for key, value := range map[string]*string{"type": input.Type, "title": input.Title, "description": input.Description, "status": input.Status, "resource": input.Resource, "stale_after": input.StaleAfter, "intent_id": input.IntentID} {
		if value != nil {
			metadata[key] = *value
		}
	}
	for key, provided := range map[string]*string{"type": input.Type, "title": input.Title, "description": input.Description} {
		if old != nil && provided == nil {
			continue
		}
		value, ok := metadata[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fail(key + " required")
		}
	}
	if input.IntentID != nil && !ValidID(*input.IntentID) {
		return fail("invalid intent_id")
	}
	if input.ClearTags && input.Tags != nil {
		return fail("tag and clear-tags conflict")
	}
	if input.ClearTags {
		metadata["tags"] = []string{}
	} else if input.Tags != nil {
		metadata["tags"] = append([]string{}, input.Tags...)
	}
	for key, raw := range map[string]*string{"sources": input.SourcesJSON, "verified": input.VerifiedJSON, "extra": input.ExtraJSON} {
		if raw == nil {
			continue
		}
		value, err := metadataJSON(*raw)
		if err != nil {
			return fail(key + ": " + err.Error())
		}
		if key != "extra" {
			metadata[key] = value
			continue
		}
		fields, ok := value.(map[string]any)
		if !ok {
			return fail("metadata-json must be an object")
		}
		for name, value := range fields {
			switch name {
			case "type", "title", "description", "status", "resource", "stale_after", "intent_id", "tags", "sources", "generated", "verified", "okf_version":
				return fail("metadata-json reserved key: " + name)
			}
			metadata[name] = value
		}
	}
	if old != nil && old.Body == string(body) {
		previous := make(map[string]any, len(old.Metadata))
		candidate := make(map[string]any, len(metadata))
		for k, v := range old.Metadata {
			if k != "generated" {
				previous[k] = v
			}
		}
		for k, v := range metadata {
			if k != "generated" {
				candidate[k] = v
			}
		}
		a, err := yaml.Marshal(previous)
		if err != nil {
			return Document{}, err
		}
		b, err := yaml.Marshal(candidate)
		if err != nil {
			return Document{}, err
		}
		if bytes.Equal(a, b) {
			return fail("Concept body and metadata unchanged")
		}
	}
	metadata["generated"] = map[string]any{"by": input.Actor, "at": now.UTC().Format(time.RFC3339Nano)}
	doc := Document{Metadata: metadata, Body: string(body)}
	raw, err := doc.Bytes()
	if err != nil {
		return Document{}, err
	}
	result, err := Parse(raw)
	if old != nil {
		result.ID = old.ID
	}
	return result, err
}

// metadataJSON rejects duplicate keys at every depth and preserves numeric spelling.
func metadataJSON(raw string) (any, error) {
	if len(raw) > MaxBytes || !utf8.ValidString(raw) {
		return nil, fmt.Errorf("JSON must be UTF-8 within 256 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 64 {
			return nil, fmt.Errorf("JSON nesting exceeds 64")
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				result := map[string]any{}
				for decoder.More() {
					key, err := decoder.Token()
					if err != nil {
						return nil, err
					}
					name, ok := key.(string)
					if !ok {
						return nil, fmt.Errorf("object key must be string")
					}
					if _, exists := result[name]; exists {
						return nil, fmt.Errorf("duplicate JSON key %q", name)
					}
					v, err := read(depth + 1)
					if err != nil {
						return nil, err
					}
					result[name] = v
				}
				if _, err := decoder.Token(); err != nil {
					return nil, err
				}
				return result, nil
			case '[':
				result := []any{}
				for decoder.More() {
					v, err := read(depth + 1)
					if err != nil {
						return nil, err
					}
					result = append(result, v)
				}
				if _, err := decoder.Token(); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, fmt.Errorf("unexpected delimiter")
		case json.Number:
			tag := "!!int"
			if strings.ContainsAny(string(value), ".eE") {
				tag = "!!float"
			}
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: string(value)}, nil
		default:
			return token, nil
		}
	}
	value, err := read(0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return value, nil
}
