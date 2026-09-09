package okfmemory

import (
	"fmt"
	"github.com/sori883/ai-dd/src/internal/filestore"
	"go.yaml.in/yaml/v3"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

type DocumentMatch struct {
	Type        string    `json:"type" yaml:"type"`
	IntentID    *string   `json:"intent_id,omitempty" yaml:"intent_id,omitempty"`
	Title       *string   `json:"title,omitempty" yaml:"title,omitempty"`
	Description *string   `json:"description,omitempty" yaml:"description,omitempty"`
	Status      *string   `json:"status,omitempty" yaml:"status,omitempty"`
	Tags        *[]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}
type SelectedDocument struct {
	Path     string   `json:"path"`
	Hash     string   `json:"hash"`
	Document Document `json:"-"`
	Raw      []byte   `json:"-"`
}

// Validate checks the strict selector vocabulary; the Intent placeholder is resolved by workflow callers.
func (m DocumentMatch) Validate() error {
	if strings.TrimSpace(m.Type) == "" || !utf8.ValidString(m.Type) {
		return fmt.Errorf("type required: %w", fs.ErrInvalid)
	}
	for _, value := range []*string{m.IntentID, m.Title, m.Description, m.Status} {
		if value != nil && (strings.TrimSpace(*value) == "" || !utf8.ValidString(*value)) {
			return fmt.Errorf("empty or invalid metadata condition: %w", fs.ErrInvalid)
		}
	}
	if m.IntentID != nil && !ValidID(*m.IntentID) && *m.IntentID != "${intent_id}" {
		return fmt.Errorf("invalid intent_id: %w", fs.ErrInvalid)
	}
	if m.Status != nil && *m.Status != "draft" && *m.Status != "stable" && *m.Status != "deprecated" {
		return fmt.Errorf("invalid status: %w", fs.ErrInvalid)
	}
	if m.Tags != nil {
		for _, tag := range *m.Tags {
			if strings.TrimSpace(tag) == "" || !utf8.ValidString(tag) {
				return fmt.Errorf("invalid tags: %w", fs.ErrInvalid)
			}
		}
	}
	return nil
}

// Matches compares metadata exactly; tag order and duplicate set elements do not matter.
func (m DocumentMatch) Matches(d Document) bool {
	if d.String("type") != m.Type {
		return false
	}
	for key, value := range map[string]*string{"intent_id": m.IntentID, "title": m.Title, "description": m.Description, "status": m.Status} {
		if value != nil && d.String(key) != *value {
			return false
		}
	}
	if m.Tags != nil {
		expected := map[string]bool{}
		for _, tag := range *m.Tags {
			expected[tag] = true
		}
		actual := map[string]bool{}
		switch tags := d.Metadata["tags"].(type) {
		case []any:
			for _, value := range tags {
				tag, ok := value.(string)
				if !ok {
					return false
				}
				actual[tag] = true
			}
		case []string:
			for _, tag := range tags {
				actual[tag] = true
			}
		case nil:
		default:
			return false
		}
		if len(actual) != len(expected) {
			return false
		}
		for tag := range expected {
			if !actual[tag] {
				return false
			}
		}
	}
	return true
}
func SelectDocuments(root string, match DocumentMatch, count string) ([]SelectedDocument, error) {
	if err := match.Validate(); err != nil {
		return nil, err
	}
	if match.IntentID != nil && !ValidID(*match.IntentID) {
		return nil, fmt.Errorf("unresolved intent_id: %w", fs.ErrInvalid)
	}
	if count != "one" && count != "optional" && count != "many" {
		return nil, fmt.Errorf("invalid selector count: %w", fs.ErrInvalid)
	}
	documents, err := ScanDocuments(root)
	if err != nil {
		return nil, err
	}
	out := []SelectedDocument{}
	for _, doc := range documents {
		if match.Matches(doc.Document) {
			out = append(out, doc)
		}
	}
	if count == "one" && len(out) != 1 || count == "optional" && len(out) > 1 || count == "many" && len(out) == 0 {
		return out, fmt.Errorf("selector count %s resolved %d: %w", count, len(out), fs.ErrInvalid)
	}
	return out, nil
}

// ScanDocuments returns one safely read snapshot of the bundle's documents.
func ScanDocuments(root string) ([]SelectedDocument, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("invalid bundle root: %w", fs.ErrInvalid)
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	out := []SelectedDocument{}
	err = fs.WalkDir(directory.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink document %s: %w", name, fs.ErrInvalid)
		}
		if d.IsDir() || !strings.HasSuffix(name, ".md") || path.Base(name) == "index.md" || path.Base(name) == "log.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nonregular document %s: %w", name, fs.ErrInvalid)
		}
		raw, err := ReadFile(root, name)
		if err != nil {
			return err
		}
		doc, err := Parse(raw)
		if err != nil {
			return err
		}
		doc.ID = strings.TrimSuffix(name, ".md")
		out = append(out, SelectedDocument{Path: name, Hash: filestore.Hash(raw), Document: doc, Raw: raw})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b SelectedDocument) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// UnmarshalYAML rejects coercion of numbers, booleans and unknown selector fields.
func (m *DocumentMatch) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("metadata must be a mapping: %w", fs.ErrInvalid)
	}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Tag != "!!str" {
			return fmt.Errorf("metadata key must be a string: %w", fs.ErrInvalid)
		}
		switch key.Value {
		case "type", "title", "description", "intent_id", "status":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("metadata %s must be a string: %w", key.Value, fs.ErrInvalid)
			}
		case "tags":
			if value.Kind != yaml.SequenceNode {
				return fmt.Errorf("tags must be a string array: %w", fs.ErrInvalid)
			}
			for _, tag := range value.Content {
				if tag.Kind != yaml.ScalarNode || tag.Tag != "!!str" {
					return fmt.Errorf("tag must be a string: %w", fs.ErrInvalid)
				}
			}
		default:
			return fmt.Errorf("unknown metadata condition %s: %w", key.Value, fs.ErrInvalid)
		}
	}
	type wire DocumentMatch
	var value wire
	if err := node.Decode(&value); err != nil {
		return err
	}
	*m = DocumentMatch(value)
	return nil
}
