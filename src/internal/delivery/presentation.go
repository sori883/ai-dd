package delivery

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/recordlock"
	"github.com/sori883/ai-dd/src/internal/state"
)

type runStagePresentation struct {
	ProtocolModules  []string
	ConductorPersona *string
	Narration        string
}

func buildRunStagePresentation(projectRoot *os.Root, identity recordlock.Identity, stage graph.Stage, current state.State, catalog graph.Snapshot) runStagePresentation {
	firstSubstantive := isFirstSubstantiveStage(stage, current, catalog)
	var conductorPersona *string
	if firstSubstantive {
		conductorPersona = readOptionalConductorPersona(projectRoot)
	}
	return runStagePresentation{
		ProtocolModules:  runStageProtocolModules(stage),
		ConductorPersona: conductorPersona,
		Narration:        runStageNarration(identity, stage, current, catalog),
	}
}

func isFirstSubstantiveStage(stage graph.Stage, current state.State, catalog graph.Snapshot) bool {
	if stage.Phase == "initialization" {
		return false
	}
	phaseBySlug := make(map[string]string, len(catalog.Stages()))
	for _, candidate := range catalog.Stages() {
		phaseBySlug[candidate.Slug] = candidate.Phase
	}
	for _, progress := range current.Stages() {
		if progress.CheckboxState != state.CheckboxStateCompleted && progress.CheckboxState != state.CheckboxStateSkipped {
			continue
		}
		if phase, ok := phaseBySlug[progress.Slug]; ok && phase != "initialization" {
			return false
		}
	}
	return true
}

func readOptionalConductorPersona(projectRoot *os.Root) *string {
	if projectRoot == nil {
		return nil
	}
	data, err := fs.ReadFile(projectRoot.FS(), path.Join(".codex", "aidlc-common", "conductor.md"))
	if err != nil {
		return nil
	}
	value := strings.ToValidUTF8(string(data), "\uFFFD")
	return &value
}

func runStageProtocolModules(stage graph.Stage) []string {
	modules := make([]string, 0, 3)
	if stage.Reviewer != "" {
		modules = append(modules, "reviewer")
	}
	if stage.Mode == "subagent" || stage.Mode == "pipeline" || stage.Mode == "mob" || len(stage.SupportAgents) != 0 {
		modules = append(modules, "ensemble")
	}
	if stage.Phase == "construction" {
		modules = append(modules, "construction")
	}
	return modules
}

var runStageRoleOverrides = map[string]string{
	"product":               "product manager",
	"product lead":          "product lead",
	"design":                "designer",
	"delivery":              "delivery lead",
	"architect":             "architect",
	"architecture reviewer": "architecture reviewer",
	"aws platform":          "platform engineer",
	"compliance":            "compliance specialist",
	"devsecops":             "security engineer",
	"developer":             "developer",
	"quality":               "quality engineer",
	"pipeline deploy":       "release engineer",
	"operations":            "operations engineer",
}

func runStageNarration(identity recordlock.Identity, stage graph.Stage, current state.State, catalog graph.Snapshot) string {
	if stage.Mode == "subagent" || stage.Mode == "pipeline" {
		role := runStageRoleInWords(stage.LeadAgent)
		if role == "" {
			role = "specialist"
		}
		return fmt.Sprintf("Bringing in the %s to work on %s.", role, stage.Name)
	}
	if isFirstSubstantiveStage(stage, current, catalog) {
		return fmt.Sprintf(
			"Starting the %s plan for this project. First step is %s, and I will stop for your review before anything is final.",
			current.Scope(),
			stage.Name,
		)
	}
	if stage.Mode == "inline" || stage.Mode == "mob" {
		return fmt.Sprintf("Now working on %s, %s.", stage.Name, runStagePeopleClause(stage))
	}
	return fmt.Sprintf("Now working on %s.", stage.Name)
}

func runStageRoleInWords(agent string) string {
	agent = strings.TrimSpace(agent)
	if !strings.HasPrefix(agent, "aidlc-") || !strings.HasSuffix(agent, "-agent") {
		return ""
	}
	fragment := strings.TrimSuffix(strings.TrimPrefix(agent, "aidlc-"), "-agent")
	if fragment == "" {
		return ""
	}
	fragment = strings.ReplaceAll(fragment, "-", " ")
	if role, ok := runStageRoleOverrides[fragment]; ok {
		return role
	}
	return fragment
}

func runStagePeopleClause(stage graph.Stage) string {
	lead := runStageRoleInWords(stage.LeadAgent)
	if lead == "" {
		leadClause := fmt.Sprintf("in the %s phase", strings.ToLower(stage.Phase))
		if len(stage.SupportAgents) == 0 {
			return leadClause
		}
		return leadClause + ", " + runStageSupportClause(stage.SupportAgents)
	}
	leadClause := fmt.Sprintf("wearing the %s hat", lead)
	if len(stage.SupportAgents) == 0 {
		return leadClause
	}
	return leadClause + ", " + runStageSupportClause(stage.SupportAgents)
}

func runStageSupportClause(agents []string) string {
	roles := make([]string, 0, len(agents))
	for _, agent := range agents {
		role := runStageRoleInWords(agent)
		if role == "" {
			role = "specialist"
		}
		roles = append(roles, role)
	}
	switch len(roles) {
	case 0:
		return ""
	case 1:
		return "with the " + roles[0] + " on hand"
	case 2:
		return "with the " + roles[0] + " and " + roles[1] + " on hand"
	default:
		return "with the " + strings.Join(roles[:len(roles)-1], ", ") + ", and " + roles[len(roles)-1] + " on hand"
	}
}
