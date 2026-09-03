package state

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sori883/ai-dd/src/internal/scope"
)

var stateReaderRequiredSections = [...]string{
	"Project Information",
	"Execution Plan Summary",
	"Phase Progress",
	"Stage Progress",
	"Current Status",
}

func TestParseCanonicalState(t *testing.T) {
	t.Parallel()

	got, err := Parse([]byte(canonicalStateContent()))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Version() != 8 {
		t.Errorf("Version() = %d, want 8", got.Version())
	}
	if got.Scope() != "classic" {
		t.Errorf("Scope() = %q, want classic", got.Scope())
	}
	if got.ProjectType() != "Brownfield" {
		t.Errorf("ProjectType() = %q, want Brownfield", got.ProjectType())
	}
	if got.WorkflowStatus() != WorkflowStatusRunning {
		t.Errorf("WorkflowStatus() = %q, want %q", got.WorkflowStatus(), WorkflowStatusRunning)
	}
	if got.LifecyclePhase() != LifecyclePhaseIdeation {
		t.Errorf("LifecyclePhase() = %q, want %q", got.LifecyclePhase(), LifecyclePhaseIdeation)
	}
	if got.CurrentStage() != "intent-capture" {
		t.Errorf("CurrentStage() = %q, want intent-capture", got.CurrentStage())
	}
	if got.NextStage() != "reverse-engineering" {
		t.Errorf("NextStage() = %q, want reverse-engineering", got.NextStage())
	}

	summary := got.Summary()
	if summary.TotalStages != 5 || summary.Completed != 2 || summary.InProgress != "intent-capture" {
		t.Errorf("Summary() = %#v, want total 5 completed 2 in-progress intent-capture", summary)
	}

	phases := got.PhaseProgress()
	if len(phases) != 5 {
		t.Fatalf("PhaseProgress() length = %d, want 5", len(phases))
	}
	wantPhases := []PhaseProgress{
		{Phase: LifecyclePhaseInitialization, Status: PhaseStatusVerified},
		{Phase: LifecyclePhaseIdeation, Status: PhaseStatusActive},
		{Phase: LifecyclePhaseInception, Status: PhaseStatusPending},
		{Phase: LifecyclePhaseConstruction, Status: PhaseStatusPending},
		{Phase: LifecyclePhaseOperation, Status: PhaseStatusSkipped},
	}
	for index, want := range wantPhases {
		if phases[index] != want {
			t.Errorf("PhaseProgress()[%d] = %#v, want %#v", index, phases[index], want)
		}
	}

	stages := got.Stages()
	if len(stages) != 7 {
		t.Fatalf("Stages() length = %d, want 7", len(stages))
	}
	wantStages := []StageProgress{
		{
			Slug:           "workspace-scaffold",
			CheckboxMarker: "[x]",
			CheckboxState:  CheckboxStateCompleted,
			Suffix:         "EXECUTE",
			PlanAction:     PlanActionExecute,
		},
		{
			Slug:           "state-init",
			CheckboxMarker: "[x]",
			CheckboxState:  CheckboxStateCompleted,
			Suffix:         "EXECUTE",
			PlanAction:     PlanActionExecute,
		},
		{
			Slug:           "intent-capture",
			CheckboxMarker: "[-]",
			CheckboxState:  CheckboxStateInProgress,
			Suffix:         "EXECUTE",
			PlanAction:     PlanActionExecute,
		},
		{
			Slug:           "market-research",
			CheckboxMarker: "[ ]",
			CheckboxState:  CheckboxStatePending,
			Suffix:         "SKIP",
			PlanAction:     PlanActionSkip,
		},
		{
			Slug:           "reverse-engineering",
			CheckboxMarker: "[?]",
			CheckboxState:  CheckboxStateAwaitingApproval,
			Suffix:         "EXECUTE",
			PlanAction:     PlanActionExecute,
		},
		{
			Slug:           "code-generation",
			CheckboxMarker: "[R]",
			CheckboxState:  CheckboxStateRevising,
			Suffix:         "SKIP",
			PlanAction:     PlanActionSkip,
		},
		{
			Slug:           "deployment-pipeline",
			CheckboxMarker: "[S]",
			CheckboxState:  CheckboxStateSkipped,
			Suffix:         "EXECUTE",
			PlanAction:     PlanActionExecute,
		},
	}
	for index, want := range wantStages {
		if stages[index] != want {
			t.Errorf("Stages()[%d] = %#v, want %#v", index, stages[index], want)
		}
	}
}

func TestParseBuildInitialState(t *testing.T) {
	t.Parallel()

	snapshot := loadTestSnapshot(t, []map[string]any{
		stageFixture("workspace-scaffold", "0.1", "initialization"),
		stageFixture("state-init", "0.3", "initialization"),
		stageFixture("intent-capture", "1.1", "ideation"),
		stageFixture("reverse-engineering", "2.1", "inception"),
	}, map[string]any{
		"classic": map[string]any{"stages": map[string]any{
			"workspace-scaffold":  "EXECUTE",
			"state-init":          "EXECUTE",
			"intent-capture":      "EXECUTE",
			"reverse-engineering": "EXECUTE",
		}},
	})
	initial, err := BuildInitial(Input{
		Graph:                     snapshot,
		Scope:                     "classic",
		ScopeMetadata:             scopeMetadata("classic"),
		Workspace:                 WorkspaceInfo{ProjectType: "Brownfield"},
		ProjectRoot:               "/project",
		ProjectDescription:        "description",
		ProjectDescriptionPreview: "Description",
		StartDate:                 "2026-09-03T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("BuildInitial() error = %v", err)
	}

	got, err := Parse([]byte(initial.StateContent))
	if err != nil {
		t.Fatalf("Parse(BuildInitial().StateContent) error = %v", err)
	}
	if got.Version() != 8 || got.Scope() != "classic" || got.ProjectType() != "Brownfield" {
		t.Fatalf("parsed identity = version %d scope %q project type %q", got.Version(), got.Scope(), got.ProjectType())
	}
	if len(got.Stages()) != 4 {
		t.Fatalf("Stages() length = %d, want 4", len(got.Stages()))
	}
}

func TestStateAccessorsReturnDefensiveCopies(t *testing.T) {
	t.Parallel()

	got, err := Parse([]byte(canonicalStateContent()))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	phases := got.PhaseProgress()
	stages := got.Stages()
	phases[0].Status = PhaseStatusSkipped
	stages[0].Slug = "mutated"
	if got.PhaseProgress()[0].Status != PhaseStatusVerified {
		t.Error("PhaseProgress() exposed internal storage")
	}
	if got.Stages()[0].Slug != "workspace-scaffold" {
		t.Error("Stages() exposed internal storage")
	}
}

func TestParseDoesNotRetainInputBytes(t *testing.T) {
	t.Parallel()

	content := []byte(canonicalStateContent())
	got, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	for index := range content {
		content[index] = 'z'
	}
	if got.Scope() != "classic" || got.Stages()[0].Slug != "workspace-scaffold" {
		t.Errorf("parsed State changed after input mutation: scope=%q stages=%#v", got.Scope(), got.Stages())
	}
}

func TestParseEncodingAndHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		valid   bool
	}{
		{
			name:    "single leading bom",
			content: []byte("\ufeff" + canonicalStateContent()),
			valid:   true,
		},
		{
			name:    "double leading bom",
			content: []byte("\ufeff\ufeff" + canonicalStateContent()),
		},
		{
			name:    "invalid utf 8",
			content: append([]byte(canonicalStateContent()), 0xff),
		},
		{
			name:    "lone carriage return",
			content: []byte(strings.Replace(canonicalStateContent(), "\n\n", "\rX", 1)),
		},
		{
			name:    "double carriage return before line feed",
			content: []byte(strings.Replace(canonicalStateContent(), "\n", "\r\r\n", 1)),
		},
		{
			name:    "crlf",
			content: []byte(strings.ReplaceAll(canonicalStateContent(), "\n", "\r\n")),
			valid:   true,
		},
		{
			name:    "mixed line endings",
			content: []byte(strings.Replace(canonicalStateContent(), "\n", "\r\n", 1)),
			valid:   true,
		},
		{
			name:    "missing final line feed",
			content: []byte(strings.TrimSuffix(canonicalStateContent(), "\n")),
			valid:   true,
		},
		{
			name:    "header with trailing space",
			content: []byte(strings.Replace(canonicalStateContent(), stateHeader, stateHeader+" ", 1)),
		},
		{
			name:    "unknown section over scanner limit",
			content: []byte(canonicalStateContent() + "\n## Additional\n" + strings.Repeat("x", 70*1024)),
			valid:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.content)
			if tt.valid {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if got.Version() != 8 {
					t.Errorf("Version() = %d, want 8", got.Version())
				}
				return
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || got.Scope() != "" || got.ProjectType() != "" ||
				got.WorkflowStatus() != WorkflowStatusUnknown || got.LifecyclePhase() != LifecyclePhaseUnknown ||
				got.CurrentStage() != "" || got.NextStage() != "" || len(got.PhaseProgress()) != 0 || len(got.Stages()) != 0 {
				t.Fatalf("Parse() state = %#v, want zero State", got)
			}
		})
	}
}

func TestParseSectionScopedFieldsAndValues(t *testing.T) {
	t.Parallel()

	maxIntPlusOne := strconv.FormatUint(uint64(^uint(0)>>1)+1, 10)
	tests := []struct {
		name    string
		content string
		valid   bool
	}{
		{
			name:    "decoy field in unknown section",
			content: canonicalStateContent() + "\n## Decoy\n- **Scope**: wrong\n",
			valid:   true,
		},
		{
			name: "required field in wrong section",
			content: strings.Replace(
				strings.Replace(canonicalStateContent(), "- **Scope**: classic\n", "", 1),
				"## Execution Plan Summary\n",
				"## Execution Plan Summary\n- **Scope**: wrong\n",
				1,
			),
		},
		{
			name: "duplicate required field",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Project Type**: Brownfield\n",
				"- **Project Type**: Brownfield\n- **Project Type**: other\n",
				1,
			),
		},
		{
			name:    "missing required field",
			content: strings.Replace(canonicalStateContent(), "- **Scope**: classic\n", "", 1),
		},
		{
			name:    "empty required string",
			content: strings.Replace(canonicalStateContent(), "- **Scope**: classic\n", "- **Scope**:\n", 1),
		},
		{
			name:    "state version with leading zero",
			content: strings.Replace(canonicalStateContent(), "- **State Version**: 8", "- **State Version**: 08", 1),
		},
		{
			name:    "state version with suffix",
			content: strings.Replace(canonicalStateContent(), "- **State Version**: 8", "- **State Version**: 8 extra", 1),
		},
		{
			name:    "total stages with leading zero",
			content: strings.Replace(canonicalStateContent(), "- **Total Stages**: 5", "- **Total Stages**: 05", 1),
		},
		{
			name:    "total stages with sign",
			content: strings.Replace(canonicalStateContent(), "- **Total Stages**: 5", "- **Total Stages**: +5", 1),
		},
		{
			name:    "completed stages negative",
			content: strings.Replace(canonicalStateContent(), "- **Completed**: 2", "- **Completed**: -1", 1),
		},
		{
			name:    "total stages overflow",
			content: strings.Replace(canonicalStateContent(), "- **Total Stages**: 5", "- **Total Stages**: "+maxIntPlusOne, 1),
		},
		{
			name:    "unknown workflow status",
			content: strings.Replace(canonicalStateContent(), "- **Status**: Running", "- **Status**: running", 1),
		},
		{
			name:    "completed workflow status",
			content: strings.Replace(canonicalStateContent(), "- **Status**: Running", "- **Status**: Completed", 1),
			valid:   true,
		},
		{
			name: "unknown lifecycle phase",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Lifecycle Phase**: IDEATION",
				"- **Lifecycle Phase**: ideation",
				1,
			),
		},
		{
			name:    "unknown phase status",
			content: strings.Replace(canonicalStateContent(), "- **Ideation**: Active", "- **Ideation**: active", 1),
		},
		{
			name:    "summary meaning not cross validated",
			content: strings.Replace(canonicalStateContent(), "- **Completed**: 2", "- **Completed**: 0", 1),
			valid:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse([]byte(tt.content))
			if tt.valid {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				return
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || got.Scope() != "" || len(got.Stages()) != 0 {
				t.Fatalf("Parse() returned partial state %#v", got)
			}
		})
	}
}

func TestParseRequiredSectionsMayBeReorderedButMustBeUnique(t *testing.T) {
	t.Parallel()

	for _, section := range stateReaderRequiredSections {
		section := section
		t.Run("missing "+section, func(t *testing.T) {
			t.Parallel()
			content := removeCanonicalSection(canonicalStateContent(), section)
			got, err := Parse([]byte(content))
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || got.Scope() != "" || len(got.PhaseProgress()) != 0 || len(got.Stages()) != 0 {
				t.Fatalf("Parse() state = %#v, want zero State", got)
			}
		})

		t.Run("duplicate "+section, func(t *testing.T) {
			t.Parallel()
			content := duplicateCanonicalSectionBefore(canonicalStateContent(), section)
			got, err := Parse([]byte(content))
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || got.Scope() != "" || len(got.PhaseProgress()) != 0 || len(got.Stages()) != 0 {
				t.Fatalf("Parse() state = %#v, want zero State", got)
			}
		})
	}

	sectionOrder := []string{
		"Current Status",
		"Stage Progress",
		"Phase Progress",
		"Execution Plan Summary",
		"Project Information",
	}
	sectionReordered := reorderCanonicalSections(canonicalStateContent(), sectionOrder)
	if got, err := Parse([]byte(sectionReordered)); err != nil {
		t.Fatalf("Parse(reordered sections) error = %v", err)
	} else if got.Version() != 8 || len(got.PhaseProgress()) != 5 || len(got.Stages()) != 7 {
		t.Fatalf("Parse(reordered sections) = version %d, %d phases, %d stages", got.Version(), len(got.PhaseProgress()), len(got.Stages()))
	}

	fieldReordered := []struct {
		name   string
		first  string
		second string
	}{
		{
			name:   "project information",
			first:  "- **Project Type**: Brownfield",
			second: "- **Scope**: classic",
		},
		{
			name:   "execution plan summary",
			first:  "- **Total Stages**: 5",
			second: "- **Completed**: 2",
		},
		{
			name:   "current status",
			first:  "- **Lifecycle Phase**: IDEATION",
			second: "- **Current Stage**: intent-capture",
		},
	}
	for _, tt := range fieldReordered {
		t.Run("field order in "+tt.name, func(t *testing.T) {
			t.Parallel()
			content := swapCanonicalLines(canonicalStateContent(), tt.first, tt.second)
			got, err := Parse([]byte(content))
			if err != nil {
				t.Fatalf("Parse(reordered fields) error = %v", err)
			}
			if got.Version() != 8 {
				t.Errorf("Version() = %d, want 8", got.Version())
			}
		})
	}
}

func TestParseStageProgressStrictness(t *testing.T) {
	t.Parallel()

	const stageRow = "- [S] deployment-pipeline — EXECUTE"
	tests := []struct {
		name    string
		content string
		valid   bool
	}{
		{
			name: "flexible horizontal whitespace and raw suffix",
			content: strings.Replace(
				canonicalStateContent(),
				stageRow,
				"- [S]\todd.slug/v2\t—\tEXECUTE explanation with  spaces  ",
				1,
			),
			valid: true,
		},
		{
			name:    "slug grammar is not graph validated",
			content: strings.Replace(canonicalStateContent(), "workspace-scaffold", "not-a-graph-slug", 1),
			valid:   true,
		},
		{
			name:    "stage decoy outside stage section",
			content: canonicalStateContent() + "\n## Decoy\n- [x] decoy-stage — EXECUTE\n",
			valid:   true,
		},
		{
			name:    "duplicate slug",
			content: strings.Replace(canonicalStateContent(), stageRow, stageRow+"\n"+stageRow, 1),
		},
		{
			name:    "unknown marker",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [X] deployment-pipeline — EXECUTE", 1),
		},
		{
			name:    "missing em dash",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] deployment-pipeline - EXECUTE", 1),
		},
		{
			name:    "missing marker separator",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S]deployment-pipeline — EXECUTE", 1),
		},
		{
			name:    "empty slug",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] — EXECUTE", 1),
		},
		{
			name:    "slug contains whitespace",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] deployment pipeline — EXECUTE", 1),
		},
		{
			name:    "empty suffix",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] deployment-pipeline —", 1),
		},
		{
			name:    "action token must be exact",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] deployment-pipeline — EXECUTEfoo", 1),
		},
		{
			name:    "skip action word continuation",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [S] deployment-pipeline — SKIP_foo", 1),
		},
		{
			name:    "stage-like malformed row",
			content: strings.Replace(canonicalStateContent(), stageRow, "- [", 1),
		},
		{
			name:    "missing stage rows",
			content: removeCanonicalStageRows(canonicalStateContent()),
		},
		{
			name: "phase order is strict",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Initialization**: Verified\n- **Ideation**: Active",
				"- **Ideation**: Active\n- **Initialization**: Verified",
				1,
			),
		},
		{
			name: "duplicate phase",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Ideation**: Active\n",
				"- **Ideation**: Active\n- **Ideation**: Active\n",
				1,
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse([]byte(tt.content))
			if tt.valid {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if tt.name == "flexible horizontal whitespace and raw suffix" {
					stage := got.Stages()[6]
					if stage.Slug != "odd.slug/v2" || stage.Suffix != "EXECUTE explanation with  spaces" ||
						stage.PlanAction != PlanActionExecute || stage.CheckboxState != CheckboxStateSkipped {
						t.Errorf("StageProgress = %#v, want preserved suffix and derived values", stage)
					}
				}
				return
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
			}
			if got.Version() != 0 || len(got.Stages()) != 0 || len(got.PhaseProgress()) != 0 {
				t.Fatalf("Parse() returned partial state %#v", got)
			}
		})
	}
}

func TestParseLargeMalformedStageRowScalesLinearly(t *testing.T) {
	t.Parallel()

	const (
		emDashCount = 50_000
		maxDuration = 750 * time.Millisecond
	)
	const canonicalRow = "- [S] deployment-pipeline — EXECUTE"
	malformedRow := "- [ ] " + strings.Repeat("a—", emDashCount) + "INVALID" + strings.Repeat(" ", emDashCount)
	content := strings.Replace(canonicalStateContent(), canonicalRow, malformedRow, 1)

	started := time.Now()
	got, err := Parse([]byte(content))
	elapsed := time.Since(started)
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Parse() error = %v, want fs.ErrInvalid", err)
	}
	if got.Version() != 0 || len(got.Stages()) != 0 {
		t.Fatalf("Parse() returned partial state %#v", got)
	}
	if elapsed > maxDuration {
		t.Fatalf("Parse() took %v for %d em dash candidates, want <= %v", elapsed, emDashCount, maxDuration)
	}
}

func TestParseStageSuffixWithColonExplanation(t *testing.T) {
	t.Parallel()

	const stageRow = "- [S] deployment-pipeline — EXECUTE"
	for _, action := range []PlanAction{PlanActionExecute, PlanActionSkip} {
		action := action
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()
			content := strings.Replace(
				canonicalStateContent(),
				stageRow,
				"- [S] deployment-pipeline — "+string(action)+": reason",
				1,
			)
			got, err := Parse([]byte(content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			stage := got.Stages()[6]
			if stage.PlanAction != action || stage.Suffix != string(action)+": reason" {
				t.Errorf("StageProgress = %#v, want %s: reason with derived action", stage, action)
			}
		})
	}
}

func TestParseStageSlugMayContainEmDash(t *testing.T) {
	t.Parallel()

	const stageRow = "- [S] deployment-pipeline — EXECUTE"
	tests := []struct {
		name       string
		row        string
		wantSlug   string
		wantAction PlanAction
	}{
		{
			name:       "internal em dash",
			row:        "- [S] feature—v2 — EXECUTE",
			wantSlug:   "feature—v2",
			wantAction: PlanActionExecute,
		},
		{
			name:       "greedy slug keeps action-like text",
			row:        "- [S] feature—EXECUTEfoo—v2 — SKIP",
			wantSlug:   "feature—EXECUTEfoo—v2",
			wantAction: PlanActionSkip,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := strings.Replace(canonicalStateContent(), stageRow, tt.row, 1)
			got, err := Parse([]byte(content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			stage := got.Stages()[6]
			if stage.Slug != tt.wantSlug || stage.PlanAction != tt.wantAction {
				t.Errorf("StageProgress = %#v, want slug %q/action %q", stage, tt.wantSlug, tt.wantAction)
			}
		})
	}
}

func TestParseDoesNotCrossValidateSemanticRelationships(t *testing.T) {
	t.Parallel()

	content := canonicalStateContent()
	content = strings.Replace(content, "- **Total Stages**: 5", "- **Total Stages**: 0", 1)
	content = strings.Replace(content, "- **Completed**: 2", "- **Completed**: 99", 1)
	content = strings.Replace(
		content,
		"- **Current Stage**: intent-capture",
		"- **Current Stage**: absent-from-stage-list",
		1,
	)
	content = strings.Replace(content, "- [x] workspace-scaffold", "- [ ] workspace-scaffold", 1)
	got, err := Parse([]byte(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Summary().TotalStages != 0 || got.Summary().Completed != 99 || got.CurrentStage() != "absent-from-stage-list" {
		t.Errorf("Parse() applied semantic cross validation: %#v current=%q", got.Summary(), got.CurrentStage())
	}
}

func removeCanonicalStageRows(content string) string {
	rows := []string{
		"- [x] workspace-scaffold — EXECUTE\n",
		"- [x] state-init — EXECUTE\n",
		"- [-] intent-capture — EXECUTE\n",
		"- [ ] market-research — SKIP\n",
		"- [?] reverse-engineering — EXECUTE\n",
		"- [R] code-generation — SKIP\n",
		"- [S] deployment-pipeline — EXECUTE\n",
	}
	for _, row := range rows {
		content = strings.Replace(content, row, "", 1)
	}
	return content
}

func removeCanonicalSection(content, section string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if line != "## "+section {
			continue
		}
		start = index
		for next := index + 1; next < len(lines); next++ {
			if strings.HasPrefix(lines[next], "## ") {
				end = next
				break
			}
		}
		break
	}
	if start < 0 {
		return content
	}
	lines = append(lines[:start], lines[end:]...)
	return strings.Join(lines, "\n") + "\n"
}

func duplicateCanonicalSectionBefore(content, section string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	start := -1
	end := len(lines)
	for index, line := range lines {
		if line != "## "+section {
			continue
		}
		start = index
		for next := index + 1; next < len(lines); next++ {
			if strings.HasPrefix(lines[next], "## ") {
				end = next
				break
			}
		}
		break
	}
	if start < 0 {
		return content
	}
	block := append([]string(nil), lines[start:end]...)
	duplicated := make([]string, 0, len(lines)+len(block)+1)
	duplicated = append(duplicated, lines[:start]...)
	duplicated = append(duplicated, block...)
	duplicated = append(duplicated, "")
	duplicated = append(duplicated, lines[start:]...)
	return strings.Join(duplicated, "\n") + "\n"
}

func reorderCanonicalSections(content string, order []string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	blocks := make(map[string][]string, len(order))
	for index := 1; index < len(lines); {
		if !strings.HasPrefix(lines[index], "## ") {
			index++
			continue
		}
		section := strings.TrimPrefix(lines[index], "## ")
		end := index + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "## ") {
			end++
		}
		blocks[section] = append([]string(nil), lines[index:end]...)
		index = end
	}

	result := []string{lines[0], ""}
	for _, section := range order {
		result = append(result, blocks[section]...)
		result = append(result, "")
	}
	return strings.Join(result, "\n")
}

func swapCanonicalLines(content, first, second string) string {
	lines := strings.Split(content, "\n")
	firstIndex := -1
	secondIndex := -1
	for index, line := range lines {
		switch line {
		case first:
			firstIndex = index
		case second:
			secondIndex = index
		}
	}
	if firstIndex < 0 || secondIndex < 0 {
		return content
	}
	lines[firstIndex], lines[secondIndex] = lines[secondIndex], lines[firstIndex]
	return strings.Join(lines, "\n")
}

func scopeMetadata(name string) scope.Metadata {
	return scope.Metadata{Name: name, Depth: "Standard"}
}

func canonicalStateContent() string {
	return strings.TrimLeft(`# AI-DLC State Tracking

## Project Information
- **Project**: Sample project
- **Project Description Source**: project-description.json
- **Project Type**: Brownfield
- **Scope**: classic
- **Start Date**: 2026-09-02T00:00:00Z
- **State Version**: 8
- **Active Agent**: aidlc-product-agent
- **Worktree Path**:
- **Bolt Refs**:
- **Practices Affirmed Timestamp**:

## Execution Plan Summary
- **Total Stages**: 5
- **Completed**: 2
- **In Progress**: intent-capture

## Phase Progress
<!-- Status values: Pending, Active, Verified, Skipped -->

- **Initialization**: Verified
- **Ideation**: Active
- **Inception**: Pending
- **Construction**: Pending
- **Operation**: Skipped

## Stage Progress
<!-- Checkbox states: [ ] not started, [-] in progress, [?] awaiting approval (gate open), [R] revising (user rejected gate), [x] completed, [S] skipped via --stage/--phase jump -->

### INITIALIZATION PHASE
- [x] workspace-scaffold — EXECUTE
- [x] state-init — EXECUTE

### IDEATION PHASE
- [-] intent-capture — EXECUTE
- [ ] market-research — SKIP

### INCEPTION PHASE
- [?] reverse-engineering — EXECUTE

### CONSTRUCTION PHASE
Per unit: [TBD]
- [R] code-generation — SKIP

### OPERATION PHASE
- [S] deployment-pipeline — EXECUTE

## Current Status
- **Lifecycle Phase**: IDEATION
- **Current Stage**: intent-capture
- **Next Stage**: reverse-engineering
- **Status**: Running
- **Last Updated**: 2026-09-02T00:00:00Z
`, "\n")
}
