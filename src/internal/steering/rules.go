// Package steering reads the required rule documents supplied by the caller.
package steering

import (
	"bytes"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/memory"
)

// RuleContent is the text of one required rule document.
type RuleContent struct {
	Path string
	Text string
}

// ReadRules reads the requested rule documents in the order given by paths.
func ReadRules(rulesFS fs.FS, paths []string) ([]RuleContent, error) {
	for _, path := range paths {
		if err := validateRulePath(path); err != nil {
			return nil, err
		}
	}
	if isNilFS(rulesFS) {
		return nil, fmt.Errorf("read rules: nil filesystem: %w", fs.ErrInvalid)
	}

	sources := make([]memory.Source, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		text, err := fs.ReadFile(rulesFS, path)
		if err != nil {
			return nil, fmt.Errorf("read required rule %q: %w", path, err)
		}
		if !utf8.Valid(text) {
			return nil, fmt.Errorf("read required rule %q: invalid utf-8: %w", path, fs.ErrInvalid)
		}
		text = bytes.TrimPrefix(text, []byte{0xef, 0xbb, 0xbf})
		sources = append(sources, memory.Source{Path: path, Content: string(text)})
	}

	bundle := memory.BuildBundle(sources)
	rules := make([]RuleContent, 0, len(bundle))
	for _, source := range bundle {
		rules = append(rules, RuleContent{Path: source.Path, Text: source.Content})
	}
	return rules, nil
}

func validateRulePath(path string) error {
	if path == "." || strings.Contains(path, `\`) || !fs.ValidPath(path) {
		return fmt.Errorf("invalid rule path %q: %w", path, fs.ErrInvalid)
	}
	return nil
}

func isNilFS(rulesFS fs.FS) bool {
	if rulesFS == nil {
		return true
	}

	value := reflect.ValueOf(rulesFS)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
