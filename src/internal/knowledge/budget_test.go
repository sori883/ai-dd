package knowledge_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/graph"
	"github.com/sori883/ai-dd/src/internal/knowledge"
)

func TestBuildRosterStopsAtFirstPathThatExceedsJSONSizeCap(t *testing.T) {
	longName := "a-" + strings.Repeat("x", 8200) + ".md"
	input := knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework: knowledge.Source{
			FS: fstest.MapFS{
				"agents/lead.md":                     {Data: []byte("persona")},
				"knowledge/aidlc-shared/" + longName: {Data: []byte("too long")},
				"knowledge/aidlc-shared/b.md":        {Data: []byte("short but later")},
			},
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	want := []string{".codex/agents/lead.md"}
	if !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("BuildRoster() paths = %#v, want first over-cap candidate to stop later refill", got.Paths)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "omitted") {
		t.Errorf("BuildRoster() warnings = %#v, want path omission warning", got.Warnings)
	}
	encoded, err := json.Marshal(got.Paths)
	if err != nil {
		t.Fatalf("json.Marshal(paths): %v", err)
	}
	if len(encoded) > 8192 {
		t.Errorf("JSON path size = %d, want <= 8192", len(encoded))
	}
}

func TestBuildRosterPathBudgetExactBoundary(t *testing.T) {
	const maxJSONPathBytes = 8192

	persona := budgetPathWire{
		relative: "agents/lead.md",
		display:  ".codex/agents/lead.md",
		wire:     `".codex/agents/lead.md"`,
	}
	baseSpecial := specialBudgetPath(0)
	exactPadding := maxJSONPathBytes - explicitJSONPathArraySize(persona, baseSpecial)
	if exactPadding < 0 {
		t.Fatalf("special path base exceeds test budget: padding=%d", exactPadding)
	}
	exact := specialBudgetPath(exactPadding)
	over := specialBudgetPath(exactPadding + 1)

	const onePathOmissionWarning = "Warning: 1 optional persona/knowledge path(s) were omitted because there was no room to pass them all " +
		"(inline_context_paths is capped at 8192 bytes). Configure fewer knowledge files if this matters; the stage runs without the omitted optional context."
	const twoPathOmissionWarning = "Warning: 2 optional persona/knowledge path(s) were omitted because there was no room to pass them all " +
		"(inline_context_paths is capped at 8192 bytes). Configure fewer knowledge files if this matters; the stage runs without the omitted optional context."
	tests := []struct {
		name          string
		special       budgetPathWire
		includeLater  bool
		expectedBytes int
		wantPaths     []string
		wantWarnings  []string
	}{
		{
			name:          "exact cap retains all paths",
			special:       exact,
			includeLater:  true,
			expectedBytes: maxJSONPathBytes,
			wantPaths:     []string{persona.display, exact.display},
			wantWarnings:  []string{onePathOmissionWarning},
		},
		{
			name:          "one byte over stops without later refill",
			special:       over,
			includeLater:  true,
			expectedBytes: maxJSONPathBytes + 1,
			wantPaths:     []string{persona.display},
			wantWarnings:  []string{twoPathOmissionWarning},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := explicitJSONPathArraySize(persona, tt.special); got != tt.expectedBytes {
				t.Fatalf("independent JSON wire size = %d, want %d", got, tt.expectedBytes)
			}

			got, err := knowledge.BuildRoster(pathBudgetInput(tt.special, tt.includeLater))
			if err != nil {
				t.Fatalf("BuildRoster() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got.Paths, tt.wantPaths) {
				t.Errorf("BuildRoster() paths = %#v, want %#v", got.Paths, tt.wantPaths)
			}
			if !reflect.DeepEqual(got.Warnings, tt.wantWarnings) {
				t.Errorf("BuildRoster() warnings = %#v, want %#v", got.Warnings, tt.wantWarnings)
			}
		})
	}
}

type budgetPathWire struct {
	relative string
	display  string
	wire     string
}

const specialBudgetRawPrefix = "special-\"-\\-\b-\f-\n-\r-\t-\x01-\x02-\x1f-<>&-\u2028-\u2029-日本語-😀-"

const specialBudgetWirePrefix = `special-\"-\\-\b-\f-\n-\r-\t-\u0001-\u0002-\u001f-<>&-` + "\u2028-\u2029-日本語-😀-"

func specialBudgetPath(padding int) budgetPathWire {
	paddingValue := strings.Repeat("p", padding)
	const relativePrefix = "knowledge/aidlc-shared/"
	const displayPrefix = ".codex/knowledge/aidlc-shared/"
	return budgetPathWire{
		relative: relativePrefix + specialBudgetRawPrefix + paddingValue + ".md",
		display:  displayPrefix + specialBudgetRawPrefix + paddingValue + ".md",
		wire:     `"` + displayPrefix + specialBudgetWirePrefix + paddingValue + `.md"`,
	}
}

func pathBudgetInput(special budgetPathWire, includeLater bool) knowledge.RosterInput {
	files := fstest.MapFS{
		"agents/lead.md": {Data: []byte("persona")},
		special.relative: {Data: []byte("special")},
	}
	if includeLater {
		files["knowledge/aidlc-shared/zz-later.md"] = &fstest.MapFile{Data: []byte("later")}
	}
	return knowledge.RosterInput{
		Stage: graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework: knowledge.Source{
			FS:            files,
			DisplayPrefix: ".codex",
		},
		FrameworkDir: "/project/.codex",
	}
}

func explicitJSONPathArraySize(paths ...budgetPathWire) int {
	wireValues := make([]string, 0, len(paths))
	for _, path := range paths {
		wireValues = append(wireValues, path.wire)
	}
	return len([]byte("[" + strings.Join(wireValues, ",") + "]"))
}

func TestBuildRosterBoundsWarningsWithReservedSummary(t *testing.T) {
	files := fstest.MapFS{
		"agents/lead.md": {Data: []byte("persona")},
	}
	for index := 0; index < 120; index++ {
		name := "knowledge/aidlc-shared/broken-" + strings.Repeat("x", 20) + "-" + formatIndex(index) + ".md"
		files[name] = &fstest.MapFile{Data: []byte{0xff, 0xfe}}
	}
	input := knowledge.RosterInput{
		Stage:        graph.Stage{Mode: "inline", LeadAgent: "lead"},
		Framework:    knowledge.Source{FS: files, DisplayPrefix: ".codex"},
		FrameworkDir: "/project/.codex",
	}

	got, err := knowledge.BuildRoster(input)
	if err != nil {
		t.Fatalf("BuildRoster() error = %v, want nil", err)
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "additional optional persona/knowledge warning(s)") {
		t.Fatalf("BuildRoster() warnings = %#v, want omission summary", got.Warnings)
	}
	encoded, err := json.Marshal(got.Warnings)
	if err != nil {
		t.Fatalf("json.Marshal(warnings): %v", err)
	}
	if len(encoded) > 6144 {
		t.Errorf("JSON warning size = %d, want <= 6144", len(encoded))
	}
}

func formatIndex(index int) string {
	return strings.Repeat("0", 3-len(indexString(index))) + indexString(index)
}

func indexString(index int) string {
	if index == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for index > 0 {
		position--
		digits[position] = byte('0' + index%10)
		index /= 10
	}
	return string(digits[position:])
}
