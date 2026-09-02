// Package graph loads the read-only AI-DLC stage graph and scope routing grid.
package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
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

// Consume describes one artifact required or optionally used by a stage.
type Consume struct {
	Artifact      string `json:"artifact"`
	Required      bool   `json:"required"`
	ConditionalOn string `json:"conditional_on"`
}

// Stage is the routing metadata required by the AI-DLC runtime.
type Stage struct {
	Slug             string
	Number           string
	Name             string
	Phase            string
	Execution        string
	LeadAgent        string
	SupportAgents    []string
	Mode             string
	Scopes           []string
	Enabled          bool
	Produces         []string
	OptionalProduces []string
	Consumes         []Consume
	RequiresStages   []string
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
	rawStages, err := decodeStageDocuments(data)
	if err != nil {
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
			Slug:             raw.Slug,
			Number:           raw.Number,
			Name:             raw.Name,
			Phase:            raw.Phase,
			Execution:        raw.Execution,
			LeadAgent:        raw.LeadAgent,
			SupportAgents:    raw.SupportAgents,
			Mode:             raw.Mode,
			Scopes:           raw.Scopes,
			Enabled:          true,
			Produces:         raw.Produces,
			OptionalProduces: raw.OptionalProduces,
			Consumes:         consumeValues(raw.Consumes),
			RequiresStages:   raw.RequiresStages,
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
	slices.SortFunc(scopeNames, func(a, b string) int {
		return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
	})
	return Snapshot{stages: stages, scopeNames: scopeNames, scopes: scopes}, nil
}

type stageDocument struct {
	Slug                    string   `json:"slug"`
	Number                  string   `json:"number"`
	Name                    string   `json:"name"`
	Phase                   string   `json:"phase"`
	Execution               string   `json:"execution"`
	LeadAgent               string   `json:"lead_agent"`
	SupportAgents           []string `json:"support_agents"`
	Mode                    string   `json:"mode"`
	Scopes                  []string `json:"scopes"`
	Enabled                 *bool    `json:"enabled"`
	Produces                []string
	ProducesPresent         bool
	OptionalProduces        []string
	OptionalProducesPresent bool
	Consumes                []consumeDocument
	ConsumesPresent         bool
	RequiresStages          []string
	RequiresStagesPresent   bool
}

type consumeDocument struct {
	Consume
	ArtifactPresent      bool
	RequiredPresent      bool
	ConditionalOnPresent bool
}

func decodeStageDocuments(data []byte) ([]stageDocument, error) {
	var rawStages []json.RawMessage
	if err := json.Unmarshal(data, &rawStages); err != nil {
		return nil, err
	}
	if rawStages == nil {
		return nil, nil
	}

	stages := make([]stageDocument, len(rawStages))
	for index, raw := range rawStages {
		stage, err := decodeStageDocument(raw)
		if err != nil {
			return nil, fmt.Errorf("stage %d: %w", index, err)
		}
		stages[index] = stage
	}
	return stages, nil
}

func decodeStageDocument(data []byte) (stageDocument, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return stageDocument{}, err
	}
	if fields == nil {
		return stageDocument{}, nil
	}

	var stage stageDocument
	stageFields := []struct {
		name   string
		target any
	}{
		{name: "slug", target: &stage.Slug},
		{name: "number", target: &stage.Number},
		{name: "name", target: &stage.Name},
		{name: "phase", target: &stage.Phase},
		{name: "execution", target: &stage.Execution},
		{name: "lead_agent", target: &stage.LeadAgent},
		{name: "support_agents", target: &stage.SupportAgents},
		{name: "mode", target: &stage.Mode},
		{name: "scopes", target: &stage.Scopes},
		{name: "enabled", target: &stage.Enabled},
	}
	for _, field := range stageFields {
		raw, exists := fields[field.name]
		if !exists {
			continue
		}
		if err := json.Unmarshal(raw, field.target); err != nil {
			return stageDocument{}, fmt.Errorf("field %q: %w", field.name, err)
		}
	}
	if raw, exists := fields["produces"]; exists {
		stage.ProducesPresent = true
		var err error
		stage.Produces, err = decodeStringArray(raw)
		if err != nil {
			return stageDocument{}, fmt.Errorf("field %q: %w", "produces", err)
		}
	}
	if raw, exists := fields["optional_produces"]; exists {
		stage.OptionalProducesPresent = true
		var err error
		stage.OptionalProduces, err = decodeStringArray(raw)
		if err != nil {
			return stageDocument{}, fmt.Errorf("field %q: %w", "optional_produces", err)
		}
	}
	if raw, exists := fields["consumes"]; exists {
		stage.ConsumesPresent = true
		var err error
		stage.Consumes, err = decodeConsumes(raw)
		if err != nil {
			return stageDocument{}, fmt.Errorf("field %q: %w", "consumes", err)
		}
	}
	if raw, exists := fields["requires_stage"]; exists {
		stage.RequiresStagesPresent = true
		var err error
		stage.RequiresStages, err = decodeStringArray(raw)
		if err != nil {
			return stageDocument{}, fmt.Errorf("field %q: %w", "requires_stage", err)
		}
	}
	return stage, nil
}

func decodeStringArray(data json.RawMessage) ([]string, error) {
	if isJSONNull(data) {
		return nil, errors.New("must be an array")
	}

	var rawValues []json.RawMessage
	if err := json.Unmarshal(data, &rawValues); err != nil {
		return nil, err
	}
	if rawValues == nil {
		return nil, errors.New("must be an array")
	}
	values := make([]string, len(rawValues))
	for index, rawValue := range rawValues {
		if isJSONNull(rawValue) {
			return nil, fmt.Errorf("item %d must be a string", index)
		}
		if err := json.Unmarshal(rawValue, &values[index]); err != nil {
			return nil, fmt.Errorf("item %d must be a string: %w", index, err)
		}
	}
	return values, nil
}

func decodeConsumes(data json.RawMessage) ([]consumeDocument, error) {
	if isJSONNull(data) {
		return nil, errors.New("must be an array")
	}

	var rawConsumes []json.RawMessage
	if err := json.Unmarshal(data, &rawConsumes); err != nil {
		return nil, err
	}
	if rawConsumes == nil {
		return nil, errors.New("must be an array")
	}

	consumes := make([]consumeDocument, len(rawConsumes))
	for index, rawConsume := range rawConsumes {
		consume, err := decodeConsume(rawConsume)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", index, err)
		}
		consumes[index] = consume
	}
	return consumes, nil
}

func decodeConsume(data json.RawMessage) (consumeDocument, error) {
	if isJSONNull(data) {
		return consumeDocument{}, errors.New("must be an object")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return consumeDocument{}, err
	}
	if fields == nil {
		return consumeDocument{}, errors.New("must be an object")
	}

	var consume consumeDocument
	if raw, exists := fields["artifact"]; exists {
		consume.ArtifactPresent = true
		if isJSONNull(raw) {
			return consumeDocument{}, errors.New(`field "artifact" must be a string`)
		}
		if err := json.Unmarshal(raw, &consume.Artifact); err != nil {
			return consumeDocument{}, fmt.Errorf("field %q: %w", "artifact", err)
		}
	}
	if raw, exists := fields["required"]; exists {
		consume.RequiredPresent = true
		if isJSONNull(raw) {
			return consumeDocument{}, errors.New(`field "required" must be a boolean`)
		}
		if err := json.Unmarshal(raw, &consume.Required); err != nil {
			return consumeDocument{}, fmt.Errorf("field %q: %w", "required", err)
		}
	}
	if raw, exists := fields["conditional_on"]; exists {
		consume.ConditionalOnPresent = true
		if isJSONNull(raw) {
			return consumeDocument{}, errors.New(`field "conditional_on" must be a string`)
		}
		if err := json.Unmarshal(raw, &consume.ConditionalOn); err != nil {
			return consumeDocument{}, fmt.Errorf("field %q: %w", "conditional_on", err)
		}
	}
	return consume, nil
}

func isJSONNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func consumeValues(documents []consumeDocument) []Consume {
	if documents == nil {
		return nil
	}
	consumes := make([]Consume, len(documents))
	for index, document := range documents {
		consumes[index] = document.Consume
	}
	return consumes
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
		if !stage.ProducesPresent || stage.Produces == nil {
			return fmt.Errorf("stage %d: produces array is required", index)
		}
		if !stage.ConsumesPresent || stage.Consumes == nil {
			return fmt.Errorf("stage %d: consumes array is required", index)
		}
		if !stage.RequiresStagesPresent || stage.RequiresStages == nil {
			return fmt.Errorf("stage %d: requires_stage array is required", index)
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
	for index, stage := range stages {
		seenDependencies := make(map[string]struct{}, len(stage.RequiresStages))
		for dependencyIndex, dependency := range stage.RequiresStages {
			if _, duplicate := seenDependencies[dependency]; duplicate {
				return fmt.Errorf("stage %d: requires_stage[%d] duplicates %q", index, dependencyIndex, dependency)
			}
			seenDependencies[dependency] = struct{}{}

			dependencyStage, exists := findStageDocument(stages, dependency)
			if !exists {
				return fmt.Errorf("stage %d: requires_stage[%d] references unknown stage %q", index, dependencyIndex, dependency)
			}
			if !stageNumberBefore(dependencyStage.Number, stage.Number) {
				return fmt.Errorf("stage %d: requires_stage[%d] dependency %q (%s) must precede stage %q (%s)", index, dependencyIndex, dependency, dependencyStage.Number, stage.Slug, stage.Number)
			}
		}
		for consumeIndex, consume := range stage.Consumes {
			if !consume.ArtifactPresent || consume.Artifact == "" {
				return fmt.Errorf("stage %d: consumes[%d].artifact is required", index, consumeIndex)
			}
			if !consume.RequiredPresent {
				return fmt.Errorf("stage %d: consumes[%d].required is required", index, consumeIndex)
			}
			if consume.ConditionalOnPresent && consume.ConditionalOn != "brownfield" && consume.ConditionalOn != "greenfield" {
				return fmt.Errorf("stage %d: consumes[%d].conditional_on %q is invalid", index, consumeIndex, consume.ConditionalOn)
			}
		}
	}
	return nil
}

func findStageDocument(stages []stageDocument, slug string) (stageDocument, bool) {
	for _, stage := range stages {
		if stage.Slug == slug {
			return stage, true
		}
	}
	return stageDocument{}, false
}

func stageNumberBefore(dependency, stage string) bool {
	dependencyPhase, dependencyIndex, dependencyOK := parseStageNumber(dependency)
	stagePhase, stageIndex, stageOK := parseStageNumber(stage)
	if !dependencyOK || !stageOK {
		return false
	}
	if dependencyPhase != stagePhase {
		return dependencyPhase < stagePhase
	}
	return dependencyIndex < stageIndex
}

func parseStageNumber(number string) (int, int, bool) {
	parts := strings.Split(number, ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	phase, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return phase, index, true
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
		stage.Produces = slices.Clone(stage.Produces)
		stage.OptionalProduces = slices.Clone(stage.OptionalProduces)
		stage.Consumes = slices.Clone(stage.Consumes)
		stage.RequiresStages = slices.Clone(stage.RequiresStages)
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
