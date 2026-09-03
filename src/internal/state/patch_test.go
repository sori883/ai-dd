package state

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestPatchStageMarkerPreservesSuffixAndInput(t *testing.T) {
	t.Parallel()

	input := []byte("\ufeff" + strings.ReplaceAll(
		strings.Replace(
			canonicalStateContent()+"\n## Unknown\n<!-- preserve this comment -->\n",
			"- [-] intent-capture — EXECUTE\n",
			"- [-] intent-capture — EXECUTE — retain this suffix  \n",
			1,
		),
		"\n",
		"\r\n",
	))
	original := bytes.Clone(input)

	got, err := Patch(input, PatchRequest{
		StageMarkers: []StageMarkerPatch{
			{
				Slug:        "intent-capture",
				Expected:    StageMarkerInProgress,
				Replacement: StageMarkerAwaitingApproval,
			},
		},
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}

	want := strings.Replace(
		string(input),
		"- [-] intent-capture — EXECUTE — retain this suffix  ",
		"- [?] intent-capture — EXECUTE — retain this suffix  ",
		1,
	)
	if string(got) != want {
		t.Fatalf("Patch() changed unexpected bytes:\n got %q\nwant %q", got, want)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Patch() mutated input bytes")
	}
	if _, err := Parse(got); err != nil {
		t.Fatalf("Parse(Patch()) error = %v", err)
	}
}

func TestPatchStageMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		slug        string
		expected    StageMarker
		replacement StageMarker
	}{
		{
			name:        "in progress to awaiting approval",
			slug:        "intent-capture",
			expected:    StageMarkerInProgress,
			replacement: StageMarkerAwaitingApproval,
		},
		{
			name:        "awaiting approval to completed",
			slug:        "reverse-engineering",
			expected:    StageMarkerAwaitingApproval,
			replacement: StageMarkerCompleted,
		},
		{
			name:        "awaiting approval to revising",
			slug:        "reverse-engineering",
			expected:    StageMarkerAwaitingApproval,
			replacement: StageMarkerRevising,
		},
		{
			name:        "revising to awaiting approval",
			slug:        "code-generation",
			expected:    StageMarkerRevising,
			replacement: StageMarkerAwaitingApproval,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Patch([]byte(canonicalStateContent()), PatchRequest{
				StageMarkers: []StageMarkerPatch{{
					Slug:        tt.slug,
					Expected:    tt.expected,
					Replacement: tt.replacement,
				}},
			})
			if err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
			parsed, err := Parse(got)
			if err != nil {
				t.Fatalf("Parse(Patch()) error = %v", err)
			}
			var found bool
			for _, stage := range parsed.Stages() {
				if stage.Slug != tt.slug {
					continue
				}
				found = true
				if stage.CheckboxMarker != string(tt.replacement) {
					t.Errorf("stage marker = %q, want %q", stage.CheckboxMarker, tt.replacement)
				}
				if stage.Suffix == "" {
					t.Error("stage suffix was lost")
				}
			}
			if !found {
				t.Fatalf("stage %q was not found after patch", tt.slug)
			}
		})
	}
}

func TestPatchRejectsMarkerAmbiguityAndStaleExpected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request PatchRequest
	}{
		{
			name: "duplicate target",
			request: PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarkerAwaitingApproval},
				{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarkerCompleted},
			}},
		},
		{
			name: "missing target",
			request: PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "missing-stage", Expected: StageMarkerPending, Replacement: StageMarkerCompleted},
			}},
		},
		{
			name: "expected marker mismatch",
			request: PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "intent-capture", Expected: StageMarkerPending, Replacement: StageMarkerCompleted},
			}},
		},
		{
			name: "invalid expected marker",
			request: PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "intent-capture", Expected: StageMarker("invalid"), Replacement: StageMarkerCompleted},
			}},
		},
		{
			name: "invalid replacement marker",
			request: PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarker("invalid")},
			}},
		},
		{
			name:    "empty request",
			request: PatchRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(canonicalStateContent())
			original := bytes.Clone(input)
			got, err := Patch(input, tt.request)
			if err == nil {
				t.Fatal("Patch() error = nil, want fs.ErrInvalid")
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Patch() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Fatalf("Patch() output = %q, want nil partial result", got)
			}
			if !bytes.Equal(input, original) {
				t.Fatal("Patch() mutated input bytes after rejection")
			}
		})
	}
}

func TestPatchStageMarkerIgnoresDecoyOutsideCanonicalSection(t *testing.T) {
	t.Parallel()

	input := []byte(canonicalStateContent() +
		"\n## Unknown\n- [-] intent-capture — EXECUTE\n")
	got, err := Patch(input, PatchRequest{StageMarkers: []StageMarkerPatch{
		{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarkerAwaitingApproval},
	}})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	want := strings.Replace(string(input), "- [-] intent-capture — EXECUTE", "- [?] intent-capture — EXECUTE", 1)
	if string(got) != want {
		t.Fatalf("Patch() changed decoy bytes:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(string(got), "## Unknown\n- [-] intent-capture — EXECUTE") {
		t.Fatal("Patch() changed a Stage marker decoy outside Stage Progress")
	}
}

func TestPatchPreservesTerminalNewlineFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "lf with terminal newline", content: canonicalStateContent()},
		{name: "lf without terminal newline", content: strings.TrimSuffix(canonicalStateContent(), "\n")},
		{name: "crlf with terminal newline", content: strings.ReplaceAll(canonicalStateContent(), "\n", "\r\n")},
		{name: "crlf without terminal newline", content: strings.TrimSuffix(strings.ReplaceAll(canonicalStateContent(), "\n", "\r\n"), "\r\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(tt.content)
			got, err := Patch(input, PatchRequest{StageMarkers: []StageMarkerPatch{
				{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarkerAwaitingApproval},
			}})
			if err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
			if strings.HasSuffix(tt.content, "\r\n") && !bytes.HasSuffix(got, []byte("\r\n")) {
				t.Error("Patch() did not preserve CRLF terminal newline")
			}
			if strings.HasSuffix(tt.content, "\n") && !strings.HasSuffix(tt.content, "\r\n") && !bytes.HasSuffix(got, []byte("\n")) {
				t.Error("Patch() did not preserve LF terminal newline")
			}
			if !strings.HasSuffix(tt.content, "\n") && bytes.HasSuffix(got, []byte("\n")) {
				t.Error("Patch() added terminal newline")
			}
			if !strings.Contains(string(got), "- [?] intent-capture — EXECUTE") {
				t.Error("Patch() did not replace marker")
			}
		})
	}
}

func TestPatchRejectsMalformedInputWithoutPartialOutput(t *testing.T) {
	t.Parallel()

	input := []byte(strings.Replace(canonicalStateContent(), "- [?] reverse-engineering", "- [!] reverse-engineering", 1))
	original := bytes.Clone(input)
	got, err := Patch(input, PatchRequest{StageMarkers: []StageMarkerPatch{
		{Slug: "intent-capture", Expected: StageMarkerInProgress, Replacement: StageMarkerAwaitingApproval},
	}})
	if err == nil {
		t.Fatal("Patch() error = nil, want malformed input rejection")
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("Patch() error = %v, want fs.ErrInvalid", err)
	}
	if got != nil {
		t.Fatalf("Patch() output = %q, want nil partial result", got)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Patch() mutated malformed input")
	}
}

func TestPatchCanonicalFieldsPreservesRawBytes(t *testing.T) {
	t.Parallel()

	input := []byte("\ufeff" + strings.ReplaceAll(
		canonicalStateContent()+"\n## Unknown\nunknown bytes  \n",
		"\n",
		"\r\n",
	))
	original := bytes.Clone(input)
	request := PatchRequest{Fields: []FieldPatch{
		{Field: CanonicalFieldTotalStages, Expected: "5", Replacement: "6"},
		{Field: CanonicalFieldCompleted, Expected: "2", Replacement: "3"},
		{Field: CanonicalFieldInProgress, Expected: "intent-capture", Replacement: "reverse-engineering"},
		{Field: CanonicalFieldLifecyclePhase, Expected: "IDEATION", Replacement: "INCEPTION"},
		{Field: CanonicalFieldCurrentStage, Expected: "intent-capture", Replacement: "reverse-engineering"},
		{Field: CanonicalFieldNextStage, Expected: "reverse-engineering", Replacement: "code-generation"},
		{Field: CanonicalFieldStatus, Expected: "Running", Replacement: "Completed"},
		{Field: CanonicalFieldLastUpdated, Expected: "2026-09-02T00:00:00Z", Replacement: "2026-09-03T00:00:00Z"},
	}}

	got, err := Patch(input, request)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	want := string(input)
	for _, replacement := range []struct{ old, new string }{
		{"- **Total Stages**: 5", "- **Total Stages**: 6"},
		{"- **Completed**: 2", "- **Completed**: 3"},
		{"- **In Progress**: intent-capture", "- **In Progress**: reverse-engineering"},
		{"- **Lifecycle Phase**: IDEATION", "- **Lifecycle Phase**: INCEPTION"},
		{"- **Current Stage**: intent-capture", "- **Current Stage**: reverse-engineering"},
		{"- **Next Stage**: reverse-engineering", "- **Next Stage**: code-generation"},
		{"- **Status**: Running", "- **Status**: Completed"},
		{"- **Last Updated**: 2026-09-02T00:00:00Z", "- **Last Updated**: 2026-09-03T00:00:00Z"},
	} {
		want = strings.Replace(want, replacement.old, replacement.new, 1)
	}
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("Patch() changed unexpected bytes:\n got %q\nwant %q", got, want)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Patch() mutated input bytes")
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatalf("Parse(Patch()) error = %v", err)
	}
	if parsed.Summary() != (Summary{TotalStages: 6, Completed: 3, InProgress: "reverse-engineering"}) {
		t.Errorf("Summary() = %#v", parsed.Summary())
	}
	if parsed.LifecyclePhase() != LifecyclePhaseInception ||
		parsed.CurrentStage() != "reverse-engineering" ||
		parsed.NextStage() != "code-generation" ||
		parsed.WorkflowStatus() != WorkflowStatusCompleted {
		t.Errorf("parsed current status = phase %q current %q next %q status %q",
			parsed.LifecyclePhase(), parsed.CurrentStage(), parsed.NextStage(), parsed.WorkflowStatus())
	}
}

func TestPatchFieldIgnoresDecoyOutsideCanonicalSection(t *testing.T) {
	t.Parallel()

	input := []byte(canonicalStateContent() + "\n## Unknown\n- **Status**: Running\n")
	got, err := Patch(input, PatchRequest{Fields: []FieldPatch{
		{Field: CanonicalFieldStatus, Expected: "Running", Replacement: "Completed"},
	}})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	want := strings.Replace(string(input), "- **Status**: Running", "- **Status**: Completed", 1)
	if string(got) != want {
		t.Fatalf("Patch() changed decoy bytes:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(string(got), "## Unknown\n- **Status**: Running") {
		t.Fatal("Patch() changed a field decoy outside its canonical section")
	}
}

func TestPatchFieldPreservesTargetLineWhitespace(t *testing.T) {
	t.Parallel()

	input := []byte(strings.Replace(
		canonicalStateContent(),
		"- **Current Stage**: intent-capture\n",
		"- **Current Stage**:\t intent-capture  \n",
		1,
	))
	got, err := Patch(input, PatchRequest{Fields: []FieldPatch{
		{Field: CanonicalFieldCurrentStage, Expected: "intent-capture", Replacement: "reverse-engineering"},
	}})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	want := strings.Replace(
		string(input),
		"- **Current Stage**:\t intent-capture  ",
		"- **Current Stage**:\t reverse-engineering  ",
		1,
	)
	if string(got) != want {
		t.Fatalf("Patch() did not preserve target line whitespace:\n got %q\nwant %q", got, want)
	}
	if _, err := Parse(got); err != nil {
		t.Fatalf("Parse(Patch()) error = %v", err)
	}
}

func TestPatchRejectsFieldAmbiguityAndInjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content func() []byte
		request PatchRequest
	}{
		{
			name: "unknown field",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalField("Scope"), Expected: "classic", Replacement: "other"},
			}},
		},
		{
			name: "duplicate field target",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldCompleted, Expected: "2", Replacement: "3"},
				{Field: CanonicalFieldCompleted, Expected: "2", Replacement: "4"},
			}},
		},
		{
			name: "expected field mismatch",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldCurrentStage, Expected: "other-stage", Replacement: "next-stage"},
			}},
		},
		{
			name: "missing field",
			content: func() []byte {
				return []byte(strings.Replace(canonicalStateContent(),
					"- **Last Updated**: 2026-09-02T00:00:00Z\n", "", 1))
			},
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldLastUpdated, Expected: "2026-09-02T00:00:00Z", Replacement: "2026-09-03T00:00:00Z"},
			}},
		},
		{
			name: "decoy field outside canonical section",
			content: func() []byte {
				return []byte(strings.Replace(canonicalStateContent(),
					"- **Last Updated**: 2026-09-02T00:00:00Z\n", "", 1) +
					"\n## Unknown\n- **Last Updated**: 2026-09-02T00:00:00Z\n")
			},
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldLastUpdated, Expected: "2026-09-02T00:00:00Z", Replacement: "2026-09-03T00:00:00Z"},
			}},
		},
		{
			name: "duplicate field in canonical section",
			content: func() []byte {
				return []byte(strings.Replace(canonicalStateContent(),
					"- **Last Updated**: 2026-09-02T00:00:00Z\n",
					"- **Last Updated**: 2026-09-02T00:00:00Z\n- **Last Updated**: 2026-09-02T00:00:00Z\n", 1))
			},
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldLastUpdated, Expected: "2026-09-02T00:00:00Z", Replacement: "2026-09-03T00:00:00Z"},
			}},
		},
		{
			name: "scalar newline injection",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldCurrentStage, Expected: "intent-capture", Replacement: "next-stage\n## Injected"},
			}},
		},
		{
			name: "scalar surrounding whitespace",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldCurrentStage, Expected: "intent-capture", Replacement: " next-stage"},
			}},
		},
		{
			name: "invalid lifecycle phase",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldLifecyclePhase, Expected: "IDEATION", Replacement: "ideation"},
			}},
		},
		{
			name: "invalid workflow status",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldStatus, Expected: "Running", Replacement: "running"},
			}},
		},
		{
			name: "noncanonical count",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldCompleted, Expected: "2", Replacement: "03"},
			}},
		},
		{
			name: "stage field whitespace",
			request: PatchRequest{Fields: []FieldPatch{
				{Field: CanonicalFieldNextStage, Expected: "reverse-engineering", Replacement: "next stage"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(canonicalStateContent())
			if tt.content != nil {
				input = tt.content()
			}
			original := bytes.Clone(input)
			got, err := Patch(input, tt.request)
			if err == nil {
				t.Fatal("Patch() error = nil, want fs.ErrInvalid")
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Patch() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Fatalf("Patch() output = %q, want nil partial result", got)
			}
			if !bytes.Equal(input, original) {
				t.Fatal("Patch() mutated input bytes after rejection")
			}
		})
	}
}

func TestPatchPhaseProgressPreservesRawBytes(t *testing.T) {
	t.Parallel()

	input := []byte("\ufeff" + strings.ReplaceAll(
		canonicalStateContent()+"\n## Unknown\nphase comment  \n",
		"\n",
		"\r\n",
	))
	original := bytes.Clone(input)
	request := PatchRequest{PhaseProgress: []PhaseProgressPatch{
		{Phase: LifecyclePhaseInitialization, Expected: PhaseStatusVerified, Replacement: PhaseStatusActive},
		{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusActive, Replacement: PhaseStatusVerified},
	}}

	got, err := Patch(input, request)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	want := string(input)
	want = strings.Replace(want, "- **Initialization**: Verified", "- **Initialization**: Active", 1)
	want = strings.Replace(want, "- **Ideation**: Active", "- **Ideation**: Verified", 1)
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("Patch() changed unexpected bytes:\n got %q\nwant %q", got, want)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Patch() mutated input bytes")
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatalf("Parse(Patch()) error = %v", err)
	}
	phases := parsed.PhaseProgress()
	if phases[0].Status != PhaseStatusActive || phases[1].Status != PhaseStatusVerified {
		t.Fatalf("PhaseProgress() = %#v, want initialization Active and ideation Verified", phases[:2])
	}
}

func TestPatchRejectsPhaseProgressAmbiguity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content func() []byte
		request PatchRequest
	}{
		{
			name: "unknown phase",
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhase("UNKNOWN"), Expected: PhaseStatusPending, Replacement: PhaseStatusActive},
			}},
		},
		{
			name: "duplicate phase target",
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusActive, Replacement: PhaseStatusVerified},
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusActive, Replacement: PhaseStatusPending},
			}},
		},
		{
			name: "expected status mismatch",
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusPending, Replacement: PhaseStatusVerified},
			}},
		},
		{
			name: "invalid expected status",
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatus("active"), Replacement: PhaseStatusVerified},
			}},
		},
		{
			name: "invalid replacement status",
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusActive, Replacement: PhaseStatus("verified")},
			}},
		},
		{
			name: "duplicate phase row",
			content: func() []byte {
				return []byte(strings.Replace(canonicalStateContent(),
					"- **Ideation**: Active\n", "- **Ideation**: Active\n- **Ideation**: Active\n", 1))
			},
			request: PatchRequest{PhaseProgress: []PhaseProgressPatch{
				{Phase: LifecyclePhaseIdeation, Expected: PhaseStatusActive, Replacement: PhaseStatusVerified},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(canonicalStateContent())
			if tt.content != nil {
				input = tt.content()
			}
			original := bytes.Clone(input)
			got, err := Patch(input, tt.request)
			if err == nil {
				t.Fatal("Patch() error = nil, want fs.ErrInvalid")
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("Patch() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Fatalf("Patch() output = %q, want nil partial result", got)
			}
			if !bytes.Equal(input, original) {
				t.Fatal("Patch() mutated input bytes after rejection")
			}
		})
	}
}

func TestPatchNextActionAllowsInternalSpacesAndRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	input := canonicalStateContent() + "\n## Session Resume Point\n" +
		"- **Last Completed Stage**: state-init\n" +
		"- **Next Action**: Execute intent-capture\n"

	tests := []struct {
		name        string
		replacement string
		wantErr     bool
	}{
		{name: "internal spaces are accepted", replacement: "Execute the next stage"},
		{name: "newline is rejected", replacement: "Execute next\nstage", wantErr: true},
		{name: "line separator is rejected", replacement: "Execute next\u2028stage", wantErr: true},
		{name: "paragraph separator is rejected", replacement: "Execute next\u2029stage", wantErr: true},
		{name: "control character is rejected", replacement: "Execute next\x00stage", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Patch([]byte(input), PatchRequest{Fields: []FieldPatch{{
				Field:       CanonicalFieldNextAction,
				Expected:    "Execute intent-capture",
				Replacement: tt.replacement,
			}}})
			if tt.wantErr {
				if err == nil {
					t.Fatal("Patch() error = nil, want invalid patch")
				}
				return
			}
			if err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
			gotAction, err := NextAction(got)
			if err != nil {
				t.Fatalf("NextAction(Patch()) error = %v", err)
			}
			if gotAction != tt.replacement {
				t.Fatalf("NextAction(Patch()) = %q, want %q", gotAction, tt.replacement)
			}
		})
	}
}
