package scope

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestReadAllLoadsMetadata(t *testing.T) {
	t.Parallel()

	scopesFS := fstest.MapFS{
		"scope.md": {Data: []byte(`---
name: feature
depth: Standard
description: "Feature delivery"
keywords:
  - feature
  - "user story"
future_field: ignored
---

# Feature
`)},
	}

	got, err := ReadAll(scopesFS)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	want := []Metadata{
		{
			Name:        "feature",
			Depth:       "Standard",
			Description: "Feature delivery",
			Keywords:    []string{"feature", "user story"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadAll() = %#v, want %#v", got, want)
	}
}

func TestReadAllUsesDirectMarkdownFilesInJavaScriptUTF16Order(t *testing.T) {
	t.Parallel()

	scopesFS := fstest.MapFS{
		"\ue000.md":       {Data: []byte("---\nname: private-use\n---\n")},
		"😀.md":            {Data: []byte("---\nname: emoji\n---\n")},
		"ignored.txt":     {Data: []byte("not frontmatter")},
		"nested/scope.md": {Data: []byte("not frontmatter")},
	}

	got, err := ReadAll(scopesFS)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if gotNames, want := metadataNames(got), []string{"emoji", "private-use"}; !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("ReadAll() names = %q, want filename UTF-16 order %q", gotNames, want)
	}
}

func TestReadAllParsesFrontmatterLineContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		body             string
		want             Metadata
		wantErrorContain string
	}{
		{
			name: "crlf quotes and block marker",
			body: "---\r\nname: \"crlf\"\r\ndepth: 'Minimal'\r\ndescription: >\r\n---\r\n",
			want: Metadata{
				Name:     "crlf",
				Depth:    "Minimal",
				Keywords: []string{},
			},
		},
		{
			name:             "empty scalar does not steal next field",
			body:             "---\nname:\ndepth: Minimal\n---\n",
			wantErrorContain: "missing required frontmatter: name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ReadAll(fstest.MapFS{"scope.md": {Data: []byte(tt.body)}})
			if tt.wantErrorContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrorContain) {
					t.Fatalf("ReadAll() error = %v, want context %q", err, tt.wantErrorContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if want := []Metadata{tt.want}; !reflect.DeepEqual(got, want) {
				t.Errorf("ReadAll() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestReadAllParsesKeywordLists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		keywords string
		want     []string
	}{
		{
			name:     "flow quotes separators and terminal comment",
			keywords: `[alpha, "comma,value", 'close]bracket'] # retained comment`,
			want:     []string{"alpha", "comma,value", "close]bracket"},
		},
		{
			name:     "flow filters empty members",
			keywords: `[alpha, "", beta]`,
			want:     []string{"alpha", "beta"},
		},
		{
			name:     "unclosed flow is empty",
			keywords: `["alpha"`,
			want:     []string{},
		},
		{
			name:     "trailing content is empty",
			keywords: `[alpha] trailing`,
			want:     []string{},
		},
		{
			name:     "block requires whitespace after dash",
			keywords: "\n  -alpha",
			want:     []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := "---\nname: fixture\nkeywords:" + tt.keywords + "\n---\n"
			got, err := ReadAll(fstest.MapFS{"scope.md": {Data: []byte(body)}})
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if gotKeywords := got[0].Keywords; !reflect.DeepEqual(gotKeywords, tt.want) {
				t.Errorf("Keywords = %q, want %q", gotKeywords, tt.want)
			}
		})
	}
}

func TestReadAllParsesOptionalMetadata(t *testing.T) {
	t.Parallel()

	scopesFS := fstest.MapFS{
		"a-full.md": {Data: []byte(`---
name: full
plugin: demo
depth:
description:
keywords: []
testStrategy: Minimal
runner: true
skeleton: on
freeform_default: true
review_cap: advisory
---
`)},
		"b-false.md": {Data: []byte(`---
name: false-values
runner: false
skeleton: off
freeform_default: false
review_cap: none
---
`)},
		"c-invalid-tolerated.md": {Data: []byte(`---
name: invalid-tolerated
runner: TRUE
freeform_default: yes
review_cap: adversarial
---
`)},
	}

	got, err := ReadAll(scopesFS)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	want := []Metadata{
		{
			Name:            "full",
			Plugin:          "demo",
			Keywords:        []string{},
			TestStrategy:    "Minimal",
			Runner:          boolPointer(true),
			Skeleton:        true,
			ReviewCap:       ReviewCapAdvisory,
			FreeformDefault: true,
		},
		{
			Name:        "false-values",
			Keywords:    []string{},
			Runner:      boolPointer(false),
			ReviewCap:   ReviewCapNone,
			Skeleton:    false,
			Plugin:      "",
			Depth:       "",
			Description: "",
		},
		{
			Name:      "invalid-tolerated",
			Keywords:  []string{},
			ReviewCap: ReviewCapAdversarial,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadAll() = %#v, want %#v", got, want)
	}
}

func TestReadAllRejectsInvalidOptionalMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		field        string
		wantContains []string
	}{
		{
			name:         "reserved plugin prefix",
			field:        "plugin: aidlc-demo",
			wantContains: []string{"scope.md", "aidlc-demo", "reserved"},
		},
		{
			name:         "invalid skeleton",
			field:        "skeleton: true",
			wantContains: []string{"scope.md", "true", "on", "off"},
		},
		{
			name:         "invalid review cap",
			field:        "review_cap: heavy",
			wantContains: []string{"scope.md", "heavy", "adversarial", "advisory", "none"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := "---\nname: fixture\n" + tt.field + "\n---\n"
			got, err := ReadAll(fstest.MapFS{"scope.md": {Data: []byte(body)}})
			if err == nil {
				t.Fatalf("ReadAll() = %#v, nil error", got)
			}
			if got != nil {
				t.Errorf("ReadAll() result = %#v, want nil on error", got)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("ReadAll() error = %q, want context %q", err, want)
				}
			}
		})
	}
}

func TestReadAllRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	scopesFS := fstest.MapFS{
		"a-first.md":  {Data: []byte("---\nname: shared\n---\n")},
		"z-second.md": {Data: []byte("---\nname: shared\n---\n")},
	}
	got, err := ReadAll(scopesFS)
	if err == nil {
		t.Fatalf("ReadAll() = %#v, nil error", got)
	}
	if got != nil {
		t.Errorf("ReadAll() result = %#v, want nil", got)
	}
	for _, want := range []string{"duplicate", "shared", "a-first.md", "z-second.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ReadAll() error = %q, want context %q", err, want)
		}
	}
}

func TestReadAllReturnsContextualFileErrors(t *testing.T) {
	t.Parallel()

	t.Run("frontmatter must start the file", func(t *testing.T) {
		t.Parallel()

		got, err := ReadAll(fstest.MapFS{
			"scope.md": {Data: []byte(" \n---\nname: scope\n---\n")},
		})
		assertReadAllError(t, got, err, "scope.md", "missing frontmatter")
	})

	t.Run("name is required", func(t *testing.T) {
		t.Parallel()

		got, err := ReadAll(fstest.MapFS{
			"scope.md": {Data: []byte("---\ndepth: Minimal\n---\n")},
		})
		assertReadAllError(t, got, err, "scope.md", "name")
	})

	t.Run("read error keeps cause and discards earlier result", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("injected read failure")
		scopesFS := readFailureFS{
			FS: fstest.MapFS{
				"a-good.md": {Data: []byte("---\nname: good\n---\n")},
				"b-bad.md":  {Data: []byte("---\nname: bad\n---\n")},
			},
			name: "b-bad.md",
			err:  readErr,
		}
		got, err := ReadAll(scopesFS)
		assertReadAllError(t, got, err, "b-bad.md", "read scope file")
		if !errors.Is(err, readErr) {
			t.Errorf("ReadAll() error = %v, want injected cause", err)
		}
	})
}

func TestReadAllHandlesRootReadDirContracts(t *testing.T) {
	t.Parallel()

	t.Run("missing directory is an empty result", func(t *testing.T) {
		t.Parallel()

		got, err := ReadAll(readDirFailureFS{
			entries: []fs.DirEntry{stubDirEntry("partial.md")},
			err:     fs.ErrNotExist,
		})
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("ReadAll() = %#v, want non-nil empty slice", got)
		}
	})

	t.Run("partial entries with other error are discarded", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("injected directory failure")
		got, err := ReadAll(readDirFailureFS{
			entries: []fs.DirEntry{stubDirEntry("partial.md")},
			err:     readErr,
		})
		assertReadAllError(t, got, err, "read scopes directory")
		if !errors.Is(err, readErr) {
			t.Errorf("ReadAll() error = %v, want injected cause", err)
		}
	})

	t.Run("nil filesystem returns error", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("ReadAll(nil) panicked: %v", recovered)
			}
		}()
		got, err := ReadAll(nil)
		assertReadAllError(t, got, err, "nil filesystem")
	})
}

func TestReadAllRereadsAndReturnsCallerOwnedSlices(t *testing.T) {
	t.Parallel()

	scopesFS := fstest.MapFS{
		"scope.md": {Data: []byte("---\nname: first\nkeywords: [one]\n---\n")},
	}
	first, err := ReadAll(scopesFS)
	if err != nil {
		t.Fatalf("first ReadAll() error = %v", err)
	}
	first[0].Name = "caller mutation"
	first[0].Keywords[0] = "caller mutation"

	scopesFS["scope.md"] = &fstest.MapFile{
		Data: []byte("---\nname: second\nkeywords: [two]\n---\n"),
	}
	second, err := ReadAll(scopesFS)
	if err != nil {
		t.Fatalf("second ReadAll() error = %v", err)
	}
	want := []Metadata{{Name: "second", Keywords: []string{"two"}}}
	if !reflect.DeepEqual(second, want) {
		t.Errorf("second ReadAll() = %#v, want fresh filesystem result %#v", second, want)
	}
}

func assertReadAllError(t *testing.T, got []Metadata, err error, contexts ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("ReadAll() = %#v, nil error", got)
	}
	if got != nil {
		t.Errorf("ReadAll() result = %#v, want nil", got)
	}
	for _, context := range contexts {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("ReadAll() error = %q, want context %q", err, context)
		}
	}
}

type readFailureFS struct {
	fs.FS
	name string
	err  error
}

func (f readFailureFS) Open(name string) (fs.File, error) {
	if name == f.name {
		return nil, f.err
	}
	return f.FS.Open(name)
}

func (f readFailureFS) ReadFile(name string) ([]byte, error) {
	if name == f.name {
		return []byte("---\nname: partial\n---\n"), f.err
	}
	return fs.ReadFile(f.FS, name)
}

type readDirFailureFS struct {
	entries []fs.DirEntry
	err     error
}

func (f readDirFailureFS) Open(string) (fs.File, error) { return nil, fs.ErrInvalid }

func (f readDirFailureFS) ReadDir(string) ([]fs.DirEntry, error) {
	return f.entries, f.err
}

type stubDirEntry string

func (e stubDirEntry) Name() string             { return string(e) }
func (stubDirEntry) IsDir() bool                { return false }
func (stubDirEntry) Type() fs.FileMode          { return 0 }
func (stubDirEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func boolPointer(value bool) *bool { return &value }

func metadataNames(metadata []Metadata) []string {
	names := make([]string, len(metadata))
	for index, item := range metadata {
		names[index] = item.Name
	}
	return names
}
