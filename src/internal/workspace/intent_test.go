package workspace

import (
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

func TestListIntentDirs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		intentsFS fs.FS
		expected  []string
	}{
		{
			name:      "empty intents",
			intentsFS: fstest.MapFS{},
			expected:  []string{},
		},
		{
			name: "one current name",
			intentsFS: fstest.MapFS{
				"260831-reader/aidlc-state.md": {Data: []byte("state body is not parsed")},
			},
			expected: []string{"260831-reader"},
		},
		{
			name: "current legacy and unrestricted names coexist",
			intentsFS: fstest.MapFS{
				"260831-reader/aidlc-state.md":   {Data: []byte{}},
				"reader-deadbeef/aidlc-state.md": {Data: []byte{}},
				".hidden/aidlc-state.md":         {Data: []byte{}},
				"Other Name/aidlc-state.md":      {Data: []byte{}},
			},
			expected: []string{".hidden", "260831-reader", "Other Name", "reader-deadbeef"},
		},
		{
			name: "directory marker is sufficient",
			intentsFS: fstest.MapFS{
				"record/aidlc-state.md": {Mode: fs.ModeDir},
			},
			expected: []string{"record"},
		},
		{
			name: "missing markers and nested records are excluded",
			intentsFS: fstest.MapFS{
				"empty":                       {Mode: fs.ModeDir},
				"outer/nested/aidlc-state.md": {Data: []byte{}},
				"ordinary-file":               {Data: []byte("not a record")},
				"intents.json":                {Data: []byte("not valid json")},
				"active-intent":               {Data: []byte("missing")},
			},
			expected: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ListIntentDirs(tt.intentsFS)
			if got == nil || !slices.Equal(got, tt.expected) {
				t.Errorf("ListIntentDirs() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}

func TestListIntentDirsIgnoresEntryType(t *testing.T) {
	t.Parallel()

	entries, err := fs.ReadDir(fstest.MapFS{"virtual": {Data: []byte("entry reports a file")}}, ".")
	if err != nil {
		t.Fatal(err)
	}
	intentsFS := readDirResultFS{
		FS:      fstest.MapFS{"virtual/aidlc-state.md": {Data: []byte{}}},
		entries: entries,
	}
	expected := []string{"virtual"}
	if got := ListIntentDirs(intentsFS); !slices.Equal(got, expected) {
		t.Errorf("ListIntentDirs() = %q, want %q", got, expected)
	}
}

func TestListIntentDirsReadDirError(t *testing.T) {
	t.Parallel()

	baseFS := fstest.MapFS{"record/aidlc-state.md": {Data: []byte{}}}
	entries, err := fs.ReadDir(baseFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		entries []fs.DirEntry
		err     error
	}{
		{name: "missing directory", err: fs.ErrNotExist},
		{name: "permission error", err: fs.ErrPermission},
		{name: "arbitrary error", err: errors.New("injected directory read failure")},
		{name: "partial entries with error", entries: entries, err: fs.ErrPermission},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := readDirResultFS{FS: baseFS, entries: tt.entries, err: tt.err}
			if got := ListIntentDirs(intentsFS); got == nil || len(got) != 0 {
				t.Errorf("ListIntentDirs() = %#v, want non-nil empty slice", got)
			}
		})
	}
}

func TestListIntentDirsContinuesAfterStatError(t *testing.T) {
	t.Parallel()

	intentsFS := statErrorFS{
		FS: fstest.MapFS{
			"alpha/aidlc-state.md":  {Data: []byte("alpha state")},
			"middle/aidlc-state.md": {Data: []byte("unavailable state")},
			"zeta/aidlc-state.md":   {Data: []byte("zeta state")},
		},
		failedPath: "middle/aidlc-state.md",
	}
	expected := []string{"alpha", "zeta"}
	if got := ListIntentDirs(intentsFS); !slices.Equal(got, expected) {
		t.Errorf("ListIntentDirs() = %q, want %q", got, expected)
	}
}

func TestListIntentDirsUTF16Order(t *testing.T) {
	t.Parallel()

	intentsFS := fstest.MapFS{
		"\ue000/aidlc-state.md":     {Data: []byte{}},
		"\U00010000/aidlc-state.md": {Data: []byte{}},
		"\U0001f600/aidlc-state.md": {Data: []byte{}},
		"alpha/aidlc-state.md":      {Data: []byte{}},
		"Alpha/aidlc-state.md":      {Data: []byte{}},
	}
	expected := []string{"Alpha", "alpha", "\U00010000", "\U0001f600", "\ue000"}
	if got := ListIntentDirs(intentsFS); !slices.Equal(got, expected) {
		t.Errorf("ListIntentDirs() = %q, want %q", got, expected)
	}
}

func TestActiveIntentExplicit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		explicit string
	}{
		{name: "named intent", explicit: "chosen"},
		{name: "whitespace is not trimmed", explicit: " \t\n "},
		{name: "traversal is not validated", explicit: "../outside"},
		{name: "absolute path is not validated", explicit: "/outside"},
		{name: "path components are not cleaned", explicit: "a/../b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := &trackingFS{FS: fstest.MapFS{}, openedPaths: []string{}}
			got, ok := ActiveIntent(intentsFS, tt.explicit)
			if got != tt.explicit || !ok {
				t.Errorf(
					"ActiveIntent() = (%q, %t), want (%q, true)",
					got,
					ok,
					tt.explicit,
				)
			}
			if len(intentsFS.openedPaths) != 0 {
				t.Errorf("ActiveIntent() accessed the filesystem: %q", intentsFS.openedPaths)
			}
		})
	}
}

func TestActiveIntentCursor(t *testing.T) {
	t.Parallel()

	intentsFS := fstest.MapFS{
		"active-intent":        {Data: []byte("beta")},
		"alpha/aidlc-state.md": {Data: []byte{}},
		"beta/aidlc-state.md":  {Data: []byte{}},
	}
	if got, ok := ActiveIntent(intentsFS, ""); got != "beta" || !ok {
		t.Errorf("ActiveIntent() = (%q, %t), want (beta, true)", got, ok)
	}
}

func TestActiveIntentTrim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cursor   string
		expected string
	}{
		{name: "surrounding whitespace", cursor: " \nchosen\t ", expected: "chosen"},
		{name: "byte order mark is trimmed", cursor: " \ufeffchosen\ufeff ", expected: "chosen"},
		{name: "next line is retained", cursor: " \u0085chosen\u0085 ", expected: "\u0085chosen\u0085"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := fstest.MapFS{
				"active-intent":                     {Data: []byte(tt.cursor)},
				"chosen/aidlc-state.md":             {Data: []byte{}},
				"\u0085chosen\u0085/aidlc-state.md": {Data: []byte{}},
			}
			got, ok := ActiveIntent(intentsFS, "")
			if got != tt.expected || !ok {
				t.Errorf(
					"ActiveIntent() = (%q, %t), want (%q, true)",
					got,
					ok,
					tt.expected,
				)
			}
		})
	}
}

func TestActiveIntentFallbackCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		intentsFS fs.FS
		expected  string
		ok        bool
	}{
		{name: "no candidates", intentsFS: fstest.MapFS{}},
		{
			name:      "one candidate",
			intentsFS: fstest.MapFS{"only/aidlc-state.md": {Data: []byte{}}},
			expected:  "only",
			ok:        true,
		},
		{
			name: "many candidates",
			intentsFS: fstest.MapFS{
				"alpha/aidlc-state.md": {Data: []byte{}},
				"beta/aidlc-state.md":  {Data: []byte{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ActiveIntent(tt.intentsFS, "")
			if got != tt.expected || ok != tt.ok {
				t.Errorf(
					"ActiveIntent() = (%q, %t), want (%q, %t)",
					got,
					ok,
					tt.expected,
					tt.ok,
				)
			}
		})
	}
}

func TestActiveIntentRejectsInvalidCursorBeforeStat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cursor string
	}{
		{name: "empty cursor"},
		{name: "blank cursor", cursor: " \ufeff\t\n "},
		{name: "parent directory", cursor: ".."},
		{name: "parent traversal", cursor: "../other"},
		{name: "traversal must not be cleaned", cursor: "a/../fallback"},
		{name: "absolute path", cursor: "/other"},
		{name: "leading dot component", cursor: "./other"},
		{name: "empty component", cursor: "a//b"},
		{name: "trailing separator", cursor: "a/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := &intentTrackingFS{MapFS: fstest.MapFS{
				"active-intent":           {Data: []byte(tt.cursor)},
				"fallback/aidlc-state.md": {Data: []byte{}},
			}}
			if got, ok := ActiveIntent(intentsFS, ""); got != "fallback" || !ok {
				t.Errorf("ActiveIntent() = (%q, %t), want (fallback, true)", got, ok)
			}
			expected := []string{"active-intent/aidlc-state.md", "fallback/aidlc-state.md"}
			if !slices.Equal(intentsFS.statPaths, expected) {
				t.Errorf("Stat paths = %q, want only fallback enumeration %q", intentsFS.statPaths, expected)
			}
		})
	}
}

func TestActiveIntentCursorPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cursor     string
		markerPath string
	}{
		{name: "dot selects root marker", cursor: ".", markerPath: "aidlc-state.md"},
		{
			name:       "nested path need not be a listed candidate",
			cursor:     "nested/name",
			markerPath: "nested/name/aidlc-state.md",
		},
		{
			name:       "backslash is a name character",
			cursor:     "a\\..\\b",
			markerPath: "a\\..\\b/aidlc-state.md",
		},
		{name: "colon is a name character", cursor: "C:record", markerPath: "C:record/aidlc-state.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := fstest.MapFS{
				"active-intent":        {Data: []byte(tt.cursor)},
				"alpha/aidlc-state.md": {Data: []byte{}},
				"beta/aidlc-state.md":  {Data: []byte{}},
				tt.markerPath:          {Data: []byte{}},
			}
			got, ok := ActiveIntent(intentsFS, "")
			if got != tt.cursor || !ok {
				t.Errorf(
					"ActiveIntent() = (%q, %t), want (%q, true)",
					got,
					ok,
					tt.cursor,
				)
			}
		})
	}
}

func TestActiveIntentFallbackReason(t *testing.T) {
	t.Parallel()

	baseFS := fstest.MapFS{
		"fallback/aidlc-state.md":      {Data: []byte{}},
		"nested/chosen/aidlc-state.md": {Data: []byte{}},
	}
	tests := []struct {
		name      string
		intentsFS fs.FS
	}{
		{
			name: "stale cursor",
			intentsFS: fstest.MapFS{
				"active-intent":           {Data: []byte("missing")},
				"fallback/aidlc-state.md": {Data: []byte{}},
			},
		},
		{
			name: "cursor is a directory",
			intentsFS: fstest.MapFS{
				"active-intent":           {Mode: fs.ModeDir},
				"fallback/aidlc-state.md": {Data: []byte{}},
			},
		},
		{
			name: "cursor read permission error",
			intentsFS: readFileErrorFS{
				FS: baseFS, err: fs.ErrPermission,
			},
		},
		{
			name: "arbitrary cursor read error",
			intentsFS: readFileErrorFS{
				FS: baseFS, err: errors.New("injected cursor read failure"),
			},
		},
		{
			name: "partial cursor data with error is discarded",
			intentsFS: readFileErrorFS{
				FS: baseFS, data: []byte("nested/chosen"), err: fs.ErrPermission,
			},
		},
		{
			name: "cursor marker stat error",
			intentsFS: statErrorFS{
				FS: fstest.MapFS{
					"active-intent":           {Data: []byte("chosen")},
					"chosen/aidlc-state.md":   {Data: []byte{}},
					"fallback/aidlc-state.md": {Data: []byte{}},
				},
				failedPath: "chosen/aidlc-state.md",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got, ok := ActiveIntent(tt.intentsFS, ""); got != "fallback" || !ok {
				t.Errorf("ActiveIntent() = (%q, %t), want (fallback, true)", got, ok)
			}
		})
	}
}

func TestActiveIntentCursorWithoutEnumeration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode fs.FileMode
	}{
		{name: "regular marker"},
		{name: "directory marker", mode: fs.ModeDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			intentsFS := readDirResultFS{
				FS: fstest.MapFS{
					"active-intent":         {Data: []byte("chosen")},
					"chosen/aidlc-state.md": {Mode: tt.mode},
				},
				err: fs.ErrPermission,
			}
			if got, ok := ActiveIntent(intentsFS, ""); got != "chosen" || !ok {
				t.Errorf("ActiveIntent() = (%q, %t), want (chosen, true)", got, ok)
			}
		})
	}
}

func TestIntentReadersReadOnlyCursor(t *testing.T) {
	t.Parallel()

	intentsFS := &intentTrackingFS{MapFS: fstest.MapFS{
		"active-intent":        {Data: []byte("beta")},
		"intents.json":         {Data: []byte("not valid json")},
		"alpha/aidlc-state.md": {Data: []byte("state body is not parsed")},
		"beta/aidlc-state.md":  {Data: []byte("state body is not parsed")},
	}}
	expected := []string{"alpha", "beta"}
	if got := ListIntentDirs(intentsFS); !slices.Equal(got, expected) {
		t.Errorf("ListIntentDirs() = %q, want %q", got, expected)
	}
	if len(intentsFS.readPaths) != 0 {
		t.Errorf("ListIntentDirs() read files: %q", intentsFS.readPaths)
	}
	if got, ok := ActiveIntent(intentsFS, ""); got != "beta" || !ok {
		t.Errorf("ActiveIntent() = (%q, %t), want (beta, true)", got, ok)
	}
	if !slices.Equal(intentsFS.readPaths, []string{"active-intent"}) {
		t.Errorf("ActiveIntent() read files %q, want only active-intent", intentsFS.readPaths)
	}
}

type intentTrackingFS struct {
	fstest.MapFS
	statPaths []string
	readPaths []string
}

func (f *intentTrackingFS) Stat(name string) (fs.FileInfo, error) {
	f.statPaths = append(f.statPaths, name)
	return f.MapFS.Stat(name)
}

func (f *intentTrackingFS) ReadFile(name string) ([]byte, error) {
	f.readPaths = append(f.readPaths, name)
	return f.MapFS.ReadFile(name)
}
