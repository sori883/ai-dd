// Package okf reads and searches caller-rooted Open Knowledge Format metadata.
package okf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const MaxFrontmatterBytes = 64 * 1024

// Concept contains validated metadata, never Markdown body content.
type Concept struct {
	ID, Path, Type, Title, Description, Resource, Status, TrustTier string
	Tags                                                            []string
	GeneratedAt, StaleAfter                                         *time.Time
}

// ParseConcept validates a complete UTF-8 Concept while retaining only metadata.
func ParseConcept(reader io.Reader) (Concept, error) {
	br := bufio.NewReader(reader)
	data := []byte{}
	for len(data) <= MaxFrontmatterBytes {
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && bytes.HasSuffix(data, []byte("\n---")) {
				return decodeFrontmatter(data, len(data)-3)
			}
			return Concept{}, fmt.Errorf("unclosed frontmatter: %w", err)
		}
		data = append(data, b)
		if b != '\n' {
			continue
		}
		end := len(data)
		start := bytes.LastIndexByte(data[:end-1], '\n') + 1
		line := bytes.TrimSuffix(data[start:end-1], []byte("\r"))
		if start == 0 {
			if !bytes.Equal(line, []byte("---")) {
				return Concept{}, errors.New("missing opening frontmatter delimiter")
			}
			continue
		}
		if !bytes.Equal(line, []byte("---")) {
			continue
		}
		if end > MaxFrontmatterBytes {
			break
		}
		if !utf8.Valid(data) {
			return Concept{}, errors.New("invalid utf-8")
		}
		if err := validateBody(br); err != nil {
			return Concept{}, err
		}
		return decodeFrontmatter(data, start)
	}
	return Concept{}, errors.New("frontmatter exceeds 65536 bytes")
}

func decodeFrontmatter(data []byte, closingStart int) (Concept, error) {
	if len(data) > MaxFrontmatterBytes {
		return Concept{}, errors.New("frontmatter exceeds 65536 bytes")
	}
	if !utf8.Valid(data) {
		return Concept{}, errors.New("invalid utf-8")
	}
	openingEnd := bytes.IndexByte(data, '\n') + 1
	if openingEnd == 0 || !bytes.Equal(bytes.TrimSuffix(data[:openingEnd-1], []byte("\r")), []byte("---")) {
		return Concept{}, errors.New("missing opening frontmatter delimiter")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data[openingEnd:closingStart]))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		return Concept{}, fmt.Errorf("invalid yaml: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Concept{}, errors.New("frontmatter must contain exactly one yaml document")
	}
	if len(node.Content) != 1 {
		return Concept{}, errors.New("frontmatter must be a mapping")
	}
	return parseMetadata(node.Content[0])
}

func validateBody(br *bufio.Reader) error {
	for {
		r, size, err := br.ReadRune()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		if r == utf8.RuneError && size == 1 {
			return errors.New("invalid utf-8")
		}
	}
}

func resolveNode(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

func mapping(n *yaml.Node) (map[string]yaml.Node, error) {
	n = resolveNode(n)
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("expected mapping")
	}
	out := map[string]yaml.Node{}
	if err := n.Decode(&out); err != nil {
		return nil, fmt.Errorf("invalid mapping: %w", err)
	}
	return out, nil
}

func stringField(fields map[string]yaml.Node, key string, required bool) (string, error) {
	n, ok := fields[key]
	if !ok {
		if required {
			return "", fmt.Errorf("missing %s", key)
		}
		return "", nil
	}
	v := resolveNode(&n)
	if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
		return "", fmt.Errorf("%s must be a string", key)
	}
	if required && v.Value == "" {
		return "", fmt.Errorf("%s must be nonempty", key)
	}
	return v.Value, nil
}

func timestampField(fields map[string]yaml.Node, key string, required bool) (*time.Time, error) {
	n, ok := fields[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("missing %s", key)
		}
		return nil, nil
	}
	v := resolveNode(&n)
	if v.Kind != yaml.ScalarNode || (v.Tag != "!!str" && v.Tag != "!!timestamp") {
		return nil, fmt.Errorf("%s must be a timestamp", key)
	}
	t, err := time.Parse(time.RFC3339Nano, v.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s timestamp: %w", key, err)
	}
	t = t.UTC()
	return &t, nil
}

func parseMetadata(node *yaml.Node) (Concept, error) {
	fields, err := mapping(node)
	if err != nil {
		return Concept{}, err
	}
	c := Concept{Tags: []string{}, Status: "stable", TrustTier: "unverified"}
	for _, f := range []struct {
		key      string
		target   *string
		required bool
	}{
		{key: "type", target: &c.Type, required: true}, {key: "title", target: &c.Title},
		{key: "description", target: &c.Description}, {key: "resource", target: &c.Resource},
	} {
		value, err := stringField(fields, f.key, f.required)
		if err != nil {
			return Concept{}, err
		}
		*f.target = value
	}
	if n, ok := fields["tags"]; ok {
		v := resolveNode(&n)
		if v.Kind != yaml.SequenceNode {
			return Concept{}, errors.New("tags must be a string list")
		}
		for _, tag := range v.Content {
			s, err := stringField(map[string]yaml.Node{"tag": *tag}, "tag", false)
			if err != nil {
				return Concept{}, err
			}
			c.Tags = append(c.Tags, s)
		}
	}
	if _, ok := fields["status"]; ok {
		c.Status, err = stringField(fields, "status", false)
		if err != nil {
			return Concept{}, err
		}
		switch c.Status {
		case "draft", "stable", "deprecated":
		default:
			return Concept{}, errors.New("invalid status")
		}
	}
	c.StaleAfter, err = timestampField(fields, "stale_after", false)
	if err != nil {
		return Concept{}, err
	}
	if n, ok := fields["generated"]; ok {
		m, err := mapping(&n)
		if err != nil {
			return Concept{}, fmt.Errorf("generated: %w", err)
		}
		if _, err := stringField(m, "by", true); err != nil {
			return Concept{}, fmt.Errorf("generated: %w", err)
		}
		c.GeneratedAt, err = timestampField(m, "at", false)
		if err != nil {
			return Concept{}, err
		}
	}
	if n, ok := fields["verified"]; ok {
		v := resolveNode(&n)
		entries := v.Content
		if v.Kind == yaml.MappingNode {
			entries = []*yaml.Node{v}
		} else if v.Kind != yaml.SequenceNode {
			return Concept{}, errors.New("verified must be a mapping or list")
		}
		for _, entry := range entries {
			m, err := mapping(entry)
			if err != nil {
				return Concept{}, fmt.Errorf("verified: %w", err)
			}
			by, err := stringField(m, "by", true)
			if err != nil {
				return Concept{}, err
			}
			if _, err := timestampField(m, "at", true); err != nil {
				return Concept{}, err
			}
			if c.TrustTier == "unverified" {
				c.TrustTier = "machine-confirmed"
			}
			if strings.HasPrefix(by, "human:") && len(by) > len("human:") {
				c.TrustTier = "human-reviewed"
			}
		}
	}
	if err := validateUsageWindow(fields); err != nil {
		return Concept{}, err
	}
	if n, ok := fields["sources"]; ok {
		if err := validateSources(&n); err != nil {
			return Concept{}, err
		}
	}
	return c, nil
}

func validateSources(node *yaml.Node) error {
	node = resolveNode(node)
	if node.Kind != yaml.SequenceNode {
		return errors.New("sources must be a list")
	}
	for _, entry := range node.Content {
		m, err := mapping(entry)
		if err != nil {
			return fmt.Errorf("sources: %w", err)
		}
		for _, key := range []string{"resource", "id", "title", "author"} {
			if _, err := stringField(m, key, key == "resource"); err != nil {
				return err
			}
		}
		if _, err := timestampField(m, "last_modified", false); err != nil {
			return err
		}
		if err := validateUsageWindow(m); err != nil {
			return fmt.Errorf("sources: %w", err)
		}
		if n, ok := m["usage_count"]; ok {
			v := resolveNode(&n)
			if v.Kind != yaml.ScalarNode || v.Tag != "!!int" {
				return errors.New("usage_count must be an integer")
			}
		}
	}
	return nil
}

func validateUsageWindow(fields map[string]yaml.Node) error {
	node, exists := fields["usage_window"]
	if !exists {
		return nil
	}
	window, err := mapping(&node)
	if err != nil {
		return fmt.Errorf("usage_window: %w", err)
	}
	for _, key := range []string{"from", "to"} {
		if _, err := timestampField(window, key, true); err != nil {
			return fmt.Errorf("usage_window: %w", err)
		}
	}
	return nil
}
