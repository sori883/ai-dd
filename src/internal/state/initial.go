// Package state builds the in-memory initial state for an AI-DLC workflow.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/stageplan"
)

const (
	currentStateVersion     = "8"
	projectDescriptionFile  = "project-description.json"
	projectDescriptionValue = "[Project description]"
)

var phases = [...]string{
	"initialization",
	"ideation",
	"inception",
	"construction",
	"operation",
}

var phaseHeaders = map[string]string{
	"initialization": "INITIALIZATION PHASE",
	"ideation":       "IDEATION PHASE",
	"inception":      "INCEPTION PHASE",
	"construction":   "CONSTRUCTION PHASE",
	"operation":      "OPERATION PHASE",
}

// WorkspaceInfo is the workspace summary embedded in an initial state.
// It intentionally duplicates the four scalar values returned by the
// workspace package so this package does not depend on workspace.
type WorkspaceInfo struct {
	ProjectType string
	Languages   string
	Frameworks  string
	BuildSystem string
}

// Input contains the resolved inputs for initial state construction.
// ProjectDescription is the raw caller input written to the JSON sidecar;
// ProjectDescriptionPreview is a separately resolved, safe single-line value
// used only in aidlc-state.md.
type Input struct {
	Graph                     graph.Snapshot
	Scope                     string
	ScopeMetadata             scope.Metadata
	Workspace                 WorkspaceInfo
	ProjectRoot               string
	ProjectDescription        string
	ProjectDescriptionPreview string
	StartDate                 string
	DepthOverride             string
	TestStrategyOverride      string
	ReviewOverride            string
}

// StageRoute is one stage's initial routing decision.
type StageRoute struct {
	Slug   string
	Number string
	Action graph.Action
	Reason string
}

// Routing is the structured routing result used to author the state and to
// drive later workflow composition.
type Routing struct {
	Scope                               string
	EffectiveDepth                      string
	EffectiveTestStrategy               string
	ReviewOverride                      string
	ExecuteStages                       []StageRoute
	SkipStages                          []StageRoute
	FirstStage                          string
	NextStage                           string
	FirstPhase                          string
	FirstAgent                          string
	TotalStages                         int
	CompletedInitializationStages       int
	ReverseEngineeringSkippedGreenfield bool
	IncrementalGreenfieldWarning        bool
}

// Initial contains the canonical in-memory files and their structured
// routing metadata. It performs no filesystem or audit I/O.
type Initial struct {
	StateContent           string
	ProjectDescriptionJSON string
	Routing                Routing
	Plan                   stageplan.Plan
}

// BuildInitial builds the initial aidlc-state.md and project-description.json
// contents from already-resolved graph, scope, and workspace inputs.
func BuildInitial(input Input) (Initial, error) {
	if input.Scope == "" {
		return Initial{}, errors.New("build initial state: scope is required")
	}
	rawScope, ok := input.Graph.Scope(input.Scope)
	if !ok {
		return Initial{}, fmt.Errorf("build initial state: unknown scope %q", input.Scope)
	}
	if input.ScopeMetadata.Name != "" && input.ScopeMetadata.Name != input.Scope {
		return Initial{}, fmt.Errorf(
			"build initial state: scope metadata name %q does not match scope %q",
			input.ScopeMetadata.Name,
			input.Scope,
		)
	}
	effectiveDepth, err := resolveTier(
		input.DepthOverride,
		input.ScopeMetadata.Depth,
		"depth",
	)
	if err != nil {
		return Initial{}, err
	}
	effectiveTestStrategy, err := resolveTestStrategy(
		input.TestStrategyOverride,
		input.ScopeMetadata.TestStrategy,
		effectiveDepth,
	)
	if err != nil {
		return Initial{}, err
	}
	reviewOverride, err := resolveReviewOverride(input.ReviewOverride)
	if err != nil {
		return Initial{}, err
	}
	plan, err := stageplan.Build(stageplan.Input{
		Graph:       input.Graph,
		Scope:       input.Scope,
		ProjectType: input.Workspace.ProjectType,
	})
	if err != nil {
		return Initial{}, fmt.Errorf("build initial state: %w", err)
	}

	entries := plan.Entries()
	stages := make([]graph.Stage, len(entries))
	rawActions := make(map[string]graph.Action, len(stages))
	adjustedActions := make(map[string]graph.Action, len(stages))
	for index, entry := range entries {
		stage := entry.Stage
		stages[index] = stage
		action := rawScope.Action(stage.Slug)
		rawActions[stage.Slug] = action
		adjustedActions[stage.Slug] = entry.Action
	}

	routing := Routing{
		Scope:                 input.Scope,
		EffectiveDepth:        effectiveDepth,
		EffectiveTestStrategy: effectiveTestStrategy,
		ReviewOverride:        reviewOverride,
		ExecuteStages:         make([]StageRoute, 0, len(stages)),
		SkipStages:            make([]StageRoute, 0, len(stages)),
	}
	for _, entry := range entries {
		if entry.Reason == "greenfield" {
			continue
		}
		route := StageRoute{
			Slug:   entry.Stage.Slug,
			Number: entry.Stage.Number,
			Action: entry.Action,
		}
		if route.Action == graph.ActionExecute {
			routing.ExecuteStages = append(routing.ExecuteStages, route)
			continue
		}
		routing.SkipStages = append(routing.SkipStages, route)
	}

	if plan.ReverseEngineeringSkippedGreenfield() {
		for _, entry := range entries {
			if entry.Reason != "greenfield" {
				continue
			}
			routing.SkipStages = append(routing.SkipStages, StageRoute{
				Slug:   entry.Stage.Slug,
				Number: entry.Stage.Number,
				Action: entry.Action,
				Reason: entry.Reason,
			})
			break
		}
		routing.ReverseEngineeringSkippedGreenfield = true
		if isIncrementalScope(input.Scope) {
			routing.IncrementalGreenfieldWarning = true
		}
	}

	routing.FirstStage = firstPostInitializationStage(stages, adjustedActions)
	routing.NextStage = nextRawStage(stages, rawActions, routing.FirstStage)
	firstEntry := findStage(stages, routing.FirstStage)
	if firstEntry == nil {
		routing.FirstPhase = "IDEATION"
		routing.FirstAgent = "aidlc-product-agent"
	} else {
		routing.FirstPhase = strings.ToUpper(firstEntry.Phase)
		routing.FirstAgent = firstEntry.LeadAgent
	}
	routing.TotalStages = len(routing.ExecuteStages)
	for _, stage := range stages {
		if stage.Phase == "initialization" {
			routing.CompletedInitializationStages++
		}
	}

	stateContent := renderState(input, stages, adjustedActions, routing)
	rawDescription := input.ProjectDescription
	if rawDescription == "" {
		rawDescription = projectDescriptionValue
	}
	descriptionJSON, err := jsonStringifyString(rawDescription)
	if err != nil {
		return Initial{}, fmt.Errorf("build initial state: encode project description: %w", err)
	}
	descriptionJSON += "\n"

	return Initial{
		StateContent:           stateContent,
		ProjectDescriptionJSON: string(descriptionJSON),
		Routing:                routing,
		Plan:                   plan,
	}, nil
}

// jsonStringifyString uses encoding/json for JSON syntax and restores the
// five characters that JSON.stringify leaves unescaped but encoding/json
// escapes for HTML-safe output.
func jsonStringifyString(value string) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return restoreJSONStringEscapes(data), nil
}

func restoreJSONStringEscapes(data []byte) string {
	var builder strings.Builder
	builder.Grow(len(data))
	for index := 0; index < len(data); {
		if data[index] != '\\' {
			builder.WriteByte(data[index])
			index++
			continue
		}
		if index+1 >= len(data) {
			builder.WriteByte(data[index])
			index++
			continue
		}
		if data[index+1] == '\\' {
			builder.Write(data[index : index+2])
			index += 2
			continue
		}
		if data[index+1] == 'u' && index+6 <= len(data) {
			switch string(data[index : index+6]) {
			case `\u003c`:
				builder.WriteByte('<')
				index += 6
				continue
			case `\u003e`:
				builder.WriteByte('>')
				index += 6
				continue
			case `\u0026`:
				builder.WriteByte('&')
				index += 6
				continue
			case `\u2028`:
				builder.WriteRune('\u2028')
				index += 6
				continue
			case `\u2029`:
				builder.WriteRune('\u2029')
				index += 6
				continue
			}
		}
		builder.Write(data[index : index+2])
		index += 2
	}
	return builder.String()
}

func resolveTier(override, fallback, field string) (string, error) {
	value := fallback
	if override != "" {
		value = override
	}
	canonical, ok := canonicalTier(value)
	if !ok {
		return "", fmt.Errorf("build initial state: invalid %s %q", field, value)
	}
	return canonical, nil
}

func resolveTestStrategy(override, fallback, depth string) (string, error) {
	value := depth
	if fallback != "" {
		value = fallback
	}
	if override != "" {
		value = override
	}
	canonical, ok := canonicalTier(value)
	if !ok {
		return "", fmt.Errorf("build initial state: invalid test strategy %q", value)
	}
	return canonical, nil
}

func canonicalTier(value string) (string, bool) {
	switch strings.ToLower(value) {
	case "minimal":
		return "Minimal", true
	case "standard":
		return "Standard", true
	case "comprehensive":
		return "Comprehensive", true
	default:
		return "", false
	}
}

func resolveReviewOverride(value string) (string, error) {
	switch strings.ToLower(value) {
	case "":
		return "", nil
	case "adversarial":
		return "", nil
	case "advisory", "none":
		return strings.ToLower(value), nil
	default:
		return "", fmt.Errorf("build initial state: invalid review override %q", value)
	}
}

func isIncrementalScope(scopeName string) bool {
	switch scopeName {
	case "bugfix", "refactor", "security-patch":
		return true
	default:
		return false
	}
}

func firstPostInitializationStage(stages []graph.Stage, actions map[string]graph.Action) string {
	for _, stage := range stages {
		if stage.Phase != "initialization" && actions[stage.Slug] == graph.ActionExecute {
			return stage.Slug
		}
	}
	return "intent-capture"
}

func nextRawStage(stages []graph.Stage, actions map[string]graph.Action, first string) string {
	firstIndex := -1
	for index, stage := range stages {
		if stage.Slug == first {
			firstIndex = index
			break
		}
	}
	if firstIndex < 0 {
		return "none"
	}
	for _, stage := range stages[firstIndex+1:] {
		if actions[stage.Slug] == graph.ActionExecute {
			return stage.Slug
		}
	}
	return "none"
}

func findStage(stages []graph.Stage, slug string) *graph.Stage {
	for index := range stages {
		if stages[index].Slug == slug {
			return &stages[index]
		}
	}
	return nil
}

func renderState(
	input Input,
	stages []graph.Stage,
	actions map[string]graph.Action,
	routing Routing,
) string {
	projectDescription := input.ProjectDescriptionPreview
	if projectDescription == "" {
		projectDescription = projectDescriptionValue
	}

	var builder strings.Builder
	builder.WriteString("# AI-DLC State Tracking\n\n")
	builder.WriteString("## Project Information\n")
	fmt.Fprintf(&builder, "- **Project**: %s\n", projectDescription)
	fmt.Fprintf(&builder, "- **Project Description Source**: %s\n", projectDescriptionFile)
	fmt.Fprintf(&builder, "- **Project Type**: %s\n", input.Workspace.ProjectType)
	fmt.Fprintf(&builder, "- **Scope**: %s\n", input.Scope)
	fmt.Fprintf(&builder, "- **Start Date**: %s\n", input.StartDate)
	fmt.Fprintf(&builder, "- **State Version**: %s\n", currentStateVersion)
	fmt.Fprintf(&builder, "- **Active Agent**: %s\n", routing.FirstAgent)
	builder.WriteString("- **Worktree Path**:\n")
	builder.WriteString("- **Bolt Refs**:\n")
	builder.WriteString("- **Practices Affirmed Timestamp**:\n\n")

	builder.WriteString("## Scope Configuration\n")
	fmt.Fprintf(&builder, "- **Stages to Execute**: %s\n", stageNumbers(routing.ExecuteStages))
	fmt.Fprintf(&builder, "- **Stages to Skip**: %s\n", stageSkipValues(routing.SkipStages))
	fmt.Fprintf(&builder, "- **Depth**: %s\n", routing.EffectiveDepth)
	fmt.Fprintf(&builder, "- **Test Strategy**: %s\n", routing.EffectiveTestStrategy)
	fmt.Fprintf(&builder, "- **Review Override**: %s\n\n", routing.ReviewOverride)

	builder.WriteString("## Workspace State\n")
	fmt.Fprintf(&builder, "- **Project Root**: %s\n", input.ProjectRoot)
	fmt.Fprintf(&builder, "- **Languages**: %s\n", input.Workspace.Languages)
	fmt.Fprintf(&builder, "- **Frameworks**: %s\n", input.Workspace.Frameworks)
	fmt.Fprintf(&builder, "- **Build System**: %s\n\n", input.Workspace.BuildSystem)

	builder.WriteString("## Execution Plan Summary\n")
	fmt.Fprintf(&builder, "- **Total Stages**: %d\n", routing.TotalStages)
	fmt.Fprintf(&builder, "- **Completed**: %d\n", routing.CompletedInitializationStages)
	fmt.Fprintf(&builder, "- **In Progress**: %s\n\n", routing.FirstStage)

	builder.WriteString("## Runtime State\n")
	builder.WriteString("- **Revision Count**: 0\n\n")

	builder.WriteString("## Phase Progress\n")
	builder.WriteString("<!-- Status values: Pending, Active, Verified, Skipped -->\n\n")
	for _, phase := range phases {
		fmt.Fprintf(&builder, "- **%s**: %s\n", titlePhase(phase), phaseStatus(phase, stages, actions, routing.FirstStage))
	}
	builder.WriteString("\n## Stage Progress\n")
	builder.WriteString("<!-- Checkbox states: [ ] not started, [-] in progress, [?] awaiting approval (gate open), [R] revising (user rejected gate), [x] completed, [S] skipped via --stage/--phase jump -->\n")

	for _, phase := range phases {
		builder.WriteString("\n### ")
		builder.WriteString(phaseHeaders[phase])
		builder.WriteString("\n")
		if phase == "construction" {
			builder.WriteString("Per unit: [TBD]\n")
		}
		for _, stage := range stages {
			if stage.Phase != phase {
				continue
			}
			marker := "[ ]"
			if phase == "initialization" {
				marker = "[x]"
			} else if stage.Slug == routing.FirstStage {
				marker = "[-]"
			}
			action := actions[stage.Slug]
			label := "SKIP"
			if action == graph.ActionExecute {
				label = "EXECUTE"
			}
			fmt.Fprintf(&builder, "- %s %s — %s\n", marker, stage.Slug, label)
		}
	}

	builder.WriteString("\n")
	builder.WriteString("## Current Status\n")
	fmt.Fprintf(&builder, "- **Lifecycle Phase**: %s\n", routing.FirstPhase)
	fmt.Fprintf(&builder, "- **Current Stage**: %s\n", routing.FirstStage)
	fmt.Fprintf(&builder, "- **Next Stage**: %s\n", routing.NextStage)
	builder.WriteString("- **Status**: Running\n")
	fmt.Fprintf(&builder, "- **Last Updated**: %s\n\n", input.StartDate)

	builder.WriteString("## Session Resume Point\n")
	builder.WriteString("- **Last Completed Stage**: state-init\n")
	fmt.Fprintf(&builder, "- **Next Action**: Execute %s\n", routing.FirstStage)
	builder.WriteString("- **Pending Artifacts**: none\n")
	return builder.String()
}

func stageNumbers(routes []StageRoute) string {
	values := make([]string, len(routes))
	for index, route := range routes {
		values[index] = route.Number
	}
	return strings.Join(values, ", ")
}

func stageSkipValues(routes []StageRoute) string {
	if len(routes) == 0 {
		return "none"
	}
	values := make([]string, len(routes))
	for index, route := range routes {
		label := route.Slug
		if route.Reason != "" {
			label += " — " + route.Reason
		}
		values[index] = fmt.Sprintf("%s (%s)", route.Number, label)
	}
	return strings.Join(values, ", ")
}

func titlePhase(phase string) string {
	if phase == "initialization" {
		return "Initialization"
	}
	return strings.ToUpper(phase[:1]) + phase[1:]
}

func phaseStatus(
	phase string,
	stages []graph.Stage,
	actions map[string]graph.Action,
	firstStage string,
) string {
	if phase == "initialization" {
		return "Verified"
	}
	first := findStage(stages, firstStage)
	if first != nil && first.Phase == phase {
		return "Active"
	}
	for _, stage := range stages {
		if stage.Phase == phase && actions[stage.Slug] == graph.ActionExecute {
			return "Pending"
		}
	}
	return "Skipped"
}
