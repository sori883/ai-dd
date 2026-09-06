package delivery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/scope"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/steering"
)

// runStageWire is the required, ordered portion of a run-stage directive.
// Struct field order is the canonical JSON property order for this subset.
type runStageWire struct {
	Kind                  string           `json:"kind"`
	Stage                 string           `json:"stage"`
	Phase                 string           `json:"phase"`
	LeadAgent             string           `json:"lead_agent"`
	SupportAgents         []string         `json:"support_agents"`
	Mode                  string           `json:"mode"`
	InlineContextPaths    []string         `json:"inline_context_paths"`
	Gate                  bool             `json:"gate"`
	MemoryPath            string           `json:"memory_path"`
	Consumes              []string         `json:"consumes"`
	Produces              []string         `json:"produces"`
	RulesInContext        []string         `json:"rules_in_context"`
	SensorsApplicable     []string         `json:"sensors_applicable"`
	StageFile             string           `json:"stage_file"`
	ContextWarnings       []string         `json:"context_warnings,omitempty"`
	ConsumesAbsent        []runStageAbsent `json:"consumes_absent,omitempty"`
	NextStage             *string          `json:"next_stage"`
	Reviewer              string           `json:"reviewer,omitempty"`
	ReviewArtifact        string           `json:"review_artifact,omitempty"`
	ReviewClass           string           `json:"review_class,omitempty"`
	ReviewerMaxIterations int              `json:"reviewer_max_iterations,omitempty"`
	ProtocolModules       []string         `json:"protocol_modules,omitempty"`
	ConductorPersona      *string          `json:"conductor_persona,omitempty"`
	Narration             string           `json:"narration"`
}

type runStageAbsent struct {
	Path     string `json:"path"`
	Expected bool   `json:"expected"`
}

func buildRunStageWire(identity recordlock.Identity, stage graph.Stage, current state.State, catalog graph.Snapshot, projectRoot, recordRoot *os.Root, rules []steering.RuleContent, roster knowledge.Roster) ([]byte, error) {
	nextStage, err := nextStageName(current.NextStage(), catalog)
	if err != nil {
		return nil, err
	}
	resolved, err := artifact.ResolvePaths(stage, catalog, current.ProjectType())
	if err != nil {
		return nil, fmt.Errorf("resolve artifact paths: %w", err)
	}
	recordPrefix := path.Join("aidlc", "spaces", identity.Space(), "intents", identity.Intent())
	consumes := make([]string, 0, len(resolved.Consumes))
	consumesAbsent := make([]runStageAbsent, 0)
	for _, consume := range resolved.Consumes {
		wirePath := path.Join(recordPrefix, consume.Path)
		if regularRecordFile(recordRoot, consume.Path) {
			consumes = append(consumes, wirePath)
			continue
		}
		if consume.Required {
			expected, err := missingConsumeExpected(catalog, current, consume.Artifact)
			if err != nil {
				return nil, err
			}
			consumesAbsent = append(consumesAbsent, runStageAbsent{
				Path:     wirePath,
				Expected: expected,
			})
		}
	}
	produces := make([]string, 0, len(resolved.Produces))
	for _, produce := range resolved.Produces {
		produces = append(produces, path.Join(recordPrefix, produce))
	}
	reviewer, reviewArtifact, reviewClass, reviewerMaxIterations, err := resolveRunStageReview(projectRoot, stage, current)
	if err != nil {
		return nil, fmt.Errorf("resolve review contract: %w", err)
	}
	presentationStage := stage
	if reviewer == "" {
		presentationStage.Reviewer = ""
	}
	presentation := buildRunStagePresentation(projectRoot, identity, presentationStage, current, catalog)
	if roster.HasOKF {
		presentation.Narration += okfNarration
	}
	inlineContextPaths, err := intentCaptureInlineContextPaths(projectRoot, recordRoot, identity, stage, reviewer, roster.Paths)
	if err != nil {
		return nil, fmt.Errorf("resolve inline intent-capture context: %w", err)
	}

	wire := runStageWire{
		Kind:               string(orchestrator.DirectiveKindRunStage),
		Stage:              stage.Slug,
		Phase:              stage.Phase,
		LeadAgent:          stage.LeadAgent,
		SupportAgents:      nonNilStrings(stage.SupportAgents),
		Mode:               stage.Mode,
		InlineContextPaths: inlineContextPaths,
		Gate:               true,
		MemoryPath: path.Join(
			"aidlc",
			"spaces",
			identity.Space(),
			"intents",
			identity.Intent(),
			stage.Phase,
			stage.Slug,
			"memory.md",
		),
		Consumes:          consumes,
		Produces:          produces,
		RulesInContext:    nonNilRuleContentPaths(rules),
		SensorsApplicable: nonNilStrings(stage.Sensors),
		StageFile: path.Join(
			".codex",
			"aidlc-common",
			"stages",
			stage.Phase,
			stage.Slug+".md",
		),
		ContextWarnings:       nonNilStrings(roster.Warnings),
		ConsumesAbsent:        consumesAbsent,
		NextStage:             nextStage,
		Reviewer:              reviewer,
		ReviewArtifact:        reviewArtifact,
		ReviewClass:           string(reviewClass),
		ReviewerMaxIterations: reviewerMaxIterations,
		ProtocolModules:       presentation.ProtocolModules,
		ConductorPersona:      presentation.ConductorPersona,
		Narration:             presentation.Narration,
	}

	return marshalRunStageWire(wire)
}

func intentCaptureInlineContextPaths(projectRoot, recordRoot *os.Root, identity recordlock.Identity, stage graph.Stage, reviewer string, paths []string) ([]string, error) {
	result := nonNilStrings(paths)
	if !requiresIntentCaptureInlineContext(stage) {
		return result, nil
	}
	appendRequiredProjectPath := func(name string) error {
		if !regularProjectFile(projectRoot, name) {
			return fmt.Errorf("required inline context %q is missing or not a regular non-symlink file", name)
		}
		if !containsString(result, name) {
			result = append(result, name)
		}
		return nil
	}
	if err := appendRequiredProjectPath(path.Join(".codex", "aidlc-common", "protocols", "stage-protocol.md")); err != nil {
		return nil, err
	}
	if reviewer != "" {
		if err := appendRequiredProjectPath(path.Join(".codex", "aidlc-common", "protocols", "stage-protocol-reviewer.md")); err != nil {
			return nil, err
		}
	}
	if stage.Mode == "mob" || stage.Mode == "pipeline" || len(stage.SupportAgents) != 0 {
		if err := appendRequiredProjectPath(path.Join(".codex", "aidlc-common", "protocols", "stage-protocol-ensemble.md")); err != nil {
			return nil, err
		}
	}
	if err := appendRequiredProjectPath(path.Join(".codex", "skills", "aidlc", "question-rendering.md")); err != nil {
		return nil, err
	}
	if !regularRecordFile(recordRoot, "project-description.json") {
		return nil, errors.New("required inline context project-description.json is missing or not a regular non-symlink file")
	}
	name := path.Join("aidlc", "spaces", identity.Space(), "intents", identity.Intent(), "project-description.json")
	if !containsString(result, name) {
		result = append(result, name)
	}
	return result, nil
}

func requiresIntentCaptureInlineContext(stage graph.Stage) bool {
	return stage.Enabled && stage.Slug == "intent-capture" && stage.Phase == "ideation" && stage.Execution == "ALWAYS" && stage.Mode == "inline" &&
		stage.LeadAgent == "aidlc-product-agent" && slices.Equal(stage.SupportAgents, []string{"aidlc-architect-agent"}) &&
		slices.Equal(stage.Scopes, []string{"enterprise", "feature", "mvp", "poc"}) && stage.Reviewer == "aidlc-product-lead-agent" &&
		stage.ReviewArtifact == "intent-statement" && stage.ReviewerMaxIterations == 2 && stage.ReviewClass == graph.ReviewClassAdvisory &&
		stage.SummaryConfirmation == "required" && slices.Equal(stage.Sensors, []string{"claim-sources", "required-sections", "upstream-coverage"}) &&
		slices.Equal(stage.Produces, []string{"intent-statement", "stakeholder-map", "intent-capture-questions"}) &&
		len(stage.OptionalProduces) == 0 && len(stage.Consumes) == 0 && len(stage.RequiresStages) == 0 && stage.ProducesKinds == nil
}

func regularProjectFile(projectRoot *os.Root, name string) bool {
	if projectRoot == nil {
		return false
	}
	info, err := projectRoot.Lstat(name)
	return err == nil && info != nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func resolveRunStageReview(projectRoot *os.Root, stage graph.Stage, current state.State) (string, string, graph.ReviewClass, int, error) {
	if stage.Reviewer == "" {
		return "", "", graph.ReviewClassNone, 0, nil
	}
	declared := stage.ReviewClass
	if declared == "" {
		declared = graph.ReviewClassAdversarial
	}
	cap := scope.ReviewCapAdversarial
	if projectRoot != nil {
		scopesFS, err := fs.Sub(projectRoot.FS(), path.Join(".codex", "scopes"))
		if err == nil {
			metadata, err := scope.ReadAll(scopesFS)
			if err != nil {
				return "", "", graph.ReviewClassNone, 0, fmt.Errorf("read active scope metadata: %w", err)
			}
			for _, item := range metadata {
				if item.Name == current.Scope() && item.ReviewCap != "" {
					cap = item.ReviewCap
					break
				}
			}
		}
	}
	effective, maxIterations := scope.ResolveReviewPolicy(declared, cap, current.ReviewOverride(), stage.ReviewerMaxIterations)
	if effective == graph.ReviewClassNone {
		return "", "", graph.ReviewClassNone, 0, nil
	}
	return stage.Reviewer, stage.ReviewArtifact, effective, maxIterations, nil
}

func marshalRunStageWire(wire runStageWire) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString(`{"kind":`)
	appendRunStageJSONString(&builder, wire.Kind)
	builder.WriteString(`,"stage":`)
	appendRunStageJSONString(&builder, wire.Stage)
	builder.WriteString(`,"phase":`)
	appendRunStageJSONString(&builder, wire.Phase)
	builder.WriteString(`,"lead_agent":`)
	appendRunStageJSONString(&builder, wire.LeadAgent)
	builder.WriteString(`,"support_agents":`)
	appendRunStageStringArray(&builder, wire.SupportAgents)
	builder.WriteString(`,"mode":`)
	appendRunStageJSONString(&builder, wire.Mode)
	builder.WriteString(`,"inline_context_paths":`)
	appendRunStageStringArray(&builder, wire.InlineContextPaths)
	builder.WriteString(`,"gate":true`)
	builder.WriteString(`,"memory_path":`)
	appendRunStageJSONString(&builder, wire.MemoryPath)
	builder.WriteString(`,"consumes":`)
	appendRunStageStringArray(&builder, wire.Consumes)
	builder.WriteString(`,"produces":`)
	appendRunStageStringArray(&builder, wire.Produces)
	builder.WriteString(`,"rules_in_context":`)
	appendRunStageStringArray(&builder, wire.RulesInContext)
	builder.WriteString(`,"sensors_applicable":`)
	appendRunStageStringArray(&builder, wire.SensorsApplicable)
	builder.WriteString(`,"stage_file":`)
	appendRunStageJSONString(&builder, wire.StageFile)
	if len(wire.ContextWarnings) != 0 {
		builder.WriteString(`,"context_warnings":`)
		appendRunStageStringArray(&builder, wire.ContextWarnings)
	}
	if len(wire.ConsumesAbsent) != 0 {
		builder.WriteString(`,"consumes_absent":[`)
		for index, absent := range wire.ConsumesAbsent {
			if index != 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(`{"path":`)
			appendRunStageJSONString(&builder, absent.Path)
			builder.WriteString(`,"expected":`)
			if absent.Expected {
				builder.WriteString("true")
			} else {
				builder.WriteString("false")
			}
			builder.WriteByte('}')
		}
		builder.WriteByte(']')
	}
	builder.WriteString(`,"next_stage":`)
	if wire.NextStage == nil {
		builder.WriteString("null")
	} else {
		appendRunStageJSONString(&builder, *wire.NextStage)
	}
	if wire.Reviewer != "" {
		builder.WriteString(`,"reviewer":`)
		appendRunStageJSONString(&builder, wire.Reviewer)
		builder.WriteString(`,"review_artifact":`)
		appendRunStageJSONString(&builder, wire.ReviewArtifact)
		builder.WriteString(`,"review_class":`)
		appendRunStageJSONString(&builder, wire.ReviewClass)
		builder.WriteString(`,"reviewer_max_iterations":`)
		builder.WriteString(strconv.Itoa(wire.ReviewerMaxIterations))
	}
	if len(wire.ProtocolModules) != 0 {
		builder.WriteString(`,"protocol_modules":`)
		appendRunStageStringArray(&builder, wire.ProtocolModules)
	}
	if wire.ConductorPersona != nil {
		builder.WriteString(`,"conductor_persona":`)
		appendRunStageJSONString(&builder, *wire.ConductorPersona)
	}
	builder.WriteString(`,"narration":`)
	appendRunStageJSONString(&builder, wire.Narration)
	builder.WriteByte('}')
	return []byte(builder.String()), nil
}

func appendRunStageStringArray(builder *strings.Builder, values []string) {
	builder.WriteByte('[')
	for index, value := range values {
		if index != 0 {
			builder.WriteByte(',')
		}
		appendRunStageJSONString(builder, value)
	}
	builder.WriteByte(']')
}

func appendRunStageJSONString(builder *strings.Builder, value string) {
	const hexDigits = "0123456789abcdef"

	builder.WriteByte('"')
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if value[index] < 0x20 {
				builder.WriteString(`\u00`)
				builder.WriteByte(hexDigits[value[index]>>4])
				builder.WriteByte(hexDigits[value[index]&0x0f])
				continue
			}
			builder.WriteByte(value[index])
		}
	}
	builder.WriteByte('"')
}

func regularRecordFile(recordRoot *os.Root, name string) bool {
	if recordRoot == nil {
		return false
	}
	info, err := recordRoot.Lstat(name)
	if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return info.Mode().IsRegular()
}

func missingConsumeExpected(catalog graph.Snapshot, current state.State, artifactName string) (bool, error) {
	scope, ok := catalog.Scope(current.Scope())
	if !ok {
		return true, nil
	}
	progress := make(map[string]state.CheckboxState)
	for _, row := range current.Stages() {
		progress[row.Slug] = row.CheckboxState
	}
	for _, candidate := range catalog.Stages() {
		if !stageProduces(candidate, artifactName) || scope.Action(candidate.Slug) != graph.ActionExecute {
			continue
		}
		if progress[candidate.Slug] == state.CheckboxStateSkipped {
			return false, fmt.Errorf(
				"required consume %q has skipped producer %q without conditional-runtime provenance: %w",
				artifactName,
				candidate.Slug,
				ErrUnsupportedConsumeProvenance,
			)
		}
		return false, nil
	}
	return true, nil
}

func stageProduces(stage graph.Stage, artifactName string) bool {
	for _, produce := range stage.Produces {
		if produce == artifactName {
			return true
		}
	}
	for _, produce := range stage.OptionalProduces {
		if produce == artifactName {
			return true
		}
	}
	return false
}

func nextStageName(slug string, catalog graph.Snapshot) (*string, error) {
	if slug == "none" {
		return nil, nil
	}
	if slug == "" {
		return nil, fmt.Errorf("next stage slug is empty")
	}
	for _, stage := range catalog.Stages() {
		if stage.Slug == slug {
			name := stage.Name
			return &name, nil
		}
	}
	return nil, fmt.Errorf("next stage %q is absent from graph", slug)
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func nonNilRulePaths(values []graph.Rule) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		paths = append(paths, value.Path)
	}
	return paths
}

func nonNilRuleContentPaths(values []steering.RuleContent) []string {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		paths = append(paths, value.Path)
	}
	return paths
}
