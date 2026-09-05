package delivery

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sori883/ai-dd/src/internal/artifact"
	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
	"github.com/sori883/ai-dd/src/internal/orchestrator"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
	"github.com/sori883/ai-dd/src/internal/steering"
)

// runStageWire is the required, ordered portion of a run-stage directive.
// Struct field order is the canonical JSON property order for this subset.
type runStageWire struct {
	Kind               string           `json:"kind"`
	Stage              string           `json:"stage"`
	Phase              string           `json:"phase"`
	LeadAgent          string           `json:"lead_agent"`
	SupportAgents      []string         `json:"support_agents"`
	Mode               string           `json:"mode"`
	InlineContextPaths []string         `json:"inline_context_paths"`
	Gate               bool             `json:"gate"`
	MemoryPath         string           `json:"memory_path"`
	Consumes           []string         `json:"consumes"`
	Produces           []string         `json:"produces"`
	RulesInContext     []string         `json:"rules_in_context"`
	SensorsApplicable  []string         `json:"sensors_applicable"`
	StageFile          string           `json:"stage_file"`
	ContextWarnings    []string         `json:"context_warnings,omitempty"`
	ConsumesAbsent     []runStageAbsent `json:"consumes_absent,omitempty"`
	NextStage          *string          `json:"next_stage"`
	ProtocolModules    []string         `json:"protocol_modules,omitempty"`
	ConductorPersona   *string          `json:"conductor_persona,omitempty"`
	Narration          string           `json:"narration"`
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
	presentation := buildRunStagePresentation(projectRoot, identity, stage, current, catalog)

	wire := runStageWire{
		Kind:               string(orchestrator.DirectiveKindRunStage),
		Stage:              stage.Slug,
		Phase:              stage.Phase,
		LeadAgent:          stage.LeadAgent,
		SupportAgents:      nonNilStrings(stage.SupportAgents),
		Mode:               stage.Mode,
		InlineContextPaths: nonNilStrings(roster.Paths),
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
		ContextWarnings:  nonNilStrings(roster.Warnings),
		ConsumesAbsent:   consumesAbsent,
		NextStage:        nextStage,
		ProtocolModules:  presentation.ProtocolModules,
		ConductorPersona: presentation.ConductorPersona,
		Narration:        presentation.Narration,
	}

	return marshalRunStageWire(wire)
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
