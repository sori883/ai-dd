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
