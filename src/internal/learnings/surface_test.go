package learnings

import (
	"errors"
	"strings"
	"testing"
)

func TestParseIntentCaptureLearningSurface(t *testing.T) {
	content := "## Interpretations\n- [desc] Users need one flow\n~~~md\n- hidden rule\n~~~\n<!--\n- hidden comment\n-->\n\n## Deviations\n- Keep the local format\n\n## Tradeoffs\n1. Prefer deterministic output\n\n## Open questions\n- Park this question\n"
	surface, err := ParseSurface(SurfaceInput{Stage: "intent-capture", Identity: "project\x00team\x00build", Space: "team", Intent: "build", Content: []byte(content)})
	if err != nil {
		t.Fatalf("ParseSurface(): %v", err)
	}
	if len(surface.Candidates) != 3 {
		t.Fatalf("candidates = %#v, want 3 visible entries", surface.Candidates)
	}
	if surface.Candidates[0].ID != "c1" || surface.Candidates[0].Heading != "Interpretations" || surface.Candidates[1].ID != "c2" || surface.Candidates[2].ID != "c3" {
		t.Fatalf("candidates = %#v, want stable c1..c3 headings", surface.Candidates)
	}
	if strings.Contains(surface.Candidates[0].Text, "hidden") || len(surface.Parked) != 1 || surface.Parked[0] != "Park this question" {
		t.Fatalf("surface = %#v, want hidden entries excluded and open question parked", surface)
	}
}

func TestParseIntentCaptureLearningSurfaceParsesCanonicalEntry(t *testing.T) {
	surface, err := ParseSurface(SurfaceInput{
		Stage: "intent-capture", Identity: "project\x00team\x00build", Space: "team", Intent: "build",
		Content: []byte("## Interpretations\n- 2026-09-06T01:02:03Z — Keep the source register; preserve the visible citation\n"),
	})
	if err != nil {
		t.Fatalf("ParseSurface(): %v", err)
	}
	if len(surface.Candidates) != 1 {
		t.Fatalf("candidates = %#v, want one candidate", surface.Candidates)
	}
	candidate := surface.Candidates[0]
	if candidate.ID != "c1" || candidate.Timestamp != "2026-09-06T01:02:03Z" || candidate.Summary != "Keep the source register" || candidate.Context != "preserve the visible citation" || candidate.Text != candidate.Summary || candidate.Source != "Interpretations" || candidate.Scope != "project" {
		t.Fatalf("candidate = %#v, want canonical timestamp/summary/context/source/scope", candidate)
	}
}

func TestParseIntentCaptureLearningSurfaceCountsVisibleProseAndStopsAtUnknownH2(t *testing.T) {
	surface, err := ParseSurface(SurfaceInput{
		Stage: "intent-capture", Identity: "project\x00team\x00build", Space: "team", Intent: "build",
		Content: []byte("## Interpretations\nA visible prose entry\n\n- A list entry\n## Not a learning section\n- ignored after unknown section\n## Tradeoffs\nA second prose entry\n"),
	})
	if err != nil {
		t.Fatalf("ParseSurface(): %v", err)
	}
	if len(surface.Candidates) != 3 {
		t.Fatalf("candidates = %#v, want one candidate per visible substantive entry", surface.Candidates)
	}
	want := []string{"A visible prose entry", "A list entry", "A second prose entry"}
	for index, candidate := range surface.Candidates {
		if candidate.Text != want[index] {
			t.Errorf("candidate[%d].Text = %q, want %q", index, candidate.Text, want[index])
		}
	}
}

func TestParseIntentCaptureLearningSurfaceRejectsUnsafeContent(t *testing.T) {
	_, err := ParseSurface(SurfaceInput{Stage: "intent-capture", Identity: "identity", Content: []byte{0xff}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("ParseSurface(non-UTF8) error = %v, want ErrInvalid", err)
	}
	_, err = ParseSurface(SurfaceInput{Stage: "intent-capture", Identity: "identity", Content: []byte("## Interpretations\n- one\n## Interpretations\n- two\n")})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("ParseSurface(duplicate heading) error = %v, want ErrInvalid", err)
	}
}

func TestValidateSelectionsRejectsCallerAuthority(t *testing.T) {
	selection := Selection{CandidateID: "c1", Type: SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: "Use deterministic output", Source: "orchestrator"}
	if err := ValidateSelections([]Selection{selection}); err != nil {
		t.Fatalf("ValidateSelections(valid learning): %v", err)
	}
	for _, tc := range []struct {
		name string
		bad  Selection
	}{
		{name: "scope", bad: Selection{CandidateID: "c1", Type: SelectionTypeLearning, Scope: "org", Heading: "Corrections", Text: "x", Source: "user_addition"}},
		{name: "authority", bad: Selection{CandidateID: "c1", Type: SelectionTypeSensor, OriginStage: "intent-capture", ManifestFields: map[string]string{"fire_id": "forged"}, Source: "orchestrator"}},
		{name: "path", bad: Selection{CandidateID: "../escape", Type: SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: "x", Source: "user_addition"}},
		{name: "learning control", bad: Selection{CandidateID: "c1", Type: SelectionTypeLearning, Scope: "project", Heading: "Corrections", Text: "x\ny", Source: "user_addition"}},
		{name: "manifest control", bad: Selection{CandidateID: "c1", Type: SelectionTypeSensor, OriginStage: "intent-capture", ManifestFields: map[string]string{"id": "s", "kind": "check", "command": "x\ty", "default_severity": "advisory", "description": "x", "matches": "**/*"}, Source: "user_addition"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSelections([]Selection{tc.bad}); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateSelections(%#v) error = %v, want ErrInvalid", tc.bad, err)
			}
		})
	}
}
