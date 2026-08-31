//go:build integration

package workspace

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReadSelectionProjectClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		closeErr error
	}{
		{name: "successful close"},
		{name: "close failure is returned", closeErr: errors.New("injected project close failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured *os.Root
			closeCalls := 0
			got, err := readSelection(
				RootInput{ExplicitDir: t.TempDir()},
				func(name string) (*os.Root, error) {
					root, err := os.OpenRoot(name)
					if err != nil {
						return nil, err
					}
					captured = root
					t.Cleanup(func() {
						if closeCalls == 0 {
							if err := root.Close(); err != nil {
								t.Error(err)
							}
						}
					})
					return root, nil
				},
				func(*os.Root, string) (*os.Root, error) { return nil, fs.ErrNotExist },
				func(root *os.Root) error {
					closeCalls++
					return errors.Join(root.Close(), tt.closeErr)
				},
			)
			if !errors.Is(err, tt.closeErr) {
				t.Errorf("readSelection() error = %v, want cause %v", err, tt.closeErr)
			}
			if tt.closeErr != nil {
				assertSelection(t, got, Selection{})
			}
			if captured == nil {
				t.Fatal("project root was not opened")
			}
			if closeCalls != 1 {
				t.Errorf("close calls = %d, want 1", closeCalls)
			}
			if _, err := captured.Stat("."); err == nil {
				t.Error("project root remains open after readSelection returned")
			}
		})
	}
}

func TestReadSelectionChildOpenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cause   error
		isEmpty bool
	}{
		{name: "missing intents", cause: fs.ErrNotExist, isEmpty: true},
		{
			name:    "wrapped missing intents",
			cause:   &fs.PathError{Op: "open", Path: "intents", Err: fs.ErrNotExist},
			isEmpty: true,
		},
		{name: "permission denied", cause: fs.ErrPermission},
		{name: "other failure", cause: errors.New("injected child open failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			writeSpaceFixture(
				t,
				projectPath,
				nil,
				map[string]string{"aidlc/active-space": "\ufeff Team_研究\n"},
			)
			openedChildren := []string{}
			closeCalls := 0
			got, err := readSelection(
				RootInput{ExplicitDir: projectPath},
				os.OpenRoot,
				func(_ *os.Root, name string) (*os.Root, error) {
					openedChildren = append(openedChildren, name)
					return nil, tt.cause
				},
				func(root *os.Root) error {
					closeCalls++
					return root.Close()
				},
			)
			expected := Selection{}
			if tt.isEmpty {
				if err != nil {
					t.Errorf("readSelection() error = %v, want nil", err)
				}
				expected = Selection{ProjectRoot: projectPath, SpaceName: "Team_研究", IntentDirs: []string{}}
			} else if !errors.Is(err, tt.cause) {
				t.Errorf("readSelection() error = %v, want cause %v", err, tt.cause)
			}
			expectedChild := filepath.Join(
				"aidlc",
				"spaces",
				"Team_研究",
				"intents",
			)
			if !slices.Equal(openedChildren, []string{expectedChild}) {
				t.Errorf("opened child paths = %q, want only %q", openedChildren, expectedChild)
			}
			if closeCalls != 1 {
				t.Errorf("close calls = %d, want 1 acquired project root", closeCalls)
			}
			assertSelection(t, got, expected)
		})
	}
}

func TestReadSelectionMetadataAndClose(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	writeSpaceFixture(
		t,
		projectPath,
		nil,
		map[string]string{
			"aidlc/active-space":                                 "research",
			"aidlc/spaces/research/intents/active-intent":        "beta",
			"aidlc/spaces/research/intents/alpha/aidlc-state.md": "alpha state",
			"aidlc/spaces/research/intents/beta/aidlc-state.md":  "beta state",
			"aidlc/spaces/default/intents/wrong/aidlc-state.md":  "other space",
		},
	)
	acquired := []*os.Root{}
	closed := []*os.Root{}
	registerRoot := func(root *os.Root) {
		acquired = append(acquired, root)
		t.Cleanup(func() {
			if !slices.Contains(closed, root) {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			}
		})
	}
	got, err := readSelection(
		RootInput{ExplicitDir: projectPath},
		func(name string) (*os.Root, error) {
			root, err := os.OpenRoot(name)
			if err == nil {
				registerRoot(root)
			}
			return root, err
		},
		func(project *os.Root, name string) (*os.Root, error) {
			root, err := project.OpenRoot(name)
			if err == nil {
				registerRoot(root)
			}
			return root, err
		},
		func(root *os.Root) error {
			closed = append(closed, root)
			return root.Close()
		},
	)
	if err != nil {
		t.Errorf("readSelection() error = %v, want nil", err)
	}
	assertSelection(t, got, Selection{
		ProjectRoot:     projectPath,
		SpaceName:       "research",
		IntentDirs:      []string{"alpha", "beta"},
		ActiveIntent:    "beta",
		HasActiveIntent: true,
	})
	if len(acquired) != 2 {
		t.Fatalf("acquired roots = %d, want project and intents roots", len(acquired))
	}
	if !slices.Equal(closed, []*os.Root{acquired[1], acquired[0]}) {
		t.Errorf("closed roots = %v, want reverse acquisition order %v", closed, acquired)
	}
	for _, root := range acquired {
		if _, err := root.Stat("."); err == nil {
			t.Error("acquired root remains open after readSelection returned")
		}
	}
}

func TestReadSelectionJoinsCloseErrors(t *testing.T) {
	t.Parallel()

	projectCloseErr := errors.New("injected project close failure")
	intentsCloseErr := errors.New("injected intents close failure")
	tests := []struct {
		name            string
		childOpenErr    error
		projectCloseErr error
		intentsCloseErr error
		expectedCauses  []error
	}{
		{
			name:            "child open and project close fail",
			childOpenErr:    fs.ErrPermission,
			projectCloseErr: projectCloseErr,
			expectedCauses:  []error{fs.ErrPermission, projectCloseErr},
		},
		{
			name:            "both roots fail to close",
			projectCloseErr: projectCloseErr,
			intentsCloseErr: intentsCloseErr,
			expectedCauses:  []error{projectCloseErr, intentsCloseErr},
		},
		{
			name:            "intents close failure alone",
			intentsCloseErr: intentsCloseErr,
			expectedCauses:  []error{intentsCloseErr},
		},
		{
			name:            "missing child still reports project close failure",
			childOpenErr:    fs.ErrNotExist,
			projectCloseErr: projectCloseErr,
			expectedCauses:  []error{projectCloseErr},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			writeSpaceFixture(
				t,
				projectPath,
				nil,
				map[string]string{"aidlc/spaces/default/intents/only/aidlc-state.md": "state body"},
			)
			acquired := []*os.Root{}
			closed := []*os.Root{}
			registerRoot := func(root *os.Root) {
				acquired = append(acquired, root)
				t.Cleanup(func() {
					if !slices.Contains(closed, root) {
						if err := root.Close(); err != nil {
							t.Error(err)
						}
					}
				})
			}
			got, err := readSelection(
				RootInput{ExplicitDir: projectPath},
				func(name string) (*os.Root, error) {
					root, err := os.OpenRoot(name)
					if err == nil {
						registerRoot(root)
					}
					return root, err
				},
				func(project *os.Root, name string) (*os.Root, error) {
					if tt.childOpenErr != nil {
						return nil, tt.childOpenErr
					}
					root, err := project.OpenRoot(name)
					if err == nil {
						registerRoot(root)
					}
					return root, err
				},
				func(root *os.Root) error {
					closed = append(closed, root)
					closeErr := tt.intentsCloseErr
					if root == acquired[0] {
						closeErr = tt.projectCloseErr
					}
					return errors.Join(root.Close(), closeErr)
				},
			)
			for _, cause := range tt.expectedCauses {
				if !errors.Is(err, cause) {
					t.Errorf("readSelection() error = %v, lost cause %v", err, cause)
				}
			}
			if errors.Is(err, fs.ErrNotExist) {
				t.Errorf("readSelection() error = %v, child absence should be absorbed", err)
			}
			assertSelection(t, got, Selection{})
			expectedClosed := slices.Clone(acquired)
			slices.Reverse(expectedClosed)
			if !slices.Equal(closed, expectedClosed) {
				t.Errorf("closed roots = %v, want %v", closed, expectedClosed)
			}
			for _, root := range acquired {
				if _, err := root.Stat("."); err == nil {
					t.Error("acquired root remains open after readSelection returned")
				}
			}
		})
	}
}

func TestReadSelectionRejectsInvalidSpace(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("injected project close failure")
	tests := []struct {
		name     string
		cursor   string
		closeErr error
	}{
		{name: "dot", cursor: "."},
		{name: "parent directory", cursor: ".."},
		{name: "parent traversal", cursor: "../other"},
		{name: "traversal must not be cleaned", cursor: "a/../b"},
		{name: "nested name", cursor: "nested/name"},
		{name: "absolute name", cursor: "/other"},
		{name: "empty component", cursor: "a//b"},
		{name: "trailing separator", cursor: "a/"},
		{name: "leading dot component", cursor: "./other"},
		{name: "localize rejects null byte", cursor: "a\x00b"},
		{name: "invalid space and close failure", cursor: "..", closeErr: closeErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			writeSpaceFixture(
				t,
				projectPath,
				nil,
				map[string]string{"aidlc/active-space": tt.cursor},
			)
			projectRoot, err := os.OpenRoot(projectPath)
			if err != nil {
				t.Fatal(err)
			}
			closeCalls := 0
			t.Cleanup(func() {
				if closeCalls == 0 {
					if err := projectRoot.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			childCalls := 0
			got, err := readSelection(
				RootInput{ExplicitDir: projectPath},
				func(string) (*os.Root, error) { return projectRoot, nil },
				func(*os.Root, string) (*os.Root, error) {
					childCalls++
					return nil, fs.ErrNotExist
				},
				func(root *os.Root) error {
					closeCalls++
					return errors.Join(root.Close(), tt.closeErr)
				},
			)
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("readSelection() error = %v, want fs.ErrInvalid", err)
			}
			if tt.closeErr != nil && !errors.Is(err, tt.closeErr) {
				t.Errorf("readSelection() error = %v, lost close cause %v", err, tt.closeErr)
			}
			if childCalls != 0 {
				t.Errorf("child open calls = %d, want 0 for invalid space", childCalls)
			}
			if closeCalls != 1 {
				t.Errorf("close calls = %d, want 1 project root", closeCalls)
			}
			if _, err := projectRoot.Stat("."); err == nil {
				t.Error("project root remains open after invalid space")
			}
			assertSelection(t, got, Selection{})
		})
	}
}

func TestReadSelectionFilesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dirs     []string
		files    map[string]string
		expected Selection
	}{
		{
			name:     "missing aidlc",
			expected: Selection{SpaceName: "default", IntentDirs: []string{}},
		},
		{
			name:     "missing spaces",
			dirs:     []string{"aidlc"},
			expected: Selection{SpaceName: "default", IntentDirs: []string{}},
		},
		{
			name:     "missing default space",
			dirs:     []string{"aidlc/spaces"},
			expected: Selection{SpaceName: "default", IntentDirs: []string{}},
		},
		{
			name:     "missing intents",
			dirs:     []string{"aidlc/spaces/default"},
			expected: Selection{SpaceName: "default", IntentDirs: []string{}},
		},
		{
			name:     "unknown safe space is retained",
			files:    map[string]string{"aidlc/active-space": "unknown"},
			expected: Selection{SpaceName: "unknown", IntentDirs: []string{}},
		},
		{
			name:     "empty intents",
			dirs:     []string{"aidlc/spaces/default/intents"},
			expected: Selection{SpaceName: "default", IntentDirs: []string{}},
		},
		{
			name:  "one default intent is selected",
			files: map[string]string{"aidlc/spaces/default/intents/only/aidlc-state.md": "not parsed"},
			expected: Selection{
				SpaceName: "default", IntentDirs: []string{"only"}, ActiveIntent: "only", HasActiveIntent: true,
			},
		},
		{
			name: "multiple intents without a cursor are not selected",
			files: map[string]string{
				"aidlc/spaces/default/intents/alpha/aidlc-state.md": "alpha state",
				"aidlc/spaces/default/intents/beta/aidlc-state.md":  "beta state",
			},
			expected: Selection{SpaceName: "default", IntentDirs: []string{"alpha", "beta"}},
		},
		{
			name: "custom unicode name preserves next line",
			files: map[string]string{
				"aidlc/active-space": "\ufeff\u0085Team_研究\u0085\ufeff\n",
				"aidlc/spaces/\u0085Team_研究\u0085/intents/only/aidlc-state.md": "state body",
			},
			expected: Selection{
				SpaceName: "\u0085Team_研究\u0085", IntentDirs: []string{"only"},
				ActiveIntent: "only", HasActiveIntent: true,
			},
		},
		{
			name: "stale intent cursor falls back",
			files: map[string]string{
				"aidlc/spaces/default/intents/active-intent":       "missing",
				"aidlc/spaces/default/intents/only/aidlc-state.md": "state body",
			},
			expected: Selection{
				SpaceName: "default", IntentDirs: []string{"only"}, ActiveIntent: "only", HasActiveIntent: true,
			},
		},
		{
			name: "unreadable space cursor falls back",
			dirs: []string{"aidlc/active-space"},
			files: map[string]string{
				"aidlc/spaces/default/intents/only/aidlc-state.md": "state body",
			},
			expected: Selection{
				SpaceName: "default", IntentDirs: []string{"only"}, ActiveIntent: "only", HasActiveIntent: true,
			},
		},
		{
			name: "unreadable intent cursor falls back",
			dirs: []string{"aidlc/spaces/default/intents/active-intent"},
			files: map[string]string{
				"aidlc/spaces/default/intents/only/aidlc-state.md": "state body",
			},
			expected: Selection{
				SpaceName: "default", IntentDirs: []string{"only"}, ActiveIntent: "only", HasActiveIntent: true,
			},
		},
		{
			name: "directory marker is sufficient",
			dirs: []string{"aidlc/spaces/default/intents/only/aidlc-state.md"},
			expected: Selection{
				SpaceName: "default", IntentDirs: []string{"only"}, ActiveIntent: "only", HasActiveIntent: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			writeSpaceFixture(
				t,
				projectPath,
				tt.dirs,
				tt.files,
			)
			got, err := readSelectionWithoutChanges(t, RootInput{ExplicitDir: projectPath}, projectPath)
			if err != nil {
				t.Errorf("ReadSelection() error = %v, want nil", err)
			}
			expected := tt.expected
			expected.ProjectRoot = projectPath
			assertSelection(t, got, expected)
		})
	}
}

func TestReadSelectionFilesystemErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		cause error
	}{
		{name: "missing project does not use lower priority root", cause: fs.ErrNotExist},
		{name: "project is a file", files: map[string]string{"project": "not a directory"}},
		{
			name: "intents is a file",
			files: map[string]string{
				"project/aidlc/spaces/default/intents": "not a directory",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			writeSpaceFixture(
				t,
				fixture,
				[]string{"lower"},
				tt.files,
			)
			got, err := readSelectionWithoutChanges(t, RootInput{
				ExplicitDir:     filepath.Join(fixture, "project"),
				AIDLCProjectDir: filepath.Join(fixture, "lower"),
				WorkingDir:      fixture,
			}, fixture)
			if err == nil {
				t.Error("ReadSelection() error = nil, want open failure")
			}
			if tt.cause != nil && !errors.Is(err, tt.cause) {
				t.Errorf("ReadSelection() error = %v, want cause %v", err, tt.cause)
			}
			assertSelection(t, got, Selection{})
		})
	}
}

func TestReadSelectionProjectSymlink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		isAbsolute bool
		isBroken   bool
	}{
		{name: "relative project link"},
		{name: "absolute project link", isAbsolute: true},
		{name: "broken project link is an error", isBroken: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			writeSpaceFixture(
				t,
				fixture,
				nil,
				map[string]string{"project/aidlc/spaces/default/intents/only/aidlc-state.md": "state body"},
			)
			target := "project"
			if tt.isBroken {
				target = "missing"
			}
			if tt.isAbsolute {
				target = filepath.Join(fixture, target)
			}
			projectLink := filepath.Join(fixture, "selected-project")
			createSpaceSymlink(t, target, projectLink)
			got, err := readSelectionWithoutChanges(t, RootInput{
				ExplicitDir:     projectLink,
				AIDLCProjectDir: filepath.Join(fixture, "project"),
			}, fixture)
			if tt.isBroken {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("ReadSelection() error = %v, want fs.ErrNotExist", err)
				}
				assertSelection(t, got, Selection{})
				return
			}
			if err != nil {
				t.Errorf("ReadSelection() error = %v, want nil", err)
			}
			assertSelection(t, got, Selection{
				ProjectRoot: projectLink, SpaceName: "default", IntentDirs: []string{"only"},
				ActiveIntent: "only", HasActiveIntent: true,
			})
		})
	}
}

func TestReadSelectionChildSymlink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		isOutside  bool
		isAbsolute bool
		isBroken   bool
	}{
		{name: "relative link elsewhere inside project"},
		{name: "relative link outside project", isOutside: true},
		{name: "absolute link inside project", isAbsolute: true},
		{name: "broken relative link is empty", isBroken: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			writeSpaceFixture(
				t,
				fixture,
				[]string{"project/aidlc/spaces/default"},
				map[string]string{
					"project/shared/intents/only/aidlc-state.md": "inside project",
					"outside/intents/other/aidlc-state.md":       "outside project",
				},
			)
			projectPath := filepath.Join(fixture, "project")
			link := filepath.Join(projectPath, filepath.FromSlash("aidlc/spaces/default/intents"))
			target := filepath.Join(projectPath, "shared", "intents")
			if tt.isOutside {
				target = filepath.Join(fixture, "outside", "intents")
			}
			if tt.isBroken {
				target = filepath.Join(projectPath, "shared", "missing")
			}
			if !tt.isAbsolute {
				var err error
				target, err = filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, target, link)
			got, err := readSelectionWithoutChanges(t, RootInput{ExplicitDir: projectPath}, fixture)
			if tt.isOutside || tt.isAbsolute {
				if err == nil || errors.Is(err, fs.ErrNotExist) {
					t.Errorf("ReadSelection() error = %v, want non-absence boundary error", err)
				}
				assertSelection(t, got, Selection{})
				return
			}
			if err != nil {
				t.Errorf("ReadSelection() error = %v, want nil", err)
			}
			expected := Selection{ProjectRoot: projectPath, SpaceName: "default", IntentDirs: []string{}}
			if !tt.isBroken {
				expected.IntentDirs = []string{"only"}
				expected.ActiveIntent = "only"
				expected.HasActiveIntent = true
			}
			assertSelection(t, got, expected)
		})
	}
}

func TestReadSelectionSpaceCursorSymlink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		isOutside     bool
		isAbsolute    bool
		isBroken      bool
		expectedSpace string
	}{
		{name: "relative cursor inside project", expectedSpace: "research"},
		{name: "relative cursor outside project", isOutside: true, expectedSpace: "default"},
		{name: "absolute cursor inside project", isAbsolute: true, expectedSpace: "default"},
		{name: "broken relative cursor", isBroken: true, expectedSpace: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			writeSpaceFixture(
				t,
				fixture,
				nil,
				map[string]string{
					"project/aidlc/spaces/default/intents/default-only/aidlc-state.md":   "default state",
					"project/aidlc/spaces/research/intents/research-only/aidlc-state.md": "research state",
					"project/target-cursor": "research",
					"outside-cursor":        "research",
				},
			)
			projectPath := filepath.Join(fixture, "project")
			link := filepath.Join(projectPath, "aidlc", "active-space")
			target := filepath.Join(projectPath, "target-cursor")
			if tt.isOutside {
				target = filepath.Join(fixture, "outside-cursor")
			}
			if tt.isBroken {
				target = filepath.Join(projectPath, "missing-cursor")
			}
			if !tt.isAbsolute {
				var err error
				target, err = filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, target, link)
			got, err := readSelectionWithoutChanges(t, RootInput{ExplicitDir: projectPath}, fixture)
			if err != nil {
				t.Errorf("ReadSelection() error = %v, want cursor error absorbed", err)
			}
			expectedIntent := tt.expectedSpace + "-only"
			assertSelection(t, got, Selection{
				ProjectRoot: projectPath, SpaceName: tt.expectedSpace, IntentDirs: []string{expectedIntent},
				ActiveIntent: expectedIntent, HasActiveIntent: true,
			})
		})
	}
}

func TestReadSelectionIntentsBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		isCursor  bool
		isOutside bool
		expected  Selection
	}{
		{
			name: "cursor link inside intents", isCursor: true,
			expected: Selection{
				IntentDirs: []string{"fallback"}, ActiveIntent: "nested/chosen", HasActiveIntent: true,
			},
		},
		{
			name: "cursor inside project but outside intents", isCursor: true, isOutside: true,
			expected: Selection{
				IntentDirs: []string{"fallback"}, ActiveIntent: "fallback", HasActiveIntent: true,
			},
		},
		{
			name: "marker link inside intents",
			expected: Selection{
				IntentDirs: []string{"candidate", "fallback"}, ActiveIntent: "candidate", HasActiveIntent: true,
			},
		},
		{
			name: "marker inside project but outside intents", isOutside: true,
			expected: Selection{
				IntentDirs: []string{"fallback"}, ActiveIntent: "fallback", HasActiveIntent: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			intentsRelative := "project/aidlc/spaces/default/intents"
			dirs := []string{intentsRelative + "/candidate"}
			files := map[string]string{intentsRelative + "/fallback/aidlc-state.md": "fallback state"}
			linkName, targetName, targetContent := "candidate/aidlc-state.md", "state", "marker body"
			if tt.isCursor {
				linkName, targetName, targetContent = "active-intent", "cursor", "nested/chosen"
				files[intentsRelative+"/nested/chosen/aidlc-state.md"] = "nested state"
			} else {
				files[intentsRelative+"/active-intent"] = "candidate"
			}
			files[intentsRelative+"/targets/"+targetName] = targetContent
			files["project/shared/"+targetName] = targetContent
			writeSpaceFixture(
				t,
				fixture,
				dirs,
				files,
			)
			projectPath := filepath.Join(fixture, "project")
			intentsPath := filepath.Join(fixture, filepath.FromSlash(intentsRelative))
			link := filepath.Join(intentsPath, filepath.FromSlash(linkName))
			targetPath := filepath.Join(intentsPath, "targets", targetName)
			if tt.isOutside {
				targetPath = filepath.Join(projectPath, "shared", targetName)
			}
			target, err := filepath.Rel(filepath.Dir(link), targetPath)
			if err != nil {
				t.Fatal(err)
			}
			createSpaceSymlink(t, target, link)
			got, err := readSelectionWithoutChanges(t, RootInput{ExplicitDir: projectPath}, fixture)
			if err != nil {
				t.Errorf("ReadSelection() error = %v, want reader errors absorbed", err)
			}
			expected := tt.expected
			expected.ProjectRoot = projectPath
			expected.SpaceName = "default"
			assertSelection(t, got, expected)
		})
	}
}

func readSelectionWithoutChanges(t *testing.T, input RootInput, snapshotRoot string) (Selection, error) {
	t.Helper()

	before := snapshotSpaceTree(t, snapshotRoot)
	selection, err := ReadSelection(input)
	if after := snapshotSpaceTree(t, snapshotRoot); !maps.Equal(after, before) {
		t.Errorf("filesystem changed: before=%v, after=%v", before, after)
	}
	return selection, err
}
