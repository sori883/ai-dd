// Package harness projects shared source assets into host-specific distributions.
package harness

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Mapping projects one file or a complete source directory into a destination.
type Mapping struct {
	Files               fs.FS
	Source, Destination string
	Tree                bool
}

// Asset is a completed file with a slash-separated relative deployment path.
type Asset struct {
	Path string
	Data []byte
}

// Manifest combines source mappings and already-generated host assets.
type Manifest struct {
	Mappings  []Mapping
	Generated []Asset
}

// Render returns the distribution in path order without writing files.
func (m Manifest) Render(binary string) ([]Asset, error) {
	assets := make(map[string][]byte)
	add := func(asset Asset) error {
		if !validPath(asset.Path) {
			return fmt.Errorf("invalid destination %q: %w", asset.Path, fs.ErrInvalid)
		}
		if _, exists := assets[asset.Path]; exists {
			return fmt.Errorf("duplicate destination %q: %w", asset.Path, fs.ErrInvalid)
		}
		assets[asset.Path] = asset.Data
		return nil
	}
	for _, mapping := range m.Mappings {
		if err := mapping.render(binary, add); err != nil {
			return nil, err
		}
	}
	for _, asset := range m.Generated {
		if err := add(asset); err != nil {
			return nil, err
		}
	}
	result := make([]Asset, 0, len(assets))
	for name, data := range assets {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, exists := assets[parent]; exists {
				return nil, fmt.Errorf("destination %q is also a parent: %w", parent, fs.ErrInvalid)
			}
		}
		result = append(result, Asset{Path: name, Data: data})
	}
	slices.SortFunc(result, func(a, b Asset) int { return strings.Compare(a.Path, b.Path) })
	return result, nil
}

func (m Mapping) render(binary string, add func(Asset) error) error {
	sourceValid := validPath(m.Source) || (m.Tree && m.Source == ".")
	destinationValid := validPath(m.Destination) || (m.Tree && m.Destination == "")
	if m.Files == nil || !sourceValid || !destinationValid {
		return fmt.Errorf("invalid mapping %q to %q: %w", m.Source, m.Destination, fs.ErrInvalid)
	}
	info, err := fs.Stat(m.Files, m.Source)
	if err != nil {
		return fmt.Errorf("source %q: %w", m.Source, err)
	}
	if info.IsDir() != m.Tree {
		return fmt.Errorf("source %q has wrong file type: %w", m.Source, fs.ErrInvalid)
	}
	read := func(source, destination string, mode fs.FileMode) error {
		if !mode.IsRegular() {
			return fmt.Errorf("source %q is not regular: %w", source, fs.ErrInvalid)
		}
		data, err := fs.ReadFile(m.Files, source)
		if err != nil {
			return fmt.Errorf("read source %q: %w", source, err)
		}
		quoted := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
		data = []byte(strings.ReplaceAll(string(data), "@@BINARY@@", quoted))
		return add(Asset{Path: destination, Data: data})
	}
	if !m.Tree {
		return read(m.Source, m.Destination, info.Mode())
	}
	return fs.WalkDir(m.Files, m.Source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk source %q: %w", name, err)
		}
		if entry.IsDir() {
			return nil
		}
		relative := name
		if m.Source != "." {
			relative = strings.TrimPrefix(name, m.Source+"/")
		}
		destination := relative
		if m.Destination != "" {
			destination = m.Destination + "/" + relative
		}
		return read(name, destination, entry.Type())
	})
}

func validPath(name string) bool {
	return name != "." && fs.ValidPath(name) && !strings.ContainsAny(name, "\\:\x00")
}
