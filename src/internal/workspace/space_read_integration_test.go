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

func TestReadSpacesSharedCursorListing(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeSpaceFixture(
		t,
		project,
		[]string{"aidlc/spaces/zeta", "aidlc/spaces/alpha"},
		map[string]string{"aidlc/active-space": "zeta\n", "aidlc/spaces/a-file": "not a space"},
	)
	before := snapshotSpaceTree(t, project)
	got, err := ReadSpaces(RootInput{ExplicitDir: project})
	if err != nil {
		t.Fatal(err)
	}
	want := []Space{{Name: "alpha"}, {Name: "default"}, {Name: "zeta", Active: true}}
	if !slices.Equal(got, want) {
		t.Errorf("ReadSpaces() = %v, want %v", got, want)
	}
	if after := snapshotSpaceTree(t, project); !maps.Equal(before, after) {
		t.Error("ReadSpaces changed the project tree")
	}
}

func TestReadSpacesProjectClose(t *testing.T) {
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
			got, err := readSpaces(
				RootInput{ExplicitDir: t.TempDir()},
				func(path string) (*os.Root, error) {
					root, err := os.OpenRoot(path)
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
			)
			if !errors.Is(err, tt.closeErr) {
				t.Errorf("readSpaces() error = %v, want cause %v", err, tt.closeErr)
			}
			if tt.closeErr != nil && got != nil {
				t.Errorf("readSpaces() = %v on close failure, want nil", got)
			}
			if captured == nil {
				t.Fatal("project root was not opened")
			}
			if closeCalls != 1 {
				t.Errorf("close calls = %d, want 1", closeCalls)
			}
			if _, err := captured.Stat("."); err == nil {
				t.Error("project root remains open after readSpaces returned")
			}
		})
	}
}

func TestReadSpacesFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		dirs  []string
		files map[string]string
		want  []Space
	}{
		{name: "uninitialized project", want: []Space{{Name: "default", Active: true}}},
		{name: "missing spaces", dirs: []string{"aidlc"}, want: []Space{{Name: "default", Active: true}}},
		{
			name: "empty spaces directory",
			dirs: []string{"aidlc/spaces"},
			want: []Space{{Name: "default", Active: true}},
		},
		{
			name: "created default is not duplicated",
			dirs: []string{"aidlc/spaces/default"},
			want: []Space{{Name: "default", Active: true}},
		},
		{
			name:  "aidlc is a file",
			files: map[string]string{"aidlc": "not a directory"},
			want:  []Space{{Name: "default", Active: true}},
		},
		{
			name:  "spaces is a file and cursor is unknown",
			files: map[string]string{"aidlc/spaces": "not a directory", "aidlc/active-space": "alpha\n"},
			want:  []Space{{Name: "default"}},
		},
		{
			name: "missing cursor",
			dirs: []string{"aidlc/spaces/alpha"},
			want: []Space{{Name: "alpha"}, {Name: "default", Active: true}},
		},
		{
			name: "cursor is a directory",
			dirs: []string{"aidlc/spaces/alpha", "aidlc/active-space"},
			want: []Space{{Name: "alpha"}, {Name: "default", Active: true}},
		},
		{
			name:  "blank cursor",
			dirs:  []string{"aidlc/spaces/alpha"},
			files: map[string]string{"aidlc/active-space": "\t\n\ufeff"},
			want:  []Space{{Name: "alpha"}, {Name: "default", Active: true}},
		},
		{
			name:  "JS BOM trim",
			dirs:  []string{"aidlc/spaces/alpha"},
			files: map[string]string{"aidlc/active-space": " \ufeffalpha\ufeff\r\n"},
			want:  []Space{{Name: "alpha", Active: true}, {Name: "default"}},
		},
		{
			name:  "JS NEL retained",
			dirs:  []string{"aidlc/spaces/alpha"},
			files: map[string]string{"aidlc/active-space": "\u0085alpha\u0085"},
			want:  []Space{{Name: "alpha"}, {Name: "default"}},
		},
		{
			name:  "cursor is not a path input",
			dirs:  []string{"aidlc/spaces/alpha"},
			files: map[string]string{"aidlc/active-space": "../outside"},
			want:  []Space{{Name: "alpha"}, {Name: "default"}},
		},
		{
			name: "UTF16 order",
			dirs: []string{"aidlc/spaces/\ue000", "aidlc/spaces/\U00010000"},
			want: []Space{{Name: "default", Active: true}, {Name: "\U00010000"}, {Name: "\ue000"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			writeSpaceFixture(
				t,
				project,
				tt.dirs,
				tt.files,
			)
			before := snapshotSpaceTree(t, project)
			got, err := ReadSpaces(RootInput{ExplicitDir: project})
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf(
					"ReadSpaces()=(%v, %v), want %v, nil",
					got,
					err,
					tt.want,
				)
			}
			if after := snapshotSpaceTree(t, project); !maps.Equal(before, after) {
				t.Error("ReadSpaces fallback changed the project")
			}
		})
	}
}

func TestReadSpacesProjectOpenIntegration(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"missing", "ordinary file"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			project := filepath.Join(base, "project")
			if name == "ordinary file" {
				if err := os.WriteFile(project, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotSpaceTree(t, base)
			got, err := ReadSpaces(RootInput{ExplicitDir: project, WorkingDir: base})
			if err == nil || got != nil {
				t.Errorf("ReadSpaces()=(%v, %v), want nil and open error without cwd fallback", got, err)
			}
			if name == "missing" && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("missing project error=%v, want fs.ErrNotExist cause", err)
			}
			if _, ok := errors.AsType[*fs.PathError](err); !ok {
				t.Errorf("open error=%v, want wrapped filesystem cause", err)
			}
			if after := snapshotSpaceTree(t, base); !maps.Equal(before, after) {
				t.Error("failed project open changed the fixture")
			}
		})
	}
}

func TestReadSpacesSymlinkBoundaries(t *testing.T) {
	t.Parallel()

	for _, location := range []string{"cursor", "spaces directory", "space entry"} {
		linkKinds := []string{"relative inside", "relative outside", "absolute inside", "broken"}
		if location == "space entry" {
			linkKinds = append(linkKinds, "relative file")
		}
		for _, kind := range linkKinds {
			t.Run(location+"/"+kind, func(t *testing.T) {
				t.Parallel()

				base := t.TempDir()
				project := filepath.Join(base, "project")
				dirs := []string{"project/aidlc", "project/stored-spaces/team", "outside/spaces/team"}
				files := map[string]string{
					"project/cursors/selected": "team\n",
					"outside/selected":         "team\n",
				}
				var link, inside, outside string
				var want []Space
				switch location {
				case "cursor":
					dirs = append(dirs, "project/aidlc/spaces/team")
					link = filepath.Join(project, "aidlc", "active-space")
					inside = filepath.Join(project, "cursors", "selected")
					outside = filepath.Join(base, "outside", "selected")
					want = []Space{{Name: "default", Active: true}, {Name: "team"}}
					if kind == "relative inside" {
						want = []Space{{Name: "default"}, {Name: "team", Active: true}}
					}
				case "spaces directory":
					files["project/aidlc/active-space"] = "team\n"
					link = filepath.Join(project, "aidlc", "spaces")
					inside = filepath.Join(project, "stored-spaces")
					outside = filepath.Join(base, "outside", "spaces")
					want = []Space{{Name: "default"}}
					if kind == "relative inside" {
						want = append(want, Space{Name: "team", Active: true})
					}
				case "space entry":
					dirs = append(dirs, "project/aidlc/spaces/a-before", "project/aidlc/spaces/z-after")
					files["project/aidlc/active-space"] = "a-before\n"
					link = filepath.Join(
						project,
						"aidlc",
						"spaces",
						"m-link",
					)
					inside = filepath.Join(project, "stored-spaces", "team")
					outside = filepath.Join(
						base,
						"outside",
						"spaces",
						"team",
					)
					want = []Space{{Name: "a-before", Active: true}, {Name: "default"}}
					if kind == "relative inside" {
						want = append(want, Space{Name: "m-link"}, Space{Name: "z-after"})
					}
					if kind == "relative file" {
						inside = filepath.Join(project, "cursors", "selected")
						want = append(want, Space{Name: "z-after"})
					}
				}
				writeSpaceFixture(
					t,
					base,
					dirs,
					files,
				)
				target := "missing-target"
				switch kind {
				case "relative inside", "relative file":
					var err error
					target, err = filepath.Rel(filepath.Dir(link), inside)
					if err != nil {
						t.Fatal(err)
					}
				case "relative outside":
					var err error
					target, err = filepath.Rel(filepath.Dir(link), outside)
					if err != nil {
						t.Fatal(err)
					}
				case "absolute inside":
					target = inside
				}
				createSpaceSymlink(t, target, link)
				before := snapshotSpaceTree(t, base)
				got, err := ReadSpaces(RootInput{ExplicitDir: project})
				if err != nil || !slices.Equal(got, want) {
					t.Errorf(
						"ReadSpaces()=(%v, %v), want %v, nil",
						got,
						err,
						want,
					)
				}
				if after := snapshotSpaceTree(t, base); !maps.Equal(before, after) {
					t.Error("ReadSpaces changed project or outside symlink fixtures")
				}
			})
		}
	}
}

func TestReadSpacesInitialProjectSymlink(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	writeSpaceFixture(
		t,
		base,
		[]string{"project with spaces/aidlc/spaces/team"},
		map[string]string{"project with spaces/aidlc/active-space": "team\n"},
	)
	projectLink := filepath.Join(base, "project link")
	createSpaceSymlink(t, filepath.Join(base, "project with spaces"), projectLink)
	before := snapshotSpaceTree(t, base)
	got, err := ReadSpaces(RootInput{ExplicitDir: projectLink})
	want := []Space{{Name: "default"}, {Name: "team", Active: true}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf(
			"ReadSpaces()=(%v, %v), want %v, nil",
			got,
			err,
			want,
		)
	}
	if after := snapshotSpaceTree(t, base); !maps.Equal(before, after) {
		t.Error("ReadSpaces changed the project symlink or target")
	}
}
