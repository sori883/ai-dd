package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type pluginOwners map[string]map[string]struct{}

func readPluginOwners(source Source, frameworkDir string, warnings *[]string) pluginOwners {
	owners := make(pluginOwners)
	entries, err := fs.ReadDir(source.FS, "tools/data")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return owners
		}
		*warnings = append(*warnings, unreadablePluginDirectoryWarning(
			filepath.ToSlash(filepath.Join(frameworkDir, "tools", "data")),
			err,
		))
		return owners
	}
	sort.SliceStable(entries, func(left, right int) bool {
		return compareUTF16(entries[left].Name(), entries[right].Name()) < 0
	})
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "plugin-files-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		relative := filepath.ToSlash(filepath.Join("tools", "data", name))
		display := filepath.ToSlash(filepath.Join(frameworkDir, relative))
		data, readErr := fs.ReadFile(source.FS, relative)
		if readErr != nil {
			*warnings = append(*warnings, invalidPluginManifestWarning(display, readErr))
			continue
		}
		if !utf8.Valid(data) {
			*warnings = append(*warnings, invalidPluginManifestWarning(display, errors.New("invalid UTF-8")))
			continue
		}
		plugin, values, parseErr := parsePluginManifest(data)
		if parseErr != nil {
			*warnings = append(*warnings, invalidPluginManifestWarning(display, parseErr))
		}
		for _, value := range values {
			rel := filepath.ToSlash(filepath.Join("knowledge", filepath.FromSlash(value)))
			paths := owners[rel]
			if paths == nil {
				paths = make(map[string]struct{})
				owners[rel] = paths
			}
			paths[plugin] = struct{}{}
		}
	}
	return owners
}

func parsePluginManifest(data []byte) (string, []string, error) {
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &fields); err != nil {
		return "", nil, err
	}
	if fields == nil {
		return "", nil, errors.New("expected a JSON object")
	}

	versionRaw, ok := fields["schema_version"]
	if !ok {
		return "", nil, errors.New("expected schema_version 1, plugin, and knowledge[]")
	}
	var version any
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return "", nil, fmt.Errorf("schema_version: %w", err)
	}
	versionNumber, ok := version.(float64)
	if !ok || versionNumber != 1 {
		return "", nil, errors.New("expected schema_version 1, plugin, and knowledge[]")
	}

	pluginRaw, ok := fields["plugin"]
	if !ok {
		return "", nil, errors.New("expected schema_version 1, plugin, and knowledge[]")
	}
	var pluginValue any
	if err := json.Unmarshal(pluginRaw, &pluginValue); err != nil {
		return "", nil, fmt.Errorf("plugin: %w", err)
	}
	plugin, ok := pluginValue.(string)
	if !ok {
		return "", nil, errors.New("expected plugin to be a string")
	}

	knowledgeRaw, ok := fields["knowledge"]
	if !ok || len(strings.TrimSpace(string(knowledgeRaw))) == 0 ||
		strings.TrimSpace(string(knowledgeRaw))[0] != '[' {
		return plugin, nil, errors.New("expected schema_version 1, plugin, and knowledge[]")
	}
	var rawValues []json.RawMessage
	if err := json.Unmarshal(knowledgeRaw, &rawValues); err != nil {
		return plugin, nil, fmt.Errorf("knowledge: %w", err)
	}

	values := make([]string, 0, len(rawValues))
	for index, rawValue := range rawValues {
		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return plugin, values, fmt.Errorf("knowledge[%d]: %w", index, err)
		}
		stringValue, ok := value.(string)
		if !ok || !validPluginKnowledgePath(stringValue) {
			return plugin, values, fmt.Errorf("knowledge[%d]: knowledge paths must be relative path segments", index)
		}
		values = append(values, stringValue)
	}
	return plugin, values, nil
}

func validPluginKnowledgePath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." {
			return false
		}
	}
	return true
}

func unreadablePluginDirectoryWarning(display string, err error) string {
	return fmt.Sprintf("Warning: plugin knowledge ownership data \"%s\" is unreadable (%v). Minimal context will continue without plugin provenance.", display, err)
}

func invalidPluginManifestWarning(display string, err error) string {
	return fmt.Sprintf("Warning: plugin knowledge ownership file \"%s\" is invalid (%v). Re-run plugin composition before relying on Minimal context pruning.", display, err)
}
