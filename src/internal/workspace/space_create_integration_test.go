//go:build integration

package workspace

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestCreateSpaceProjectClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		closeErr error
	}{
		{name: "successful close"},
		{name: "close failure", closeErr: errors.New("injected project close failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured *os.Root
			closeCalls := 0
			got, err := createSpace(
				RootInput{ExplicitDir: t.TempDir()},
				"Team Alpha",
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
				func(root *os.Root) error {
					closeCalls++
					return errors.Join(root.Close(), tt.closeErr)
				},
				populateSpace,
			)
			if !errors.Is(err, tt.closeErr) {
				t.Errorf("createSpace() error = %v, want cause %v", err, tt.closeErr)
			}
			if tt.closeErr != nil && got != "" {
				t.Errorf("createSpace() name = %q, want empty on error", got)
			}
			if captured == nil {
				t.Fatal("project root was not opened")
			}
			if closeCalls != 1 {
				t.Errorf("close calls = %d, want 1", closeCalls)
			}
			if _, err := captured.Stat("."); err == nil {
				t.Error("project root remains open after return")
			}
			assertSpaceScaffold(
				t,
				captured.Name(),
				"team-alpha",
				"# Organization defaults\n",
			)
		})
	}
}

func TestCreateSpaceClaimsNewTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "normalized name", raw: "Team Alpha", expected: "team-alpha"},
		{name: "new default", raw: "default", expected: "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, tt.raw)
			if err != nil || got != tt.expected {
				t.Errorf(
					"CreateSpace() = (%q, %v), want (%q, nil)",
					got,
					err,
					tt.expected,
				)
			}
			target := filepath.Join(
				projectPath,
				"aidlc",
				"spaces",
				tt.expected,
			)
			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("new space target is unavailable: %v", err)
			}
			if !info.IsDir() {
				t.Error("new space target is not a directory")
			}
		})
	}
}

func TestCreateSpaceExistingTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		dirs       []string
		files      map[string]string
		linkTarget string
	}{
		{name: "directory", dirs: []string{"aidlc/spaces/team"}},
		{
			name:  "populated directory",
			files: map[string]string{"aidlc/spaces/team/memory/org.md": "preserve existing organization"},
		},
		{name: "file", files: map[string]string{"aidlc/spaces/team": "existing file"}},
		{name: "directory link", dirs: []string{"elsewhere"}, linkTarget: "elsewhere"},
		{
			name:       "file link",
			files:      map[string]string{"elsewhere": "existing linked file"},
			linkTarget: "elsewhere",
		},
		{name: "broken link", linkTarget: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			dirs := append([]string{"aidlc/spaces"}, tt.dirs...)
			writeSpaceFixture(
				t,
				projectPath,
				dirs,
				tt.files,
			)
			if tt.linkTarget != "" {
				createSpaceSymlink(
					t,
					filepath.Join(projectPath, tt.linkTarget),
					filepath.Join(
						projectPath,
						"aidlc",
						"spaces",
						"team",
					),
				)
			}
			before := snapshotSpaceTree(t, projectPath)
			got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, "Team")
			if !errors.Is(err, fs.ErrExist) || got != "" {
				t.Errorf("CreateSpace() = (%q, %v), want empty name and fs.ErrExist", got, err)
			}
			if after := snapshotSpaceTree(t, projectPath); !maps.Equal(after, before) {
				t.Errorf("existing tree changed: before=%v, after=%v", before, after)
			}
		})
	}
}

func TestCreateSpaceScaffold(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"team", "default"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, name)
			if err != nil || got != name {
				t.Fatalf(
					"CreateSpace() = (%q, %v), want (%q, nil)",
					got,
					err,
					name,
				)
			}
			assertSpaceScaffold(
				t,
				projectPath,
				name,
				"# Organization defaults\n",
			)
		})
	}
}

func TestCreateSpaceCopiesOnlyDefaultOrganization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "organization text", content: "\ufeff# 会社の規約\r\nCustom defaults.\n"},
		{name: "empty organization file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			writeSpaceFixture(
				t,
				projectPath,
				nil,
				map[string]string{
					"aidlc/active-space":                              "other\n",
					"aidlc/spaces/default/memory/org.md":              tt.content,
					"aidlc/spaces/default/memory/team.md":             "do not copy team",
					"aidlc/spaces/default/memory/project.md":          "do not copy project",
					"aidlc/spaces/default/memory/phases/custom.md":    "do not copy phase",
					"aidlc/spaces/default/memory/templates/custom.md": "do not copy template",
					"aidlc/spaces/default/codekb/custom.md":           "do not copy codekb",
					"aidlc/spaces/default/knowledge/custom.md":        "do not copy knowledge",
					"aidlc/spaces/default/intents/active-intent":      "do not copy intent",
					"aidlc/spaces/other/memory/org.md":                "do not copy active space",
					"keep.txt":                                        "untouched",
				},
			)
			before := snapshotSpaceTree(t, projectPath)
			got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, "research")
			if err != nil || got != "research" {
				t.Fatalf("CreateSpace() = (%q, %v), want (research, nil)", got, err)
			}
			assertSpaceScaffold(
				t,
				projectPath,
				"research",
				tt.content,
			)
			after := snapshotSpaceTree(t, projectPath)
			if len(after) != len(before)+13 {
				t.Errorf("created %d entries, want exactly 13", len(after)-len(before))
			}
			for name, expected := range before {
				actual, ok := after[name]
				if name == filepath.Join(projectPath, "aidlc", "spaces") {
					// Adding the target changes only this existing parent directory's mtime.
					actual.modified = expected.modified
				}
				if !ok || actual != expected {
					t.Errorf(
						"existing entry %q changed: before=%v, after=%v",
						name,
						expected,
						actual,
					)
				}
			}
		})
	}
}

func TestCreateSpacePartialWriteAndCloseFailuresRemain(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	writeFailure := errors.New("injected partial write failure")
	fileCloseFailure := errors.New("injected file close failure")
	rootCloseFailure := errors.New("injected root close failure")
	var captured *os.Root
	got, err := createSpace(
		RootInput{ExplicitDir: projectPath},
		"team",
		func(name string) (*os.Root, error) {
			root, err := os.OpenRoot(name)
			captured = root
			return root, err
		},
		func(root *os.Root) error {
			return errors.Join(root.Close(), rootCloseFailure)
		},
		func(root *os.Root, target string) error {
			return writeSpaceFile(
				filepath.Join(target, "partial.md"),
				"incomplete content",
				func(name string, flags int, mode fs.FileMode) (io.WriteCloser, error) {
					file, err := root.OpenFile(name, flags, mode)
					if err != nil {
						return nil, err
					}
					return spaceTestWriteCloser{
						write: func(p []byte) (int, error) {
							n, err := file.Write(p[:2])
							return n, errors.Join(err, writeFailure)
						},
						close: func() error {
							return errors.Join(file.Close(), fileCloseFailure)
						},
					}, nil
				},
			)
		},
	)
	if got != "" {
		t.Errorf("createSpace() name = %q, want empty on failure", got)
	}
	for _, cause := range []error{writeFailure, fileCloseFailure, rootCloseFailure} {
		if !errors.Is(err, cause) {
			t.Errorf("createSpace() error = %v, want cause %v", err, cause)
		}
	}
	if captured == nil {
		t.Fatal("project root was not acquired")
	}
	if _, err := captured.Stat("."); err == nil {
		t.Error("project root remains open after failure")
	}
	data, err := os.ReadFile(filepath.Join(
		projectPath,
		"aidlc",
		"spaces",
		"team",
		"partial.md",
	))
	if err != nil || string(data) != "in" {
		t.Errorf("partial file = (%q, %v), want (in, nil)", data, err)
	}
	beforeRetry := snapshotSpaceTree(t, projectPath)
	got, err = CreateSpace(RootInput{ExplicitDir: projectPath}, "team")
	if got != "" || !errors.Is(err, fs.ErrExist) {
		t.Errorf("retry = (%q, %v), want empty name and fs.ErrExist", got, err)
	}
	if afterRetry := snapshotSpaceTree(t, projectPath); !maps.Equal(beforeRetry, afterRetry) {
		t.Error("retry altered the partial target")
	}
}

func TestCreateSpaceConcurrentTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		names     [2]string
		wantExist int
	}{
		{name: "same target", names: [2]string{"team", "team"}, wantExist: 1},
		{name: "different targets", names: [2]string{"team", "research"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projectPath := t.TempDir()
			results := make([]struct {
				name string
				err  error
			}, len(tt.names))
			start := make(chan struct{})
			var workers sync.WaitGroup
			for i, name := range tt.names {
				workers.Go(func() {
					<-start
					results[i].name, results[i].err = CreateSpace(RootInput{ExplicitDir: projectPath}, name)
				})
			}
			close(start)
			workers.Wait()
			exists := 0
			for i, result := range results {
				if errors.Is(result.err, fs.ErrExist) && result.name == "" {
					exists++
					continue
				}
				if result.err != nil || result.name != tt.names[i] {
					t.Errorf(
						"create %d = (%q, %v), want success or existing target",
						i,
						result.name,
						result.err,
					)
					continue
				}
				assertSpaceScaffold(
					t,
					projectPath,
					result.name,
					"# Organization defaults\n",
				)
			}
			if exists != tt.wantExist {
				t.Errorf("existing-target failures = %d, want %d", exists, tt.wantExist)
			}
		})
	}
}

func TestCreateSpaceRequiresExistingProject(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"missing", "file"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			projectPath := filepath.Join(base, "project")
			if kind == "file" {
				writeSpaceFixture(
					t,
					base,
					nil,
					map[string]string{"project": "not a directory"},
				)
			}
			before := snapshotSpaceTree(t, base)
			got, err := CreateSpace(RootInput{ExplicitDir: projectPath, WorkingDir: base}, "team")
			if got != "" || err == nil {
				t.Errorf("CreateSpace() = (%q, %v), want project-open error", got, err)
			}
			if kind == "missing" && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("missing project error = %v, want fs.ErrNotExist", err)
			}
			if after := snapshotSpaceTree(t, base); !maps.Equal(after, before) {
				t.Error("failed project open created or changed data")
			}
		})
	}
}

func TestCreateSpaceNativeNameValidation(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	before := snapshotSpaceTree(t, projectPath)
	got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, "CON")
	if runtime.GOOS == "windows" {
		if got != "" || !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("CreateSpace(CON) = (%q, %v), want fs.ErrInvalid on Windows", got, err)
		}
		if after := snapshotSpaceTree(t, projectPath); !maps.Equal(after, before) {
			t.Error("invalid localized name changed the project")
		}
		return
	}
	if got != "con" || err != nil {
		t.Errorf(
			"CreateSpace(CON) = (%q, %v), want (con, nil) on %s",
			got,
			err,
			runtime.GOOS,
		)
	}
}

func TestCreateSpaceRootPriority(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"explicit", "aidlc", "claude", "working", "relative explicit"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			writeSpaceFixture(
				t,
				base,
				[]string{"explicit", "aidlc", "claude", "working"},
				nil,
			)
			input := RootInput{
				ExplicitDir:      filepath.Join(base, "explicit"),
				AIDLCProjectDir:  filepath.Join(base, "aidlc"),
				ClaudeProjectDir: filepath.Join(base, "claude"),
				WorkingDir:       filepath.Join(base, "working"),
			}
			selected := source
			switch source {
			case "aidlc":
				input.ExplicitDir = ""
			case "claude":
				input.ExplicitDir = ""
				input.AIDLCProjectDir = ""
			case "working":
				input.ExplicitDir = ""
				input.AIDLCProjectDir = ""
				input.ClaudeProjectDir = ""
			case "relative explicit":
				input.ExplicitDir = filepath.Join("..", "explicit")
				selected = "explicit"
			}
			got, err := CreateSpace(input, "team")
			if err != nil || got != "team" {
				t.Fatalf("CreateSpace() = (%q, %v), want (team, nil)", got, err)
			}
			for _, candidate := range []string{"explicit", "aidlc", "claude", "working"} {
				path := filepath.Join(
					base,
					candidate,
					"aidlc",
					"spaces",
					"team",
				)
				_, err := os.Stat(path)
				if candidate == selected && err != nil {
					t.Errorf("selected project %q has no target: %v", candidate, err)
				}
				if candidate != selected && !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("unselected project %q changed: stat error %v", candidate, err)
				}
			}
		})
	}
}

func TestCreateSpaceInitialProjectSymlink(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	projectPath := t.TempDir()
	link := filepath.Join(base, "project-link")
	createSpaceSymlink(t, projectPath, link)
	got, err := CreateSpace(RootInput{ExplicitDir: link}, "team")
	if got != "team" || err != nil {
		t.Fatalf("CreateSpace() = (%q, %v), want (team, nil)", got, err)
	}
	assertSpaceScaffold(
		t,
		projectPath,
		"team",
		"# Organization defaults\n",
	)
}

func TestCreateSpaceSymlinkBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		outside  bool
		absolute bool
		broken   bool
	}{
		{name: "relative inside"},
		{name: "relative outside", outside: true},
		{name: "absolute inside", absolute: true},
		{name: "absolute outside", outside: true, absolute: true},
		{name: "broken inside", broken: true},
		{name: "broken outside", outside: true, broken: true},
	}
	for _, boundary := range []string{"creation parent", "organization source"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					projectPath := t.TempDir()
					outsidePath := t.TempDir()
					targetRoot := projectPath
					if tt.outside {
						targetRoot = outsidePath
					}
					linkPath := filepath.Join(projectPath, "aidlc")
					targetPath := filepath.Join(targetRoot, "storage")
					if boundary == "organization source" {
						writeSpaceFixture(
							t,
							projectPath,
							[]string{"aidlc/spaces/default/memory"},
							nil,
						)
						linkPath = filepath.Join(
							projectPath,
							"aidlc",
							"spaces",
							"default",
							"memory",
							"org.md",
						)
						targetPath = filepath.Join(targetRoot, "shared-org.md")
						if !tt.broken {
							writeSpaceFixture(
								t,
								targetRoot,
								nil,
								map[string]string{"shared-org.md": "shared organization\n"},
							)
						}
					} else {
						writeSpaceFixture(
							t,
							targetRoot,
							[]string{"storage"},
							nil,
						)
					}
					linkTarget := targetPath
					if !tt.absolute {
						var err error
						linkTarget, err = filepath.Rel(filepath.Dir(linkPath), targetPath)
						if err != nil {
							t.Fatal(err)
						}
					}
					createSpaceSymlink(t, linkTarget, linkPath)
					if tt.broken && boundary == "creation parent" {
						// Windows needs an existing directory when creating a directory symlink.
						if err := os.Remove(targetPath); err != nil {
							t.Fatal(err)
						}
					}
					outsideBefore := snapshotSpaceTree(t, outsidePath)
					got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, "team")
					// A missing internal parent may be created; a missing internal seed uses the default text.
					wantSuccess := !tt.outside && !tt.absolute
					if wantSuccess {
						if got != "team" || err != nil {
							t.Fatalf("CreateSpace() = (%q, %v), want (team, nil)", got, err)
						}
						org := "# Organization defaults\n"
						if boundary == "organization source" && !tt.broken {
							org = "shared organization\n"
						}
						assertSpaceScaffold(
							t,
							projectPath,
							"team",
							org,
						)
					} else if got != "" || err == nil {
						t.Errorf("CreateSpace() = (%q, %v), want boundary error", got, err)
					}
					if after := snapshotSpaceTree(t, outsidePath); !maps.Equal(after, outsideBefore) {
						t.Error("creation changed data outside the project")
					}
					if actual, err := os.Readlink(linkPath); err != nil || actual != linkTarget {
						t.Errorf("link changed: target=%q, error=%v", actual, err)
					}
				})
			}
		})
	}
}

func TestCreateSpaceOrganizationDirectoryIsError(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	writeSpaceFixture(
		t,
		projectPath,
		[]string{"aidlc/spaces/default/memory/org.md"},
		nil,
	)
	got, err := CreateSpace(RootInput{ExplicitDir: projectPath}, "team")
	if got != "" || err == nil {
		t.Errorf("CreateSpace() = (%q, %v), want organization read error", got, err)
	}
	if _, err := os.Stat(filepath.Join(
		projectPath,
		"aidlc",
		"spaces",
		"team",
		"memory",
	)); err != nil {
		t.Errorf("claimed target should remain after read failure: %v", err)
	}
	orgPath := filepath.Join(projectPath, filepath.FromSlash("aidlc/spaces/team/memory/org.md"))
	if _, err := os.Stat(orgPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("failed source read wrote a fallback file: %v", err)
	}
}

func assertSpaceScaffold(t *testing.T, projectPath, name, orgContent string) {
	t.Helper()

	target := filepath.Join(
		projectPath,
		"aidlc",
		"spaces",
		name,
	)
	actual := snapshotSpaceTree(t, target)
	want := map[string]struct {
		dir     bool
		content string
	}{
		".":                         {dir: true},
		"memory":                    {dir: true},
		"memory/phases":             {dir: true},
		"memory/templates":          {dir: true},
		"intents":                   {dir: true},
		"codekb":                    {dir: true},
		"knowledge":                 {dir: true},
		"memory/org.md":             {content: orgContent},
		"memory/team.md":            {content: "# Team practices\n"},
		"memory/project.md":         {content: "# Project overrides\n"},
		"memory/templates/.gitkeep": {},
		"codekb/.gitkeep":           {},
		"knowledge/.gitkeep":        {},
	}
	if len(actual) != len(want) {
		t.Errorf("new space contains %d entries, want 7 directories and 6 files", len(actual))
	}
	for relative, expected := range want {
		entry, ok := actual[filepath.Join(target, filepath.FromSlash(relative))]
		if !ok {
			t.Errorf("missing scaffold entry %q", relative)
			continue
		}
		if entry.mode.IsDir() != expected.dir || (!expected.dir && !entry.mode.IsRegular()) {
			t.Errorf(
				"entry %q mode = %v, want directory=%v",
				relative,
				entry.mode,
				expected.dir,
			)
		}
		if entry.content != expected.content {
			t.Errorf(
				"entry %q content = %q, want %q",
				relative,
				entry.content,
				expected.content,
			)
		}
	}
}
