// Package graph loads the read-only AI-DLC stage graph and scope routing grid.
package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
)

// Action is the routing decision for a stage in a scope.
type Action string

const (
	// ActionUnknown is the zero-value sentinel. Load never stores it.
	ActionUnknown Action = ""
	// ActionExecute routes execution through a stage.
	ActionExecute Action = "EXECUTE"
	// ActionSkip omits a stage from a scope.
	ActionSkip Action = "SKIP"
)

// Stage is the routing metadata required by the AI-DLC runtime.
type Stage struct {
	Slug          string
	Number        string
	Name          string
	Phase         string
	Execution     string
	LeadAgent     string
	SupportAgents []string
	Mode          string
	Scopes        []string
	Enabled       bool
}

// Scope contains the routing actions for one scope.
type Scope struct {
	actions map[string]Action
}

// Action returns the stage's action, defaulting to SKIP when no cell exists.
func (s Scope) Action(stageSlug string) Action {
	if action, ok := s.actions[stageSlug]; ok {
		return action
	}
	return ActionSkip
}

// Actions returns an independently mutable copy of the scope's explicit cells.
func (s Scope) Actions() map[string]Action {
	clone := make(map[string]Action, len(s.actions))
	for slug, action := range s.actions {
		clone[slug] = action
	}
	return clone
}

// Snapshot is an immutable view of the enabled stages and their scope routes.
type Snapshot struct {
	stages     []Stage
	scopeNames []string
	scopes     map[string]Scope
}

// Load reads stage-graph.json and scope-grid.json from dataFS.
func Load(dataFS fs.FS) (Snapshot, error) {
	if dataFS == nil {
		return Snapshot{}, errors.New("load graph: nil filesystem")
	}
	data, err := fs.ReadFile(dataFS, "stage-graph.json")
	if err != nil {
		return Snapshot{}, fmt.Errorf("load stage graph: read stage-graph.json: %w", err)
	}
	var rawStages []stageDocument
	if err := json.Unmarshal(data, &rawStages); err != nil {
		return Snapshot{}, fmt.Errorf("load stage graph: decode stage-graph.json: %w", err)
	}
	if err := validateStageDocuments(rawStages); err != nil {
		return Snapshot{}, fmt.Errorf("load stage graph: validate stage-graph.json: %w", err)
	}

	stages := make([]Stage, 0, len(rawStages))
	enabledSlugs := make(map[string]struct{}, len(rawStages))
	allSlugs := make(map[string]struct{}, len(rawStages))
	for _, raw := range rawStages {
		allSlugs[raw.Slug] = struct{}{}
		if raw.Enabled != nil && !*raw.Enabled {
			continue
		}
		stages = append(stages, Stage{
			Slug:          raw.Slug,
			Number:        raw.Number,
			Name:          raw.Name,
			Phase:         raw.Phase,
			Execution:     raw.Execution,
			LeadAgent:     raw.LeadAgent,
			SupportAgents: raw.SupportAgents,
			Mode:          raw.Mode,
			Scopes:        raw.Scopes,
			Enabled:       true,
		})
		enabledSlugs[raw.Slug] = struct{}{}
	}

	rawGrid, err := loadScopeDocuments(dataFS, stages)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validateScopeActions(rawGrid, allSlugs); err != nil {
		return Snapshot{}, fmt.Errorf("load scope grid: validate scope-grid.json: %w", err)
	}
	scopeNames := make([]string, 0, len(rawGrid))
	scopes := make(map[string]Scope, len(rawGrid))
	for name, rawScope := range rawGrid {
		actions := make(map[string]Action, len(rawScope.Stages))
		for slug, action := range rawScope.Stages {
			if _, enabled := enabledSlugs[slug]; enabled {
				actions[slug] = action
			}
		}
		scopeNames = append(scopeNames, name)
		scopes[name] = Scope{actions: actions}
	}
	slices.Sort(scopeNames)
	return Snapshot{stages: stages, scopeNames: scopeNames, scopes: scopes}, nil
}

type stageDocument struct {
	Slug          string   `json:"slug"`
	Number        string   `json:"number"`
	Name          string   `json:"name"`
	Phase         string   `json:"phase"`
	Execution     string   `json:"execution"`
	LeadAgent     string   `json:"lead_agent"`
	SupportAgents []string `json:"support_agents"`
	Mode          string   `json:"mode"`
	Scopes        []string `json:"scopes"`
	Enabled       *bool    `json:"enabled"`
}

func validateStageDocuments(stages []stageDocument) error {
	if stages == nil {
		return errors.New("top-level array is required")
	}

	slugs := make(map[string]struct{}, len(stages))
	numbers := make(map[string]struct{}, len(stages))
	for index, stage := range stages {
		requiredStrings := []struct {
			name  string
			value string
		}{
			{name: "slug", value: stage.Slug},
			{name: "number", value: stage.Number},
			{name: "name", value: stage.Name},
			{name: "phase", value: stage.Phase},
			{name: "execution", value: stage.Execution},
			{name: "lead_agent", value: stage.LeadAgent},
			{name: "mode", value: stage.Mode},
		}
		for _, field := range requiredStrings {
			if field.value == "" {
				return fmt.Errorf("stage %d: %s is required", index, field.name)
			}
		}
		if stage.SupportAgents == nil {
			return fmt.Errorf("stage %d: support_agents array is required", index)
		}
		if stage.Execution != "ALWAYS" && stage.Execution != "CONDITIONAL" {
			return fmt.Errorf("stage %d: execution %q is invalid", index, stage.Execution)
		}
		if _, exists := slugs[stage.Slug]; exists {
			return fmt.Errorf("stage %d: duplicate slug %q", index, stage.Slug)
		}
		if _, exists := numbers[stage.Number]; exists {
			return fmt.Errorf("stage %d: duplicate number %q", index, stage.Number)
		}
		slugs[stage.Slug] = struct{}{}
		numbers[stage.Number] = struct{}{}
	}
	return nil
}

type scopeDocument struct {
	Stages map[string]Action `json:"stages"`
}

func loadScopeDocuments(dataFS fs.FS, stages []Stage) (map[string]scopeDocument, error) {
	data, err := fs.ReadFile(dataFS, "scope-grid.json")
	if err != nil || !json.Valid(data) {
		return transposeStageScopes(stages), nil
	}

	grid, err := decodeScopeDocuments(data)
	if err != nil {
		return nil, fmt.Errorf("load scope grid: validate scope-grid.json: %w", err)
	}
	return grid, nil
}

func decodeScopeDocuments(data []byte) (map[string]scopeDocument, error) {
	var rawGrid map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawGrid); err != nil {
		return nil, fmt.Errorf("top-level object: %w", err)
	}
	if rawGrid == nil {
		return nil, errors.New("top-level object is required")
	}

	grid := make(map[string]scopeDocument, len(rawGrid))
	for name, rawScope := range rawGrid {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawScope, &fields); err != nil {
			return nil, fmt.Errorf("scope %q must be an object: %w", name, err)
		}
		if fields == nil {
			return nil, fmt.Errorf("scope %q must be an object", name)
		}
		rawStages, exists := fields["stages"]
		if !exists {
			return nil, fmt.Errorf("scope %q: stages object is required", name)
		}
		var actions map[string]Action
		if err := json.Unmarshal(rawStages, &actions); err != nil {
			return nil, fmt.Errorf("scope %q: stages must be an object: %w", name, err)
		}
		if actions == nil {
			return nil, fmt.Errorf("scope %q: stages object is required", name)
		}
		grid[name] = scopeDocument{Stages: actions}
	}
	return grid, nil
}

func validateScopeActions(grid map[string]scopeDocument, stageSlugs map[string]struct{}) error {
	for scope, document := range grid {
		for slug, action := range document.Stages {
			if _, exists := stageSlugs[slug]; !exists {
				return fmt.Errorf("scope %q references unknown stage %q", scope, slug)
			}
			if action != ActionExecute && action != ActionSkip {
				return fmt.Errorf("scope %q stage %q has invalid action %q", scope, slug, action)
			}
		}
	}
	return nil
}

func transposeStageScopes(stages []Stage) map[string]scopeDocument {
	names := make(map[string]struct{})
	for _, stage := range stages {
		for _, name := range stage.Scopes {
			names[name] = struct{}{}
		}
	}

	grid := make(map[string]scopeDocument, len(names))
	for name := range names {
		actions := make(map[string]Action, len(stages))
		for _, stage := range stages {
			action := ActionSkip
			if slices.Contains(stage.Scopes, name) {
				action = ActionExecute
			}
			actions[stage.Slug] = action
		}
		grid[name] = scopeDocument{Stages: actions}
	}
	return grid
}

// Stages returns the enabled stages in graph order.
func (s Snapshot) Stages() []Stage {
	stages := make([]Stage, len(s.stages))
	for index, stage := range s.stages {
		stage.SupportAgents = slices.Clone(stage.SupportAgents)
		stage.Scopes = slices.Clone(stage.Scopes)
		stages[index] = stage
	}
	return stages
}

// ScopeNames returns the available scope names.
func (s Snapshot) ScopeNames() []string { return slices.Clone(s.scopeNames) }

// Scope returns a named scope.
func (s Snapshot) Scope(name string) (Scope, bool) {
	scope, ok := s.scopes[name]
	if !ok {
		return Scope{}, false
	}
	return Scope{actions: scope.Actions()}, true
}
