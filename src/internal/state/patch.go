package state

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CanonicalField identifies a lifecycle-owned field that may be patched.
// Fields outside this allowlist are intentionally not patchable.
type CanonicalField string

const (
	// CanonicalFieldUnknown is the zero value and is not patchable.
	CanonicalFieldUnknown CanonicalField = ""
	// CanonicalFieldTotalStages identifies the workflow stage count field.
	CanonicalFieldTotalStages CanonicalField = "Total Stages"
	// CanonicalFieldCompleted identifies the completed stage count field.
	CanonicalFieldCompleted CanonicalField = "Completed"
	// CanonicalFieldInProgress identifies the current summary stage field.
	CanonicalFieldInProgress CanonicalField = "In Progress"
	// CanonicalFieldLifecyclePhase identifies the current lifecycle phase field.
	CanonicalFieldLifecyclePhase CanonicalField = "Lifecycle Phase"
	// CanonicalFieldCurrentStage identifies the current stage field.
	CanonicalFieldCurrentStage CanonicalField = "Current Stage"
	// CanonicalFieldNextStage identifies the next stage field.
	CanonicalFieldNextStage CanonicalField = "Next Stage"
	// CanonicalFieldStatus identifies the workflow status field.
	CanonicalFieldStatus CanonicalField = "Status"
	// CanonicalFieldLastUpdated identifies the last-updated field.
	CanonicalFieldLastUpdated CanonicalField = "Last Updated"
	// CanonicalFieldRevisionCount identifies the runtime revision count field.
	CanonicalFieldRevisionCount CanonicalField = "Revision Count"
	// CanonicalFieldActiveAgent identifies the current lead agent field.
	CanonicalFieldActiveAgent CanonicalField = "Active Agent"
	// CanonicalFieldLastCompletedStage identifies the resume-point stage field.
	CanonicalFieldLastCompletedStage CanonicalField = "Last Completed Stage"
	// CanonicalFieldNextAction identifies the resume-point action field.
	CanonicalFieldNextAction CanonicalField = "Next Action"
)

// FieldPatch replaces one typed canonical field value after checking its
// expected current value.
type FieldPatch struct {
	Field       CanonicalField
	Expected    string
	Replacement string
}

// PhaseProgressPatch replaces one canonical phase status after checking its
// expected current value.
type PhaseProgressPatch struct {
	Phase       LifecyclePhase
	Expected    PhaseStatus
	Replacement PhaseStatus
}

// StageMarker is one of the six canonical checkbox markers in Stage Progress.
type StageMarker string

const (
	// StageMarkerUnknown is the zero value and is not a valid marker.
	StageMarkerUnknown StageMarker = ""
	// StageMarkerPending identifies a not-started stage.
	StageMarkerPending StageMarker = "[ ]"
	// StageMarkerInProgress identifies an in-progress stage.
	StageMarkerInProgress StageMarker = "[-]"
	// StageMarkerAwaitingApproval identifies a stage awaiting approval.
	StageMarkerAwaitingApproval StageMarker = "[?]"
	// StageMarkerRevising identifies a stage being revised.
	StageMarkerRevising StageMarker = "[R]"
	// StageMarkerCompleted identifies a completed stage.
	StageMarkerCompleted StageMarker = "[x]"
	// StageMarkerSkipped identifies a skipped stage.
	StageMarkerSkipped StageMarker = "[S]"
)

// StageMarkerPatch replaces one Stage Progress marker. Slug identifies the
// row, while Expected prevents an update against stale raw state.
type StageMarkerPatch struct {
	Slug        string
	Expected    StageMarker
	Replacement StageMarker
}

// PatchRequest describes typed, local changes to aidlc-state.md.
type PatchRequest struct {
	Fields        []FieldPatch
	PhaseProgress []PhaseProgressPatch
	StageMarkers  []StageMarkerPatch
}

// Patch validates content, applies only requested typed lifecycle changes, and
// validates the resulting state again. It performs no filesystem or input
// mutation and returns no partial output on error.
func Patch(content []byte, request PatchRequest) ([]byte, error) {
	if _, err := Parse(content); err != nil {
		return nil, fmt.Errorf("patch state: parse input: %w", err)
	}
	if len(request.Fields) == 0 && len(request.PhaseProgress) == 0 && len(request.StageMarkers) == 0 {
		return nil, invalidPatch("patch request is empty")
	}
	if err := validateFieldPatches(request.Fields); err != nil {
		return nil, err
	}
	if err := validatePhaseProgressPatches(request.PhaseProgress); err != nil {
		return nil, err
	}
	if err := validateStageMarkerPatches(request.StageMarkers); err != nil {
		return nil, err
	}
	edits, err := locateFieldPatches(content, request.Fields)
	if err != nil {
		return nil, err
	}
	phaseEdits, err := locatePhaseProgressPatches(content, request.PhaseProgress)
	if err != nil {
		return nil, err
	}
	edits = append(edits, phaseEdits...)
	markerEdits, err := locateStageMarkerPatches(content, request.StageMarkers)
	if err != nil {
		return nil, err
	}
	edits = append(edits, markerEdits...)
	patched, err := applyPatchEdits(content, edits)
	if err != nil {
		return nil, err
	}
	if _, err := Parse(patched); err != nil {
		return nil, fmt.Errorf("patch state: parse output: %w", err)
	}
	return patched, nil
}

func invalidPatch(format string, args ...any) error {
	return fmt.Errorf("patch state: %w: %s", fs.ErrInvalid, fmt.Sprintf(format, args...))
}

type fieldValueKind uint8

const (
	fieldValueScalar fieldValueKind = iota + 1
	fieldValueStage
	fieldValueCount
	fieldValueLifecyclePhase
	fieldValueWorkflowStatus
	fieldValueNextAction
)

type canonicalFieldDefinition struct {
	section string
	label   string
	kind    fieldValueKind
}

func canonicalFieldDefinitionFor(field CanonicalField) (canonicalFieldDefinition, bool) {
	switch field {
	case CanonicalFieldTotalStages:
		return canonicalFieldDefinition{section: "Execution Plan Summary", label: "Total Stages", kind: fieldValueCount}, true
	case CanonicalFieldCompleted:
		return canonicalFieldDefinition{section: "Execution Plan Summary", label: "Completed", kind: fieldValueCount}, true
	case CanonicalFieldInProgress:
		return canonicalFieldDefinition{section: "Execution Plan Summary", label: "In Progress", kind: fieldValueStage}, true
	case CanonicalFieldLifecyclePhase:
		return canonicalFieldDefinition{section: "Current Status", label: "Lifecycle Phase", kind: fieldValueLifecyclePhase}, true
	case CanonicalFieldCurrentStage:
		return canonicalFieldDefinition{section: "Current Status", label: "Current Stage", kind: fieldValueStage}, true
	case CanonicalFieldNextStage:
		return canonicalFieldDefinition{section: "Current Status", label: "Next Stage", kind: fieldValueStage}, true
	case CanonicalFieldStatus:
		return canonicalFieldDefinition{section: "Current Status", label: "Status", kind: fieldValueWorkflowStatus}, true
	case CanonicalFieldLastUpdated:
		return canonicalFieldDefinition{section: "Current Status", label: "Last Updated", kind: fieldValueScalar}, true
	case CanonicalFieldRevisionCount:
		return canonicalFieldDefinition{section: "Runtime State", label: "Revision Count", kind: fieldValueCount}, true
	case CanonicalFieldActiveAgent:
		return canonicalFieldDefinition{section: "Project Information", label: "Active Agent", kind: fieldValueScalar}, true
	case CanonicalFieldLastCompletedStage:
		return canonicalFieldDefinition{section: "Session Resume Point", label: "Last Completed Stage", kind: fieldValueStage}, true
	case CanonicalFieldNextAction:
		return canonicalFieldDefinition{section: "Session Resume Point", label: "Next Action", kind: fieldValueNextAction}, true
	default:
		return canonicalFieldDefinition{}, false
	}
}

func validateFieldPatches(patches []FieldPatch) error {
	seen := make(map[CanonicalField]struct{}, len(patches))
	for _, patch := range patches {
		definition, ok := canonicalFieldDefinitionFor(patch.Field)
		if !ok {
			return invalidPatch("field %q is not patchable", patch.Field)
		}
		if _, exists := seen[patch.Field]; exists {
			return invalidPatch("duplicate field target %q", patch.Field)
		}
		seen[patch.Field] = struct{}{}
		if err := validateFieldValue(definition.kind, patch.Expected); err != nil {
			return invalidPatch("invalid expected value for %q: %v", patch.Field, err)
		}
		if err := validateFieldValue(definition.kind, patch.Replacement); err != nil {
			return invalidPatch("invalid replacement value for %q: %v", patch.Field, err)
		}
	}
	return nil
}

func validateFieldValue(kind fieldValueKind, value string) error {
	switch kind {
	case fieldValueScalar:
		if err := validatePatchScalar(value); err != nil {
			return err
		}
	case fieldValueStage:
		if value == "none" {
			return nil
		}
		if err := validateStageSlug(value); err != nil {
			return err
		}
	case fieldValueCount:
		if _, err := parseCanonicalNonNegativeInt(value, "patch value"); err != nil {
			return err
		}
	case fieldValueLifecyclePhase:
		if _, ok := parseLifecyclePhase(value); !ok {
			return invalidPatch("invalid lifecycle phase %q", value)
		}
	case fieldValueWorkflowStatus:
		if _, ok := parseWorkflowStatus(value); !ok {
			return invalidPatch("invalid workflow status %q", value)
		}
	case fieldValueNextAction:
		if !validCanonicalSingleLine(value) || strings.TrimSpace(value) != value {
			return invalidPatch("next action must be nonempty, valid utf-8, and a single line without surrounding whitespace")
		}
	default:
		return invalidPatch("unknown field value kind %d", kind)
	}
	return nil
}

func validatePatchScalar(value string) error {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return invalidPatch("scalar value must be nonempty, valid utf-8, and have no surrounding whitespace")
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.IsSpace(character) {
			return invalidPatch("scalar value contains whitespace or a control character")
		}
	}
	return nil
}

func validatePhaseProgressPatches(patches []PhaseProgressPatch) error {
	seen := make(map[LifecyclePhase]struct{}, len(patches))
	for _, patch := range patches {
		if _, ok := phaseDisplayFor(patch.Phase); !ok {
			return invalidPatch("phase %q is not canonical", patch.Phase)
		}
		if _, exists := seen[patch.Phase]; exists {
			return invalidPatch("duplicate phase progress target %q", patch.Phase)
		}
		seen[patch.Phase] = struct{}{}
		if _, ok := parsePhaseStatus(string(patch.Expected)); !ok {
			return invalidPatch("invalid expected phase status %q", patch.Expected)
		}
		if _, ok := parsePhaseStatus(string(patch.Replacement)); !ok {
			return invalidPatch("invalid replacement phase status %q", patch.Replacement)
		}
	}
	return nil
}

func phaseDisplayFor(phase LifecyclePhase) (string, bool) {
	for _, definition := range phaseDefinitions {
		if definition.value == phase {
			return definition.display, true
		}
	}
	return "", false
}

func validateStageMarkerPatches(patches []StageMarkerPatch) error {
	seen := make(map[string]struct{}, len(patches))
	for _, patch := range patches {
		if err := validateStageSlug(patch.Slug); err != nil {
			return err
		}
		if _, exists := seen[patch.Slug]; exists {
			return invalidPatch("duplicate stage progress target %q", patch.Slug)
		}
		seen[patch.Slug] = struct{}{}
		if !validStageMarker(patch.Expected) {
			return invalidPatch("invalid expected stage progress marker %q", patch.Expected)
		}
		if !validStageMarker(patch.Replacement) {
			return invalidPatch("invalid replacement stage progress marker %q", patch.Replacement)
		}
	}
	return nil
}

func validateStageSlug(slug string) error {
	if slug == "" || !utf8.ValidString(slug) {
		return invalidPatch("stage progress slug must be nonempty valid utf-8")
	}
	for _, value := range slug {
		if unicode.IsSpace(value) || unicode.IsControl(value) {
			return invalidPatch("stage progress slug %q contains whitespace or control characters", slug)
		}
	}
	return nil
}

func validStageMarker(marker StageMarker) bool {
	switch marker {
	case StageMarkerPending,
		StageMarkerInProgress,
		StageMarkerAwaitingApproval,
		StageMarkerRevising,
		StageMarkerCompleted,
		StageMarkerSkipped:
		return true
	default:
		return false
	}
}

type patchLine struct {
	start   int
	bodyEnd int
}

func splitPatchLines(content []byte) []patchLine {
	lines := make([]patchLine, 0)
	start := 0
	for index, value := range content {
		if value != '\n' {
			continue
		}
		bodyEnd := index
		if bodyEnd > start && content[bodyEnd-1] == '\r' {
			bodyEnd--
		}
		lines = append(lines, patchLine{start: start, bodyEnd: bodyEnd})
		start = index + 1
	}
	bodyEnd := len(content)
	if bodyEnd > start && content[bodyEnd-1] == '\r' {
		bodyEnd--
	}
	lines = append(lines, patchLine{start: start, bodyEnd: bodyEnd})
	return lines
}

type patchEdit struct {
	start       int
	end         int
	replacement []byte
}

func locateFieldPatches(content []byte, patches []FieldPatch) ([]patchEdit, error) {
	if len(patches) == 0 {
		return nil, nil
	}
	targets := make(map[string][]FieldPatch, len(patches))
	for _, patch := range patches {
		definition, _ := canonicalFieldDefinitionFor(patch.Field)
		targets[definition.section] = append(targets[definition.section], patch)
	}
	found := make(map[CanonicalField]bool, len(patches))
	edits := make([]patchEdit, 0, len(patches))
	section := ""
	for _, line := range splitPatchLines(content) {
		sectionName, ok := patchSectionName(content[line.start:line.bodyEnd])
		if ok {
			section = sectionName
			continue
		}
		sectionTargets := targets[section]
		if len(sectionTargets) == 0 {
			continue
		}
		body := content[line.start:line.bodyEnd]
		for _, patch := range sectionTargets {
			definition, _ := canonicalFieldDefinitionFor(patch.Field)
			prefix := []byte("- **" + definition.label + "**:")
			if !hasPatchPrefix(body, prefix) {
				continue
			}
			if found[patch.Field] {
				return nil, invalidPatch("duplicate field target %q in section", patch.Field)
			}
			found[patch.Field] = true
			left, right := trimPatchValue(body[len(prefix):])
			if left == right {
				return nil, invalidPatch("field %q has an empty value", patch.Field)
			}
			current := string(body[len(prefix)+left : len(prefix)+right])
			if current != patch.Expected {
				return nil, invalidPatch("field %q is %q, expected %q", patch.Field, current, patch.Expected)
			}
			edits = append(edits, patchEdit{
				start:       line.start + len(prefix) + left,
				end:         line.start + len(prefix) + right,
				replacement: []byte(patch.Replacement),
			})
		}
	}
	for _, patch := range patches {
		if !found[patch.Field] {
			return nil, invalidPatch("field target %q is missing", patch.Field)
		}
	}
	return edits, nil
}

func hasPatchPrefix(line, prefix []byte) bool {
	if len(line) < len(prefix) {
		return false
	}
	for index, value := range prefix {
		if line[index] != value {
			return false
		}
	}
	return true
}

func trimPatchValue(value []byte) (left, right int) {
	for left < len(value) {
		character, size := utf8.DecodeRune(value[left:])
		if !unicode.IsSpace(character) {
			break
		}
		left += size
	}
	right = len(value)
	for right > left {
		character, size := utf8.DecodeLastRune(value[:right])
		if !unicode.IsSpace(character) {
			break
		}
		right -= size
	}
	return left, right
}

func locatePhaseProgressPatches(content []byte, patches []PhaseProgressPatch) ([]patchEdit, error) {
	if len(patches) == 0 {
		return nil, nil
	}
	byPhase := make(map[LifecyclePhase]PhaseProgressPatch, len(patches))
	for _, patch := range patches {
		byPhase[patch.Phase] = patch
	}
	found := make(map[LifecyclePhase]bool, len(patches))
	edits := make([]patchEdit, 0, len(patches))
	section := ""
	for _, line := range splitPatchLines(content) {
		sectionName, ok := patchSectionName(content[line.start:line.bodyEnd])
		if ok {
			section = sectionName
			continue
		}
		if section != "Phase Progress" {
			continue
		}
		body := content[line.start:line.bodyEnd]
		for phase, patch := range byPhase {
			display, _ := phaseDisplayFor(phase)
			prefix := []byte("- **" + display + "**:")
			if !hasPatchPrefix(body, prefix) {
				continue
			}
			if found[phase] {
				return nil, invalidPatch("duplicate phase progress target %q in section", phase)
			}
			found[phase] = true
			left, right := trimPatchValue(body[len(prefix):])
			if left == right {
				return nil, invalidPatch("phase %q has an empty status", phase)
			}
			current := string(body[len(prefix)+left : len(prefix)+right])
			if current != string(patch.Expected) {
				return nil, invalidPatch("phase %q is %q, expected %q", phase, current, patch.Expected)
			}
			edits = append(edits, patchEdit{
				start:       line.start + len(prefix) + left,
				end:         line.start + len(prefix) + right,
				replacement: []byte(patch.Replacement),
			})
		}
	}
	for _, patch := range patches {
		if !found[patch.Phase] {
			return nil, invalidPatch("phase progress target %q is missing", patch.Phase)
		}
	}
	return edits, nil
}

func locateStageMarkerPatches(content []byte, patches []StageMarkerPatch) ([]patchEdit, error) {
	bySlug := make(map[string]StageMarkerPatch, len(patches))
	for _, patch := range patches {
		bySlug[patch.Slug] = patch
	}
	found := make(map[string]bool, len(patches))
	edits := make([]patchEdit, 0, len(patches))
	section := ""
	for _, line := range splitPatchLines(content) {
		body := content[line.start:line.bodyEnd]
		if sectionName, ok := patchSectionName(body); ok {
			section = sectionName
			continue
		}
		if section != "Stage Progress" || !strings.HasPrefix(string(body), "- [") {
			continue
		}
		stage, err := parseStageRow(string(body))
		if err != nil {
			return nil, fmt.Errorf("patch state: inspect stage progress: %w", err)
		}
		patch, ok := bySlug[stage.Slug]
		if !ok {
			continue
		}
		found[stage.Slug] = true
		if StageMarker(stage.CheckboxMarker) != patch.Expected {
			return nil, invalidPatch("stage progress marker for %q is %q, expected %q", stage.Slug, stage.CheckboxMarker, patch.Expected)
		}
		edits = append(edits, patchEdit{
			start:       line.start + 2,
			end:         line.start + 5,
			replacement: []byte(patch.Replacement),
		})
	}
	for _, patch := range patches {
		if !found[patch.Slug] {
			return nil, invalidPatch("stage progress target %q is missing", patch.Slug)
		}
	}
	return edits, nil
}

func patchSectionName(line []byte) (string, bool) {
	const prefix = "## "
	if !strings.HasPrefix(string(line), prefix) {
		return "", false
	}
	return string(line[len(prefix):]), true
}

func applyPatchEdits(content []byte, edits []patchEdit) ([]byte, error) {
	sort.Slice(edits, func(left, right int) bool { return edits[left].start < edits[right].start })
	for index := 1; index < len(edits); index++ {
		if edits[index-1].end > edits[index].start {
			return nil, invalidPatch("patch targets overlap")
		}
	}
	output := make([]byte, 0, len(content))
	cursor := 0
	for _, edit := range edits {
		output = append(output, content[cursor:edit.start]...)
		output = append(output, edit.replacement...)
		cursor = edit.end
	}
	output = append(output, content[cursor:]...)
	return output, nil
}
