package state

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestRevisionCountRequiresUniqueCanonicalRuntimeField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name:    "canonical value",
			content: withRuntimeState(canonicalStateContent(), "0"),
			want:    0,
		},
		{
			name:    "large canonical value",
			content: withRuntimeState(canonicalStateContent(), "9223372036854775807"),
			want:    int(^uint(0) >> 1),
		},
		{
			name:    "missing runtime section",
			content: canonicalStateContent(),
			wantErr: true,
		},
		{
			name:    "missing revision field",
			content: strings.Replace(withRuntimeState(canonicalStateContent(), "0"), "- **Revision Count**: 0\n\n", "", 1),
			wantErr: true,
		},
		{
			name:    "duplicate revision field",
			content: strings.Replace(withRuntimeState(canonicalStateContent(), "0"), "- **Revision Count**: 0\n", "- **Revision Count**: 0\n- **Revision Count**: 1\n", 1),
			wantErr: true,
		},
		{
			name:    "noncanonical decimal",
			content: withRuntimeState(canonicalStateContent(), "01"),
			wantErr: true,
		},
		{
			name:    "negative",
			content: withRuntimeState(canonicalStateContent(), "-1"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := RevisionCount([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RevisionCount() error = nil, want fs.ErrInvalid")
				}
				if !errors.Is(err, fs.ErrInvalid) {
					t.Fatalf("RevisionCount() error = %v, want fs.ErrInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("RevisionCount() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("RevisionCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLastUpdatedRequiresUniqueCanonicalCurrentStatusField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "canonical value",
			content: canonicalStateContent(),
			want:    "2026-09-02T00:00:00Z",
		},
		{
			name: "missing field",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Last Updated**: 2026-09-02T00:00:00Z\n",
				"",
				1,
			),
			wantErr: true,
		},
		{
			name: "duplicate field",
			content: strings.Replace(
				canonicalStateContent(),
				"- **Last Updated**: 2026-09-02T00:00:00Z\n",
				"- **Last Updated**: 2026-09-02T00:00:00Z\n- **Last Updated**: 2026-09-03T00:00:00Z\n",
				1,
			),
			wantErr: true,
		},
		{
			name:    "unknown section decoy is ignored",
			content: canonicalStateContent() + "\n## Unknown\n- **Last Updated**: decoy\n",
			want:    "2026-09-02T00:00:00Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LastUpdated([]byte(tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("LastUpdated() error = nil, want fs.ErrInvalid")
				}
				if !errors.Is(err, fs.ErrInvalid) {
					t.Fatalf("LastUpdated() error = %v, want fs.ErrInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LastUpdated() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("LastUpdated() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPatchRevisionCountPreservesUnknownBytes(t *testing.T) {
	t.Parallel()

	input := []byte("\ufeff" + withRuntimeState(canonicalStateContent(), "2") + "\r\n## Unknown\r\ncomment  \r\n")
	want := strings.Replace(string(input), "- **Revision Count**: 2", "- **Revision Count**: 3", 1)

	got, err := Patch(input, PatchRequest{Fields: []FieldPatch{{
		Field:       CanonicalFieldRevisionCount,
		Expected:    "2",
		Replacement: "3",
	}}})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("Patch() bytes = %q, want %q", got, want)
	}
	if gotCount, err := RevisionCount(got); err != nil || gotCount != 3 {
		t.Fatalf("RevisionCount(Patch()) = (%d, %v), want (3, nil)", gotCount, err)
	}
}

func withRuntimeState(content, count string) string {
	return strings.Replace(content, "## Phase Progress\n", "## Runtime State\n- **Revision Count**: "+count+"\n\n## Phase Progress\n", 1)
}
