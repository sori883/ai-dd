package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// Intent describes one registry row or orphan intent directory. Repos is
// always non-nil; DirName is nil when a registry row has no matching record.
type Intent struct {
	UUID    string
	Slug    string
	Status  string
	Scope   *string
	Repos   []string
	DirName *string
	Active  bool
}

// IntentListing identifies the project and active space represented by Intents.
type IntentListing struct {
	ProjectRoot string
	SpaceName   string
	Intents     []Intent
}

type registryIntent struct {
	UUID    string   `json:"uuid"`
	Slug    string   `json:"slug"`
	Status  string   `json:"status"`
	Scope   *string  `json:"scope"`
	Repos   []string `json:"repos"`
	DirName *string  `json:"dirName"`
}

// ListIntents correlates the intent registry with record directories.
func ListIntents(intentsFS fs.FS, activeOverride *string) ([]Intent, error) {
	intents := []Intent{}
	registry := []registryIntent{}
	if data, err := fs.ReadFile(intentsFS, "intents.json"); err == nil {
		var decodeErr error
		registry, decodeErr = decodeIntentRegistry(data)
		if decodeErr != nil {
			return nil, decodeErr
		}
	}
	dirs := ListIntentDirs(intentsFS)
	activeName := ""
	hasActive := true
	if activeOverride == nil {
		activeName, hasActive = ActiveIntent(intentsFS, "")
	} else {
		activeName = *activeOverride
	}
	claimed := make(map[string]struct{}, len(registry))
	for _, entry := range registry {
		repos := entry.Repos
		if repos == nil {
			repos = []string{}
		}
		dirName := matchingIntentDir(entry, dirs)
		active := dirName != nil && hasActive && *dirName == activeName
		if dirName != nil {
			claimed[*dirName] = struct{}{}
		}
		intents = append(intents, Intent{
			UUID:    entry.UUID,
			Slug:    entry.Slug,
			Status:  entry.Status,
			Scope:   entry.Scope,
			Repos:   repos,
			DirName: dirName,
			Active:  active,
		})
	}
	for _, dir := range dirs {
		if _, ok := claimed[dir]; ok {
			continue
		}
		dirName := dir
		intents = append(intents, Intent{
			Slug:    orphanIntentSlug(dir),
			Status:  "unknown",
			Repos:   []string{},
			DirName: &dirName,
			Active:  hasActive && dir == activeName,
		})
	}
	return intents, nil
}

func decodeIntentRegistry(data []byte) ([]registryIntent, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return []registryIntent{}, nil
	}

	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return []registryIntent{}, nil
	}
	registry := make([]registryIntent, 0, len(rows))
	for index, row := range rows {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(row, &fields); err != nil || fields == nil {
			return nil, invalidRegistryField(index, "row")
		}

		uuid, ok := requiredRegistryString(fields, "uuid")
		if !ok {
			return nil, invalidRegistryField(index, "uuid")
		}
		slug, ok := requiredRegistryString(fields, "slug")
		if !ok {
			return nil, invalidRegistryField(index, "slug")
		}
		status, ok := requiredRegistryString(fields, "status")
		if !ok {
			return nil, invalidRegistryField(index, "status")
		}
		dirName, ok := optionalRegistryString(fields, "dirName")
		if !ok {
			return nil, invalidRegistryField(index, "dirName")
		}
		scope, ok := optionalRegistryString(fields, "scope")
		if !ok {
			return nil, invalidRegistryField(index, "scope")
		}
		repos, ok := optionalRegistryStrings(fields, "repos")
		if !ok {
			return nil, invalidRegistryField(index, "repos")
		}
		registry = append(registry, registryIntent{
			UUID: uuid, Slug: slug, Status: status, Scope: scope, Repos: repos, DirName: dirName,
		})
	}
	return registry, nil
}

func requiredRegistryString(fields map[string]json.RawMessage, name string) (string, bool) {
	raw, ok := fields[name]
	trimmed := bytes.TrimSpace(raw)
	if !ok || len(trimmed) == 0 || trimmed[0] != '"' {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func optionalRegistryString(fields map[string]json.RawMessage, name string) (*string, bool) {
	raw, ok := fields[name]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return &value, true
}

func optionalRegistryStrings(fields map[string]json.RawMessage, name string) ([]string, bool) {
	raw, ok := fields[name]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return nil, false
	}
	values := make([]string, 0, len(members))
	for _, member := range members {
		trimmed := bytes.TrimSpace(member)
		if len(trimmed) == 0 || trimmed[0] != '"' {
			return nil, false
		}
		var value string
		if err := json.Unmarshal(member, &value); err != nil {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func invalidRegistryField(index int, field string) error {
	return fmt.Errorf("intent registry row %d field %q: %w", index, field, fs.ErrInvalid)
}

func matchingIntentDir(entry registryIntent, dirs []string) *string {
	if entry.DirName != nil && *entry.DirName != "" {
		for _, dir := range dirs {
			if dir == *entry.DirName {
				matched := dir
				return &matched
			}
		}
		return nil
	}

	prefix := entry.Slug + "-"
	uuid := strings.ReplaceAll(entry.UUID, "-", "")
	for _, dir := range dirs {
		if !strings.HasPrefix(dir, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(dir, prefix)
		if !isLowerHex(suffix) || len(suffix) > len(uuid) {
			continue
		}
		if uuid[len(uuid)-len(suffix):] == suffix {
			matched := dir
			return &matched
		}
	}
	return nil
}

func isLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func orphanIntentSlug(dir string) string {
	if len(dir) > 7 && dir[6] == '-' && isASCIIDigits(dir[:6]) {
		return dir[7:]
	}
	if dash := strings.LastIndexByte(dir, '-'); dash >= 0 && isLowerHex(dir[dash+1:]) {
		return dir[:dash]
	}
	return dir
}

func isASCIIDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
