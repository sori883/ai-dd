//go:build integration

package workspace

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSwitchSpaceSavesNormalizedName(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc/spaces/team-alpha"},
		map[string]string{},
	)
	var captured *os.Root
	saveCalls := 0
	name, err := switchSpace(
		RootInput{ExplicitDir: project},
		" Team__Alpha! ",
		os.OpenRoot,
		(*os.Root).Close,
		func(root *os.Root, name string) error {
			captured = root
			saveCalls++
			if name != "team-alpha" {
				t.Errorf("saved name = %q, want team-alpha", name)
			}
			if _, err := root.Stat("aidlc/spaces/team-alpha"); err != nil {
				t.Errorf("save did not receive selected project root: %v", err)
			}
			return nil
		},
	)
	if name != "team-alpha" || err != nil {
		t.Errorf("switchSpace() = (%q, %v), want (team-alpha, nil)", name, err)
	}
	if saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", saveCalls)
	}
	if _, err := captured.Stat("."); err == nil {
		t.Error("project root remains open after switch")
	}
}

func TestSwitchSpaceRejectsCursorSymlink(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc"},
		map[string]string{"aidlc/old": "old\n"},
	)
	createSpaceSymlink(t, "old", filepath.Join(project, "aidlc", "active-space"))
	before := snapshotSpaceTree(t, project)
	name, err := SwitchSpace(RootInput{ExplicitDir: project}, "default")
	if name != "" || !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("SwitchSpace() = (%q, %v), want empty name and fs.ErrInvalid", name, err)
	}
	if !maps.Equal(before, snapshotSpaceTree(t, project)) {
		t.Error("rejected cursor link changed the project")
	}
}

func TestSwitchSpaceCreatesOnlySharedCursor(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	name, err := SwitchSpace(RootInput{ExplicitDir: project}, "Default")
	if name != "default" || err != nil {
		t.Fatalf("SwitchSpace() = (%q, %v), want (default, nil)", name, err)
	}
	data, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
	if err != nil || string(data) != "default\n" {
		t.Fatalf("cursor = (%q, %v), want default and one newline", data, err)
	}
	tree := snapshotSpaceTree(t, project)
	if len(tree) != 3 {
		t.Errorf("tree = %v, want only project, aidlc and active-space", tree)
	}
}

func TestSwitchSpaceUnknownDoesNotSave(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{},
		map[string]string{"keep": "unchanged"},
	)
	before := snapshotSpaceTree(t, project)
	name, err := switchSpace(
		RootInput{ExplicitDir: project},
		"unknown",
		os.OpenRoot,
		(*os.Root).Close,
		func(*os.Root, string) error {
			t.Error("unknown space reached cursor save")
			return nil
		},
	)
	if name != "" || err == nil {
		t.Errorf("switchSpace() = (%q, %v), want empty name and unknown-space error", name, err)
	}
	if !maps.Equal(before, snapshotSpaceTree(t, project)) {
		t.Error("unknown space changed the project")
	}
}

func TestSwitchSpaceNamesAndProtectedData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "same target rewrites newline", raw: "Team", want: "team"},
		{name: "normalized help is selectable", raw: "Help", want: "help"},
		{name: "list reserved only for create", raw: "list", want: "list"},
		{name: "create reserved only for create", raw: "create", want: "create"},
		{name: "switch reserved only for create", raw: "switch", want: "switch"},
		{name: "archive", raw: "archive", want: "archive"},
		{name: "rename", raw: "rename", want: "rename"},
		{name: "show", raw: "show", want: "show"},
		{name: "birth", raw: "birth", want: "birth"},
		{name: "whitespace", raw: " \t\n", want: "intent"},
		{name: "unicode lowercase", raw: "AİB", want: "ai-b"},
		{name: "kelvin lowercase", raw: "AKB", want: "akb"},
		{name: "prefix after limit", raw: strings.Repeat("7", 49), want: "intent-" + strings.Repeat("7", 48)},
		{name: "trim after limit", raw: strings.Repeat("a", 47) + "-b", want: strings.Repeat("a", 47)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{"aidlc/spaces/" + tt.want},
				map[string]string{
					"aidlc/active-space":                                      " \t" + tt.want + " ",
					"aidlc/.aidlc-sessions/session.binding.json":              "binding",
					"aidlc/.aidlc-sessions/.current-session":                  "session",
					"aidlc/.aidlc-sessions/session.rebind-offer":              "offer",
					"aidlc/.aidlc-sessions/session":                           "intent id",
					"aidlc/spaces/" + tt.want + "/intents/active-intent":      "current",
					"aidlc/spaces/" + tt.want + "/intents/a/aidlc-state.md":   "state",
					"aidlc/spaces/" + tt.want + "/intents/registry.json":      "registry",
					"aidlc/spaces/" + tt.want + "/knowledge/audit/keep.jsonl": "audit",
					".codex/config.toml":                                      "rules",
				},
			)
			before := snapshotSpaceTree(t, project)
			name, err := SwitchSpace(RootInput{ExplicitDir: project}, tt.raw)
			if name != tt.want || err != nil {
				t.Fatalf(
					"SwitchSpace() = (%q, %v), want (%q, nil)",
					name,
					err,
					tt.want,
				)
			}
			data, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
			if err != nil || string(data) != tt.want+"\n" {
				t.Errorf(
					"cursor = (%q, %v), want %q",
					data,
					err,
					tt.want+"\n",
				)
			}
			after := snapshotSpaceTree(t, project)
			for _, path := range []string{"aidlc", "aidlc/active-space"} {
				delete(before, filepath.Join(project, filepath.FromSlash(path)))
				delete(after, filepath.Join(project, filepath.FromSlash(path)))
			}
			if !maps.Equal(before, after) {
				t.Error("switch changed data outside the cursor and its parent metadata")
			}
		})
	}
}

func TestSwitchSpaceCursorFailurePreservesOldFile(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"write", "short write", "chmod", "close", "rename", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{},
				map[string]string{"aidlc/active-space": "old bytes"},
			)
			before := snapshotSpaceTree(t, project)
			cause := errors.New("injected " + stage + " failure")
			cleanupCause := errors.New("injected cleanup failure")
			var ownedTemp string
			name, err := switchSpace(
				RootInput{ExplicitDir: project},
				"default",
				os.OpenRoot,
				(*os.Root).Close,
				func(projectRoot *os.Root, name string) (err error) {
					root, err := projectRoot.OpenRoot("aidlc")
					if err != nil {
						return err
					}
					defer func() { err = errors.Join(err, root.Close()) }()
					ops := cursorOperations(root)
					ops.openFile = func(path string, flags int, mode fs.FileMode) (*os.File, error) {
						ownedTemp = path
						return root.OpenFile(path, flags, mode)
					}
					switch stage {
					case "write", "short write", "cleanup":
						ops.write = func(file *os.File, _ string) (int, error) {
							n, err := file.WriteString("x")
							if stage == "short write" {
								return n, err
							}
							return n, errors.Join(err, cause)
						}
					case "chmod":
						ops.chmod = func(*os.File, fs.FileMode) error { return cause }
					case "close":
						ops.close = func(file *os.File) error { return errors.Join(file.Close(), cause) }
					case "rename":
						ops.rename = func(string, string) error { return cause }
					}
					if stage == "cleanup" {
						ops.remove = func(string) error { return cleanupCause }
					}
					return replaceSpaceCursor(name, ops)
				},
			)
			if stage == "short write" {
				cause = io.ErrShortWrite
			}
			if name != "" || !errors.Is(err, cause) {
				t.Errorf(
					"switchSpace() = (%q, %v), want empty name and cause %v",
					name,
					err,
					cause,
				)
			}
			after := snapshotSpaceTree(t, project)
			if stage == "cleanup" {
				if !errors.Is(err, cleanupCause) {
					t.Errorf("error %v lost cleanup cause", err)
				}
				path := filepath.Join(project, "aidlc", ownedTemp)
				if _, exists := after[path]; !exists {
					t.Error("failed cleanup did not leave the owned temporary file")
				}
				delete(after, path)
			}
			delete(before, filepath.Join(project, "aidlc"))
			delete(after, filepath.Join(project, "aidlc"))
			if !maps.Equal(before, after) {
				t.Error("pre-rename failure changed the old cursor or left unexpected files")
			}
		})
	}
}

func TestSwitchSpaceRootCloseFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		isBadCursor bool
		failAidlc   bool
		failProject bool
	}{
		{name: "success"},
		{name: "aidlc close after save", failAidlc: true},
		{name: "project close after save", failProject: true},
		{name: "both closes after save", failAidlc: true, failProject: true},
		{name: "primary error and both closes", isBadCursor: true, failAidlc: true, failProject: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			dirs := []string{"aidlc"}
			if tt.isBadCursor {
				dirs = append(dirs, "aidlc/active-space")
			}
			writeSpaceFixture(
				t,
				project,
				dirs,
				map[string]string{},
			)
			aidlcCause := errors.New("aidlc close failure")
			projectCause := errors.New("project close failure")
			steps := []string{}
			roots := []*os.Root{}
			name, err := switchSpace(
				RootInput{ExplicitDir: project},
				"default",
				os.OpenRoot,
				func(root *os.Root) error {
					roots = append(roots, root)
					steps = append(steps, "project")
					if tt.failProject {
						return errors.Join(root.Close(), projectCause)
					}
					return root.Close()
				},
				func(root *os.Root, name string) error {
					return saveCursorInRoot(
						name,
						func() (*os.Root, error) { return root.OpenRoot("aidlc") },
						func(root *os.Root) error {
							roots = append(roots, root)
							steps = append(steps, "aidlc")
							if tt.failAidlc {
								return errors.Join(root.Close(), aidlcCause)
							}
							return root.Close()
						},
					)
				},
			)
			if tt.isBadCursor && !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("error %v lost primary cursor type failure", err)
			}
			if tt.failAidlc && !errors.Is(err, aidlcCause) {
				t.Errorf("error %v lost aidlc close failure", err)
			}
			if tt.failProject && !errors.Is(err, projectCause) {
				t.Errorf("error %v lost project close failure", err)
			}
			hasCloseFailure := tt.failAidlc || tt.failProject
			wantFailure := tt.isBadCursor || hasCloseFailure
			if (err != nil) != wantFailure {
				t.Errorf("error=%v, want failure=%t", err, wantFailure)
			}
			want := "default"
			if err != nil {
				want = ""
			}
			if name != want {
				t.Errorf("name = %q, want %q", name, want)
			}
			if !slices.Equal(steps, []string{"aidlc", "project"}) {
				t.Errorf("root close order = %q, want aidlc then project once each", steps)
			}
			for _, root := range roots {
				if _, err := root.Stat("."); err == nil {
					t.Error("acquired root remains open")
				}
			}
			if !tt.isBadCursor {
				data, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
				if err != nil || string(data) != "default\n" {
					t.Errorf("cursor after close = (%q, %v), want saved value without rollback", data, err)
				}
			}
		})
	}
}

func TestSwitchSpaceCursorLinkBoundaries(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"inside relative", "outside relative", "absolute inside", "broken"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			outside := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{"aidlc"},
				map[string]string{"aidlc/old": "old"},
			)
			writeSpaceFixture(
				t,
				outside,
				[]string{},
				map[string]string{"keep": "outside"},
			)
			target := "old"
			switch kind {
			case "outside relative":
				var err error
				target, err = filepath.Rel(filepath.Join(project, "aidlc"), filepath.Join(outside, "keep"))
				if err != nil {
					t.Fatal(err)
				}
			case "absolute inside":
				target = filepath.Join(project, "aidlc", "old")
			case "broken":
				target = "missing"
			}
			createSpaceSymlink(t, target, filepath.Join(project, "aidlc", "active-space"))
			before, outsideBefore := snapshotSpaceTree(t, project), snapshotSpaceTree(t, outside)
			name, err := SwitchSpace(RootInput{ExplicitDir: project}, "default")
			if name != "" || !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("SwitchSpace() = (%q, %v), want empty name and fs.ErrInvalid", name, err)
			}
			if !maps.Equal(before, snapshotSpaceTree(t, project)) || !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("rejected cursor symlink changed a tree")
			}
		})
	}
}

func TestSwitchSpaceAncestorLinkBoundaries(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"inside relative", "outside relative", "absolute inside"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			outside := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{"storage"},
				map[string]string{},
			)
			writeSpaceFixture(
				t,
				outside,
				[]string{},
				map[string]string{"keep": "outside"},
			)
			target := "storage"
			switch kind {
			case "outside relative":
				var err error
				target, err = filepath.Rel(project, outside)
				if err != nil {
					t.Fatal(err)
				}
			case "absolute inside":
				target = filepath.Join(project, "storage")
			}
			createSpaceSymlink(t, target, filepath.Join(project, "aidlc"))
			before, outsideBefore := snapshotSpaceTree(t, project), snapshotSpaceTree(t, outside)
			name, err := SwitchSpace(RootInput{ExplicitDir: project}, "default")
			if kind == "inside relative" {
				if name != "default" || err != nil {
					t.Errorf("SwitchSpace() = (%q, %v), want default through inside relative parent", name, err)
				}
				data, err := os.ReadFile(filepath.Join(project, "storage", "active-space"))
				if err != nil || string(data) != "default\n" {
					t.Errorf("inside cursor=(%q, %v), want default", data, err)
				}
			} else {
				if name != "" || err == nil {
					t.Errorf("SwitchSpace()=(%q, %v), want rejected parent link", name, err)
				}
				if !maps.Equal(before, snapshotSpaceTree(t, project)) {
					t.Error("rejected ancestor changed the project")
				}
			}
			if !maps.Equal(outsideBefore, snapshotSpaceTree(t, outside)) {
				t.Error("switch changed a tree outside the project root")
			}
		})
	}
}

func TestSwitchSpaceMembershipUsesListing(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{
		"inside relative", "outside relative", "absolute inside", "broken before target", "file",
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			outside := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{"aidlc/spaces/zeta"},
				map[string]string{"aidlc/active-space": "old"},
			)
			beforeOutside := snapshotSpaceTree(t, outside)
			space := "alpha"
			target := "zeta"
			switch kind {
			case "outside relative":
				var err error
				target, err = filepath.Rel(filepath.Join(project, "aidlc", "spaces"), outside)
				if err != nil {
					t.Fatal(err)
				}
			case "absolute inside":
				target = filepath.Join(
					project,
					"aidlc",
					"spaces",
					"zeta",
				)
			case "broken before target":
				target = "missing"
				space = "zeta"
			}
			if kind == "file" {
				writeSpaceFixture(
					t,
					project,
					[]string{},
					map[string]string{"aidlc/spaces/alpha": "not a directory"},
				)
			} else {
				createSpaceSymlink(t, target, filepath.Join(
					project,
					"aidlc",
					"spaces",
					"alpha",
				))
			}
			before := snapshotSpaceTree(t, project)
			name, err := SwitchSpace(RootInput{ExplicitDir: project}, space)
			if kind == "inside relative" {
				if name != "alpha" || err != nil {
					t.Errorf("SwitchSpace() = (%q, %v), want listed internal directory link", name, err)
				}
			} else {
				if name != "" || err == nil || !strings.Contains(err.Error(), "unknown space") {
					t.Errorf("SwitchSpace() = (%q, %v), want unknown listed name", name, err)
				}
				if !maps.Equal(before, snapshotSpaceTree(t, project)) {
					t.Error("unlisted target changed the project")
				}
			}
			if !maps.Equal(beforeOutside, snapshotSpaceTree(t, outside)) {
				t.Error("membership lookup changed the outside tree")
			}
		})
	}
}

func TestSwitchSpaceInitialProjectLink(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	project := t.TempDir()
	link := filepath.Join(base, "project-link")
	createSpaceSymlink(t, project, link)
	name, err := SwitchSpace(RootInput{ExplicitDir: link}, "default")
	if name != "default" || err != nil {
		t.Fatalf("SwitchSpace() = (%q, %v), want default at initial project link", name, err)
	}
	data, err := os.ReadFile(filepath.Join(project, "aidlc", "active-space"))
	if err != nil || string(data) != "default\n" {
		t.Errorf("initial project target cursor=(%q, %v), want default", data, err)
	}
}

func TestSwitchSpaceFilesystemFailures(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"missing project", "aidlc file", "cursor directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			input := RootInput{ExplicitDir: project, WorkingDir: project}
			switch kind {
			case "missing project":
				input.ExplicitDir = filepath.Join(project, "missing")
			case "aidlc file":
				writeSpaceFixture(
					t,
					project,
					[]string{},
					map[string]string{"aidlc": "not a directory"},
				)
			case "cursor directory":
				writeSpaceFixture(
					t,
					project,
					[]string{"aidlc/active-space"},
					map[string]string{},
				)
			}
			before := snapshotSpaceTree(t, project)
			name, err := SwitchSpace(input, "default")
			if name != "" || err == nil {
				t.Errorf("SwitchSpace() = (%q, %v), want failure", name, err)
			}
			if kind == "missing project" && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("missing project error=%v, want fs.ErrNotExist", err)
			}
			if !maps.Equal(before, snapshotSpaceTree(t, project)) {
				t.Error("filesystem failure changed an existing tree")
			}
		})
	}
}

func TestSwitchSpaceFileModes(t *testing.T) {
	t.Parallel()

	for _, isExisting := range []bool{false, true} {
		name := "new cursor follows umask"
		if isExisting {
			name = "existing cursor preserves permission bits"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			project := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				[]string{"aidlc"},
				map[string]string{},
			)
			reference := filepath.Join(project, "reference-mode")
			if err := os.WriteFile(reference, []byte("reference"), 0o666); err != nil {
				t.Fatal(err)
			}
			cursor := filepath.Join(project, "aidlc", "active-space")
			if isExisting {
				reference = cursor
				if err := os.WriteFile(cursor, []byte("old"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Stat(reference)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := SwitchSpace(RootInput{ExplicitDir: project}, "default"); err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(cursor)
			if err != nil {
				t.Fatal(err)
			}
			if after.Mode().Perm() != before.Mode().Perm() {
				t.Errorf("cursor permissions=%o, want %o", after.Mode().Perm(), before.Mode().Perm())
			}
		})
	}
}
