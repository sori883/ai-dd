//go:build integration

package workspace

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

func TestReadIntentsFilesystemBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("initial project symlink is followed", func(t *testing.T) {
		t.Parallel()

		fixture := t.TempDir()
		project := filepath.Join(fixture, "real-project")
		writeIntentIntegrationRecord(t, filepath.Join(project, "aidlc", "spaces", "default", "intents"))
		projectLink := filepath.Join(fixture, "project-link")
		createSpaceSymlink(t, project, projectLink)

		got, err := readIntentsIntegrationWithoutChanges(t, RootInput{ExplicitDir: projectLink}, fixture)
		if err != nil {
			t.Fatalf("ReadIntents() error = %v, want nil", err)
		}
		assertIntentIntegrationListing(t, got, projectLink)
	})

	t.Run("project-internal relative intents symlink is followed", func(t *testing.T) {
		t.Parallel()

		fixture := t.TempDir()
		project := filepath.Join(fixture, "project")
		target := filepath.Join(project, "aidlc", "shared-intents")
		writeIntentIntegrationRecord(t, target)
		link := filepath.Join(project, "aidlc", "spaces", "default", "intents")
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		relativeTarget, err := filepath.Rel(filepath.Dir(link), target)
		if err != nil {
			t.Fatal(err)
		}
		createSpaceSymlink(t, relativeTarget, link)

		got, err := readIntentsIntegrationWithoutChanges(t, RootInput{ExplicitDir: project}, fixture)
		if err != nil {
			t.Fatalf("ReadIntents() error = %v, want nil", err)
		}
		assertIntentIntegrationListing(t, got, project)
	})

	for _, tt := range []struct {
		name           string
		absoluteTarget bool
	}{
		{name: "project-external relative intents symlink is rejected"},
		{name: "absolute intents symlink is rejected", absoluteTarget: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := t.TempDir()
			project := filepath.Join(fixture, "project")
			target := filepath.Join(fixture, "outside-intents")
			writeIntentIntegrationRecord(t, target)
			link := filepath.Join(project, "aidlc", "spaces", "default", "intents")
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			linkTarget := target
			if !tt.absoluteTarget {
				var err error
				linkTarget, err = filepath.Rel(filepath.Dir(link), target)
				if err != nil {
					t.Fatal(err)
				}
			}
			createSpaceSymlink(t, linkTarget, link)

			got, err := readIntentsIntegrationWithoutChanges(t, RootInput{ExplicitDir: project}, fixture)
			if err == nil {
				t.Fatalf("ReadIntents() error = nil, want boundary error; listing = %#v", got)
			}
			assertIntentListing(t, got, IntentListing{})
		})
	}

	t.Run("broken intents symlink is treated as missing", func(t *testing.T) {
		t.Parallel()

		fixture := t.TempDir()
		project := filepath.Join(fixture, "project")
		link := filepath.Join(project, "aidlc", "spaces", "default", "intents")
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		createSpaceSymlink(t, "missing-target", link)

		got, err := readIntentsIntegrationWithoutChanges(t, RootInput{ExplicitDir: project}, fixture)
		if err != nil {
			t.Fatalf("ReadIntents() error = %v, want missing-root fallback", err)
		}
		assertIntentListing(t, got, IntentListing{
			ProjectRoot: project,
			SpaceName:   "default",
			Intents:     []Intent{},
		})
	})
}

func writeIntentIntegrationRecord(t *testing.T, intentsRoot string) {
	t.Helper()

	writeSpaceFixture(t, intentsRoot, nil, map[string]string{
		"intents.json":                     `[]`,
		"active-intent":                    "240901-build-auth\n",
		"240901-build-auth/aidlc-state.md": "state",
	})
}

func assertIntentIntegrationListing(t *testing.T, got IntentListing, project string) {
	t.Helper()

	dirName := "240901-build-auth"
	assertIntentListing(t, got, IntentListing{
		ProjectRoot: project,
		SpaceName:   "default",
		Intents: []Intent{{
			Slug: "build-auth", Status: "unknown", Repos: []string{}, DirName: &dirName, Active: true,
		}},
	})
}

func readIntentsIntegrationWithoutChanges(
	t *testing.T,
	input RootInput,
	snapshotRoot string,
) (IntentListing, error) {
	t.Helper()

	before := snapshotSpaceTree(t, snapshotRoot)
	listing, err := ReadIntents(input)
	if after := snapshotSpaceTree(t, snapshotRoot); !maps.Equal(after, before) {
		t.Errorf("filesystem changed: before=%v, after=%v", before, after)
	}
	return listing, err
}
