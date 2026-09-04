// Package knowledge builds the ordered, preflighted knowledge context roster.
package knowledge

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/sori883/ai-dd/src/internal/graph"
)

// Source is a caller-owned filesystem root and its display prefix.
type Source struct {
	FS            fs.FS
	DisplayPrefix string
}

// RosterInput describes the stage and the installed knowledge roots to read.
type RosterInput struct {
	Stage          graph.Stage
	Depth          string
	Framework      Source
	FrameworkDir   string
	SpaceKnowledge *Source
	EnabledPlugins []string
}

// Roster contains ordered display paths and non-blocking read warnings.
type Roster struct {
	Paths    []string
	Warnings []string
}

// BuildRoster builds the ordered knowledge context roster.
func BuildRoster(input RosterInput) (Roster, error) {
	agents, err := inlineAgents(input.Stage)
	if err != nil {
		return Roster{}, err
	}
	if len(agents) == 0 {
		return emptyRoster(), nil
	}
	if err := validateSource(input.Framework); err != nil {
		return Roster{}, fmt.Errorf("validate framework source: %w", err)
	}
	if input.SpaceKnowledge != nil {
		if err := validateSource(*input.SpaceKnowledge); err != nil {
			return Roster{}, fmt.Errorf("validate space knowledge source: %w", err)
		}
	}
	if err := validateFrameworkDir(input.FrameworkDir); err != nil {
		return Roster{}, fmt.Errorf("validate framework directory: %w", err)
	}

	warnings := []string{}
	candidates, scanWarnings := collectCandidates(input, agents)
	warnings = append(warnings, scanWarnings...)
	preflightCandidates(candidates, &warnings)
	owners := readPluginOwners(input.Framework, input.FrameworkDir, &warnings)
	selected := selectCandidates(candidates, input.Depth, input.EnabledPlugins, owners)
	paths := deduplicatePaths(selected)
	paths, warnings = boundRoster(paths, warnings)
	return Roster{Paths: paths, Warnings: warnings}, nil
}

func emptyRoster() Roster {
	return Roster{Paths: []string{}, Warnings: []string{}}
}

func inlineAgents(stage graph.Stage) ([]string, error) {
	var declared []string
	switch stage.Mode {
	case "inline":
		declared = append(declared, stage.LeadAgent)
		declared = append(declared, stage.SupportAgents...)
	case "mob":
		declared = []string{stage.LeadAgent}
	default:
		return []string{}, nil
	}

	agents := make([]string, 0, len(declared))
	seen := make(map[string]struct{}, len(declared))
	for _, agent := range declared {
		if agent == "orchestrator" {
			continue
		}
		if err := validateAgent(agent); err != nil {
			return nil, fmt.Errorf("invalid agent %q: %w", agent, err)
		}
		if _, ok := seen[agent]; ok {
			continue
		}
		seen[agent] = struct{}{}
		agents = append(agents, agent)
	}
	return agents, nil
}

func validateAgent(agent string) error {
	if !utf8.ValidString(agent) || strings.IndexByte(agent, 0) >= 0 {
		return fmt.Errorf("agent must be valid UTF-8 without NUL: %w", fs.ErrInvalid)
	}
	if agent == "." || strings.Contains(agent, "/") || !fs.ValidPath(agent) {
		return fmt.Errorf("agent must be one native path component: %w", fs.ErrInvalid)
	}
	if _, err := filepath.Localize(agent); err != nil {
		return fmt.Errorf("agent is not a native path component: %w: %v", fs.ErrInvalid, err)
	}
	return nil
}

func validateSource(source Source) error {
	if isNilFS(source.FS) {
		return fmt.Errorf("nil filesystem: %w", fs.ErrInvalid)
	}
	if err := validateDisplayPath(source.DisplayPrefix); err != nil {
		return fmt.Errorf("invalid display prefix: %w", err)
	}
	return nil
}

func validateDisplayPath(value string) error {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("path must be valid UTF-8 without NUL: %w", fs.ErrInvalid)
	}
	if value == "." || !fs.ValidPath(value) {
		return fmt.Errorf("path must be a non-dot fs.ValidPath: %w", fs.ErrInvalid)
	}
	if _, err := filepath.Localize(value); err != nil {
		return fmt.Errorf("path is not native: %w: %v", fs.ErrInvalid, err)
	}
	return nil
}

func validateFrameworkDir(value string) error {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("path must be valid UTF-8 without NUL: %w", fs.ErrInvalid)
	}
	if !filepath.IsAbs(value) {
		return fmt.Errorf("path must be native absolute: %w", fs.ErrInvalid)
	}
	return nil
}

func isNilFS(value fs.FS) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func joinDisplay(prefix, relative string) string {
	return path.Join(prefix, relative)
}

func missingPersonaWarning(display string) string {
	return fmt.Sprintf("Warning: optional persona/knowledge file %q is missing. Restore the file; this stage will continue without that context.", display)
}

func unreadableWarning(display string, err error) string {
	return fmt.Sprintf("Warning: optional persona/knowledge file %q is unreadable or invalid UTF-8 (%v). Fix the file, encoding, or permissions; this stage will continue without that context.", display, err)
}

func invalidUTF8Warning(display string) string {
	return fmt.Sprintf("Warning: optional persona/knowledge file %q is unreadable or invalid UTF-8 (invalid UTF-8). Fix the file, encoding, or permissions; this stage will continue without that context.", display)
}
