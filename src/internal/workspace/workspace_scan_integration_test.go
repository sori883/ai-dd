//go:build integration

package workspace

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDetectIntegrationKnownWorkspaceIsReadOnly(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	writeScanIntegrationFile(t, project, "package.json", `{"dependencies":{"react":"18"}}`)
	writeScanIntegrationFile(t, project, "vite.config.ts", "export default {};\n")
	writeScanIntegrationFile(t, project, "src/App.tsx", "export const App = () => null;\n")
	before := snapshotScanIntegrationTree(t, project)

	root, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close project root: %v", err)
		}
	})
	got := Detect(root)
	want := ScanResult{
		ProjectType: "Brownfield",
		Languages:   "TypeScript",
		Frameworks:  "Vite, React",
		BuildSystem: "npm (package.json)",
		Submodules:  []Submodule{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Detect() = %#v, want %#v", got, want)
	}
	if _, err := root.Stat("."); err != nil {
		t.Errorf("Detect() closed caller-owned root: %v", err)
	}
	after := snapshotScanIntegrationTree(t, project)
	if !reflect.DeepEqual(after, before) {
		t.Errorf("Detect() changed project tree\nbefore: %v\nafter:  %v", before, after)
	}
}

func TestDetectIntegrationSymlinkBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("inner relative config link is followed", func(t *testing.T) {
		t.Parallel()

		project := t.TempDir()
		writeScanIntegrationFile(t, project, "config/package.json", `{"dependencies":{"react":"18"}}`)
		createScanIntegrationSymlink(t, "config/package.json", filepath.Join(project, "package.json"))
		assertScanIntegrationUnchanged(t, project, func(root *os.Root) {
			got := Detect(root)
			if got.ProjectType != "Brownfield" || got.Frameworks != "React" {
				t.Errorf("Detect() = (%q, %q), want inner linked React", got.ProjectType, got.Frameworks)
			}
		})
	})

	outside := t.TempDir()
	outsidePackage := filepath.Join(outside, "package.json")
	writeScanIntegrationFile(t, outside, "package.json", `{"dependencies":{"react":"18"}}`)
	for _, tt := range []struct {
		name   string
		target func(string) (string, error)
	}{
		{name: "outer relative config", target: func(project string) (string, error) {
			return filepath.Rel(project, outsidePackage)
		}},
		{name: "absolute config", target: func(string) (string, error) { return outsidePackage, nil }},
		{name: "broken config", target: func(string) (string, error) { return "missing/package.json", nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := t.TempDir()
			target, err := tt.target(project)
			if err != nil {
				t.Fatal(err)
			}
			createScanIntegrationSymlink(t, target, filepath.Join(project, "package.json"))
			assertScanIntegrationUnchanged(t, project, func(root *os.Root) {
				got := Detect(root)
				if got.ProjectType != "Greenfield" || got.Frameworks != "Unknown" || got.BuildSystem != "npm (package.json)" {
					t.Errorf(
						"Detect() = (%q, %q, %q), want rejected config signal",
						got.ProjectType,
						got.Frameworks,
						got.BuildSystem,
					)
				}
			})
		})
	}

	t.Run("outer source and nested links are ignored", func(t *testing.T) {
		t.Parallel()

		outsideSources := t.TempDir()
		writeScanIntegrationFile(t, outsideSources, "main.go", "package outside\n")
		project := t.TempDir()
		createScanIntegrationSymlink(t, filepath.Join(outsideSources, "main.go"), filepath.Join(project, "main.go"))
		createScanIntegrationSymlink(t, outsideSources, filepath.Join(project, "container"))
		assertScanIntegrationUnchanged(t, project, func(root *os.Root) {
			got := Detect(root)
			if got.ProjectType != "Greenfield" || got.Languages != "Unknown" || got.NestedRoot != "" {
				t.Errorf("Detect() = (%q, %q, %q), want linked source ignored", got.ProjectType, got.Languages, got.NestedRoot)
			}
		})
	})

	t.Run("outer submodule git link is uninitialized", func(t *testing.T) {
		t.Parallel()

		outsideGit := t.TempDir()
		project := t.TempDir()
		writeScanIntegrationFile(t, project, ".gitmodules", "[submodule \"module\"]\n\tpath = module\n")
		if err := os.MkdirAll(filepath.Join(project, "module"), 0o755); err != nil {
			t.Fatal(err)
		}
		createScanIntegrationSymlink(t, outsideGit, filepath.Join(project, "module", ".git"))
		assertScanIntegrationUnchanged(t, project, func(root *os.Root) {
			got := Detect(root)
			if len(got.Submodules) != 1 || got.Submodules[0].Initialized {
				t.Errorf("Detect().Submodules = %#v, want outer .git link rejected", got.Submodules)
			}
		})
	})
}

func assertScanIntegrationUnchanged(t *testing.T, project string, scan func(*os.Root)) {
	t.Helper()

	before := snapshotScanIntegrationTree(t, project)
	root, err := os.OpenRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	scan(root)
	if _, err := root.Stat("."); err != nil {
		t.Errorf("Detect() closed caller-owned root: %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	after := snapshotScanIntegrationTree(t, project)
	if !reflect.DeepEqual(after, before) {
		t.Errorf("Detect() changed project tree\nbefore: %v\nafter:  %v", before, after)
	}
}

func writeScanIntegrationFile(t *testing.T, root, name, content string) {
	t.Helper()

	fullName := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(fullName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullName, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createScanIntegrationSymlink(t *testing.T, target, name string) {
	t.Helper()

	if err := os.Symlink(target, name); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable on Windows: %v", err)
		}
		t.Fatal(err)
	}
}

func snapshotScanIntegrationTree(t *testing.T, root string) []string {
	t.Helper()

	snapshot := []string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		record := fmt.Sprintf("%s|%s", filepath.ToSlash(relative), info.Mode())
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			record += "|" + target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			record += fmt.Sprintf("|%x", sha256.Sum256(data))
		}
		snapshot = append(snapshot, record)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
