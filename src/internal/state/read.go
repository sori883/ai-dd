package state

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	stateHeader = "# AI-DLC State Tracking"
	emDash      = "—"
)

var requiredSections = [...]string{
	"Project Information",
	"Execution Plan Summary",
	"Phase Progress",
	"Stage Progress",
	"Current Status",
}

var phaseDefinitions = [...]struct {
	display string
	value   LifecyclePhase
}{
	{display: "Initialization", value: LifecyclePhaseInitialization},
	{display: "Ideation", value: LifecyclePhaseIdeation},
	{display: "Inception", value: LifecyclePhaseInception},
	{display: "Construction", value: LifecyclePhaseConstruction},
	{display: "Operation", value: LifecyclePhaseOperation},
}

// Read reads and parses the fixed aidlc-state.md leaf beneath recordRoot.
// The caller owns recordRoot; Read never closes it. Read leaves the state
// bytes, mode, and mtime unchanged, but does not guarantee that an ordinary
// read leaves atime unchanged.
func Read(recordRoot *os.Root) (State, error) {
	document, err := ReadDocument(recordRoot)
	if err != nil {
		return State{}, err
	}
	return document.State, nil
}

// Parse validates a State Version 8 canonical Markdown document and returns a
// typed snapshot. The input bytes are not retained.
func Parse(content []byte) (State, error) {
	if !utf8.Valid(content) {
		return State{}, invalidState("content is not valid utf-8")
	}
	if len(content) >= 3 && content[0] == 0xef && content[1] == 0xbb && content[2] == 0xbf {
		content = content[3:]
	}
	if err := validateNewlines(content); err != nil {
		return State{}, err
	}

	text := string(content)
	lines := strings.Split(text, "\n")
	for index := range lines {
		lines[index] = strings.TrimSuffix(lines[index], "\r")
	}
	if len(lines) == 0 || lines[0] != stateHeader {
		return State{}, invalidState("first line must be %q", stateHeader)
	}

	sections, err := collectSections(lines[1:])
	if err != nil {
		return State{}, err
	}

	projectType, err := requiredStringField(sections["Project Information"], "Project Type")
	if err != nil {
		return State{}, err
	}
	scope, err := requiredStringField(sections["Project Information"], "Scope")
	if err != nil {
		return State{}, err
	}
	versionText, err := requiredStringField(sections["Project Information"], "State Version")
	if err != nil {
		return State{}, err
	}
	version, err := parseStateVersion(versionText)
	if err != nil {
		return State{}, err
	}

	totalText, err := requiredStringField(sections["Execution Plan Summary"], "Total Stages")
	if err != nil {
		return State{}, err
	}
	totalStages, err := parseCanonicalNonNegativeInt(totalText, "Total Stages")
	if err != nil {
		return State{}, err
	}
	completedText, err := requiredStringField(sections["Execution Plan Summary"], "Completed")
	if err != nil {
		return State{}, err
	}
	completed, err := parseCanonicalNonNegativeInt(completedText, "Completed")
	if err != nil {
		return State{}, err
	}
	inProgress, err := requiredStringField(sections["Execution Plan Summary"], "In Progress")
	if err != nil {
		return State{}, err
	}

	phaseText, err := requiredStringField(sections["Current Status"], "Lifecycle Phase")
	if err != nil {
		return State{}, err
	}
	lifecyclePhase, ok := parseLifecyclePhase(phaseText)
	if !ok {
		return State{}, invalidState("invalid Lifecycle Phase %q", phaseText)
	}
	currentStage, err := requiredStringField(sections["Current Status"], "Current Stage")
	if err != nil {
		return State{}, err
	}
	nextStage, err := requiredStringField(sections["Current Status"], "Next Stage")
	if err != nil {
		return State{}, err
	}
	statusText, err := requiredStringField(sections["Current Status"], "Status")
	if err != nil {
		return State{}, err
	}
	workflowStatus, ok := parseWorkflowStatus(statusText)
	if !ok {
		return State{}, invalidState("invalid Status %q", statusText)
	}

	phases, err := parsePhaseProgress(sections["Phase Progress"])
	if err != nil {
		return State{}, err
	}
	stages, err := parseStageProgress(sections["Stage Progress"])
	if err != nil {
		return State{}, err
	}

	return State{
		version:        version,
		scope:          scope,
		projectType:    projectType,
		workflowStatus: workflowStatus,
		lifecyclePhase: lifecyclePhase,
		currentStage:   currentStage,
		nextStage:      nextStage,
		summary: Summary{
			TotalStages: totalStages,
			Completed:   completed,
			InProgress:  inProgress,
		},
		phaseProgress: phases,
		stages:        stages,
	}, nil
}

func invalidState(format string, args ...any) error {
	return fmt.Errorf("parse state: %w: %s", fs.ErrInvalid, fmt.Sprintf(format, args...))
}

func validateNewlines(content []byte) error {
	for index, value := range content {
		if value != '\r' {
			continue
		}
		if index+1 >= len(content) || content[index+1] != '\n' {
			return invalidState("invalid carriage return at byte %d", index)
		}
	}
	return nil
}

func collectSections(lines []string) (map[string][]string, error) {
	sections := make(map[string][]string, len(requiredSections))
	required := make(map[string]struct{}, len(requiredSections))
	for _, name := range requiredSections {
		required[name] = struct{}{}
	}
	current := ""
	seen := make(map[string]bool, len(requiredSections))
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimPrefix(line, "## ")
			if _, ok := required[current]; ok {
				if seen[current] {
					return nil, invalidState("duplicate section %q", current)
				}
				seen[current] = true
				sections[current] = make([]string, 0)
				continue
			}
			sections[current] = make([]string, 0)
			continue
		}
		if current != "" {
			sections[current] = append(sections[current], line)
		}
	}
	for _, name := range requiredSections {
		if !seen[name] {
			return nil, invalidState("missing section %q", name)
		}
	}
	return sections, nil
}

func requiredStringField(lines []string, label string) (string, error) {
	prefix := "- **" + label + "**:"
	value := ""
	found := false
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if found {
			return "", invalidState("duplicate field %q", label)
		}
		found = true
		value = strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	if !found {
		return "", invalidState("missing field %q", label)
	}
	if value == "" {
		return "", invalidState("empty field %q", label)
	}
	return value, nil
}

func parseStateVersion(value string) (int, error) {
	if value != "8" {
		return 0, invalidState("State Version must be the bare token 8")
	}
	return 8, nil
}

func parseCanonicalNonNegativeInt(value, label string) (int, error) {
	if value == "" {
		return 0, invalidState("empty field %q", label)
	}
	if value != "0" && value[0] == '0' {
		return 0, invalidState("field %q is not canonical decimal", label)
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, invalidState("field %q is not canonical decimal", label)
		}
	}
	parsed, err := strconv.ParseInt(value, 10, strconv.IntSize)
	if err != nil {
		return 0, invalidState("field %q overflows int", label)
	}
	return int(parsed), nil
}

func parseWorkflowStatus(value string) (WorkflowStatus, bool) {
	switch WorkflowStatus(value) {
	case WorkflowStatusRunning, WorkflowStatusCompleted:
		return WorkflowStatus(value), true
	default:
		return WorkflowStatusUnknown, false
	}
}

func parseLifecyclePhase(value string) (LifecyclePhase, bool) {
	switch LifecyclePhase(value) {
	case LifecyclePhaseInitialization,
		LifecyclePhaseIdeation,
		LifecyclePhaseInception,
		LifecyclePhaseConstruction,
		LifecyclePhaseOperation:
		return LifecyclePhase(value), true
	default:
		return LifecyclePhaseUnknown, false
	}
}

func parsePhaseProgress(lines []string) ([]PhaseProgress, error) {
	phases := make([]PhaseProgress, 0, len(phaseDefinitions))
	seen := make(map[LifecyclePhase]bool, len(phaseDefinitions))
	for _, line := range lines {
		for _, definition := range phaseDefinitions {
			prefix := "- **" + definition.display + "**:"
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			if seen[definition.value] {
				return nil, invalidState("duplicate phase %q", definition.display)
			}
			statusText := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			status, ok := parsePhaseStatus(statusText)
			if !ok {
				return nil, invalidState("invalid status for phase %q: %q", definition.display, statusText)
			}
			seen[definition.value] = true
			phases = append(phases, PhaseProgress{Phase: definition.value, Status: status})
		}
	}
	if len(phases) != len(phaseDefinitions) {
		return nil, invalidState("Phase Progress must contain each canonical phase exactly once")
	}
	for index, definition := range phaseDefinitions {
		if phases[index].Phase != definition.value {
			return nil, invalidState("Phase Progress is out of canonical order")
		}
	}
	return phases, nil
}

func parsePhaseStatus(value string) (PhaseStatus, bool) {
	switch PhaseStatus(value) {
	case PhaseStatusPending, PhaseStatusActive, PhaseStatusVerified, PhaseStatusSkipped:
		return PhaseStatus(value), true
	default:
		return PhaseStatusUnknown, false
	}
}

func parseStageProgress(lines []string) ([]StageProgress, error) {
	stages := make([]StageProgress, 0)
	seen := make(map[string]struct{})
	for _, line := range lines {
		if !strings.HasPrefix(line, "- [") {
			continue
		}
		stage, err := parseStageRow(line)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[stage.Slug]; exists {
			return nil, invalidState("duplicate stage slug %q", stage.Slug)
		}
		seen[stage.Slug] = struct{}{}
		stages = append(stages, stage)
	}
	if len(stages) == 0 {
		return nil, invalidState("Stage Progress must contain at least one stage")
	}
	return stages, nil
}

func parseStageRow(line string) (StageProgress, error) {
	if len(line) < len("- [x]") {
		return StageProgress{}, invalidState("malformed stage row %q", line)
	}
	marker := line[2:5]
	checkboxState, ok := checkboxStateForMarker(marker)
	if !ok {
		return StageProgress{}, invalidState("unknown checkbox marker %q", marker)
	}
	rest := line[5:]
	if rest == "" {
		return StageProgress{}, invalidState("malformed stage row %q", line)
	}
	firstRune, _ := utf8.DecodeRuneInString(rest)
	if !isHorizontalWhitespace(firstRune) {
		return StageProgress{}, invalidState("stage row must separate marker and slug with horizontal whitespace")
	}
	rest = strings.TrimLeftFunc(rest, isHorizontalWhitespace)
	candidates, suffixEnd := collectStageRowCandidates(rest)
	if len(candidates) == 0 {
		return StageProgress{}, invalidState("stage row is missing em dash separator")
	}

	var (
		validSlug   bool
		validSuffix bool
	)
	for index := len(candidates) - 1; index >= 0; index-- {
		candidate := candidates[index]
		if !candidate.slugValid {
			continue
		}
		validSlug = true

		if candidate.suffixStart >= suffixEnd {
			continue
		}
		validSuffix = true
		suffix := rest[candidate.suffixStart:suffixEnd]
		planAction, ok := parsePlanActionPrefix(suffix)
		if !ok {
			continue
		}
		slug := strings.TrimFunc(rest[:candidate.dashIndex], isHorizontalWhitespace)
		return StageProgress{
			Slug:           slug,
			CheckboxMarker: marker,
			CheckboxState:  checkboxState,
			Suffix:         suffix,
			PlanAction:     planAction,
		}, nil
	}
	if !validSlug {
		return StageProgress{}, invalidState("stage slug must be nonempty and contain no whitespace")
	}
	if !validSuffix {
		return StageProgress{}, invalidState("stage suffix must be nonempty")
	}
	return StageProgress{}, invalidState("stage suffix must begin with exact EXECUTE or SKIP token")
}

func checkboxStateForMarker(marker string) (CheckboxState, bool) {
	switch marker {
	case "[ ]":
		return CheckboxStatePending, true
	case "[-]":
		return CheckboxStateInProgress, true
	case "[?]":
		return CheckboxStateAwaitingApproval, true
	case "[R]":
		return CheckboxStateRevising, true
	case "[x]":
		return CheckboxStateCompleted, true
	case "[S]":
		return CheckboxStateSkipped, true
	default:
		return CheckboxStateUnknown, false
	}
}

func parsePlanActionPrefix(value string) (PlanAction, bool) {
	for _, candidate := range []struct {
		word   string
		action PlanAction
	}{
		{word: string(PlanActionExecute), action: PlanActionExecute},
		{word: string(PlanActionSkip), action: PlanActionSkip},
	} {
		if !strings.HasPrefix(value, candidate.word) {
			continue
		}
		if len(value) == len(candidate.word) || !isASCIIWordContinuation(value[len(candidate.word)]) {
			return candidate.action, true
		}
	}
	return PlanActionUnknown, false
}

type stageRowCandidate struct {
	dashIndex   int
	suffixStart int
	slugValid   bool
}

func collectStageRowCandidates(rest string) ([]stageRowCandidate, int) {
	suffixEnd := len(rest)
	for suffixEnd > 0 {
		value, size := utf8.DecodeLastRuneInString(rest[:suffixEnd])
		if !unicode.IsSpace(value) {
			break
		}
		suffixEnd -= size
	}

	candidates := make([]stageRowCandidate, 0)
	var (
		slugHasContent      bool
		slugInvalid         bool
		pendingHorizontalWS bool
	)
	for index := 0; index < len(rest); {
		value, size := utf8.DecodeRuneInString(rest[index:])
		if value == '—' {
			suffixStart := index + size
			for suffixStart < len(rest) {
				suffixValue, suffixSize := utf8.DecodeRuneInString(rest[suffixStart:])
				if !unicode.IsSpace(suffixValue) {
					break
				}
				suffixStart += suffixSize
			}
			candidates = append(candidates, stageRowCandidate{
				dashIndex:   index,
				suffixStart: suffixStart,
				slugValid:   slugHasContent && !slugInvalid,
			})

			if pendingHorizontalWS && slugHasContent {
				slugInvalid = true
			}
			pendingHorizontalWS = false
			slugHasContent = true
			index += size
			continue
		}
		if unicode.IsSpace(value) {
			if !isHorizontalWhitespace(value) {
				slugInvalid = true
			}
			if slugHasContent {
				pendingHorizontalWS = true
			}
			index += size
			continue
		}
		if pendingHorizontalWS && slugHasContent {
			slugInvalid = true
		}
		pendingHorizontalWS = false
		slugHasContent = true
		index += size
	}
	return candidates, suffixEnd
}

func isASCIIWordContinuation(value byte) bool {
	return value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '_'
}

func isHorizontalWhitespace(value rune) bool {
	return value == ' ' || value == '\t' || unicode.Is(unicode.Zs, value)
}
