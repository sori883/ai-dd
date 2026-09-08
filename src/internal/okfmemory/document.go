// Package okfmemory reads and maintains Space-local OKF documents.
package okfmemory

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/filestore"
	"github.com/sori883/ai-dd/src/internal/okf"
	"go.yaml.in/yaml/v3"
)

const MaxBytes = 256 * 1024

// Document retains all metadata when a Concept is rewritten.
type Document struct {
	ID       string
	Metadata map[string]any
	Body     string
}

// Parse validates standard OKF metadata while preserving extension fields.
func Parse(raw []byte) (Document, error) {
	if len(raw) > MaxBytes {
		return Document{}, fmt.Errorf("document exceeds 256 KiB: %w", fs.ErrInvalid)
	}
	if _, err := okf.ParseConcept(bytes.NewReader(raw)); err != nil {
		return Document{}, fmt.Errorf("invalid OKF: %v: %w", err, fs.ErrInvalid)
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	end := strings.Index(normalized[4:], "\n---") + 4
	if end < 4 {
		return Document{}, fmt.Errorf("missing frontmatter end: %w", fs.ErrInvalid)
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(normalized[4:end]), &metadata); err != nil {
		return Document{}, err
	}
	body := strings.TrimPrefix(normalized[end+4:], "\n")
	return Document{Metadata: metadata, Body: body}, nil
}

// String returns a string metadata field or an empty string for other types.
func (d Document) String(key string) string { value, _ := d.Metadata[key].(string); return value }

// Bytes serializes and revalidates a document without discarding extension fields.
func (d Document) Bytes() ([]byte, error) {
	metadata, err := yaml.Marshal(d.Metadata)
	if err != nil {
		return nil, err
	}
	raw := append([]byte("---\n"), metadata...)
	raw = append(raw, []byte("---\n"+d.Body)...)
	if _, err := Parse(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ValidID recognizes the canonical generated Intent identity.
func ValidID(id string) bool { return regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) }

// ConceptPath maps a logical Concept ID to a nonreserved relative Markdown path.
func ConceptPath(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, "\\\x00\r\n") || strings.HasSuffix(id, ".md") || path.Clean(id) != id || !fs.ValidPath(id) {
		return "", fmt.Errorf("invalid Concept ID %q: %w", id, fs.ErrInvalid)
	}
	for _, part := range strings.Split(id, "/") {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("invalid Concept ID: %w", fs.ErrInvalid)
		}
	}
	if base := path.Base(id); base == "index" || base == "log" {
		return "", fmt.Errorf("reserved Concept ID: %w", fs.ErrInvalid)
	}
	return id + ".md", nil
}

// Read loads a Concept without following bundle symlinks.
func Read(root, id string) (Document, error) {
	name, err := ConceptPath(id)
	if err != nil {
		return Document{}, err
	}
	raw, err := ReadFile(root, name)
	if err != nil {
		return Document{}, err
	}
	doc, err := Parse(raw)
	doc.ID = id
	return doc, err
}

// Search matches metadata with AND terms and an optional exact Intent ID.
func Search(root, query string, id *string) ([]Document, error) {
	if id != nil && !ValidID(*id) {
		return nil, fmt.Errorf("intent_id must be 32 lowercase hexadecimal digits: %w", fs.ErrInvalid)
	}
	docs, err := documents(root)
	if err != nil {
		return nil, err
	}
	out := []Document{}
	for _, doc := range docs {
		if id != nil && doc.String("intent_id") != *id {
			continue
		}
		haystack := strings.ToLower(doc.String("title") + " " + doc.String("description"))
		if tags, ok := doc.Metadata["tags"].([]any); ok {
			for _, tag := range tags {
				haystack += " " + strings.ToLower(fmt.Sprint(tag))
			}
		}
		matches := true
		for _, term := range strings.Fields(strings.ToLower(query)) {
			if !strings.Contains(haystack, term) {
				matches = false
			}
		}
		if matches {
			out = append(out, doc)
		}
	}
	return out, nil
}
func documents(root string) ([]Document, error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	var docs []Document
	err = fs.WalkDir(directory.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink %s: %w", name, fs.ErrInvalid)
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			return nil
		}
		if path.Base(name) == "index.md" || path.Base(name) == "log.md" {
			return nil
		}
		doc, err := Read(root, strings.TrimSuffix(name, ".md"))
		if err != nil {
			return err
		}
		docs = append(docs, doc)
		return nil
	})
	return docs, err
}

// Validate checks document metadata and reserved Bundle files without resolving content links.
func Validate(root string) error {
	if _, err := documents(root); err != nil {
		return err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	return fs.WalkDir(directory.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		base := path.Base(name)
		if base != "index.md" && base != "log.md" {
			return nil
		}
		raw, err := ReadFile(root, name)
		if err != nil {
			return err
		}
		if !utf8.Valid(raw) {
			return fmt.Errorf("invalid UTF-8 %s: %w", name, fs.ErrInvalid)
		}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if strings.HasPrefix(text, "---\n") {
			if name != "index.md" {
				return fmt.Errorf("reserved file frontmatter %s: %w", name, fs.ErrInvalid)
			}
			end := strings.Index(text[4:], "\n---") + 4
			if end < 4 {
				return fmt.Errorf("invalid root index: %w", fs.ErrInvalid)
			}
			var fields map[string]any
			if yaml.Unmarshal([]byte(text[4:end]), &fields) != nil || fields["okf_version"] != "0.2" {
				return fmt.Errorf("unsupported root index version: %w", fs.ErrInvalid)
			}
		}
		if base == "log.md" {
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "#") {
					if !strings.HasPrefix(line, "## ") {
						return fmt.Errorf("invalid log heading: %w", fs.ErrInvalid)
					}
					if _, err := time.Parse("2006-01-02", strings.TrimPrefix(line, "## ")); err != nil {
						return fmt.Errorf("invalid log date: %w", fs.ErrInvalid)
					}
				}
			}
		}
		return nil
	})
}

// ReadFile loads a bounded regular file within the supplied root.
func ReadFile(root, name string) ([]byte, error) { return filestore.ReadFile(root, name) }

// WriteFile atomically replaces one file; the caller holds its bundle lock.
func WriteFile(root, name string, raw []byte) error { return filestore.WriteFile(root, name, raw) }

// Bookkeeping updates the immediate parent index and then the bundle change log.
func Bookkeeping(root string, doc Document, action string, now time.Time) error {
	name, err := ConceptPath(doc.ID)
	if err != nil {
		return err
	}
	indexPath := path.Join(path.Dir(name), "index.md")
	index, err := ReadFile(root, indexPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(index) == 0 {
		index = []byte("# Index\n\n")
	}
	title := strings.NewReplacer("[", "", "]", "", "\n", " ", "\r", " ").Replace(doc.String("title"))
	if title == "" {
		title = doc.ID
	}
	link := "(" + path.Base(name) + ")"
	line := "- [" + title + "]" + link + ": " + strings.ReplaceAll(doc.String("description"), "\n", " ")
	lines := strings.Split(strings.TrimRight(string(index), "\n"), "\n")
	active := map[string]bool{}
	for _, line := range BookkeepingLines(string(index)) {
		active[line] = true
	}
	found := false
	for i, current := range lines {
		if active[current] && strings.HasPrefix(current, "- [") && strings.Contains(current, link) {
			lines[i] = line
			found = true
		}
	}
	if !found {
		at := 0
		if len(lines) > 0 && lines[0] == "---" {
			for i := 1; i < len(lines); i++ {
				if lines[i] == "---" {
					at = i + 1
					break
				}
			}
		}
		lines = append(lines, "")
		copy(lines[at+1:], lines[at:])
		lines[at] = line
	}
	if err := WriteFile(root, indexPath, []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return fmt.Errorf("parent index %s: %w", indexPath, err)
	}
	log, err := ReadFile(root, "log.md")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	entry := fmt.Sprintf("## %s\n- %s: `%s`.\n\n", now.UTC().Format("2006-01-02"), action, doc.ID)
	if err := WriteFile(root, "log.md", append([]byte(entry), log...)); err != nil {
		return fmt.Errorf("log.md: %w", err)
	}
	return nil
}

// BookkeepingLines excludes Markdown comments and fenced examples from active entries.
func BookkeepingLines(raw string) []string {
	raw = regexp.MustCompile(`(?s)<!--(?:.*?-->|.*$)`).ReplaceAllString(raw, "")
	var lines []string
	fence := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if fence == "" {
				fence = trimmed[:3]
			} else if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if fence == "" {
			lines = append(lines, line)
		}
	}
	return lines
}
