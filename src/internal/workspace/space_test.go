package workspace

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

func TestActiveSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cursor   string
		expected string
	}{
		{
			name:     "cursor selects a named space",
			cursor:   "research",
			expected: "research",
		},
		{
			name:     "surrounding whitespace is trimmed",
			cursor:   " \t\nresearch\r\n ",
			expected: "research",
		},
		{
			name:     "byte order mark is trimmed",
			cursor:   "\ufeffresearch\ufeff",
			expected: "research",
		},
		{
			name:     "next line character is retained",
			cursor:   "\u0085research\u0085",
			expected: "\u0085research\u0085",
		},
		{
			name: "javascript whitespace is trimmed",
			cursor: "\v\f\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005" +
				"\u2006\u2007\u2008\u2009\u200aresearch\u2028\u2029\u202f\u205f\u3000",
			expected: "research",
		},
		{
			name:     "next line character alone is not blank",
			cursor:   " \u0085 ",
			expected: "\u0085",
		},
		{
			name:     "path like cursor is not validated",
			cursor:   " ../outside ",
			expected: "../outside",
		},
		{
			name:     "multiple lines and internal whitespace are retained",
			cursor:   " research\nother space\ufeffname ",
			expected: "research\nother space\ufeffname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectFS := &trackingFS{
				FS: fstest.MapFS{
					"aidlc/active-space": {Data: []byte(tt.cursor)},
				},
				openedPaths: []string{},
			}
			if got := ActiveSpace(projectFS); got != tt.expected {
				t.Errorf("ActiveSpace() = %q, want %q", got, tt.expected)
			}
			for _, name := range projectFS.openedPaths {
				if name != "aidlc/active-space" {
					t.Errorf("ActiveSpace() accessed %q instead of only the cursor", name)
				}
			}
		})
	}
}

func TestActiveSpaceFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		projectFS fs.FS
	}{
		{
			name:      "missing cursor",
			projectFS: fstest.MapFS{},
		},
		{
			name: "empty cursor",
			projectFS: fstest.MapFS{
				"aidlc/active-space": {Data: []byte{}},
			},
		},
		{
			name: "whitespace only cursor",
			projectFS: fstest.MapFS{
				"aidlc/active-space": {Data: []byte(" \t\n\r\u00a0\u2028\u2029\ufeff")},
			},
		},
		{
			name: "cursor is a directory",
			projectFS: fstest.MapFS{
				"aidlc/active-space": {Mode: fs.ModeDir},
			},
		},
		{
			name: "permission error",
			projectFS: readFileErrorFS{
				FS:  fstest.MapFS{},
				err: fs.ErrPermission,
			},
		},
		{
			name: "arbitrary read error",
			projectFS: readFileErrorFS{
				FS:  fstest.MapFS{},
				err: errors.New("injected read failure"),
			},
		},
		{
			name: "partial data with error is discarded",
			projectFS: readFileErrorFS{
				FS:   fstest.MapFS{},
				data: []byte("research"),
				err:  fs.ErrPermission,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ActiveSpace(tt.projectFS); got != "default" {
				t.Errorf("ActiveSpace() = %q, want default", got)
			}
		})
	}
}

func TestListSpacesDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		projectFS fs.FS
	}{
		{
			name:      "missing spaces directory",
			projectFS: fstest.MapFS{},
		},
		{
			name: "empty spaces directory",
			projectFS: fstest.MapFS{
				"aidlc/spaces": {Mode: fs.ModeDir},
			},
		},
		{
			name: "spaces path is a regular file",
			projectFS: fstest.MapFS{
				"aidlc/spaces": {Data: []byte("not a directory")},
			},
		},
		{
			name: "only regular files exist",
			projectFS: fstest.MapFS{
				"aidlc/spaces/note.txt": {Data: []byte("not a space")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := []Space{{Name: "default", Active: true}}
			if got := ListSpaces(tt.projectFS, nil); !slices.Equal(got, expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, expected)
			}
		})
	}
}

func TestListSpacesDirectories(t *testing.T) {
	t.Parallel()

	projectFS := fstest.MapFS{
		"aidlc/active-space":                     {Data: []byte(" research\n")},
		"aidlc/spaces/research/intents/note.txt": {Data: []byte("nested content")},
		"aidlc/spaces/zeta":                      {Mode: fs.ModeDir},
		"aidlc/spaces/note.txt":                  {Data: []byte("not a space")},
		"aidlc/spaces/default":                   {Data: []byte("not a directory")},
		"aidlc/unrelated":                        {Mode: fs.ModeDir},
	}
	expected := []Space{
		{Name: "default"},
		{Name: "research", Active: true},
		{Name: "zeta"},
	}
	if got := ListSpaces(projectFS, nil); !slices.Equal(got, expected) {
		t.Errorf("ListSpaces() = %v, want %v", got, expected)
	}
}

func TestListSpacesSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cursor         string
		hasOverride    bool
		override       string
		expectedActive string
	}{
		{
			name:           "nil override uses the trimmed cursor",
			cursor:         " research\n",
			expectedActive: "research",
		},
		{
			name:   "unknown cursor leaves all spaces inactive",
			cursor: "unknown",
		},
		{
			name:           "override takes precedence",
			cursor:         "research",
			hasOverride:    true,
			override:       "zeta",
			expectedActive: "zeta",
		},
		{
			name:        "empty override is explicit",
			cursor:      "research",
			hasOverride: true,
		},
		{
			name:        "unknown override leaves all spaces inactive",
			cursor:      "research",
			hasOverride: true,
			override:    "unknown",
		},
		{
			name:        "override is not trimmed",
			cursor:      "research",
			hasOverride: true,
			override:    " research ",
		},
		{
			name:        "override is case sensitive",
			cursor:      "research",
			hasOverride: true,
			override:    "Research",
		},
		{
			name:           "default override selects the synthetic space",
			cursor:         "research",
			hasOverride:    true,
			override:       "default",
			expectedActive: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectFS := &trackingFS{
				FS: fstest.MapFS{
					"aidlc/active-space":    {Data: []byte(tt.cursor)},
					"aidlc/spaces/research": {Mode: fs.ModeDir},
					"aidlc/spaces/zeta":     {Mode: fs.ModeDir},
				},
				openedPaths: []string{},
			}
			var override *string
			if tt.hasOverride {
				override = &tt.override
			}
			expected := []Space{
				{Name: "default", Active: tt.expectedActive == "default"},
				{Name: "research", Active: tt.expectedActive == "research"},
				{Name: "zeta", Active: tt.expectedActive == "zeta"},
			}
			if got := ListSpaces(projectFS, override); !slices.Equal(got, expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, expected)
			}
			if tt.hasOverride && slices.Contains(projectFS.openedPaths, "aidlc/active-space") {
				t.Error("ListSpaces() read the cursor despite an explicit override")
			}
		})
	}
}

func TestListSpacesUnique(t *testing.T) {
	t.Parallel()

	projectFS := fstest.MapFS{
		"aidlc/spaces/default":  {Mode: fs.ModeDir},
		"aidlc/spaces/research": {Mode: fs.ModeDir},
	}
	entries, err := fs.ReadDir(projectFS, "aidlc/spaces")
	if err != nil {
		t.Fatal(err)
	}
	duplicates := make([]fs.DirEntry, 0, len(entries)*2)
	for _, entry := range entries {
		duplicates = append(duplicates, entry, entry)
	}
	tests := []struct {
		name      string
		projectFS fs.FS
	}{
		{
			name:      "existing default is not repeated",
			projectFS: projectFS,
		},
		{
			name: "repeated entries are not repeated",
			projectFS: readDirResultFS{
				FS:      projectFS,
				entries: duplicates,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected := []Space{{Name: "default", Active: true}, {Name: "research"}}
			if got := ListSpaces(tt.projectFS, nil); !slices.Equal(got, expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, expected)
			}
		})
	}
}

func TestListSpacesOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dirs     []string
		expected []Space
	}{
		{
			name: "default is not forced to the front",
			dirs: []string{"zeta", "Alpha", ".hidden", "alpha"},
			expected: []Space{
				{Name: ".hidden"},
				{Name: "Alpha"},
				{Name: "alpha"},
				{Name: "default", Active: true},
				{Name: "zeta"},
			},
		},
		{
			name: "non bmp names use utf16 code unit order",
			dirs: []string{"\ue000", "\U0001f600", "\U00010000"},
			expected: []Space{
				{Name: "default", Active: true},
				{Name: "\U00010000"},
				{Name: "\U0001f600"},
				{Name: "\ue000"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectFS := fstest.MapFS{}
			for _, name := range tt.dirs {
				projectFS["aidlc/spaces/"+name] = &fstest.MapFile{Mode: fs.ModeDir}
			}
			if got := ListSpaces(projectFS, nil); !slices.Equal(got, tt.expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestListSpacesReadDirError(t *testing.T) {
	t.Parallel()

	projectFS := fstest.MapFS{
		"aidlc/active-space":    {Data: []byte("research")},
		"aidlc/spaces/research": {Mode: fs.ModeDir},
	}
	entries, err := fs.ReadDir(projectFS, "aidlc/spaces")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		entries []fs.DirEntry
		err     error
	}{
		{
			name:    "permission error",
			entries: []fs.DirEntry{},
			err:     fs.ErrPermission,
		},
		{
			name:    "arbitrary directory read error",
			entries: []fs.DirEntry{},
			err:     errors.New("injected directory read failure"),
		},
		{
			name:    "partial entries with error are discarded",
			entries: entries,
			err:     fs.ErrPermission,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			failedFS := readDirResultFS{FS: projectFS, entries: tt.entries, err: tt.err}
			expected := []Space{{Name: "default"}}
			if got := ListSpaces(failedFS, nil); !slices.Equal(got, expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, expected)
			}
		})
	}
}

func TestListSpacesStopsOnStatError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		failedPath string
		expected   []Space
	}{
		{
			name:       "first entry fails",
			failedPath: "aidlc/spaces/alpha",
			expected:   []Space{{Name: "default", Active: true}},
		},
		{
			name:       "middle entry fails",
			failedPath: "aidlc/spaces/research",
			expected:   []Space{{Name: "alpha"}, {Name: "default", Active: true}},
		},
		{
			name:       "last entry fails",
			failedPath: "aidlc/spaces/zeta",
			expected:   []Space{{Name: "alpha"}, {Name: "default", Active: true}, {Name: "research"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectFS := statErrorFS{
				FS: fstest.MapFS{
					"aidlc/spaces/alpha":    {Mode: fs.ModeDir},
					"aidlc/spaces/research": {Mode: fs.ModeDir},
					"aidlc/spaces/zeta":     {Mode: fs.ModeDir},
				},
				failedPath: tt.failedPath,
			}
			if got := ListSpaces(projectFS, nil); !slices.Equal(got, tt.expected) {
				t.Errorf("ListSpaces() = %v, want %v", got, tt.expected)
			}
		})
	}
}

type trackingFS struct {
	fs.FS
	openedPaths []string
}

func (f *trackingFS) Open(name string) (fs.File, error) {
	f.openedPaths = append(f.openedPaths, name)
	return f.FS.Open(name)
}

type readFileErrorFS struct {
	fs.FS
	data []byte
	err  error
}

func (r readFileErrorFS) ReadFile(name string) ([]byte, error) {
	return r.data, r.err
}

type readDirResultFS struct {
	fs.FS
	entries []fs.DirEntry
	err     error
}

func (r readDirResultFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return r.entries, r.err
}

type statErrorFS struct {
	fs.FS
	failedPath string
}

func (s statErrorFS) Stat(name string) (fs.FileInfo, error) {
	if name == s.failedPath {
		return nil, fs.ErrPermission
	}
	return fs.Stat(s.FS, name)
}
