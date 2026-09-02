// Package memory reads the layered memory sources of an AI-DLC space.
package memory

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"unicode/utf8"
)

// Layer identifies the memory layer a source belongs to.
type Layer string

const (
	LayerOrg     Layer = "org"
	LayerTeam    Layer = "team"
	LayerProject Layer = "project"
	LayerPhase   Layer = "phase"
)

// Source is one memory source read from the memory root.
type Source struct {
	Layer   Layer
	Path    string
	Content string
}

// ReadSources reads the fixed memory source paths in priority order.
func ReadSources(memoryFS fs.FS, phase string) ([]Source, error) {
	if !validPhase(phase) {
		return nil, fmt.Errorf("invalid memory phase %q: %w", phase, fs.ErrInvalid)
	}
	if isNilFS(memoryFS) {
		return nil, fmt.Errorf("read memory sources: nil filesystem: %w", fs.ErrInvalid)
	}

	paths := []struct {
		layer Layer
		path  string
	}{
		{layer: LayerOrg, path: "org.md"},
		{layer: LayerTeam, path: "team.md"},
		{layer: LayerProject, path: "project.md"},
		{layer: LayerPhase, path: "phases/" + phase + ".md"},
	}

	sources := make([]Source, 0, len(paths))
	for _, candidate := range paths {
		content, err := fs.ReadFile(memoryFS, candidate.path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read memory source %q: %w", candidate.path, err)
		}
		if !utf8.Valid(content) {
			return nil, fmt.Errorf("read memory source %q: invalid utf-8: %w", candidate.path, fs.ErrInvalid)
		}
		sources = append(sources, Source{
			Layer:   candidate.layer,
			Path:    candidate.path,
			Content: string(content),
		})
	}
	return sources, nil
}

func isNilFS(memoryFS fs.FS) bool {
	if memoryFS == nil {
		return true
	}

	value := reflect.ValueOf(memoryFS)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validPhase(phase string) bool {
	if phase == "" || phase[0] < 'a' || phase[0] > 'z' {
		return false
	}
	for index := 1; index < len(phase); index++ {
		character := phase[index]
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}
