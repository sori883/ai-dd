//go:build integration

package workspace

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIntentReadersFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		dirs           []string
		files          map[string]string
		expectedNames  []string
		expectedActive string
		expectedOK     bool
	}{
		{name: "uninitialized intents", expectedNames: []string{}},
		{
			name:           "one candidate without cursor",
			files:          map[string]string{"260831-reader/aidlc-state.md": "state body"},
			expectedNames:  []string{"260831-reader"},
			expectedActive: "260831-reader",
			expectedOK:     true,
		},
		{
			name: "many candidates without cursor",
			files: map[string]string{
				"260831-reader/aidlc-state.md":   "current record",
				"reader-deadbeef/aidlc-state.md": "legacy record",
			},
			expectedNames: []string{"260831-reader", "reader-deadbeef"},
		},
		{
			name: "cursor wins among current and legacy names",
			files: map[string]string{
				"active-intent":                  "\ufeff reader-deadbeef\r\n",
				"260831-reader/aidlc-state.md":   "current record",
				"reader-deadbeef/aidlc-state.md": "legacy record",
				"nested/record/aidlc-state.md":   "not a direct candidate",
				"intents.json":                   "not valid json",
				"ordinary-file":                  "not a directory",
				"keep.txt":                       "untouched",
			},
			expectedNames:  []string{"260831-reader", "reader-deadbeef"},
			expectedActive: "reader-deadbeef",
			expectedOK:     true,
		},
		{
			name: "blank cursor falls back",
			files: map[string]string{
				"active-intent":       " \ufeff\r\n",
				"only/aidlc-state.md": "state body",
			},
			expectedNames:  []string{"only"},
			expectedActive: "only",
			expectedOK:     true,
		},
		{
			name: "stale cursor falls back",
			files: map[string]string{
				"active-intent":       "missing",
				"only/aidlc-state.md": "state body",
			},
			expectedNames:  []string{"only"},
			expectedActive: "only",
			expectedOK:     true,
		},
		{
			name:           "cursor is a directory",
			dirs:           []string{"active-intent"},
			files:          map[string]string{"only/aidlc-state.md": "state body"},
			expectedNames:  []string{"only"},
			expectedActive: "only",
			expectedOK:     true,
		},
		{
			name:           "directory marker is sufficient",
			dirs:           []string{"record/aidlc-state.md"},
			files:          map[string]string{"active-intent": "record"},
			expectedNames:  []string{"record"},
			expectedActive: "record",
			expectedOK:     true,
		},
		{
			name: "nested cursor does not need list membership",
			files: map[string]string{
				"active-intent":                "nested/record",
				"nested/record/aidlc-state.md": "state body",
			},
			expectedNames:  []string{},
			expectedActive: "nested/record",
			expectedOK:     true,
		},
		{
			name:           "dot cursor selects root marker",
			files:          map[string]string{"active-intent": ".", "aidlc-state.md": "state body"},
			expectedNames:  []string{},
			expectedActive: ".",
			expectedOK:     true,
		},
		{
			name:          "regular file is not a candidate",
			files:         map[string]string{"active-intent": "record", "record": "not a directory"},
			expectedNames: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeSpaceFixture(
				t,
				root,
				tt.dirs,
				tt.files,
			)
			assertReadOnlyIntentReaders(
				t,
				root,
				tt.expectedNames,
				tt.expectedActive,
				tt.expectedOK,
			)
		})
	}
}

func TestIntentReadersSymlinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		isCursor       bool
		targetKind     string
		expectedNames  []string
		expectedActive string
	}{
		{
			name: "cursor inner relative link", isCursor: true, targetKind: "inner",
			expectedNames: []string{"fallback"}, expectedActive: "nested/chosen",
		},
		{
			name: "cursor outside relative link", isCursor: true, targetKind: "outside",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
		{
			name: "cursor absolute link to inner file", isCursor: true, targetKind: "absolute",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
		{
			name: "cursor broken relative link", isCursor: true, targetKind: "broken",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
		{
			name: "marker inner relative link", targetKind: "inner",
			expectedNames: []string{"candidate", "fallback"}, expectedActive: "candidate",
		},
		{
			name: "marker outside relative link", targetKind: "outside",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
		{
			name: "marker absolute link to inner file", targetKind: "absolute",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
		{
			name: "marker broken relative link", targetKind: "broken",
			expectedNames: []string{"fallback"}, expectedActive: "fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			files := map[string]string{"fallback/aidlc-state.md": "fallback marker"}
			linkName, targetName, targetContent := "candidate/aidlc-state.md", "state", "marker body"
			if tt.isCursor {
				files["nested/chosen/aidlc-state.md"] = "cursor-selected marker"
				linkName, targetName, targetContent = "active-intent", "cursor", "nested/chosen"
			} else {
				files["active-intent"] = "candidate"
			}
			writeSpaceFixture(
				t,
				root,
				[]string{"candidate"},
				files,
			)
			targetRoot := root
			if tt.targetKind == "outside" {
				targetRoot = t.TempDir()
			}
			if tt.targetKind != "broken" {
				writeSpaceFixture(
					t,
					targetRoot,
					nil,
					map[string]string{"targets/" + targetName: targetContent},
				)
			}
			link := filepath.Join(root, filepath.FromSlash(linkName))
			target := filepath.Join(targetRoot, "targets", targetName)
			if tt.targetKind != "absolute" {
				var err error
				target, err = filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, target, link)
			beforeTarget := snapshotSpaceTree(t, targetRoot)
			assertReadOnlyIntentReaders(
				t,
				root,
				tt.expectedNames,
				tt.expectedActive,
				true,
			)
			if after := snapshotSpaceTree(t, targetRoot); !maps.Equal(after, beforeTarget) {
				t.Errorf("target filesystem changed: before=%v, after=%v", beforeTarget, after)
			}
		})
	}
}

func assertReadOnlyIntentReaders(
	t *testing.T,
	root string,
	expectedNames []string,
	expectedActive string,
	expectedOK bool,
) {
	t.Helper()

	before := snapshotSpaceTree(t, root)
	intentsRoot, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := intentsRoot.Close(); err != nil {
			t.Error(err)
		}
	})
	intentsFS := intentsRoot.FS()
	if got := ListIntentDirs(intentsFS); got == nil || !slices.Equal(got, expectedNames) {
		t.Errorf("ListIntentDirs() = %#v, want %#v", got, expectedNames)
	}
	got, ok := ActiveIntent(intentsFS, "")
	if got != expectedActive || ok != expectedOK {
		t.Errorf(
			"ActiveIntent() = (%q, %t), want (%q, %t)",
			got,
			ok,
			expectedActive,
			expectedOK,
		)
	}
	if after := snapshotSpaceTree(t, root); !maps.Equal(after, before) {
		t.Errorf("intents filesystem changed: before=%v, after=%v", before, after)
	}
}
