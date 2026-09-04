package steering_test

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sori883/ai-dd/src/internal/steering"
)

func TestReadRulesPreservesRequestedOrder(t *testing.T) {
	rulesFS := fstest.MapFS{
		"first.md":  {Data: []byte("first rule")},
		"second.md": {Data: []byte("second rule")},
	}

	got, err := steering.ReadRules(rulesFS, []string{"second.md", "first.md"})
	if err != nil {
		t.Fatalf("ReadRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{
		{Path: "second.md", Text: "second rule"},
		{Path: "first.md", Text: "first rule"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ReadRules() = %#v, want %#v", got, want)
	}
}

func TestReadRulesUsesFirstOccurrenceForDuplicates(t *testing.T) {
	rulesFS := &recordingReadFileFS{
		files: map[string][]byte{
			"first.md":  []byte("first rule"),
			"second.md": []byte("second rule"),
		},
	}

	got, err := steering.ReadRules(
		rulesFS,
		[]string{"first.md", "first.md", "second.md", "first.md"},
	)
	if err != nil {
		t.Fatalf("ReadRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{
		{Path: "first.md", Text: "first rule"},
		{Path: "second.md", Text: "second rule"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ReadRules() = %#v, want %#v", got, want)
	}
	if !slices.Equal(rulesFS.calls, []string{"first.md", "second.md"}) {
		t.Fatalf("read paths = %q, want first occurrence paths only", rulesFS.calls)
	}
}

func TestReadRulesReturnsNonNilEmptyWithoutReading(t *testing.T) {
	rulesFS := &countingFS{}

	got, err := steering.ReadRules(rulesFS, []string{})
	if err != nil {
		t.Fatalf("ReadRules() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("ReadRules() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("ReadRules() returned %d rules, want zero", len(got))
	}
	if rulesFS.calls != 0 {
		t.Fatalf("ReadRules() performed %d reads, want zero", rulesFS.calls)
	}
}

func TestReadRulesValidatesAllPathsBeforeIO(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{name: "empty path", paths: []string{""}},
		{name: "current directory", paths: []string{"."}},
		{name: "backslash", paths: []string{"rules\\required.md"}},
		{name: "parent path", paths: []string{"../required.md"}},
		{name: "absolute path", paths: []string{"/required.md"}},
		{name: "double separator", paths: []string{"rules//required.md"}},
		{name: "invalid path after valid path", paths: []string{"valid.md", "invalid\\path.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rulesFS := &countingFS{}

			got, err := steering.ReadRules(rulesFS, tt.paths)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadRules() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Errorf("ReadRules() result = %#v, want nil", got)
			}
			if rulesFS.calls != 0 {
				t.Errorf("ReadRules() performed %d reads, want zero", rulesFS.calls)
			}
		})
	}
}

func TestReadRulesRejectsNilFilesystem(t *testing.T) {
	var typedNil *countingFS
	tests := []struct {
		name string
		fsys fs.FS
	}{
		{name: "nil interface", fsys: nil},
		{name: "typed nil", fsys: typedNil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err, panicValue := readRulesSafely(tt.fsys, []string{"required.md"})
			if panicValue != nil {
				t.Fatalf("ReadRules() panicked: %v", panicValue)
			}
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("ReadRules() error = %v, want fs.ErrInvalid", err)
			}
			if got != nil {
				t.Errorf("ReadRules() result = %#v, want nil", got)
			}
		})
	}
}

func TestReadRulesFailsClosedOnReadError(t *testing.T) {
	readFailure := errors.New("injected rule read failure")
	tests := []struct {
		name      string
		files     map[string]readFileResult
		paths     []string
		cause     error
		path      string
		wantCalls []string
	}{
		{
			name:  "missing required rule",
			files: map[string]readFileResult{},
			paths: []string{"required.md"},
			cause: fs.ErrNotExist,
			path:  "required.md",
		},
		{
			name: "partial read failure",
			files: map[string]readFileResult{
				"required.md": {data: []byte("partial rule"), err: readFailure},
			},
			paths:     []string{"required.md", "after.md"},
			cause:     readFailure,
			path:      "required.md",
			wantCalls: []string{"required.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rulesFS := &readFileFS{files: tt.files}

			got, err := steering.ReadRules(rulesFS, tt.paths)
			if err == nil {
				t.Fatal("ReadRules() error = nil, want read failure")
			}
			if !errors.Is(err, tt.cause) {
				t.Errorf("ReadRules() error = %v, want cause %v", err, tt.cause)
			}
			if !strings.Contains(err.Error(), tt.path) {
				t.Errorf("ReadRules() error = %v, want path context %q", err, tt.path)
			}
			if got != nil {
				t.Errorf("ReadRules() result = %#v, want nil", got)
			}
			if tt.wantCalls != nil && !slices.Equal(rulesFS.calls, tt.wantCalls) {
				t.Errorf("read paths = %q, want %q", rulesFS.calls, tt.wantCalls)
			}
		})
	}
}

func TestReadRulesRejectsInvalidUTF8(t *testing.T) {
	rulesFS := &readFileFS{files: map[string]readFileResult{
		"required.md": {data: []byte{0xff, 0xfe}},
	}}

	got, err := steering.ReadRules(rulesFS, []string{"required.md"})
	if err == nil {
		t.Fatal("ReadRules() error = nil, want invalid utf-8 error")
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("ReadRules() error = %v, want fs.ErrInvalid", err)
	}
	if !strings.Contains(err.Error(), "required.md") {
		t.Errorf("ReadRules() error = %v, want path context", err)
	}
	if !strings.Contains(err.Error(), "invalid utf-8") {
		t.Errorf("ReadRules() error = %v, want invalid utf-8 context", err)
	}
	if got != nil {
		t.Errorf("ReadRules() result = %#v, want nil", got)
	}
}

func TestReadRulesFailsClosedOnInvalidUTF8AfterEarlierRule(t *testing.T) {
	rulesFS := &readFileFS{files: map[string]readFileResult{
		"first.md":  {data: []byte("first rule")},
		"second.md": {data: []byte{0xff}},
	}}

	got, err := steering.ReadRules(rulesFS, []string{"first.md", "second.md", "after.md"})
	if err == nil {
		t.Fatal("ReadRules() error = nil, want invalid utf-8 error")
	}
	if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("ReadRules() error = %v, want fs.ErrInvalid", err)
	}
	if got != nil {
		t.Errorf("ReadRules() result = %#v, want nil", got)
	}
	if !slices.Equal(rulesFS.calls, []string{"first.md", "second.md"}) {
		t.Errorf("read paths = %q, want reads through invalid rule only", rulesFS.calls)
	}
}

func TestReadRulesRemovesOnlyLeadingBOM(t *testing.T) {
	rulesFS := &readFileFS{files: map[string]readFileResult{
		"required.md": {
			data: []byte("\ufefffirst rule\r\n<!-- keep this comment -->\nsecond rule\n\ufefftail"),
		},
	}}

	got, err := steering.ReadRules(rulesFS, []string{"required.md"})
	if err != nil {
		t.Fatalf("ReadRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{{
		Path: "required.md",
		Text: "first rule\r\n<!-- keep this comment -->\nsecond rule\n\ufefftail",
	}}
	if !slices.Equal(got, want) {
		t.Fatalf("ReadRules() = %#v, want %#v", got, want)
	}
}

func TestReadRulesFiltersNonSubstantiveTemplates(t *testing.T) {
	preamble := strings.Join([]string{
		"> This team's affirmed practices and corrections. Loaded after `org.md` as",
		"> strict-additive guidance; contradictions with broader policy are rejected.",
		"> Populated by the practices-discovery affirmation gate. Edit at the gate,",
		"> not directly.",
		"> Project-specific specialisation and corrections. Loaded after `org.md` and",
		"> `team.md` as strict-additive guidance; contradictions with broader policy",
		"> are rejected. Populated by practices-discovery and the self-learning loop.",
		">",
		"> Use sparingly: most teams don't need a project layer. Reach for it",
		"> only when this specific project needs stable, durable guidance beyond the",
		"> team practice (for example, package-specific release checks or an additional",
		"> regression suite for a legacy component).",
	}, "\n")
	tests := []struct {
		name    string
		content string
		want    []steering.RuleContent
	}{
		{name: "headings", content: "# heading\n## another", want: []steering.RuleContent{}},
		{name: "frontmatter separators", content: "---\n---", want: []steering.RuleContent{}},
		{name: "comment only", content: "<!-- hidden -->", want: []steering.RuleContent{}},
		{name: "shipped template preamble", content: preamble, want: []steering.RuleContent{}},
		{
			name:    "authored body and comment",
			content: "> authored guidance\n<!-- keep this comment -->\nactual rule",
			want: []steering.RuleContent{{
				Path: "required.md",
				Text: "> authored guidance\n<!-- keep this comment -->\nactual rule",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rulesFS := &readFileFS{files: map[string]readFileResult{
				"required.md": {data: []byte(tt.content)},
			}}

			got, err := steering.ReadRules(rulesFS, []string{"required.md"})
			if err != nil {
				t.Fatalf("ReadRules() error = %v, want nil", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("ReadRules() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReadRulesReadsFreshContentOnEveryCall(t *testing.T) {
	rulesFS := fstest.MapFS{
		"required.md": {Data: []byte("first rule")},
	}
	paths := []string{"required.md"}
	originalPaths := slices.Clone(paths)

	first, err := steering.ReadRules(rulesFS, paths)
	if err != nil {
		t.Fatalf("first ReadRules() error = %v, want nil", err)
	}
	if !slices.Equal(paths, originalPaths) {
		t.Fatalf("ReadRules() mutated paths: got %q, want %q", paths, originalPaths)
	}
	if len(first) != 1 || first[0].Text != "first rule" {
		t.Fatalf("first ReadRules() = %#v, want first rule", first)
	}

	first[0].Text = "caller mutation"
	rulesFS["required.md"].Data = []byte("second rule")
	second, err := steering.ReadRules(rulesFS, paths)
	if err != nil {
		t.Fatalf("second ReadRules() error = %v, want nil", err)
	}
	want := []steering.RuleContent{{Path: "required.md", Text: "second rule"}}
	if !slices.Equal(second, want) {
		t.Fatalf("second ReadRules() = %#v, want %#v", second, want)
	}
}

type recordingReadFileFS struct {
	files map[string][]byte
	calls []string
}

type readFileResult struct {
	data []byte
	err  error
}

type readFileFS struct {
	files map[string]readFileResult
	calls []string
}

func (f *readFileFS) Open(name string) (fs.File, error) {
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return nil, result.err
}

func (f *readFileFS) ReadFile(name string) ([]byte, error) {
	f.calls = append(f.calls, name)
	result, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return result.data, result.err
}

func (f *recordingReadFileFS) Open(string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (f *recordingReadFileFS) ReadFile(name string) ([]byte, error) {
	f.calls = append(f.calls, name)
	content, ok := f.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return content, nil
}

func readRulesSafely(
	rulesFS fs.FS,
	paths []string,
) (rules []steering.RuleContent, err error, panicValue any) {
	defer func() {
		panicValue = recover()
	}()
	rules, err = steering.ReadRules(rulesFS, paths)
	return rules, err, nil
}

type countingFS struct {
	calls int
}

func (f *countingFS) Open(string) (fs.File, error) {
	f.calls++
	return nil, fs.ErrNotExist
}
