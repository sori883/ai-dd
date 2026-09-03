// Package stageplan builds the immutable stage composition for an Intent.
package stageplan

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sori883/ai-dd/src/internal/graph"
)

const (
	projectTypeGreenfield = "greenfield"
	projectTypeBrownfield = "brownfield"

	reasonScope        = "scope"
	reasonScopeDefault = "scope default"
)

// Input contains the catalog and routing inputs used to build a Plan.
type Input struct {
	Graph       graph.Snapshot
	Scope       string
	ProjectType string
}

// Entry is one stage's immutable-at-build-time routing decision and metadata.
// Stage is copied whenever an Entry is returned by a Plan accessor.
type Entry struct {
	Stage  graph.Stage
	Action graph.Action
	Reason string
}

// Advisory describes a required artifact whose available producers are all
// skipped by the selected scope.
type Advisory struct {
	StageSlug     string
	Artifact      string
	ProducerSlugs []string
}

// Plan is the stage composition selected for one Intent.
//
// Its internal slices are never exposed directly. Accessors return deep copies
// so a caller cannot change a plan after it has been built.
type Plan struct {
	scope                string
	projectType          string
	projectTypeCanonical string
	entries              []Entry
	advisories           []Advisory
	greenfieldAdjustment bool
}

// Build creates a Plan without filesystem, process, or other external side
// effects. The returned Plan is empty when validation fails.
func Build(input Input) (Plan, error) {
	if input.Scope == "" {
		return Plan{}, fmt.Errorf("build stage plan: scope is required")
	}
	if _, ok := input.Graph.Scope(input.Scope); !ok {
		return Plan{}, fmt.Errorf("build stage plan: unknown scope %q", input.Scope)
	}
	projectTypeCanonical, ok := canonicalProjectType(input.ProjectType)
	if !ok {
		return Plan{}, fmt.Errorf("build stage plan: unknown project type %q", input.ProjectType)
	}

	stages := input.Graph.Stages()
	slices.SortStableFunc(stages, compareStageNumber)
	scope, _ := input.Graph.Scope(input.Scope)
	explicitActions := scope.Actions()
	entries := make([]Entry, len(stages))
	greenfieldAdjustment := false
	for index, stage := range stages {
		action := scope.Action(stage.Slug)
		reason := reasonScope
		if _, exists := explicitActions[stage.Slug]; !exists {
			reason = reasonScopeDefault
		}
		if projectTypeCanonical == projectTypeGreenfield &&
			stage.Slug == "reverse-engineering" && action == graph.ActionExecute {
			action = graph.ActionSkip
			reason = "greenfield"
			greenfieldAdjustment = true
		}
		entries[index] = Entry{
			Stage:  cloneStage(stage),
			Action: action,
			Reason: reason,
		}
	}

	advisories, err := validateArtifactDependencies(entries, projectTypeCanonical)
	if err != nil {
		return Plan{}, err
	}

	return Plan{
		scope:                input.Scope,
		projectType:          input.ProjectType,
		projectTypeCanonical: projectTypeCanonical,
		entries:              entries,
		advisories:           advisories,
		greenfieldAdjustment: greenfieldAdjustment,
	}, nil
}

// Entries returns all stage decisions in ascending stage-number order.
func (p Plan) Entries() []Entry {
	entries := make([]Entry, len(p.entries))
	for index, entry := range p.entries {
		entries[index] = cloneEntry(entry)
	}
	return entries
}

// Stages is an alias for Entries for callers that describe the plan as stage
// rows.
func (p Plan) Stages() []Entry { return p.Entries() }

// ExecuteStages returns the stages whose effective action is EXECUTE.
func (p Plan) ExecuteStages() []Entry {
	return p.entriesWithAction(graph.ActionExecute)
}

// SkipStages returns the stages whose effective action is SKIP.
func (p Plan) SkipStages() []Entry {
	return p.entriesWithAction(graph.ActionSkip)
}

// Advisories returns artifact dependency advisories for this plan.
func (p Plan) Advisories() []Advisory {
	advisories := make([]Advisory, len(p.advisories))
	for index, advisory := range p.advisories {
		advisories[index] = Advisory{
			StageSlug:     advisory.StageSlug,
			Artifact:      advisory.Artifact,
			ProducerSlugs: slices.Clone(advisory.ProducerSlugs),
		}
	}
	return advisories
}

// Scope returns the scope used to build the plan.
func (p Plan) Scope() string { return p.scope }

// ScopeName returns the scope used to build the plan.
func (p Plan) ScopeName() string { return p.scope }

// ProjectType returns the validated project type as supplied by the caller.
func (p Plan) ProjectType() string { return p.projectType }

// GreenfieldAdjusted reports whether reverse-engineering was changed from
// EXECUTE to SKIP for a Greenfield project.
func (p Plan) GreenfieldAdjusted() bool { return p.greenfieldAdjustment }

// ReverseEngineeringSkippedGreenfield reports whether reverse-engineering was
// changed from EXECUTE to SKIP for a Greenfield project.
func (p Plan) ReverseEngineeringSkippedGreenfield() bool {
	return p.greenfieldAdjustment
}

func (p Plan) entriesWithAction(action graph.Action) []Entry {
	entries := make([]Entry, 0, len(p.entries))
	for _, entry := range p.entries {
		if entry.Action != action {
			continue
		}
		entries = append(entries, cloneEntry(entry))
	}
	return entries
}

func canonicalProjectType(value string) (string, bool) {
	switch strings.ToLower(value) {
	case projectTypeGreenfield:
		return projectTypeGreenfield, true
	case projectTypeBrownfield:
		return projectTypeBrownfield, true
	default:
		return "", false
	}
}

func compareStageNumber(left, right graph.Stage) int {
	leftPhase, leftIndex, leftOK := parseStageNumber(left.Number)
	rightPhase, rightIndex, rightOK := parseStageNumber(right.Number)
	if leftOK && rightOK {
		if leftPhase != rightPhase {
			return compareInt(leftPhase, rightPhase)
		}
		if leftIndex != rightIndex {
			return compareInt(leftIndex, rightIndex)
		}
		return 0
	}
	return strings.Compare(left.Number, right.Number)
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
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

func cloneEntry(entry Entry) Entry {
	return Entry{
		Stage:  cloneStage(entry.Stage),
		Action: entry.Action,
		Reason: entry.Reason,
	}
}

func cloneStage(stage graph.Stage) graph.Stage {
	stage.SupportAgents = slices.Clone(stage.SupportAgents)
	stage.Scopes = slices.Clone(stage.Scopes)
	stage.Produces = slices.Clone(stage.Produces)
	stage.OptionalProduces = slices.Clone(stage.OptionalProduces)
	stage.Consumes = slices.Clone(stage.Consumes)
	stage.RequiresStages = slices.Clone(stage.RequiresStages)
	return stage
}

func validateArtifactDependencies(entries []Entry, projectType string) ([]Advisory, error) {
	producers := make(map[string][]string)
	for _, entry := range entries {
		for _, artifact := range entry.Stage.Produces {
			producers[artifact] = appendUnique(producers[artifact], entry.Stage.Slug)
		}
		for _, artifact := range entry.Stage.OptionalProduces {
			producers[artifact] = appendUnique(producers[artifact], entry.Stage.Slug)
		}
	}

	actions := make(map[string]graph.Action, len(entries))
	for _, entry := range entries {
		actions[entry.Stage.Slug] = entry.Action
	}

	advisories := make([]Advisory, 0)
	for _, entry := range entries {
		if entry.Action != graph.ActionExecute {
			continue
		}
		for _, consume := range entry.Stage.Consumes {
			if !consume.Required || !consumeMatchesProjectType(consume, projectType) {
				continue
			}
			producerSlugs := slices.Clone(producers[consume.Artifact])
			if len(producerSlugs) == 0 {
				return nil, fmt.Errorf(
					"build stage plan: stage %q requires artifact %q but no enabled producer exists",
					entry.Stage.Slug,
					consume.Artifact,
				)
			}
			hasExecuteProducer := false
			for _, producerSlug := range producerSlugs {
				if actions[producerSlug] == graph.ActionExecute {
					hasExecuteProducer = true
					break
				}
			}
			if hasExecuteProducer {
				continue
			}
			advisories = append(advisories, Advisory{
				StageSlug:     entry.Stage.Slug,
				Artifact:      consume.Artifact,
				ProducerSlugs: producerSlugs,
			})
		}
	}
	return advisories, nil
}

func consumeMatchesProjectType(consume graph.Consume, projectType string) bool {
	return consume.ConditionalOn == "" || strings.EqualFold(consume.ConditionalOn, projectType)
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
