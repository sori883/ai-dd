package memory_test

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/memory"
)

func TestReadSourcesReturnsFixedOrderAndExactContent(t *testing.T) {
	fsys := fstest.MapFS{
		"org.md": {
			Data: []byte("\ufeff# org\r\n"),
		},
		"team.md": {
			Data: []byte(""),
		},
		"project.md": {
			Data: []byte("---\nowner: team\n---\n# project\n"),
		},
		"phases/ideation.md": {
			Data: []byte("# phase\r\n"),
		},
		"unknown.md": {
			Data: []byte("ignored"),
		},
	}

	got, err := memory.ReadSources(fsys, "ideation")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	want := []memory.Source{
		{Layer: memory.LayerOrg, Path: "org.md", Content: "\ufeff# org\r\n"},
		{Layer: memory.LayerTeam, Path: "team.md", Content: ""},
		{Layer: memory.LayerProject, Path: "project.md", Content: "---\nowner: team\n---\n# project\n"},
		{Layer: memory.LayerPhase, Path: "phases/ideation.md", Content: "# phase\r\n"},
	}
	if len(got) != len(want) {
		t.Fatalf("ReadSources() returned %d sources, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("source %d = %#v, want %#v", index, got[index], want[index])
		}
	}
	if _, ok := fsys["unknown.md"]; !ok {
		t.Fatal("test fixture unexpectedly lost unknown.md")
	}
}

func TestReadSourcesReadsCandidatesInPriorityOrder(t *testing.T) {
	fsys := &recordingReadFileFS{files: map[string]readFileResult{
		"org.md":                 {data: []byte("org")},
		"team.md":                {data: []byte("team")},
		"project.md":             {data: []byte("project")},
		"phases/construction.md": {data: []byte("phase")},
	}}

	if _, err := memory.ReadSources(fsys, "construction"); err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	want := []string{"org.md", "team.md", "project.md", "phases/construction.md"}
	if !slices.Equal(fsys.calls, want) {
		t.Errorf("read paths = %q, want %q", fsys.calls, want)
	}
}

func TestReadSourcesSkipsMissingLayers(t *testing.T) {
	fsys := fstest.MapFS{
		"project.md": {
			Data: []byte("project"),
		},
		"phases/implementation.md": {
			Data: []byte("implementation"),
		},
	}

	got, err := memory.ReadSources(fsys, "implementation")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	want := []memory.Source{
		{Layer: memory.LayerProject, Path: "project.md", Content: "project"},
		{Layer: memory.LayerPhase, Path: "phases/implementation.md", Content: "implementation"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("ReadSources() = %#v, want %#v", got, want)
	}
}

func TestReadSourcesReturnsEmptyResultWhenAllLayersAreMissing(t *testing.T) {
	got, err := memory.ReadSources(fstest.MapFS{}, "implementation")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("ReadSources() returned nil result, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("ReadSources() returned %d sources, want zero", len(got))
	}
}

func TestReadSourcesAcceptsAnySafePhaseSlug(t *testing.T) {
	fsys := fstest.MapFS{
		"phases/future-v2.md": {Data: []byte("future phase")},
	}

	got, err := memory.ReadSources(fsys, "future-v2")
	if err != nil {
		t.Fatalf("ReadSources() error = %v, want nil", err)
	}
	want := []memory.Source{{
		Layer: memory.LayerPhase, Path: "phases/future-v2.md", Content: "future phase",
	}}
	if !slices.Equal(got, want) {
		t.Errorf("ReadSources() = %#v, want %#v", got, want)
	}
}

func TestReadSourcesRejectsInvalidPhaseBeforeIO(t *testing.T) {
	tests := []struct {
		name  string
		phase string
	}{
		{name: "empty", phase: ""},
		{name: "uppercase", phase: "Ideation"},
		{name: "leading digit", phase: "1-implementation"},
		{name: "underscore", phase: "implementation_v2"},
		{name: "path traversal", phase: "../implementation"},
		{name: "nested path", phase: "implementation/v2"},
		{name: "extension", phase: "implementation.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := &countingFS{}
			got, err := memory.ReadSources(fsys, tt.phase)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadSources() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Errorf("ReadSources() result = %#v, want nil", got)
			}
			if fsys.opens != 0 {
				t.Errorf("ReadSources() opened %d paths, want no I/O", fsys.opens)
			}
		})
	}
}

func TestReadSourcesFailsClosedOnReadError(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "read error", data: nil},
		{name: "partial data with read error", data: []byte("partial")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := errors.New("injected source read failure")
			fsys := readFileFS{files: map[string]readFileResult{
				"project.md": {data: tt.data, err: cause},
			}}
			got, err := memory.ReadSources(fsys, "implementation")
			if got != nil {
				t.Errorf("ReadSources() result = %#v, want nil", got)
			}
			if !errors.Is(err, cause) {
				t.Errorf("ReadSources() error = %v, want cause %v", err, cause)
			}
			if err == nil {
				t.Fatal("ReadSources() error = nil, want read failure")
			}
			if !strings.Contains(err.Error(), "project.md") {
				t.Errorf("ReadSources() error = %v, want path context", err)
			}
		})
	}
}

func TestReadSourcesRejectsInvalidUTF8(t *testing.T) {
	fsys := readFileFS{files: map[string]readFileResult{
		"org.md": {data: []byte{0xff, 0xfe}},
	}}

	got, err := memory.ReadSources(fsys, "implementation")
	if got != nil {
		t.Errorf("ReadSources() result = %#v, want nil", got)
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("ReadSources() error = %v, want fs.ErrInvalid", err)
	}
	if err == nil {
		t.Fatal("ReadSources() error = nil, want invalid utf-8 error")
	}
	if !strings.Contains(err.Error(), "org.md") {
		t.Errorf("ReadSources() error = %v, want path context", err)
	}
	if !strings.Contains(err.Error(), "invalid utf-8") {
		t.Errorf("ReadSources() error = %v, want invalid utf-8 context", err)
	}
}

func TestReadSourcesFailsClosedOnInvalidUTF8AfterEarlierLayer(t *testing.T) {
	fsys := readFileFS{files: map[string]readFileResult{
		"org.md":  {data: []byte("valid")},
		"team.md": {data: []byte{0xff}},
	}}

	got, err := memory.ReadSources(fsys, "implementation")
	if got != nil {
		t.Errorf("ReadSources() result = %#v, want nil", got)
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("ReadSources() error = %v, want fs.ErrInvalid", err)
	}
	if err == nil || !strings.Contains(err.Error(), "team.md") {
		t.Errorf("ReadSources() error = %v, want team.md context", err)
	}
}

func TestReadSourcesRejectsNilFS(t *testing.T) {
	tests := []struct {
		name string
		fsys fs.FS
	}{
		{name: "nil interface", fsys: nil},
		{name: "typed nil", fsys: (*countingFS)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := memory.ReadSources(tt.fsys, "implementation")
			if got != nil {
				t.Errorf("ReadSources() result = %#v, want nil", got)
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadSources() error = %v, want fs.ErrInvalid", err)
			}
		})
	}
}

func TestReadSourcesReadsFreshContentOnEveryCall(t *testing.T) {
	fsys := fstest.MapFS{
		"org.md": {Data: []byte("first")},
	}

	first, err := memory.ReadSources(fsys, "implementation")
	if err != nil {
		t.Fatalf("first ReadSources() error = %v, want nil", err)
	}
	fsys["org.md"].Data = []byte("second")
	second, err := memory.ReadSources(fsys, "implementation")
	if err != nil {
		t.Fatalf("second ReadSources() error = %v, want nil", err)
	}
	if len(first) != 1 || first[0].Content != "first" {
		t.Errorf("first ReadSources() = %#v, want first content", first)
	}
	if len(second) != 1 || second[0].Content != "second" {
		t.Errorf("second ReadSources() = %#v, want fresh second content", second)
	}
}

func TestReadSourcesReturnsCallerOwnedResults(t *testing.T) {
	fsys := fstest.MapFS{"org.md": {Data: []byte("original")}}

	got, err := memory.ReadSources(fsys, "implementation")
	if err != nil {
		t.Fatalf("first ReadSources() error = %v, want nil", err)
	}
	got[0].Content = "caller mutation"

	fresh, err := memory.ReadSources(fsys, "implementation")
	if err != nil {
		t.Fatalf("second ReadSources() error = %v, want nil", err)
	}
	if len(fresh) != 1 || fresh[0].Content != "original" {
		t.Errorf("second ReadSources() = %#v, want original content", fresh)
	}
}

type readFileResult struct {
	data []byte
	err  error
}

type readFileFS struct {
	files map[string]readFileResult
}

func (f readFileFS) Open(name string) (fs.File, error) {
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return nil, result.err
}

func (f readFileFS) ReadFile(name string) ([]byte, error) {
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return result.data, result.err
}

type recordingReadFileFS struct {
	files map[string]readFileResult
	calls []string
}

func (f *recordingReadFileFS) Open(name string) (fs.File, error) {
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return nil, result.err
}

func (f *recordingReadFileFS) ReadFile(name string) ([]byte, error) {
	f.calls = append(f.calls, name)
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return result.data, result.err
}

type countingFS struct {
	opens int
}

func (f *countingFS) Open(string) (fs.File, error) {
	f.opens++
	return nil, fs.ErrNotExist
}
